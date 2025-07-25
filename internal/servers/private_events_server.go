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
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	privatev1 "github.com/innabox/fulfillment-service/internal/api/private/v1"
	"github.com/innabox/fulfillment-service/internal/database"
)

type PrivateEventsServerBuilder struct {
	logger *slog.Logger
	flags  *pflag.FlagSet
	dbUrl  string
}

var _ privatev1.EventsServer = (*PrivateEventsServer)(nil)

type PrivateEventsServer struct {
	privatev1.UnimplementedEventsServer

	logger   *slog.Logger
	listener *database.Listener
	subs     map[string]eventsServerSubInfo
	subsLock *sync.RWMutex
	celEnv   *cel.Env
}

type eventsServerSubInfo struct {
	stream     grpc.ServerStreamingServer[privatev1.EventsWatchResponse]
	filterSrc  string
	filterPrg  cel.Program
	eventsChan chan *privatev1.Event
}

func NewPrivateEventsServer() *PrivateEventsServerBuilder {
	return &PrivateEventsServerBuilder{}
}

func (b *PrivateEventsServerBuilder) SetLogger(value *slog.Logger) *PrivateEventsServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateEventsServerBuilder) SetFlags(value *pflag.FlagSet) *PrivateEventsServerBuilder {
	b.flags = value
	return b
}

func (b *PrivateEventsServerBuilder) SetDbUrl(value string) *PrivateEventsServerBuilder {
	b.dbUrl = value
	return b
}

func (b *PrivateEventsServerBuilder) Build() (result *PrivateEventsServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.dbUrl == "" {
		err = errors.New("database connection URL is mandatory")
		return
	}

	// Create  the CEL environment:
	celEnv, err := b.createCelEnv()
	if err != nil {
		err = fmt.Errorf("failed to create CEL environment: %w", err)
		return
	}

	// Create the object early so that whe can use its methods as callback functions:
	s := &PrivateEventsServer{
		logger:   b.logger,
		subs:     map[string]eventsServerSubInfo{},
		subsLock: &sync.RWMutex{},
		celEnv:   celEnv,
	}

	// Create the notification listener:
	s.listener, err = database.NewListener().
		SetLogger(b.logger).
		SetUrl(b.dbUrl).
		SetChannel("events").
		AddPayloadCallback(s.processPayload).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create notification listener: %w", err)
		return
	}

	result = s
	return
}

func (b *PrivateEventsServerBuilder) createCelEnv() (result *cel.Env, err error) {
	// Declare contants for the enum types of the package:
	var options []cel.EnvOption
	protoregistry.GlobalTypes.RangeEnums(func(enumType protoreflect.EnumType) bool {
		enumDesc := enumType.Descriptor()
		if !privateEventsServerPackages[enumDesc.FullName().Parent()] {
			return true
		}
		enumValues := enumDesc.Values()
		for i := range enumValues.Len() {
			valueDesc := enumValues.Get(i)
			valueName := string(valueDesc.Name())
			valueNumber := valueDesc.Number()
			valueConst := cel.Constant(valueName, cel.IntType, types.Int(valueNumber))
			options = append(options, valueConst)
			b.logger.Debug(
				"Added enum constant",
				slog.String("type", string(enumDesc.FullName())),
				slog.String("name", valueName),
				slog.Int64("value", int64(valueNumber)),
			)
		}
		return true
	})

	// Declare the event type:
	var eventModel *privatev1.Event
	options = append(options, cel.Types(eventModel))

	// Declare the event variable:
	eventDesc := eventModel.ProtoReflect().Descriptor()
	eventType := cel.ObjectType(string(eventDesc.FullName()))
	options = append(options, cel.Variable("event", eventType))

	// Create the CEL environment:
	result, err = cel.NewEnv(options...)
	return
}

// Starts starts the background components of the server, in particular the notification listener. This is a blocking
// operation, and will return only when the context is canceled.
func (s *PrivateEventsServer) Start(ctx context.Context) error {
	return s.listener.Listen(ctx)
}

func (s *PrivateEventsServer) Watch(request *privatev1.EventsWatchRequest,
	stream grpc.ServerStreamingServer[privatev1.EventsWatchResponse]) (err error) {
	// Get the context:
	ctx := stream.Context()

	// Compile the filterPrg expression:
	var (
		filterSrc string
		filterPrg cel.Program
	)
	if request.Filter != nil {
		filterSrc = *request.Filter
		if filterSrc != "" {
			filterPrg, err = s.compileFilter(ctx, filterSrc)
			if err != nil {
				s.logger.ErrorContext(
					ctx,
					"Failed to compile filter",
					slog.String("filter", filterSrc),
					slog.Any("error", err),
				)
				return grpcstatus.Errorf(
					grpccodes.InvalidArgument,
					"failed to compile filter '%s'",
					filterSrc,
				)
			}
		}
	}

	// Create a subscription and remember to remove it when done:
	subId := uuid.NewString()
	logger := s.logger.With(
		slog.String("subscription", subId),
	)
	subInfo := eventsServerSubInfo{
		stream:     stream,
		filterSrc:  filterSrc,
		filterPrg:  filterPrg,
		eventsChan: make(chan *privatev1.Event),
	}
	s.subsLock.Lock()
	s.subs[subId] = subInfo
	s.subsLock.Unlock()
	logger.DebugContext(ctx, "Created subcription")
	defer func() {
		s.subsLock.Lock()
		defer s.subsLock.Unlock()
		delete(s.subs, subId)
		close(subInfo.eventsChan)
		logger.DebugContext(ctx, "Canceled subcription")
	}()

	// Wait to receive events on the channel of the subscription and forward them to the client:
	for {
		select {
		case event, ok := <-subInfo.eventsChan:
			if !ok {
				logger.DebugContext(ctx, "Subscription channel closed")
				return nil
			}
			err = stream.Send(&privatev1.EventsWatchResponse{
				Event: event,
			})
			if err != nil {
				return err
			}
		case <-stream.Context().Done():
			s.logger.DebugContext(ctx, "Subscription context canceled")
			return nil
		}
	}
}

func (s *PrivateEventsServer) compileFilter(ctx context.Context, filterSrc string) (result cel.Program, err error) {
	tree, issues := s.celEnv.Compile(filterSrc)
	err = issues.Err()
	if err != nil {
		return
	}
	result, err = s.celEnv.Program(tree)
	return
}

func (s *PrivateEventsServer) evalFilter(ctx context.Context, filterPrg cel.Program, event *privatev1.Event) (result bool,
	err error) {
	activation, err := cel.NewActivation(map[string]any{
		"event": event,
	})
	if err != nil {
		return
	}
	value, _, err := filterPrg.ContextEval(ctx, activation)
	if err != nil {
		return
	}
	result, ok := value.Value().(bool)
	if !ok {
		err = fmt.Errorf("result of filter should be a boolean, but it is of type '%T'", result)
		return
	}
	return
}

func (s *PrivateEventsServer) processPayload(ctx context.Context, payload proto.Message) error {
	event, ok := payload.(*privatev1.Event)
	if !ok {
		s.logger.ErrorContext(
			ctx,
			"Unexpected payload type",
			slog.String("expected", fmt.Sprintf("%T", event)),
			slog.String("actual", fmt.Sprintf("%T", payload)),
		)
		return nil
	}
	return s.processEvent(ctx, event)
}

func (s *PrivateEventsServer) processEvent(ctx context.Context, event *privatev1.Event) error {
	s.subsLock.RLock()
	defer s.subsLock.RUnlock()
	for subId, sub := range s.subs {
		logger := s.logger.With(
			slog.String("filter", sub.filterSrc),
			slog.String("sub", subId),
			slog.Any("event", event),
		)
		accepted := true
		if sub.filterPrg != nil {
			var err error
			accepted, err = s.evalFilter(ctx, sub.filterPrg, event)
			if err != nil {
				logger.DebugContext(
					ctx,
					"Failed to evaluate filter",
					slog.Any("error", err),
				)
				accepted = false
			}
		}
		if accepted {
			logger.DebugContext(ctx, "Event accepted by filter")
			sub.eventsChan <- event
		} else {
			s.logger.DebugContext(ctx, "Event rejected by filter")
		}
	}
	return nil
}

// Names of the packages whose enums will be available in the filter expressions:
var privateEventsServerPackages = map[protoreflect.FullName]bool{
	"private.v1": true,
}
