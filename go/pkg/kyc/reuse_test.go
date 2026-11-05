package kyc

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"udyam/pkg/registry"
)

type reuseReg struct {
	gst  *registry.GSTIN
	udy  *registry.Udyam
	down bool
}

func (r reuseReg) GSTIN(_ context.Context, id string) (registry.GSTINMatch, error) {
	if r.down {
		return registry.GSTINMatch{}, errors.New("down")
	}
	m := registry.GSTINMatch{Match: registry.Match[registry.GSTIN]{Query: id, MatchType: "none"}}
	if r.gst != nil {
		m.MatchType, m.ID, m.Record = "exact", id, r.gst
	}
	return m, nil
}

func (r reuseReg) Udyam(_ context.Context, id string) (registry.Match[registry.Udyam], error) {
	if r.down {
		return registry.Match[registry.Udyam]{}, errors.New("down")
	}
	m := registry.Match[registry.Udyam]{Query: id, MatchType: "none"}
	if r.udy != nil {
		m.MatchType, m.ID, m.Record = "exact", id, r.udy
	}
	return m, nil
}

func TestReuseDecide(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	gst := registry.GSTIN{GSTIN: "33AAAAA0000A1Z5", LegalName: "Smile Dental LLP", Address: "12 Anna Salai, Chennai", Status: "Active"}
	udy := registry.Udyam{UdyamNumber: "UDYAM-TN-02-0000001"}
	prev := func(validTill string) *Result {
		g := gst
		return &Result{Route: "pass",
			Verified:  Verified{GSTIN: gst.GSTIN, GSTINRecord: &g, UdyamNumber: udy.UdyamNumber},
			Documents: map[string]DocResult{DocSignatory: {Extracted: Extracted{Fields: map[string]string{FValidTill: validTill}}}}}
	}
	live := reuseReg{gst: &gst, udy: &udy}
	moved := gst
	moved.Address = "8 Mount Road, Chennai"
	cancelled := gst
	cancelled.Status = "Cancelled"
	p := DefaultReusePolicy()

	cases := []struct {
		name      string
		reg       reuseReg
		prev      *Result
		at        time.Time
		unchanged bool
		want      []string
	}{
		{"same business, recent, valid ID", live, prev("2030-01-01"), now.AddDate(0, -2, 0), true, []string{}},
		{"registry whitespace and case ignored", reuseReg{gst: &registry.GSTIN{LegalName: "SMILE  dental llp", Address: "12 Anna Salai,  Chennai", Status: "Active"}, udy: &udy}, prev("2030-01-01"), now, true, []string{}},
		{"never verified", live, nil, now, true, []string{ReuseNoVerification}},
		{"owner says something changed", live, prev("2030-01-01"), now, false, []string{ReuseOwnerChanged}},
		{"older than a year", live, prev("2030-01-01"), now.AddDate(-1, 0, -1), true, []string{ReuseTooOld}},
		{"signatory ID expired", live, prev("2026-10-01"), now, true, []string{ReuseIDExpired}},
		{"address moved in the registry", reuseReg{gst: &moved, udy: &udy}, prev("2030-01-01"), now, true, []string{ReuseRegistryChanged}},
		{"GSTIN cancelled since", reuseReg{gst: &cancelled, udy: &udy}, prev("2030-01-01"), now, true, []string{ReuseGstinNotActive}},
		{"Udyam gone", reuseReg{gst: &gst}, prev("2030-01-01"), now, true, []string{ReuseUdyamNotFound}},
		{"registry down: no reuse, reported once", reuseReg{down: true}, prev("2030-01-01"), now, true, []string{ReuseRegistryDown}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := p.Decide(context.Background(), c.reg, c.prev, c.at, c.unchanged, now)
			if !reflect.DeepEqual(d.Reasons, c.want) || d.Reusable != (len(c.want) == 0) {
				t.Fatalf("got %v, want reasons %v", d, c.want)
			}
		})
	}
}

func TestReusedResultIsAPass(t *testing.T) {
	now := time.Now().UTC()
	r := Reused(Result{Route: "manual_review", Findings: []Finding{{Code: "NAME_MISMATCH"}}, Verified: Verified{GSTIN: "X"}}, now)
	if r.Route != "pass" || len(r.Findings) != 0 || r.Verified.GSTIN != "X" || !r.CheckedAt.Equal(now) {
		t.Fatalf("unexpected %+v", r)
	}
}
