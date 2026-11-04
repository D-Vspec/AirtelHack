package kyc

import (
	"context"
	"time"
)

// Run analyses the submitted documents (keyed by document type) and runs every check.
func Run(ctx context.Context, ocr OCR, reg Registry, p Policy, docs map[string][]byte) (Result, error) {
	analysed := make(map[string]DocResult, len(docs))
	for _, d := range DocTypes {
		data, ok := docs[d]
		if !ok {
			continue
		}
		dr, err := Analyze(ctx, ocr, d, data)
		if err != nil {
			return Result{}, err
		}
		analysed[d] = dr
	}
	res, err := Check(ctx, reg, p, analysed)
	res.CheckedAt = time.Now().UTC()
	return res, err
}
