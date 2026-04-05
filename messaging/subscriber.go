package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"cloud.google.com/go/pubsub"
)

// Subscriber pulls messages from a Pub/Sub subscription.
// Use this on GKE where pods are long-lived and pull subscriptions are
// more natural than push endpoints. On Cloud Run, prefer PushHandler.
type Subscriber struct {
	client    *pubsub.Client
	projectID string
	mu        sync.Mutex
	cancels   []context.CancelFunc
}

// NewSubscriber creates a Pub/Sub pull subscriber for the given GCP project.
func NewSubscriber(ctx context.Context, projectID string) (*Subscriber, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("messaging: create subscriber client: %w", err)
	}
	return &Subscriber{
		client:    client,
		projectID: projectID,
	}, nil
}

// Subscribe starts pulling messages from the named subscription and routes
// them to the handler. Blocks until ctx is cancelled or a fatal error occurs.
// Call this in a goroutine per subscription.
//
// Example:
//
//	go sub.Subscribe(ctx, "audit-events-sub", func(e messaging.Event) error {
//	    return processAuditEvent(e)
//	})
func (s *Subscriber) Subscribe(ctx context.Context, subscriptionID string, handler EventHandler) error {
	sub := s.client.Subscription(subscriptionID)

	// Verify subscription exists
	exists, err := sub.Exists(ctx)
	if err != nil {
		return fmt.Errorf("messaging: check subscription %s: %w", subscriptionID, err)
	}
	if !exists {
		return fmt.Errorf("messaging: subscription %s does not exist", subscriptionID)
	}

	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancels = append(s.cancels, cancel)
	s.mu.Unlock()

	slog.Info("messaging: starting pull subscription", "subscription", subscriptionID)

	return sub.Receive(ctx, func(_ context.Context, msg *pubsub.Message) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			slog.Error("messaging: unmarshal pull message",
				"subscription", subscriptionID,
				"message_id", msg.ID,
				"error", err,
			)
			msg.Nack()
			return
		}

		slog.Info("messaging: received event",
			"event_id", event.ID,
			"type", event.Type,
			"source", event.Source,
			"tenant_id", event.TenantID,
			"subscription", subscriptionID,
		)

		if err := handler(event); err != nil {
			slog.Error("messaging: handle event failed",
				"event_id", event.ID,
				"type", event.Type,
				"subscription", subscriptionID,
				"error", err,
			)
			msg.Nack()
			return
		}

		msg.Ack()
	})
}

// SubscribeRouted starts pulling messages and routes them by event type.
// Unhandled event types are acked without processing.
func (s *Subscriber) SubscribeRouted(ctx context.Context, subscriptionID string, handlers map[string]EventHandler) error {
	return s.Subscribe(ctx, subscriptionID, func(event Event) error {
		handler, ok := handlers[event.Type]
		if !ok {
			slog.Warn("messaging: no handler for event type, acking",
				"type", event.Type,
				"subscription", subscriptionID,
			)
			return nil
		}
		return handler(event)
	})
}

// Close stops all active subscriptions and releases resources.
func (s *Subscriber) Close() error {
	s.mu.Lock()
	for _, cancel := range s.cancels {
		cancel()
	}
	s.cancels = nil
	s.mu.Unlock()

	return s.client.Close()
}
