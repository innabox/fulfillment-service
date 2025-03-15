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
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/pflag"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
)

// GrpcJwksAuthnType is the name of the JWKS authentication function.
const GrpcJwksAuthnType = "jwks"

// GrpcJwksAuthnFuncBuilder is a gRPC authentication function that expects to receive a JSON web token in the
// authorization header and checks its signature using one or more JSON web key sets.
type GrpcJwksAuthnFuncBuilder struct {
	logger        *slog.Logger
	publicMethods []string
	keysFiles     []string
	keysUrls      []string
	keysCa        *x509.CertPool
	keysCaFile    string
	keysInsecure  bool
	keysToken     string
	keysTokenFile string
	tolerance     time.Duration
}

// grpcJwksAuthnFunc contains the data needed by the function.
type grpcJwksAuthnFunc struct {
	logger        *slog.Logger
	publicMethods []*regexp.Regexp
	tokenParser   *jwt.Parser
	keysFiles     []string
	keysURLs      []string
	keysToken     string
	keysTokenFile string
	keysClient    *http.Client
	keys          *sync.Map
	lastRefresh   time.Time
	refreshLock   *sync.Mutex
}

// NewGrpcJwksAuthnFunc creates a builder that can then be used to configure and create a new gRPMC authentication
// function.
func NewGrpcJwksAuthnFunc() *GrpcJwksAuthnFuncBuilder {
	return &GrpcJwksAuthnFuncBuilder{}
}

// SetLogger sets the logger that will be used to write to the log. This is mandatory.
func (b *GrpcJwksAuthnFuncBuilder) SetLogger(value *slog.Logger) *GrpcJwksAuthnFuncBuilder {
	b.logger = value
	return b
}

// AddPublicMethodRegex adds a regular expression that describes a sets of methods that are considered public, and
// therefore require no authentication. The regular expression will be matched against to the full gRPC method name,
// including the leading slash. For example, to consider public all the methods of the `example.v1.Products` service
// the regular expression could be `^/example\.v1\.Products/.*$`.
//
// This method may be called multiple times to add multiple regular expressions. A method will be considered public if
// it matches at least one of them.
func (b *GrpcJwksAuthnFuncBuilder) AddPublicMethodRegex(value string) *GrpcJwksAuthnFuncBuilder {
	b.publicMethods = append(b.publicMethods, value)
	return b
}

// AddKeysFile adds a file containing a JSON web key set that will be used to verify the signatures of the tokens. The
// keys from this file will be loaded when a received token contains an unknown key identifier.
//
// If no keys file or URL are provided then all requests will be accepted and the guest subject will be added to the
// context.
func (b *GrpcJwksAuthnFuncBuilder) AddKeysFile(value string) *GrpcJwksAuthnFuncBuilder {
	if value != "" {
		b.keysFiles = append(b.keysFiles, value)
	}
	return b
}

// AddKeysUrl sets the URL containing a JSON web key set that will be used to verify the signatures of the tokens. The
// keys from these URLs will be loaded when a received token contains an unknown key identifier.
//
// If no keys file or URL are provided then all requests will be accepted and the guest subject Will be added to the
// context.
func (b *GrpcJwksAuthnFuncBuilder) AddKeysUrl(value string) *GrpcJwksAuthnFuncBuilder {
	if value != "" {
		b.keysUrls = append(b.keysUrls, value)
	}
	return b
}

// SetKeysCa sets the certificate authorities that will be trusted when verifying the certificate of the web server
// where keys are loaded from.
func (b *GrpcJwksAuthnFuncBuilder) SetKeysCa(value *x509.CertPool) *GrpcJwksAuthnFuncBuilder {
	b.keysCa = value
	return b
}

// SetKeysCaFile sets the file containing the certificates of the certificate authorities that will be trusted when
// verifying the certificate of the web server where keys are loaded from.
func (b *GrpcJwksAuthnFuncBuilder) SetKeysCaFile(value string) *GrpcJwksAuthnFuncBuilder {
	b.keysCaFile = value
	return b
}

// SetKeysTokenFile sets the name of the file containing the bearer token that will be used in the HTTP requests to
// download JSON web key sets.
//
// This is intended for use when running inside a Kubernetes cluster and using service account tokens for
// authentication. In that case it is convenient to set this to the following value:
//
//	/run/secrets/kubernetes.io/serviceaccount/token
//
// Kubernetes writes in that file the token of the service account of the pod. That token grants access to the following
// JSON web key set URL, which should be set using the AddKeysUrl method:
//
//	https://kubernetes.default.svc/openid/v1/jwks
//
// The Kubernetes CA also needs to be set with the SetKeysCaFile to the following value:
//
//	/run/secrets/kubernetes.io/serviceaccount/ca.crt
//
// That can also be done with the equivalent command line flags:
//
//   - --grpc-authn-jwks-url=https://kubernetes.default.svc/openid/v1/jwks
//   - --grpc-authn-jwks-ca-file=/run/secrets/kubernetes.io/serviceaccount/ca.crt
//   - --grpc-authn-jwks-token-file=/run/secrets/kubernetes.io/serviceaccount/token
//
// This is optional, by default no token file is used.
func (b *GrpcJwksAuthnFuncBuilder) SetKeysTokenFile(value string) *GrpcJwksAuthnFuncBuilder {
	b.keysTokenFile = value
	return b
}

// SetTolerance sets the maximum time that a token will be considered valid after it has expired. For example, to accept
// requests with tokens that have expired up to five minutes ago:
//
//	wrapper, err := auth.NewGrpcJwksAuthnFunc().
//	        SetLogger(logger).
//	        SetKeysUrl("https://...").
//	        SetTolerance(5 * time.Minute).
//	        Build()
//	if err != nil {
//	        ...
//	}
//
// The default value is zero tolerance.
func (b *GrpcJwksAuthnFuncBuilder) SetTolerance(value time.Duration) *GrpcJwksAuthnFuncBuilder {
	b.tolerance = value
	return b
}

// SetKeysInsecure sets the flag that indicates that the certificate of the web server where the keys are loaded from
// should not be checked. The default is false and changing it to true makes the token verification insecure, so refrain
// from doing that in security sensitive environments.
func (b *GrpcJwksAuthnFuncBuilder) SetKeysInsecure(value bool) *GrpcJwksAuthnFuncBuilder {
	b.keysInsecure = value
	return b
}

// SetKeysToken sets the bearer token that will be used in the HTTP requests to download JSON web key sets. This is
// optional, by default no token is used.
func (b *GrpcJwksAuthnFuncBuilder) SetKeysToken(value string) *GrpcJwksAuthnFuncBuilder {
	b.keysToken = value
	return b
}

// SetFlags sets the command line flags that should be used to configure the function. This is optional.
func (b *GrpcJwksAuthnFuncBuilder) SetFlags(flags *pflag.FlagSet) *GrpcJwksAuthnFuncBuilder {
	if flags.Changed(grpcJwksAuthnFileFlagName) {
		values, err := flags.GetStringArray(grpcJwksAuthnFileFlagName)
		if err == nil {
			for _, value := range values {
				b.AddKeysFile(value)
			}
		}
	}
	if flags.Changed(grpcJwksAutnUrlFlagName) {
		values, err := flags.GetStringArray(grpcJwksAutnUrlFlagName)
		if err == nil {
			for _, value := range values {
				b.AddKeysUrl(value)
			}
		}
	}
	if flags.Changed(grpcJwksAuthnTokenFlagName) {
		value, err := flags.GetString(grpcJwksAuthnTokenFlagName)
		if err == nil {
			b.SetKeysToken(value)
		}
	}
	if flags.Changed(grpcjwksAuthnTokenFileFlagName) {
		value, err := flags.GetString(grpcjwksAuthnTokenFileFlagName)
		if err == nil {
			b.SetKeysTokenFile(value)
		}
	}
	if flags.Changed(grpcJwksAuthnCaFileFlagName) {
		value, err := flags.GetString(grpcJwksAuthnCaFileFlagName)
		if err == nil {
			b.SetKeysCaFile(value)
		}
	}

	return b
}

// Build uses the data stored in the builder to create and configure a new gRPC JWKS authentication function.
func (b *GrpcJwksAuthnFuncBuilder) Build() (result GrpcAuthnFunc, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.tolerance < 0 {
		err = errors.New("tolerance must be zero or positive")
		return
	}

	// Check that all the configured keys files exist:
	for _, file := range b.keysFiles {
		var info os.FileInfo
		info, err = os.Stat(file)
		if err != nil {
			err = fmt.Errorf("keys file '%s' doesn't exist: %w", file, err)
			return
		}
		if !info.Mode().IsRegular() {
			err = fmt.Errorf("keys file '%s' isn't a regular file", file)
			return
		}
	}

	// Check that all the configured keys URLs are valid HTTPS URLs:
	for _, addr := range b.keysUrls {
		var parsed *url.URL
		parsed, err = url.Parse(addr)
		if err != nil {
			err = fmt.Errorf("keys URL '%s' isn't a valid URL: %w", addr, err)
			return
		}
		if !strings.EqualFold(parsed.Scheme, "https") {
			err = fmt.Errorf(
				"keys URL '%s' doesn't use the HTTPS protocol: %w",
				addr, err,
			)
			return
		}
	}

	// Load the keys CA file:
	var keysCA *x509.CertPool
	if b.keysCa != nil {
		keysCA = b.keysCa
	} else {
		keysCA, err = x509.SystemCertPool()
		if err != nil {
			return
		}
	}
	if b.keysCaFile != "" {
		var data []byte
		data, err = os.ReadFile(b.keysCaFile)
		if err != nil {
			err = fmt.Errorf("failed to read CA file '%s': %w", b.keysCaFile, err)
			return
		}
		keysCA = keysCA.Clone()
		ok := keysCA.AppendCertsFromPEM(data)
		if !ok {
			b.logger.Warn(
				"No certificate was loaded from CA file",
				slog.String("file", b.keysCaFile),
			)
		}
	}

	// Create the HTTP client that will be used to load the keys:
	keysClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            keysCA,
				InsecureSkipVerify: b.keysInsecure,
			},
		},
	}

	// Create the bearer token parser:
	tokenParser := jwt.NewParser(
		jwt.WithValidMethods([]string{
			"RS256",
		}),
		jwt.WithLeeway(b.tolerance),
		jwt.WithIssuedAt(),
	)

	// Make copies of the lists of keys files and URLs:
	keysFiles := slices.Clone(b.keysFiles)
	keysURLs := slices.Clone(b.keysUrls)

	// Create the initial empty map of keys:
	keys := &sync.Map{}

	// Try to compile the regular expressions that define the set of public methods:
	publicMethods := make([]*regexp.Regexp, len(b.publicMethods))
	for i, expr := range b.publicMethods {
		publicMethods[i], err = regexp.Compile(expr)
		if err != nil {
			return
		}
	}

	// Create and populate the object:
	object := &grpcJwksAuthnFunc{
		logger:        b.logger,
		publicMethods: publicMethods,
		tokenParser:   tokenParser,
		keysFiles:     keysFiles,
		keysURLs:      keysURLs,
		keysToken:     b.keysToken,
		keysTokenFile: b.keysTokenFile,
		keysClient:    keysClient,
		keys:          keys,
		refreshLock:   &sync.Mutex{},
	}
	result = object.call
	return
}

// call is the implementation of the `GrpcAuthnFunc` type.
func (f *grpcJwksAuthnFunc) call(ctx context.Context, method string) (result context.Context, err error) {
	values := metadata.ValueFromIncomingContext(ctx, Authorization)
	length := len(values)
	switch length {
	case 0:
		result, err = f.callWithoutAuth(ctx, method)
	case 1:
		result, err = f.callWithAuth(ctx, method, values[0])
	default:
		err = grpcstatus.Errorf(
			grpccodes.Unauthenticated,
			"Expected at most one 'authorization' header, but received %d",
			length,
		)
	}
	return
}

func (f *grpcJwksAuthnFunc) callWithAuth(ctx context.Context, method string, auth string) (result context.Context,
	err error) {
	// Check the authorization header. If that fails, for whatever the reason, we still want to allow access to
	// public methods, with the guest subject.
	token, err := f.checkAuth(ctx, auth)
	if err != nil {
		if f.isPublicMethod(method) {
			f.logger.InfoContext(
				ctx,
				"Authentication failed for public method",
				slog.String("method", method),
				slog.Any("error", err),
			)
			result = ContextWithSubject(ctx, Guest)
			err = nil
		}
		return
	}

	// Add the authenticated subject to the context:
	claims := token.Claims.(jwt.MapClaims)
	name := claims["sub"].(string)
	subject := &Subject{
		Token:  token.Raw,
		Name:   name,
		Claims: claims,
	}
	result = ContextWithSubject(ctx, subject)
	return
}

func (f *grpcJwksAuthnFunc) callWithoutAuth(ctx context.Context, method string) (result context.Context,
	err error) {
	// Check if the request is for a public method:
	isPublic := f.isPublicMethod(method)

	// Check if there are keys:
	haveKeys := len(f.keysFiles)+len(f.keysURLs) > 0

	// If the request is for a public method or there are no configured keys then we accept it and add the guest
	// subject to the context:
	if isPublic || !haveKeys {
		result = ContextWithSubject(ctx, Guest)
		return
	}

	// If the request isn't for a public method then we reject it:
	err = grpcstatus.Errorf(grpccodes.Unauthenticated, "Method '%s' requires authentication", method)
	return
}

func (f *grpcJwksAuthnFunc) isPublicMethod(method string) bool {
	for _, publicMethod := range f.publicMethods {
		if publicMethod.MatchString(method) {
			return true
		}
	}
	return false
}

// checkAuth checks if the given authorization header is valid.
func (f *grpcJwksAuthnFunc) checkAuth(ctx context.Context, auth string) (token *jwt.Token, err error) {
	// Try to extract the bearer token from the authorization header:
	matches := grpcJwksAuthFuncBearerRE.FindStringSubmatch(auth)
	if len(matches) != 3 {
		f.logger.ErrorContext(
			ctx,
			"Authorization header is malformed",
			slog.String("!authorization", auth),
		)
		err = grpcstatus.Errorf(grpccodes.Unauthenticated, "Authorization header is malformed")
		return
	}
	scheme := matches[1]
	if !strings.EqualFold(scheme, "Bearer") {
		err = grpcstatus.Errorf(grpccodes.Unauthenticated, "Authentication type '%s' isn't supported", scheme)
		return
	}
	bearer := matches[2]

	// Use the JWT library to verify that the token is correctly signed and that the basic claims are correct:
	token, err = f.checkToken(ctx, bearer)
	if err != nil {
		return
	}

	// The library that we use considers tokens valid if the claims that it checks don't exist, but we want to
	// reject those tokens, so we need to do some additional validations:
	err = f.checkClaims(token.Claims.(jwt.MapClaims))
	if err != nil {
		return
	}

	return
}

// selectKey selects the key that should be used to verify the given token.
func (f *grpcJwksAuthnFunc) selectKey(ctx context.Context, token *jwt.Token) (key any, err error) {
	// Get the key identifier:
	value, ok := token.Header["kid"]
	if !ok {
		err = fmt.Errorf("token doesn't have a 'kid' field in the header")
		return
	}
	kid, ok := value.(string)
	if !ok {
		err = fmt.Errorf(
			"token has a 'kid' field, but it is a %T instead of a string",
			value,
		)
		return
	}

	// Get the key for that key identifier. If there is no such key, refresh the keys and try again:
	key, ok = f.keys.Load(kid)
	if !ok {
		err = f.refreshKeys(ctx)
		if err != nil {
			err = fmt.Errorf(
				"failed to refresh keys in order to get key with identifier '%s': %w",
				kid, err,
			)
			return
		}
		key, ok = f.keys.Load(kid)
		if !ok {
			err = fmt.Errorf("there is no key for key identifier '%s'", kid)
			return
		}
	}

	return
}

// grpcJwksAuthFuncKeyData is the type used to read a single key from a JSON document.
type grpcJwksAuthFuncKeyData struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// SetData is the type used to read a collection of keys from a JSON document.
type grpcJwksAuthFuncSetData struct {
	Keys []grpcJwksAuthFuncKeyData `json:"keys"`
}

// refreshKeys reloads the JSON web key set, but only if it hasn't been loaded recently. This is to avoid repeatedly
// loading the keys when we receive tokens that reference keys that don't exist.
func (f *grpcJwksAuthnFunc) refreshKeys(ctx context.Context) error {
	f.refreshLock.Lock()
	defer f.refreshLock.Unlock()
	if time.Since(f.lastRefresh) > 1*time.Minute {
		err := f.loadKeys(ctx)
		if err != nil {
			return err
		}
		f.lastRefresh = time.Now()
	}
	return nil
}

// loadKeys loads the JSON web key set from the URLs specified in the configuration.
func (f *grpcJwksAuthnFunc) loadKeys(ctx context.Context) error {
	// Load keys from the files given in the configuration:
	for _, keysFile := range f.keysFiles {
		f.logger.InfoContext(
			ctx,
			"Loading keys from file", keysFile,
			slog.String("file", keysFile),
		)
		err := f.loadKeysFile(ctx, keysFile)
		if err != nil {
			f.logger.ErrorContext(
				ctx,
				"Failed load keys from file",
				slog.String("file", keysFile),
				slog.Any("error", err),
			)
		}
	}

	// Load keys from URLs given in the configuration:
	for _, keysURL := range f.keysURLs {
		f.logger.InfoContext(
			ctx,
			"Loading keys from URL",
			slog.String("url", keysURL),
		)
		err := f.loadKeysURL(ctx, keysURL)
		if err != nil {
			f.logger.ErrorContext(
				ctx,
				"Failed to load keys from URL",
				slog.String("url", keysURL),
				slog.Any("error", err),
			)
		}
	}

	return nil
}

// loadKeysFile loads a JSON web key set from a file.
func (f *grpcJwksAuthnFunc) loadKeysFile(ctx context.Context, file string) error {
	reader, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() {
		err := reader.Close()
		if err != nil {
			f.logger.ErrorContext(
				ctx,
				"Failed to close keys file",
				slog.String("file", file),
				slog.Any("error", err),
			)
		}
	}()
	return f.readKeys(ctx, reader)
}

// loadKeysURL loads a JSON we key set from an URL.
func (f *grpcJwksAuthnFunc) loadKeysURL(ctx context.Context, addr string) error {
	request, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return err
	}
	token := f.selectKeysToken(ctx)
	if token != "" {
		request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	request = request.WithContext(ctx)
	response, err := f.keysClient.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			f.logger.ErrorContext(
				ctx,
				"Failed to close response body",
				slog.String("url", addr),
				slog.Any("error", err),
			)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"request to load keys from '%s' failed with status code %d",
			addr, response.StatusCode,
		)
	}
	return f.readKeys(ctx, response.Body)
}

// selectKeysToken selects the token that should be for authentication to the server that contains the JSON web key set.
// Note that this will never return an error; if something fails it will report it in the log and will return an empty
// string.
func (f *grpcJwksAuthnFunc) selectKeysToken(ctx context.Context) string {
	// First try to read the token from the configured file:
	if f.keysTokenFile != "" {
		data, err := os.ReadFile(f.keysTokenFile)
		if err != nil {
			f.logger.ErrorContext(
				ctx,
				"Failed to read keys token from file",
				slog.String("file", f.keysTokenFile),
				slog.Any("error", err),
			)
		} else {
			return string(data)
		}
	}

	// If there is no token file or something failed while reading it then return the configured token (which may be
	// empty):
	return f.keysToken
}

// readKeys reads the keys from JSON web key set available in the given reader.
func (f *grpcJwksAuthnFunc) readKeys(ctx context.Context, reader io.Reader) error {
	// Read the JSON data:
	jsonData, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Parse the JSON data:
	var setData grpcJwksAuthFuncSetData
	err = json.Unmarshal(jsonData, &setData)
	if err != nil {
		return err
	}

	// Convert the key data to actual keys that can be used to verify the signatures of the tokens:
	for _, keyData := range setData.Keys {
		f.logger.DebugContext(
			ctx,
			"Key data",
			slog.String("kid", keyData.Kid),
			slog.String("kty", keyData.Kty),
			slog.String("alg", keyData.Alg),
			slog.String("e", keyData.E),
			slog.String("n", keyData.N),
		)
		if keyData.Kid == "" {
			f.logger.ErrorContext(ctx, "Can't read key because 'kid' is empty")
			continue
		}
		if keyData.Kty == "" {
			f.logger.ErrorContext(
				ctx,
				"Can't read key because 'kty' is empty",
				slog.String("kid", keyData.Kid),
			)
			continue
		}
		if keyData.Alg == "" {
			f.logger.ErrorContext(
				ctx,
				"Can't read key because 'alg' is empty",
				slog.String("kid", keyData.Kid),
			)
			continue
		}
		if keyData.E == "" {
			f.logger.ErrorContext(
				ctx,
				"Can't read key because 'e' is empty",
				slog.String("kid", keyData.Kid),
			)
			continue
		}
		if keyData.N == "" {
			f.logger.ErrorContext(
				ctx,
				"Can't read key because 'n' is empty",
				slog.String("kid", keyData.Kid),
			)
			continue
		}
		var key any
		key, err = f.parseKey(keyData)
		if err != nil {
			f.logger.ErrorContext(
				ctx,
				"Key will be ignored because it can't be parsed",
				slog.String("kid", keyData.Kid),
				slog.Any("error", err),
			)
			continue
		}
		f.keys.Store(keyData.Kid, key)
		f.logger.InfoContext(
			ctx,
			"Loaded key",
			slog.String("kid", keyData.Kid),
		)
	}

	return nil
}

// parseKey converts the key data loaded from the JSON document to an actual key that can be used to verify the
// signatures of tokens.
func (f *grpcJwksAuthnFunc) parseKey(data grpcJwksAuthFuncKeyData) (key any, err error) {
	// Check key type:
	if data.Kty != "RSA" {
		err = fmt.Errorf("key type '%s' isn't supported", data.Kty)
		return
	}

	// Decode the e and n values:
	nb, err := base64.RawURLEncoding.DecodeString(data.N)
	if err != nil {
		return
	}
	eb, err := base64.RawURLEncoding.DecodeString(data.E)
	if err != nil {
		return
	}

	// Create the key:
	key = &rsa.PublicKey{
		N: new(big.Int).SetBytes(nb),
		E: int(new(big.Int).SetBytes(eb).Int64()),
	}

	return
}

// checkToken checks if the token is valid. If it is valid it returns the parsed token.
func (f *grpcJwksAuthnFunc) checkToken(ctx context.Context, bearer string) (token *jwt.Token, err error) {
	token, err = f.tokenParser.ParseWithClaims(
		bearer, jwt.MapClaims{},
		func(token *jwt.Token) (key any, err error) {
			return f.selectKey(ctx, token)
		},
	)
	if err != nil {
		f.logger.ErrorContext(
			ctx,
			"Failed to parse token",
			slog.String("!token", bearer),
			slog.Any("error", err),
		)
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			err = grpcstatus.Error(grpccodes.Unauthenticated, "Bearer token is malformed")
		case errors.Is(err, jwt.ErrTokenUnverifiable):
			err = grpcstatus.Error(grpccodes.Unauthenticated, "Bearer token can't be verified")
		case errors.Is(err, jwt.ErrSignatureInvalid):
			err = grpcstatus.Error(grpccodes.Unauthenticated, "Signature of bearer token isn't valid")
		case errors.Is(err, jwt.ErrTokenExpired):
			err = grpcstatus.Error(grpccodes.Unauthenticated, "Bearer token is expired")
		case errors.Is(err, jwt.ErrTokenUsedBeforeIssued):
			err = grpcstatus.Error(grpccodes.Unauthenticated, "Bearer token was issued in the future")
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			err = grpcstatus.Error(grpccodes.Unauthenticated, "Bearer token isn't valid yet")
		default:
			err = grpcstatus.Error(grpccodes.Unauthenticated, "Bearer token isn't valid")
		}
	}
	return
}

// checkClaims checks that the required claims are present and that they have valid values.
func (f *grpcJwksAuthnFunc) checkClaims(claims jwt.MapClaims) error {
	// The `typ` claim is optional, but if it exists the value must be `Bearer`:
	value, ok := claims["typ"]
	if ok {
		typ, ok := value.(string)
		if !ok {
			return grpcstatus.Errorf(
				grpccodes.Unauthenticated,
				"Bearer token type claim contains incorrect string value '%v'",
				value,
			)
		}
		if !strings.EqualFold(typ, "Bearer") {
			return grpcstatus.Errorf(
				grpccodes.Unauthenticated,
				"Bearer token type '%s' isn't allowed",
				typ,
			)
		}
	}

	// Check the format of the `sub` claim:
	_, err := f.checkStringClaim(claims, "sub")
	if err != nil {
		return err
	}

	// Check the format of the issue and expiration date claims:
	_, err = f.checkTimeClaim(claims, "iat")
	if err != nil {
		return err
	}
	_, err = f.checkTimeClaim(claims, "exp")
	if err != nil {
		return err
	}

	return nil
}

// checkStringClaim checks that the given claim exists and that the value is a string. If it exist it returns the value.
func (f *grpcJwksAuthnFunc) checkStringClaim(claims jwt.MapClaims, name string) (result string, err error) {
	value, err := f.checkClaim(claims, name)
	if err != nil {
		return
	}
	text, ok := value.(string)
	if !ok {
		err = fmt.Errorf(
			"bearer token claim '%s' with value '%v' should be a string, but it is "+
				"of type '%T'",
			name, value, value,
		)
		return
	}
	result = text
	return
}

// checkTimeClaim checks that the given claim exists and that the value is a time. If it exists it returns the value.
func (f *grpcJwksAuthnFunc) checkTimeClaim(claims jwt.MapClaims, name string) (result time.Time,
	err error) {
	value, err := f.checkClaim(claims, name)
	if err != nil {
		return
	}
	seconds, ok := value.(float64)
	if !ok {
		err = grpcstatus.Errorf(
			grpccodes.Unauthenticated,
			"Bearer token claim '%s' contains incorrect time value '%v'",
			name, value,
		)
		return
	}
	result = time.Unix(int64(seconds), 0)
	return
}

// checkClaim checks that the given claim exists. If it exists it returns the value.
func (f *grpcJwksAuthnFunc) checkClaim(claims jwt.MapClaims, name string) (result any, err error) {
	value, ok := claims[name]
	if !ok {
		err = grpcstatus.Errorf(
			grpccodes.Unauthenticated,
			"Bearer token doesn't contain required claim '%s'",
			name,
		)
		return
	}
	result = value
	return
}

// Regular expression used to extract the bearer token from the authorization header:
var grpcJwksAuthFuncBearerRE = regexp.MustCompile(`^([a-zA-Z0-9]+)\s+(.*)$`)
