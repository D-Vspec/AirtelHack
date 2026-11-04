package kyc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// OCR turns an image into text.
type OCR func(ctx context.Context, img *image.Gray) (string, error)

// Tesseract runs the tesseract CLI (PNG on stdin, text on stdout). psm 6 = one uniform text block,
// which keeps "label value" rows of the certificates and bills on one line.
func Tesseract(ctx context.Context, img *image.Gray) (string, error) {
	var in, out, errb bytes.Buffer
	if err := png.Encode(&in, img); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "tesseract", "stdin", "stdout", "--psm", "6")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = &in, &out, &errb
	// One thread per process: concurrent uploads each run tesseract, and OpenMP threads per process
	// oversubscribe the CPU (timeouts in the container with 12 parallel applicants).
	cmd.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1")
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// Document types (contracts/kyc-findings.json documentTypes).
const (
	DocGST       = "gst_certificate"
	DocUdyam     = "udyam_certificate"
	DocAddress   = "address_proof"
	DocSignatory = "signatory_id"
)

var RequiredDocs = []string{DocGST, DocUdyam, DocAddress, DocSignatory}

// Fields read from one document. Only what the checks need is kept (data minimisation).
type Fields map[string]string

// label -> field per document type. A label may carry its value on the same line ("GSTIN 27...")
// or on the next line (signatory ID cards print the label above the value). Continuation labels
// append to an earlier field: certificates wrap the address onto a "place of business" line.
var labels = map[string][]struct{ label, field string }{
	DocGST: {
		{"gstin", "gstin"}, {"legal name", "legalName"}, {"trade name", "tradeName"}, {"constitution", "constitution"},
		{"address of principal", "address"}, {"place of business", "address+"}, {"date of liability", "dateOfLiability"},
	},
	DocUdyam: {
		{"udyam reg. number", "udyamNumber"}, {"enterprise name", "enterpriseName"}, {"major activity", "majorActivity"},
		{"city", "city"}, {"state", "state"}, {"date of registration", "dateOfRegistration"},
	},
	DocAddress: {
		{"consumer no.", "consumerNo"}, {"billed to", "billedTo"}, {"supply address", "address"}, {"bill date", "billDate"},
		{"units consumed", ""}, {"amount due", ""},
	},
	DocSignatory: {
		{"name", "name"}, {"designation", "designation"}, {"for business", "forBusiness"}, {"id no.", "idNo"}, {"valid till", "validTill"},
	},
}

// ParseFields extracts labelled fields from OCR text. Lines that match no label continue the
// previous field when it is an address (bills wrap the supply address onto the next line).
func ParseFields(docType, text string) Fields {
	f := Fields{}
	var pending, last string // pending: label seen with no value yet; last: field the previous line wrote
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		matched := false
		for _, l := range labels[docType] {
			if !strings.HasPrefix(lower, l.label) {
				continue
			}
			matched = true
			value := strings.TrimSpace(line[len(l.label):])
			field := strings.TrimSuffix(l.field, "+")
			switch {
			case field == "":
				pending, last = "", ""
			case strings.HasSuffix(l.field, "+"):
				f[field] = strings.TrimSpace(f[field] + " " + value)
				pending, last = "", field
			case value == "":
				f[field] = ""
				pending, last = field, ""
			default:
				f[field], pending, last = value, "", field
			}
			break
		}
		if matched {
			continue
		}
		switch {
		case pending != "":
			f[pending], last, pending = line, pending, ""
		case last == "address":
			f["address"] += " " + line
		}
	}
	return f
}

var (
	dateRe  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	gstinRe = regexp.MustCompile(`[0-9A-Z]{15}`)
)

// IDText normalises an identifier read by OCR: uppercase, no spaces.
func IDText(s string) string { return strings.ToUpper(strings.Join(strings.Fields(s), "")) }

// Date returns the first YYYY-MM-DD in s, or "".
func Date(s string) string { return dateRe.FindString(s) }

// GSTINText returns the 15-character GSTIN candidate in an OCR'd value, or "".
func GSTINText(s string) string { return gstinRe.FindString(IDText(s)) }
