package kyc

import (
	"context"
	"encoding/json"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"udyam/pkg/registry"
)

const data = "../../../data/"

// LoadApplicant reads <id>_<docType>.png for every document type that exists.
func loadApplicant(t *testing.T, id string) map[string]image.Image {
	docs := map[string]image.Image{}
	for _, d := range RequiredDocs {
		f, err := os.Open(data + "kyc_documents/" + id + "_" + d + ".png")
		if err != nil {
			continue
		}
		im, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		docs[d] = im
	}
	return docs
}

// The Phase 4 "done when": every applicant gets the route and finding codes in data/kyc_expected.json.
func TestTwelveApplicants(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed")
	}
	var exp struct {
		ReferenceDate string `json:"referenceDate"`
		Applicants    map[string]struct {
			FinalRoute string   `json:"finalRoute"`
			Codes      []string `json:"codes"`
		} `json:"applicants"`
	}
	b, err := os.ReadFile(data + "kyc_expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &exp); err != nil {
		t.Fatal(err)
	}
	g, u, err := registry.LoadCSV(data+"mock_gstin_registry.csv", data+"mock_udyam_registry.csv")
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := time.Parse(time.DateOnly, exp.ReferenceDate)
	c := NewChecker(IndexRegistry{g, u}, Tesseract, ref)
	if len(exp.Applicants) != 12 {
		t.Fatalf("expected 12 applicants, got %d", len(exp.Applicants))
	}
	for id, want := range exp.Applicants {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			res, err := c.Check(context.Background(), loadApplicant(t, id))
			if err != nil {
				t.Fatal(err)
			}
			var codes []string
			for _, f := range res.Findings {
				codes = append(codes, f.Code)
			}
			slices.Sort(codes)
			wantCodes := slices.Clone(want.Codes)
			slices.Sort(wantCodes)
			if res.Route != want.FinalRoute || !slices.Equal(codes, wantCodes) {
				t.Errorf("route %s codes %v, want route %s codes %v", res.Route, codes, want.FinalRoute, wantCodes)
				for _, f := range res.Findings {
					t.Logf("  %s/%s %s: %s", f.Stage, f.Doc, f.Code, f.Detail)
				}
				for d, f := range res.Fields {
					t.Logf("  %s %v", d, f)
				}
			}
			for _, f := range res.Findings {
				if len(f.Messages) != 10 || f.Messages["hi"] == "" {
					t.Errorf("%s has %d messages", f.Code, len(f.Messages))
				}
			}
		})
	}
}
