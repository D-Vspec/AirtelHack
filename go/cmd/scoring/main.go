// Command scoring serves lead propensity scores (go/pkg/scoring) and the partner handoff queue.
// It scores live leads from lead.created and advisor.profile.updated (emitting lead.scored),
// opens a partner task on lead.unclosed, and can load the case data's newest month as a demo
// pipeline for the sales "morning list". Contract: contracts/scoring.schema.json.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/config"
	"udyam/pkg/events"
	"udyam/pkg/kafkax"
	"udyam/pkg/outbox"
	"udyam/pkg/scoring"
	"udyam/pkg/svc"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS leads (
	lead_id       text PRIMARY KEY,
	source        text NOT NULL,             -- live | case
	business_name text NOT NULL DEFAULT '',
	city          text NOT NULL DEFAULT '',
	features      jsonb NOT NULL,
	probability   double precision NOT NULL,
	band          text NOT NULL,
	reasons       jsonb NOT NULL,
	model         text NOT NULL,
	updated_at    timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS leads_rank ON leads (source, probability DESC);
CREATE TABLE IF NOT EXISTS partner_tasks (
	id          text PRIMARY KEY,
	lead_id     text NOT NULL,
	reason      text NOT NULL,
	summary     text NOT NULL,
	transcript  jsonb NOT NULL,
	status      text NOT NULL,
	partner     text NOT NULL DEFAULT '',
	created_at  timestamptz NOT NULL,
	updated_at  timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS partner_tasks_status ON partner_tasks (status, created_at);
`

// The advisor journey is a new digital channel that the case data does not have. It is scored as
// a website form, the closest historical source (4.3% conversion), which keeps its scores modest
// until there is outcome data for the new channel (docs/model-card.md).
const digitalChannelSource = "Website form"

type server struct {
	a *svc.App
	m *scoring.Model
}

func main() {
	m, err := scoring.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	svc.Run(svc.Options{
		Name: "scoring", DefaultPort: "8087", Schema: "scoring", UsesKafka: true,
		Setup: func(ctx context.Context, a *svc.App) error {
			if err := outbox.Init(ctx, a.DB); err != nil {
				return err
			}
			if _, err := a.DB.Exec(ctx, schemaSQL); err != nil {
				return err
			}
			s := &server{a: a, m: m}
			if path := config.String("SCORING_IMPORT_CSV", ""); path != "" {
				if n, err := s.importCase(ctx, path); err != nil {
					a.Log.Error("case import failed", "path", path, "err", err)
				} else if n > 0 {
					a.Log.Info("case pipeline scored", "leads", n)
				}
			}
			a.Mux.HandleFunc("POST /score", s.score)
			a.Mux.HandleFunc("GET /leads", s.leads)
			a.Mux.HandleFunc("GET /tasks", s.tasks)
			a.Mux.HandleFunc("POST /tasks/{id}/claim", s.claim)
			a.Mux.HandleFunc("POST /tasks/{id}/close", s.close)
			go outbox.Relay(ctx, a.DB, a.Producer, a.Log)
			go kafkax.Run(ctx, a.Log, a.Brokers, "scoring",
				[]string{events.TopicLeadCreated, events.TopicAdvisorProfileUpdated, events.TopicLeadUnclosed}, s.handle)
			a.Log.Info("model loaded", "version", m.Version, "trees", len(m.Trees))
			return nil
		},
	})
}

func (s *server) handle(ctx context.Context, topic string, env events.Envelope) error {
	if env.LeadID == "" {
		return nil
	}
	_, err := outbox.Once(ctx, s.a.DB, "scoring", env, topic, func(tx pgx.Tx) error {
		switch topic {
		case events.TopicLeadCreated:
			return s.upsert(ctx, tx, env.LeadID, env.CorrelationID, func(l *scoring.Lead) { l.LeadSource = digitalChannelSource })
		case events.TopicAdvisorProfileUpdated:
			var p struct {
				Profile json.RawMessage `json:"profile"`
			}
			if err := json.Unmarshal(env.Payload, &p); err != nil {
				return err
			}
			var prof scoring.Lead // profile field names match the model's inputs; extra fields are ignored
			if err := json.Unmarshal(p.Profile, &prof); err != nil {
				return err
			}
			return s.upsert(ctx, tx, env.LeadID, env.CorrelationID, func(l *scoring.Lead) {
				src := l.LeadSource
				*l = prof
				l.LeadSource = src
				if l.LeadSource == "" {
					l.LeadSource = digitalChannelSource
				}
			})
		case events.TopicLeadUnclosed:
			return s.openTask(ctx, tx, env)
		}
		return nil
	})
	return err
}

// upsert applies a change to a live lead's features, re-scores it, and emits lead.scored when
// the score moved.
func (s *server) upsert(ctx context.Context, tx pgx.Tx, leadID, corr string, change func(*scoring.Lead)) error {
	var l scoring.Lead
	var raw []byte
	var prev float64 = -1
	err := tx.QueryRow(ctx, `SELECT features, probability FROM leads WHERE lead_id = $1 FOR UPDATE`, leadID).Scan(&raw, &prev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &l); err != nil {
			return err
		}
	}
	change(&l)
	sc := s.m.Score(l)
	feat, _ := json.Marshal(l)
	reasons, _ := json.Marshal(sc.Reasons)
	if _, err := tx.Exec(ctx, `INSERT INTO leads (lead_id, source, features, probability, band, reasons, model, updated_at)
		VALUES ($1, 'live', $2, $3, $4, $5, $6, now())
		ON CONFLICT (lead_id) DO UPDATE SET features = EXCLUDED.features, probability = EXCLUDED.probability, band = EXCLUDED.band,
		reasons = EXCLUDED.reasons, model = EXCLUDED.model, updated_at = EXCLUDED.updated_at`,
		leadID, feat, sc.Probability, sc.Band, reasons, sc.Model); err != nil {
		return err
	}
	if math.Abs(sc.Probability-prev) < 1e-9 {
		return nil
	}
	s.a.Log.Info("lead scored", "leadId", leadID, "probability", math.Round(sc.Probability*1000)/1000, "band", sc.Band)
	_, err = outbox.Enqueue(ctx, tx, events.TopicLeadScored, leadID, corr, map[string]any{
		"leadId": leadID, "probability": sc.Probability, "band": sc.Band, "reasons": sc.Reasons, "model": sc.Model})
	return err
}

func (s *server) openTask(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	var p struct {
		Reason     string          `json:"reason"`
		Summary    string          `json:"summary"`
		Transcript json.RawMessage `json:"transcript"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return err
	}
	if len(p.Transcript) == 0 {
		p.Transcript = json.RawMessage("[]")
	}
	// one open task per lead: a second request updates it
	tag, err := tx.Exec(ctx, `UPDATE partner_tasks SET reason = $2, summary = $3, transcript = $4, updated_at = now()
		WHERE lead_id = $1 AND status <> 'closed'`, env.LeadID, p.Reason, p.Summary, p.Transcript)
	if err != nil || tag.RowsAffected() > 0 {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO partner_tasks (id, lead_id, reason, summary, transcript, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'open', now(), now())`, events.NewID(), env.LeadID, p.Reason, p.Summary, p.Transcript)
	if err == nil {
		s.a.Log.Info("partner task opened", "leadId", env.LeadID, "reason", p.Reason)
	}
	return err
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	svc.JSON(w, status, map[string]string{"code": code, "message": msg})
}

func (s *server) score(w http.ResponseWriter, r *http.Request) {
	var l scoring.Lead
	if !svc.DecodeJSON(w, r, &l) {
		return
	}
	svc.JSON(w, 200, s.m.Score(l))
}

type scoredLead struct {
	LeadID       string           `json:"leadId"`
	Source       string           `json:"source"`
	BusinessName string           `json:"businessName"`
	City         string           `json:"city"`
	Segment      string           `json:"segment"`
	LeadSource   string           `json:"leadSource"`
	Probability  float64          `json:"probability"`
	Band         string           `json:"band"`
	Reasons      []scoring.Reason `json:"reasons"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

func (s *server) leads(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	source := r.URL.Query().Get("source")
	if source != "" && source != "live" && source != "case" {
		fail(w, 400, "INVALID_REQUEST", "source must be live or case")
		return
	}
	rows, err := s.a.DB.Query(r.Context(), `SELECT lead_id, source, business_name, city, features, probability, band, reasons, updated_at
		FROM leads WHERE ($1 = '' OR source = $1) ORDER BY probability DESC, lead_id LIMIT $2`, source, limit)
	if err != nil {
		fail(w, 500, "INTERNAL", "list failed")
		return
	}
	defer rows.Close()
	out := []scoredLead{}
	for rows.Next() {
		var l scoredLead
		var feat, reasons []byte
		if err := rows.Scan(&l.LeadID, &l.Source, &l.BusinessName, &l.City, &feat, &l.Probability, &l.Band, &reasons, &l.UpdatedAt); err != nil {
			fail(w, 500, "INTERNAL", "list failed")
			return
		}
		var f scoring.Lead
		_ = json.Unmarshal(feat, &f)
		_ = json.Unmarshal(reasons, &l.Reasons)
		l.Segment, l.LeadSource = f.Segment, f.LeadSource
		out = append(out, l)
	}
	svc.JSON(w, 200, map[string]any{"leads": out})
}

type task struct {
	ID          string           `json:"id"`
	LeadID      string           `json:"leadId"`
	Reason      string           `json:"reason"`
	Summary     string           `json:"summary"`
	Transcript  json.RawMessage  `json:"transcript"`
	Status      string           `json:"status"`
	Partner     string           `json:"partner,omitempty"`
	Probability *float64         `json:"probability,omitempty"`
	Band        string           `json:"band,omitempty"`
	Reasons     []scoring.Reason `json:"reasons,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
}

const taskQuery = `SELECT t.id, t.lead_id, t.reason, t.summary, t.transcript, t.status, t.partner, t.created_at, l.probability, coalesce(l.band, ''), l.reasons
	FROM partner_tasks t LEFT JOIN leads l ON l.lead_id = t.lead_id`

func scanTask(row pgx.Row) (*task, error) {
	var t task
	var reasons []byte
	if err := row.Scan(&t.ID, &t.LeadID, &t.Reason, &t.Summary, &t.Transcript, &t.Status, &t.Partner, &t.CreatedAt, &t.Probability, &t.Band, &reasons); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(reasons, &t.Reasons)
	return &t, nil
}

// tasks lists partner tasks, highest-scoring leads first: partners call the likeliest buyers first.
func (s *server) tasks(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "claimed" && status != "closed" {
		fail(w, 400, "INVALID_REQUEST", "status must be open, claimed or closed")
		return
	}
	rows, err := s.a.DB.Query(r.Context(), taskQuery+` WHERE ($1 = '' OR t.status = $1) ORDER BY l.probability DESC NULLS LAST, t.created_at LIMIT 200`, status)
	if err != nil {
		fail(w, 500, "INTERNAL", "list failed")
		return
	}
	defer rows.Close()
	out := []*task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			fail(w, 500, "INTERNAL", "list failed")
			return
		}
		out = append(out, t)
	}
	svc.JSON(w, 200, map[string]any{"tasks": out})
}

func (s *server) setStatus(w http.ResponseWriter, r *http.Request, from []string, to, partner string) {
	tag, err := s.a.DB.Exec(r.Context(), `UPDATE partner_tasks SET status = $2, partner = CASE WHEN $3 = '' THEN partner ELSE $3 END, updated_at = now()
		WHERE id = $1 AND status = ANY($4)`, r.PathValue("id"), to, partner, from)
	if err != nil {
		fail(w, 500, "INTERNAL", "update failed")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "TASK_STATE", "task not found or not in a state that allows this")
		return
	}
	t, err := scanTask(s.a.DB.QueryRow(r.Context(), taskQuery+` WHERE t.id = $1`, r.PathValue("id")))
	if err != nil {
		fail(w, 500, "INTERNAL", "load failed")
		return
	}
	svc.JSON(w, 200, t)
}

func (s *server) claim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Partner string `json:"partner"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	if req.Partner == "" || len(req.Partner) > 64 {
		fail(w, 400, "INVALID_REQUEST", "partner is required")
		return
	}
	s.setStatus(w, r, []string{"open"}, "claimed", req.Partner)
}

func (s *server) close(w http.ResponseWriter, r *http.Request) {
	s.setStatus(w, r, []string{"open", "claimed"}, "closed", "")
}

// importCase scores the case data's newest month (the model's test month) as the demo pipeline.
// Runs once: skipped when case leads are already present.
func (s *server) importCase(ctx context.Context, path string) (int, error) {
	var n int
	if err := s.a.DB.QueryRow(ctx, `SELECT count(*) FROM leads WHERE source = 'case'`).Scan(&n); err != nil || n > 0 {
		return 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return 0, err
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	latest := ""
	for _, r := range rows[1:] {
		if m := r[col["created_date"]][:7]; m > latest {
			latest = m
		}
	}
	num := func(s string) *float64 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		return &v
	}
	yes := func(s string) *bool { b := s == "Y"; return &b }
	batch := &pgx.Batch{}
	for _, r := range rows[1:] {
		get := func(c string) string { return r[col[c]] }
		if !strings.HasPrefix(get("created_date"), latest) {
			continue
		}
		var interests []string
		for _, p := range strings.Split(get("products_of_interest"), ";") {
			if p != "" {
				interests = append(interests, p)
			}
		}
		l := scoring.Lead{LeadSource: get("lead_source"), Segment: get("segment"), PreferredLanguage: get("preferred_language"),
			CurrentInternet: get("current_internet"), CurrentProvider: get("current_provider"), CityTier: num(get("city_tier")),
			EmployeeCount: num(get("employee_count")), NumSites: num(get("num_sites")), AnnualTurnoverBand: get("annual_turnover_band"),
			HasGSTIN: yes(get("has_gstin")), HasUdyam: yes(get("has_udyam")), MonthlyTelecomSpendInr: num(get("monthly_telecom_spend_inr")),
			ProductsOfInterest: interests}
		sc := s.m.Score(l)
		feat, _ := json.Marshal(l)
		reasons, _ := json.Marshal(sc.Reasons)
		batch.Queue(`INSERT INTO leads (lead_id, source, business_name, city, features, probability, band, reasons, model, updated_at)
			VALUES ($1, 'case', $2, $3, $4, $5, $6, $7, $8, now()) ON CONFLICT (lead_id) DO NOTHING`,
			get("lead_id"), get("business_name"), get("city"), feat, sc.Probability, sc.Band, reasons, sc.Model)
	}
	if err := s.a.DB.SendBatch(ctx, batch).Close(); err != nil {
		return 0, err
	}
	return batch.Len(), nil
}
