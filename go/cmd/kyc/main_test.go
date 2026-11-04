package main

import (
	"encoding/json"
	"strings"
	"testing"

	"udyam/pkg/kyc"
	"udyam/pkg/registry"
)

func TestSniffAcceptsOnlyPNGAndJPEG(t *testing.T) {
	for in, want := range map[string]bool{
		"\x89PNG\r\n\x1a\nrest": true,
		"\xFF\xD8\xFF\xE0rest":  true,
		"GIF89a":                false,
		"%PDF-1.7":              false,
		"<svg>":                 false,
		"":                      false,
	} {
		if _, ok := sniff([]byte(in)); ok != want {
			t.Errorf("sniff(%q) = %v, want %v", in, ok, want)
		}
	}
}

func TestApplicationViewMasksIdentifiers(t *testing.T) {
	res := &kyc.Result{Route: "pass", Verified: kyc.Verified{
		GSTIN: "27XUIXG0767D1ZP", UdyamNumber: "UDYAM-MH-00-9516696",
		GSTINRecord: &registry.GSTIN{GSTIN: "27XUIXG0767D1ZP", TradeName: "Ganesh Woodcraft"},
		UdyamRecord: &registry.Udyam{UdyamNumber: "UDYAM-MH-00-9516696"},
	}}
	b, _ := json.Marshal(applicationView("lead-1", "pass", nil, res, nil))
	s := string(b)
	for _, raw := range []string{"27XUIXG0767D1ZP", "UDYAM-MH-00-9516696"} {
		if strings.Contains(s, raw) {
			t.Errorf("response contains raw identifier %s: %s", raw, s)
		}
	}
	if !strings.Contains(s, "Ganesh Woodcraft") || !strings.Contains(s, `"documents":[]`) {
		t.Errorf("unexpected view: %s", s)
	}
	if res.Verified.GSTIN != "27XUIXG0767D1ZP" {
		t.Error("view must not modify the stored result")
	}
}

func TestLeadIDValidation(t *testing.T) {
	for in, want := range map[string]bool{"lead-k07": true, "L1:2.3_x": true, "": false, "a/b": false, strings.Repeat("x", 65): false, "x y": false} {
		if leadIDRe.MatchString(in) != want {
			t.Errorf("leadId %q: want %v", in, want)
		}
	}
}
