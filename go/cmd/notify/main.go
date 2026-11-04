// Command notify consumes notification events. Phase 0: consumes system.ping into notify.ping_log
// so the smoke test proves gateway -> Kafka -> consumer -> Postgres -> HTTP end to end.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"udyam/pkg/events"
	"udyam/pkg/kafkax"
	"udyam/pkg/svc"
)

func main() {
	svc.Run(svc.Options{
		Name: "notify", DefaultPort: "8086", Schema: "notify", UsesKafka: true,
		Setup: func(ctx context.Context, a *svc.App) error {
			if _, err := a.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS ping_log (
				event_id    text PRIMARY KEY,
				message     text NOT NULL,
				received_at timestamptz NOT NULL DEFAULT now())`); err != nil {
				return err
			}

			handler := func(ctx context.Context, topic string, env events.Envelope) error {
				var p events.PingPayload
				if err := json.Unmarshal(env.Payload, &p); err != nil {
					return err
				}
				_, err := a.DB.Exec(ctx,
					`INSERT INTO ping_log (event_id, message) VALUES ($1, $2) ON CONFLICT (event_id) DO NOTHING`,
					env.ID, p.Message)
				if err == nil {
					a.Log.Info("ping consumed", "eventId", env.ID, "correlationId", env.CorrelationID)
				}
				return err
			}

			go func() {
				for {
					err := kafkax.Consume(ctx, a.Log, a.Brokers, "notify", []string{events.TopicSystemPing}, handler)
					if ctx.Err() != nil {
						return
					}
					a.Log.Error("consumer stopped, retrying", "err", err)
					time.Sleep(2 * time.Second)
				}
			}()

			a.Mux.HandleFunc("GET /pings", func(w http.ResponseWriter, r *http.Request) {
				rows, err := a.DB.Query(r.Context(), `SELECT event_id, message, received_at FROM ping_log ORDER BY received_at DESC LIMIT 20`)
				if err != nil {
					svc.JSON(w, 500, map[string]string{"error": err.Error()})
					return
				}
				defer rows.Close()
				out := []map[string]any{}
				for rows.Next() {
					var id, msg string
					var at time.Time
					if err := rows.Scan(&id, &msg, &at); err != nil {
						svc.JSON(w, 500, map[string]string{"error": err.Error()})
						return
					}
					out = append(out, map[string]any{"eventId": id, "message": msg, "receivedAt": at})
				}
				svc.JSON(w, 200, out)
			})
			return nil
		},
	})
}
