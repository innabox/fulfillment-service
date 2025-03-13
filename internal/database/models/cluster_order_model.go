/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package models

import "fmt"

type ClusterOrderState int

const (
	ClusterOrderStateUnspecified ClusterOrderState = iota
	ClusterOrderStateAccepted
	ClusterOrderStateRejected
	ClusterOrderStateFailed
	ClusterOrderStateCanceled
	ClusterOrderStateFulfilled
)

func (s ClusterOrderState) String() string {
	switch s {
	case ClusterOrderStateUnspecified:
		return "UNSPECIFIED"
	case ClusterOrderStateAccepted:
		return "ACCEPTED"
	case ClusterOrderStateRejected:
		return "REJECTED"
	case ClusterOrderStateFailed:
		return "FAILED"
	case ClusterOrderStateCanceled:
		return "CANCELED"
	case ClusterOrderStateFulfilled:
		return "FULFILLED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

func (s *ClusterOrderState) Scan(value any) error {
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}
	switch text {
	case "UNSPECIFIED":
		*s = ClusterOrderStateUnspecified
	case "ACCEPTED":
		*s = ClusterOrderStateAccepted
	case "REJECTED":
		*s = ClusterOrderStateRejected
	case "FAILED":
		*s = ClusterOrderStateFailed
	case "CANCELED":
		*s = ClusterOrderStateCanceled
	case "FULFILLED":
		*s = ClusterOrderStateFulfilled
	default:
		return fmt.Errorf("text '%s' doesn't correspond to any known cluster order state", text)
	}
	return nil
}

type ClusterOrder struct {
	ID         string
	TemplateID string
	State      ClusterOrderState
	ClusterID  string
}
