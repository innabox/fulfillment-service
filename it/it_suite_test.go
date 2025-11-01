/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package it

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/innabox/fulfillment-common/auth"
	"github.com/innabox/fulfillment-common/logging"
	"github.com/innabox/fulfillment-common/network"
	. "github.com/innabox/fulfillment-common/testing"
	"github.com/kelseyhightower/envconfig"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/ginkgo/v2/dsl/decorators"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
)

// Config contains configuration options for the integration tests.
type Config struct {
	// KeepKind indicates whether to preserve the kind cluster after tests complete.
	// By default, the kind cluster is deleted after running the tests.
	KeepKind bool `json:"keep_kind" envconfig:"keep_kind" default:"false"`

	// KeepService indicates whether to preserve the application chart after tests complete.
	// By default, the application chart is uninstalled after running the tests.
	KeepService bool `json:"keep_service" envconfig:"keep_service" default:"false"`
}

var (
	logger      *slog.Logger
	config      *Config
	cluster     *Kind
	clientConn  *grpc.ClientConn
	adminConn   *grpc.ClientConn
	userClient  *http.Client
	adminClient *http.Client
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration")
}

var _ = BeforeSuite(func() {
	var err error

	// Create a context:
	ctx := context.Background()

	// Create the logger:
	logger, err = logging.NewLogger().
		SetWriter(GinkgoWriter).
		SetLevel(slog.LevelDebug.String()).
		Build()
	Expect(err).ToNot(HaveOccurred())

	// Load configuration from environment variables:
	config = &Config{}
	err = envconfig.Process("it", config)
	Expect(err).ToNot(HaveOccurred())
	logger.Info(
		"Configuration",
		slog.Any("values", config),
	)

	// Configure the Kubernetes libraries to use our logger:
	logrLogger := logr.FromSlogHandler(logger.Handler())
	crlog.SetLogger(logrLogger)
	klog.SetLogger(logrLogger)

	// Create a temporary directory:
	tmpDir, err := os.MkdirTemp("", "*.it")
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		err := os.RemoveAll(tmpDir)
		Expect(err).ToNot(HaveOccurred())
	})

	// Get the project directory:
	currentDir, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	for {
		modFile := filepath.Join(currentDir, "go.mod")
		_, err := os.Stat(modFile)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			Expect(err).ToNot(HaveOccurred())
		}
		parentDir := filepath.Dir(currentDir)
		Expect(parentDir).ToNot(Equal(currentDir))
		currentDir = parentDir
	}
	projectDir := currentDir

	// Check that the required tools are available:
	_, err = exec.LookPath(kubectlCmd)
	Expect(err).ToNot(HaveOccurred())
	_, err = exec.LookPath(podmanCmd)
	Expect(err).ToNot(HaveOccurred())
	_, err = exec.LookPath(helmCmd)
	Expect(err).ToNot(HaveOccurred())

	// Start the cluster, and remember to stop it:
	cluster, err = NewKind().
		SetLogger(logger).
		SetName("it").
		AddCrdFile(filepath.Join("crds", "clusterorders.cloudkit.openshift.io.yaml")).
		AddCrdFile(filepath.Join("crds", "hostedclusters.hypershift.openshift.io.yaml")).
		Build()
	Expect(err).ToNot(HaveOccurred())
	err = cluster.Start(ctx)
	Expect(err).ToNot(HaveOccurred())
	if !config.KeepKind {
		DeferCleanup(func() {
			err := cluster.Stop(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	}

	// Build the container image:
	imageTag := time.Now().Format("20060102150405")
	imageRef := fmt.Sprintf("%s:%s", imageName, imageTag)
	buildCmd, err := NewCommand().
		SetLogger(logger).
		SetDir(projectDir).
		SetName("podman").
		SetArgs(
			"build",
			"--tag", imageRef,
			"--file", "Containerfile",
		).
		Build()
	Expect(err).ToNot(HaveOccurred())
	err = buildCmd.Execute(ctx)
	Expect(err).ToNot(HaveOccurred())

	// Save the container image to the tar file and load it into the cluster:
	imageTar := filepath.Join(tmpDir, "image.tar")
	saveCmd, err := NewCommand().
		SetLogger(logger).
		SetDir(projectDir).
		SetName(podmanCmd).
		SetArgs(
			"save",
			"--output", imageTar,
			imageRef,
		).
		Build()
	Expect(err).ToNot(HaveOccurred())
	err = saveCmd.Execute(ctx)
	Expect(err).ToNot(HaveOccurred())
	err = cluster.LoadArchive(ctx, imageTar)
	Expect(err).ToNot(HaveOccurred())

	// Get the kubeconfig:
	kcFile := filepath.Join(tmpDir, "kubeconfig")
	err = os.WriteFile(kcFile, cluster.Kubeconfig(), 0400)
	Expect(err).ToNot(HaveOccurred())

	// Get the client:
	kubeClient := cluster.Client()
	kubeClientSet := cluster.ClientSet()

	// Load the certificates of the CA bundle of the cluster:
	var caFiles []string
	caBundleKey := crclient.ObjectKey{
		Namespace: "default",
		Name:      "ca-bundle",
	}
	caBundleMap := &corev1.ConfigMap{}
	Eventually(
		func(g Gomega) {
			err := kubeClient.Get(ctx, caBundleKey, caBundleMap)
			g.Expect(err).ToNot(HaveOccurred())
		},
		time.Minute,
		time.Second,
	).Should(Succeed())
	for caBundleKey, caBundleText := range caBundleMap.Data {
		caBundleFile := filepath.Join(tmpDir, caBundleKey)
		err = os.WriteFile(caBundleFile, []byte(caBundleText), 0400)
		Expect(err).ToNot(HaveOccurred())
		caFiles = append(caFiles, caBundleFile)
	}

	// Create the CA pool:
	caPool, err := network.NewCertPool().
		SetLogger(logger).
		AddFiles(caFiles...).
		Build()
	Expect(err).ToNot(HaveOccurred())

	// Install Keycloak:
	logger.DebugContext(ctx, "Installing Keycloak chart")
	installCmd, err := NewCommand().
		SetLogger(logger).
		SetDir(projectDir).
		SetName(helmCmd).
		SetArgs(
			"upgrade",
			"--install",
			"keycloak",
			"charts/keycloak",
			"--kubeconfig", kcFile,
			"--namespace", "keycloak",
			"--create-namespace",
			"--set", "variant=kind",
			"--set", "hostname=keycloak.keycloak.svc.cluster.local",
			"--set", "certs.issuerRef.name=default-ca",
		).
		Build()
	Expect(err).ToNot(HaveOccurred())
	err = installCmd.Execute(ctx)
	Expect(err).ToNot(HaveOccurred())

	// Deploy the service, and remember to uninstall it:
	logger.DebugContext(ctx, "Installing service")
	installCmd, err = NewCommand().
		SetLogger(logger).
		SetDir(projectDir).
		SetName(helmCmd).
		SetArgs(
			"upgrade",
			"--install",
			"fulfillment-service",
			"charts/service",
			"--kubeconfig", kcFile,
			"--namespace", "innabox",
			"--create-namespace",
			"--set", "log.level=debug",
			"--set", "log.headers=true",
			"--set", "log.bodies=true",
			"--set", "variant=kind",
			"--set", fmt.Sprintf("images.service=%s", imageRef),
			"--set", "certs.issuerRef.kind=ClusterIssuer",
			"--set", "certs.issuerRef.name=default-ca",
			"--set", "certs.caBundle.configMap=ca-bundle",
			"--set", "auth.issuerUrl=https://keycloak.keycloak.svc.cluster.local:8001/realms/innabox",
			"--wait",
		).
		Build()
	Expect(err).ToNot(HaveOccurred())
	err = installCmd.Execute(ctx)
	Expect(err).ToNot(HaveOccurred())
	if config.KeepKind && !config.KeepService {
		DeferCleanup(func() {
			logger.InfoContext(ctx, "Uninstalling service")
			uninstallCmd, err := NewCommand().
				SetLogger(logger).
				SetDir(projectDir).
				SetName(helmCmd).
				SetArgs(
					"uninstall",
					"fulfillment-service",
					"--kubeconfig", kcFile,
					"--namespace", "innabox",
				).
				Build()
			Expect(err).ToNot(HaveOccurred())
			err = uninstallCmd.Execute(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	}

	// Helper function to create a token source from a service account:
	makeTokenSource := func(sa string) auth.TokenSource {
		response, err := kubeClientSet.CoreV1().ServiceAccounts("innabox").CreateToken(
			ctx,
			sa,
			&authenticationv1.TokenRequest{
				Spec: authenticationv1.TokenRequestSpec{
					ExpirationSeconds: ptr.To(int64(3600)),
				},
			},
			metav1.CreateOptions{},
		)
		Expect(err).ToNot(HaveOccurred())
		token := &auth.Token{
			Access: response.Status.Token,
		}
		source, err := auth.NewStaticTokenSource().
			SetLogger(logger).
			SetToken(token).
			Build()
		Expect(err).ToNot(HaveOccurred())
		return source
	}

	// Create the token sources:
	clientTokenSource := makeTokenSource("client")
	adminTokenSource := makeTokenSource("admin")

	// Create the gRPC clients:
	makeConn := func(tokenSource auth.TokenSource) *grpc.ClientConn {
		conn, err := network.NewGrpcClient().
			SetLogger(logger).
			SetCaPool(caPool).
			SetAddress("localhost:8000").
			SetTokenSource(tokenSource).
			Build()
		Expect(err).ToNot(HaveOccurred())
		return conn
	}
	clientConn = makeConn(clientTokenSource)
	adminConn = makeConn(adminTokenSource)

	// Helper function to create an HTTP client from a token source:
	makeClient := func(tokenSource auth.TokenSource) *http.Client {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caPool,
			},
		}
		return &http.Client{
			Transport: ghttp.RoundTripperFunc(
				func(request *http.Request) (response *http.Response, err error) {
					token, err := tokenSource.Token(request.Context())
					Expect(err).ToNot(HaveOccurred())
					request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.Access))
					response, err = transport.RoundTrip(request)
					return
				},
			),
		}
	}
	userClient = makeClient(clientTokenSource)
	adminClient = makeClient(adminTokenSource)

	// Wait till the application is healthy:
	healthClient := healthv1.NewHealthClient(adminConn)
	healthRequest := &healthv1.HealthCheckRequest{}
	Eventually(
		func(g Gomega) {
			healthResponse, err := healthClient.Check(ctx, healthRequest)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(healthResponse.Status).To(Equal(healthv1.HealthCheckResponse_SERVING))
		},
		time.Minute,
		5*time.Second,
	).Should(Succeed())

	// Create the namespace for the hub:
	hubNamespaceObject := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: hubNamespace,
		},
	}
	err = kubeClient.Create(ctx, hubNamespaceObject)
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).ToNot(HaveOccurred())
	}

	// Register the kind cluster as a hub. Note that in order to do this we need to replace the 127.0.0.1 IP
	// with the internal DNS name of the API server, as otherwise the controller will not be able to connect.
	hubKubeconfigBytes := cluster.Kubeconfig()
	hubKubeconfigObject, err := clientcmd.Load(hubKubeconfigBytes)
	Expect(err).ToNot(HaveOccurred())
	for clusterKey := range hubKubeconfigObject.Clusters {
		hubKubeconfigObject.Clusters[clusterKey].Server = "https://kubernetes.default.svc"
	}
	hubKubeconfigBytes, err = clientcmd.Write(*hubKubeconfigObject)
	Expect(err).ToNot(HaveOccurred())
	hubsClient := privatev1.NewHubsClient(adminConn)
	_, err = hubsClient.Create(ctx, privatev1.HubsCreateRequest_builder{
		Object: privatev1.Hub_builder{
			Id:         hubId,
			Kubeconfig: hubKubeconfigBytes,
			Namespace:  hubNamespace,
		}.Build(),
	}.Build())
	Expect(err).ToNot(HaveOccurred())
})

var _ = Describe("Integration", func() {
	It("Setup", Label("setup"), func() {
		// This is a dummy test to have a mechanism to run the setup of the integration tests without running
		// any actual tests, with a command like this:
		//
		// ginkgo run --label-filter setup it
		//
		// This will create the kind cluster, install the dependencies, and deploy the application.
	})
})

// Names of the command line tools:
const (
	helmCmd    = "helm"
	kubectlCmd = "kubectl"
	podmanCmd  = "podman"
)

// Name and namespace of the hub:
const hubId = "local"
const hubNamespace = "cloudkit-operator-system"

// Image details:
const imageName = "ghcr.io/innabox/fulfillment-service"
