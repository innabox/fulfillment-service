/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package bcm

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/stmcginnis/gofish"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
)

// SynchronizerBuilder is used to build synchronizers using the builder pattern.
type SynchronizerBuilder struct {
	logger    *slog.Logger
	bcmClient Client
	grpcConn  *grpc.ClientConn
	interval  time.Duration
}

// Synchronizer periodically synchronizes inventory from BCM to the fulfillment service.
type Synchronizer struct {
	logger            *slog.Logger
	bcmClient         Client
	hostsClient       privatev1.HostsClient
	hostClassesClient privatev1.HostClassesClient
	interval          time.Duration
}

// synchronizerTask contains the data and logic needed for an individual synchronization cycle.
type synchronizerTask struct {
	logger            *slog.Logger
	bcmClient         Client
	hostsClient       privatev1.HostsClient
	hostClassesClient privatev1.HostClassesClient
	racksByUuid       map[string]*Rack
	networksByUuid    map[string]*Network
	categoriesByUuid  map[string]*Category
	devicesByUuid     map[string]*Device
}

// NewSynchronizer creates a new synchronizer builder.
func NewSynchronizer() *SynchronizerBuilder {
	return &SynchronizerBuilder{}
}

// SetLogger sets the logger for the synchronizer.
func (b *SynchronizerBuilder) SetLogger(value *slog.Logger) *SynchronizerBuilder {
	b.logger = value
	return b
}

// SetBcmClient sets the BCM client.
func (b *SynchronizerBuilder) SetBcmClient(value Client) *SynchronizerBuilder {
	b.bcmClient = value
	return b
}

// SetGrpcClient sets the gRPC connection to the fulfillment service.
func (b *SynchronizerBuilder) SetGrpcClient(value *grpc.ClientConn) *SynchronizerBuilder {
	b.grpcConn = value
	return b
}

// SetInterval sets the synchronization interval.
func (b *SynchronizerBuilder) SetInterval(value time.Duration) *SynchronizerBuilder {
	b.interval = value
	return b
}

// Build creates the synchronizer.
func (b *SynchronizerBuilder) Build() (result *Synchronizer, err error) {
	// Check required parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}
	if b.bcmClient == nil {
		err = fmt.Errorf("BCM client is mandatory")
		return
	}
	if b.grpcConn == nil {
		err = fmt.Errorf("gRPC client is mandatory")
		return
	}

	// Set defaults:
	interval := b.interval
	if interval == 0 {
		interval = 10 * time.Second
	}

	// Create and populate the object:
	result = &Synchronizer{
		logger:            b.logger,
		bcmClient:         b.bcmClient,
		hostsClient:       privatev1.NewHostsClient(b.grpcConn),
		hostClassesClient: privatev1.NewHostClassesClient(b.grpcConn),
		interval:          interval,
	}
	return
}

// Run starts the synchronization loop.
func (s *Synchronizer) Run(ctx context.Context) error {
	s.logger.InfoContext(
		ctx,
		"Starting inventory synchronization",
		slog.Duration("interval", s.interval),
	)

	// Create a ticker for periodic synchronization:
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run the first synchronization immediately:
	err := s.synchronize(ctx)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Initial synchronization failed",
			slog.Any("error", err),
		)
		// Continue despite the error
	}

	// Run periodic synchronization:
	for {
		select {
		case <-ctx.Done():
			s.logger.InfoContext(ctx, "Stopping inventory synchronization")
			return ctx.Err()
		case <-ticker.C:
			err := s.synchronize(ctx)
			if err != nil {
				s.logger.ErrorContext(
					ctx,
					"Synchronization failed",
					slog.Any("error", err),
				)
				// Continue despite the error
			}
		}
	}
}

func (s *Synchronizer) synchronize(ctx context.Context) error {
	task := &synchronizerTask{
		logger:            s.logger,
		bcmClient:         s.bcmClient,
		hostsClient:       s.hostsClient,
		hostClassesClient: s.hostClassesClient,
		racksByUuid:       map[string]*Rack{},
		networksByUuid:    map[string]*Network{},
		categoriesByUuid:  map[string]*Category{},
		devicesByUuid:     map[string]*Device{},
	}
	return task.run(ctx)
}

func (t *synchronizerTask) run(ctx context.Context) error {
	// Take note of the start time:
	start := time.Now()
	t.logger.InfoContext(
		ctx,
		"Starting synchronization cycle",
	)

	// Load networks, racks, categories, and devices:
	t.loadNetworks(ctx)
	t.loadRacks(ctx)
	t.loadCategories(ctx)
	t.loadDevices(ctx)

	// Synchronize categories as host classes:
	err := t.synchronizeCategories(ctx)
	if err != nil {
		t.logger.WarnContext(
			ctx,
			"Failed to synchronize categories",
			slog.Any("error", err),
		)
	}

	// Synchronize devices as hosts:
	err = t.synchronizeDevices(ctx)
	if err != nil {
		t.logger.WarnContext(
			ctx,
			"Failed to synchronize devices",
			slog.Any("error", err),
		)
	}

	// Log the results:
	duration := time.Since(start)
	t.logger.InfoContext(
		ctx,
		"Synchronization cycle completed",
		slog.Int("categories", len(t.categoriesByUuid)),
		slog.Int("devices", len(t.devicesByUuid)),
		slog.Duration("duration", duration),
	)

	return nil
}

func (t *synchronizerTask) loadRacks(ctx context.Context) {
	racks, err := t.bcmClient.GetRacks(ctx)
	if err != nil {
		t.logger.WarnContext(
			ctx,
			"Failed to get racks from BCM",
			slog.Any("error", err),
		)
		return
	}
	for _, rack := range racks {
		t.racksByUuid[rack.Uuid] = rack
	}
	t.logger.InfoContext(
		ctx,
		"Retrieved racks from BCM",
		slog.Int("count", len(racks)),
	)
}

func (t *synchronizerTask) loadNetworks(ctx context.Context) {
	networks, err := t.bcmClient.GetNetworks(ctx)
	if err != nil {
		t.logger.WarnContext(
			ctx,
			"Failed to get networks from BCM",
			slog.Any("error", err),
		)
		return
	}
	for _, network := range networks {
		t.networksByUuid[network.Uuid] = network
	}
	t.logger.InfoContext(
		ctx,
		"Retrieved networks from BCM",
		slog.Int("count", len(networks)),
	)
}

func (t *synchronizerTask) loadCategories(ctx context.Context) {
	categories, err := t.bcmClient.GetCategories(ctx)
	if err != nil {
		t.logger.WarnContext(
			ctx,
			"Failed to get categories from BCM",
			slog.Any("error", err),
		)
		return
	}
	for _, category := range categories {
		t.categoriesByUuid[category.Uuid] = category
	}
	t.logger.InfoContext(
		ctx,
		"Retrieved categories from BCM",
		slog.Int("count", len(categories)),
	)
}

func (t *synchronizerTask) loadDevices(ctx context.Context) {
	devices, err := t.bcmClient.GetDevices(ctx)
	if err != nil {
		t.logger.WarnContext(
			ctx,
			"Failed to get devices from BCM",
			slog.Any("error", err),
		)
		return
	}
	for _, device := range devices {
		t.devicesByUuid[device.Uuid] = device
	}
	t.logger.InfoContext(
		ctx,
		"Retrieved devices from BCM",
		slog.Int("count", len(devices)),
	)
}

func (t *synchronizerTask) synchronizeDevices(ctx context.Context) error {
	for _, device := range t.devicesByUuid {
		if device.ChildType != "PhysicalNode" {
			continue
		}
		err := t.synchronizeDevice(ctx, device)
		if err != nil {
			t.logger.WarnContext(
				ctx,
				"Failed to synchronize device",
				slog.String("hostname", device.Hostname),
				slog.Any("error", err),
			)
		}
	}
	return nil
}

func (t *synchronizerTask) synchronizeDevice(ctx context.Context, device *Device) error {
	host, err := t.deviceToHost(ctx, device)
	if err != nil {
		return fmt.Errorf("failed to convert device to host: %w", err)
	}
	err = t.createOrUpdateHost(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to create or update host: %w", err)
	}
	t.logger.DebugContext(
		ctx,
		"Synchronized device",
		slog.String("hostname", device.Hostname),
	)
	return nil
}

// buildRedfishUrl connects to a Redfish server, finds the system ID matching the device name, and builds a virtual
// media URL.
func (t *synchronizerTask) buildRedfishUrl(ctx context.Context, address, username, password, device string) (result string,
	err error) {
	// Connect to the server. Yes, using insecure mode because BCM doesn't contain the CA certificates to use.
	config := gofish.ClientConfig{
		Endpoint:  fmt.Sprintf("https://%s", address),
		Username:  username,
		Password:  password,
		BasicAuth: true,
		Insecure:  true,
	}
	client, err := gofish.ConnectContext(ctx, config)
	if err != nil {
		err = fmt.Errorf("failed to connect to Redfish server: %w", err)
		return
	}
	defer client.Logout()

	// Get the systems:
	systems, err := client.Service.Systems()
	if err != nil {
		err = fmt.Errorf("failed to get systems from Redfish server: %w", err)
		return
	}

	// Find the system matching the device name:
	var path string
	for _, system := range systems {
		if system.Name == device {
			path = system.ODataID
			break
		}
	}
	if path == "" {
		err = fmt.Errorf("system with name '%s' not found", device)
		return
	}

	// Build the virtual media URL:
	result = fmt.Sprintf("redfish-virtualmedia://%s%s", address, path)
	return
}

// deviceToHost converts a BCM device to a fulfillment service host.
func (t *synchronizerTask) deviceToHost(ctx context.Context, device *Device) (
	*privatev1.Host, error) {
	// Create the host with the basic data:
	host := privatev1.Host_builder{
		Id: device.Uuid,
		Metadata: privatev1.Metadata_builder{
			Name: device.Hostname,
		}.Build(),
		Spec: privatev1.HostSpec_builder{
			Class:   device.Category,
			BootMac: device.Mac,
			Bmc: privatev1.BMC_builder{
				User:     device.BmcSettings.UserName,
				Password: device.BmcSettings.Password,
				Insecure: true,
			}.Build(),
		}.Build(),
	}.Build()

	// Find the BMC network interface in order to build the BMC URL:
	for _, iface := range device.Interfaces {
		if iface.ChildType == "NetworkBmcInterface" {
			switch {
			case redfishInterfaceRegex.MatchString(iface.Name):
				url, err := t.buildRedfishUrl(
					ctx,
					iface.IP,
					device.BmcSettings.UserName,
					device.BmcSettings.Password,
					device.Hostname,
				)
				if err != nil {
					t.logger.ErrorContext(
						ctx,
						"Failed to build Redfish URL",
						slog.String("interface", iface.Name),
						slog.String("ip", iface.IP),
						slog.Any("error", err),
					)
				} else {
					host.GetSpec().GetBmc().SetUrl(url)
				}
			default:
				t.logger.ErrorContext(
					ctx,
					"Unsupported BMC interface type",
					slog.String("interface", iface.Name),
					slog.String("ip", iface.IP),
				)
			}
		}
	}

	// Find the boot network interface in order to extract the boot IP address:
	for _, iface := range device.Interfaces {
		if iface.Name == "BOOTIF" {
			host.GetSpec().SetBootIp(iface.IP)
			break
		}
	}

	// Set the rack:
	rack := t.racksByUuid[device.RackPosition.Rack]
	if rack != nil {
		host.GetSpec().SetRack(rack.Name)
	}

	return host, nil
}

func (t *synchronizerTask) createOrUpdateHost(ctx context.Context, host *privatev1.Host) error {
	// Try to create the host:
	_, err := t.hostsClient.Create(ctx, privatev1.HostsCreateRequest_builder{
		Object: host,
	}.Build())
	if err == nil {
		t.logger.InfoContext(
			ctx,
			"Created new host",
			slog.String("id", host.GetId()),
		)
		return nil
	}
	if grpcstatus.Code(err) != grpccodes.AlreadyExists {
		return fmt.Errorf("failed to create host: %w", err)
	}

	// Try to update the host:
	_, err = t.hostsClient.Update(ctx, &privatev1.HostsUpdateRequest{
		Object: host,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{
				"metadata.name",
				"spec",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update host: %w", err)
	}
	t.logger.InfoContext(
		ctx,
		"Updated existing host",
		slog.String("id", host.GetId()),
	)

	return nil
}

func (t *synchronizerTask) synchronizeCategories(ctx context.Context) error {
	for _, category := range t.categoriesByUuid {
		err := t.synchronizeCategory(ctx, category)
		if err != nil {
			t.logger.WarnContext(
				ctx,
				"Failed to synchronize category",
				slog.String("id", category.Uuid),
				slog.String("name", category.Name),
				slog.Any("error", err),
			)
			continue
		}
	}
	return nil
}

func (t *synchronizerTask) synchronizeCategory(ctx context.Context, category *Category) error {
	// Convert the category to a host class:
	hostClass := privatev1.HostClass_builder{
		Id: category.Uuid,
		Metadata: privatev1.Metadata_builder{
			Name: category.Name,
		}.Build(),
		Title:       fmt.Sprintf("BCM `%s` category", category.Name),
		Description: fmt.Sprintf("Extracted from BCM device category `%s`.", category.Name),
	}.Build()

	// Create or update the host class:
	err := t.createOrUpdateHostClass(ctx, hostClass)
	if err != nil {
		return fmt.Errorf("failed to create or update host class: %w", err)
	}
	t.logger.DebugContext(
		ctx,
		"Synchronized host class",
		slog.String("id", category.Uuid),
		slog.String("name", category.Name),
	)

	return nil
}

func (t *synchronizerTask) createOrUpdateHostClass(ctx context.Context, hostClass *privatev1.HostClass) error {
	// Try to create the host class:
	_, err := t.hostClassesClient.Create(ctx, &privatev1.HostClassesCreateRequest{
		Object: hostClass,
	})
	if err == nil {
		t.logger.InfoContext(
			ctx,
			"Created new host class",
			slog.String("id", hostClass.GetId()),
		)
		return nil
	}
	if grpcstatus.Code(err) != grpccodes.AlreadyExists {
		return fmt.Errorf("failed to create host class: %w", err)
	}

	// Try to update the host class:
	_, err = t.hostClassesClient.Update(ctx, &privatev1.HostClassesUpdateRequest{
		Object: hostClass,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{
				"metadata.name",
				"title",
				"description",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update host class: %w", err)
	}
	t.logger.InfoContext(
		ctx,
		"Updated existing host class",
		slog.String("id", hostClass.GetId()),
	)
	return nil
}

var (
	redfishInterfaceRegex = regexp.MustCompile(`^rf\d+$`)
)
