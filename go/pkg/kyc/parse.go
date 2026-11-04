package kyc

import (
	"image"
	"regexp"
	"strings"
	"unicode"
)

// Document types (contracts/kyc-findings.json documentTypes).
const (
	DocGST       = "gst_certificate"
	DocUdyam     = "udyam_certificate"
	DocAddress   = "address_proof"
	DocSignatory = "signatory_id"
)

// DocTypes lists the documents every applicant must submit, in check order.
var DocTypes = []string{DocGST, DocUdyam, DocAddress, DocSignatory}

// Field names extracted per document type.
const (
	FGSTIN        = "gstin"
	FLegalName    = "legal_name"
	FTradeName    = "trade_name"
	FConstitution = "constitution"
	FAddress      = "address"
	FLiability    = "date_of_liability"
	FRegType      = "registration_type"
	FUdyamNumber  = "udyam_number"
	FEnterprise   = "enterprise_name"
	FActivity     = "major_activity"
	FCity         = "city"
	FState        = "state"
	FUdyamDate    = "date_of_registration"
	FConsumerNo   = "consumer_no"
	FBilledTo     = "billed_to"
	FBillDate     = "bill_date"
	FName         = "name"
	FDesignation  = "designation"
	FBusiness     = "business"
	FIDNo         = "id_no"
	FValidTill    = "valid_till"
)

type label struct {
	field string
	text  string // normalised printed label
	cont  bool   // continuation label: appends to field instead of starting it
}

// labels are the printed labels on each specimen layout.
var labels = map[string][]label{
	DocGST: {
		{FGSTIN, "gstin", false}, {FLegalName, "legal name", false}, {FTradeName, "trade name", false},
		{FConstitution, "constitution", false}, {FAddress, "address of principal", false},
		{FAddress, "place of business", true}, {FLiability, "date of liability", false},
		{FRegType, "type of registration", false},
	},
	DocUdyam: {
		{FUdyamNumber, "udyam reg number", false}, {FEnterprise, "enterprise name", false},
		{FActivity, "major activity", false}, {FCity, "city", false}, {FState, "state", false},
		{FUdyamDate, "date of registration", false},
	},
	DocAddress: {
		{FConsumerNo, "consumer no", false}, {FBilledTo, "billed to", false}, {FAddress, "supply address", false},
		{FBillDate, "bill date", false}, {"units", "units consumed", false}, {"amount", "amount due inr", false},
	},
	DocSignatory: {
		{FName, "name", false}, {FDesignation, "designation", false}, {FBusiness, "for business", false},
		{FIDNo, "id no", false}, {FValidTill, "valid till", false},
	},
}

// Extracted is what OCR read from one document.
type Extracted struct {
	Fields map[string]string          `json:"fields"`
	Seen   map[string]bool            `json:"-"` // label printed on the page (value may be blank)
	Conf   map[string]float64         `json:"-"` // lowest word confidence per field
	Boxes  map[string]image.Rectangle `json:"-"` // where the value is printed
}

// SetID replaces an identifier field with a better reading, normalised like Parse does.
func (ex *Extracted) SetID(field, text string, conf float64) {
	if strings.TrimSpace(text) == "" {
		return
	}
	ex.Fields[field] = text
	ex.Conf[field] = conf
	*ex = ex.clean()
}

func newExtracted() Extracted {
	return Extracted{Fields: map[string]string{}, Seen: map[string]bool{}, Conf: map[string]float64{}, Boxes: map[string]image.Rectangle{}}
}

// normLabel keeps lowercase letters and single spaces.
func normLabel(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
			space = false
		} else if !space && b.Len() > 0 {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// matchLabel finds the label that the OCR text most resembles (OCR garbles labels too,
// e.g. "Legaj hame"). Short labels need an exact match.
func matchLabel(doc, text string) (label, bool) {
	t := normLabel(text)
	if t == "" {
		return label{}, false
	}
	var best label
	bestSim := 0.0
	for _, l := range labels[doc] {
		s := Similarity(t, l.text)
		if s > bestSim {
			best, bestSim = l, s
		}
	}
	need := 0.75
	if len(best.text) <= 5 {
		need = 1
	}
	return best, bestSim >= need
}

// Parse turns OCR lines into fields. Certificates and bills print "label   value" on one line
// (values may wrap onto the next line); the signatory ID prints the label above the value.
// pageWidth locates the value column (it starts at about 40% of the page width).
func Parse(doc string, lines []Line, pageWidth int) Extracted {
	if doc == DocSignatory {
		return parseStacked(doc, lines)
	}
	return parseSideBySide(doc, lines, pageWidth)
}

func parseSideBySide(doc string, lines []Line, pageWidth int) Extracted {
	ex := newExtracted()
	last := ""
	for _, ln := range lines {
		labelWords, valueWords := splitColumns(ln, pageWidth*36/100)
		if len(labelWords) > 0 {
			l, ok := matchLabel(doc, joinWords(labelWords))
			if !ok {
				// unknown label line (title, footer): ends any value continuation
				last = ""
				continue
			}
			ex.Seen[l.field] = true
			if l.cont {
				ex.add(l.field, valueWords)
			} else if _, dup := ex.Fields[l.field]; !dup {
				ex.Fields[l.field] = ""
				ex.add(l.field, valueWords)
			}
			last = l.field
			continue
		}
		if last != "" && len(valueWords) > 0 {
			ex.add(last, valueWords) // wrapped value
		}
	}
	return ex.clean()
}

func parseStacked(doc string, lines []Line) Extracted {
	ex := newExtracted()
	pending := ""
	for _, ln := range lines {
		if l, ok := matchLabel(doc, ln.Text()); ok && len(normLabel(ln.Text())) <= len(l.text)+2 {
			if _, dup := ex.Fields[l.field]; !dup {
				ex.Seen[l.field] = true
				ex.Fields[l.field] = ""
				pending = l.field
				continue
			}
		}
		if pending != "" {
			ex.add(pending, ln.Words)
			pending = ""
		}
	}
	return ex.clean()
}

// splitColumns splits a line at the first wide gap: words before it are the label, words
// after it the value. A line that starts in the value column is all value.
func splitColumns(ln Line, valueX int) (lbl, val []Word) {
	ws := ln.Words
	h := 0
	for _, w := range ws {
		if w.H > h {
			h = w.H
		}
	}
	gap := 3 * h // labels have normal word spacing; the label/value gap is much wider
	if gap < 40 {
		gap = 40
	}
	if ws[0].X >= valueX { // starts in the value column: wrapped value
		return nil, ws
	}
	for i := 1; i < len(ws); i++ {
		if ws[i].X-(ws[i-1].X+ws[i-1].W) > gap {
			return ws[:i], ws[i:]
		}
	}
	return ws, nil
}

func joinWords(ws []Word) string {
	parts := make([]string, len(ws))
	for i, w := range ws {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

func (ex *Extracted) add(field string, ws []Word) {
	if len(ws) == 0 {
		return
	}
	v := joinWords(ws)
	if cur := ex.Fields[field]; cur != "" {
		v = cur + " " + v
	}
	ex.Fields[field] = v
	c, ok := ex.Conf[field]
	for _, w := range ws {
		if !ok || w.Conf < c {
			c, ok = w.Conf, true
		}
	}
	ex.Conf[field] = c
	box := ex.Boxes[field]
	for _, w := range ws {
		box = box.Union(image.Rect(w.X, w.Y, w.X+w.W, w.Y+w.H))
	}
	ex.Boxes[field] = box
}

var (
	dateRe  = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// clean normalises identifier and date fields.
func (ex Extracted) clean() Extracted {
	for k, v := range ex.Fields {
		v = strings.TrimSpace(spaceRe.ReplaceAllString(v, " "))
		switch k {
		case FGSTIN:
			v = strings.ToUpper(strings.Map(keepAlnum, v))
		case FUdyamNumber:
			v = normUdyam(v)
		case FLiability, FUdyamDate, FBillDate, FValidTill:
			if m := dateRe.FindString(strings.ReplaceAll(v, " ", "")); m != "" {
				v = m
			}
		}
		ex.Fields[k] = v
	}
	return ex
}

func keepAlnum(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	return -1
}

// normUdyam rebuilds UDYAM-XX-00-0000000 from OCR text; OCR may read the 0s in "00" as O.
func normUdyam(v string) string {
	s := strings.ToUpper(strings.Map(keepAlnum, v))
	if !strings.HasPrefix(s, "UDYAM") || len(s) != 5+2+2+7 {
		return strings.ToUpper(strings.TrimSpace(v))
	}
	digits := strings.Map(func(r rune) rune {
		switch r {
		case 'O':
			return '0'
		case 'I', 'L':
			return '1'
		case 'S':
			return '5'
		case 'B':
			return '8'
		}
		return r
	}, s[7:])
	return "UDYAM-" + s[5:7] + "-" + digits[:2] + "-" + digits[2:]
}

// Similarity is 1 - normalised Levenshtein distance (1 = identical).
func Similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 && len(rb) == 0 {
		return 1
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	n := max(len(ra), len(rb))
	return 1 - float64(prev[len(rb)])/float64(n)
}
