//go:build integration

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The June case leads are scored at startup and served highest first, each with reasons.
func TestScoringRanksCaseLeads(t *testing.T) {
	waitHealthy(t)
	out := call(t, "GET", "/scoring/leads?source=case&limit=50", nil, 200)
	leads := out["leads"].([]any)
	if len(leads) != 50 {
		t.Fatalf("got %d case leads", len(leads))
	}
	prev := 2.0
	for _, l := range leads {
		m := l.(map[string]any)
		p := m["probability"].(float64)
		if p > prev || p <= 0 || p >= 1 {
			t.Fatalf("not ranked or out of range: %v after %v", p, prev)
		}
		prev = p
		if len(m["reasons"].([]any)) == 0 {
			t.Fatalf("lead %v has no reasons", m["leadId"])
		}
	}
	s := call(t, "POST", "/scoring/score", map[string]any{"leadSource": "Referral", "hasGstin": true, "segment": "Professional offices"}, 200)
	w := call(t, "POST", "/scoring/score", map[string]any{"leadSource": "WhatsApp campaign", "hasGstin": false, "segment": "Micro retail & services"}, 200)
	if s["probability"].(float64) <= w["probability"].(float64) {
		t.Errorf("referral+GSTIN should outscore WhatsApp without GSTIN: %v vs %v", s["probability"], w["probability"])
	}
}

// lead.unclosed (as the advisor's POST /handoff emits it) opens a partner task that can be claimed.
func TestUnclosedLeadOpensPartnerTask(t *testing.T) {
	waitHealthy(t)
	lead := fmt.Sprintf("it-unclosed-%d", time.Now().UnixNano())
	env := fmt.Sprintf(`{"id":"%s","type":"lead.unclosed","version":1,"occurredAt":"%s","leadId":"%s","correlationId":"it","payload":{"sessionId":"s1","leadId":"%s","reason":"custom_request","summary":"Printing press, in Surat, 25 staff.","profile":{},"transcript":[{"role":"owner","text":"private 5G?"}]}}`,
		lead+"-ev", time.Now().UTC().Format(time.RFC3339), lead, lead)
	root, _ := filepath.Abs("../..")
	cmd := exec.Command("docker", "compose", "exec", "-T", "kafka", "/opt/kafka/bin/kafka-console-producer.sh", "--bootstrap-server", "kafka:19092", "--topic", "lead.unclosed")
	cmd.Dir, cmd.Env, cmd.Stdin = root, append(os.Environ(), "MSYS_NO_PATHCONV=1"), strings.NewReader(env+"\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("produce: %v %s", err, out)
	}
	var id string
	eventually(t, "partner task", 30*time.Second, func() bool {
		for _, x := range call(t, "GET", "/scoring/tasks?status=open", nil, 200)["tasks"].([]any) {
			if m := x.(map[string]any); m["leadId"] == lead {
				id = m["id"].(string)
				return m["summary"] == "Printing press, in Surat, 25 staff." && len(m["transcript"].([]any)) == 1
			}
		}
		return false
	})
	task := call(t, "POST", "/scoring/tasks/"+id+"/claim", map[string]string{"partner": "partner-it"}, 200)
	if task["status"] != "claimed" || task["partner"] != "partner-it" {
		t.Errorf("claim: %v", task)
	}
	call(t, "POST", "/scoring/tasks/"+id+"/claim", map[string]string{"partner": "someone-else"}, 409)
}
