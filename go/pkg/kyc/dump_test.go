package kyc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// KYC_DUMP=1 prints what OCR read from every specimen (needs tesseract).
func TestDumpSpecimens(t *testing.T) {
	if os.Getenv("KYC_DUMP") == "" {
		t.Skip("set KYC_DUMP=1")
	}
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not installed")
	}
	files, _ := filepath.Glob(specimenDir() + "/" + os.Getenv("KYC_DUMP_GLOB") + "*.png")
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".png")
		data, _ := os.ReadFile(f)
		res, err := Analyze(context.Background(), Tesseract{}, base[4:], data)
		if err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("KYC_DUMP_CROPS"); dir != "" {
			if box, ok := res.Extracted.Boxes[FGSTIN]; ok {
				img, _, _ := image.Decode(bytes.NewReader(data))
				red := RedChannel(img)
				page := red
				if res.SkewDeg != 0 {
					page = Rotate(red, res.SkewDeg)
				}
				png, _ := Upscale2x(Binarize(page).Crop(box.Inset(-8))).PNG()
				_ = os.WriteFile(filepath.Join(dir, base+"_gstin.png"), png, 0o644)
			}
		}
		fmt.Printf("== %s sharp=%.0f skew=%.1f blurry=%v\n", base, res.Sharpness, res.SkewDeg, res.Blurry)
		if os.Getenv("KYC_DUMP_LINES") != "" {
			for _, l := range res.Lines {
				fmt.Println("   |", l)
			}
		}
		for k, v := range res.Extracted.Fields {
			fmt.Printf("   %-20s %q conf=%.0f\n", k, v, res.Extracted.Conf[k])
		}
	}
}

func specimenDir() string {
	if d := os.Getenv("KYC_SPECIMENS"); d != "" {
		return d
	}
	return "../../../data/kyc_documents"
}
