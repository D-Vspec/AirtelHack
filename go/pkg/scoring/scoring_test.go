package scoring

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// Go must give the same probability as ml/train.py for the same lead.
func TestParityWithPython(t *testing.T) {
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("testdata/parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Lead        Lead    `json:"lead"`
		Probability float64 `json:"probability"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 40 {
		t.Fatalf("only %d parity cases", len(cases))
	}
	for i, c := range cases {
		got := m.Score(c.Lead).Probability
		if math.Abs(got-c.Probability) > 1e-5 {
			t.Errorf("case %d: go %.6f, python %.6f", i, got, c.Probability)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func TestReasonsFollowTheData(t *testing.T) {
	m, _ := Load()
	base := Lead{Segment: "Professional offices", LeadSource: "Field sales", CityTier: ptr(1.0), EmployeeCount: ptr(8.0), NumSites: ptr(1.0),
		HasGSTIN: ptr(true), MonthlyTelecomSpendInr: ptr(3000.0), CurrentProvider: "Competitor A", CurrentInternet: "Business broadband"}
	noGST := base
	noGST.HasGSTIN = ptr(false)
	if m.Score(base).Probability <= m.Score(noGST).Probability {
		t.Error("a GSTIN should raise the score (9.6% vs 2.0% in the data)")
	}
	ref, wa := base, base
	ref.LeadSource, wa.LeadSource = "Referral", "WhatsApp campaign"
	if m.Score(ref).Probability <= m.Score(wa).Probability {
		t.Error("referrals should outscore WhatsApp campaigns (14.8% vs 2.8%)")
	}
	s := m.Score(noGST)
	if len(s.Reasons) == 0 || len(s.Reasons) > 3 {
		t.Fatalf("reasons: %+v", s.Reasons)
	}
	found := false
	for _, r := range s.Reasons {
		if r.Field == "hasGstin" && r.Effect == "lowers" && r.Value == "no" {
			found = true
		}
	}
	if !found {
		t.Errorf("no GSTIN should be a top reason lowering the score: %+v", s.Reasons)
	}
	if s.Probability <= 0 || s.Probability >= 1 || s.Model == "" {
		t.Errorf("bad score %+v", s)
	}
}

func TestUnknownFieldsUseDefaults(t *testing.T) {
	m, _ := Load()
	s := m.Score(Lead{})
	if s.Probability <= 0 || s.Probability >= 1 {
		t.Errorf("empty lead: %+v", s)
	}
}
