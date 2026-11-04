// Package events defines the event envelope shared by every service.
// Contract: /contracts/events.schema.json. Topic names are generated into topics_gen.go.
package events

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"
)

// Envelope wraps every event published to Kafka.
type Envelope struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurredAt"`
	LeadID        string          `json:"leadId,omitempty"`
	CorrelationID string          `json:"correlationId"`
	Payload       json.RawMessage `json:"payload"`
}

// NewID returns a random UUID v4.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// New builds an envelope. leadID may be empty for system events; correlationID defaults to a fresh ID.
func New(eventType, leadID, correlationID string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	if correlationID == "" {
		correlationID = NewID()
	}
	return Envelope{
		ID: NewID(), Type: eventType, Version: 1, OccurredAt: time.Now().UTC(),
		LeadID: leadID, CorrelationID: correlationID, Payload: raw,
	}, nil
}

// PingPayload is the Phase 0 smoke-test payload.
type PingPayload struct {
	Message string `json:"message"`
}
