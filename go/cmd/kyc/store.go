package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"udyam/pkg/kyc"
	"udyam/pkg/outbox"
)

// Tables live in the kyc schema (pg.Connect sets search_path). Images are kept only until a
// final decision, consent withdrawal or the retention window, whichever comes first.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS consents (
	id           text PRIMARY KEY,
	lead_id      text NOT NULL,
	purposes     text[] NOT NULL,
	version      text NOT NULL,
	language     text NOT NULL DEFAULT '',
	granted_at   timestamptz NOT NULL,
	withdrawn_at timestamptz
);
CREATE INDEX IF NOT EXISTS consents_lead ON consents (lead_id, granted_at DESC);
CREATE TABLE IF NOT EXISTS documents (
	lead_id      text NOT NULL,
	doc_type     text NOT NULL,
	content      bytea NOT NULL,
	content_type text NOT NULL,
	uploaded_at  timestamptz NOT NULL,
	PRIMARY KEY (lead_id, doc_type)
);
CREATE TABLE IF NOT EXISTS applications (
	lead_id        text PRIMARY KEY,
	status         text NOT NULL,
	result         jsonb,
	review_id      text,
	correlation_id text NOT NULL DEFAULT '',
	updated_at     timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS reviews (
	id          text PRIMARY KEY,
	lead_id     text NOT NULL,
	status      text NOT NULL,
	codes       text[] NOT NULL,
	decision    text,
	reviewer    text,
	note        text,
	created_at  timestamptz NOT NULL,
	decided_at  timestamptz
);
CREATE INDEX IF NOT EXISTS reviews_status ON reviews (status, created_at);
CREATE TABLE IF NOT EXISTS resume_links (
	token       text PRIMARY KEY,
	lead_id     text NOT NULL,
	created_at  timestamptz NOT NULL,
	expires_at  timestamptz NOT NULL,
	reason      text NOT NULL
);
CREATE INDEX IF NOT EXISTS resume_links_lead ON resume_links (lead_id, created_at DESC);
`

type Consent struct {
	ID          string     `json:"id"`
	LeadID      string     `json:"leadId"`
	Purposes    []string   `json:"purposes"`
	Version     string     `json:"version"`
	Language    string     `json:"language,omitempty"`
	GrantedAt   time.Time  `json:"grantedAt"`
	WithdrawnAt *time.Time `json:"withdrawnAt"`
}

type Review struct {
	ID        string        `json:"id"`
	LeadID    string        `json:"leadId"`
	Status    string        `json:"status"`
	Codes     []string      `json:"codes"`
	Findings  []kyc.Finding `json:"findings,omitempty"`
	Decision  *string       `json:"decision,omitempty"`
	Reviewer  *string       `json:"reviewer,omitempty"`
	Note      *string       `json:"note,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	DecidedAt *time.Time    `json:"decidedAt,omitempty"`
}

type store struct{ db *pgxpool.Pool }

// evt is an event written to the outbox in the same transaction as the change it describes.
type evt struct {
	topic, lead, corr string
	payload           any
}

func enqueueAll(ctx context.Context, tx pgx.Tx, evs []evt) error {
	for _, e := range evs {
		if _, err := outbox.Enqueue(ctx, tx, e.topic, e.lead, e.corr, e.payload); err != nil {
			return err
		}
	}
	return nil
}

// activeConsent returns the latest consent for the lead if it is not withdrawn.
func (s store) activeConsent(ctx context.Context, leadID string) (*Consent, error) {
	var c Consent
	err := s.db.QueryRow(ctx, `SELECT id, lead_id, purposes, version, language, granted_at, withdrawn_at FROM consents
		WHERE lead_id = $1 ORDER BY granted_at DESC LIMIT 1`, leadID).
		Scan(&c.ID, &c.LeadID, &c.Purposes, &c.Version, &c.Language, &c.GrantedAt, &c.WithdrawnAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s store) insertConsent(ctx context.Context, c Consent, evs ...evt) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO consents (id, lead_id, purposes, version, language, granted_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			c.ID, c.LeadID, c.Purposes, c.Version, c.Language, c.GrantedAt); err != nil {
			return err
		}
		return enqueueAll(ctx, tx, evs)
	})
}

// withdraw marks consent withdrawn and purges every document, result and open review.
func (s store) withdraw(ctx context.Context, consentID, leadID string, at time.Time, event func(purged int64) evt) (int64, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE consents SET withdrawn_at = $2 WHERE id = $1`, consentID, at); err != nil {
		return 0, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM documents WHERE lead_id = $1`, leadID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM applications WHERE lead_id = $1`, leadID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM reviews WHERE lead_id = $1`, leadID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM resume_links WHERE lead_id = $1`, leadID); err != nil {
		return 0, err
	}
	if err := enqueueAll(ctx, tx, []evt{event(tag.RowsAffected())}); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), tx.Commit(ctx)
}

func (s store) putDocument(ctx context.Context, leadID, docType, contentType string, data []byte, at time.Time) error {
	_, err := s.db.Exec(ctx, `INSERT INTO documents (lead_id, doc_type, content, content_type, uploaded_at) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (lead_id, doc_type) DO UPDATE SET content = EXCLUDED.content, content_type = EXCLUDED.content_type, uploaded_at = EXCLUDED.uploaded_at`,
		leadID, docType, data, contentType, at)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO applications (lead_id, status, updated_at) VALUES ($1, 'collecting', $2) ON CONFLICT (lead_id) DO NOTHING`, leadID, at)
	return err
}

func (s store) documents(ctx context.Context, leadID string) (map[string][]byte, error) {
	rows, err := s.db.Query(ctx, `SELECT doc_type, content FROM documents WHERE lead_id = $1`, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var t string
		var b []byte
		if err := rows.Scan(&t, &b); err != nil {
			return nil, err
		}
		out[t] = b
	}
	return out, rows.Err()
}

func (s store) documentTypes(ctx context.Context, leadID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT doc_type FROM documents WHERE lead_id = $1 ORDER BY doc_type`, leadID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s store) purgeDocuments(ctx context.Context, leadID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM documents WHERE lead_id = $1`, leadID)
	return err
}

// purgeExpired deletes images older than the retention window.
func (s store) purgeExpired(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM documents WHERE uploaded_at < $1`, before)
	return tag.RowsAffected(), err
}

type application struct {
	LeadID        string
	Status        string
	Result        *kyc.Result
	ReviewID      *string
	CorrelationID string
}

func (s store) application(ctx context.Context, leadID string) (*application, error) {
	var a application
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT lead_id, status, result, review_id, correlation_id FROM applications WHERE lead_id = $1`, leadID).
		Scan(&a.LeadID, &a.Status, &raw, &a.ReviewID, &a.CorrelationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if raw != nil {
		a.Result = &kyc.Result{}
		if err := json.Unmarshal(raw, a.Result); err != nil {
			return nil, err
		}
	}
	return &a, nil
}

// saveResult stores a check result and, for manual_review, opens a review in one transaction.
func (s store) saveResult(ctx context.Context, leadID, correlationID string, res kyc.Result, review *Review, evs []evt) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var reviewID *string
	if review != nil {
		// one open review per lead: a re-check supersedes the earlier one
		if _, err := tx.Exec(ctx, `DELETE FROM reviews WHERE lead_id = $1 AND status = 'open'`, leadID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO reviews (id, lead_id, status, codes, created_at) VALUES ($1,$2,'open',$3,$4)`,
			review.ID, leadID, review.Codes, review.CreatedAt); err != nil {
			return err
		}
		reviewID = &review.ID
	}
	_, err = tx.Exec(ctx, `INSERT INTO applications (lead_id, status, result, review_id, correlation_id, updated_at) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (lead_id) DO UPDATE SET status = EXCLUDED.status, result = EXCLUDED.result, review_id = EXCLUDED.review_id,
		correlation_id = EXCLUDED.correlation_id, updated_at = EXCLUDED.updated_at`,
		leadID, res.Route, res, reviewID, correlationID, res.CheckedAt)
	if err != nil {
		return err
	}
	if err := enqueueAll(ctx, tx, evs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s store) reviews(ctx context.Context, status string) ([]Review, error) {
	rows, err := s.db.Query(ctx, `SELECT id, lead_id, status, codes, decision, reviewer, note, created_at, decided_at FROM reviews
		WHERE ($1 = '' OR status = $1) ORDER BY created_at LIMIT 200`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.LeadID, &r.Status, &r.Codes, &r.Decision, &r.Reviewer, &r.Note, &r.CreatedAt, &r.DecidedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s store) review(ctx context.Context, id string) (*Review, error) {
	var r Review
	err := s.db.QueryRow(ctx, `SELECT id, lead_id, status, codes, decision, reviewer, note, created_at, decided_at FROM reviews WHERE id = $1`, id).
		Scan(&r.ID, &r.LeadID, &r.Status, &r.Codes, &r.Decision, &r.Reviewer, &r.Note, &r.CreatedAt, &r.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}

// decide closes an open review and sets the application status to the decision's route.
// Returns false when the review was already decided.
func (s store) decide(ctx context.Context, id, decision, reviewer, note, status string, at time.Time, event func(leadID, corr string) evt) (bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var leadID string
	err = tx.QueryRow(ctx, `UPDATE reviews SET status = 'decided', decision = $2, reviewer = $3, note = $4, decided_at = $5
		WHERE id = $1 AND status = 'open' RETURNING lead_id`, id, decision, reviewer, note, at).Scan(&leadID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var corr string
	if err := tx.QueryRow(ctx, `UPDATE applications SET status = $2, updated_at = $3 WHERE lead_id = $1 RETURNING correlation_id`, leadID, status, at).Scan(&corr); err != nil {
		return false, err
	}
	if err := enqueueAll(ctx, tx, []evt{event(leadID, corr)}); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
