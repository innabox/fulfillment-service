/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// KindBuilder contains the data and logic needed to create an object that helps manage a Kind cluster used for
// integration tests. Don't create instances of this type directly, use the NewKind function instead.
type KindBuilder struct {
	logger   *slog.Logger
	name     string
	crdFiles []string
}

// Kind helps manage a Kind cluster used for integration tests. Don't create instances of this type directly, use the
// NewKind function instead.
type Kind struct {
	logger          *slog.Logger
	name            string
	crdFiles        []string
	kubeconfigBytes []byte
	kubeconfigFile  string
	kubeClient      crclient.WithWatch
	kubeClientSet   *kubernetes.Clientset
}

// NewKind creates a builder that can then be used to configure and create a new Kind cluster used for integration
// tests.
func NewKind() *KindBuilder {
	return &KindBuilder{}
}

// SetLogger sets the logger. This is mandatory.
func (b *KindBuilder) SetLogger(value *slog.Logger) *KindBuilder {
	b.logger = value
	return b
}

// SetName sets the name of the Kind cluster. This is mandatory.
func (b *KindBuilder) SetName(name string) *KindBuilder {
	b.name = name
	return b
}

// AddCrdFile adds a file containing custom resource definition to be installed in the cluster.
func (b *KindBuilder) AddCrdFile(file string) *KindBuilder {
	b.crdFiles = append(b.crdFiles, file)
	return b
}

// Build uses the configuration stored in the builder to create a new Kind cluster
func (b *KindBuilder) Build() (result *Kind, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}
	if b.name == "" {
		err = fmt.Errorf("name is mandatory")
		return
	}

	// Add the name to the logger:
	logger := b.logger.With(
		slog.String("kind", b.name),
	)

	// Check that the command line tools that we need are available:
	_, err = exec.LookPath(helmCmd)
	if err != nil {
		err = fmt.Errorf("command line tool '%s' isn't available: %w", helmCmd, err)
		return
	}
	_, err = exec.LookPath(kindCmd)
	if err != nil {
		err = fmt.Errorf("command line tool '%s' isn't available: %w", kindCmd, err)
		return
	}
	_, err = exec.LookPath(kubectlCmd)
	if err != nil {
		err = fmt.Errorf("command line tool '%s' isn't available: %w", kubectlCmd, err)
		return
	}

	// Create and populate the object:
	result = &Kind{
		logger:   logger,
		name:     b.name,
		crdFiles: slices.Clone(b.crdFiles),
	}
	return
}

// Start makes sure that the cluster is created and ready to use. It will remove the cluster if it already exists and
// will create a new one if it doesn't exist.
func (k *Kind) Start(ctx context.Context) error {
	// Check if the cluster already exists. If it does, delete it. Then create a new one.
	names, err := k.getClusters(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing clusters: %w", err)
	}
	if slices.Contains(names, k.name) {
		k.logger.LogAttrs(
			ctx,
			slog.LevelDebug,
			"Deleting existing kind cluster",
		)
		err = k.deleteCluster(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete existing kind cluster '%s': %w", k.name, err)
		}
	}
	k.logger.LogAttrs(
		ctx,
		slog.LevelDebug,
		"Creating new kind cluster",
	)
	err = k.createCluster(ctx)
	if err != nil {
		return fmt.Errorf("failed to create kind cluster '%s': %w", k.name, err)
	}
	k.logger.LogAttrs(
		ctx,
		slog.LevelDebug,
		"Created new kind cluster",
	)

	// Create the kubeconfig and the Kubernetes client:
	err = k.createKubeconfig(ctx)
	if err != nil {
		return err
	}
	err = k.createKubeClient(ctx)
	if err != nil {
		return err
	}
	err = k.createKubeClientSet(ctx)
	if err != nil {
		return err
	}

	// Install custom resource definitions:
	err = k.installCrdFiles(ctx)
	if err != nil {
		return err
	}

	// Install other components in parallel:
	wg := sync.WaitGroup{}
	wg.Add(2)
	errors := &atomic.Int32{}
	go func() {
		defer wg.Done()
		err := k.installCertManager(ctx)
		if err != nil {
			k.logger.ErrorContext(
				ctx,
				"Failed to install cert-manager",
				slog.Any("error", err),
			)
			errors.Add(1)
		}
	}()
	go func() {
		defer wg.Done()
		err := k.installAuthorino(ctx)
		if err != nil {
			k.logger.ErrorContext(
				ctx,
				"Failed to install authorino",
				slog.Any("error", err),
			)
			errors.Add(1)
		}
	}()
	wg.Wait()
	if errors.Load() > 0 {
		return fmt.Errorf("failed to install some components, see the logs for details")
	}

	return nil
}

// Stop removes the Kind cluster.
func (k *Kind) Stop(ctx context.Context) error {
	k.logger.DebugContext(ctx, "Stopping kind cluster")
	var errs []error
	err := k.deleteCluster(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to delete kind cluster '%s': %w", k.name, err))
	}
	err = os.Remove(k.kubeconfigFile)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to remove kubeconfig file '%s': %w", k.kubeconfigFile, err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to stop kind cluster '%s': %v", k.name, errs)
	}
	k.logger.DebugContext(ctx, "Stopped kind cluster")
	return nil
}

// LoadImage loads the specified image into the Kind cluster.
//
// Note that this will take the image from the local Docker daemon, regardless of what provider was used to create the
// cluster. For example, if the cluster was created with the Podman provider this wills still load the image from
// Docker.
func (k *Kind) LoadImage(ctx context.Context, image string) error {
	if image == "" {
		return fmt.Errorf("image is mandatory")
	}
	logger := k.logger.With(
		slog.String("image", image),
	)
	logger.DebugContext(ctx, "Loading image into kind cluster")
	loadCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kindCmd).
		SetArgs("load", "docker-image", "--name", k.name, image).
		Build()
	if err != nil {
		return fmt.Errorf(
			"failed to create command to load image '%s' into kind cluster '%s': %w",
			image, k.name, err,
		)
	}
	err = loadCmd.Execute(ctx)
	if err != nil {
		return fmt.Errorf(
			"failed to load image '%s' into kind cluster '%s': %w",
			image, k.name, err,
		)
	}
	logger.DebugContext(ctx, "Loaded image into kind cluster")
	return nil
}

// LoadArchive loads the specified image archive into the Kind cluster.
func (k *Kind) LoadArchive(ctx context.Context, archive string) error {
	if archive == "" {
		return fmt.Errorf("archive is mandatory")
	}
	logger := k.logger.With(
		slog.String("archive", archive),
	)
	logger.DebugContext(ctx, "Loading image archive into kind cluster")
	loadCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kindCmd).
		SetArgs("load", "image-archive", "--name", k.name, archive).
		Build()
	if err != nil {
		return fmt.Errorf(
			"failed to create command to load image archive '%s' into kind cluster '%s': %w",
			archive, k.name, err,
		)
	}
	err = loadCmd.Execute(ctx)
	if err != nil {
		return fmt.Errorf(
			"failed to load image archive '%s' into kind cluster '%s': %w",
			archive, k.name, err,
		)
	}
	logger.DebugContext(ctx, "Loaded image archive into kind cluster")
	return nil
}

// Kubeconfig returns the kubeconfig bytes.
func (k *Kind) Kubeconfig() []byte {
	return slices.Clone(k.kubeconfigBytes)
}

// Client returns the controller runtime client for the cluster.
func (k *Kind) Client() crclient.WithWatch {
	return k.kubeClient
}

// ClientSet returns the Kubernetes set client for the cluster.
func (k *Kind) ClientSet() *kubernetes.Clientset {
	return k.kubeClientSet
}

func (k *Kind) getClusters(ctx context.Context) (result []string, err error) {
	getCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kindCmd).
		SetArgs("get", "clusters").
		Build()
	if err != nil {
		err = fmt.Errorf(
			"failed to create command to get existing kind clusters: %w",
			err,
		)
		return
	}
	getOut, _, err := getCmd.Evaluate(ctx)
	if err != nil {
		err = fmt.Errorf(
			"failed to get existing kind clusters: %w",
			err,
		)
		return
	}
	names := strings.Split(string(getOut), "\n")
	for len(names) > 0 && names[len(names)-1] == "" {
		names = names[:len(names)-1]
	}
	result = names
	return
}

func (k *Kind) createCluster(ctx context.Context) error {
	// Create a temporary directory to store the configuration file for the kind cluster:
	tmpDir, err := os.MkdirTemp("", "*.kind")
	if err != nil {
		return fmt.Errorf(
			"failed to create temporary directory for configuration of kind cluster '%s': %w",
			k.name, err,
		)
	}
	defer func() {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			k.logger.LogAttrs(
				ctx,
				slog.LevelError,
				"Failed to remove temporary directory for configuration of kind cluster",
				slog.String("tmp", tmpDir),
				slog.Any("error", err),
			)
		}
	}()
	configData := map[string]any{
		"apiVersion": "kind.x-k8s.io/v1alpha4",
		"kind":       "Cluster",
		"name":       k.name,
		"nodes": []any{
			map[string]any{
				"role": "control-plane",
				"extraPortMappings": []any{
					map[string]any{
						"containerPort": 30000,
						"hostPort":      8000,
						"listenAddress": "0.0.0.0",
					},
					map[string]any{
						"containerPort": 30001,
						"hostPort":      8001,
						"listenAddress": "0.0.0.0",
					},
				},
			},
		},
	}
	configBytes, err := json.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration for kind cluster '%s': %w", k.name, err)
	}
	configFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(configFile, configBytes, 0644)
	if err != nil {
		return fmt.Errorf(
			"failed to write configuration for kind cluster '%s' to file '%s': %w",
			k.name, configFile, err,
		)
	}
	k.logger.LogAttrs(
		ctx,
		slog.LevelDebug,
		"Creating kind cluster",
		slog.Any("config", configData),
	)

	// Create the cluster:
	createCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kindCmd).
		SetArgs("create", "cluster", "--name", k.name, "--config", configFile).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create command to create kind cluster '%s': %w", k.name, err)
	}
	err = createCmd.Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to create kind cluster '%s': %w", k.name, err)
	}
	k.logger.DebugContext(ctx, "Created kind cluster")
	return nil
}

func (k *Kind) deleteCluster(ctx context.Context) error {
	k.logger.DebugContext(ctx, "Deleting kind cluster")
	deleteCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kindCmd).
		SetArgs("delete", "cluster", "--name", k.name).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create command to delete kind cluster '%s': %w", k.name, err)
	}
	err = deleteCmd.Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete kind cluster '%s': %w", k.name, err)
	}
	k.logger.DebugContext(ctx, "Deleted kind cluster")
	return err
}

func (k *Kind) createKubeconfig(ctx context.Context) error {
	k.logger.DebugContext(ctx, "Creating kubeconfig")
	getCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kindCmd).
		SetArgs("get", "kubeconfig", "--name", k.name).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create command to get kubeconfig for kind cluster '%s': %w", k.name, err)
	}
	k.kubeconfigBytes, _, err = getCmd.Evaluate(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig for kind cluster '%s': %w", k.name, err)
	}

	err = func() error {
		tmpFile, err := os.CreateTemp("", "*.kubeconfig")
		if err != nil {
			return fmt.Errorf("failed to create kubeconfig file for kind cluster '%s': %w", k.name, err)
		}
		defer func() {
			err := tmpFile.Close()
			if err != nil {
				k.logger.LogAttrs(
					ctx,
					slog.LevelError,
					"Failed to close kubeconfig file",
					slog.String("file", tmpFile.Name()),
					slog.Any("error", err),
				)
			}
		}()
		_, err = tmpFile.Write(k.kubeconfigBytes)
		if err != nil {
			return fmt.Errorf(
				"failed to write kubeconfig file '%s' for kind cluster '%s': %w",
				tmpFile.Name(), k.name, err,
			)
		}
		k.kubeconfigFile = tmpFile.Name()
		return nil
	}()
	if err != nil {
		return err
	}
	k.logger.LogAttrs(
		ctx,
		slog.LevelDebug,
		"Created kubeconfig",
		slog.String("file", k.kubeconfigFile),
		slog.Int("size", len(k.kubeconfigBytes)),
	)
	return nil
}

func (k *Kind) createKubeClient(ctx context.Context) error {
	k.logger.DebugContext(ctx, "Creating Kubernetes client")
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(k.kubeconfigBytes)
	if err != nil {
		return fmt.Errorf("failed to create REST config for kind cluster '%s': %w", k.name, err)
	}
	k.kubeClient, err = crclient.NewWithWatch(restConfig, crclient.Options{})
	if err != nil {
		return fmt.Errorf("failed to create client for kind cluster '%s': %w", k.name, err)
	}
	k.logger.DebugContext(ctx, "Created Kubernetes client")
	return nil
}

func (k *Kind) createKubeClientSet(ctx context.Context) error {
	k.logger.DebugContext(ctx, "Creating Kubernetes client set")
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(k.kubeconfigBytes)
	if err != nil {
		return fmt.Errorf("failed to create REST config for kind cluster '%s': %w", k.name, err)
	}
	k.kubeClientSet, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create client set for kind cluster '%s': %w", k.name, err)
	}
	k.logger.DebugContext(ctx, "Created Kubernetes client")
	return nil
}

func (k *Kind) installCertManager(ctx context.Context) (err error) {
	// Apply the cert-manager manifest:
	k.logger.DebugContext(ctx, "Applying cert-manager manifests")
	applyCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kubectlCmd).
		SetArgs(
			"apply",
			"--kubeconfig", k.kubeconfigFile,
			"--filename", certManagerManifests,
		).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create command to apply cert-manager manifests: %w", err)
	}
	err = applyCmd.Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to apply cert-manager manifests: %w", err)
	}
	k.logger.DebugContext(ctx, "Applied cert-manager manifests")

	// Wait till it is possible to create cert-manager objects. Any other way to check this has proven to be
	// unreliable. The fact that the apply worked doesn't mean that the CRDs are established already, and the fact
	// that the CRDs are established doesn't mean that the webhooks are already running.
	k.logger.DebugContext(ctx, "Waiting for cert-manager to be ready")
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Issuer",
	})
	object.SetNamespace("default")
	object.SetName("default")
	object.Object["spec"] = map[string]any{
		"selfSigned": map[string]any{},
	}
	start := time.Now()
	for {
		err = k.kubeClient.Create(ctx, object, crclient.DryRunAll)
		if err == nil {
			break
		}
		k.logger.LogAttrs(
			ctx,
			slog.LevelDebug,
			"Cert-manager not ready yet",
			slog.Any("error", err),
		)
		if time.Since(start) > time.Minute {
			return fmt.Errorf("failed to create cert-manager object after 1 minute: %w", err)
		}
		time.Sleep(5 * time.Second)
	}
	k.logger.DebugContext(ctx, "Cert-manager is ready")

	return nil
}

func (k *Kind) installAuthorino(ctx context.Context) (err error) {
	// Apply the authorino manifest:
	k.logger.DebugContext(ctx, "Applying authorino manifests")
	applyCmd, err := NewCommand().
		SetLogger(k.logger).
		SetName(kubectlCmd).
		SetArgs(
			"apply",
			"--kubeconfig", k.kubeconfigFile,
			"--filename", authorinoManifests,
		).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create command to apply authorino manifests: %w", err)
	}
	err = applyCmd.Execute(ctx)
	if err != nil {
		return fmt.Errorf("failed to apply authorino manifests: %w", err)
	}
	k.logger.DebugContext(ctx, "Applied authorino manifests")

	// Wait till it is possible to create authorino objects. Any other way to check this has proven to be
	// unreliable. The fact that the apply worked doesn't mean that the CRDs are established already, and the fact
	// that the CRDs are established doesn't mean that the webhooks are already running.
	k.logger.DebugContext(ctx, "Waiting for authorino to be ready")
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.authorino.kuadrant.io",
		Version: "v1beta1",
		Kind:    "Authorino",
	})
	object.SetNamespace("default")
	object.SetName("default")
	object.Object["spec"] = map[string]any{
		"logLevel": "debug",
		"listener": map[string]any{
			"tls": map[string]any{
				"enabled": false,
			},
		},
		"oidcServer": map[string]any{
			"tls": map[string]any{
				"enabled": false,
			},
		},
	}
	start := time.Now()
	for {
		err = k.kubeClient.Create(ctx, object, crclient.DryRunAll)
		if err == nil {
			break
		}
		k.logger.LogAttrs(
			ctx,
			slog.LevelDebug,
			"Authorino not ready yet",
			slog.Any("error", err),
		)
		if time.Since(start) > time.Minute {
			return fmt.Errorf("failed to create authorino object after 1 minute: %w", err)
		}
		time.Sleep(5 * time.Second)
	}
	k.logger.DebugContext(ctx, "Authorino is ready")

	return nil
}

func (k *Kind) installCrdFiles(ctx context.Context) error {
	for _, crdFile := range k.crdFiles {
		logger := k.logger.With(slog.String("file", crdFile))
		logger.DebugContext(ctx, "Applying CRD")
		applyCmd, err := NewCommand().
			SetLogger(k.logger).
			SetName(kubectlCmd).
			SetArgs(
				"apply",
				"--kubeconfig", k.kubeconfigFile,
				"--filename", crdFile,
			).
			Build()
		if err != nil {
			return err
		}
		err = applyCmd.Execute(ctx)
		if err != nil {
			return err
		}
		logger.DebugContext(ctx, "Applied CRD")
	}
	return nil
}

// Names of commands:
const (
	helmCmd    = "helm"
	kindCmd    = "kind"
	kubectlCmd = "kubectl"
)

// Location of manifests:
const (
	certManagerManifests = "https://github.com/cert-manager/cert-manager/releases/download/v1.17.2/cert-manager.yaml"
	authorinoManifests   = "https://raw.githubusercontent.com/Kuadrant/authorino-operator/refs/heads/release-v0.20.0/config/deploy/manifests.yaml"
)
