package messaging

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// PubSubPushMessage is the envelope Pub/Sub sends to push subscription endpoints.
type PubSubPushMessage struct {
	Message struct {
		Data        string            `json:"data"`
		Attributes  map[string]string `json:"attributes"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// EventHandler processes a single event. Return an error to trigger Pub/Sub retry.
type EventHandler func(event Event) error

// PushHandler returns an http.HandlerFunc that receives Pub/Sub push messages,
// decodes the event, and calls the handler. Returns 200 on success (ack),
// 500 on error (Pub/Sub retries).
//
// Register as: mux.HandleFunc("POST /events", messaging.PushHandler(myHandler))
func PushHandler(handler EventHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var msg PubSubPushMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			slog.Error("messaging: decode push message", "error", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		data, err := base64.StdEncoding.DecodeString(msg.Message.Data)
		if err != nil {
			slog.Error("messaging: decode base64 data", "error", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			slog.Error("messaging: unmarshal event", "error", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		slog.Info("messaging: received event",
			"event_id", event.ID,
			"type", event.Type,
			"source", event.Source,
			"tenant_id", event.TenantID,
		)

		if err := handler(event); err != nil {
			slog.Error("messaging: handle event failed",
				"event_id", event.ID,
				"type", event.Type,
				"error", err,
			)
			// 500 tells Pub/Sub to retry
			http.Error(w, fmt.Sprintf("processing failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// RouterHandler routes events to different handlers based on event type.
// Unhandled event types return 200 (ack without processing).
func RouterHandler(handlers map[string]EventHandler) http.HandlerFunc {
	return PushHandler(func(event Event) error {
		handler, ok := handlers[event.Type]
		if !ok {
			slog.Warn("messaging: no handler for event type, acking", "type", event.Type)
			return nil
		}
		return handler(event)
	})
}
