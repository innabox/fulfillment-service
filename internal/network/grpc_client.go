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
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	experiementalcredentials "google.golang.org/grpc/experimental/credentials"
)

// GrpcClientBuilder contains the data and logic needed to create a gRPC client. Don't create instances of this object
// directly, use the NewClient function instead.
type GrpcClientBuilder struct {
	logger          *slog.Logger
	serverNetwork   string
	serverAddress   string
	serverPlaintext bool
	serverInsecure  bool
	caFiles         []string
	token           string
	tokenFile       string
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

	// Token:
	flag = grpcClientFlagName(name, grpcClientTokenFlagSuffix)
	tokenValue, err := flags.GetString(flag)
	if err != nil {
		failure()
	} else {
		b.SetToken(tokenValue)
	}

	// Token file:
	flag = grpcClientFlagName(name, grpcClientTokenFileFlagSuffix)
	tokenFileValue, err := flags.GetString(flag)
	if err != nil {
		failure()
	} else {
		b.SetTokenFile(tokenFileValue)
	}

	// CA file:
	flag = grpcClientFlagName(name, grpcClientCaFileFlagSuffix)
	caFileValues, err := flags.GetStringArray(flag)
	if err != nil {
		failure()
	} else {
		for _, caFileValue := range caFileValues {
			b.AddCaFile(caFileValue)
		}
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

// AddCaFile adds a file containing CA certificates trusted by the client. This is optional, by default all the CAs
// trusted by the system are also trusted by the client.
func (b *GrpcClientBuilder) AddCaFile(value string) *GrpcClientBuilder {
	b.caFiles = append(b.caFiles, value)
	return b
}

// SetToken sets the token that the client will use to authenticate to the server. This is optional, by default no
// authentication credentials are sent.
//
// Note that this is incompatible with SetTokenFile.
func (b *GrpcClientBuilder) SetToken(value string) *GrpcClientBuilder {
	b.token = value
	return b
}

// SetTokenFile sets the path of the file containing the token that the client will use to authenticate to the server.
// This is optional, by default no authentication credentials are sent.
//
// Note that this is incompatible with SetToken.
func (b *GrpcClientBuilder) SetTokenFile(value string) *GrpcClientBuilder {
	b.tokenFile = value
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
	if b.token != "" && b.tokenFile != "" {
		err = errors.New("token and token file are incompatible")
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

	// Set the TLS options:
	var options []grpc.DialOption
	var transportCredentials credentials.TransportCredentials
	if b.serverPlaintext {
		transportCredentials = insecure.NewCredentials()
	} else {
		tlsConfig := &tls.Config{}
		if b.serverInsecure {
			tlsConfig.InsecureSkipVerify = true
		}
		if len(b.caFiles) > 0 {
			var caPool *x509.CertPool
			caPool, err = x509.SystemCertPool()
			if err != nil {
				return
			}
			caPool = caPool.Clone()
			for _, caFile := range b.caFiles {
				var data []byte
				data, err = os.ReadFile(caFile)
				if err != nil {
					err = fmt.Errorf("failed to read CA file '%s': %w", caFile, err)
					return
				}
				ok := caPool.AppendCertsFromPEM(data)
				if !ok {
					err = fmt.Errorf("file '%s' doesn't contain any CA certificate", caFile)
					return
				}
				b.logger.Debug(
					"Loaded CA file",
					slog.String("file", caFile),
				)
			}
			tlsConfig.RootCAs = caPool
		}

		// TODO: This should have been the non-experimental package, but we need to use this one because
		// currently the OpenShift router doesn't seem to support ALPN, and the regular credentials package
		// requires it since version 1.67. See here for details:
		//
		// https://github.com/grpc/grpc-go/issues/434
		// https://github.com/grpc/grpc-go/pull/7980
		//
		// Is there a way to configure the OpenShift router to avoid this?
		transportCredentials = experiementalcredentials.NewTLSWithALPNDisabled(tlsConfig)
	}
	if transportCredentials != nil {
		options = append(options, grpc.WithTransportCredentials(transportCredentials))
	}

	// Set the authentication options:
	token := b.token
	if token == "" && b.tokenFile != "" {
		var data []byte
		data, err = os.ReadFile(b.tokenFile)
		if err != nil {
			err = fmt.Errorf("failed to read token from file '%s': %w", b.tokenFile, err)
			return
		}
		token = strings.TrimSpace(string(data))
	}
	if token != "" {
		oauthToken := &oauth2.Token{
			AccessToken: token,
		}
		oauthSource := oauth.TokenSource{
			TokenSource: oauth2.StaticTokenSource(oauthToken),
		}
		options = append(options, grpc.WithPerRPCCredentials(oauthSource))
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
