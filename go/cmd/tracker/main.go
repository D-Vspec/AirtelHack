// Command tracker owns orders: it opens one on quote.accepted, moves it on kyc.passed and
// kyc.failed, runs the state machine (go/pkg/orders) and serves the timeline. Every change and
// the event describing it are written in one transaction (pkg/outbox); consumers are
// idempotent. Contract: contracts/orders.schema.json.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/catalogue"
	"udyam/pkg/config"
	"udyam/pkg/events"
	"udyam/pkg/kafkax"
	"udyam/pkg/orders"
	"udyam/pkg/outbox"
	"udyam/pkg/svc"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS orders (
	id                  text PRIMARY KEY,
	lead_id             text NOT NULL UNIQUE,
	quote_id            text NOT NULL,
	status              text NOT NULL,
	status_reason       text NOT NULL DEFAULT '',
	lines               jsonb NOT NULL,
	product_ids         text[] NOT NULL,
	monthly_total_inr   int NOT NULL,
	one_time_total_inr  int NOT NULL,
	install_days        int NOT NULL,
	expected_activation date,
	fallback_offered    boolean NOT NULL DEFAULT false,
	correlation_id      text NOT NULL DEFAULT '',
	created_at          timestamptz NOT NULL,
	updated_at          timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS orders_status ON orders (status, updated_at);
CREATE TABLE IF NOT EXISTS order_events (
	seq      bigserial PRIMARY KEY,
	order_id text NOT NULL REFERENCES orders(id),
	status   text NOT NULL,
	reason   text NOT NULL DEFAULT '',
	note     text NOT NULL DEFAULT '',
	at       timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS order_events_order ON order_events (order_id, seq);
-- KYC outcome per lead, kept even when KYC finishes before the quote is accepted
CREATE TABLE IF NOT EXISTS kyc_status (
	lead_id text PRIMARY KEY,
	status  text NOT NULL,
	at      timestamptz NOT NULL
);
`

type Line struct {
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	Quantity    int    `json:"quantity"`
	InstallDays int    `json:"installDays"`
}

type Order struct {
	ID                     string    `json:"id"`
	LeadID                 string    `json:"leadId"`
	QuoteID                string    `json:"quoteId"`
	Status                 string    `json:"status"`
	StatusReason           string    `json:"statusReason,omitempty"`
	ProductIDs             []string  `json:"productIds"`
	Lines                  []Line    `json:"lines"`
	MonthlyTotalInr        int       `json:"monthlyTotalInr"`
	OneTimeTotalInr        int       `json:"oneTimeTotalInr"`
	ExpectedInstallDays    int       `json:"expectedInstallDays"`
	ExpectedActivationDate string    `json:"expectedActivationDate,omitempty"`
	FallbackOffered        bool      `json:"fallbackOffered"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
	correlationID          string
}

const orderCols = `id, lead_id, quote_id, status, status_reason, lines, product_ids, monthly_total_inr, one_time_total_inr,
	install_days, coalesce(expected_activation::text, ''), fallback_offered, correlation_id, created_at, updated_at`

func scanOrder(row pgx.Row) (*Order, error) {
	var o Order
	var lines []byte
	err := row.Scan(&o.ID, &o.LeadID, &o.QuoteID, &o.Status, &o.StatusReason, &lines, &o.ProductIDs, &o.MonthlyTotalInr, &o.OneTimeTotalInr,
		&o.ExpectedInstallDays, &o.ExpectedActivationDate, &o.FallbackOffered, &o.correlationID, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, json.Unmarshal(lines, &o.Lines)
}

type server struct {
	a        *svc.App
	rulesURL string
	mu       sync.Mutex
	p04Days  int // install days of the 5G fallback product, read once from the rules service
}

func main() {
	svc.Run(svc.Options{
		Name: "tracker", DefaultPort: "8085", Schema: "tracker", UsesKafka: true,
		Setup: func(ctx context.Context, a *svc.App) error {
			if err := outbox.Init(ctx, a.DB); err != nil {
				return err
			}
			if _, err := a.DB.Exec(ctx, schemaSQL); err != nil {
				return err
			}
			s := &server{a: a, rulesURL: config.String("RULES_URL", "http://rules:8082")}
			a.Mux.HandleFunc("GET /orders", s.list)
			a.Mux.HandleFunc("GET /orders/{id}", s.get)
			a.Mux.HandleFunc("GET /orders/{id}/timeline", s.timeline)
			a.Mux.HandleFunc("POST /orders/{id}/transitions", s.transition)
			a.Mux.HandleFunc("POST /orders/{id}/fallback/accept", s.acceptFallback)
			go outbox.Relay(ctx, a.DB, a.Producer, a.Log)
			a.Gauge("outbox_pending", outbox.PendingGauge(a.DB))
			go kafkax.Run(ctx, a.Log, a.Brokers, "tracker",
				[]string{events.TopicQuoteAccepted, events.TopicKycPassed, events.TopicKycFailed}, s.handle)
			return nil
		},
	})
}

// handle applies one event exactly once.
func (s *server) handle(ctx context.Context, topic string, env events.Envelope) error {
	done, err := outbox.Once(ctx, s.a.DB, "tracker", env, topic, func(tx pgx.Tx) error {
		switch topic {
		case events.TopicQuoteAccepted:
			var q catalogue.Quote
			if err := json.Unmarshal(env.Payload, &q); err != nil {
				return err
			}
			return s.onQuoteAccepted(ctx, tx, q, env.CorrelationID)
		case events.TopicKycPassed:
			return s.onKyc(ctx, tx, env.LeadID, "passed", env.CorrelationID)
		case events.TopicKycFailed:
			var p struct {
				Route string `json:"route"`
			}
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				return err
			}
			if p.Route != "reject" {
				return nil // a re-upload request does not change the order
			}
			return s.onKyc(ctx, tx, env.LeadID, "rejected", env.CorrelationID)
		}
		return nil
	})
	if err == nil && !done {
		s.a.Log.Info("duplicate event ignored", "topic", topic, "eventId", env.ID)
	}
	return err
}

func (s *server) onQuoteAccepted(ctx context.Context, tx pgx.Tx, q catalogue.Quote, corr string) error {
	if q.LeadID == "" {
		return fmt.Errorf("quote %s has no leadId", q.ID)
	}
	existing, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE lead_id = $1`, q.LeadID))
	if err != nil {
		return err
	}
	if existing != nil {
		s.a.Log.Info("lead already has an order; quote ignored", "leadId", q.LeadID, "orderId", existing.ID, "quoteId", q.ID)
		return nil
	}
	now := time.Now().UTC()
	o := Order{ID: events.NewID(), LeadID: q.LeadID, QuoteID: q.ID, Status: orders.QuoteAccepted, MonthlyTotalInr: q.MonthlyTotalInr,
		OneTimeTotalInr: q.OneTimeTotalInr, ExpectedInstallDays: q.ExpectedInstallDays, CreatedAt: now, UpdatedAt: now, correlationID: corr}
	for _, l := range q.Lines {
		o.ProductIDs = append(o.ProductIDs, l.ProductID)
		o.Lines = append(o.Lines, Line{ProductID: l.ProductID, ProductName: l.ProductName, Quantity: l.Quantity, InstallDays: l.InstallDays})
		o.ExpectedInstallDays = max(o.ExpectedInstallDays, l.InstallDays)
	}
	o.ExpectedActivationDate = orders.ExpectedActivation(now, o.ExpectedInstallDays)
	lines, _ := json.Marshal(o.Lines)
	if _, err := tx.Exec(ctx, `INSERT INTO orders (id, lead_id, quote_id, status, lines, product_ids, monthly_total_inr, one_time_total_inr,
		install_days, expected_activation, correlation_id, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12)`,
		o.ID, o.LeadID, o.QuoteID, o.Status, lines, o.ProductIDs, o.MonthlyTotalInr, o.OneTimeTotalInr, o.ExpectedInstallDays,
		o.ExpectedActivationDate, corr, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO order_events (order_id, status, at) VALUES ($1, $2, $3)`, o.ID, o.Status, now); err != nil {
		return err
	}
	if _, err := outbox.Enqueue(ctx, tx, events.TopicOrderCreated, o.LeadID, corr, o); err != nil {
		return err
	}
	s.a.Log.Info("order created", "orderId", o.ID, "leadId", o.LeadID, "quoteId", o.QuoteID)
	var kyc string
	err = tx.QueryRow(ctx, `SELECT status FROM kyc_status WHERE lead_id = $1`, o.LeadID).Scan(&kyc)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	switch kyc {
	case "passed":
		return s.move(ctx, tx, &o, orders.KycPassed, "kyc_passed_before_quote", "")
	case "rejected":
		return s.move(ctx, tx, &o, orders.Failed, orders.ReasonKycRejected, "")
	}
	return s.move(ctx, tx, &o, orders.KycPending, "", "")
}

func (s *server) onKyc(ctx context.Context, tx pgx.Tx, leadID, outcome, corr string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO kyc_status (lead_id, status, at) VALUES ($1, $2, now())
		ON CONFLICT (lead_id) DO UPDATE SET status = EXCLUDED.status, at = EXCLUDED.at`, leadID, outcome); err != nil {
		return err
	}
	o, err := scanOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE lead_id = $1 FOR UPDATE`, leadID))
	if err != nil || o == nil {
		return err // no order yet: the quote.accepted handler reads kyc_status
	}
	if outcome == "passed" && (o.Status == orders.KycPending || o.Status == orders.QuoteAccepted) {
		return s.move(ctx, tx, o, orders.KycPassed, "", "")
	}
	if outcome == "rejected" && !orders.Final(o.Status) {
		return s.move(ctx, tx, o, orders.Failed, orders.ReasonKycRejected, "")
	}
	return nil
}

var errInvalidTransition = errors.New("invalid transition")

// move applies one transition: order row, timeline entry and events in the caller's transaction.
func (s *server) move(ctx context.Context, tx pgx.Tx, o *Order, to, reason, note string) error {
	if !orders.CanMove(o.Status, to) {
		return fmt.Errorf("%w: %s -> %s", errInvalidTransition, o.Status, to)
	}
	from, now := o.Status, time.Now().UTC()
	if to == orders.KycPassed {
		// the install clock starts once KYC is through
		o.ExpectedActivationDate = orders.ExpectedActivation(now, o.ExpectedInstallDays)
	}
	offerFallback := to == orders.Delayed && reason == orders.ReasonFibreNotFeasible && len(orders.FibreIn(o.ProductIDs)) > 0 && !o.FallbackOffered
	o.Status, o.StatusReason, o.UpdatedAt = to, reason, now
	o.FallbackOffered = o.FallbackOffered || offerFallback
	lines, _ := json.Marshal(o.Lines)
	if _, err := tx.Exec(ctx, `UPDATE orders SET status = $2, status_reason = $3, expected_activation = $4, fallback_offered = $5,
		lines = $6, product_ids = $7, install_days = $8, updated_at = $9 WHERE id = $1`,
		o.ID, to, reason, o.ExpectedActivationDate, o.FallbackOffered, lines, o.ProductIDs, o.ExpectedInstallDays, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO order_events (order_id, status, reason, note, at) VALUES ($1,$2,$3,$4,$5)`, o.ID, to, reason, note, now); err != nil {
		return err
	}
	change := map[string]any{"orderId": o.ID, "leadId": o.LeadID, "from": from, "to": to, "reason": reason,
		"expectedActivationDate": o.ExpectedActivationDate, "at": now}
	if _, err := outbox.Enqueue(ctx, tx, events.TopicOrderStatusChanged, o.LeadID, o.correlationID, change); err != nil {
		return err
	}
	if offerFallback {
		offer := map[string]any{"orderId": o.ID, "leadId": o.LeadID, "replaceProductIds": orders.FibreIn(o.ProductIDs),
			"offerProductId": orders.FallbackProduct, "offerInstallDays": s.fallbackDays(ctx), "reason": reason}
		if _, err := outbox.Enqueue(ctx, tx, events.TopicOrderFallbackOffered, o.LeadID, o.correlationID, offer); err != nil {
			return err
		}
		req := map[string]any{"leadId": o.LeadID, "templateKey": "order.fallback_offer", "channels": []string{"whatsapp", "sms"},
			"params": map[string]string{"order": orders.ShortID(o.ID)}}
		if _, err := outbox.Enqueue(ctx, tx, events.TopicNotificationRequested, o.LeadID, o.correlationID, req); err != nil {
			return err
		}
	}
	s.a.Log.Info("order moved", "orderId", o.ID, "leadId", o.LeadID, "from", from, "to", to, "reason", reason)
	return nil
}

// fallbackDays reads P04's typical_install_days from the rules service once (7 if unreachable,
// the catalogue value at the time of writing).
func (s *server) fallbackDays(ctx context.Context) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p04Days > 0 {
		return s.p04Days
	}
	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(c, http.MethodGet, s.rulesURL+"/products", nil)
	if resp, err := http.DefaultClient.Do(req); err == nil {
		defer resp.Body.Close()
		var products []catalogue.Product
		if json.NewDecoder(resp.Body).Decode(&products) == nil {
			for _, p := range products {
				if p.ID == orders.FallbackProduct {
					s.p04Days = p.InstallDays
				}
			}
		}
	}
	if s.p04Days == 0 {
		return 7
	}
	return s.p04Days
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	svc.JSON(w, status, map[string]string{"code": code, "message": msg})
}

func (s *server) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statuses := []string{} // never nil: a NULL array would make cardinality() NULL and match nothing
	if v := q.Get("status"); v != "" {
		statuses = strings.Split(v, ",")
		for _, st := range statuses {
			if !orders.Valid(st) {
				fail(w, 400, "INVALID_REQUEST", "unknown status "+st)
				return
			}
		}
	}
	rows, err := s.a.DB.Query(r.Context(), `SELECT `+orderCols+` FROM orders
		WHERE ($1 = '' OR lead_id = $1) AND (cardinality($2::text[]) = 0 OR status = ANY($2)) ORDER BY updated_at DESC LIMIT 200`,
		q.Get("leadId"), statuses)
	if err != nil {
		s.a.Log.Error("list orders", "err", err)
		fail(w, 500, "INTERNAL", "list failed")
		return
	}
	defer rows.Close()
	out := []*Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			fail(w, 500, "INTERNAL", "list failed")
			return
		}
		out = append(out, o)
	}
	svc.JSON(w, 200, map[string]any{"orders": out})
}

func (s *server) load(w http.ResponseWriter, r *http.Request) *Order {
	o, err := scanOrder(s.a.DB.QueryRow(r.Context(), `SELECT `+orderCols+` FROM orders WHERE id = $1`, r.PathValue("id")))
	if err != nil {
		fail(w, 500, "INTERNAL", "load failed")
		return nil
	}
	if o == nil {
		fail(w, 404, "NOT_FOUND", "no such order")
	}
	return o
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	if o := s.load(w, r); o != nil {
		svc.JSON(w, 200, o)
	}
}

func (s *server) timeline(w http.ResponseWriter, r *http.Request) {
	o := s.load(w, r)
	if o == nil {
		return
	}
	rows, err := s.a.DB.Query(r.Context(), `SELECT status, reason, note, at FROM order_events WHERE order_id = $1 ORDER BY seq`, o.ID)
	if err != nil {
		fail(w, 500, "INTERNAL", "timeline failed")
		return
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (orders.Entry, error) {
		var e orders.Entry
		err := row.Scan(&e.Status, &e.Reason, &e.Note, &e.At)
		return e, err
	})
	if err != nil {
		fail(w, 500, "INTERNAL", "timeline failed")
		return
	}
	svc.JSON(w, 200, map[string]any{"order": o, "entries": entries, "steps": orders.Steps(o.Status, entries)})
}

// withOrder locks the order row and runs fn in one transaction.
func (s *server) withOrder(w http.ResponseWriter, r *http.Request, fn func(tx pgx.Tx, o *Order) error) {
	var out *Order
	err := pgx.BeginFunc(r.Context(), s.a.DB, func(tx pgx.Tx) error {
		o, err := scanOrder(tx.QueryRow(r.Context(), `SELECT `+orderCols+` FROM orders WHERE id = $1 FOR UPDATE`, r.PathValue("id")))
		if err != nil {
			return err
		}
		if o == nil {
			return pgx.ErrNoRows
		}
		out = o
		return fn(tx, o)
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		fail(w, 404, "NOT_FOUND", "no such order")
	case errors.Is(err, errInvalidTransition):
		fail(w, 409, "INVALID_TRANSITION", err.Error())
	case err != nil:
		s.a.Log.Error("order update failed", "err", err)
		fail(w, 500, "INTERNAL", "update failed")
	default:
		svc.JSON(w, 200, out)
	}
}

func (s *server) transition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To     string `json:"to"`
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	if !orders.Valid(req.To) || len(req.Reason) > 64 || len(req.Note) > 500 {
		fail(w, 400, "INVALID_REQUEST", "to must be a known status; reason max 64, note max 500")
		return
	}
	s.withOrder(w, r, func(tx pgx.Tx, o *Order) error { return s.move(r.Context(), tx, o, req.To, req.Reason, req.Note) })
}

// acceptFallback swaps the fibre lines for 5G AirFiber after a fibre_not_feasible delay.
func (s *server) acceptFallback(w http.ResponseWriter, r *http.Request) {
	days := s.fallbackDays(r.Context())
	s.withOrder(w, r, func(tx pgx.Tx, o *Order) error {
		if o.Status != orders.Delayed || !o.FallbackOffered {
			return fmt.Errorf("%w: no 5G fallback offer is open on this order", errInvalidTransition)
		}
		fibre := orders.FibreIn(o.ProductIDs)
		lines := []Line{}
		install := days
		for _, l := range o.Lines {
			if !slices.Contains(fibre, l.ProductID) {
				lines = append(lines, l)
				install = max(install, l.InstallDays)
			}
		}
		o.Lines = append([]Line{{ProductID: orders.FallbackProduct, ProductName: "Biz 5G AirFiber 100", Quantity: 1, InstallDays: days}}, lines...)
		o.ProductIDs = nil
		for _, l := range o.Lines {
			o.ProductIDs = append(o.ProductIDs, l.ProductID)
		}
		o.ExpectedInstallDays = install
		o.ExpectedActivationDate = orders.ExpectedActivation(time.Now(), install)
		return s.move(r.Context(), tx, o, orders.InstallScheduled, orders.ReasonFallbackAccepted, "fibre replaced by 5G AirFiber; monthly price to be re-confirmed from the quote service")
	})
}
