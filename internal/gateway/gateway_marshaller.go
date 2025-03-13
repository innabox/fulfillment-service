/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package gateway

import (
	"io"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/encoding/protojson"
)

type MarshallerBuilder struct {
}

func NewMarshaller() *MarshallerBuilder {
	return &MarshallerBuilder{}
}

func (b *MarshallerBuilder) Build() (result runtime.Marshaler, err error) {
	result = &Marshaller{
		delegate: &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames: true,
			},
		},
	}
	return
}

type Marshaller struct {
	protojson.MarshalOptions
	protojson.UnmarshalOptions
	delegate runtime.Marshaler
}

func (m *Marshaller) ContentType(v any) string {
	return m.delegate.ContentType(v)
}

func (m *Marshaller) Marshal(v any) ([]byte, error) {
	return m.delegate.Marshal(v)
}

func (m *Marshaller) Unmarshal(data []byte, v interface{}) error {
	return m.delegate.Unmarshal(data, v)
}

func (m *Marshaller) NewDecoder(r io.Reader) runtime.Decoder {
	return m.delegate.NewDecoder(r)
}

func (m *Marshaller) NewEncoder(w io.Writer) runtime.Encoder {
	return m.delegate.NewEncoder(w)
}
