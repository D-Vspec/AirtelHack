//go:build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Failure drills (make drills): stop a dependency, show graceful behaviour, start it, show
// recovery. Results and timings are written up in docs/failure-drills.md.

func metric(t *testing.T, service, name string) string {
	req, _ := http.NewRequest("GET", api+"/"+service+"/metrics", nil)
	req.Header.Set("X-Admin-Token", envOr("ADMIN_TOKEN", "demo-admin-token"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, name+"{") {
			return l[strings.LastIndex(l, " ")+1:]
		}
	}
	return ""
}

func statusOf(method, path string) (int, string) {
	req, _ := http.NewRequest(method, api+path, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", envOr("ADMIN_TOKEN", "demo-admin-token"))
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// Kafka down: owners can still accept quotes and pass KYC (writes + outbox succeed); the outbox
// backlog is visible in /metrics; when Kafka returns the backlog drains and the order appears.
func TestDrillKafkaDown(t *testing.T) {
	waitHealthy(t)
	lead := fmt.Sprintf("drill-kafka-%d", time.Now().UnixNano())
	qid := quote(t, lead)
	compose(t, "stop", "kafka")
	defer compose(t, "start", "kafka")
	t0 := time.Now()
	call(t, "POST", "/quote/quotes/"+qid+"/accept", nil, 200)
	passKYC(t, lead, "K03")
	t.Logf("with Kafka down: quote accepted and KYC passed in %s", time.Since(t0).Round(time.Millisecond))
	eventually(t, "outbox backlog in metrics", 20*time.Second, func() bool {
		q, k := metric(t, "quote", "udyam_outbox_pending"), metric(t, "kyc", "udyam_outbox_pending")
		return q != "" && q != "0" && k != "" && k != "0"
	})
	t.Logf("outbox backlog: quote=%s kyc=%s", metric(t, "quote", "udyam_outbox_pending"), metric(t, "kyc", "udyam_outbox_pending"))
	compose(t, "start", "kafka")
	t1 := time.Now()
	eventually(t, "outbox drained", 150*time.Second, func() bool {
		return metric(t, "quote", "udyam_outbox_pending") == "0" && metric(t, "kyc", "udyam_outbox_pending") == "0"
	})
	eventually(t, "order at kyc_passed", 60*time.Second, func() bool {
		o := orderFor(t, lead)
		return o != nil && o["status"] != "quote_accepted" && o["status"] != "kyc_pending"
	})
	t.Logf("Kafka back: backlog drained and order caught up in %s", time.Since(t1).Round(time.Second))
}

// kyc down: KYC routes answer a clear 503 UPSTREAM_UNAVAILABLE, health shows kyc down, the rest of
// the journey (rules, quotes, orders) keeps working; kyc comes back without manual steps.
func TestDrillKycDown(t *testing.T) {
	waitHealthy(t)
	compose(t, "stop", "kyc")
	defer compose(t, "start", "kyc")
	code, body := statusOf("GET", "/kyc/prefill/27XUIXG0767D1ZP")
	if code != 503 || !strings.Contains(body, "UPSTREAM_UNAVAILABLE") {
		t.Fatalf("kyc down: %d %s", code, body)
	}
	h := call(t, "GET", "/health", nil, 200)
	if h["status"] != "degraded" || h["services"].(map[string]any)["kyc"] != "down" {
		t.Errorf("health: %v", h)
	}
	lead := fmt.Sprintf("drill-kyc-%d", time.Now().UnixNano())
	qid := quote(t, lead) // the rest still works
	call(t, "POST", "/quote/quotes/"+qid+"/accept", nil, 200)
	eventually(t, "order while kyc is down", 30*time.Second, func() bool { return orderFor(t, lead) != nil })
	t.Logf("kyc down: KYC calls get 503 UPSTREAM_UNAVAILABLE; quote and order unaffected")
	compose(t, "start", "kyc")
	t0 := time.Now()
	eventually(t, "kyc back", 60*time.Second, func() bool {
		c, _ := statusOf("GET", "/kyc/prefill/27XUIXG0767D1ZP")
		return c == 200
	})
	passKYC(t, lead, "K03")
	eventually(t, "order moves on", 60*time.Second, func() bool {
		o := orderFor(t, lead)
		return o != nil && o["status"] != "kyc_pending"
	})
	t.Logf("kyc back in %s; the waiting order continued", time.Since(t0).Round(time.Second))
}

// Postgres restart: requests during the outage fail fast with 5xx (no hangs, no crashes), every
// service reconnects on its own (pgx pool), and the event flow resumes.
func TestDrillPostgresRestart(t *testing.T) {
	waitHealthy(t)
	compose(t, "stop", "postgres")
	stopped := true
	defer func() {
		if stopped {
			compose(t, "start", "postgres")
		}
	}()
	// a request that has to query the database (tracker order lookup)
	t0 := time.Now()
	code, body := statusOf("GET", "/tracker/orders?leadId=drill-pg-probe")
	if code < 500 || time.Since(t0) > 15*time.Second {
		t.Errorf("during outage: %d %s after %s", code, body, time.Since(t0))
	}
	t.Logf("Postgres down: DB-backed request failed fast with %d in %s", code, time.Since(t0).Round(time.Millisecond))
	compose(t, "start", "postgres")
	stopped = false
	t1 := time.Now()
	eventually(t, "tracker reads again", 90*time.Second, func() bool {
		c, _ := statusOf("GET", "/tracker/orders?leadId=drill-pg-probe")
		return c == 200
	})
	t.Logf("Postgres back: first successful DB read after %s, no service restarted", time.Since(t1).Round(time.Second))
	lead := fmt.Sprintf("drill-pg-%d", time.Now().UnixNano())
	qid := quote(t, lead)
	call(t, "POST", "/quote/quotes/"+qid+"/accept", nil, 200)
	eventually(t, "order after restart", 60*time.Second, func() bool { return orderFor(t, lead) != nil })
	t.Logf("new quote -> order flowed end to end %s after the restart", time.Since(t1).Round(time.Second))
}
