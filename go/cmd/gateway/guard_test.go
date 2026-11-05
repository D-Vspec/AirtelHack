package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminOnlyRoutes(t *testing.T) {
	admin := []struct{ m, u string }{
		{"GET", "/api/kyc/reviews?status=open"}, {"POST", "/api/kyc/reviews/abc/decision"}, {"GET", "/api/scoring/leads"},
		{"POST", "/api/scoring/tasks/x/claim"}, {"POST", "/api/tracker/orders/o1/transitions"}, {"GET", "/api/tracker/orders"},
		{"GET", "/api/notify/messages"}, {"GET", "/api/kyc/metrics"}, {"GET", "/api/advisor/sessions/abc"},
	}
	owner := []struct{ m, u string }{
		{"GET", "/api/tracker/orders?leadId=web-1"}, {"GET", "/api/notify/messages?leadId=web-1"}, {"POST", "/api/tracker/orders/o1/fallback/accept"},
		{"POST", "/api/kyc/applications/web-1/check"}, {"POST", "/api/advisor/chat"}, {"GET", "/api/health"},
	}
	for _, c := range admin {
		if !adminOnly(httptest.NewRequest(c.m, c.u, nil)) {
			t.Errorf("%s %s should be admin-only", c.m, c.u)
		}
	}
	for _, c := range owner {
		if adminOnly(httptest.NewRequest(c.m, c.u, nil)) {
			t.Errorf("%s %s should be open to owners", c.m, c.u)
		}
	}
}

func TestRequireAdmin(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	h := requireAdmin("s3cret", ok)
	for _, c := range []struct {
		token string
		want  int
	}{{"", 401}, {"wrong", 401}, {"s3cret", 204}} {
		r := httptest.NewRequest("GET", "/api/kyc/reviews", nil)
		if c.token != "" {
			r.Header.Set("X-Admin-Token", c.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("token %q: %d, want %d", c.token, w.Code, c.want)
		}
	}
	// no token configured: back office is closed, not open
	w := httptest.NewRecorder()
	requireAdmin("", ok).ServeHTTP(w, httptest.NewRequest("GET", "/api/kyc/reviews", nil))
	if w.Code != 401 {
		t.Errorf("empty ADMIN_TOKEN must not open admin routes: %d", w.Code)
	}
}

func TestRateLimit(t *testing.T) {
	l := newLimiter(5) // burst 10
	allowed := 0
	for i := 0; i < 50; i++ {
		if ok, _ := l.allow("1.2.3.4", "general"); ok {
			allowed++
		}
	}
	if allowed < 10 || allowed > 11 {
		t.Errorf("burst: %d allowed", allowed)
	}
	if ok, _ := l.allow("5.6.7.8", "general"); !ok {
		t.Error("another client has its own bucket")
	}
	if ok, wait := l.allow("1.2.3.4", "general"); ok || wait <= 0 {
		t.Error("exhausted bucket must report a wait")
	}
	if classOf(httptest.NewRequest("POST", "/api/advisor/chat", nil)) != "advisor" ||
		classOf(httptest.NewRequest("PUT", "/api/kyc/applications/x/documents/gst_certificate", nil)) != "upload" {
		t.Error("route classes")
	}
}
