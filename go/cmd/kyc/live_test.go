package main

// Live tests against the running stack (make up seed), through the gateway. Skipped when it is down.
// They prove the container's tesseract gives the same 12-applicant table as the unit test.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sync"
	"testing"
	"time"
)

const base = "http://localhost:8080/api/kyc"

func call(t *testing.T, method, path string, body any, out any) int {
	t.Helper()
	var rd *bytes.Reader
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case []byte:
		rd = bytes.NewReader(b)
	default:
		j, _ := json.Marshal(b)
		rd = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, base+path, rd)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

func live(t *testing.T) {
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(base + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		t.Skip("kyc not reachable through the gateway (make up)")
	}
	resp.Body.Close()
}

var docTypes = []string{"gst_certificate", "udyam_certificate", "address_proof", "signatory_id"}

func upload(t *testing.T, lead, applicant string) {
	t.Helper()
	for _, d := range docTypes {
		b, err := os.ReadFile(fmt.Sprintf("../../../data/kyc_documents/%s_%s.png", applicant, d))
		if err != nil {
			continue
		}
		if code := call(t, "PUT", "/leads/"+lead+"/documents/"+d, b, nil); code != 200 {
			t.Fatalf("upload %s: %d", d, code)
		}
	}
}

func consent(t *testing.T, lead string) {
	t.Helper()
	if code := call(t, "POST", "/consents", map[string]any{"leadId": lead, "purposes": []string{"kyc_verification"}, "version": "v1"}, nil); code != 201 {
		t.Fatalf("consent: %d", code)
	}
}

func TestLiveTwelveApplicants(t *testing.T) {
	live(t)
	var exp struct {
		Applicants map[string]struct {
			FinalRoute string   `json:"finalRoute"`
			Codes      []string `json:"codes"`
		} `json:"applicants"`
	}
	b, _ := os.ReadFile("../../../data/kyc_expected.json")
	if err := json.Unmarshal(b, &exp); err != nil {
		t.Fatal(err)
	}
	run := fmt.Sprint(time.Now().UnixNano())
	var mu sync.Mutex
	for id, want := range exp.Applicants {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			lead := "live-" + id + "-" + run
			consent(t, lead)
			upload(t, lead, id)
			var res checkResult
			if code := call(t, "POST", "/leads/"+lead+"/check", nil, &res); code != 200 {
				t.Fatalf("check: %d", code)
			}
			got := codes(res.Findings)
			slices.Sort(got)
			w := slices.Clone(want.Codes)
			slices.Sort(w)
			mu.Lock()
			defer mu.Unlock()
			if res.Route != want.FinalRoute || !slices.Equal(got, w) {
				t.Errorf("route %s codes %v, want %s %v", res.Route, got, want.FinalRoute, w)
				for _, f := range res.Findings {
					t.Logf("  %s/%s %s: %s", f.Stage, f.Doc, f.Code, f.Detail)
				}
			}
			if (res.Route == "manual_review") != (res.ReviewID != "") {
				t.Errorf("reviewId %q for route %s", res.ReviewID, res.Route)
			}
			for _, f := range res.Fields {
				if f["idNo"] != "" || f["consumerNo"] != "" {
					t.Errorf("identifiers kept after the check: %v", f)
				}
			}
		})
	}
}

func TestLiveConsentReviewPrefill(t *testing.T) {
	live(t)
	lead := fmt.Sprint("live-flow-", time.Now().UnixNano())
	gst, _ := os.ReadFile("../../../data/kyc_documents/K07_gst_certificate.png")

	// No consent: uploads and checks are refused.
	var e map[string]string
	if code := call(t, "PUT", "/leads/"+lead+"/documents/gst_certificate", gst, &e); code != 403 || e["code"] != "CONSENT_REQUIRED" {
		t.Fatalf("upload without consent: %d %v", code, e)
	}
	if code := call(t, "POST", "/consents", map[string]any{"leadId": lead, "purposes": []string{"marketing"}, "version": "v1"}, nil); code != 400 {
		t.Fatalf("unknown purpose accepted: %d", code)
	}

	// K07 goes to manual review; a reviewer approves it.
	consent(t, lead)
	upload(t, lead, "K07")
	var res checkResult
	call(t, "POST", "/leads/"+lead+"/check", nil, &res)
	if res.Route != "manual_review" || res.ReviewID == "" {
		t.Fatalf("K07 route %s review %q", res.Route, res.ReviewID)
	}
	var open []review
	call(t, "GET", "/reviews?status=open", nil, &open)
	if !slices.ContainsFunc(open, func(r review) bool { return r.ID == res.ReviewID }) {
		t.Fatal("review not in the open queue")
	}
	var decided review
	if code := call(t, "POST", "/reviews/"+res.ReviewID+"/decision", map[string]string{"decision": "pass", "reviewer": "ops-1", "note": "trade name is the registered name"}, &decided); code != 200 || decided.Status != "closed" {
		t.Fatalf("decision: %d %+v", code, decided)
	}
	if code := call(t, "POST", "/reviews/"+res.ReviewID+"/decision", map[string]string{"decision": "reject", "reviewer": "ops-2"}, nil); code != 409 {
		t.Fatalf("second decision: %d, want 409", code)
	}

	// Withdrawal purges the documents and the check.
	var w map[string]any
	if code := call(t, "DELETE", "/consents/"+lead, nil, &w); code != 200 || w["purgedDocuments"].(float64) != 4 {
		t.Fatalf("withdraw: %d %v", code, w)
	}
	if code := call(t, "GET", "/leads/"+lead+"/check", nil, nil); code != 404 {
		t.Fatalf("check survived withdrawal: %d", code)
	}
	if code := call(t, "POST", "/leads/"+lead+"/check", nil, nil); code != 403 {
		t.Fatalf("check after withdrawal: %d", code)
	}

	// Prefill from the registry, including an OCR-confused GSTIN.
	var p map[string]any
	call(t, "GET", "/prefill/27XUIXGO767D1ZP", nil, &p)
	if p["matchType"] != "corrected" || p["tradeName"] != "Ganesh Woodcraft" {
		t.Fatalf("prefill: %v", p)
	}
}
