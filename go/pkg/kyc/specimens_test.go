package kyc

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"udyam/pkg/registry"
)

type expectation struct {
	FinalRoute string   `json:"finalRoute"`
	Codes      []string `json:"codes"`
	Note       string   `json:"note"`
}

func loadExpectations(t *testing.T) (time.Time, map[string]expectation) {
	t.Helper()
	b, err := os.ReadFile("../../../data/kyc_expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		ReferenceDate string                 `json:"referenceDate"`
		Applicants    map[string]expectation `json:"applicants"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	ref, err := time.Parse("2006-01-02", f.ReferenceDate)
	if err != nil {
		t.Fatal(err)
	}
	return ref, f.Applicants
}

// testOCR is real tesseract when KYC_OCR=tesseract (recording answers when KYC_RECORD=1),
// otherwise the recorded answers in testdata/ocr.
func testOCR(t *testing.T) OCR {
	if os.Getenv("KYC_OCR") == "tesseract" {
		if _, err := exec.LookPath("tesseract"); err != nil {
			t.Fatal("KYC_OCR=tesseract but tesseract is not installed")
		}
		if os.Getenv("KYC_RECORD") != "" {
			return RecordingOCR{OCR: Tesseract{}, Dir: "testdata/ocr"}
		}
		return Tesseract{}
	}
	return ReplayOCR{Dir: "testdata/ocr"}
}

func specimenDocs(t *testing.T, id string) map[string][]byte {
	docs := map[string][]byte{}
	for _, d := range DocTypes {
		b, err := os.ReadFile(filepath.Join(specimenDir(), id+"_"+d+".png"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		docs[d] = b
	}
	return docs
}

// The 12-applicant expectation table (data/kyc_expected.json) must pass exactly: same final
// route and the same set of finding codes.
func TestSpecimenExpectations(t *testing.T) {
	ref, want := loadExpectations(t)
	reg, err := registry.LoadCSV("../../../data")
	if err != nil {
		t.Fatal(err)
	}
	ocr := testOCR(t)
	ids := make([]string, 0, len(want))
	for id := range want {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) != 12 {
		t.Fatalf("expected 12 applicants, got %d", len(ids))
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			res, err := Run(context.Background(), ocr, reg, DefaultPolicy(ref), specimenDocs(t, id))
			if err != nil {
				t.Fatal(err)
			}
			exp := want[id]
			wantCodes := append([]string{}, exp.Codes...)
			sort.Strings(wantCodes)
			got := res.Codes()
			if got == nil {
				got = []string{}
			}
			if res.Route != exp.FinalRoute || !reflect.DeepEqual(got, wantCodes) {
				t.Errorf("%s: got route %s codes %v, want route %s codes %v (%s)", id, res.Route, got, exp.FinalRoute, wantCodes, exp.Note)
				for _, f := range res.Findings {
					t.Logf("  %s [%s] %s: %s", f.Code, f.Route, f.Document, f.Detail)
				}
			}
		})
	}
}
