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
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/innabox/fulfillment-service/internal/logging"
	"github.com/innabox/fulfillment-service/internal/network"
	. "github.com/innabox/fulfillment-service/internal/testing"
)

var (
	logger     *slog.Logger
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

	// Configure the Kubernetes libraries to use our logger:
	logrLogger := logr.FromSlogHandler(logger.Handler())
	crlog.SetLogger(logrLogger)
	klog.SetLogger(logrLogger)

	// Ensure that the kind cluster is ready:
	kind, err = NewKind().
		SetLogger(logger).
		SetName("it").
		Build()
	Expect(err).ToNot(HaveOccurred())
	err = kind.Start(ctx)
	Expect(err).ToNot(HaveOccurred())
	DeferCleanup(func() {
		err := kind.Stop(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

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

	// Get the kubeconfig:
	kcFile := filepath.Join(tmpDir, "kubeconfig")
	err = os.WriteFile(kcFile, kind.Kubeconfig(), 0400)
	Expect(err).ToNot(HaveOccurred())

	// Get the client:
	kubeClient := kind.Client()
	kubeClientSet := kind.ClientSet()

	// In the GitHub actions environment, the image is already built and available in the 'imag.tar' file in the
	// project directory. If it is not there, we build it and save it to the temporary directory.
	imageTar := filepath.Join(projectDir, "image.tar")
	_, err = os.Stat(imageTar)
	if err != nil {
		imageTar = filepath.Join(tmpDir, "image.tar")
		DeferCleanup(func() {
			err := os.Remove(imageTar)
			Expect(err).ToNot(HaveOccurred())
		})
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
	}

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
})

// Names of the command line tools:
const (
	kubectlPath = "kubectl"
	kindPath    = "podman"
)

// Image details:
const imageName = "ghcr.io/innabox/fulfillment-service"
const imageTag = "latest"

var imageRef = fmt.Sprintf("%s:%s", imageName, imageTag)
