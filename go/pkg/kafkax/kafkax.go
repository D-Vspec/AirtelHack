// Package kafkax wraps segmentio/kafka-go with the event envelope.
package kafkax

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
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

var processed, retried, deadLettered atomic.Uint64

// Counters reports events handled, retries and dead letters since start (for /metrics).
func Counters() (uint64, uint64, uint64) {
	return processed.Load(), retried.Load(), deadLettered.Load()
}

// Retry backoffs before an event is dead-lettered (so 4 attempts in all).
var Backoffs = []time.Duration{200 * time.Millisecond, time.Second, 3 * time.Second}

// DeadLetter is the dlq.events payload.
type DeadLetter struct {
	Consumer      string          `json:"consumer"`
	OriginalTopic string          `json:"originalTopic"`
	Partition     int             `json:"partition"`
	Offset        int64           `json:"offset"`
	Attempts      int             `json:"attempts"`
	Error         string          `json:"error"`
	Original      json.RawMessage `json:"original"` // the message value as received
}

// Consume joins a consumer group and calls h for each message until ctx is cancelled. A failing
// handler is retried with Backoffs; after the last attempt (or for a message that is not a valid
// envelope) the message goes to dlq.events and its offset is committed, so one bad event does
// not block the partition. If the dead letter cannot be written, Consume returns without
// committing and the message is redelivered. Handlers must be idempotent (see pkg/outbox.Once):
// delivery is at least once.
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
	dlq := NewProducer(brokers)
	defer dlq.Close()
	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		attempts, herr := 0, error(nil)
		var env events.Envelope
		if err := json.Unmarshal(m.Value, &env); err != nil || env.ID == "" {
			herr = fmt.Errorf("not a valid event envelope: %v", err)
		} else {
			for attempts = 1; ; attempts++ {
				if herr = h(ctx, m.Topic, env); herr == nil || ctx.Err() != nil || attempts > len(Backoffs) {
					break
				}
				retried.Add(1)
				log.Warn("handler failed, retrying", "topic", m.Topic, "eventId", env.ID, "attempt", attempts, "err", herr)
				select {
				case <-ctx.Done():
				case <-time.After(Backoffs[attempts-1]):
				}
			}
		}
		if ctx.Err() != nil {
			return nil // shutting down: leave the offset uncommitted
		}
		if herr != nil {
			dl := DeadLetter{Consumer: group, OriginalTopic: m.Topic, Partition: m.Partition, Offset: m.Offset, Attempts: attempts, Error: herr.Error(), Original: validJSON(m.Value)}
			denv, err := events.New(events.TopicDlqEvents, env.LeadID, env.CorrelationID, dl)
			if err == nil {
				err = dlq.Publish(ctx, events.TopicDlqEvents, denv)
			}
			if err != nil {
				return fmt.Errorf("dead-letter failed, will redeliver: %w", err)
			}
			deadLettered.Add(1)
			log.Error("event dead-lettered", "topic", m.Topic, "eventId", env.ID, "attempts", attempts, "err", herr)
		}
		processed.Add(1)
		if err := r.CommitMessages(ctx, m); err != nil {
			log.Error("commit failed", "err", err)
		}
	}
}

func validJSON(b []byte) json.RawMessage {
	if json.Valid(b) {
		return b
	}
	q, _ := json.Marshal(string(b))
	return q
}

// Run keeps Consume running, restarting it after errors, until ctx ends.
func Run(ctx context.Context, log *slog.Logger, brokers, group string, topics []string, h Handler) {
	for ctx.Err() == nil {
		err := Consume(ctx, log, brokers, group, topics, h)
		if ctx.Err() != nil {
			return
		}
		log.Error("consumer stopped, restarting", "group", group, "err", err)
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
	}
}
