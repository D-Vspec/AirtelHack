package kyc

import (
	"context"
	"image"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"udyam/pkg/events"
	"udyam/pkg/registry"
)

// Checks the specimens never trigger: take clean K03, change one field in the OCR text, expect that code.
func TestEachCheckFires(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed")
	}
	g, u, err := registry.LoadCSV(data+"mock_gstin_registry.csv", data+"mock_udyam_registry.csv")
	if err != nil {
		t.Fatal(err)
	}
	// A GSTIN with a valid checksum that is not in the registry.
	unknown := ""
	for _, c := range gstinChars {
		if registry.GSTINChecksumValid("27AAAXG0767D1Z" + string(c)) {
			unknown = "27AAAXG0767D1Z" + string(c)
		}
	}
	docs := loadApplicant(t, "K03")
	ref := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name, old, new, want string
	}{
		{"unit number on the bill", "Supply address Shop/Unit 80", "Supply address Shop/Unit 81", events.FindingAddressMismatch},
		{"Udyam state", "State Maharashtra", "State Gujarat", events.FindingAddressMismatch},
		{"signatory is not the proprietor", "Name\nJoseph Singh", "Name\nRavi Kumar", events.FindingSignatoryNotAuthorised},
		{"expired ID", "2028-07-04", "2026-01-31", events.FindingIdExpired},
		{"suspended GSTIN", "27XUIXG0767D1ZP", "03ODMXS1407Q1ZD", events.FindingGstStatusSuspended},
		{"GSTIN not in registry", "27XUIXG0767D1ZP", unknown, events.FindingGstinNotFound},
		{"Udyam not in registry", "9516696", "1111111", events.FindingUdyamNotFound},
		{"bill name", "Billed to Ganesh Woodcraft", "Billed to Mahesh Timber Mart", events.FindingNameMismatch},
		{"proprietor on certificate", "Joseph Singh (Prop.", "Arjun Mehta (Prop.", events.FindingNameMismatch},
		{"unreadable GSTIN", "27XUIXG0767D1ZP", "27XU", events.FindingOcrLowConfidence},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hit := false
			ocr := func(ctx context.Context, img *image.Gray) (string, error) {
				s, err := Tesseract(ctx, img)
				// OCR reads K03's GSTIN as ...G0767... or ...GO767...; normalise before replacing.
				s = strings.ReplaceAll(s, "27XUIXGO767D1ZP", "27XUIXG0767D1ZP")
				if strings.Contains(s, tc.old) {
					hit = true
				}
				return strings.ReplaceAll(s, tc.old, tc.new), err
			}
			res, err := NewChecker(IndexRegistry{g, u}, ocr, ref).Check(context.Background(), docs)
			if err != nil {
				t.Fatal(err)
			}
			if !hit {
				t.Fatalf("%q not in K03 OCR text", tc.old)
			}
			var codes []string
			for _, f := range res.Findings {
				codes = append(codes, f.Code)
			}
			if !slices.Contains(codes, tc.want) {
				t.Errorf("want %s, got %v", tc.want, codes)
			}
			if res.Route == events.RoutePass {
				t.Errorf("route pass despite %s", tc.want)
			}
		})
	}
}

const gstinChars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
