// Command notify sends owner messages. It consumes notification.requested and
// order.status.changed (and advisor.profile.updated, to learn each lead's language). WhatsApp,
// SMS and email adapters are mocks that write to the sent_messages table, which GET /messages
// shows. It also keeps the Phase 0 system.ping log used by `make smoke`.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/events"
	"udyam/pkg/kafkax"
	"udyam/pkg/logx"
	"udyam/pkg/orders"
	"udyam/pkg/outbox"
	"udyam/pkg/svc"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS ping_log (
	event_id    text PRIMARY KEY,
	message     text NOT NULL,
	received_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS contacts (
	lead_id    text PRIMARY KEY,
	language   text NOT NULL,
	updated_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS sent_messages (
	id           text PRIMARY KEY,
	event_id     text NOT NULL,
	lead_id      text NOT NULL,
	channel      text NOT NULL,
	template_key text NOT NULL,
	language     text NOT NULL,
	recipient    text NOT NULL,
	body         text NOT NULL,
	created_at   timestamptz NOT NULL,
	UNIQUE (event_id, channel)
);
CREATE INDEX IF NOT EXISTS sent_messages_lead ON sent_messages (lead_id, created_at);
`

var channels = []string{"whatsapp", "sms", "email"}

// Adapter delivers one message. The demo adapters only record it; a real WhatsApp Business API,
// SMS gateway or email provider would implement the same interface.
type Adapter interface {
	Send(ctx context.Context, tx pgx.Tx, m Message) error
}

type Message struct {
	EventID, LeadID, Channel, TemplateKey, Language, Body string
}

type mockAdapter struct{ channel string }

func (a mockAdapter) Send(ctx context.Context, tx pgx.Tx, m Message) error {
	// no phone or email is collected in the demo, so the recipient is the lead reference
	_, err := tx.Exec(ctx, `INSERT INTO sent_messages (id, event_id, lead_id, channel, template_key, language, recipient, body, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now()) ON CONFLICT (event_id, channel) DO NOTHING`,
		events.NewID(), m.EventID, m.LeadID, a.channel, m.TemplateKey, m.Language, "lead:"+logx.MaskID(m.LeadID), m.Body)
	return err
}

type server struct {
	a        *svc.App
	adapters map[string]Adapter
}

func main() {
	svc.Run(svc.Options{
		Name: "notify", DefaultPort: "8086", Schema: "notify", UsesKafka: true,
		Setup: func(ctx context.Context, a *svc.App) error {
			if err := outbox.Init(ctx, a.DB); err != nil {
				return err
			}
			if _, err := a.DB.Exec(ctx, schemaSQL); err != nil {
				return err
			}
			s := &server{a: a, adapters: map[string]Adapter{}}
			for _, c := range channels {
				s.adapters[c] = mockAdapter{c}
			}
			go kafkax.Run(ctx, a.Log, a.Brokers, "notify", []string{events.TopicSystemPing}, s.ping)
			go kafkax.Run(ctx, a.Log, a.Brokers, "notify-messages",
				[]string{events.TopicNotificationRequested, events.TopicOrderStatusChanged, events.TopicOrderCreated, events.TopicAdvisorProfileUpdated}, s.handle)
			a.Mux.HandleFunc("GET /pings", s.pings)
			a.Mux.HandleFunc("GET /messages", s.messages)
			return nil
		},
	})
}

func (s *server) ping(ctx context.Context, _ string, env events.Envelope) error {
	var p events.PingPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	_, err := s.a.DB.Exec(ctx, `INSERT INTO ping_log (event_id, message) VALUES ($1, $2) ON CONFLICT (event_id) DO NOTHING`, env.ID, p.Message)
	if err == nil {
		s.a.Log.Info("ping consumed", "eventId", env.ID, "correlationId", env.CorrelationID)
	}
	return err
}

// statusTemplates are the order statuses the owner hears about (quote_accepted is covered by
// order.created).
var statusTemplates = []string{orders.KycPending, orders.KycPassed, orders.InstallScheduled, orders.Installing, orders.Activated, orders.Delayed, orders.Failed}

func (s *server) handle(ctx context.Context, topic string, env events.Envelope) error {
	_, err := outbox.Once(ctx, s.a.DB, "notify", env, topic, func(tx pgx.Tx) error {
		switch topic {
		case events.TopicAdvisorProfileUpdated:
			var p struct {
				Profile struct {
					PreferredLanguage string `json:"preferredLanguage"`
				} `json:"profile"`
			}
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				return err
			}
			code, ok := languageCodes[p.Profile.PreferredLanguage]
			if !ok || env.LeadID == "" {
				return nil
			}
			_, err := tx.Exec(ctx, `INSERT INTO contacts (lead_id, language, updated_at) VALUES ($1,$2,now())
				ON CONFLICT (lead_id) DO UPDATE SET language = EXCLUDED.language, updated_at = EXCLUDED.updated_at`, env.LeadID, code)
			return err
		case events.TopicOrderCreated:
			var o struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(env.Payload, &o); err != nil {
				return err
			}
			return s.send(ctx, tx, env, "order.created", map[string]string{"order": orders.ShortID(o.ID)}, []string{"whatsapp"})
		case events.TopicOrderStatusChanged:
			var c struct {
				OrderID string `json:"orderId"`
				To      string `json:"to"`
				Reason  string `json:"reason"`
				Date    string `json:"expectedActivationDate"`
			}
			if err := json.Unmarshal(env.Payload, &c); err != nil {
				return err
			}
			if !slices.Contains(statusTemplates, c.To) {
				return nil
			}
			chs := []string{"whatsapp"}
			if c.To == orders.Activated || c.To == orders.Failed {
				chs = append(chs, "sms") // milestones that matter even without WhatsApp
			}
			reason := c.Reason
			if reason == "" {
				reason = "-"
			}
			return s.send(ctx, tx, env, "order.status."+c.To, map[string]string{"order": orders.ShortID(c.OrderID), "date": c.Date, "reason": reason}, chs)
		case events.TopicNotificationRequested:
			var n struct {
				TemplateKey string            `json:"templateKey"`
				Params      map[string]string `json:"params"`
				Channels    []string          `json:"channels"`
			}
			if err := json.Unmarshal(env.Payload, &n); err != nil {
				return err
			}
			if len(n.Channels) == 0 {
				n.Channels = []string{"whatsapp"}
			}
			return s.send(ctx, tx, env, n.TemplateKey, n.Params, n.Channels)
		}
		return nil
	})
	return err
}

func (s *server) send(ctx context.Context, tx pgx.Tx, env events.Envelope, key string, params map[string]string, chs []string) error {
	lang := "en"
	if err := tx.QueryRow(ctx, `SELECT language FROM contacts WHERE lead_id = $1`, env.LeadID).Scan(&lang); err != nil && err != pgx.ErrNoRows {
		return err
	}
	body, used, ok := render(key, lang, params)
	if !ok {
		return fmt.Errorf("unknown template %q", key) // retried, then dead-lettered
	}
	for _, c := range chs {
		ad, ok := s.adapters[c]
		if !ok {
			return fmt.Errorf("unknown channel %q", c)
		}
		if err := ad.Send(ctx, tx, Message{EventID: env.ID, LeadID: env.LeadID, Channel: c, TemplateKey: key, Language: used, Body: body}); err != nil {
			return err
		}
	}
	s.a.Log.Info("message sent", "leadId", env.LeadID, "template", key, "language", used, "channels", chs)
	return nil
}

func (s *server) pings(w http.ResponseWriter, r *http.Request) {
	rows, err := s.a.DB.Query(r.Context(), `SELECT event_id, message, received_at FROM ping_log ORDER BY received_at DESC LIMIT 20`)
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
}

func (s *server) messages(w http.ResponseWriter, r *http.Request) {
	rows, err := s.a.DB.Query(r.Context(), `SELECT id, event_id, lead_id, channel, template_key, language, recipient, body, created_at
		FROM sent_messages WHERE ($1 = '' OR lead_id = $1) ORDER BY created_at DESC LIMIT 200`, r.URL.Query().Get("leadId"))
	if err != nil {
		svc.JSON(w, 500, map[string]string{"code": "INTERNAL", "message": "list failed"})
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, ev, lead, ch, key, lang, rcpt, body string
		var at time.Time
		if err := rows.Scan(&id, &ev, &lead, &ch, &key, &lang, &rcpt, &body, &at); err != nil {
			svc.JSON(w, 500, map[string]string{"code": "INTERNAL", "message": "list failed"})
			return
		}
		out = append(out, map[string]any{"id": id, "eventId": ev, "leadId": lead, "channel": ch, "templateKey": key, "language": lang,
			"recipient": rcpt, "body": body, "createdAt": at})
	}
	svc.JSON(w, 200, map[string]any{"messages": out})
}
