package main

import (
	"encoding/csv"
	"os"
	"testing"

	"udyam/pkg/registry"
)

func loadCSV(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows[1:]
}

func gstinIndex(t *testing.T) *registry.Index[registry.GSTIN] {
	m := map[string]registry.GSTIN{}
	for _, r := range loadCSV(t, "../../../data/mock_gstin_registry.csv") {
		m[r[0]] = registry.GSTIN{GSTIN: r[0], LegalName: r[1], TradeName: r[2], Status: r[7]}
	}
	return registry.NewIndex(m)
}

func TestEveryRegistryGSTINPassesChecksum(t *testing.T) {
	rows := loadCSV(t, "../../../data/mock_gstin_registry.csv")
	if len(rows) != 3534 {
		t.Fatalf("expected 3534 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if !registry.GSTINChecksumValid(r[0]) {
			t.Errorf("checksum fails for registry GSTIN %s", r[0])
		}
	}
}

func TestChecksumRejectsBadInput(t *testing.T) {
	for _, g := range []string{"", "37IHKXV5167L1Z", "37IHKXV5167L1ZT", "37ihkxv5167l1zs", "08YKZXS8189L1ZO"} {
		if registry.GSTINChecksumValid(g) {
			t.Errorf("%q should fail checksum", g)
		}
	}
}

// Certificate GSTINs for K01-K12 as tesseract reads them (docs/kyc-findings.md). OCR mixes up
// O/0 and I/J, so several resolve via correction. K04 is too blurred to OCR; we use the value an
// owner would type for that business.
func TestKYCCertificateGSTINsResolveAsDocumented(t *testing.T) {
	ix := gstinIndex(t)
	cases := []struct {
		applicant, ocr, matchType, id, status string
		checksumOK                            bool
	}{
		{"K01", "37IHKXV5167L1ZS", "exact", "37IHKXV5167L1ZS", "Cancelled", true},
		{"K02", "19E0GXH4115Y1ZL", "corrected", "19EOGXH4115Y1ZL", "Active", false},
		{"K03", "27XUIXGO767D1ZP", "corrected", "27XUIXG0767D1ZP", "Active", false},
		{"K04", "27PZSXS1936O1ZH", "exact", "27PZSXS1936O1ZH", "Active", true},
		{"K05", "32TZAXB5481U1Z1", "exact", "32TZAXB5481U1Z1", "Active", true},
		{"K06", "O8YKZXS8189L1Z0", "none", "", "", false},
		{"K06-typed", "08YKZXS8189L1ZO", "none", "", "", false},
		{"K07", "07CPCXNO398P1ZT", "corrected", "07CPCXN0398P1ZT", "Active", false},
		{"K08", "24X0RXG9365M1Z3", "corrected", "24XORXG9365M1Z3", "Active", false},
		{"K09", "37LDYXK0296F1ZB", "exact", "37LDYXK0296F1ZB", "Active", true},
		{"K10", "27QHIXKO658U1ZL", "corrected", "27QHJXK0658U1ZL", "Active", false},
		{"K11", "03KGZXT1320R1ZZ", "exact", "03KGZXT1320R1ZZ", "Active", true},
		{"K12", "37NBLXS1145E1ZU", "exact", "37NBLXS1145E1ZU", "Active", true},
	}
	for _, c := range cases {
		m := ix.Resolve(c.ocr)
		if m.MatchType != c.matchType || m.ID != c.id {
			t.Errorf("%s: got %s %s, want %s %s", c.applicant, m.MatchType, m.ID, c.matchType, c.id)
			continue
		}
		if m.Record != nil && m.Record.Status != c.status {
			t.Errorf("%s: status %s, want %s", c.applicant, m.Record.Status, c.status)
		}
		if got := registry.GSTINChecksumValid(m.Query); got != c.checksumOK {
			t.Errorf("%s: checksumValid %v, want %v", c.applicant, got, c.checksumOK)
		}
	}
	// K06: one mistyped digit; the near-match must be suggested for review.
	m := ix.Resolve("08YKZXS8189L1ZO")
	if len(m.Suggestions) != 1 || m.Suggestions[0] != "08YKZXS5189L1ZO" {
		t.Errorf("K06 suggestions = %v, want [08YKZXS5189L1ZO]", m.Suggestions)
	}
}

func TestUdyamResolve(t *testing.T) {
	m := map[string]registry.Udyam{}
	for _, r := range loadCSV(t, "../../../data/mock_udyam_registry.csv") {
		m[r[0]] = registry.Udyam{UdyamNumber: r[0], EnterpriseName: r[1]}
	}
	ix := registry.NewIndex(m)
	if got := ix.Resolve("udyam-kl-00-3078801"); got.MatchType != "exact" || got.Record.EnterpriseName != "Balaji Dental Care LLP" {
		t.Errorf("exact (case-insensitive) failed: %+v", got)
	}
	if got := ix.Resolve("UDYAM-KL-OO-3O788O1"); got.MatchType != "corrected" || got.ID != "UDYAM-KL-00-3078801" {
		t.Errorf("corrected failed: %+v", got)
	}
	if got := ix.Resolve("UDYAM-XX-00-0000000"); got.MatchType != "none" {
		t.Errorf("unknown should be none: %+v", got)
	}
}
