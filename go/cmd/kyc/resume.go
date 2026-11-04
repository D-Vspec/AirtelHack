package main

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/events"
	"udyam/pkg/kyc"
	"udyam/pkg/svc"
)

// Resume links bring owners back to an unfinished KYC. The case data loses leads to "missing
// document", "paperwork too long" and "signatory not available". A link reopens the KYC screen
// with what is still missing; it carries no personal data, only a random token.

const resumeTTL = 7 * 24 * time.Hour

// createResumeLink stores a token and asks notify to send it, in one transaction.
func (s *server) createResumeLink(ctx context.Context, lead, reason string) (string, time.Time, error) {
	token := events.NewID()
	now := time.Now().UTC()
	exp := now.Add(resumeTTL)
	err := pgx.BeginFunc(ctx, s.a.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO resume_links (token, lead_id, created_at, expires_at, reason) VALUES ($1,$2,$3,$4,$5)`,
			token, lead, now, exp, reason); err != nil {
			return err
		}
		return enqueueAll(ctx, tx, []evt{{events.TopicNotificationRequested, lead, "", map[string]any{
			"leadId": lead, "templateKey": "kyc.resume", "channels": []string{"whatsapp", "sms"},
			"params": map[string]string{"link": s.resumeURL + token}}}})
	})
	return token, exp, err
}

func (s *server) resumeLink(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	if _, ok := s.requireConsent(w, r, lead); !ok {
		return
	}
	token, exp, err := s.createResumeLink(r.Context(), lead, "requested")
	if err != nil {
		s.internal(w, "resume link failed", err)
		return
	}
	s.a.Log.Info("resume link sent", "leadId", lead)
	svc.JSON(w, 201, map[string]any{"token": token, "url": s.resumeURL + token, "expiresAt": exp})
}

// resume tells the UI where the owner left off: documents still to upload and findings that
// need a re-upload.
func (s *server) resume(w http.ResponseWriter, r *http.Request) {
	var lead string
	var exp time.Time
	err := s.a.DB.QueryRow(r.Context(), `SELECT lead_id, expires_at FROM resume_links WHERE token = $1`, r.PathValue("token")).Scan(&lead, &exp)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "NOT_FOUND", "unknown link")
		return
	}
	if err != nil {
		s.internal(w, "resume lookup failed", err)
		return
	}
	if time.Now().After(exp) {
		fail(w, 410, "LINK_EXPIRED", "this link has expired; ask for a new one")
		return
	}
	docs, err := s.st.documentTypes(r.Context(), lead)
	if err != nil {
		s.internal(w, "document list failed", err)
		return
	}
	app, err := s.st.application(r.Context(), lead)
	if err != nil {
		s.internal(w, "application load failed", err)
		return
	}
	status, todo := "collecting", []kyc.Finding{}
	if app != nil {
		status = app.Status
		if app.Result != nil {
			for _, f := range app.Result.Findings {
				if f.Route == events.RouteReupload {
					todo = append(todo, f)
				}
			}
		}
	}
	missing := []string{}
	for _, d := range kyc.DocTypes {
		if !slices.Contains(docs, d) {
			missing = append(missing, d)
		}
	}
	svc.JSON(w, 200, map[string]any{"leadId": lead, "status": status, "documents": docs, "missingDocuments": missing, "actionNeeded": todo, "expiresAt": exp})
}

// abandonSweep sends one resume link to each application left in collecting or reupload for
// longer than `after` (KYC_ABANDON_AFTER_MINUTES).
func (s *server) abandonSweep(ctx context.Context, after time.Duration) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		leads, err := s.abandoned(ctx, after)
		for _, lead := range leads {
			if _, _, err := s.createResumeLink(ctx, lead, "abandoned"); err != nil {
				s.a.Log.Error("abandoned resume link failed", "leadId", lead, "err", err)
			} else {
				s.a.Log.Info("abandoned KYC: resume link sent", "leadId", lead)
			}
		}
		if err != nil && ctx.Err() == nil {
			s.a.Log.Error("abandon sweep failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *server) abandoned(ctx context.Context, after time.Duration) ([]string, error) {
	rows, err := s.a.DB.Query(ctx, `SELECT a.lead_id FROM applications a
		WHERE a.status IN ('collecting', 'reupload') AND a.updated_at < $1
		AND NOT EXISTS (SELECT 1 FROM resume_links l WHERE l.lead_id = a.lead_id AND l.created_at > a.updated_at)
		LIMIT 100`, time.Now().Add(-after))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
