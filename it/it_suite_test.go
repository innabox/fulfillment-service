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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kelseyhightower/envconfig"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/logging"
	"github.com/innabox/fulfillment-service/internal/network"
	. "github.com/innabox/fulfillment-service/internal/testing"
)

// Config contains configuration options for the integration tests.
type Config struct {
	// KeepKind indicates whether to preserve the kind cluster after tests complete.
	// By default, the kind cluster is deleted after running the tests.
	KeepKind bool `json:"keep_kind" envconfig:"keep_kind" default:"false"`
}

var (
	logger     *slog.Logger
	config     *Config
	kind       *Kind
	clientConn *grpc.ClientConn
	adminConn  *grpc.ClientConn
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
	_, err = exec.LookPath(kubectlPath)
	Expect(err).ToNot(HaveOccurred())
	_, err = exec.LookPath(kindPath)
	Expect(err).ToNot(HaveOccurred())

	// We will create the kind cluster and build the container image in parallel, and will use this
	// to wait for both to complete.
	var wg sync.WaitGroup

	// Start the kind cluster:
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer GinkgoRecover()

		// Create the kind cluster, and start it:
		var err error
		kind, err = NewKind().
			SetLogger(logger).
			SetName("it").
			AddCrdFile(filepath.Join("crds", "clusterorders.cloudkit.openshift.io.yaml")).
			AddCrdFile(filepath.Join("crds", "hostedclusters.hypershift.openshift.io.yaml")).
			Build()
		Expect(err).ToNot(HaveOccurred())
		err = kind.Start(ctx)
		Expect(err).ToNot(HaveOccurred())

		// Remember to stop the kind cluster:
		if !config.KeepKind {
			DeferCleanup(func() {
				err := kind.Stop(ctx)
				Expect(err).ToNot(HaveOccurred())
			})
		}
	}()

	// In the GitHub actions environment, the image is already built and available in the 'image.tar'
	// file in the project directory. If it is not there, we build it and save it to the temporary
	// directory.
	imageTar := filepath.Join(projectDir, "image.tar")
	_, err = os.Stat(imageTar)
	if err != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer GinkgoRecover()

			// Build the container image:
			buildCmd, err := NewCommand().
				SetLogger(logger).
				SetDir(projectDir).
				SetName("podman").
				SetArgs(
					"build",
					"--tag", fmt.Sprintf("%s:%s", imageName, imageTag),
					"--file", "Containerfile",
				).
				Build()
			Expect(err).ToNot(HaveOccurred())
			err = buildCmd.Execute(ctx)
			Expect(err).ToNot(HaveOccurred())

			// Save the container image to the tar file:
			imageTar = filepath.Join(tmpDir, "image.tar")
			saveCmd, err := NewCommand().
				SetLogger(logger).
				SetDir(projectDir).
				SetName("podman").
				SetArgs(
					"save",
					"--output", imageTar,
					imageRef,
				).
				Build()
			Expect(err).ToNot(HaveOccurred())
			err = saveCmd.Execute(ctx)
			Expect(err).ToNot(HaveOccurred())

			// Clean up the container image tar file:
			DeferCleanup(func() {
				err := os.Remove(imageTar)
				Expect(err).ToNot(HaveOccurred())
			})
		}()
	}

	// Wait for the kind cluster to be started and the container image to be built:
	wg.Wait()

	// Get the kubeconfig:
	kcFile := filepath.Join(tmpDir, "kubeconfig")
	err = os.WriteFile(kcFile, kind.Kubeconfig(), 0400)
	Expect(err).ToNot(HaveOccurred())

	// Get the client:
	kubeClient := kind.Client()
	kubeClientSet := kind.ClientSet()

	// Load the image:
	err = kind.LoadArchive(ctx, imageTar)
	Expect(err).ToNot(HaveOccurred())

	// Deploy the application:
	applyCmd, err := NewCommand().
		SetLogger(logger).
		SetDir(projectDir).
		SetName(kubectlPath).
		SetArgs(
			"apply",
			"--kubeconfig", kcFile,
			"--kustomize", filepath.Join("manifests", "overlays", "kind"),
		).
		Build()
	Expect(err).ToNot(HaveOccurred())
	err = applyCmd.Execute(ctx)
	Expect(err).ToNot(HaveOccurred())

	// Wait till the CA certificate has been issued, and fetch it. We need it to configure gRPC and HTTP clients
	// so that they trust it.
	caKey := crclient.ObjectKey{
		Namespace: "innabox",
		Name:      "ca-key",
	}
	caSecret := &corev1.Secret{}
	Eventually(
		func(g Gomega) {
			err := kubeClient.Get(ctx, caKey, caSecret)
			g.Expect(err).ToNot(HaveOccurred())
		},
		time.Minute,
		time.Second,
	).Should(Succeed())
	caBytes := caSecret.Data["ca.crt"]
	Expect(caBytes).ToNot(BeEmpty())
	caFile := filepath.Join(tmpDir, "ca.crt")
	err = os.WriteFile(caFile, caBytes, 0400)
	Expect(err).ToNot(HaveOccurred())

	// Create a client token:
	makeToken := func(sa string) string {
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
		return response.Status.Token
	}
	clientToken := makeToken("client")
	adminToken := makeToken("admin")

	// Create the gRPC clients:
	makeConn := func(token string) *grpc.ClientConn {
		conn, err := network.NewClient().
			SetLogger(logger).
			SetServerNetwork("tcp").
			SetServerAddress("localhost:8000").
			AddCaFile(caFile).
			SetToken(token).
			Build()
		Expect(err).ToNot(HaveOccurred())
		return conn
	}
	clientConn = makeConn(clientToken)
	adminConn = makeConn(adminToken)

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
	Expect(err).ToNot(HaveOccurred())

	// Register the kind cluster as a hub. Note that in order to do this we need to replace the 127.0.0.1 IP
	// with the internal DNS name of the API server, as otherwise the controller will not be able to connect.
	hubKubeconfigBytes := kind.Kubeconfig()
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

// Names of the command line tools:
const (
	kubectlPath = "kubectl"
	kindPath    = "podman"
)

// Name and namespace of the hub:
const hubId = "local"
const hubNamespace = "cloudkit-operator-system"

// Image details:
const imageName = "ghcr.io/innabox/fulfillment-service"
const imageTag = "latest"

var imageRef = fmt.Sprintf("%s:%s", imageName, imageTag)
