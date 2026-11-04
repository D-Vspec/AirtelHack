// Command kyc is the Smart KYC service (contracts/kyc-api.schema.json): purpose-limited consent,
// document upload with live per-document feedback, the deterministic check pipeline (go/pkg/kyc),
// GSTIN prefill, and a manual review queue. Verdicts are code; nothing here asks an LLM.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/config"
	"udyam/pkg/events"
	"udyam/pkg/kyc"
	"udyam/pkg/logx"
	"udyam/pkg/registry"
	"udyam/pkg/svc"
)

const maxUpload = 10 << 20

var purposes = []string{"kyc_verification", "service_activation"}

// registryClient is kyc.Registry over the registry service (GET /gstin/{id}, /udyam/{id}; 404 = no match).
type registryClient struct {
	base string
	http *http.Client
}

func getMatch[T any](ctx context.Context, c registryClient, path string) (registry.Match[T], error) {
	var m registry.Match[T]
	req, err := http.NewRequestWithContext(ctx, "GET", c.base+path, nil)
	if err != nil {
		return m, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		return m, fmt.Errorf("registry %d", resp.StatusCode)
	}
	return m, json.NewDecoder(resp.Body).Decode(&m)
}

func (c registryClient) GSTIN(ctx context.Context, id string) (registry.Match[registry.GSTIN], error) {
	return getMatch[registry.GSTIN](ctx, c, "/gstin/"+url.PathEscape(id))
}
func (c registryClient) Udyam(ctx context.Context, id string) (registry.Match[registry.Udyam], error) {
	return getMatch[registry.Udyam](ctx, c, "/udyam/"+url.PathEscape(id))
}

type checkResult struct {
	ID        string                `json:"id"`
	LeadID    string                `json:"leadId"`
	Route     string                `json:"route"`
	Findings  []kyc.Finding         `json:"findings"`
	Fields    map[string]kyc.Fields `json:"fields"`
	ReviewID  string                `json:"reviewId,omitempty"`
	CheckedAt time.Time             `json:"checkedAt"`
}

type review struct {
	ID        string        `json:"id"`
	CheckID   string        `json:"checkId"`
	LeadID    string        `json:"leadId"`
	Status    string        `json:"status"`
	Findings  []kyc.Finding `json:"findings"`
	Decision  string        `json:"decision,omitempty"`
	Reviewer  string        `json:"reviewer,omitempty"`
	Note      string        `json:"note,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	DecidedAt *time.Time    `json:"decidedAt,omitempty"`
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	svc.JSON(w, status, map[string]string{"code": code, "message": msg})
}

// minimise drops identifiers the checks no longer need once they have run.
func minimise(fields map[string]kyc.Fields) {
	for _, f := range fields {
		delete(f, "idNo")
		delete(f, "consumerNo")
	}
}

func codes(fs []kyc.Finding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func main() {
	reg := registryClient{base: config.String("REGISTRY_URL", "http://registry:8081"), http: &http.Client{Timeout: 5 * time.Second}}
	ref, err := time.Parse(time.DateOnly, config.String("KYC_REFERENCE_DATE", "2026-07-15"))
	if err != nil {
		panic("KYC_REFERENCE_DATE must be YYYY-MM-DD")
	}
	retention := time.Duration(config.Int("KYC_RETENTION_DAYS", 30)) * 24 * time.Hour
	checker := kyc.NewChecker(reg, kyc.Tesseract, ref)

	svc.Run(svc.Options{
		Name: "kyc", DefaultPort: "8084", Schema: "kyc", UsesKafka: true,
		Setup: func(ctx context.Context, a *svc.App) error {
			if _, err := a.DB.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS consents (
					lead_id      text PRIMARY KEY,
					purposes     text[] NOT NULL,
					version      text NOT NULL,
					language     text,
					granted_at   timestamptz NOT NULL,
					withdrawn_at timestamptz);
				CREATE TABLE IF NOT EXISTS documents (
					lead_id      text NOT NULL,
					doc_type     text NOT NULL,
					content      bytea NOT NULL,
					content_type text NOT NULL,
					uploaded_at  timestamptz NOT NULL DEFAULT now(),
					PRIMARY KEY (lead_id, doc_type));
				ALTER TABLE documents ADD COLUMN IF NOT EXISTS read jsonb;
				CREATE TABLE IF NOT EXISTS checks (
					id         text PRIMARY KEY,
					lead_id    text NOT NULL,
					route      text NOT NULL,
					result     jsonb NOT NULL,
					checked_at timestamptz NOT NULL);
				CREATE INDEX IF NOT EXISTS checks_lead ON checks (lead_id, checked_at DESC);
				CREATE TABLE IF NOT EXISTS reviews (
					id         text PRIMARY KEY,
					check_id   text NOT NULL,
					lead_id    text NOT NULL,
					status     text NOT NULL,
					findings   jsonb NOT NULL,
					decision   text,
					reviewer   text,
					note       text,
					created_at timestamptz NOT NULL,
					decided_at timestamptz);`); err != nil {
				return err
			}

			// ponytail: publish after commit without an outbox (lost if Kafka is down); Phase 5 adds the outbox.
			publish := func(ctx context.Context, topic, leadID, corr string, payload any) {
				env, err := events.New(topic, leadID, corr, payload)
				if err == nil {
					err = a.Producer.Publish(ctx, topic, env)
				}
				if err != nil {
					a.Log.Error("publish failed", "topic", topic, "leadId", leadID, "err", err)
				}
			}
			consented := func(ctx context.Context, leadID string) (bool, error) {
				var ps []string
				err := a.DB.QueryRow(ctx, `SELECT purposes FROM consents WHERE lead_id = $1 AND withdrawn_at IS NULL`, leadID).Scan(&ps)
				if errors.Is(err, pgx.ErrNoRows) {
					return false, nil
				}
				return slices.Contains(ps, "kyc_verification"), err
			}

			// Retention: stored images are deleted after KYC_RETENTION_DAYS (parsed fields and findings stay with the check).
			go func() {
				for t := time.NewTicker(time.Hour); ; {
					tag, err := a.DB.Exec(ctx, `DELETE FROM documents WHERE uploaded_at < $1`, time.Now().Add(-retention))
					if err != nil {
						a.Log.Error("retention sweep failed", "err", err)
					} else if tag.RowsAffected() > 0 {
						a.Log.Info("retention sweep", "deletedDocuments", tag.RowsAffected())
					}
					select {
					case <-ctx.Done():
						return
					case <-t.C:
					}
				}
			}()

			a.Mux.HandleFunc("POST /consents", func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					LeadID   string   `json:"leadId"`
					Purposes []string `json:"purposes"`
					Version  string   `json:"version"`
					Language string   `json:"language"`
				}
				if !svc.DecodeJSON(w, r, &req) {
					return
				}
				if req.LeadID == "" || req.Version == "" || len(req.Purposes) == 0 {
					fail(w, 400, "INVALID_REQUEST", "leadId, purposes and version are required")
					return
				}
				for _, p := range req.Purposes {
					if !slices.Contains(purposes, p) {
						fail(w, 400, "INVALID_REQUEST", "unknown purpose "+p)
						return
					}
				}
				now := time.Now().UTC()
				if _, err := a.DB.Exec(r.Context(), `INSERT INTO consents (lead_id, purposes, version, language, granted_at) VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT (lead_id) DO UPDATE SET purposes = $2, version = $3, language = $4, granted_at = $5, withdrawn_at = NULL`,
					req.LeadID, req.Purposes, req.Version, req.Language, now); err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				c := map[string]any{"leadId": req.LeadID, "purposes": req.Purposes, "version": req.Version, "language": req.Language, "grantedAt": now}
				publish(r.Context(), events.TopicConsentRecorded, req.LeadID, r.Header.Get("X-Request-Id"), c)
				svc.JSON(w, 201, c)
			})

			a.Mux.HandleFunc("GET /consents/{leadId}", func(w http.ResponseWriter, r *http.Request) {
				var c struct {
					LeadID      string     `json:"leadId"`
					Purposes    []string   `json:"purposes"`
					Version     string     `json:"version"`
					Language    *string    `json:"language"`
					GrantedAt   time.Time  `json:"grantedAt"`
					WithdrawnAt *time.Time `json:"withdrawnAt"`
				}
				err := a.DB.QueryRow(r.Context(), `SELECT lead_id, purposes, version, language, granted_at, withdrawn_at FROM consents WHERE lead_id = $1`,
					r.PathValue("leadId")).Scan(&c.LeadID, &c.Purposes, &c.Version, &c.Language, &c.GrantedAt, &c.WithdrawnAt)
				if errors.Is(err, pgx.ErrNoRows) {
					fail(w, 404, "NOT_FOUND", "no consent for this lead")
					return
				}
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				svc.JSON(w, 200, c)
			})

			// Withdrawal purges documents, checks (parsed personal fields) and open reviews; the consent row
			// is kept with withdrawn_at as the record that consent existed and was withdrawn.
			a.Mux.HandleFunc("DELETE /consents/{leadId}", func(w http.ResponseWriter, r *http.Request) {
				lead := r.PathValue("leadId")
				now := time.Now().UTC()
				var purged int64
				err := pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
					tag, err := tx.Exec(r.Context(), `UPDATE consents SET withdrawn_at = $2 WHERE lead_id = $1 AND withdrawn_at IS NULL`, lead, now)
					if err != nil {
						return err
					}
					if tag.RowsAffected() == 0 {
						return pgx.ErrNoRows
					}
					tag, err = tx.Exec(r.Context(), `DELETE FROM documents WHERE lead_id = $1`, lead)
					if err != nil {
						return err
					}
					purged = tag.RowsAffected()
					if _, err = tx.Exec(r.Context(), `DELETE FROM reviews WHERE lead_id = $1`, lead); err != nil {
						return err
					}
					_, err = tx.Exec(r.Context(), `DELETE FROM checks WHERE lead_id = $1`, lead)
					return err
				})
				if errors.Is(err, pgx.ErrNoRows) {
					fail(w, 404, "NOT_FOUND", "no active consent for this lead")
					return
				}
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				p := map[string]any{"leadId": lead, "withdrawnAt": now, "purgedDocuments": purged}
				publish(r.Context(), events.TopicConsentWithdrawn, lead, r.Header.Get("X-Request-Id"), p)
				svc.JSON(w, 200, p)
			})

			a.Mux.HandleFunc("PUT /leads/{leadId}/documents/{docType}", func(w http.ResponseWriter, r *http.Request) {
				lead, docType := r.PathValue("leadId"), r.PathValue("docType")
				if !slices.Contains(kyc.RequiredDocs, docType) {
					fail(w, 400, "UNKNOWN_DOCUMENT_TYPE", "docType must be one of gst_certificate, udyam_certificate, address_proof, signatory_id")
					return
				}
				ok, err := consented(r.Context(), lead)
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				if !ok {
					fail(w, 403, "CONSENT_REQUIRED", "record kyc_verification consent before uploading documents")
					return
				}
				body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxUpload))
				if err != nil {
					fail(w, 400, "UNSUPPORTED_IMAGE", "image too large (max 10 MB)")
					return
				}
				img, format, err := image.Decode(bytes.NewReader(body))
				if err != nil {
					fail(w, 400, "UNSUPPORTED_IMAGE", "body must be a PNG or JPEG image")
					return
				}
				doc, err := checker.ReadDocument(r.Context(), docType, img)
				if err != nil {
					a.Log.Error("ocr failed", "leadId", lead, "doc", docType, "err", err)
					fail(w, 502, "OCR_FAILED", "could not run OCR")
					return
				}
				minimise(map[string]kyc.Fields{docType: doc.Fields})
				// The read (fields + findings) is stored so the check does not OCR again.
				if _, err := a.DB.Exec(r.Context(), `INSERT INTO documents (lead_id, doc_type, content, content_type, read, uploaded_at) VALUES ($1, $2, $3, $4, $5, now())
					ON CONFLICT (lead_id, doc_type) DO UPDATE SET content = $3, content_type = $4, read = $5, uploaded_at = now()`, lead, docType, body, "image/"+format, doc); err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				a.Log.Info("document uploaded", "leadId", lead, "doc", docType, "findings", codes(doc.Findings))
				svc.JSON(w, 200, doc)
			})

			a.Mux.HandleFunc("POST /leads/{leadId}/check", func(w http.ResponseWriter, r *http.Request) {
				lead, corr := r.PathValue("leadId"), r.Header.Get("X-Request-Id")
				ok, err := consented(r.Context(), lead)
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				if !ok {
					fail(w, 403, "CONSENT_REQUIRED", "record kyc_verification consent first")
					return
				}
				rows, err := a.DB.Query(r.Context(), `SELECT doc_type, read FROM documents WHERE lead_id = $1 AND read IS NOT NULL`, lead)
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				docs := map[string]kyc.Document{}
				var submitted []string
				for rows.Next() {
					var t string
					var d kyc.Document
					if err := rows.Scan(&t, &d); err != nil {
						rows.Close()
						fail(w, 500, "INVALID_REQUEST", err.Error())
						return
					}
					docs[t] = d
					submitted = append(submitted, t)
				}
				rows.Close()
				if len(docs) == 0 {
					fail(w, 400, "NO_DOCUMENTS", "upload documents before checking")
					return
				}
				res := checkResult{ID: events.NewID(), LeadID: lead, CheckedAt: time.Now().UTC()}
				publish(r.Context(), events.TopicKycSubmitted, lead, corr, map[string]any{"leadId": lead, "checkId": res.ID, "documents": submitted})

				out, err := checker.Evaluate(r.Context(), docs)
				if err != nil {
					a.Log.Error("check failed", "leadId", lead, "err", err)
					fail(w, 503, "REGISTRY_UNAVAILABLE", "registry or OCR unavailable; try again")
					return
				}
				res.Route, res.Findings, res.Fields = out.Route, out.Findings, out.Fields
				if res.Findings == nil {
					res.Findings = []kyc.Finding{}
				}
				minimise(res.Fields)

				var rev *review
				if res.Route == events.RouteManualReview {
					rev = &review{ID: events.NewID(), CheckID: res.ID, LeadID: lead, Status: "open", Findings: res.Findings, CreatedAt: res.CheckedAt}
					res.ReviewID = rev.ID
				}
				err = pgx.BeginFunc(r.Context(), a.DB, func(tx pgx.Tx) error {
					if _, err := tx.Exec(r.Context(), `INSERT INTO checks (id, lead_id, route, result, checked_at) VALUES ($1, $2, $3, $4, $5)`,
						res.ID, lead, res.Route, res, res.CheckedAt); err != nil {
						return err
					}
					if rev != nil {
						_, err := tx.Exec(r.Context(), `INSERT INTO reviews (id, check_id, lead_id, status, findings, created_at) VALUES ($1, $2, $3, 'open', $4, $5)`,
							rev.ID, rev.CheckID, lead, rev.Findings, rev.CreatedAt)
						return err
					}
					return nil
				})
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}

				for _, stage := range kyc.Stages {
					var fs []map[string]string
					for _, f := range res.Findings {
						if f.Stage == stage {
							fs = append(fs, map[string]string{"code": f.Code, "route": f.Route, "doc": f.Doc})
						}
					}
					if fs != nil {
						publish(r.Context(), events.TopicKycChecked, lead, corr, map[string]any{"leadId": lead, "checkId": res.ID, "stage": stage, "findings": fs})
					}
				}
				verdict := map[string]any{"leadId": lead, "checkId": res.ID, "route": res.Route, "codes": codes(res.Findings), "decidedBy": "pipeline"}
				switch res.Route {
				case events.RoutePass:
					publish(r.Context(), events.TopicKycPassed, lead, corr, verdict)
				case events.RouteManualReview:
					publish(r.Context(), events.TopicKycReviewRequested, lead, corr, map[string]any{"leadId": lead, "checkId": res.ID, "reviewId": rev.ID, "codes": codes(res.Findings)})
				default:
					publish(r.Context(), events.TopicKycFailed, lead, corr, verdict)
				}
				a.Log.Info("kyc checked", "leadId", lead, "route", res.Route, "codes", codes(res.Findings))
				svc.JSON(w, 200, res)
			})

			a.Mux.HandleFunc("GET /leads/{leadId}/check", func(w http.ResponseWriter, r *http.Request) {
				var res checkResult
				err := a.DB.QueryRow(r.Context(), `SELECT result FROM checks WHERE lead_id = $1 ORDER BY checked_at DESC LIMIT 1`, r.PathValue("leadId")).Scan(&res)
				if errors.Is(err, pgx.ErrNoRows) {
					fail(w, 404, "NOT_FOUND", "no check for this lead")
					return
				}
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				svc.JSON(w, 200, res)
			})

			// Prefill: after the owner types a GSTIN, return what the registry knows so they type less.
			a.Mux.HandleFunc("GET /prefill/{gstin}", func(w http.ResponseWriter, r *http.Request) {
				m, err := reg.GSTIN(r.Context(), r.PathValue("gstin"))
				if err != nil {
					fail(w, 503, "REGISTRY_UNAVAILABLE", "registry unavailable")
					return
				}
				p := map[string]any{"query": m.Query, "matchType": m.MatchType, "checksumValid": registry.GSTINChecksumValid(m.Query), "suggestions": m.Suggestions}
				if g := m.Record; g != nil {
					p["gstin"], p["legalName"], p["tradeName"], p["constitution"], p["address"], p["state"], p["status"] =
						g.GSTIN, g.LegalName, g.TradeName, g.Constitution, g.Address, g.State, g.Status
				}
				a.Log.Info("prefill", "gstin", logx.MaskID(m.Query), "matchType", m.MatchType)
				svc.JSON(w, 200, p)
			})

			a.Mux.HandleFunc("GET /reviews", func(w http.ResponseWriter, r *http.Request) {
				status := r.URL.Query().Get("status")
				if status == "" {
					status = "open"
				}
				rows, err := a.DB.Query(r.Context(), `SELECT id, check_id, lead_id, status, findings, coalesce(decision, ''), coalesce(reviewer, ''), coalesce(note, ''), created_at, decided_at
					FROM reviews WHERE status = $1 ORDER BY created_at`, status)
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (review, error) {
					var v review
					err := row.Scan(&v.ID, &v.CheckID, &v.LeadID, &v.Status, &v.Findings, &v.Decision, &v.Reviewer, &v.Note, &v.CreatedAt, &v.DecidedAt)
					return v, err
				})
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				if list == nil {
					list = []review{}
				}
				svc.JSON(w, 200, list)
			})

			a.Mux.HandleFunc("POST /reviews/{id}/decision", func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Decision string `json:"decision"`
					Reviewer string `json:"reviewer"`
					Note     string `json:"note"`
				}
				if !svc.DecodeJSON(w, r, &req) {
					return
				}
				if !slices.Contains([]string{events.RoutePass, events.RouteReupload, events.RouteReject}, req.Decision) || req.Reviewer == "" {
					fail(w, 400, "INVALID_REQUEST", "decision must be pass, reupload or reject, and reviewer is required")
					return
				}
				var v review
				now := time.Now().UTC()
				err := a.DB.QueryRow(r.Context(), `UPDATE reviews SET status = 'closed', decision = $2, reviewer = $3, note = $4, decided_at = $5
					WHERE id = $1 AND status = 'open' RETURNING id, check_id, lead_id, status, findings, decision, reviewer, note, created_at, decided_at`,
					r.PathValue("id"), req.Decision, req.Reviewer, req.Note, now).
					Scan(&v.ID, &v.CheckID, &v.LeadID, &v.Status, &v.Findings, &v.Decision, &v.Reviewer, &v.Note, &v.CreatedAt, &v.DecidedAt)
				if errors.Is(err, pgx.ErrNoRows) {
					fail(w, 409, "REVIEW_CLOSED", "review not found or already decided")
					return
				}
				if err != nil {
					fail(w, 500, "INVALID_REQUEST", err.Error())
					return
				}
				verdict := map[string]any{"leadId": v.LeadID, "checkId": v.CheckID, "route": v.Decision, "codes": codes(v.Findings), "decidedBy": "reviewer"}
				topic := events.TopicKycFailed
				if v.Decision == events.RoutePass {
					topic = events.TopicKycPassed
				}
				publish(r.Context(), topic, v.LeadID, r.Header.Get("X-Request-Id"), verdict)
				svc.JSON(w, 200, v)
			})
			return nil
		},
	})
}
