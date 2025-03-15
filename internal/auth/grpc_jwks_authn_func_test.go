/*
Copyright (c) 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package auth

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/ghttp"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"

	. "github.com/innabox/fulfillment-service/internal/testing"
)

var _ = Describe("gRPC JWKS authentication function", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Building", func() {
		It("Can't be built without a logger", func() {
			_, err := NewGrpcJwksAuthnFunc().
				AddKeysFile(keysFile).
				Build()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("logger"))
			Expect(err.Error()).To(ContainSubstring("mandatory"))
		})

		It("Can't be built with a keys file that doesn't exist", func() {
			_, err := NewGrpcJwksAuthnFunc().
				SetLogger(logger).
				AddKeysFile("/does/not/exist").
				Build()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("/does/not/exist"))
		})

		It("Can't be built with a malformed keys URL", func() {
			_, err := NewGrpcJwksAuthnFunc().
				SetLogger(logger).
				AddKeysUrl("junk").
				Build()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("junk"))
		})

		It("Can't be built with a URL that isn't HTTPS", func() {
			_, err := NewGrpcJwksAuthnFunc().
				SetLogger(logger).
				AddKeysUrl("http://example.com/.well-known/jwks.json").
				Build()
			Expect(err).To(HaveOccurred())
		})

		It("Can be built with one keys file", func() {
			_, err := NewGrpcJwksAuthnFunc().
				SetLogger(logger).
				AddKeysFile(keysFile).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Can be built with one keys URL", func() {
			_, err := NewGrpcJwksAuthnFunc().
				SetLogger(logger).
				AddKeysUrl("https://example.com/.well-known/jwks.json").
				Build()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Token validation", func() {
		var function GrpcAuthnFunc

		BeforeEach(func() {
			var err error
			function, err = NewGrpcJwksAuthnFunc().
				SetLogger(logger).
				AddKeysFile(keysFile).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		It("Rejects request without authorization header for private method", func() {
			ctx, err := function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			Expect(ctx).To(BeNil())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Method '/my_package/MyMethod' requires authentication"))
		})

		It("Rejects bad authorization type", func() {
			bearer := MakeTokenString("Bearer", 1*time.Minute)
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bad "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Authentication type 'Bad' isn't supported"))
		})

		It("Rejects bad bearer token", func() {
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer bad",
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token is malformed"))
		})

		It("Rejects expired bearer token", func() {
			bearer := MakeTokenString("Bearer", -1*time.Hour)
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token is expired"))
		})

		It("Accepts token without `typ` claim", func() {
			bearer := MakeTokenObject(jwt.MapClaims{
				"typ": nil,
			}).Raw
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).ToNot(HaveOccurred())
		})

		It("Rejects `typ` claim with incorrect type", func() {
			bearer := MakeTokenObject(jwt.MapClaims{
				"typ": 123,
			}).Raw
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token type claim contains incorrect string value '123'"))
		})

		It("Rejects refresh tokens", func() {
			bearer := MakeTokenString("Refresh", 1*time.Hour)
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token type 'Refresh' isn't allowed"))
		})

		It("Rejects offline tokens", func() {
			bearer := MakeTokenString("Offline", 0)
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token type 'Offline' isn't allowed"))
		})

		It("Rejects token without issue date", func() {
			token := MakeTokenObject(jwt.MapClaims{
				"typ": "Bearer",
				"iat": nil,
			})
			bearer := token.Raw
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token doesn't contain required claim 'iat'"))
		})

		It("Rejects token without expiration date", func() {
			token := MakeTokenObject(jwt.MapClaims{
				"typ": "Bearer",
				"exp": nil,
			})
			bearer := token.Raw
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token doesn't contain required claim 'exp'"))
		})

		It("Rejects token issued in the future", func() {
			now := time.Now()
			iat := now.Add(1 * time.Minute)
			exp := iat.Add(1 * time.Minute)
			token := MakeTokenObject(jwt.MapClaims{
				"typ": "Bearer",
				"iat": iat.Unix(),
				"exp": exp.Unix(),
			})
			bearer := token.Raw
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token was issued in the future"))
		})

		It("Rejects token that isn't valid yet", func() {
			iat := time.Now()
			nbf := iat.Add(1 * time.Minute)
			exp := nbf.Add(1 * time.Minute)
			token := MakeTokenObject(jwt.MapClaims{
				"typ": "Bearer",
				"iat": iat.Unix(),
				"nbf": nbf.Unix(),
				"exp": exp.Unix(),
			})
			bearer := token.Raw
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
				Authorization, "Bearer "+bearer,
			))
			var err error
			ctx, err = function(ctx, "/my_package/MyMethod")
			Expect(err).To(HaveOccurred())
			status, ok := grpcstatus.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
			Expect(status.Message()).To(Equal("Bearer token isn't valid yet"))
		})
	})

	It("Loads keys from file", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify that the token is accepted:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Adds token to the context", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify that subject contains the token:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).ToNot(BeNil())
			Expect(subject.Token).To(Equal(bearer))
		}).ToNot(Panic())
	})

	It("Adds subject to the context", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify context contains the subject name:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).ToNot(BeNil())
			Expect(subject.Name).To(Equal("mysubject"))
		}).ToNot(Panic())
	})

	It("Doesn't require authorization header for public method", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			AddPublicMethodRegex("^/my_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the response:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs())
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Accepts malformed authorization header for public method", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			AddPublicMethodRegex("^/my_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the response:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bad junk",
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).To(BeIdenticalTo(Guest))
		}).ToNot(Panic())
	})

	It("Ignores expired token for public URL", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			AddPublicMethodRegex("^/my_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the response:
		bearer := MakeTokenString("Bearer", -1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).To(BeIdenticalTo(Guest))
		}).ToNot(Panic())
	})

	It("Combines multiple public methods", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			AddPublicMethodRegex("^/my_package/.*$").
			AddPublicMethodRegex("^/your_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the results:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs())
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		ctx, err = function(ctx, "/your_package/YourMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Adds authenticated subject to the context for public methods", func() {
		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			AddPublicMethodRegex("^/my_package/.*$").
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the results:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(ctx).ToNot(BeNil())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).ToNot(BeNil())
			Expect(subject.Name).To(Equal("mysubject"))
		}).ToNot(Panic())
	})

	It("Doesn't load insecure keys by default", func() {
		// Prepare the keys server:
		server, ca := MakeTCPTLSServer()
		defer func() {
			server.Close()
			err := os.Remove(ca)
			Expect(err).ToNot(HaveOccurred())
		}()
		server.AppendHandlers(
			RespondWith(http.StatusOK, keysBytes),
		)
		server.SetAllowUnhandledRequests(true)

		// Create the funtion:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysUrl(server.URL()).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify that result:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(HaveOccurred())
		Expect(ctx).To(BeNil())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
		Expect(status.Message()).To(Equal("Bearer token can't be verified"))
	})

	It("Loads insecure keys in insecure mode", func() {
		// Prepare the keys server:
		server, ca := MakeTCPTLSServer()
		defer func() {
			server.Close()
			err := os.Remove(ca)
			Expect(err).ToNot(HaveOccurred())
		}()
		server.AppendHandlers(
			RespondWith(http.StatusOK, keysBytes),
		)
		server.SetAllowUnhandledRequests(true)

		// Create the funtion:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysUrl(server.URL()).
			SetKeysInsecure(true).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify that result:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Accepts token expired within the configured tolerance", func() {
		// Create a function tolerates tokens that expired up to 10 minutes ago:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			SetTolerance(10 * time.Minute).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result with a tokent that expired 5 minutes ago:
		bearer := MakeTokenString("Bearer", -5*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Rejects token expired outside the configured tolerance", func() {
		// Create a function tolerates tokens that expired up to 10 minutes ago:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysFile(keysFile).
			SetTolerance(10 * time.Minute).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result with a tokent that expired 15 minutes ago:
		bearer := MakeTokenString("Bearer", -15*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(HaveOccurred())
		Expect(ctx).To(BeNil())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
		Expect(status.Message()).To(Equal("Bearer token is expired"))
	})

	It("Accepts all requests if no keys are configured", func() {
		// Prepare a function that has no keys, so will not perform any authentication and accept all requests:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the response:
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs())
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
		Expect(func() {
			subject := SubjectFromContext(ctx)
			Expect(subject).To(BeIdenticalTo(Guest))
		}).ToNot(Panic())
	})

	It("Uses token to load keys", func() {
		// Prepare the server that will return the keys:
		server, ca := MakeTCPTLSServer()
		defer func() {
			server.Close()
			err := os.Remove(ca)
			Expect(err).ToNot(HaveOccurred())
		}()
		server.AppendHandlers(CombineHandlers(
			VerifyHeaderKV("Authorization", "Bearer mykeystoken"),
			RespondWith(http.StatusOK, keysBytes),
		))

		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysUrl(server.URL()).
			SetKeysToken("mykeystoken").
			SetKeysInsecure(true).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the response:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Uses token file to load keys", func() {
		// Prepare the server that will return the keys:
		server, ca := MakeTCPTLSServer()
		defer func() {
			server.Close()
			err := os.Remove(ca)
			Expect(err).ToNot(HaveOccurred())
		}()
		server.AppendHandlers(CombineHandlers(
			VerifyHeaderKV("Authorization", "Bearer mykeystoken"),
			RespondWith(http.StatusOK, keysBytes),
		))

		// Prepare the token file:
		fd, err := os.CreateTemp("", "*.token")
		Expect(err).ToNot(HaveOccurred())
		file := fd.Name()
		defer func() {
			err := os.Remove(file)
			Expect(err).ToNot(HaveOccurred())
		}()
		err = fd.Close()
		Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(file, []byte("mykeystoken"), 0600)
		Expect(err).ToNot(HaveOccurred())

		// Create the funtion:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysUrl(server.URL()).
			SetKeysTokenFile(file).
			SetKeysInsecure(true).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Uses token to load keys if token file doesn't exist", func() {
		// Prepare the server that will return the keys:
		server, ca := MakeTCPTLSServer()
		defer func() {
			server.Close()
			err := os.Remove(ca)
			Expect(err).ToNot(HaveOccurred())
		}()
		server.AppendHandlers(CombineHandlers(
			VerifyHeaderKV("Authorization", "Bearer mykeystoken"),
			RespondWith(http.StatusOK, keysBytes),
		))

		// Prepare a token file that doesn't exist:
		dir, err := os.MkdirTemp("", "*.tokens")
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(dir)
			Expect(err).ToNot(HaveOccurred())
		}()
		file := filepath.Join(dir, "bad.token")

		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysUrl(server.URL()).
			SetKeysToken("mykeystoken").
			SetKeysTokenFile(file).
			SetKeysInsecure(true).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify that the restul:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).ToNot(HaveOccurred())
	})

	It("Rejects if token is required for loading keys and none has been configured", func() {
		// Prepare the server that will return the keys:
		server, ca := MakeTCPTLSServer()
		defer func() {
			server.Close()
			err := os.Remove(ca)
			Expect(err).ToNot(HaveOccurred())
		}()
		server.AppendHandlers(CombineHandlers(
			RespondWith(http.StatusUnauthorized, nil),
		))

		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysUrl(server.URL()).
			SetKeysInsecure(true).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(HaveOccurred())
		Expect(ctx).To(BeNil())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
		Expect(status.Message()).To(Equal("Bearer token can't be verified"))
	})

	It("Rejects if token is required for loading keys and token file doesn't exist", func() {
		// Prepare the server that will return the keys:
		server, ca := MakeTCPTLSServer()
		defer func() {
			server.Close()
			err := os.Remove(ca)
			Expect(err).ToNot(HaveOccurred())
		}()
		server.AppendHandlers(
			RespondWith(http.StatusUnauthorized, nil),
		)

		// Prepare a token file that doesn't exist:
		dir, err := os.MkdirTemp("", "*.tokens")
		Expect(err).ToNot(HaveOccurred())
		defer func() {
			err := os.RemoveAll(dir)
			Expect(err).ToNot(HaveOccurred())
		}()
		file := filepath.Join(dir, "bad.token")

		// Create the function:
		function, err := NewGrpcJwksAuthnFunc().
			SetLogger(logger).
			AddKeysUrl(server.URL()).
			SetKeysInsecure(true).
			SetKeysTokenFile(file).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Verify the result:
		bearer := MakeTokenString("Bearer", 1*time.Minute)
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			Authorization, "Bearer "+bearer,
		))
		ctx, err = function(ctx, "/my_package/MyMethod")
		Expect(err).To(HaveOccurred())
		Expect(ctx).To(BeNil())
		status, ok := grpcstatus.FromError(err)
		Expect(ok).To(BeTrue())
		Expect(status.Code()).To(Equal(grpccodes.Unauthenticated))
		Expect(status.Message()).To(Equal("Bearer token can't be verified"))
	})
})
