// Package kafkax wraps segmentio/kafka-go with the event envelope.
package kafkax

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"udyam/pkg/events"
)

// Producer publishes envelopes keyed by leadId (or event id when no lead).
type Producer struct{ w *kafka.Writer }

// NewProducer creates a producer for a comma-separated broker list.
func NewProducer(brokers string) *Producer {
	return &Producer{w: &kafka.Writer{
		Addr:                   kafka.TCP(strings.Split(brokers, ",")...),
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: false,
		WriteTimeout:           10 * time.Second,
		BatchTimeout:           10 * time.Millisecond,
	}}
}

// Publish sends one envelope to a topic.
func (p *Producer) Publish(ctx context.Context, topic string, env events.Envelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	key := env.LeadID
	if key == "" {
		key = env.ID
	}
	return p.w.WriteMessages(ctx, kafka.Message{Topic: topic, Key: []byte(key), Value: b})
}

// Close flushes and closes the producer.
func (p *Producer) Close() error { return p.w.Close() }

// Handler processes one event. Return an error to signal failure.
type Handler func(ctx context.Context, topic string, env events.Envelope) error

// Consume joins a consumer group and calls h for each message until ctx is cancelled.
// Phase 0 behaviour: a failing handler is logged and the offset is still committed.
// Phase 5 replaces this with retries, an idempotency table and the dlq.events topic.
func Consume(ctx context.Context, log *slog.Logger, brokers, group string, topics []string, h Handler) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(brokers, ","),
		GroupID:     group,
		GroupTopics: topics,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     500 * time.Millisecond,
	})
	defer r.Close()
	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		var env events.Envelope
		if err := json.Unmarshal(m.Value, &env); err != nil {
			log.Error("bad event payload, skipping", "topic", m.Topic, "offset", m.Offset, "err", err)
		} else if err := h(ctx, m.Topic, env); err != nil {
			log.Error("handler failed", "topic", m.Topic, "eventId", env.ID, "err", err)
		}
		if err := r.CommitMessages(ctx, m); err != nil {
			log.Error("commit failed", "err", err)
		}
	}
}
