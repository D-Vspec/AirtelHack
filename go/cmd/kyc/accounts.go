package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"udyam/pkg/events"
	"udyam/pkg/kyc"
	"udyam/pkg/logx"
	"udyam/pkg/svc"
)

// Customer accounts: a returning owner signs in with their mobile number and a one-time code,
// sees every order they placed, and starts a new one. A new order reuses the account's earlier
// verification when kyc.ReusePolicy says nothing relevant changed; otherwise the full pipeline
// runs again. Codes and session tokens are stored only as SHA-256 hashes.

const accountsSQL = `
CREATE TABLE IF NOT EXISTS accounts (
	id         text PRIMARY KEY,
	mobile     text NOT NULL UNIQUE,
	created_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS account_leads (
	lead_id    text PRIMARY KEY,
	account_id text NOT NULL REFERENCES accounts(id),
	linked_at  timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS account_leads_account ON account_leads (account_id, linked_at DESC);
CREATE TABLE IF NOT EXISTS otps (
	mobile     text PRIMARY KEY,
	code_hash  text NOT NULL,
	created_at timestamptz NOT NULL,
	expires_at timestamptz NOT NULL,
	attempts   int NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS account_sessions (
	token_hash text PRIMARY KEY,
	account_id text NOT NULL REFERENCES accounts(id),
	expires_at timestamptz NOT NULL
);
ALTER TABLE applications ADD COLUMN IF NOT EXISTS reused_from text;
`

const (
	otpTTL         = 5 * time.Minute
	otpResendAfter = 30 * time.Second
	otpMaxAttempts = 5
	sessionTTL     = 30 * 24 * time.Hour
)

var mobileRe = regexp.MustCompile(`^[6-9][0-9]{9}$`)

func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

func normMobile(s string) string {
	s = strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "+91")
	if len(s) == 11 && s[0] == '0' {
		s = s[1:]
	}
	return s
}

func maskMobile(m string) string {
	if len(m) < 4 {
		return "****"
	}
	return strings.Repeat("•", len(m)-4) + m[len(m)-4:]
}

type Account struct {
	ID           string    `json:"id"`
	MobileMasked string    `json:"mobileMasked"`
	CreatedAt    time.Time `json:"createdAt"`
}

type LinkedLead struct {
	LeadID    string    `json:"leadId"`
	LinkedAt  time.Time `json:"linkedAt"`
	KycStatus string    `json:"kycStatus"` // none | collecting | pass | manual_review | reupload | reject
	ReusedKyc bool      `json:"reusedKyc"`
}

func (s *server) accountRoutes() {
	m := s.a.Mux
	m.HandleFunc("POST /accounts/otp", s.requestOTP)
	m.HandleFunc("POST /accounts/verify", s.verifyOTP)
	m.HandleFunc("GET /accounts/me", s.me)
	m.HandleFunc("POST /accounts/me/leads", s.linkLead)
	m.HandleFunc("POST /accounts/me/logout", s.logout)
	m.HandleFunc("GET /accounts/me/kyc-reuse", s.reuseStatus)
	m.HandleFunc("POST /applications/{leadId}/reuse", s.reuse)
}

func (s *server) requestOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mobile string `json:"mobile"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	mobile := normMobile(req.Mobile)
	if !mobileRe.MatchString(mobile) {
		fail(w, 400, "INVALID_REQUEST", "enter a 10-digit Indian mobile number")
		return
	}
	ctx := r.Context()
	var last time.Time
	err := s.a.DB.QueryRow(ctx, `SELECT created_at FROM otps WHERE mobile = $1`, mobile).Scan(&last)
	if err == nil && time.Since(last) < otpResendAfter {
		fail(w, 429, "OTP_TOO_SOON", "wait a few seconds before asking for another code")
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		s.internal(w, "otp generation failed", err)
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	now := time.Now().UTC()
	if _, err := s.a.DB.Exec(ctx, `INSERT INTO otps (mobile, code_hash, created_at, expires_at, attempts) VALUES ($1,$2,$3,$4,0)
		ON CONFLICT (mobile) DO UPDATE SET code_hash = EXCLUDED.code_hash, created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at, attempts = 0`,
		mobile, hash(mobile+":"+code), now, now.Add(otpTTL)); err != nil {
		s.internal(w, "otp store failed", err)
		return
	}
	s.a.Log.Info("sign-in code issued", "mobile", logx.MaskID(mobile))
	out := map[string]any{"sent": true, "mobileMasked": maskMobile(mobile), "expiresInSeconds": int(otpTTL.Seconds())}
	// Prototype: no SMS gateway is wired, so the demo shows the code on screen. Off in production.
	if s.demoOTP {
		out["demoCode"] = code
	}
	svc.JSON(w, 200, out)
}

func (s *server) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mobile string `json:"mobile"`
		Code   string `json:"code"`
		LeadID string `json:"leadId"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	mobile := normMobile(req.Mobile)
	if !mobileRe.MatchString(mobile) || len(req.Code) != 6 || (req.LeadID != "" && !leadIDRe.MatchString(req.LeadID)) {
		fail(w, 400, "INVALID_REQUEST", "mobile, a 6-digit code and an optional leadId are required")
		return
	}
	ctx := r.Context()
	var token string
	var acct Account
	err := pgx.BeginFunc(ctx, s.a.DB, func(tx pgx.Tx) error {
		var codeHash string
		var exp time.Time
		var attempts int
		err := tx.QueryRow(ctx, `SELECT code_hash, expires_at, attempts FROM otps WHERE mobile = $1 FOR UPDATE`, mobile).Scan(&codeHash, &exp, &attempts)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (time.Now().After(exp) || attempts >= otpMaxAttempts)) {
			return errCode{401, "OTP_INVALID", "the code has expired; ask for a new one"}
		}
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(codeHash), []byte(hash(mobile+":"+req.Code))) != 1 {
			if _, err := tx.Exec(ctx, `UPDATE otps SET attempts = attempts + 1 WHERE mobile = $1`, mobile); err != nil {
				return err
			}
			return errCode{401, "OTP_INVALID", "that code is not right; check the message and try again"}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM otps WHERE mobile = $1`, mobile); err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.QueryRow(ctx, `INSERT INTO accounts (id, mobile, created_at) VALUES ($1,$2,$3)
			ON CONFLICT (mobile) DO UPDATE SET mobile = EXCLUDED.mobile RETURNING id, created_at`, events.NewID(), mobile, now).Scan(&acct.ID, &acct.CreatedAt); err != nil {
			return err
		}
		acct.MobileMasked = maskMobile(mobile)
		if req.LeadID != "" {
			if err := link(ctx, tx, acct.ID, req.LeadID, now); err != nil {
				return err
			}
		}
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		token = hex.EncodeToString(b)
		_, err = tx.Exec(ctx, `INSERT INTO account_sessions (token_hash, account_id, expires_at) VALUES ($1,$2,$3)`, hash(token), acct.ID, now.Add(sessionTTL))
		return err
	})
	if s.writeErr(w, "sign-in failed", err) {
		return
	}
	s.a.Log.Info("signed in", "account", acct.ID, "linkedLead", req.LeadID)
	svc.JSON(w, 200, map[string]any{"token": token, "account": acct})
}

// link attaches a lead to the account; a lead already on another account is refused.
func link(ctx context.Context, tx pgx.Tx, accountID, leadID string, at time.Time) error {
	var owner string
	err := tx.QueryRow(ctx, `INSERT INTO account_leads (lead_id, account_id, linked_at) VALUES ($1,$2,$3)
		ON CONFLICT (lead_id) DO UPDATE SET lead_id = EXCLUDED.lead_id RETURNING account_id`, leadID, accountID, at).Scan(&owner)
	if err != nil {
		return err
	}
	if owner != accountID {
		return errCode{409, "LEAD_TAKEN", "this order belongs to another account"}
	}
	return nil
}

type errCode struct {
	status      int
	code, label string
}

func (e errCode) Error() string { return e.code }

// writeErr answers err (an errCode or an internal error); false when err is nil.
func (s *server) writeErr(w http.ResponseWriter, what string, err error) bool {
	if err == nil {
		return false
	}
	var ec errCode
	if errors.As(err, &ec) {
		fail(w, ec.status, ec.code, ec.label)
	} else {
		s.internal(w, what, err)
	}
	return true
}

// account resolves the Bearer token; it answers 401 itself when there is none.
func (s *server) account(w http.ResponseWriter, r *http.Request) (*Account, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(tok) != 64 {
		fail(w, 401, "SIGN_IN_REQUIRED", "sign in with your mobile number")
		return nil, false
	}
	var a Account
	var mobile string
	err := s.a.DB.QueryRow(r.Context(), `SELECT a.id, a.mobile, a.created_at FROM account_sessions s JOIN accounts a ON a.id = s.account_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, hash(tok)).Scan(&a.ID, &mobile, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 401, "SIGN_IN_REQUIRED", "your sign-in has expired; sign in again")
		return nil, false
	}
	if err != nil {
		s.internal(w, "session lookup failed", err)
		return nil, false
	}
	a.MobileMasked = maskMobile(mobile)
	return &a, true
}

func (s *server) leads(ctx context.Context, accountID string) ([]LinkedLead, error) {
	rows, err := s.a.DB.Query(ctx, `SELECT l.lead_id, l.linked_at, coalesce(a.status, 'none'), a.reused_from IS NOT NULL
		FROM account_leads l LEFT JOIN applications a ON a.lead_id = l.lead_id WHERE l.account_id = $1 ORDER BY l.linked_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (LinkedLead, error) {
		var l LinkedLead
		return l, row.Scan(&l.LeadID, &l.LinkedAt, &l.KycStatus, &l.ReusedKyc)
	})
	if out == nil {
		out = []LinkedLead{}
	}
	return out, err
}

// verification is the account's latest passed application, if any.
type verification struct {
	LeadID     string
	Result     kyc.Result
	VerifiedAt time.Time
}

func (s *server) latestVerification(ctx context.Context, accountID, exceptLead string) (*verification, error) {
	var v verification
	var raw []byte
	err := s.a.DB.QueryRow(ctx, `SELECT a.lead_id, a.result, a.updated_at FROM applications a JOIN account_leads l ON l.lead_id = a.lead_id
		WHERE l.account_id = $1 AND a.status = 'pass' AND a.result IS NOT NULL AND a.lead_id <> $2 ORDER BY a.updated_at DESC LIMIT 1`, accountID, exceptLead).
		Scan(&v.LeadID, &raw, &v.VerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &v.Result); err != nil {
		return nil, err
	}
	return &v, nil
}

// verifiedView is the masked summary shown to the owner.
func verifiedView(v *verification) map[string]any {
	if v == nil {
		return nil
	}
	out := map[string]any{"fromLeadId": v.LeadID, "verifiedAt": v.VerifiedAt, "gstinMasked": logx.MaskID(v.Result.Verified.GSTIN), "udyamMasked": logx.MaskID(v.Result.Verified.UdyamNumber)}
	if g := v.Result.Verified.GSTINRecord; g != nil {
		out["legalName"], out["tradeName"], out["address"] = g.LegalName, g.TradeName, g.Address
	}
	return out
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	a, ok := s.account(w, r)
	if !ok {
		return
	}
	ls, err := s.leads(r.Context(), a.ID)
	if err != nil {
		s.internal(w, "lead list failed", err)
		return
	}
	v, err := s.latestVerification(r.Context(), a.ID, "")
	if err != nil {
		s.internal(w, "verification lookup failed", err)
		return
	}
	svc.JSON(w, 200, map[string]any{"account": a, "leads": ls, "verified": verifiedView(v)})
}

func (s *server) linkLead(w http.ResponseWriter, r *http.Request) {
	a, ok := s.account(w, r)
	if !ok {
		return
	}
	var req struct {
		LeadID string `json:"leadId"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	if !leadIDRe.MatchString(req.LeadID) {
		fail(w, 400, "INVALID_REQUEST", "leadId must be 1-64 characters of letters, digits, _ . : -")
		return
	}
	err := pgx.BeginFunc(r.Context(), s.a.DB, func(tx pgx.Tx) error { return link(r.Context(), tx, a.ID, req.LeadID, time.Now().UTC()) })
	if s.writeErr(w, "link failed", err) {
		return
	}
	svc.JSON(w, 200, map[string]any{"leadId": req.LeadID, "accountId": a.ID})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if _, err := s.a.DB.Exec(r.Context(), `DELETE FROM account_sessions WHERE token_hash = $1`, hash(tok)); err != nil {
		s.internal(w, "logout failed", err)
		return
	}
	svc.JSON(w, 200, map[string]bool{"signedOut": true})
}

// reuseStatus answers whether a new order could reuse the earlier verification (assuming the
// owner confirms nothing changed), with the reasons when it could not.
func (s *server) reuseStatus(w http.ResponseWriter, r *http.Request) {
	a, ok := s.account(w, r)
	if !ok {
		return
	}
	except := r.URL.Query().Get("leadId")
	v, err := s.latestVerification(r.Context(), a.ID, except)
	if err != nil {
		s.internal(w, "verification lookup failed", err)
		return
	}
	var prev *kyc.Result
	at := time.Time{}
	if v != nil {
		prev, at = &v.Result, v.VerifiedAt
	}
	d := s.reusePolicy.Decide(r.Context(), s.reg, prev, at, true, time.Now().UTC())
	svc.JSON(w, 200, map[string]any{"reusable": d.Reusable, "reasons": d.Reasons, "verified": verifiedView(v)})
}

// reuse passes KYC for a new order from the account's earlier verification.
func (s *server) reuse(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	a, ok := s.account(w, r)
	if !ok {
		return
	}
	var req struct {
		Unchanged bool `json:"unchanged"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	var owner string
	if err := s.a.DB.QueryRow(ctx, `SELECT account_id FROM account_leads WHERE lead_id = $1`, lead).Scan(&owner); err != nil || owner != a.ID {
		fail(w, 403, "NOT_YOUR_ORDER", "this order is not on your account")
		return
	}
	if _, ok := s.requireConsent(w, r, lead); !ok {
		return
	}
	if cur, err := s.st.application(ctx, lead); err == nil && cur != nil && cur.Status == "pass" {
		fail(w, 409, "ALREADY_VERIFIED", "this order is already verified")
		return
	}
	v, err := s.latestVerification(ctx, a.ID, lead)
	if err != nil {
		s.internal(w, "verification lookup failed", err)
		return
	}
	var prev *kyc.Result
	at := time.Time{}
	if v != nil {
		prev, at = &v.Result, v.VerifiedAt
	}
	now := time.Now().UTC()
	d := s.reusePolicy.Decide(ctx, s.reg, prev, at, req.Unchanged, now)
	if !d.Reusable {
		s.a.Log.Info("kyc reuse refused", "leadId", lead, "decision", d.String())
		svc.JSON(w, 409, map[string]any{"code": "REUSE_NOT_AVAILABLE", "message": "please verify your documents again", "reasons": d.Reasons})
		return
	}
	res := kyc.Reused(v.Result, now)
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = events.NewID()
	}
	ev := evt{events.TopicKycPassed, lead, corr, map[string]any{"leadId": lead, "decidedBy": "reuse", "reusedFrom": v.LeadID, "gstinMasked": logx.MaskID(res.Verified.GSTIN)}}
	if err := s.st.saveResult(ctx, lead, corr, res, nil, []evt{ev}); err != nil {
		s.internal(w, "result store failed", err)
		return
	}
	if _, err := s.a.DB.Exec(ctx, `UPDATE applications SET reused_from = $2 WHERE lead_id = $1`, lead, v.LeadID); err != nil {
		s.a.Log.Error("reused_from update failed", "leadId", lead, "err", err)
	}
	s.a.Log.Info("kyc reused", "leadId", lead, "from", v.LeadID, "gstin", logx.MaskID(res.Verified.GSTIN))
	out := applicationView(lead, res.Route, []string{}, &res, nil)
	out["reusedFrom"] = v.LeadID
	svc.JSON(w, 200, out)
}
