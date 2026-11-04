//go:build integration

// Package integration drives the running stack (make up seed) through the gateway: lead to quote
// to KYC to activation over real Kafka and Postgres, while services and Kafka are stopped and
// restarted mid-flow. Run: make integration (needs Docker; takes a few minutes).
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var api = envOr("API_URL", "http://localhost:8080/api")

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func call(t *testing.T, method, path string, body any, want int) map[string]any {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, api+path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if want != 0 && resp.StatusCode != want {
		t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, want, raw)
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func compose(t *testing.T, args ...string) {
	t.Helper()
	root, _ := filepath.Abs("../..")
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker compose %v: %v\n%s", args, err, out)
	}
	t.Logf("docker compose %s", strings.Join(args, " "))
}

// eventually polls f until it returns true or the deadline passes.
func eventually(t *testing.T, what string, timeout time.Duration, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

func waitHealthy(t *testing.T) {
	eventually(t, "all services healthy", 90*time.Second, func() bool {
		resp, err := http.Get(api + "/health")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200
	})
}

func orderFor(t *testing.T, lead string) map[string]any {
	out := call(t, "GET", "/tracker/orders?leadId="+lead, nil, 200)
	list, _ := out["orders"].([]any)
	if len(list) == 0 {
		return nil
	}
	return list[0].(map[string]any)
}

func quote(t *testing.T, lead string) string {
	yes := true
	q := call(t, "POST", "/quote/quotes", map[string]any{
		"leadId": lead,
		"profile": map[string]any{"businessType": "Dental clinic", "segment": "Professional offices", "city": "Pune", "cityTier": 1,
			"employeeCount": 6, "numSites": 1, "hasGstin": yes, "preferredLanguage": "Hindi"},
		"items":          []map[string]any{{"productId": "P01"}, {"productId": "P16", "quantity": 6}},
		"contractMonths": 12,
	}, 201)
	return q["id"].(string)
}

func passKYC(t *testing.T, lead, applicant string) {
	call(t, "POST", "/kyc/consents", map[string]any{"leadId": lead, "purposes": []string{"kyc_verification"}, "version": "it-v1"}, 201)
	call(t, "POST", "/kyc/applications/"+lead+"/specimens/"+applicant, nil, 200)
	res := call(t, "POST", "/kyc/applications/"+lead+"/check", nil, 200)
	if res["status"] != "pass" {
		t.Fatalf("KYC for %s: %v", applicant, res["status"])
	}
}

// TestLeadToActivationSurvivesOutages: the tracker and notify are down while the quote is
// accepted and KYC passes; Kafka is down while a second quote is accepted. Nothing is lost.
func TestLeadToActivationSurvivesOutages(t *testing.T) {
	waitHealthy(t)
	lead := fmt.Sprintf("it-%d", time.Now().UnixNano())

	// 1. tracker and notify go down before the quote is accepted
	compose(t, "stop", "tracker", "notify")
	defer compose(t, "start", "tracker", "notify")
	qid := quote(t, lead)
	call(t, "POST", "/quote/quotes/"+qid+"/accept", nil, 200)
	passKYC(t, lead, "K03")

	// 2. they come back and catch up from their committed offsets
	compose(t, "start", "tracker", "notify")
	eventually(t, "order at kyc_passed or later", 60*time.Second, func() bool {
		o := orderFor(t, lead)
		return o != nil && o["status"] != "quote_accepted" && o["status"] != "kyc_pending"
	})

	// 3. the simulator installs and activates
	var o map[string]any
	eventually(t, "order activated", 120*time.Second, func() bool {
		o = orderFor(t, lead)
		return o != nil && o["status"] == "activated"
	})
	tl := call(t, "GET", "/tracker/orders/"+o["id"].(string)+"/timeline", nil, 200)
	var seen []string
	for _, e := range tl["entries"].([]any) {
		seen = append(seen, e.(map[string]any)["status"].(string))
	}
	// Both events were queued while the tracker was down; it may read kyc.passed before
	// quote.accepted (different topics), in which case the order skips kyc_pending. Both are legal.
	got := strings.Join(seen, ",")
	viaPending := got == "quote_accepted,kyc_pending,kyc_passed,install_scheduled,installing,activated"
	if !viaPending && got != "quote_accepted,kyc_passed,install_scheduled,installing,activated" {
		t.Errorf("timeline %v", seen)
	}
	if o["expectedActivationDate"] == "" {
		t.Error("no expected activation date")
	}

	// 4. notify sent every milestone exactly once (in English: this lead never talked to the advisor,
	// which is where notify learns the language)
	eventually(t, "activation message", 30*time.Second, func() bool {
		ms := call(t, "GET", "/notify/messages?leadId="+lead, nil, 200)["messages"].([]any)
		keys := map[string]int{}
		for _, m := range ms {
			mm := m.(map[string]any)
			keys[mm["templateKey"].(string)+"/"+mm["channel"].(string)]++
		}
		want := []string{"order.created/whatsapp", "order.status.kyc_passed/whatsapp",
			"order.status.install_scheduled/whatsapp", "order.status.installing/whatsapp", "order.status.activated/whatsapp", "order.status.activated/sms"}
		if viaPending {
			want = append(want, "order.status.kyc_pending/whatsapp")
		}
		if len(ms) != len(want) {
			return false
		}
		for _, k := range want {
			if keys[k] != 1 {
				return false
			}
		}
		return true
	})
}

// TestOutboxHoldsEventsWhileKafkaIsDown: with Kafka stopped the quote is still accepted (the
// event waits in the quote service's outbox) and the order appears once Kafka is back.
func TestOutboxHoldsEventsWhileKafkaIsDown(t *testing.T) {
	waitHealthy(t)
	lead := fmt.Sprintf("it-kafka-%d", time.Now().UnixNano())
	qid := quote(t, lead)

	compose(t, "stop", "kafka")
	defer compose(t, "start", "kafka")
	call(t, "POST", "/quote/quotes/"+qid+"/accept", nil, 200) // succeeds without Kafka
	time.Sleep(3 * time.Second)
	if orderFor(t, lead) != nil {
		t.Fatal("order exists while Kafka is down?")
	}
	compose(t, "start", "kafka")
	eventually(t, "order created after Kafka returns", 150*time.Second, func() bool { return orderFor(t, lead) != nil })
}

// TestFibreNotFeasibleOffers5G: the simulator's "nofibre" leads hit the delayed branch; the
// tracker offers 5G AirFiber; accepting it swaps the product and resumes installation.
func TestFibreNotFeasibleOffers5G(t *testing.T) {
	waitHealthy(t)
	lead := fmt.Sprintf("it-nofibre-%d", time.Now().UnixNano())
	qid := quote(t, lead)
	call(t, "POST", "/quote/quotes/"+qid+"/accept", nil, 200)
	passKYC(t, lead, "K09")
	var o map[string]any
	eventually(t, "fibre_not_feasible delay", 90*time.Second, func() bool {
		o = orderFor(t, lead)
		return o != nil && o["status"] == "delayed" && o["fallbackOffered"] == true
	})
	call(t, "POST", "/tracker/orders/"+o["id"].(string)+"/fallback/accept", nil, 200)
	eventually(t, "activation on 5G", 120*time.Second, func() bool {
		o = orderFor(t, lead)
		return o != nil && o["status"] == "activated"
	})
	ids := fmt.Sprint(o["productIds"])
	if !strings.Contains(ids, "P04") || strings.Contains(ids, "P01") {
		t.Errorf("products after fallback: %s", ids)
	}
	ms := call(t, "GET", "/notify/messages?leadId="+lead, nil, 200)["messages"].([]any)
	found := false
	for _, m := range ms {
		if m.(map[string]any)["templateKey"] == "order.fallback_offer" {
			found = true
		}
	}
	if !found {
		t.Error("no fallback offer message")
	}
	// invalid moves are refused
	call(t, "POST", "/tracker/orders/"+o["id"].(string)+"/transitions", map[string]string{"to": "installing"}, 409)
}

// TestPoisonEventIsDeadLettered: a malformed event on a consumed topic goes to dlq.events after
// retries instead of blocking the consumer.
func TestPoisonEventIsDeadLettered(t *testing.T) {
	waitHealthy(t)
	root, _ := filepath.Abs("../..")
	marker := fmt.Sprintf("poison-%d", time.Now().UnixNano())
	cmd := exec.Command("docker", "compose", "exec", "-T", "kafka", "/opt/kafka/bin/kafka-console-producer.sh",
		"--bootstrap-server", "kafka:19092", "--topic", "quote.accepted")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "MSYS_NO_PATHCONV=1")
	cmd.Stdin = strings.NewReader(`{"not":"an envelope","marker":"` + marker + `"}` + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("produce: %v %s", err, out)
	}
	eventually(t, "dead letter", 60*time.Second, func() bool {
		c := exec.Command("docker", "compose", "exec", "-T", "kafka", "/opt/kafka/bin/kafka-console-consumer.sh",
			"--bootstrap-server", "kafka:19092", "--topic", "dlq.events", "--from-beginning", "--timeout-ms", "4000")
		c.Dir = root
		c.Env = cmd.Env
		out, _ := c.Output()
		return bytes.Contains(out, []byte(marker))
	})
	// and the tracker still processes normal events afterwards
	lead := fmt.Sprintf("it-after-poison-%d", time.Now().UnixNano())
	qid := quote(t, lead)
	call(t, "POST", "/quote/quotes/"+qid+"/accept", nil, 200)
	eventually(t, "order after poison", 30*time.Second, func() bool { return orderFor(t, lead) != nil })
}
