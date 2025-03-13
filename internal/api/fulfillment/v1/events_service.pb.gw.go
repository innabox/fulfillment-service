// Code generated by protoc-gen-grpc-gateway. DO NOT EDIT.
// source: fulfillment/v1/events_service.proto

/*
Package fulfillmentv1 is a reverse proxy.

It translates gRPC into RESTful JSON APIs.
*/
package fulfillmentv1

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Suppress "imported and not used" errors
var (
	_ codes.Code
	_ io.Reader
	_ status.Status
	_ = errors.New
	_ = runtime.String
	_ = utilities.NewDoubleArray
	_ = metadata.Join
)

var filter_Events_Watch_0 = &utilities.DoubleArray{Encoding: map[string]int{}, Base: []int(nil), Check: []int(nil)}

func request_Events_Watch_0(ctx context.Context, marshaler runtime.Marshaler, client EventsClient, req *http.Request, pathParams map[string]string) (Events_WatchClient, runtime.ServerMetadata, error) {
	var (
		protoReq EventsWatchRequest
		metadata runtime.ServerMetadata
	)
	if err := req.ParseForm(); err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := runtime.PopulateQueryParameters(&protoReq, req.Form, filter_Events_Watch_0); err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	stream, err := client.Watch(ctx, &protoReq)
	if err != nil {
		return nil, metadata, err
	}
	header, err := stream.Header()
	if err != nil {
		return nil, metadata, err
	}
	metadata.HeaderMD = header
	return stream, metadata, nil
}

// RegisterEventsHandlerServer registers the http handlers for service Events to "mux".
// UnaryRPC     :call EventsServer directly.
// StreamingRPC :currently unsupported pending https://github.com/grpc/grpc-go/issues/906.
// Note that using this registration option will cause many gRPC library features to stop working. Consider using RegisterEventsHandlerFromEndpoint instead.
// GRPC interceptors will not work for this type of registration. To use interceptors, you must use the "runtime.WithMiddlewares" option in the "runtime.NewServeMux" call.
func RegisterEventsHandlerServer(ctx context.Context, mux *runtime.ServeMux, server EventsServer) error {
	mux.Handle(http.MethodGet, pattern_Events_Watch_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		err := status.Error(codes.Unimplemented, "streaming calls are not yet supported in the in-process transport")
		_, outboundMarshaler := runtime.MarshalerForRequest(mux, req)
		runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
		return
	})

	return nil
}

// RegisterEventsHandlerFromEndpoint is same as RegisterEventsHandler but
// automatically dials to "endpoint" and closes the connection when "ctx" gets done.
func RegisterEventsHandlerFromEndpoint(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if cerr := conn.Close(); cerr != nil {
				grpclog.Errorf("Failed to close conn to %s: %v", endpoint, cerr)
			}
			return
		}
		go func() {
			<-ctx.Done()
			if cerr := conn.Close(); cerr != nil {
				grpclog.Errorf("Failed to close conn to %s: %v", endpoint, cerr)
			}
		}()
	}()
	return RegisterEventsHandler(ctx, mux, conn)
}

// RegisterEventsHandler registers the http handlers for service Events to "mux".
// The handlers forward requests to the grpc endpoint over "conn".
func RegisterEventsHandler(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	return RegisterEventsHandlerClient(ctx, mux, NewEventsClient(conn))
}

// RegisterEventsHandlerClient registers the http handlers for service Events
// to "mux". The handlers forward requests to the grpc endpoint over the given implementation of "EventsClient".
// Note: the gRPC framework executes interceptors within the gRPC handler. If the passed in "EventsClient"
// doesn't go through the normal gRPC flow (creating a gRPC client etc.) then it will be up to the passed in
// "EventsClient" to call the correct interceptors. This client ignores the HTTP middlewares.
func RegisterEventsHandlerClient(ctx context.Context, mux *runtime.ServeMux, client EventsClient) error {
	mux.Handle(http.MethodGet, pattern_Events_Watch_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		inboundMarshaler, outboundMarshaler := runtime.MarshalerForRequest(mux, req)
		annotatedContext, err := runtime.AnnotateContext(ctx, mux, req, "/fulfillment.v1.Events/Watch", runtime.WithHTTPPathPattern("/api/fulfillment/v1/events"))
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}
		resp, md, err := request_Events_Watch_0(annotatedContext, inboundMarshaler, client, req, pathParams)
		annotatedContext = runtime.NewServerMetadataContext(annotatedContext, md)
		if err != nil {
			runtime.HTTPError(annotatedContext, mux, outboundMarshaler, w, req, err)
			return
		}
		forward_Events_Watch_0(annotatedContext, mux, outboundMarshaler, w, req, func() (proto.Message, error) { return resp.Recv() }, mux.GetForwardResponseOptions()...)
	})
	return nil
}

var (
	pattern_Events_Watch_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1, 2, 2, 2, 3}, []string{"api", "fulfillment", "v1", "events"}, ""))
)

var (
	forward_Events_Watch_0 = runtime.ForwardResponseStream
)
