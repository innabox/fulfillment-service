/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	eventsv1 "github.com/innabox/fulfillment-service/internal/api/events/v1"
)

// ReconcilerFunction is a function that receives the current state of an object and reconciles it.
type ReconcilerFunction[M proto.Message] func(ctx context.Context, object M) error

// ReconcilerBuilder contains the data and logic needed to create a controller. Don't create instances f this directly,
// use the NewReconciler function instead.
type ReconcilerBuilder[M proto.Message] struct {
	logger        *slog.Logger
	flags         *pflag.FlagSet
	function      ReconcilerFunction[M]
	eventFilter   string
	objectFilter  string
	syncInterval  time.Duration
	watchInterval time.Duration
	grpcClient    *grpc.ClientConn
}

// Client simplifies use of the API for clients.
type Reconciler[M proto.Message] struct {
	logger        *slog.Logger
	function      ReconcilerFunction[M]
	eventFilter   string
	objectFilter  string
	syncInterval  time.Duration
	lastSync      time.Time
	watchInterval time.Duration
	lastWatch     time.Time
	grpcClient    *grpc.ClientConn
	payloadField  protoreflect.FieldDescriptor
	listMethod    reconcilerListMethodInfo
	objectChannel chan M
	eventsClient  eventsv1.EventsClient
}

// NewReconciler creates a builder that can then be used to configure and create a controller.
func NewReconciler[M proto.Message]() *ReconcilerBuilder[M] {
	return &ReconcilerBuilder[M]{
		syncInterval:  1 * time.Hour,
		watchInterval: 10 * time.Second,
	}
}

// SetLogger sets the logger. This is mandatory.
func (b *ReconcilerBuilder[M]) SetLogger(value *slog.Logger) *ReconcilerBuilder[M] {
	b.logger = value
	return b
}

// SetClient sets the gRPC client that will be used to talk to the server. This is mandatory.
func (b *ReconcilerBuilder[M]) SetClient(value *grpc.ClientConn) *ReconcilerBuilder[M] {
	b.grpcClient = value
	return b
}

// SetFunction sets the function that does the actual reconciliation. This is mandatory.
func (b *ReconcilerBuilder[M]) SetFunction(value ReconcilerFunction[M]) *ReconcilerBuilder[M] {
	b.function = value
	return b
}

// SetEventFilter sets the filter that will be used to decide which events trigger a reconciliation. This is optional,
// by default all events affecting objects of the type supported by the reconciler will trigger a reconciliation.
func (b *ReconcilerBuilder[M]) SetEventFilter(value string) *ReconcilerBuilder[M] {
	b.eventFilter = value
	return b
}

// SetObjectFilter sets the filter that will be used to decide which objects will be reconciled during periodic
// synchronization. This is optional, by default all objects of the type supported by the reconciler will be
// reconciled.
func (b *ReconcilerBuilder[M]) SetObjectFilter(value string) *ReconcilerBuilder[M] {
	b.objectFilter = value
	return b
}

// SetSyncInterval sets how often the reconciler will fetch and reconcile again all the objects. This is optional, and
// the default is one hour.
func (b *ReconcilerBuilder[M]) SetSyncInterval(value time.Duration) *ReconcilerBuilder[M] {
	b.syncInterval = value
	return b
}

// SetWatchInterval sets how long the reconciler will wait before trying to start watching again when the stream stops.
// This is optional, and the default is 10 seconds.
func (b *ReconcilerBuilder[M]) SetWatchInterval(value time.Duration) *ReconcilerBuilder[M] {
	b.watchInterval = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the reconciler. This is optional.
func (b *ReconcilerBuilder[M]) SetFlags(flags *pflag.FlagSet, name string) *ReconcilerBuilder[M] {
	b.flags = flags
	return b
}

// Build uses the data stored in the buider to create a new reconciler.
func (b *ReconcilerBuilder[M]) Build() (result *Reconciler[M], err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.grpcClient == nil {
		err = errors.New("gRPC client is mandatory")
		return
	}
	if b.function == nil {
		err = errors.New("function is mandatory")
		return
	}
	if b.syncInterval <= 0 {
		err = fmt.Errorf("sync interval should be positive, but it is %s", b.syncInterval)
		return
	}

	// Find the field of the event payload that contains the type of objects supported by the reconciler:
	payloadField, err := b.findPayloadField()
	if err != nil {
		err = fmt.Errorf("failed to find payload field: %w", err)
		return
	}

	// Find the method that will be used to list objects during synchronization:
	listMethod, err := b.findListMethod()
	if err != nil {
		err = fmt.Errorf("failed to find list method: %w", err)
		return
	}

	// Set the default event filter:
	eventFilter := b.eventFilter
	if eventFilter == "" {
		eventFilter = fmt.Sprintf("has(event.%s)", payloadField.Name())
	}

	// Set the default object filter:
	objectFilter := b.objectFilter
	if objectFilter == "" {
		// TODO: Calculate a default object filter once object filtering support is implemented.
		objectFilter = ""
	}

	// Create the events client:
	eventsClient := eventsv1.NewEventsClient(b.grpcClient)

	// Create and populate the object:
	result = &Reconciler[M]{
		logger:        b.logger,
		function:      b.function,
		eventFilter:   eventFilter,
		objectFilter:  b.objectFilter,
		syncInterval:  b.syncInterval,
		watchInterval: b.watchInterval,
		grpcClient:    b.grpcClient,
		payloadField:  payloadField,
		listMethod:    listMethod,
		objectChannel: make(chan M),
		eventsClient:  eventsClient,
	}
	return
}

// findPayloadField finds the field of the event type that contains the payload for the type supported by the
// reconciler. For example, if the type is a cluster order then the field will be `cluster_order`, if it is a cluster
// template it will be `cluster_template`, etc.
func (b *ReconcilerBuilder[M]) findPayloadField() (result protoreflect.FieldDescriptor, err error) {
	var (
		event  *eventsv1.Event
		object M
	)
	eventDesc := event.ProtoReflect().Descriptor()
	eventFields := eventDesc.Fields()
	objectDesc := object.ProtoReflect().Descriptor()
	for i := range eventFields.Len() {
		eventField := eventFields.Get(i)
		if eventField.Kind() != protoreflect.MessageKind {
			continue
		}
		if eventField.Message().FullName() == objectDesc.FullName() {
			result = eventField
			return
		}
	}
	err = fmt.Errorf("failed to find event field for type '%T'", object)
	return
}

// reconcilerListMethodInfo contains the informatin needed to invoke the list method using reflection.
type reconcilerListMethodInfo struct {
	Name         string
	RequestType  protoreflect.MessageType
	ResponseType protoreflect.MessageType
	ItemsField   protoreflect.FieldDescriptor
}

// findListMethod finds the method that will be used to list objects.
func (b *ReconcilerBuilder[M]) findListMethod() (result reconcilerListMethodInfo, err error) {
	// Determine the name of the package of the supported type:
	var object M
	objectDesc := object.ProtoReflect().Descriptor()
	objectName := objectDesc.FullName()
	objectPkg := objectName.Parent()

	// Iterate over all the files, services and methods of the package to find list method:
	var (
		listMethodDesc   protoreflect.MethodDescriptor
		listRequestDesc  protoreflect.MessageDescriptor
		listResponseDesc protoreflect.MessageDescriptor
		listItemsDesc    protoreflect.FieldDescriptor
	)
	protoregistry.GlobalFiles.RangeFilesByPackage(
		objectPkg,
		func(fileDesc protoreflect.FileDescriptor) bool {
			serviceDescs := fileDesc.Services()
			for i := range serviceDescs.Len() {
				serviceDesc := serviceDescs.Get(i)
				methodDescs := serviceDesc.Methods()
				for j := range methodDescs.Len() {
					methodDesc := methodDescs.Get(j)
					if methodDesc.Name() != protoreflect.Name("List") {
						continue
					}
					inputDesc := methodDesc.Input()
					outputDesc := methodDesc.Output()
					itemsDesc := outputDesc.Fields().ByName("items")
					if itemsDesc == nil || !itemsDesc.IsList() {
						continue
					}
					if itemsDesc.Message().FullName() != objectName {
						continue
					}
					listMethodDesc = methodDesc
					listRequestDesc = inputDesc
					listResponseDesc = outputDesc
					listItemsDesc = itemsDesc
					return false
				}
			}
			return true
		},
	)
	if listMethodDesc == nil {
		err = fmt.Errorf("failed to find list method for type '%T", object)
		return
	}

	// Find the request and response types:
	requestType, err := protoregistry.GlobalTypes.FindMessageByName(listRequestDesc.FullName())
	if err != nil {
		return
	}
	responseType, err := protoregistry.GlobalTypes.FindMessageByName(listResponseDesc.FullName())
	if err != nil {
		return
	}

	// Calculate the name of the method with the `/service/method` format that is used to invoke it:
	fullMethodName := listMethodDesc.FullName()
	methodName := fmt.Sprintf("/%s/%s", fullMethodName.Parent(), fullMethodName.Name())

	// Prepare the result:
	result.Name = methodName
	result.RequestType = requestType
	result.ResponseType = responseType
	result.ItemsField = listItemsDesc
	return
}

// Start starts the controller. To stop it cancel the context.
func (c *Reconciler[M]) Start(ctx context.Context) error {
	// Start the watch and sync loops:
	go c.watchLoop(ctx)
	go c.syncLoop(ctx)

	// Run the reconcile loop:
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case object := <-c.objectChannel:
			c.logger.DebugContext(
				ctx,
				"Reconciling object",
				slog.Any("object", object),
			)
			err := c.function(ctx, object)
			if err != nil {
				c.logger.ErrorContext(
					ctx,
					"Reconciliation failed",
					slog.Any("error", err),
				)
			}
		}
	}
}

func (c *Reconciler[M]) watchLoop(ctx context.Context) {
	for {
		err := c.watchEvents(ctx)
		if errors.Is(err, context.Canceled) {
			c.logger.InfoContext(ctx, "Watch finished")
			return
		}
		if err != nil {
			c.logger.ErrorContext(
				ctx,
				"Watch failed",
				slog.Any("error", err),
			)
		}
		err = c.sleepAfterWatch(ctx)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			c.logger.ErrorContext(
				ctx,
				"Sleep failed",
				slog.Any("error", err),
			)
		}
	}
}

func (c *Reconciler[M]) watchEvents(ctx context.Context) error {
	stream, err := c.eventsClient.Watch(ctx, &eventsv1.EventsWatchRequest{
		Filter: &c.eventFilter,
	})
	if err != nil {
		return err
	}
	for {
		response, err := stream.Recv()
		if err != nil {
			return err
		}
		payload := response.Event.ProtoReflect().Get(c.payloadField).Message().Interface()
		object, ok := payload.(M)
		if !ok {
			return fmt.Errorf("expected payload of type %T, but got %T", object, payload)
		}
		c.logger.DebugContext(
			ctx,
			"Enqueueing object",
			slog.Any("object", object),
		)
		c.objectChannel <- object
	}
}

func (c *Reconciler[M]) syncLoop(ctx context.Context) {
	for {
		err := c.syncObjects(ctx)
		if errors.Is(err, context.Canceled) {
			c.logger.InfoContext(ctx, "Sync finished")
			return
		}
		if err != nil {
			c.logger.ErrorContext(
				ctx,
				"Sync failed",
				slog.Any("error", err),
			)
		}
		err = c.sleepAfterSync(ctx)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			c.logger.ErrorContext(
				ctx,
				"Sleep failed",
				slog.Any("error", err),
			)
		}
	}
}

func (c *Reconciler[M]) syncObjects(ctx context.Context) error {
	request := c.listMethod.RequestType.New().Interface()
	response := c.listMethod.ResponseType.New().Interface()
	err := c.grpcClient.Invoke(ctx, c.listMethod.Name, request, response)
	if err != nil {
		return err
	}
	value := reflect.ValueOf(response)
	field := value.Elem().FieldByName("Items")
	items := field.Interface().([]M)
	for _, item := range items {
		c.objectChannel <- item
	}
	return nil
}

// sleepAfterWatch waits till the watch interval passes, or till the context is cancelled.
func (c *Reconciler[M]) sleepAfterWatch(ctx context.Context) error {
	return c.sleep(ctx, "watch", &c.lastWatch, c.watchInterval)
}

// sleepAfterSync waits till the sync interval passes, or till the context is cancelled.
func (c *Reconciler[M]) sleepAfterSync(ctx context.Context) error {
	return c.sleep(ctx, "sync", &c.lastSync, c.syncInterval)
}

func (c *Reconciler[M]) sleep(ctx context.Context, reason string, last *time.Time, interval time.Duration) error {
	defer func() {
		*last = time.Now()
	}()
	elapsed := time.Since(*last)
	duration := interval - elapsed
	if duration <= 0 {
		c.logger.DebugContext(
			ctx,
			"No need to sleep",
			slog.String("reason", reason),
			slog.Time("last", *last),
			slog.Duration("elapsed", elapsed),
			slog.Duration("interval", interval),
		)
		return nil
	}
	c.logger.DebugContext(
		ctx,
		"Sleeping",
		slog.String("reason", reason),
		slog.Time("last", *last),
		slog.Duration("elapsed", elapsed),
		slog.Duration("interval", interval),
		slog.Duration("duration", duration),
	)
	select {
	case <-ctx.Done():
		return context.Canceled
	case <-time.After(duration):
		return nil
	}
}
