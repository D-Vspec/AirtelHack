// Command scenarios replays the user stories in docs/scenarios.md against the live stack (through
// the gateway) and prints PASS / FAIL / SKIP per scenario. Deterministic scenarios (rules, quote,
// KYC, orders, scoring) always run; advisor scenarios need an LLM key and are SKIPPED, not
// failed, when the advisor answers 503 LLM_NOT_CONFIGURED. Stories we did not build are listed
// as NOT BUILT so the report is honest.
//
//	make scenarios            (API_URL, ADMIN_TOKEN override the defaults)
//	go run ./cmd/scenarios -only K,F
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var (
	api     = env("API_URL", "http://localhost:8080/api")
	admin   = env("ADMIN_TOKEN", "demo-admin-token")
	run     = fmt.Sprintf("%d", time.Now().Unix()%1000000)
	errSkip = errors.New("skip")
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// ---- tiny HTTP client --------------------------------------------------------------------------

type resp struct {
	status int
	body   map[string]any
	raw    string
}

func call(method, path string, body any) (resp, error) {
	var b []byte
	switch v := body.(type) {
	case nil:
	case []byte:
		b = v
	default:
		b, _ = json.Marshal(v)
	}
	for attempt := 0; ; attempt++ {
		req, _ := http.NewRequest(method, api+path, bytes.NewReader(b))
		if _, raw := body.([]byte); raw {
			req.Header.Set("Content-Type", "image/png")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("X-Admin-Token", admin)
		req.Header.Set("X-Correlation-Id", "scenarios-"+run)
		hr, err := (&http.Client{Timeout: 150 * time.Second}).Do(req)
		if err != nil {
			return resp{}, err
		}
		raw, _ := io.ReadAll(hr.Body)
		hr.Body.Close()
		if hr.StatusCode == http.StatusTooManyRequests && attempt < 8 { // the gateway's rate limit: wait as told
			wait, _ := strconv.Atoi(hr.Header.Get("Retry-After"))
			time.Sleep(time.Duration(max(wait, 1)) * time.Second)
			continue
		}
		r := resp{status: hr.StatusCode, raw: string(raw)}
		_ = json.Unmarshal(raw, &r.body)
		return r, nil
	}
}

func must(method, path string, body any, want int) (map[string]any, error) {
	r, err := call(method, path, body)
	if err != nil {
		return nil, err
	}
	if r.status != want {
		return nil, fmt.Errorf("%s %s: %d (want %d): %.200s", method, path, r.status, want, r.raw)
	}
	return r.body, nil
}

func eventually(what string, timeout time.Duration, f func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := f()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", what)
		}
		time.Sleep(time.Second)
	}
}

func list(m map[string]any, key string) []map[string]any {
	var out []map[string]any
	for _, x := range m[key].([]any) {
		out = append(out, x.(map[string]any))
	}
	return out
}

func ids(items []map[string]any, key string) []string {
	var out []string
	for _, it := range items {
		out = append(out, fmt.Sprint(it[key]))
	}
	return out
}

// ---- fixtures ----------------------------------------------------------------------------------

func profile(kv ...any) map[string]any {
	p := map[string]any{"cityTier": 1, "employeeCount": 6, "numSites": 1, "hasGstin": true, "segment": "Professional offices"}
	for i := 0; i+1 < len(kv); i += 2 {
		p[kv[i].(string)] = kv[i+1]
	}
	return p
}

func eligibility(p map[string]any) (eligible []string, excluded map[string][]string, alts map[string][]string, err error) {
	m, err := must("POST", "/rules/eligibility", map[string]any{"profile": p}, 200)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, e := range list(m, "eligible") {
		eligible = append(eligible, e["id"].(string))
	}
	excluded, alts = map[string][]string{}, map[string][]string{}
	for _, x := range list(m, "excluded") {
		id := x["productId"].(string)
		for _, r := range x["reasons"].([]any) {
			excluded[id] = append(excluded[id], r.(map[string]any)["code"].(string))
		}
		if a, ok := x["alternatives"].([]any); ok {
			for _, v := range a {
				alts[id] = append(alts[id], v.(string))
			}
		}
	}
	return eligible, excluded, alts, nil
}

func quote(lead string, p map[string]any, items []map[string]any, months int) (map[string]any, error) {
	return must("POST", "/quote/quotes", map[string]any{"leadId": lead, "profile": p, "items": items, "contractMonths": months}, 201)
}

func line(q map[string]any, id string) map[string]any {
	for _, l := range list(q, "lines") {
		if l["productId"] == id {
			return l
		}
	}
	return nil
}

func lead(tag string) string { return fmt.Sprintf("sc-%s-%s", tag, run) }

func kycSpecimen(leadID, applicant string) (map[string]any, error) {
	if _, err := must("POST", "/kyc/consents", map[string]any{"leadId": leadID, "purposes": []string{"kyc_verification"}, "version": "scenarios"}, 201); err != nil {
		return nil, err
	}
	if _, err := must("POST", "/kyc/applications/"+leadID+"/specimens/"+applicant, nil, 200); err != nil {
		return nil, err
	}
	return must("POST", "/kyc/applications/"+leadID+"/check", nil, 200)
}

func codes(app map[string]any) []string {
	var out []string
	for _, f := range list(app, "findings") {
		if c := f["code"].(string); !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}

func specimen(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(env("DATA_DIR", "../data"), "kyc_documents", name))
}

func orderFor(leadID string) (map[string]any, error) {
	m, err := must("GET", "/tracker/orders?leadId="+leadID, nil, 200)
	if err != nil {
		return nil, err
	}
	if os := list(m, "orders"); len(os) > 0 {
		return os[0], nil
	}
	return nil, nil
}

func waitStatus(leadID string, timeout time.Duration, statuses ...string) (map[string]any, error) {
	var o map[string]any
	err := eventually("order "+strings.Join(statuses, "|"), timeout, func() (bool, error) {
		var err error
		o, err = orderFor(leadID)
		return o != nil && slices.Contains(statuses, o["status"].(string)), err
	})
	return o, err
}

// ---- advisor helpers (LLM) --------------------------------------------------------------------

type chat struct {
	session, lead string
	last          map[string]any
}

func (c *chat) say(msg string) error {
	body := map[string]any{"message": msg, "leadId": c.lead}
	if c.session != "" {
		body["sessionId"] = c.session
	}
	r, err := call("POST", "/advisor/chat", body)
	if err != nil {
		return err
	}
	if r.status == 503 {
		return errSkip // no LLM key configured
	}
	if r.status != 200 {
		return fmt.Errorf("chat: %d %.200s", r.status, r.raw)
	}
	c.session, c.last = r.body["sessionId"].(string), r.body
	return nil
}

// sayUntilQuote sends the messages in order and stops once a quote exists.
func (c *chat) sayUntilQuote(msgs ...string) error {
	for _, m := range msgs {
		if err := c.say(m); err != nil {
			return err
		}
		if c.last["quote"] != nil {
			return nil
		}
	}
	return fmt.Errorf("no quote after %d turns; last reply: %.160s", len(msgs), c.last["reply"])
}

func (c *chat) quote() map[string]any {
	q, _ := c.last["quote"].(map[string]any)
	return q
}

var devanagari, tamil = regexp.MustCompile(`\p{Devanagari}`), regexp.MustCompile(`\p{Tamil}`)

func latinShare(s string) float64 {
	var letters, latin int
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if r < 0x250 {
				latin++
			}
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(latin) / float64(letters)
}

// ---- scenarios ---------------------------------------------------------------------------------

type scenario struct {
	id, story, title string
	run              func() error
}

var scenarios = []scenario{
	// Advisor and eligibility: deterministic rules and quote services
	{"R1", "#1", "No GSTIN: GSTIN-only products excluded with a reason, GSTIN-free connectivity and POS stay", func() error {
		el, ex, _, err := eligibility(profile("segment", "Micro retail & services", "hasGstin", false, "employeeCount", 2, "cityTier", 2))
		if err != nil {
			return err
		}
		gst := 0
		for id, cs := range ex {
			if slices.Contains(cs, "GSTIN_REQUIRED") {
				gst++
				if slices.Contains(el, id) {
					return fmt.Errorf("%s both eligible and excluded", id)
				}
			}
		}
		if gst == 0 || !slices.Contains(el, "P19") || !slices.ContainsFunc(el, func(s string) bool { return s == "P01" || s == "P04" }) {
			return fmt.Errorf("gstin-excluded %d, eligible %v", gst, el)
		}
		return nil
	}},
	{"R2", "#2", "Clinic, Tier 1, 6 staff: bundle has fibre, email x6, backup, endpoint security, optional failover", func() error {
		m, err := must("POST", "/rules/bundles", map[string]any{"profile": profile()}, 200)
		if err != nil {
			return err
		}
		b := list(m, "bundles")[0]
		items := list(b, "items")
		have := map[string]map[string]any{}
		for _, it := range items {
			have[it["productId"].(string)] = it
		}
		if have["P16"] == nil || have["P16"]["quantity"].(float64) != 6 || have["P17"] == nil || have["P20"] == nil || have["P02"] == nil {
			return fmt.Errorf("bundle items %v", ids(items, "productId"))
		}
		if it := have["P09"]; it == nil || it["optional"] != true {
			return fmt.Errorf("failover should be optional: %v", it)
		}
		return nil
	}},
	{"R3", "#3", "CA firm with 8 staff asks for a managed firewall: excluded on minimum employees, endpoint security offered", func() error {
		el, ex, _, err := eligibility(profile("employeeCount", 8))
		if err != nil {
			return err
		}
		if !slices.Contains(ex["P21"], "BELOW_MIN_EMPLOYEES") || !slices.Contains(el, "P20") {
			return fmt.Errorf("P21 reasons %v, endpoint eligible %v", ex["P21"], slices.Contains(el, "P20"))
		}
		_, err = quote(lead("r3"), profile("employeeCount", 8), []map[string]any{{"productId": "P21"}}, 12)
		if err == nil || !strings.Contains(err.Error(), "NOT_ELIGIBLE") {
			return fmt.Errorf("quoting the firewall must be refused: %v", err)
		}
		return nil
	}},
	{"R4", "#4", "Tier 2 owner asks for a Tier 1 product (Fibre 1G): excluded, closest tier offered", func() error {
		_, ex, alts, err := eligibility(profile("cityTier", 2))
		if err != nil {
			return err
		}
		if !slices.Contains(ex["P03"], "CITY_TIER_UNAVAILABLE") || !slices.Contains(alts["P03"], "P02") {
			return fmt.Errorf("P03 reasons %v alternatives %v", ex["P03"], alts["P03"])
		}
		return nil
	}},
	{"R5", "#5", "Restaurant chain, 5 sites: per-site connectivity and failover, one bill", func() error {
		p := profile("segment", "Small multi-site businesses", "numSites", 5, "employeeCount", 40)
		q, err := quote(lead("r5"), p, []map[string]any{{"productId": "P02"}, {"productId": "P09"}, {"productId": "P23", "quantity": 10}}, 12)
		if err != nil {
			return err
		}
		if l := line(q, "P02"); l == nil || l["quantity"].(float64) != 5 {
			return fmt.Errorf("fibre per site: %v", l)
		}
		var sum float64
		for _, l := range list(q, "lines") {
			sum += l["monthlyInr"].(float64)
		}
		if sum != q["monthlyTotalInr"].(float64) {
			return fmt.Errorf("one bill: lines %v != total %v", sum, q["monthlyTotalInr"])
		}
		return nil
	}},
	{"R6", "#6", "Manufacturer, 40 staff: leased line quoted with an honest 21-30 day install", func() error {
		q, err := quote(lead("r6"), profile("segment", "Small manufacturers", "employeeCount", 40), []map[string]any{{"productId": "P05"}, {"productId": "P24", "quantity": 8}}, 12)
		if err != nil {
			return err
		}
		if d := q["expectedInstallDays"].(float64); d < 21 || d > 30 {
			return fmt.Errorf("install days %v", d)
		}
		return nil
	}},
	{"R7", "#7", "CCTV without internet: connectivity added automatically with an explanation", func() error {
		q, err := quote(lead("r7"), profile(), []map[string]any{{"productId": "P23", "quantity": 4}}, 12)
		if err != nil {
			return err
		}
		for _, l := range list(q, "lines") {
			if l["productId"] != "P23" && l["addedFor"] != nil && l["addedFor"] != "" {
				return nil
			}
		}
		return fmt.Errorf("no dependency line: %v", ids(list(q, "lines"), "productId"))
	}},
	{"R8", "#8", "Only a mobile plan: quick path, 1-month contract, no forced bundle", func() error {
		q, err := quote(lead("r8"), profile("segment", "Micro retail & services", "employeeCount", 2), []map[string]any{{"productId": "P07", "quantity": 2}}, 1)
		if err != nil {
			return err
		}
		ls := list(q, "lines")
		if len(ls) != 1 || ls[0]["contractMonths"].(float64) != 1 {
			return fmt.Errorf("lines %v", ls)
		}
		return nil
	}},
	{"Q1", "#2", "Pricing is deterministic and accepting is idempotent (one quote.accepted)", func() error {
		items := []map[string]any{{"productId": "P02"}, {"productId": "P16", "quantity": 6}}
		a, err := quote(lead("q1a"), profile(), items, 12)
		if err != nil {
			return err
		}
		b, err := quote(lead("q1b"), profile(), items, 12)
		if err != nil {
			return err
		}
		if a["monthlyTotalInr"] != b["monthlyTotalInr"] || a["totalContractValueInr"] != b["totalContractValueInr"] {
			return fmt.Errorf("same request, different price: %v vs %v", a["monthlyTotalInr"], b["monthlyTotalInr"])
		}
		for i := 0; i < 2; i++ {
			if _, err := must("POST", "/quote/quotes/"+a["id"].(string)+"/accept", nil, 200); err != nil {
				return err
			}
		}
		return nil
	}},
	{"Q2", "#7", "Removing the connectivity a CCTV line depends on is refused with an explanation", func() error {
		q, err := quote(lead("q2"), profile(), []map[string]any{{"productId": "P02"}, {"productId": "P23", "quantity": 4}}, 12)
		if err != nil {
			return err
		}
		r, err := call("PATCH", "/quote/quotes/"+q["id"].(string), map[string]any{"toggles": []map[string]any{{"productId": "P02", "enabled": false}}})
		if err != nil {
			return err
		}
		if r.status != 409 || r.body["code"] != "DEPENDENCY_REQUIRED" {
			return fmt.Errorf("got %d %.150s", r.status, r.raw)
		}
		return nil
	}},

	// KYC
	{"K1", "privacy", "No consent, no upload: 403 CONSENT_REQUIRED", func() error {
		r, err := call("POST", "/kyc/applications/"+lead("k1")+"/specimens/K03", nil)
		if err != nil {
			return err
		}
		if r.status != 403 || r.body["code"] != "CONSENT_REQUIRED" {
			return fmt.Errorf("got %d %.120s", r.status, r.raw)
		}
		return nil
	}},
	{"K2", "#11", "GSTIN Cancelled (K01): blocked with a clear message", func() error {
		a, err := kycSpecimen(lead("k2"), "K01")
		if err != nil {
			return err
		}
		if a["status"] != "reject" || !slices.Contains(codes(a), "GST_STATUS_CANCELLED") {
			return fmt.Errorf("%v %v", a["status"], codes(a))
		}
		return nil
	}},
	{"K3", "#12", "Name differs on the certificate (K07): manual review, not silent rejection", func() error {
		l := lead("k3")
		a, err := kycSpecimen(l, "K07")
		if err != nil {
			return err
		}
		if a["status"] != "manual_review" || !slices.Contains(codes(a), "NAME_MISMATCH") {
			return fmt.Errorf("%v %v", a["status"], codes(a))
		}
		m, err := must("GET", "/kyc/reviews?status=open", nil, 200)
		if err != nil {
			return err
		}
		if !slices.Contains(ids(list(m, "reviews"), "leadId"), l) {
			return fmt.Errorf("no review queued")
		}
		return nil
	}},
	{"K4", "#13", "Address proof older than 90 days (K11): asked for a fresh one", func() error {
		a, err := kycSpecimen(lead("k4"), "K11")
		if err != nil {
			return err
		}
		if a["status"] != "reupload" || !slices.Equal(codes(a), []string{"DOC_STALE"}) {
			return fmt.Errorf("%v %v", a["status"], codes(a))
		}
		return nil
	}},
	{"K5", "#17", "Blurred upload (K04): instant re-upload request; skewed page (K02) corrected and passes", func() error {
		l := lead("k5")
		if _, err := must("POST", "/kyc/consents", map[string]any{"leadId": l, "purposes": []string{"kyc_verification"}, "version": "scenarios"}, 201); err != nil {
			return err
		}
		img, err := specimen("K04_gst_certificate.png")
		if err != nil {
			return err
		}
		up, err := must("PUT", "/kyc/applications/"+l+"/documents/gst_certificate", img, 200)
		if err != nil {
			return err
		}
		if up["blurry"] != true {
			return fmt.Errorf("blurry not flagged: %v", up)
		}
		a, err := kycSpecimen(lead("k5b"), "K02")
		if err != nil {
			return err
		}
		if a["status"] != "pass" || !slices.Equal(codes(a), []string{"IMAGE_SKEWED"}) {
			return fmt.Errorf("K02: %v %v", a["status"], codes(a))
		}
		return nil
	}},
	{"K6", "#18", "Mistyped GSTIN (K06): checksum fails, closest registry entry suggested, routed to review", func() error {
		a, err := kycSpecimen(lead("k6"), "K06")
		if err != nil {
			return err
		}
		if a["status"] != "manual_review" || !slices.Equal(codes(a), []string{"GSTIN_CHECKSUM_INVALID"}) {
			return fmt.Errorf("%v %v", a["status"], codes(a))
		}
		f := list(a, "findings")[0]
		if p, _ := f["params"].(map[string]any); p == nil || p["closest"] == nil {
			return fmt.Errorf("no closest match suggested: %v", f)
		}
		return nil
	}},
	{"K7", "#15", "Owner abandons halfway: resume link restores the session with nothing re-entered", func() error {
		l := lead("k7")
		if _, err := must("POST", "/kyc/consents", map[string]any{"leadId": l, "purposes": []string{"kyc_verification"}, "version": "scenarios"}, 201); err != nil {
			return err
		}
		img, err := specimen("K03_gst_certificate.png")
		if err != nil {
			return err
		}
		if _, err := must("PUT", "/kyc/applications/"+l+"/documents/gst_certificate", img, 200); err != nil {
			return err
		}
		link, err := must("POST", "/kyc/applications/"+l+"/resume-link", nil, 201)
		if err != nil {
			return err
		}
		st, err := must("GET", "/kyc/resume/"+link["token"].(string), nil, 200)
		if err != nil {
			return err
		}
		if st["leadId"] != l || len(st["documents"].([]any)) != 1 || len(st["missingDocuments"].([]any)) != 3 {
			return fmt.Errorf("resume state %v", st)
		}
		return eventually("resume message", 20*time.Second, func() (bool, error) {
			m, err := must("GET", "/notify/messages?leadId="+l, nil, 200)
			return err == nil && slices.Contains(ids(list(m, "messages"), "templateKey"), "kyc.resume"), err
		})
	}},
	{"K8", "#16", "Owner withdraws consent: documents and results purged", func() error {
		l := lead("k8")
		if _, err := must("POST", "/kyc/consents", map[string]any{"leadId": l, "purposes": []string{"kyc_verification"}, "version": "scenarios"}, 201); err != nil {
			return err
		}
		if _, err := must("POST", "/kyc/applications/"+l+"/specimens/K07", nil, 200); err != nil {
			return err
		}
		w, err := must("POST", "/kyc/consents/"+l+"/withdraw", nil, 200)
		if err != nil {
			return err
		}
		if w["purgedDocuments"].(float64) != 4 {
			return fmt.Errorf("purged %v", w["purgedDocuments"])
		}
		r, err := call("GET", "/kyc/applications/"+l, nil)
		if err != nil || r.status != 404 {
			return fmt.Errorf("application still there: %d %v", r.status, err)
		}
		r, err = call("POST", "/kyc/applications/"+l+"/check", nil)
		if err != nil || r.status != 403 {
			return fmt.Errorf("check after withdrawal: %d %v", r.status, err)
		}
		return nil
	}},
	{"K9", "#11", "GSTIN prefill: registry data returned (O/0 OCR confusion corrected)", func() error {
		m, err := must("GET", "/kyc/prefill/27XUIXGO767D1ZP", nil, 200)
		if err != nil {
			return err
		}
		if m["matchType"] != "corrected" || m["tradeName"] != "Ganesh Woodcraft" || m["status"] != "Active" {
			return fmt.Errorf("%v", m)
		}
		return nil
	}},
	{"K10", "#11", "Clean applicant (K03) passes and images are deleted after the decision", func() error {
		a, err := kycSpecimen(lead("k10"), "K03")
		if err != nil {
			return err
		}
		if a["status"] != "pass" || len(codes(a)) != 0 || len(a["documents"].([]any)) != 0 {
			return fmt.Errorf("%v codes %v documents %v", a["status"], codes(a), a["documents"])
		}
		return nil
	}},

	// Fulfilment
	{"F1", "#2", "Quote accepted + KYC passed: order runs to activation with the expected date from the slowest product", func() error {
		l := lead("f1")
		q, err := quote(l, profile(), []map[string]any{{"productId": "P02"}, {"productId": "P16", "quantity": 6}}, 12)
		if err != nil {
			return err
		}
		if _, err := must("POST", "/quote/quotes/"+q["id"].(string)+"/accept", nil, 200); err != nil {
			return err
		}
		if _, err := kycSpecimen(l, "K03"); err != nil {
			return err
		}
		o, err := waitStatus(l, 150*time.Second, "activated")
		if err != nil {
			return err
		}
		if o["expectedInstallDays"].(float64) != q["expectedInstallDays"].(float64) || o["expectedActivationDate"] == "" {
			return fmt.Errorf("install days %v vs %v", o["expectedInstallDays"], q["expectedInstallDays"])
		}
		return nil
	}},
	{"F2", "#19 #20", "Fibre not feasible: delayed, owner told proactively, 5G offered and accepted, activation", func() error {
		l := lead("nofibre")
		q, err := quote(l, profile(), []map[string]any{{"productId": "P02"}, {"productId": "P16", "quantity": 5}}, 12)
		if err != nil {
			return err
		}
		if _, err := must("POST", "/quote/quotes/"+q["id"].(string)+"/accept", nil, 200); err != nil {
			return err
		}
		if _, err := kycSpecimen(l, "K09"); err != nil {
			return err
		}
		o, err := waitStatus(l, 120*time.Second, "delayed")
		if err != nil {
			return err
		}
		if o["fallbackOffered"] != true || o["statusReason"] != "fibre_not_feasible" {
			return fmt.Errorf("%v %v", o["fallbackOffered"], o["statusReason"])
		}
		if err := eventually("delay + offer messages", 20*time.Second, func() (bool, error) {
			m, err := must("GET", "/notify/messages?leadId="+l, nil, 200)
			keys := ids(list(m, "messages"), "templateKey")
			return err == nil && slices.Contains(keys, "order.status.delayed") && slices.Contains(keys, "order.fallback_offer"), err
		}); err != nil {
			return err
		}
		if _, err := must("POST", "/tracker/orders/"+o["id"].(string)+"/fallback/accept", nil, 200); err != nil {
			return err
		}
		o, err = waitStatus(l, 120*time.Second, "activated")
		if err != nil {
			return err
		}
		if !strings.Contains(fmt.Sprint(o["productIds"]), "P04") {
			return fmt.Errorf("products %v", o["productIds"])
		}
		return nil
	}},
	{"F3", "ops", "Illegal order moves are refused (state machine)", func() error {
		l := lead("f3")
		q, err := quote(l, profile(), []map[string]any{{"productId": "P02"}}, 12)
		if err != nil {
			return err
		}
		if _, err := must("POST", "/quote/quotes/"+q["id"].(string)+"/accept", nil, 200); err != nil {
			return err
		}
		o, err := waitStatus(l, 60*time.Second, "kyc_pending")
		if err != nil {
			return err
		}
		r, err := call("POST", "/tracker/orders/"+o["id"].(string)+"/transitions", map[string]any{"to": "activated"})
		if err != nil || r.status != 409 {
			return fmt.Errorf("kyc_pending -> activated: %d %v", r.status, err)
		}
		return nil
	}},
	{"F4", "#23", "Morning list: leads ranked by propensity with reasons", func() error {
		m, err := must("GET", "/scoring/leads?source=case&limit=20", nil, 200)
		if err != nil {
			return err
		}
		ls := list(m, "leads")
		for i := 1; i < len(ls); i++ {
			if ls[i]["probability"].(float64) > ls[i-1]["probability"].(float64) {
				return fmt.Errorf("not ranked at %d", i)
			}
		}
		if len(ls) != 20 || len(ls[0]["reasons"].([]any)) == 0 {
			return fmt.Errorf("%d leads", len(ls))
		}
		return nil
	}},
	{"F5", "ops", "Back office needs the admin token; owner routes do not", func() error {
		req, _ := http.NewRequest("GET", api+"/kyc/reviews", nil)
		hr, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		hr.Body.Close()
		if hr.StatusCode != 401 {
			return fmt.Errorf("reviews without token: %d", hr.StatusCode)
		}
		hr, err = http.Get(api + "/tracker/orders?leadId=" + lead("nobody"))
		if err != nil {
			return err
		}
		hr.Body.Close()
		if hr.StatusCode != 200 {
			return fmt.Errorf("owner order lookup: %d", hr.StatusCode)
		}
		return nil
	}},

	// Advisor (live LLM)
	{"A1", "#2", "Advisor, Tamil clinic: recommends within 6 turns, in Tamil script, price equals the quote service", func() error {
		c := &chat{lead: lead("a1")}
		if err := c.sayUntilQuote("வணக்கம், நான் சென்னையில் ஒரு பல் மருத்துவமனை நடத்துகிறேன். 6 பேர் வேலை செய்கிறார்கள், ஒரே கிளை, GSTIN உள்ளது.",
			"ஒரே கிளை, 6 பேர், GSTIN உள்ளது", "சரி", "ஆம்", "சரி", "சரி"); err != nil {
			return err
		}
		q := c.quote()
		srv, err := must("GET", "/quote/quotes/"+q["id"].(string), nil, 200)
		if err != nil {
			return err
		}
		reply := c.last["reply"].(string)
		if srv["monthlyTotalInr"] != q["monthlyTotalInr"] || !tamil.MatchString(reply) || c.last["turn"].(float64) > 6 {
			return fmt.Errorf("price %v/%v tamil %v turn %v", srv["monthlyTotalInr"], q["monthlyTotalInr"], tamil.MatchString(reply), c.last["turn"])
		}
		return nil
	}},
	{"A2", "#1", "Advisor, Hindi kirana without GSTIN: no GSTIN-only product quoted, reply in Devanagari", func() error {
		c := &chat{lead: lead("a2")}
		if err := c.sayUntilQuote("नमस्ते, मेरी लखनऊ में किराना दुकान है, 2 लोग काम करते हैं, एक ही दुकान है, GSTIN नहीं है। UPI बिलिंग और इंटरनेट चाहिए।",
			"GSTIN नहीं है, एक दुकान, 2 लोग", "ठीक है", "हाँ", "ठीक है", "ठीक है"); err != nil {
			return err
		}
		_, ex, _, err := eligibility(map[string]any{"segment": "Micro retail & services", "hasGstin": false, "cityTier": 1, "employeeCount": 2, "numSites": 1})
		if err != nil {
			return err
		}
		for _, l := range list(c.quote(), "lines") {
			if slices.Contains(ex[l["productId"].(string)], "GSTIN_REQUIRED") {
				return fmt.Errorf("quoted %v, which needs a GSTIN", l["productId"])
			}
		}
		if !devanagari.MatchString(c.last["reply"].(string)) {
			return fmt.Errorf("reply not in Devanagari: %.120s", c.last["reply"])
		}
		return nil
	}},
	{"A3", "#3", "Advisor, CA firm asks for a firewall: firewall not quoted", func() error {
		c := &chat{lead: lead("a3")}
		if err := c.sayUntilQuote("Hi, we are a CA firm in Pune with 8 employees, one office, we have GSTIN. We want a managed firewall and fast internet.",
			"8 employees, one office, GSTIN yes", "ok", "yes", "ok", "ok"); err != nil {
			return err
		}
		if line(c.quote(), "P21") != nil {
			return fmt.Errorf("firewall was quoted")
		}
		return nil
	}},
	{"A4", "#8", "Advisor, only a mobile plan: quick path with a 1-month mobile-only quote", func() error {
		c := &chat{lead: lead("a4")}
		if err := c.sayUntilQuote("I just need 2 mobile connections for my shop in Indore, nothing else. We have GSTIN.", "only mobile, 2 lines", "yes"); err != nil {
			return err
		}
		ls := list(c.quote(), "lines")
		if len(ls) != 1 || ls[0]["productId"] != "P07" || ls[0]["contractMonths"].(float64) != 1 {
			return fmt.Errorf("lines %v", ids(ls, "productId"))
		}
		return nil
	}},
	{"A5", "#9", "Advisor, 'too expensive': cheaper option with a lower total", func() error {
		c := &chat{lead: lead("a5")}
		if err := c.sayUntilQuote("We are a dental clinic in Pune, 6 staff, one branch, GSTIN yes.", "one branch, 6 staff, GSTIN", "ok"); err != nil {
			return err
		}
		before := c.quote()["totalContractValueInr"].(float64)
		if err := c.say("This is too expensive for us."); err != nil {
			return err
		}
		q := c.quote()
		if q == nil || q["totalContractValueInr"].(float64) >= before {
			return fmt.Errorf("total %v -> %v", before, q["totalContractValueInr"])
		}
		return nil
	}},
	{"A6", "#10", "Advisor, Tanglish: answers in romanised Tamil, not Tamil script or plain English", func() error {
		c := &chat{lead: lead("a6")}
		if err := c.say("Vanakkam, naan Coimbatore-la oru chinna bakery vechirukken, 3 per velai seiraanga, GSTIN irukku. Internet venum."); err != nil {
			return err
		}
		reply := c.last["reply"].(string)
		if c.last["language"] != "Tamil" || c.last["romanised"] != true || tamil.MatchString(reply) || latinShare(reply) < 0.9 {
			return fmt.Errorf("language %v romanised %v: %.120s", c.last["language"], c.last["romanised"], reply)
		}
		return nil
	}},
	{"A7", "guard", "Advisor, out-of-scope question: no products or prices invented", func() error {
		c := &chat{lead: lead("a7")}
		if err := c.say("Who will win the cricket match tomorrow?"); err != nil {
			return err
		}
		if c.last["quote"] != nil || strings.Contains(c.last["reply"].(string), "₹") {
			return fmt.Errorf("reply: %.150s", c.last["reply"])
		}
		return nil
	}},
	{"A8", "#21", "Advisor cannot close a custom request: partner task with the conversation summary", func() error {
		c := &chat{lead: lead("a8")}
		if err := c.say("I run a printing press in Surat with 25 staff, GSTIN yes. Can you set up a private 5G network for our factory floor?"); err != nil {
			return err
		}
		if _, err := must("POST", "/advisor/handoff", map[string]any{"sessionId": c.session, "reason": "custom_request"}, 200); err != nil {
			return err
		}
		return eventually("partner task", 30*time.Second, func() (bool, error) {
			m, err := must("GET", "/scoring/tasks?status=open", nil, 200)
			if err != nil {
				return false, err
			}
			for _, t := range list(m, "tasks") {
				if t["leadId"] == c.lead {
					return strings.Contains(t["summary"].(string), "Surat") && t["probability"] != nil, nil
				}
			}
			return false, nil
		})
	}},
}

var notBuilt = []struct{ story, title, why string }{
	{"#14", "Signatory not available: e-sign link to the signatory", "no e-sign/DigiLocker integration (organisers to confirm a mock is allowed)"},
	{"#22", "Partner onboards a customer on a tablet, partner credited", "the web journey works on a tablet, but there is no partner login or attribution"},
	{"#24", "Locked in with another provider: contract end date and reminder", "not captured by the advisor; no reminder scheduler"},
	{"#25", "Wrong-number leads: OTP at entry", "OTP sign-in exists for returning owners (demo code), but not at first entry; no SMS gateway"},
}

func main() {
	only := flag.String("only", "", "comma-separated scenario ID prefixes, e.g. K,F2")
	flag.Parse()
	var pass, fail, skip int
	start := time.Now()
	fmt.Printf("Scenarios against %s (run %s)\n\n", api, run)
	for _, s := range scenarios {
		if *only != "" && !slices.ContainsFunc(strings.Split(*only, ","), func(p string) bool { return strings.HasPrefix(s.id, p) }) {
			continue
		}
		t0 := time.Now()
		err := s.run()
		status := "PASS"
		switch {
		case errors.Is(err, errSkip):
			status, skip = "SKIP", skip+1
		case err != nil:
			status, fail = "FAIL", fail+1
		default:
			pass++
		}
		fmt.Printf("%-4s %-4s %-8s %-104s %5.1fs\n", status, s.id, s.story, s.title, time.Since(t0).Seconds())
		if status == "FAIL" {
			fmt.Printf("           -> %v\n", err)
		}
		if status == "SKIP" {
			fmt.Printf("           -> advisor has no LLM key (LLM_NOT_CONFIGURED)\n")
		}
	}
	if *only == "" {
		fmt.Println()
		for _, n := range notBuilt {
			fmt.Printf("NOT BUILT %-4s %s: %s\n", n.story, n.title, n.why)
		}
	}
	fmt.Printf("\n%d passed, %d failed, %d skipped in %.0fs\n", pass, fail, skip, time.Since(start).Seconds())
	if fail > 0 {
		os.Exit(1)
	}
}
