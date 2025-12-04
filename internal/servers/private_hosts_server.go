/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package servers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/auth"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

type PrivateHostsServerBuilder struct {
	logger           *slog.Logger
	notifier         *database.Notifier
	attributionLogic auth.AttributionLogic
	tenancyLogic     auth.TenancyLogic
}

var _ privatev1.HostsServer = (*PrivateHostsServer)(nil)

type PrivateHostsServer struct {
	privatev1.UnimplementedHostsServer

	logger         *slog.Logger
	hostClassesDao *dao.GenericDAO[*privatev1.HostClass]
	generic        *GenericServer[*privatev1.Host]
}

func NewPrivateHostsServer() *PrivateHostsServerBuilder {
	return &PrivateHostsServerBuilder{}
}

func (b *PrivateHostsServerBuilder) SetLogger(value *slog.Logger) *PrivateHostsServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateHostsServerBuilder) SetNotifier(value *database.Notifier) *PrivateHostsServerBuilder {
	b.notifier = value
	return b
}

func (b *PrivateHostsServerBuilder) SetAttributionLogic(value auth.AttributionLogic) *PrivateHostsServerBuilder {
	b.attributionLogic = value
	return b
}

func (b *PrivateHostsServerBuilder) SetTenancyLogic(value auth.TenancyLogic) *PrivateHostsServerBuilder {
	b.tenancyLogic = value
	return b
}

func (b *PrivateHostsServerBuilder) Build() (result *PrivateHostsServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.tenancyLogic == nil {
		err = errors.New("tenancy logic is mandatory")
		return
	}

	// Create the host classes DAO:
	hostClassesDao, err := dao.NewGenericDAO[*privatev1.HostClass]().
		SetLogger(b.logger).
		SetTable("host_classes").
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*privatev1.Host]().
		SetLogger(b.logger).
		SetService(privatev1.Hosts_ServiceDesc.ServiceName).
		SetTable("hosts").
		SetNotifier(b.notifier).
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &PrivateHostsServer{
		logger:         b.logger,
		hostClassesDao: hostClassesDao,
		generic:        generic,
	}
	return
}

func (s *PrivateHostsServer) List(ctx context.Context,
	request *privatev1.HostsListRequest) (response *privatev1.HostsListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *PrivateHostsServer) Get(ctx context.Context,
	request *privatev1.HostsGetRequest) (response *privatev1.HostsGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *PrivateHostsServer) Create(ctx context.Context,
	request *privatev1.HostsCreateRequest) (response *privatev1.HostsCreateResponse, err error) {
	s.setDefaults(request.GetObject())

	// The user may have specified the host class by name, but we want to save the identifier, so we need to look
	// it up:
	host := request.GetObject()
	hostClassRef := host.GetSpec().GetClass()
	if hostClassRef != "" {
		var hostClass *privatev1.HostClass
		hostClass, err = s.lookupHostClass(ctx, hostClassRef)
		if err != nil {
			return
		}
		host.GetSpec().SetClass(hostClass.GetId())
	}

	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *PrivateHostsServer) Update(ctx context.Context,
	request *privatev1.HostsUpdateRequest) (response *privatev1.HostsUpdateResponse, err error) {
	// The user may have specified the host class by name, but we want to save the identifier, so we need to look
	// it up:
	host := request.GetObject()
	hostClassRef := host.GetSpec().GetClass()
	if hostClassRef != "" {
		var hostClass *privatev1.HostClass
		hostClass, err = s.lookupHostClass(ctx, hostClassRef)
		if err != nil {
			return
		}
		host.GetSpec().SetClass(hostClass.GetId())
	}

	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *PrivateHostsServer) Delete(ctx context.Context,
	request *privatev1.HostsDeleteRequest) (response *privatev1.HostsDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}

func (s *PrivateHostsServer) Signal(ctx context.Context,
	request *privatev1.HostsSignalRequest) (response *privatev1.HostsSignalResponse, err error) {
	err = s.generic.Signal(ctx, request, &response)
	return
}

func (s *PrivateHostsServer) setDefaults(host *privatev1.Host) {
	if !host.HasStatus() {
		host.SetStatus(&privatev1.HostStatus{})
	}
}

// lookupHostClass looks up a host class by identifier or name. It returns the host class if found, or an error if not
// found or if there are multiple matches.
func (s *PrivateHostsServer) lookupHostClass(ctx context.Context,
	key string) (result *privatev1.HostClass, err error) {
	if key == "" {
		return
	}
	response, err := s.hostClassesDao.List(ctx, dao.ListRequest{
		Filter: fmt.Sprintf("this.id == %[1]s || this.metadata.name == %[1]s", strconv.Quote(key)),
		Limit:  1,
	})
	if err != nil {
		return
	}
	switch response.Size {
	case 0:
		err = grpcstatus.Errorf(
			grpccodes.NotFound,
			"there is no host class with identifier or name '%s'",
			key,
		)
	case 1:
		result = response.Items[0]
	default:
		err = grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"there are multiple host classes with identifier or name '%s'",
			key,
		)
	}
	return
}
