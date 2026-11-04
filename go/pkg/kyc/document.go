package kyc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"math"

	"udyam/pkg/events"
)

// Image quality thresholds. Calibrated on the 47 specimens: the blurred K04 certificate scores
// about 5, every other page scores above 11,000. Phone photos are noisier than renders, so the
// blur gate sits far below the sharp cluster.
const (
	MinSharpness  = 500.0
	SkewReportDeg = 1.5  // smaller rotations are corrected silently
	MaxSkewDeg    = 15.0 // search range
)

// DocResult is the image and OCR stage for one document.
type DocResult struct {
	Type      string    `json:"type"`
	Sharpness float64   `json:"sharpness"`
	SkewDeg   float64   `json:"skewDeg"`
	Blurry    bool      `json:"blurry"`
	Extracted Extracted `json:"extracted"`
	Lines     []string  `json:"-"` // raw OCR text, for debugging; never stored
}

// MaxImageBytes bounds an upload.
const MaxImageBytes = 8 << 20

// QualityResult is the image-quality stage alone, cheap enough to answer an upload instantly.
type QualityResult struct {
	Sharpness float64
	SkewDeg   float64
	Blurry    bool
	page      *Gray // red channel, straightened
}

// Findings are the owner-facing quality findings for one document.
func (q QualityResult) Findings(doc string) []Finding {
	out := []Finding{}
	if q.Blurry {
		out = append(out, newFinding(events.FindingImageBlurry, StageQuality, doc, fmt.Sprintf("sharpness %.0f below %.0f", q.Sharpness, MinSharpness), map[string]string{"document": doc}))
	} else if abs(q.SkewDeg) >= SkewReportDeg {
		out = append(out, newFinding(events.FindingImageSkewed, StageQuality, doc, fmt.Sprintf("page rotated %.1f degrees; straightened before OCR", q.SkewDeg), map[string]string{"document": doc}))
	}
	return out
}

// Quality decodes the image, measures blur and, if sharp enough, finds and corrects skew.
func Quality(data []byte) (QualityResult, error) {
	var q QualityResult
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return q, fmt.Errorf("decode: %w", err)
	}
	red := RedChannel(img)
	q.Sharpness = math.Round(Sharpness(red))
	if q.Sharpness < MinSharpness {
		q.Blurry = true
		return q, nil
	}
	skew := refineSkew(red, DetectSkew(red, MaxSkewDeg, 0.5))
	q.SkewDeg = math.Round(skew*10) / 10
	q.page = red
	if math.Abs(skew) >= 0.3 {
		q.page = Rotate(red, skew)
	}
	return q, nil
}

// Analyze runs quality checks and, if the image is readable, deskew plus OCR.
func Analyze(ctx context.Context, ocr OCR, doc string, data []byte) (DocResult, error) {
	res := DocResult{Type: doc}
	q, err := Quality(data)
	if err != nil {
		return res, fmt.Errorf("%s: %w", doc, err)
	}
	res.Sharpness, res.SkewDeg, res.Blurry = q.Sharpness, q.SkewDeg, q.Blurry
	if q.Blurry {
		return res, nil // do not OCR a page we already know is unreadable
	}
	page := q.page
	bin := Binarize(page)
	png, err := bin.PNG()
	if err != nil {
		return res, err
	}
	lines, err := ocr.Lines(ctx, png)
	if err != nil {
		return res, err
	}
	for _, l := range lines {
		res.Lines = append(res.Lines, l.Text())
	}
	res.Extracted = Parse(doc, lines, page.W)
	// Identifiers are printed in a monospace font that full-page OCR misreads (1Z -> 17Z).
	// Re-read each identifier from its own crop as a single line limited to A-Z, 0-9 and '-'.
	for _, f := range []string{FGSTIN, FUdyamNumber} {
		box, ok := res.Extracted.Boxes[f]
		if !ok {
			continue
		}
		crop, err := Pad(Thicken(Upscale2x(bin.Crop(box.Inset(-6)))), 20).PNG()
		if err != nil {
			return res, err
		}
		text, conf, err := ocr.Field(ctx, crop)
		if err != nil {
			return res, err
		}
		res.Extracted.SetID(f, text, conf)
	}
	return res, nil
}

// refineSkew searches ±0.5° around a coarse estimate in 0.1° steps.
func refineSkew(g *Gray, coarse float64) float64 {
	if coarse == 0 {
		return 0
	}
	best := coarse
	bestScore := -1.0
	for a := coarse - 0.5; a <= coarse+0.5+1e-9; a += 0.1 {
		if s := profileScore(g, a); s > bestScore {
			best, bestScore = a, s
		}
	}
	return best
}

func profileScore(g *Gray, deg float64) float64 {
	diag := int(math.Hypot(float64(g.W), float64(g.H))) + 2
	bins := make([]float64, diag)
	s, c := math.Sincos(deg * math.Pi / 180)
	cx, cy := float64(g.W)/2, float64(g.H)/2
	for y := 0; y < g.H; y += 2 {
		for x := 0; x < g.W; x += 2 {
			if g.at(x, y) < InkThreshold {
				yr := -(float64(x)-cx)*s + (float64(y)-cy)*c
				bins[int(yr)+diag/2]++
			}
		}
	}
	score := 0.0
	for _, v := range bins {
		score += v * v
	}
	return score
}
