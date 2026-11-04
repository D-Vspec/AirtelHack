package kyc

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"udyam/pkg/events"
	"udyam/pkg/registry"
)

// a clean applicant (K03, Ganesh Woodcraft) as parsed fields; each case mutates one thing.
func cleanDocs() map[string]DocResult {
	doc := func(t string, f map[string]string) DocResult {
		return DocResult{Type: t, Sharpness: 15000, Extracted: Extracted{Fields: f}}
	}
	return map[string]DocResult{
		DocGST: doc(DocGST, map[string]string{FGSTIN: "27XUIXGO767D1ZP", FLegalName: "Joseph Singh (Prop. Ganesh Woodcraft)", FTradeName: "Ganesh Woodcraft",
			FConstitution: "Proprietorship", FAddress: "Shop/Unit 80, Civil Lines, Nagpur, Maharashtr a - 411231"}),
		DocUdyam:     doc(DocUdyam, map[string]string{FUdyamNumber: "UDYAM-MH-00-9516696", FEnterprise: "Ganesh Woodcraft", FCity: "Nagpur", FState: "Maharashtra"}),
		DocAddress:   doc(DocAddress, map[string]string{FBilledTo: "Ganesh Woodcraft", FAddress: "Shop/Unit 80, Civil Lines, Nagpur, Maharashtra - 411231", FBillDate: "2026-05-30"}),
		DocSignatory: doc(DocSignatory, map[string]string{FName: "Joseph Singh", FBusiness: "Ganesh Woodcraft", FIDNo: "SYN-ID-447889", FValidTill: "2028-07-04"}),
	}
}

func testRegistry(t *testing.T) *registry.Local {
	t.Helper()
	reg, err := registry.LoadCSV("../../../data")
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

var ref = time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

func TestChecks(t *testing.T) {
	reg := testRegistry(t)
	// a Suspended registry entry, added for this test
	reg.G = registry.NewIndex(map[string]registry.GSTIN{
		"27XUIXG0767D1ZP": {GSTIN: "27XUIXG0767D1ZP", LegalName: "Joseph Singh (Prop. Ganesh Woodcraft)", TradeName: "Ganesh Woodcraft", Constitution: "Proprietorship",
			Address: "Shop/Unit 80, Civil Lines, Nagpur, Maharashtra - 411231", State: "Maharashtra", Status: "Active"},
		"27AAAAA0000A1Z5": {GSTIN: "27AAAAA0000A1Z5", LegalName: "Joseph Singh (Prop. Ganesh Woodcraft)", TradeName: "Ganesh Woodcraft", Constitution: "Proprietorship",
			Address: "Shop/Unit 80, Civil Lines, Nagpur, Maharashtra - 411231", Status: "Suspended"},
	})
	cases := []struct {
		name   string
		mutate func(map[string]DocResult)
		route  string
		codes  []string
	}{
		{"clean", func(map[string]DocResult) {}, events.RoutePass, nil},
		{"OCR read O for 0 in GSTIN: corrected silently", func(d map[string]DocResult) {}, events.RoutePass, nil},
		{"GSTIN read with 17Z: repaired via registry", func(d map[string]DocResult) { d[DocGST].Extracted.Fields[FGSTIN] = "27XUIXG0767D17ZP" }, events.RoutePass, nil},
		{"suspended GSTIN", func(d map[string]DocResult) { d[DocGST].Extracted.Fields[FGSTIN] = "27AAAAA0000A1Z5" }, events.RouteReject, []string{"GST_STATUS_SUSPENDED"}},
		{"unknown but valid-checksum GSTIN", func(d map[string]DocResult) { d[DocGST].Extracted.Fields[FGSTIN] = "37IHKXV5167L1ZS" }, events.RouteManualReview, []string{"GSTIN_NOT_FOUND"}},
		{"GSTIN fails checksum", func(d map[string]DocResult) { d[DocGST].Extracted.Fields[FGSTIN] = "27XUIXG0767D1ZQ" }, events.RouteManualReview, []string{"GSTIN_CHECKSUM_INVALID"}},
		{"Udyam not in registry", func(d map[string]DocResult) { d[DocUdyam].Extracted.Fields[FUdyamNumber] = "UDYAM-MH-00-1111111" }, events.RouteManualReview, []string{"UDYAM_NOT_FOUND"}},
		{"bill exactly 90 days old passes", func(d map[string]DocResult) { d[DocAddress].Extracted.Fields[FBillDate] = "2026-04-16" }, events.RoutePass, nil},
		{"bill 91 days old is stale", func(d map[string]DocResult) { d[DocAddress].Extracted.Fields[FBillDate] = "2026-04-15" }, events.RouteReupload, []string{"DOC_STALE"}},
		{"expired ID", func(d map[string]DocResult) { d[DocSignatory].Extracted.Fields[FValidTill] = "2026-07-14" }, events.RouteReupload, []string{"ID_EXPIRED"}},
		{"signatory is not the proprietor", func(d map[string]DocResult) { d[DocSignatory].Extracted.Fields[FName] = "Ravi Kumar" }, events.RouteManualReview, []string{"SIGNATORY_NOT_AUTHORISED"}},
		{"OCR slip in a name is tolerated", func(d map[string]DocResult) { d[DocAddress].Extracted.Fields[FBilledTo] = "Ganesh Woodcraf" }, events.RoutePass, nil},
		{"bill to another business", func(d map[string]DocResult) { d[DocAddress].Extracted.Fields[FBilledTo] = "Ganesh Traders" }, events.RouteManualReview, []string{"NAME_MISMATCH"}},
		{"bill at another address", func(d map[string]DocResult) {
			d[DocAddress].Extracted.Fields[FAddress] = "Shop/Unit 12, MG Road, Pune, Maharashtra - 411001"
		}, events.RouteManualReview, []string{"ADDRESS_MISMATCH"}},
		{"Udyam city elsewhere", func(d map[string]DocResult) { d[DocUdyam].Extracted.Fields[FCity] = "Pune" }, events.RouteManualReview, []string{"ADDRESS_MISMATCH"}},
		{"blank required field", func(d map[string]DocResult) { d[DocSignatory].Extracted.Fields[FIDNo] = "" }, events.RouteReupload, []string{"FIELD_MISSING"}},
		{"missing document", func(d map[string]DocResult) { delete(d, DocSignatory) }, events.RouteReupload, []string{"DOC_MISSING"}},
		{"reject outranks reupload and review", func(d map[string]DocResult) {
			d[DocGST].Extracted.Fields[FGSTIN] = "27AAAAA0000A1Z5"
			d[DocAddress].Extracted.Fields[FBillDate] = "2025-01-01"
			d[DocSignatory].Extracted.Fields[FName] = "Ravi Kumar"
		}, events.RouteReject, []string{"DOC_STALE", "GST_STATUS_SUSPENDED", "SIGNATORY_NOT_AUTHORISED"}},
		{"reupload outranks review", func(d map[string]DocResult) {
			d[DocAddress].Extracted.Fields[FBillDate] = "2025-01-01"
			d[DocSignatory].Extracted.Fields[FName] = "Ravi Kumar"
		}, events.RouteReupload, []string{"DOC_STALE", "SIGNATORY_NOT_AUTHORISED"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			docs := cleanDocs()
			c.mutate(docs)
			res, err := Check(context.Background(), reg, DefaultPolicy(ref), docs)
			if err != nil {
				t.Fatal(err)
			}
			got := res.Codes()
			if res.Route != c.route || !reflect.DeepEqual(got, c.codes) {
				t.Errorf("got %s %v, want %s %v", res.Route, got, c.route, c.codes)
				for _, f := range res.Findings {
					t.Logf("  %s: %s", f.Code, f.Detail)
				}
			}
			for _, f := range res.Findings {
				if f.Severity == "" || f.Route == "" || f.MessageKey != "kyc.finding."+f.Code {
					t.Errorf("finding %s is missing severity, route or message key", f.Code)
				}
			}
		})
	}
}

func TestFindingDetailsMaskIdentifiers(t *testing.T) {
	docs := cleanDocs()
	docs[DocGST].Extracted.Fields[FGSTIN] = "27XUIXG0767D1ZQ"
	res, err := Check(context.Background(), testRegistry(t), DefaultPolicy(ref), docs)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range res.Findings {
		for _, s := range append([]string{f.Detail}, values(f.Params)...) {
			if strings.Contains(s, "27XUIXG0767D1Z") {
				t.Errorf("unmasked GSTIN in finding: %q", s)
			}
		}
	}
}

func TestSplitLegalName(t *testing.T) {
	for in, want := range map[string][2]string{
		"Joseph Singh (Prop. Ganesh Woodcraft)":    {"Joseph Singh", "Ganesh Woodcraft"},
		"Neha Banerjee (Prop. Shri Physio Clinic)": {"Neha Banerjee", "Shri Physio Clinic"},
		"Kaveri Stays LLP":                         {"", "Kaveri Stays LLP"},
	} {
		o, b := splitLegalName(in)
		if o != want[0] || b != want[1] {
			t.Errorf("%q -> %q, %q", in, o, b)
		}
	}
}

func TestNormUdyam(t *testing.T) {
	for in, want := range map[string]string{
		"UDYAM-MH-OO-9516696":   "UDYAM-MH-00-9516696",
		"UDYAM - MH-00-95I6696": "UDYAM-MH-00-9516696",
		"":                      "",
	} {
		if got := normUdyam(in); got != want {
			t.Errorf("normUdyam(%q) = %q, want %q", in, got, want)
		}
	}
}

func values(m map[string]string) []string {
	var out []string
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
