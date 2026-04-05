// Package messaging provides Google Pub/Sub event publishing and subscription handling.
//
// Consumption modes:
//   - PushHandler / RouterHandler: HTTP push endpoints (Cloud Run / Knative)
//   - Subscriber / SubscribeRouted: Pull subscriptions (GKE long-lived pods)
//
// Publishing (Publisher) works identically on both Cloud Run and GKE.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
)

// Event is the standard event envelope published to Pub/Sub topics.
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Source    string          `json:"source"`
	TenantID  string          `json:"tenant_id"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// Publisher publishes events to Google Pub/Sub topics.
type Publisher struct {
	client    *pubsub.Client
	projectID string
	source    string
	mu        sync.RWMutex
	topics    map[string]*pubsub.Topic
}

// NewPublisher creates a Pub/Sub publisher for the given GCP project.
// source identifies the publishing service (e.g., "marketplace/orders-service").
// A 10-second timeout is enforced to prevent blocking service startup.
func NewPublisher(ctx context.Context, projectID, source string) (*Publisher, error) {
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := pubsub.NewClient(initCtx, projectID)
	if err != nil {
		return nil, fmt.Errorf("messaging: create pubsub client: %w", err)
	}
	return &Publisher{
		client:    client,
		projectID: projectID,
		source:    source,
		topics:    make(map[string]*pubsub.Topic),
	}, nil
}

// Publish sends an event to the named topic. The data is JSON-marshaled.
// This is non-blocking — the result is flushed asynchronously.
func (p *Publisher) Publish(ctx context.Context, topicName string, eventType string, tenantID string, data any) error {
	topic, err := p.getTopic(topicName)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("messaging: marshal data: %w", err)
	}

	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    p.source,
		TenantID:  tenantID,
		Timestamp: time.Now().UTC(),
		Data:      payload,
	}

	envelope, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("messaging: marshal event: %w", err)
	}

	result := topic.Publish(ctx, &pubsub.Message{
		Data: envelope,
		Attributes: map[string]string{
			"event_type": eventType,
			"tenant_id":  tenantID,
			"source":     p.source,
		},
	})

	// Wait for publish to complete
	_, err = result.Get(ctx)
	if err != nil {
		return fmt.Errorf("messaging: publish to %s: %w", topicName, err)
	}

	return nil
}

// PublishAsync sends an event without waiting for server acknowledgment.
// Use this for non-critical events where fire-and-forget is acceptable.
func (p *Publisher) PublishAsync(ctx context.Context, topicName string, eventType string, tenantID string, data any) {
	topic, err := p.getTopic(topicName)
	if err != nil {
		return
	}

	payload, _ := json.Marshal(data)
	event := Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		Source:    p.source,
		TenantID:  tenantID,
		Timestamp: time.Now().UTC(),
		Data:      payload,
	}
	envelope, _ := json.Marshal(event)

	topic.Publish(ctx, &pubsub.Message{
		Data: envelope,
		Attributes: map[string]string{
			"event_type": eventType,
			"tenant_id":  tenantID,
			"source":     p.source,
		},
	})
}

func (p *Publisher) getTopic(name string) (*pubsub.Topic, error) {
	p.mu.RLock()
	if t, ok := p.topics[name]; ok {
		p.mu.RUnlock()
		return t, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	// Double-check after acquiring write lock
	if t, ok := p.topics[name]; ok {
		return t, nil
	}
	t := p.client.Topic(name)
	t.PublishSettings.DelayThreshold = 100 * time.Millisecond
	t.PublishSettings.CountThreshold = 100
	p.topics[name] = t
	return t, nil
}

// Close flushes pending messages and releases resources.
func (p *Publisher) Close() error {
	for _, t := range p.topics {
		t.Stop()
	}
	return p.client.Close()
}
