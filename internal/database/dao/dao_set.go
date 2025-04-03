/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package dao

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	eventsv1 "github.com/innabox/fulfillment-service/internal/api/events/v1"
	fulfillmentv1 "github.com/innabox/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/innabox/fulfillment-service/internal/database"
)

type Set interface {
	ClusterOrders() *GenericDAO[*fulfillmentv1.ClusterOrder]
	ClusterTemplates() *GenericDAO[*fulfillmentv1.ClusterTemplate]
	Clusters() *GenericDAO[*fulfillmentv1.Cluster]
}

type set struct {
	logger           *slog.Logger
	notifier         *database.Notifier
	clusterTemplates *GenericDAO[*fulfillmentv1.ClusterTemplate]
	clusterOrders    *GenericDAO[*fulfillmentv1.ClusterOrder]
	clusters         *GenericDAO[*fulfillmentv1.Cluster]
}

type SetBuilder struct {
	logger *slog.Logger
}

func NewSet() *SetBuilder {
	return &SetBuilder{}
}

func (b *SetBuilder) SetLogger(value *slog.Logger) *SetBuilder {
	b.logger = value
	return b
}

func (b *SetBuilder) Build() (result Set, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}

	// Create the result early, so that we can the reference to the event callback to create the individual DAOs:
	s := &set{
		logger: b.logger,
	}

	// Create the notifier:
	s.notifier, err = database.NewNotifier().
		SetLogger(b.logger).
		SetChannel("events").
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create notifier: %w", err)
		return
	}

	// Cluster the individual DAOs:
	s.clusterTemplates, err = NewGenericDAO[*fulfillmentv1.ClusterTemplate]().
		SetLogger(b.logger).
		SetTable("public.cluster_templates").
		AddEventCallback(s.notifyEvent).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster templates DAO: %w", err)
		return
	}
	s.clusterOrders, err = NewGenericDAO[*fulfillmentv1.ClusterOrder]().
		SetLogger(b.logger).
		SetTable("public.cluster_orders").
		AddEventCallback(s.notifyEvent).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create cluster orders DAO: %w", err)
		return
	}
	s.clusters, err = NewGenericDAO[*fulfillmentv1.Cluster]().
		SetLogger(b.logger).
		SetTable("public.clusters").
		AddEventCallback(s.notifyEvent).
		Build()
	if err != nil {
		err = fmt.Errorf("failed to create clusters DAO: %w", err)
		return
	}

	result = s
	return
}

func (s *set) ClusterTemplates() *GenericDAO[*fulfillmentv1.ClusterTemplate] {
	return s.clusterTemplates
}

func (s *set) ClusterOrders() *GenericDAO[*fulfillmentv1.ClusterOrder] {
	return s.clusterOrders
}

func (s *set) Clusters() *GenericDAO[*fulfillmentv1.Cluster] {
	return s.clusters
}

// notifyEvent converts the DAO event into an API event and publishes it using the PostgreSQL NOTIFY command.
func (s *set) notifyEvent(ctx context.Context, e Event) (err error) {
	event := &eventsv1.Event{}
	event.Id = uuid.NewString()
	switch e.Type {
	case EventTypeCreated:
		event.Type = eventsv1.EventType_EVENT_TYPE_OBJECT_CREATED
	case EventTypeUpdated:
		event.Type = eventsv1.EventType_EVENT_TYPE_OBJECT_UPDATED
	case EventTypeDeleted:
		event.Type = eventsv1.EventType_EVENT_TYPE_OBJECT_DELETED
	default:
		return fmt.Errorf("unknown event kind '%s'", e.Type)
	}
	switch object := e.Object.(type) {
	case *fulfillmentv1.Cluster:
		event.Payload = &eventsv1.Event_Cluster{
			Cluster: object,
		}
	case *fulfillmentv1.ClusterOrder:
		event.Payload = &eventsv1.Event_ClusterOrder{
			ClusterOrder: object,
		}
	case *fulfillmentv1.ClusterTemplate:
		event.Payload = &eventsv1.Event_ClusterTemplate{
			ClusterTemplate: object,
		}
	default:
		return fmt.Errorf("unknown object type '%T'", object)
	}
	return s.notifier.Notify(ctx, event)
}
