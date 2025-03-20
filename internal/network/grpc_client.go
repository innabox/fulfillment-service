/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package network

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// GrpcClientBuilder contains the data and logic needed to create a gRPC client. Don't create instances of this object
// directly, use the NewClient function instead.
type GrpcClientBuilder struct {
	logger          *slog.Logger
	serverNetwork   string
	serverAddress   string
	serverPlaintext bool
	serverInsecure  bool
}

// NewClient creates a builder that can then used to configure and create a gRPC client.
func NewClient() *GrpcClientBuilder {
	return &GrpcClientBuilder{}
}

// SetLogger sets the logger that the client will use to send messages to the log. This is mandatory.
func (b *GrpcClientBuilder) SetLogger(value *slog.Logger) *GrpcClientBuilder {
	b.logger = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the client.
//
// The name is used to select the options when there are multiple clients. For example, if it is 'API' then it will only
// take into accounts the flags starting with '--api'.
//
// This is optional.
func (b *GrpcClientBuilder) SetFlags(flags *pflag.FlagSet, name string) *GrpcClientBuilder {
	if flags == nil {
		return b
	}

	var (
		flag string
		err  error
	)
	failure := func() {
		b.logger.Error(
			"Failed to get flag value",
			slog.String("flag", flag),
			slog.Any("error", err),
		)
	}

	// Server network:
	flag = grpcClientFlagName(name, grpcClientServerNetworkFlagSuffix)
	serverNetworkValue, err := flags.GetString(flag)
	if err != nil {
		failure()
	} else {
		b.SetServerNetwork(serverNetworkValue)
	}

	// Server address:
	flag = grpcClientFlagName(name, grpcClientServerAddrFlagSuffix)
	serverAddrValue, err := flags.GetString(flag)
	if err != nil {
		failure()
	} else {
		b.SetServerAddress(serverAddrValue)
	}

	// Server plaintext:
	flag = grpcClientFlagName(name, grpcClientServerPlaintextFlagSuffix)
	serverPlaintextValue, err := flags.GetBool(flag)
	if err != nil {
		failure()
	} else {
		b.SetServerPlaintext(serverPlaintextValue)
	}

	// Server insecure:
	flag = grpcClientFlagName(name, grpcClientServerInsecureFlagSuffix)
	serverInsecureValue, err := flags.GetBool(flag)
	if err != nil {
		failure()
	} else {
		b.SetServerInsecure(serverInsecureValue)
	}

	return b
}

// SetServerNetwork sets the server network.
func (b *GrpcClientBuilder) SetServerNetwork(value string) *GrpcClientBuilder {
	b.serverNetwork = value
	return b
}

// SetServerAddress sets the server address.
func (b *GrpcClientBuilder) SetServerAddress(value string) *GrpcClientBuilder {
	b.serverAddress = value
	return b
}

// SetServerPlaintext when set to true configures the client for a server that doesn't use TLS. The default is false.
func (b *GrpcClientBuilder) SetServerPlaintext(value bool) *GrpcClientBuilder {
	b.serverPlaintext = value
	return b
}

// SetServerInsecure when set to true configures the client for use TLS but to not verify the certificate presented
// by the server. This shouldn't be used in production environments. The default is false.
func (b *GrpcClientBuilder) SetServerInsecure(value bool) *GrpcClientBuilder {
	b.serverInsecure = value
	return b
}

// Build uses the data stored in the builder to create a new network client.
func (b *GrpcClientBuilder) Build() (result *grpc.ClientConn, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.serverNetwork == "" {
		err = errors.New("server network is mandatory")
		return
	}
	if b.serverAddress == "" {
		err = errors.New("server address is mandatory")
		return
	}

	// Calculate the endpoint:
	var endpoint string
	switch b.serverNetwork {
	case "tcp":
		endpoint = fmt.Sprintf("dns:///%s", b.serverAddress)
	case "unix":
		if filepath.IsAbs(b.serverAddress) {
			endpoint = fmt.Sprintf("unix://%s", b.serverAddress)
		} else {
			endpoint = fmt.Sprintf("unix:%s", b.serverAddress)
		}
	default:
		err = fmt.Errorf("unknown network '%s'", b.serverNetwork)
		return
	}

	// Calculate the options:
	var options []grpc.DialOption
	var transportCredentials credentials.TransportCredentials
	if b.serverPlaintext {
		transportCredentials = insecure.NewCredentials()
	} else {
		tlsConfig := &tls.Config{}
		if b.serverInsecure {
			tlsConfig.InsecureSkipVerify = true
		}
		transportCredentials = credentials.NewTLS(tlsConfig)
	}
	if transportCredentials != nil {
		options = append(options, grpc.WithTransportCredentials(transportCredentials))
	}

	// Create the client:
	result, err = grpc.NewClient(endpoint, options...)
	return
}

// Common client names:
const (
	GrpcClientName = "gRPC"
	HttpClientName = "HTTP"
)
