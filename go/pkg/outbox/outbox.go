// Package outbox gives services reliable event publishing and idempotent consumption on top of
// their own Postgres schema:
//
//   - Enqueue writes an event into the service's outbox table inside the same transaction as the
//     state change, so the change and the event commit or roll back together.
//   - Relay publishes outbox rows to Kafka and marks them sent. If Kafka or the service is down,
//     rows wait; nothing is lost. Delivery is at least once.
//   - Once records an event ID in processed_events in the same transaction as the handler's
//     writes, so a redelivered event has no second effect.
package outbox

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"udyam/pkg/events"
	"udyam/pkg/kafkax"
)

// Schema creates the two tables in the current search_path (the service schema).
const Schema = `
CREATE TABLE IF NOT EXISTS outbox (
	seq          bigserial PRIMARY KEY,
	event_id     text NOT NULL UNIQUE,
	topic        text NOT NULL,
	envelope     jsonb NOT NULL,
	created_at   timestamptz NOT NULL DEFAULT now(),
	published_at timestamptz
);
CREATE INDEX IF NOT EXISTS outbox_pending ON outbox (seq) WHERE published_at IS NULL;
CREATE TABLE IF NOT EXISTS processed_events (
	consumer     text NOT NULL,
	event_id     text NOT NULL,
	topic        text NOT NULL,
	processed_at timestamptz NOT NULL DEFAULT now(),
	PRIMARY KEY (consumer, event_id)
);
`

// Init creates the tables.
func Init(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, Schema)
	return err
}

// Enqueue builds an envelope and stores it for the relay. Call it inside the transaction that
// makes the change the event describes.
func Enqueue(ctx context.Context, tx pgx.Tx, topic, leadID, correlationID string, payload any) (events.Envelope, error) {
	env, err := events.New(topic, leadID, correlationID, payload)
	if err != nil {
		return env, err
	}
	b, err := json.Marshal(env)
	if err != nil {
		return env, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO outbox (event_id, topic, envelope) VALUES ($1, $2, $3)`, env.ID, topic, b)
	return env, err
}

// EnqueueNow is Enqueue in its own transaction, for changes that are not in the database.
func EnqueueNow(ctx context.Context, db *pgxpool.Pool, topic, leadID, correlationID string, payload any) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		_, err := Enqueue(ctx, tx, topic, leadID, correlationID, payload)
		return err
	})
}

// Once runs fn at most once per (consumer, event ID). The dedupe row and fn's writes share one
// transaction. Returns done=false when the event was already processed.
func Once(ctx context.Context, db *pgxpool.Pool, consumer string, env events.Envelope, topic string, fn func(tx pgx.Tx) error) (done bool, err error) {
	err = pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO processed_events (consumer, event_id, topic) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			consumer, env.ID, topic)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		done = true
		return fn(tx)
	})
	if err != nil {
		done = false
	}
	return done, err
}

// Relay publishes pending outbox rows in order until ctx ends. Rows are locked with SKIP LOCKED
// so several replicas can relay; a row is marked published only after Kafka acknowledged it.
func Relay(ctx context.Context, db *pgxpool.Pool, p *kafkax.Producer, log *slog.Logger) {
	idle := 200 * time.Millisecond
	for ctx.Err() == nil {
		n, err := relayBatch(ctx, db, p)
		if err != nil && ctx.Err() == nil {
			log.Warn("outbox relay paused", "err", err)
			sleep(ctx, 2*time.Second)
			continue
		}
		if n == 0 {
			sleep(ctx, idle)
		}
	}
}

func relayBatch(ctx context.Context, db *pgxpool.Pool, p *kafkax.Producer) (int, error) {
	sent := 0
	var pubErr error
	err := pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT seq, topic, envelope FROM outbox WHERE published_at IS NULL ORDER BY seq LIMIT 100 FOR UPDATE SKIP LOCKED`)
		if err != nil {
			return err
		}
		type row struct {
			seq   int64
			topic string
			env   events.Envelope
		}
		var batch []row
		for rows.Next() {
			var r row
			var raw []byte
			if err := rows.Scan(&r.seq, &r.topic, &raw); err != nil {
				rows.Close()
				return err
			}
			if err := json.Unmarshal(raw, &r.env); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, r)
		}
		rows.Close()
		for _, r := range batch {
			if err := p.Publish(ctx, r.topic, r.env); err != nil {
				pubErr = err // keep what was sent; the rest stays pending
				break
			}
			if _, err := tx.Exec(ctx, `UPDATE outbox SET published_at = now() WHERE seq = $1`, r.seq); err != nil {
				return err
			}
			sent++
		}
		return nil
	})
	if err == nil {
		err = pubErr
	}
	return sent, err
}

// Pending counts unpublished rows (for /metrics and tests).
func Pending(ctx context.Context, db *pgxpool.Pool) (int, error) {
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&n)
	return n, err
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
