// Command quote prices bundles deterministically. Eligibility and the catalogue come from the
// rules service over HTTP (no cross-schema reads); quotes are stored in the quote schema.
//
//	POST  /quotes              QuoteRequest -> Quote (201), emits quote.created
//	GET   /quotes/{id}
//	PATCH /quotes/{id}         {toggles, contractMonths} -> re-priced Quote
//	POST  /quotes/{id}/accept  -> accepted Quote, emits quote.accepted (idempotent)
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/catalogue"
	"udyam/pkg/config"
	"udyam/pkg/events"
	"udyam/pkg/svc"
)

type rulesClient struct {
	base string
	http *http.Client
}

func (c rulesClient) call(ctx context.Context, method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var e catalogue.QuoteError
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if resp.StatusCode == 400 {
			return &catalogue.QuoteError{Code: catalogue.ErrInvalidRequest, Message: e.Message}
		}
		return fmt.Errorf("rules %s: %d %s", path, resp.StatusCode, e.Message)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// price asks rules for the catalogue and the profile's eligibility, then builds the quote.
func (c rulesClient) price(ctx context.Context, req catalogue.QuoteRequest) (catalogue.Quote, []catalogue.Product, *catalogue.QuoteError) {
	if err := req.Profile.Validate(); err != nil {
		return catalogue.Quote{}, nil, &catalogue.QuoteError{Code: catalogue.ErrInvalidRequest, Message: err.Error()}
	}
	var all []catalogue.Product
	var el catalogue.Eligibility
	err := c.call(ctx, "GET", "/products", nil, &all)
	if err == nil {
		err = c.call(ctx, "POST", "/eligibility", map[string]any{"profile": req.Profile}, &el)
	}
	var qe *catalogue.QuoteError
	if errors.As(err, &qe) {
		return catalogue.Quote{}, nil, qe
	}
	if err != nil {
		return catalogue.Quote{}, nil, &catalogue.QuoteError{Code: catalogue.ErrRulesUnavailable, Message: err.Error()}
	}
	q, qerr := catalogue.BuildQuote(req, el, all)
	return q, all, qerr
}

func errStatus(e *catalogue.QuoteError) int {
	switch e.Code {
	case catalogue.ErrRulesUnavailable:
		return http.StatusBadGateway
	case catalogue.ErrQuoteNotFound:
		return http.StatusNotFound
	case catalogue.ErrQuoteAccepted, catalogue.ErrDependency:
		return http.StatusConflict
	case catalogue.ErrNotEligible, catalogue.ErrQuantity:
		return http.StatusUnprocessableEntity
	}
	return http.StatusBadRequest
}

type stored struct {
	Profile catalogue.Profile
	Quote   catalogue.Quote
}

func main() {
	rules := rulesClient{base: config.String("RULES_URL", "http://rules:8082"), http: &http.Client{Timeout: 5 * time.Second}}
	svc.Run(svc.Options{
		Name: "quote", DefaultPort: "8083", Schema: "quote", UsesKafka: true,
		Setup: func(ctx context.Context, a *svc.App) error {
			if _, err := a.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS quotes (
				id          text PRIMARY KEY,
				lead_id     text,
				status      text NOT NULL,
				profile     jsonb NOT NULL,
				quote       jsonb NOT NULL,
				created_at  timestamptz NOT NULL DEFAULT now(),
				accepted_at timestamptz)`); err != nil {
				return err
			}
			fail := func(w http.ResponseWriter, e *catalogue.QuoteError) { svc.JSON(w, errStatus(e), e) }
			get := func(ctx context.Context, id string) (stored, *catalogue.QuoteError) {
				var s stored
				err := a.DB.QueryRow(ctx, `SELECT profile, quote FROM quotes WHERE id = $1`, id).Scan(&s.Profile, &s.Quote)
				if errors.Is(err, pgx.ErrNoRows) {
					return s, &catalogue.QuoteError{Code: catalogue.ErrQuoteNotFound, Message: "no quote " + id}
				}
				if err != nil {
					return s, &catalogue.QuoteError{Code: catalogue.ErrInvalidRequest, Message: err.Error()}
				}
				return s, nil
			}
			// ponytail: publish after commit without an outbox; Phase 5 adds the outbox pattern.
			publish := func(r *http.Request, topic string, q catalogue.Quote) {
				env, err := events.New(topic, q.LeadID, r.Header.Get("X-Request-Id"), q)
				if err == nil {
					err = a.Producer.Publish(r.Context(), topic, env)
				}
				if err != nil {
					a.Log.Error("publish failed", "topic", topic, "quoteId", q.ID, "err", err)
				}
			}

			a.Mux.HandleFunc("POST /quotes", func(w http.ResponseWriter, r *http.Request) {
				var req catalogue.QuoteRequest
				if !svc.DecodeJSON(w, r, &req) {
					return
				}
				q, _, qerr := rules.price(r.Context(), req)
				if qerr != nil {
					fail(w, qerr)
					return
				}
				q.ID, q.CreatedAt = events.NewID(), time.Now().UTC()
				if _, err := a.DB.Exec(r.Context(), `INSERT INTO quotes (id, lead_id, status, profile, quote, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
					q.ID, q.LeadID, q.Status, req.Profile, q, q.CreatedAt); err != nil {
					svc.JSON(w, 500, map[string]string{"code": "STORE_FAILED", "message": err.Error()})
					return
				}
				publish(r, events.TopicQuoteCreated, q)
				svc.JSON(w, 201, q)
			})

			a.Mux.HandleFunc("GET /quotes/{id}", func(w http.ResponseWriter, r *http.Request) {
				s, qerr := get(r.Context(), r.PathValue("id"))
				if qerr != nil {
					fail(w, qerr)
					return
				}
				svc.JSON(w, 200, s.Quote)
			})

			a.Mux.HandleFunc("PATCH /quotes/{id}", func(w http.ResponseWriter, r *http.Request) {
				var patch struct {
					Toggles        []catalogue.Toggle `json:"toggles"`
					ContractMonths int                `json:"contractMonths"`
				}
				if !svc.DecodeJSON(w, r, &patch) {
					return
				}
				s, qerr := get(r.Context(), r.PathValue("id"))
				if qerr == nil && s.Quote.Status == "accepted" {
					qerr = &catalogue.QuoteError{Code: catalogue.ErrQuoteAccepted, Message: "quote already accepted"}
				}
				if qerr != nil {
					fail(w, qerr)
					return
				}
				req := catalogue.QuoteRequest{LeadID: s.Quote.LeadID, Profile: s.Profile, ContractMonths: s.Quote.ContractMonths}
				if patch.ContractMonths != 0 {
					req.ContractMonths = patch.ContractMonths
				}
				var all []catalogue.Product
				if err := rules.call(r.Context(), "GET", "/products", nil, &all); err != nil {
					fail(w, &catalogue.QuoteError{Code: catalogue.ErrRulesUnavailable, Message: err.Error()})
					return
				}
				if req.Items, qerr = catalogue.ApplyToggles(s.Quote.Lines, patch.Toggles, all); qerr != nil {
					fail(w, qerr)
					return
				}
				q, _, qerr := rules.price(r.Context(), req)
				if qerr != nil {
					fail(w, qerr)
					return
				}
				q.ID, q.CreatedAt = s.Quote.ID, s.Quote.CreatedAt
				tag, err := a.DB.Exec(r.Context(), `UPDATE quotes SET quote = $2 WHERE id = $1 AND status = 'draft'`, q.ID, q)
				if err != nil || tag.RowsAffected() == 0 {
					fail(w, &catalogue.QuoteError{Code: catalogue.ErrQuoteAccepted, Message: "quote changed or accepted meanwhile"})
					return
				}
				svc.JSON(w, 200, q)
			})

			a.Mux.HandleFunc("POST /quotes/{id}/accept", func(w http.ResponseWriter, r *http.Request) {
				s, qerr := get(r.Context(), r.PathValue("id"))
				if qerr != nil {
					fail(w, qerr)
					return
				}
				if s.Quote.Status == "accepted" {
					svc.JSON(w, 200, s.Quote) // idempotent; the event was already emitted
					return
				}
				now := time.Now().UTC()
				q := s.Quote
				q.Status, q.AcceptedAt = "accepted", &now
				tag, err := a.DB.Exec(r.Context(), `UPDATE quotes SET status = 'accepted', accepted_at = $2, quote = $3 WHERE id = $1 AND status = 'draft'`, q.ID, now, q)
				if err != nil {
					svc.JSON(w, 500, map[string]string{"code": "STORE_FAILED", "message": err.Error()})
					return
				}
				if tag.RowsAffected() == 1 {
					publish(r, events.TopicQuoteAccepted, q)
				}
				svc.JSON(w, 200, q)
			})
			return nil
		},
	})
}
