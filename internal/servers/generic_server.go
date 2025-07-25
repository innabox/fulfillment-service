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

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database"
	"github.com/innabox/fulfillment-service/internal/database/dao"
)

// GenericServerBuilder contains the data and logic neede to create new generic servers.
type GenericServerBuilder[Public, Private dao.Object] struct {
	logger        *slog.Logger
	service       string
	table         string
	ignoredFields []any
	notifier      *database.Notifier
}

// GenericServer is a gRPC server that knows how to implement the List, Get, Create, Update and Delete operators for
// any object that has identifier and metadata fields.
type GenericServer[Public, Private dao.Object] struct {
	logger          *slog.Logger
	service         string
	dao             *dao.GenericDAO[Private]
	inMapper        *GenericMapper[Public, Private]
	outMapper       *GenericMapper[Private, Public]
	publicTemplate  proto.Message
	privateTemplate proto.Message
	listRequest     proto.Message
	listResponse    proto.Message
	getRequest      proto.Message
	getResponse     proto.Message
	createRequest   proto.Message
	createResponse  proto.Message
	updateRequest   proto.Message
	updateResponse  proto.Message
	deleteRequest   proto.Message
	deleteResponse  proto.Message
	notifier        *database.Notifier
}

// NewGeneric server creates a builder that can then be used to configure and create a new generic server.
func NewGenericServer[Public, Private dao.Object]() *GenericServerBuilder[Public, Private] {
	return &GenericServerBuilder[Public, Private]{}
}

// SetLogger sets the logger. This is mandatory.
func (b *GenericServerBuilder[Public, Private]) SetLogger(value *slog.Logger) *GenericServerBuilder[Public, Private] {
	b.logger = value
	return b
}

// SetService sets the service description. This is mandatory.
func (b *GenericServerBuilder[Public, Private]) SetService(value string) *GenericServerBuilder[Public, Private] {
	b.service = value
	return b
}

// SetTable sets the name of the table where the objects will be stored. This is mandatory.
func (b *GenericServerBuilder[Public, Private]) SetTable(value string) *GenericServerBuilder[Public, Private] {
	b.table = value
	return b
}

// AddIgnoredFields adds a set of fields to be omitted when mapping objects. The values passed can be of the following
// types:
//
// string - This should be a field name, for example 'status' and then any field with that name in any object will
// be ignored.
//
// protoreflect.Name - Like string.
//
// protoreflect.FullName - This indicates a field of a particular type. For example, if the value is
// 'fulfillment.v1.Cluster.status' then only the 'status' field of the 'fulfillment.v1.Cluster' object will be ignored.
func (b *GenericServerBuilder[Public, Private]) AddIgnoredFields(values ...any) *GenericServerBuilder[Public, Private] {
	b.ignoredFields = append(b.ignoredFields, values...)
	return b
}

// SetNotifier sets the notifier that the server will use to send events when objects are created, updated or deleted.
// This is optional.
func (b *GenericServerBuilder[Public, Private]) SetNotifier(
	value *database.Notifier) *GenericServerBuilder[Public, Private] {
	b.notifier = value
	return b
}

// Build uses the configuration stored in the builder to create and configure a new generic server.
func (b *GenericServerBuilder[Public, Private]) Build() (result *GenericServer[Public, Private], err error) {
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
	s := &GenericServer[Public, Private]{
		logger:   b.logger,
		service:  b.service,
		notifier: b.notifier,
	}

	// Create the DAO:
	daoBuilder := dao.NewGenericDAO[Private]()
	daoBuilder.SetLogger(b.logger)
	daoBuilder.SetTable(b.table)
	if b.notifier != nil {
		daoBuilder.AddEventCallback(s.notifyEvent)
	}
	s.dao, err = daoBuilder.Build()
	if err != nil {
		err = fmt.Errorf("failed to create DAO: %w", err)
		return
	}

	// Create the mappers:
	s.inMapper, err = NewGenericMapper[Public, Private]().
		SetLogger(b.logger).
		SetStrict(true).
		AddIgnoredFields(b.ignoredFields...).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create in mapper: %w", err)
		return
	}
	s.outMapper, err = NewGenericMapper[Private, Public]().
		SetLogger(b.logger).
		SetStrict(false).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create out mapper: %w", err)
		return
	}

	// Find the descriptor:
	service, err := b.findService()
	if err != nil {
		return
	}

	// Prepare the templates for the public and private objects:
	var (
		public  Public
		private Private
	)
	s.publicTemplate = public.ProtoReflect().New().Interface()
	s.privateTemplate = private.ProtoReflect().New().Interface()

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

func (b *GenericServerBuilder[Public, Private]) findService() (result protoreflect.ServiceDescriptor, err error) {
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

func (b *GenericServerBuilder[Public, Private]) findRequestAndResponse(serviceDesc protoreflect.ServiceDescriptor,
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

func (s *GenericServer[Public, Private]) List(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetOffset() int32
		GetLimit() int32
		GetFilter() string
	}
	type responseIface interface {
		SetSize(int32)
		SetTotal(int32)
		SetItems([]Public)
	}
	requestMsg := request.(requestIface)
	daoRequest := dao.ListRequest{}
	daoRequest.Offset = requestMsg.GetOffset()
	daoRequest.Limit = requestMsg.GetLimit()
	daoRequest.Filter = requestMsg.GetFilter()
	daoResponse, err := s.dao.List(ctx, daoRequest)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to list",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to list")
	}
	responseMsg := proto.Clone(s.listResponse).(responseIface)
	responseMsg.SetSize(daoResponse.Size)
	responseMsg.SetTotal(daoResponse.Total)
	publicItems := make([]Public, len(daoResponse.Items))
	for i, privateItem := range daoResponse.Items {
		publicItem := proto.Clone(s.publicTemplate).(Public)
		err = s.outMapper.Copy(ctx, privateItem, publicItem)
		if err != nil {
			s.logger.ErrorContext(
				ctx,
				"Failed to map item",
				slog.Any("error", err),
			)
			return grpcstatus.Errorf(grpccodes.Internal, "failed to list")
		}
		publicItems[i] = publicItem
	}
	responseMsg.SetItems(publicItems)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[Public, Private]) Get(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetId() string
	}
	type responseIface interface {
		SetObject(Public)
	}
	requestMsg := request.(requestIface)
	id := requestMsg.GetId()
	if id == "" {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "identifier is mandatory")
	}
	private, err := s.dao.Get(ctx, id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get",
			slog.String("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to get object with identifier '%s'", id)
	}
	if s.isNil(private) {
		return grpcstatus.Errorf(grpccodes.NotFound, "object with identifier '%s' doesn't exist", id)
	}
	public := proto.Clone(s.publicTemplate).(Public)
	err = s.outMapper.Copy(ctx, private, public)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map",
			slog.String("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to get object with identifier '%s'", id)
	}
	responseMsg := proto.Clone(s.getResponse).(responseIface)
	responseMsg.SetObject(public)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[Public, Private]) Create(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetObject() Public
	}
	type responseIface interface {
		SetObject(Public)
	}
	requestMsg := request.(requestIface)
	public := requestMsg.GetObject()
	if s.isNil(public) {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
	}
	private := proto.Clone(s.privateTemplate).(Private)
	err := s.inMapper.Copy(ctx, public, private)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to create object")
	}
	private, err = s.dao.Create(ctx, private)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to create",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to create object")
	}
	err = s.outMapper.Copy(ctx, private, public)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(grpccodes.Internal, "failed to create object")
	}
	responseMsg := proto.Clone(s.createResponse).(responseIface)
	responseMsg.SetObject(public)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[Public, Private]) Update(ctx context.Context, request any, response any) error {
	type requestIface interface {
		GetObject() Public
	}
	type responseIface interface {
		SetObject(Public)
	}
	requestMsg := request.(requestIface)
	public := requestMsg.GetObject()
	if s.isNil(public) {
		return grpcstatus.Errorf(grpccodes.InvalidArgument, "object is mandatory")
	}
	id := public.GetId()
	if id == "" {
		return grpcstatus.Errorf(grpccodes.Internal, "object identifier is mandatory")
	}
	private, err := s.dao.Get(ctx, id)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to get object",
			slog.String("id", id),
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to get object with identifier '%s'",
			id,
		)
	}
	if s.isNil(private) {
		return grpcstatus.Errorf(
			grpccodes.InvalidArgument,
			"object with identifier '%s' doesn't exist",
			id,
		)
	}
	err = s.inMapper.Copy(ctx, public, private)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to copy object",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to update object with identifier '%s'",
			id,
		)
	}
	private, err = s.dao.Update(ctx, private)
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
	err = s.outMapper.Copy(ctx, private, public)
	if err != nil {
		s.logger.ErrorContext(
			ctx,
			"Failed to map",
			slog.Any("error", err),
		)
		return grpcstatus.Errorf(
			grpccodes.Internal,
			"failed to update object with identifier '%s'",
			id,
		)
	}
	responseMsg := proto.Clone(s.updateResponse).(responseIface)
	responseMsg.SetObject(public)
	s.setPointer(response, responseMsg)
	return nil
}

func (s *GenericServer[Public, Private]) Delete(ctx context.Context, request any, response any) error {
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
func (s *GenericServer[Public, Private]) notifyEvent(ctx context.Context, e dao.Event) error {
	// TODO: This is the only part of the generic server that depends on specific object types. Is there a way
	// to avoid that?
	event := &privatev1.Event{}
	event.SetId(uuid.NewString())
	switch e.Type {
	case dao.EventTypeCreated:
		event.SetType(privatev1.EventType_EVENT_TYPE_OBJECT_CREATED)
	case dao.EventTypeUpdated:
		event.SetType(privatev1.EventType_EVENT_TYPE_OBJECT_UPDATED)
	case dao.EventTypeDeleted:
		event.SetType(privatev1.EventType_EVENT_TYPE_OBJECT_DELETED)
	default:
		return fmt.Errorf("unknown event kind '%s'", e.Type)
	}
	switch object := e.Object.(type) {
	case *privatev1.ClusterTemplate:
		event.SetClusterTemplate(object)
	case *privatev1.Cluster:
		event.SetCluster(object)
	case *privatev1.HostClass:
		event.SetHostClass(object)
	case *privatev1.Hub:
		// TODO: We need to remove the Kubeconfig from the payload of the notification because that usually
		// exceeds the default limit of 8000 bytes of the PostgreSQL notification mechanism. A better way to
		// do this would be to store the payloads in a separate table. We will do that later.
		object = proto.Clone(object).(*privatev1.Hub)
		object.SetKubeconfig(nil)
		event.SetHub(object)
	default:
		return fmt.Errorf("unknown object type '%T'", object)
	}
	return s.notifier.Notify(ctx, event)
}

func (s *GenericServer[Public, Private]) isNil(object proto.Message) bool {
	return reflect.ValueOf(object).IsNil()
}

func (s *GenericServer[Public, Private]) setPointer(pointer any, value any) {
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
