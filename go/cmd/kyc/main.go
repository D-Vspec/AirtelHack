// Command kyc is the smart KYC service: consent, document upload, the deterministic check
// pipeline (go/pkg/kyc), registry prefill and the manual review queue. Contract:
// contracts/kyc.schema.json.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"udyam/pkg/config"
	"udyam/pkg/events"
	"udyam/pkg/kyc"
	"udyam/pkg/logx"
	"udyam/pkg/registry"
	"udyam/pkg/svc"
)

const purposeKYC = "kyc_verification"

var (
	purposes   = []string{purposeKYC, "service_provisioning", "communication"}
	leadIDRe   = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)
	specimenRe = regexp.MustCompile(`^K(0[1-9]|1[0-2])$`)
)

// regClient calls the registry service; KYC never reads the registry schema directly.
type regClient struct {
	base string
	http *http.Client
}

func (c regClient) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 404 { // 404 still carries a "none" match
		return fmt.Errorf("registry %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c regClient) GSTIN(ctx context.Context, id string) (registry.GSTINMatch, error) {
	var m registry.GSTINMatch
	err := c.get(ctx, "/gstin/"+url.PathEscape(id), &m)
	return m, err
}

func (c regClient) Udyam(ctx context.Context, id string) (registry.Match[registry.Udyam], error) {
	var m registry.Match[registry.Udyam]
	err := c.get(ctx, "/udyam/"+url.PathEscape(id), &m)
	return m, err
}

type server struct {
	a         *svc.App
	st        store
	reg       kyc.Registry
	ocr       kyc.OCR
	policy    kyc.Policy
	specimens string
}

func main() {
	svc.Run(svc.Options{
		Name: "kyc", DefaultPort: "8084", Schema: "kyc", UsesKafka: true,
		Setup: func(ctx context.Context, a *svc.App) error {
			if _, err := a.DB.Exec(ctx, schemaSQL); err != nil {
				return fmt.Errorf("create tables: %w", err)
			}
			ref, err := time.Parse("2006-01-02", config.String("KYC_REFERENCE_DATE", "2026-07-15"))
			if err != nil {
				return fmt.Errorf("KYC_REFERENCE_DATE: %w", err)
			}
			s := &server{
				a: a, st: store{a.DB},
				reg:       regClient{base: config.String("REGISTRY_URL", "http://registry:8081"), http: &http.Client{Timeout: 5 * time.Second}},
				ocr:       kyc.Tesseract{},
				policy:    kyc.DefaultPolicy(ref),
				specimens: config.String("KYC_SPECIMEN_DIR", "/data/kyc_documents"),
			}
			s.routes()
			go s.retention(ctx, time.Duration(config.Int("KYC_DOC_RETENTION_DAYS", 30))*24*time.Hour)
			return nil
		},
	})
}

func (s *server) routes() {
	m := s.a.Mux
	m.HandleFunc("POST /consents", s.createConsent)
	m.HandleFunc("GET /consents/{leadId}", s.getConsent)
	m.HandleFunc("POST /consents/{leadId}/withdraw", s.withdrawConsent)
	m.HandleFunc("GET /prefill/{gstin}", s.prefill)
	m.HandleFunc("PUT /applications/{leadId}/documents/{type}", s.upload)
	m.HandleFunc("POST /applications/{leadId}/specimens/{applicant}", s.loadSpecimen)
	m.HandleFunc("POST /applications/{leadId}/check", s.check)
	m.HandleFunc("GET /applications/{leadId}", s.getApplication)
	m.HandleFunc("GET /reviews", s.listReviews)
	m.HandleFunc("GET /reviews/{id}", s.getReview)
	m.HandleFunc("POST /reviews/{id}/decision", s.decide)
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	svc.JSON(w, status, map[string]string{"code": code, "message": msg})
}

func (s *server) internal(w http.ResponseWriter, what string, err error) {
	s.a.Log.Error(what, "err", err)
	fail(w, 500, "INTERNAL", what)
}

func (s *server) publish(ctx context.Context, topic, leadID, correlationID string, payload any) {
	env, err := events.New(topic, leadID, correlationID, payload)
	if err == nil {
		err = s.a.Producer.Publish(ctx, topic, env)
	}
	if err != nil {
		// ponytail: no outbox yet (Phase 5); the DB write already happened.
		s.a.Log.Error("publish failed", "topic", topic, "leadId", leadID, "err", err)
	}
}

func leadID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("leadId")
	if !leadIDRe.MatchString(id) {
		fail(w, 400, "INVALID_REQUEST", "leadId must be 1-64 characters of letters, digits, _ . : -")
		return "", false
	}
	return id, true
}

// requireConsent answers 403 unless the lead has an active consent covering KYC verification.
func (s *server) requireConsent(w http.ResponseWriter, r *http.Request, lead string) (*Consent, bool) {
	c, err := s.st.activeConsent(r.Context(), lead)
	if err != nil {
		s.internal(w, "consent lookup failed", err)
		return nil, false
	}
	if c == nil || c.WithdrawnAt != nil || !slices.Contains(c.Purposes, purposeKYC) {
		fail(w, 403, "CONSENT_REQUIRED", "record consent with purpose kyc_verification before uploading documents")
		return nil, false
	}
	return c, true
}

func (s *server) createConsent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LeadID   string   `json:"leadId"`
		Purposes []string `json:"purposes"`
		Version  string   `json:"version"`
		Language string   `json:"language"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	if !leadIDRe.MatchString(req.LeadID) || req.Version == "" || len(req.Version) > 32 || len(req.Purposes) == 0 || len(req.Language) > 8 {
		fail(w, 400, "INVALID_REQUEST", "leadId, version (max 32) and at least one purpose are required")
		return
	}
	for _, p := range req.Purposes {
		if !slices.Contains(purposes, p) {
			fail(w, 400, "INVALID_REQUEST", "unknown purpose "+p)
			return
		}
	}
	c := Consent{ID: events.NewID(), LeadID: req.LeadID, Purposes: req.Purposes, Version: req.Version, Language: req.Language, GrantedAt: time.Now().UTC()}
	if err := s.st.insertConsent(r.Context(), c); err != nil {
		s.internal(w, "consent insert failed", err)
		return
	}
	s.a.Log.Info("consent recorded", "leadId", c.LeadID, "purposes", c.Purposes, "version", c.Version)
	s.publish(r.Context(), events.TopicConsentRecorded, c.LeadID, r.Header.Get("X-Correlation-Id"), c)
	svc.JSON(w, 201, c)
}

func (s *server) getConsent(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	c, err := s.st.activeConsent(r.Context(), lead)
	if err != nil {
		s.internal(w, "consent lookup failed", err)
		return
	}
	if c == nil {
		fail(w, 404, "NOT_FOUND", "no consent for this lead")
		return
	}
	svc.JSON(w, 200, c)
}

func (s *server) withdrawConsent(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	c, err := s.st.activeConsent(r.Context(), lead)
	if err != nil {
		s.internal(w, "consent lookup failed", err)
		return
	}
	if c == nil || c.WithdrawnAt != nil {
		fail(w, 404, "NOT_FOUND", "no active consent for this lead")
		return
	}
	at := time.Now().UTC()
	n, err := s.st.withdraw(r.Context(), c.ID, lead, at)
	if err != nil {
		s.internal(w, "withdraw failed", err)
		return
	}
	out := map[string]any{"consentId": c.ID, "leadId": lead, "withdrawnAt": at, "purgedDocuments": n}
	s.a.Log.Info("consent withdrawn; documents purged", "leadId", lead, "purged", n)
	s.publish(r.Context(), events.TopicConsentWithdrawn, lead, r.Header.Get("X-Correlation-Id"), out)
	svc.JSON(w, 200, out)
}

// prefill returns registry data for a GSTIN the owner typed, so they upload less.
func (s *server) prefill(w http.ResponseWriter, r *http.Request) {
	m, err := s.reg.GSTIN(r.Context(), r.PathValue("gstin"))
	if err != nil {
		s.a.Log.Error("registry unavailable", "err", err)
		fail(w, 503, "REGISTRY_UNAVAILABLE", "registry lookup failed; try again")
		return
	}
	s.a.Log.Info("prefill lookup", "gstin", logx.MaskID(m.Query), "matchType", m.MatchType)
	out := map[string]any{"matchType": m.MatchType, "checksumValid": m.ChecksumValid, "suggestionCount": len(m.Suggestions)}
	if m.Record == nil {
		svc.JSON(w, 404, out)
		return
	}
	g := m.Record
	for k, v := range map[string]string{"gstin": g.GSTIN, "legalName": g.LegalName, "tradeName": g.TradeName, "constitution": g.Constitution,
		"address": g.Address, "state": g.State, "status": g.Status} {
		out[k] = v
	}
	svc.JSON(w, 200, out)
}

func validDocType(t string) bool { return slices.Contains(kyc.DocTypes, t) }

// sniff accepts PNG and JPEG only, by magic bytes rather than the client's Content-Type.
func sniff(b []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", true
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", true
	}
	return "", false
}

func (s *server) upload(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	doc := r.PathValue("type")
	if !validDocType(doc) {
		fail(w, 400, "INVALID_REQUEST", "document type must be one of gst_certificate, udyam_certificate, address_proof, signatory_id")
		return
	}
	if _, ok := s.requireConsent(w, r, lead); !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, kyc.MaxImageBytes))
	if err != nil {
		fail(w, 413, "TOO_LARGE", "image must be at most 8 MB")
		return
	}
	ct, ok := sniff(data)
	if !ok {
		fail(w, 415, "UNSUPPORTED_MEDIA", "upload a PNG or JPEG image")
		return
	}
	s.store(w, r, lead, doc, ct, data)
}

// store saves one document and answers with instant image-quality feedback.
func (s *server) store(w http.ResponseWriter, r *http.Request, lead, doc, ct string, data []byte) {
	q, err := kyc.Quality(data)
	if err != nil {
		fail(w, 415, "UNSUPPORTED_MEDIA", "image could not be decoded")
		return
	}
	if err := s.st.putDocument(r.Context(), lead, doc, ct, data, time.Now().UTC()); err != nil {
		s.internal(w, "document store failed", err)
		return
	}
	s.a.Log.Info("document uploaded", "leadId", lead, "type", doc, "bytes", len(data), "blurry", q.Blurry)
	svc.JSON(w, 200, map[string]any{"leadId": lead, "type": doc, "bytes": len(data), "sharpness": q.Sharpness, "blurry": q.Blurry,
		"skewDeg": q.SkewDeg, "findings": q.Findings(doc)})
}

// loadSpecimen copies one of the case's 12 specimen applicants into a lead (demo path).
func (s *server) loadSpecimen(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	app := r.PathValue("applicant")
	if !specimenRe.MatchString(app) {
		fail(w, 400, "INVALID_REQUEST", "applicant must be K01..K12")
		return
	}
	if _, ok := s.requireConsent(w, r, lead); !ok {
		return
	}
	var loaded []string
	for _, d := range kyc.DocTypes {
		b, err := os.ReadFile(filepath.Join(s.specimens, app+"_"+d+".png"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			s.internal(w, "specimen read failed", err)
			return
		}
		if err := s.st.putDocument(r.Context(), lead, d, "image/png", b, time.Now().UTC()); err != nil {
			s.internal(w, "document store failed", err)
			return
		}
		loaded = append(loaded, d)
	}
	svc.JSON(w, 200, map[string]any{"leadId": lead, "applicant": app, "documents": loaded})
}

func (s *server) check(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	if _, ok := s.requireConsent(w, r, lead); !ok {
		return
	}
	ctx := r.Context()
	docs, err := s.st.documents(ctx, lead)
	if err != nil {
		s.internal(w, "document load failed", err)
		return
	}
	corr := r.Header.Get("X-Correlation-Id")
	if corr == "" {
		corr = events.NewID()
	}
	types := make([]string, 0, len(docs))
	for t := range docs {
		types = append(types, t)
	}
	slices.Sort(types)
	s.publish(ctx, events.TopicKycSubmitted, lead, corr, map[string]any{"leadId": lead, "documents": types})

	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	res, err := kyc.Run(runCtx, s.ocr, s.reg, s.policy, docs)
	if err != nil {
		s.a.Log.Error("kyc pipeline failed", "leadId", lead, "err", err)
		fail(w, 503, "REGISTRY_UNAVAILABLE", "the check could not finish; try again")
		return
	}
	var review *Review
	if res.Route == events.RouteManualReview {
		review = &Review{ID: events.NewID(), LeadID: lead, Status: "open", Codes: res.Codes(), CreatedAt: time.Now().UTC()}
	}
	if err := s.st.saveResult(ctx, lead, corr, res, review); err != nil {
		s.internal(w, "result store failed", err)
		return
	}
	// images are only needed until a final decision
	if res.Route == events.RoutePass || res.Route == events.RouteReject {
		if err := s.st.purgeDocuments(ctx, lead); err != nil {
			s.a.Log.Error("purge after decision failed", "leadId", lead, "err", err)
		}
	}
	s.a.Log.Info("kyc checked", "leadId", lead, "route", res.Route, "codes", res.Codes(), "gstin", logx.MaskID(res.Verified.GSTIN))

	byStage := map[string][]kyc.Finding{}
	for _, f := range res.Findings {
		byStage[f.Stage] = append(byStage[f.Stage], f)
	}
	for _, st := range kyc.Stages {
		if fs := byStage[st]; len(fs) > 0 {
			s.publish(ctx, events.TopicKycChecked, lead, corr, map[string]any{"leadId": lead, "stage": st, "findings": fs})
		}
	}
	switch res.Route {
	case events.RoutePass:
		s.publish(ctx, events.TopicKycPassed, lead, corr, map[string]any{"leadId": lead, "decidedBy": "pipeline", "gstinMasked": logx.MaskID(res.Verified.GSTIN)})
	case events.RouteManualReview:
		s.publish(ctx, events.TopicKycReviewRequested, lead, corr, map[string]any{"leadId": lead, "reviewId": review.ID, "codes": review.Codes})
	default:
		s.publish(ctx, events.TopicKycFailed, lead, corr, map[string]any{"leadId": lead, "route": res.Route, "codes": res.Codes(), "decidedBy": "pipeline"})
	}
	var reviewID *string
	if review != nil {
		reviewID = &review.ID
	}
	remaining, _ := s.st.documentTypes(ctx, lead)
	svc.JSON(w, 200, applicationView(lead, res.Route, remaining, &res, reviewID))
}

func (s *server) getApplication(w http.ResponseWriter, r *http.Request) {
	lead, ok := leadID(w, r)
	if !ok {
		return
	}
	a, err := s.st.application(r.Context(), lead)
	if err != nil {
		s.internal(w, "application load failed", err)
		return
	}
	if a == nil {
		fail(w, 404, "NOT_FOUND", "no KYC application for this lead")
		return
	}
	docs, err := s.st.documentTypes(r.Context(), lead)
	if err != nil {
		s.internal(w, "document list failed", err)
		return
	}
	svc.JSON(w, 200, applicationView(lead, a.Status, docs, a.Result, a.ReviewID))
}

// applicationView is the API shape; identifiers are masked.
func applicationView(lead, status string, docs []string, res *kyc.Result, reviewID *string) map[string]any {
	out := map[string]any{"leadId": lead, "status": status, "documents": docs}
	if docs == nil {
		out["documents"] = []string{}
	}
	if reviewID != nil {
		out["reviewId"] = *reviewID
	}
	if res == nil {
		return out
	}
	v := res.Verified
	v.GSTIN, v.UdyamNumber = logx.MaskID(v.GSTIN), logx.MaskID(v.UdyamNumber)
	if v.GSTINRecord != nil {
		g := *v.GSTINRecord
		g.GSTIN = logx.MaskID(g.GSTIN)
		v.GSTINRecord = &g
	}
	if v.UdyamRecord != nil {
		u := *v.UdyamRecord
		u.UdyamNumber = logx.MaskID(u.UdyamNumber)
		v.UdyamRecord = &u
	}
	out["route"], out["findings"], out["verified"], out["checkedAt"] = res.Route, res.Findings, v, res.CheckedAt
	return out
}

func (s *server) listReviews(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "decided" {
		fail(w, 400, "INVALID_REQUEST", "status must be open or decided")
		return
	}
	rs, err := s.st.reviews(r.Context(), status)
	if err != nil {
		s.internal(w, "review list failed", err)
		return
	}
	svc.JSON(w, 200, map[string]any{"reviews": rs})
}

func (s *server) getReview(w http.ResponseWriter, r *http.Request) {
	rv, err := s.st.review(r.Context(), r.PathValue("id"))
	if err != nil {
		s.internal(w, "review load failed", err)
		return
	}
	if rv == nil {
		fail(w, 404, "NOT_FOUND", "no such review")
		return
	}
	if a, err := s.st.application(r.Context(), rv.LeadID); err == nil && a != nil && a.Result != nil {
		rv.Findings = a.Result.Findings
	}
	svc.JSON(w, 200, rv)
}

var decisionRoute = map[string]string{"approve": events.RoutePass, "reject": events.RouteReject, "request_reupload": events.RouteReupload}

func (s *server) decide(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Decision string `json:"decision"`
		Reviewer string `json:"reviewer"`
		Note     string `json:"note"`
	}
	if !svc.DecodeJSON(w, r, &req) {
		return
	}
	route, ok := decisionRoute[req.Decision]
	if !ok || req.Reviewer == "" || len(req.Reviewer) > 64 || len(req.Note) > 1000 {
		fail(w, 400, "INVALID_REQUEST", "decision must be approve, reject or request_reupload; reviewer is required")
		return
	}
	id := r.PathValue("id")
	rv, err := s.st.review(r.Context(), id)
	if err != nil {
		s.internal(w, "review load failed", err)
		return
	}
	if rv == nil {
		fail(w, 404, "NOT_FOUND", "no such review")
		return
	}
	done, err := s.st.decide(r.Context(), id, req.Decision, req.Reviewer, req.Note, route, time.Now().UTC())
	if err != nil {
		s.internal(w, "decision failed", err)
		return
	}
	if !done {
		fail(w, 409, "REVIEW_CLOSED", "this review was already decided")
		return
	}
	corr := ""
	if a, err := s.st.application(r.Context(), rv.LeadID); err == nil && a != nil {
		corr = a.CorrelationID
	}
	if route != events.RouteReupload {
		if err := s.st.purgeDocuments(r.Context(), rv.LeadID); err != nil {
			s.a.Log.Error("purge after decision failed", "leadId", rv.LeadID, "err", err)
		}
	}
	s.a.Log.Info("review decided", "reviewId", id, "leadId", rv.LeadID, "decision", req.Decision)
	if route == events.RoutePass {
		s.publish(r.Context(), events.TopicKycPassed, rv.LeadID, corr, map[string]any{"leadId": rv.LeadID, "decidedBy": "reviewer", "reviewId": id})
	} else {
		s.publish(r.Context(), events.TopicKycFailed, rv.LeadID, corr, map[string]any{"leadId": rv.LeadID, "route": route, "codes": rv.Codes, "decidedBy": "reviewer", "reviewId": id})
	}
	rv, _ = s.st.review(r.Context(), id)
	svc.JSON(w, 200, rv)
}

// retention deletes document images older than the window, hourly.
func (s *server) retention(ctx context.Context, window time.Duration) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		n, err := s.st.purgeExpired(ctx, time.Now().Add(-window))
		if err != nil {
			s.a.Log.Error("retention purge failed", "err", err)
		} else if n > 0 {
			s.a.Log.Info("retention purge", "deleted", n, "windowDays", int(window.Hours()/24))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
