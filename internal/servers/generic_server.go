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
	"reflect"

	"github.com/google/uuid"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	eventsv1 "github.com/innabox/fulfillment-service/internal/api/events/v1"
	ffv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

// GenericServerBuilder contains the data and logic neede to create new generic servers.
type GenericServerBuilder[O dao.Object] struct {
	logger  *slog.Logger
	service string
	table   string
}

// GenericServer is a gRPC server that knows how to implement the List, Get, Create, Update and Delete operators for
// any object that has identifier and metadata fields.
type GenericServer[O dao.Object] struct {
	logger         *slog.Logger
	service        string
	dao            *dao.GenericDAO[O]
	listRequest    proto.Message
	listResponse   proto.Message
	getRequest     proto.Message
	getResponse    proto.Message
	createRequest  proto.Message
	createResponse proto.Message
	updateRequest  proto.Message
	updateResponse proto.Message
	deleteRequest  proto.Message
	deleteResponse proto.Message
	notifier       *database.Notifier
}

// NewGeneric server creates a builder that can then be used to configure and create a new generic server.
func NewGenericServer[O dao.Object]() *GenericServerBuilder[O] {
	return &GenericServerBuilder[O]{}
}

// SetLogger sets the logger. This is mandatory.
func (b *GenericServerBuilder[O]) SetLogger(value *slog.Logger) *GenericServerBuilder[O] {
	b.logger = value
	return b
}

// SetService sets the service description. This is mandatory.
func (b *GenericServerBuilder[O]) SetService(value string) *GenericServerBuilder[O] {
	b.service = value
	return b
}

// SetTable sets the name of the table where the objects will be stored. This is mandatory.
func (b *GenericServerBuilder[O]) SetTable(value string) *GenericServerBuilder[O] {
	b.table = value
	return b
}

// Build uses the configuration stored in the builder to create and configure a new generic server.
func (b *GenericServerBuilder[O]) Build() (result *GenericServer[O], err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.service == "" {
		err = errors.New("service name is mandatory")
		return
	}
	if b.table == "" {
		err = errors.New("table name is mandatory")
		return
	}

	// Create the object early so that we can use its methods as callbacks:
	s := &GenericServer[O]{
		logger:  b.logger,
		service: b.service,
	}

	// Create the DAO:
	s.dao, err = dao.NewGenericDAO[O]().
		SetLogger(b.logger).
		SetTable(b.table).
		AddEventCallback(s.notifyEvent).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create DAO: %w", err)
		return
	}

	// Create the notifier:
	s.notifier, err = database.NewNotifier().
		SetLogger(b.logger).
		SetChannel("events").
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create notifier: %w", err)
		return
	}

	// Find the descriptor:
	service, err := b.findService()
	if err != nil {
		return
	}

	// Prepare templates for the request and response types. These are empty messages that will be cloned when
	// it is necessary to create new instances.
	s.listRequest, s.listResponse, err = b.findRequestAndResponse(service, listMethod)
	if err != nil {
		return
	}
	s.getRequest, s.getResponse, err = b.findRequestAndResponse(service, getMethod)
	if err != nil {
		return
	}
	s.createRequest, s.createResponse, err = b.findRequestAndResponse(service, createMethod)
	if err != nil {
		return
	}
	s.deleteRequest, s.deleteResponse, err = b.findRequestAndResponse(service, deleteMethod)
	if err != nil {
		return
	}
	s.updateRequest, s.updateResponse, err = b.findRequestAndResponse(service, updateMethod)
	if err != nil {
		return
	}

	result = s
	return
}

func (b *GenericServerBuilder[O]) findService() (result protoreflect.ServiceDescriptor, err error) {
	serviceName := protoreflect.FullName(b.service)
	packageName := serviceName.Parent()
	protoregistry.GlobalFiles.RangeFilesByPackage(packageName, func(fileDesc protoreflect.FileDescriptor) bool {
		serviceDescs := fileDesc.Services()
		for i := range serviceDescs.Len() {
			currentDesc := serviceDescs.Get(i)
			if currentDesc.FullName() == serviceName {
				result = currentDesc
				return false
			}
		}
		return true
	})
	if result == nil {
		err = fmt.Errorf("failed to find descriptor for service '%s'", serviceName)
		return
	}
	return
}

func (b *GenericServerBuilder[O]) findRequestAndResponse(serviceDesc protoreflect.ServiceDescriptor,
	methodName string) (request, response proto.Message, err error) {
	methodDesc := serviceDesc.Methods().ByName(protoreflect.Name(methodName))
	if methodDesc == nil {
		err = fmt.Errorf("failed to find method '%s' for service '%s'", methodName, serviceDesc.FullName())
		return
	}
	requestDesc := methodDesc.Input()
	requestName := requestDesc.FullName()
	requestType, err := protoregistry.GlobalTypes.FindMessageByName(requestName)
	if err != nil {
		err = fmt.Errorf(
			"failed to find request type '%s' for method '%s' of service '%s'",
			requestName, methodName, serviceDesc.FullName(),
		)
		return
	}
	responseDesc := methodDesc.Output()
	responseName := responseDesc.FullName()
	responseType, err := protoregistry.GlobalTypes.FindMessageByName(responseName)
	if err != nil {
		err = fmt.Errorf(
			"failed to find response type '%s' for method '%s' of service '%s'",
			responseName, methodName, serviceDesc.FullName(),
		)
		return
	}
	request = requestType.New().Interface()
	response = responseType.New().Interface()
	return
}

func (s *GenericServer[O]) List(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetOffset() int32
		GetLimit() int32
	}
	type responseIface[O dao.Object] interface {
		SetSize(int32)
		SetTotal(int32)
		SetItems([]O)
	}
	requestMsg := request.(requestIface)
	daoRequest := dao.ListRequest{}
	daoRequest.Offset = requestMsg.GetOffset()
	daoRequest.Limit = requestMsg.GetLimit()
	daoResponse, err := s.dao.List(ctx, daoRequest)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to list",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to list")
	}
	responseMsg := proto.Clone(s.listResponse).(responseIface[O])
	responseMsg.SetSize(daoResponse.Size)
	responseMsg.SetTotal(daoResponse.Total)
	responseMsg.SetItems(daoResponse.Items)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[O]) Get(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetId() string
	}
	type responseIface[O dao.Object] interface {
		SetObject(O)
	}
	requestMsg := request.(requestIface)
	id := requestMsg.GetId()
	if id == "" {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "identifier is mandatory")
	}
	object, err := s.dao.Get(ctx, id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get",
			slog.String("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to get object with identifier '%s'", id)
	}
	if s.isNil(object) {
		return grpcstatus.Errorf(grpccodes.NotFound, "object with identifier '%s' doesn't exist", id)
	}
	responseMsg := proto.Clone(s.getResponse).(responseIface[O])
	responseMsg.SetObject(object)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[O]) Create(ctx context.Context, request any, response any) error {
	type requestIface[O dao.Object] interface {
		GetObject() O
	}
	type responseIface[O dao.Object] interface {
		SetObject(O)
	}
	requestMsg := request.(requestIface[O])
	object := requestMsg.GetObject()
	if s.isNil(object) {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
	}
	created, err := s.dao.Create(ctx, object)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to create",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to create object")
	}
	responseMsg := proto.Clone(s.createResponse).(responseIface[O])
	responseMsg.SetObject(created)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[O]) Update(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetObject() O
	}
	type responseIface interface {
		SetObject(O)
	}
	requestMsg := request.(requestIface)
	object := requestMsg.GetObject()
	if s.isNil(object) {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
	}
	id := object.GetId()
	if id == "" {
		return grpcstatus.Errorf(grpccodes.Internal, "object identifier is mandatory")
	}
	exists, err := s.dao.Exists(ctx, id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get check if object exists",
			slog.String("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get check if object with identifier '%s' exists",
			id,
		)
	}
	if !exists {
		return grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"object with identifier '%s' doesn't exist",
			id,
		)
	}
	updated, err := s.dao.Update(ctx, object)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to update object",
			slog.String("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to update object with identifier '%s'",
			id,
		)
	}
	responseMsg := proto.Clone(s.updateResponse).(responseIface)
	responseMsg.SetObject(updated)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[O]) Delete(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetId() string
	}
	type responseIface interface {
	}
	requestMsg := request.(requestIface)
	id := requestMsg.GetId()
	if id == "" {
		return grpcstatus.Errorf(grpccodes.Internal, "object identifier is mandatory")
	}
	err := s.dao.Delete(ctx, id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to delete object",
			slog.String("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to delete object")
	}
	responseMsg := proto.Clone(s.deleteResponse).(responseIface)
	s.setPointer(response, responseMsg)
	return nil
}

// notifyEvent converts the DAO event into an API event and publishes it using the PostgreSQL NOTIFY command.
func (s *GenericServer[O]) notifyEvent(ctx context.Context, e dao.Event) (err error) {
	// TODO: This is the only part of the generic server that depends on specific object types. Is there a way
	// to avoid that?
	event := &eventsv1.Event{}
	event.Id = uuid.NewString()
	switch e.Type {
	case dao.EventTypeCreated:
		event.Type = eventsv1.EventType_EVENT_TYPE_OBJECT_CREATED
	case dao.EventTypeUpdated:
		event.Type = eventsv1.EventType_EVENT_TYPE_OBJECT_UPDATED
	case dao.EventTypeDeleted:
		event.Type = eventsv1.EventType_EVENT_TYPE_OBJECT_DELETED
	default:
		return fmt.Errorf("unknown event kind '%s'", e.Type)
	}
	switch object := e.Object.(type) {
	case *ffv1.Cluster:
		event.Payload = &eventsv1.Event_Cluster{
			Cluster: object,
		}
	case *ffv1.ClusterOrder:
		event.Payload = &eventsv1.Event_ClusterOrder{
			ClusterOrder: object,
		}
	case *ffv1.ClusterTemplate:
		event.Payload = &eventsv1.Event_ClusterTemplate{
			ClusterTemplate: object,
		}
	default:
		return
	}
	return s.notifier.Notify(ctx, event)
}

func (s *GenericServer[O]) isNil(object O) bool {
	return reflect.ValueOf(object).IsNil()
}

func (s *GenericServer[O]) setPointer(pointer any, value any) {
	reflect.ValueOf(pointer).Elem().Set(reflect.ValueOf(value))
}

// Names of methods:
const (
	listMethod   = "List"
	getMethod    = "Get"
	createMethod = "Create"
	updateMethod = "Update"
	deleteMethod = "Delete"
)
