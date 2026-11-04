package kyc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReplayOCR serves OCR output recorded from tesseract, keyed by the SHA-256 of the exact PNG
// the pipeline would have sent. It lets the specimen tests run where tesseract is not
// installed. Record with RecordingOCR (see `make kyc-record`).
type ReplayOCR struct{ Dir string }

// RecordingOCR calls the real OCR and saves each answer for ReplayOCR.
type RecordingOCR struct {
	OCR OCR
	Dir string
}

func ocrKey(mode string, png []byte) string {
	h := sha256.Sum256(png)
	return mode + "-" + hex.EncodeToString(h[:12]) + ".txt"
}

func (r ReplayOCR) read(mode string, png []byte) (string, error) {
	b, err := os.ReadFile(filepath.Join(r.Dir, ocrKey(mode, png)))
	if err != nil {
		return "", fmt.Errorf("no recorded OCR for this image (re-record with make kyc-record): %w", err)
	}
	return string(b), nil
}

func (r ReplayOCR) Lines(_ context.Context, png []byte) ([]Line, error) {
	s, err := r.read("lines", png)
	if err != nil {
		return nil, err
	}
	return ParseTSV(s), nil
}

func (r ReplayOCR) Field(_ context.Context, png []byte) (string, float64, error) {
	s, err := r.read("field", png)
	if err != nil {
		return "", 0, err
	}
	text, conf, _ := strings.Cut(strings.TrimRight(s, "\n"), "\t")
	c, _ := strconv.ParseFloat(conf, 64)
	return text, c, nil
}

// Lines records the TSV form so replay goes through the same parser.
func (r RecordingOCR) Lines(ctx context.Context, png []byte) ([]Line, error) {
	t, ok := r.OCR.(Tesseract)
	if !ok {
		return nil, fmt.Errorf("RecordingOCR needs Tesseract")
	}
	out, err := t.run(ctx, png, "--psm", "6", "tsv")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(r.Dir, ocrKey("lines", png)), []byte(out), 0o644); err != nil {
		return nil, err
	}
	return ParseTSV(out), nil
}

func (r RecordingOCR) Field(ctx context.Context, png []byte) (string, float64, error) {
	text, conf, err := r.OCR.Field(ctx, png)
	if err != nil {
		return "", 0, err
	}
	err = os.WriteFile(filepath.Join(r.Dir, ocrKey("field", png)), []byte(fmt.Sprintf("%s\t%g\n", text, conf)), 0o644)
	return text, conf, err
}
