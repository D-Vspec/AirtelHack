package kyc

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Word is one OCR'd word with its box and confidence (0-100).
type Word struct {
	Text       string
	X, Y, W, H int
	Conf       float64
}

// Line is a run of words tesseract put on one text line, left to right.
type Line struct {
	Words []Word
}

func (l Line) Text() string {
	parts := make([]string, len(l.Words))
	for i, w := range l.Words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

func (l Line) Top() int { return l.Words[0].Y }

// OCR turns a PNG into lines of words, or reads one identifier line.
type OCR interface {
	Lines(ctx context.Context, png []byte) ([]Line, error)
	// Field reads a single line of identifier characters (A-Z, 0-9, '-'); conf is 0-100.
	Field(ctx context.Context, png []byte) (text string, conf float64, err error)
}

// Tesseract runs the tesseract CLI (installed in the kyc container) with TSV output.
type Tesseract struct {
	Bin string // default "tesseract"
}

func (t Tesseract) run(ctx context.Context, png []byte, args ...string) (string, error) {
	bin := t.Bin
	if bin == "" {
		bin = "tesseract"
	}
	cmd := exec.CommandContext(ctx, bin, append([]string{"stdin", "stdout", "-l", "eng"}, args...)...)
	cmd.Stdin = bytes.NewReader(png)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// Lines uses psm 6 (one uniform block), which keeps a label and its value on one line.
func (t Tesseract) Lines(ctx context.Context, png []byte) ([]Line, error) {
	out, err := t.run(ctx, png, "--psm", "6", "tsv")
	if err != nil {
		return nil, err
	}
	return ParseTSV(out), nil
}

// Field uses psm 7 (single line) with an identifier character whitelist.
func (t Tesseract) Field(ctx context.Context, png []byte) (string, float64, error) {
	out, err := t.run(ctx, png, "--psm", "7", "-c", "tessedit_char_whitelist=0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-", "tsv")
	if err != nil {
		return "", 0, err
	}
	var parts []string
	conf := 100.0
	for _, l := range ParseTSV(out) {
		for _, w := range l.Words {
			parts = append(parts, w.Text)
			conf = min(conf, w.Conf)
		}
	}
	if len(parts) == 0 {
		conf = 0
	}
	return strings.Join(parts, ""), conf, nil
}

// ParseTSV groups tesseract TSV word rows (level 5) into lines.
func ParseTSV(tsv string) []Line {
	type key struct{ block, par, line int }
	byLine := map[key]*Line{}
	var order []key
	for i, row := range strings.Split(tsv, "\n") {
		if i == 0 || row == "" {
			continue
		}
		f := strings.Split(strings.TrimRight(row, "\r"), "\t")
		if len(f) < 12 || f[0] != "5" {
			continue
		}
		text := strings.TrimSpace(f[11])
		if text == "" {
			continue
		}
		n := func(j int) int { v, _ := strconv.Atoi(f[j]); return v }
		conf, _ := strconv.ParseFloat(f[10], 64)
		k := key{n(2), n(3), n(4)}
		if byLine[k] == nil {
			byLine[k] = &Line{}
			order = append(order, k)
		}
		byLine[k].Words = append(byLine[k].Words, Word{Text: text, X: n(6), Y: n(7), W: n(8), H: n(9), Conf: conf})
	}
	lines := make([]Line, 0, len(order))
	for _, k := range order {
		l := byLine[k]
		sort.Slice(l.Words, func(a, b int) bool { return l.Words[a].X < l.Words[b].X })
		lines = append(lines, *l)
	}
	sort.SliceStable(lines, func(a, b int) bool { return lines[a].Top() < lines[b].Top() })
	return lines
}
