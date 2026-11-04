package kyc

import (
	"context"
	"fmt"
	"image"
	"regexp"
	"strings"
	"time"

	"udyam/pkg/events"
	"udyam/pkg/logx"
	"udyam/pkg/registry"
)

// Finding is one typed result of a pipeline stage (codes and routes: contracts/kyc-findings.json).
type Finding struct {
	Code       string            `json:"code"`
	Severity   string            `json:"severity"`
	Route      string            `json:"route"`
	Stage      string            `json:"stage"`
	Doc        string            `json:"doc"`
	MessageKey string            `json:"messageKey"`
	Messages   map[string]string `json:"messages"` // language code -> owner-facing text
	Detail     string            `json:"detail"`   // reviewer-facing, identifiers masked
}

// Stages in run order.
var Stages = []string{"completeness", "image_quality", "ocr", "registry", "name_match", "address_match", "document_age", "signatory_id"}

// Result of checking one applicant.
type Result struct {
	Route    string             `json:"route"`
	Findings []Finding          `json:"findings"`
	Fields   map[string]Fields  `json:"fields"` // parsed fields per document type
	GSTIN    *registry.GSTIN    `json:"gstinRecord,omitempty"`
	Udyam    *registry.Udyam    `json:"udyamRecord,omitempty"`
	Skew     map[string]float64 `json:"skewDegrees,omitempty"`
}

// Registry resolves identifiers (exact, OCR-corrected, or none with suggestions).
type Registry interface {
	GSTIN(ctx context.Context, id string) (registry.Match[registry.GSTIN], error)
	Udyam(ctx context.Context, id string) (registry.Match[registry.Udyam], error)
}

// IndexRegistry is a Registry over in-memory indexes (tests, offline tools).
type IndexRegistry struct {
	G *registry.Index[registry.GSTIN]
	U *registry.Index[registry.Udyam]
}

func (r IndexRegistry) GSTIN(_ context.Context, id string) (registry.Match[registry.GSTIN], error) {
	return r.G.Resolve(id), nil
}
func (r IndexRegistry) Udyam(_ context.Context, id string) (registry.Match[registry.Udyam], error) {
	return r.U.Resolve(id), nil
}

// Checker holds the pipeline's dependencies and thresholds.
type Checker struct {
	Registry      Registry
	OCR           OCR
	ReferenceDate time.Time // KYC_REFERENCE_DATE: bills must be within MaxBillAgeDays of it, IDs valid on it
	// ponytail: thresholds calibrated on the 47 specimen scans (sharpness 941+ vs 1.0 for K04, skew 7° vs 0°);
	// recalibrate on real phone captures before production.
	MinSharpness   float64
	MinSkewDegrees float64
	MaxBillAgeDays int
}

func NewChecker(reg Registry, ocr OCR, ref time.Time) Checker {
	return Checker{Registry: reg, OCR: ocr, ReferenceDate: ref, MinSharpness: 100, MinSkewDegrees: 1, MaxBillAgeDays: 90}
}

var routeRank = map[string]int{events.RoutePass: 0, events.RouteManualReview: 1, events.RouteReupload: 2, events.RouteReject: 3}

func finding(code, stage, doc, detail string) Finding {
	msgs := map[string]string{}
	for _, l := range events.KYCLanguages {
		msgs[l] = strings.ReplaceAll(events.KYCMessages[code][l], "{doc}", events.KYCDocumentNames[doc][l])
	}
	return Finding{Code: code, Severity: events.DefaultSeverity[code], Route: events.DefaultRoute[code], Stage: stage, Doc: doc,
		MessageKey: "kyc." + code, Messages: msgs, Detail: detail}
}

// Check runs every stage on one applicant's documents (keyed by document type).
// Errors are infrastructure failures (OCR binary, registry down), not findings.
func (c Checker) Check(ctx context.Context, docs map[string]image.Image) (Result, error) {
	res := Result{Fields: map[string]Fields{}, Skew: map[string]float64{}}
	add := func(f Finding) { res.Findings = append(res.Findings, f) }

	// completeness
	for _, d := range RequiredDocs {
		if docs[d] == nil {
			add(finding(events.FindingDocMissing, "completeness", d, d+" not submitted"))
		}
	}

	// image_quality, then ocr on what is readable
	for _, d := range RequiredDocs {
		if docs[d] == nil {
			continue
		}
		g := Gray(docs[d])
		if s := Sharpness(g); s < c.MinSharpness {
			add(finding(events.FindingImageBlurry, "image_quality", d, fmt.Sprintf("sharpness %.1f below %.0f", s, c.MinSharpness)))
			continue
		}
		if sk := SkewDegrees(g, 15); sk >= c.MinSkewDegrees || sk <= -c.MinSkewDegrees {
			res.Skew[d] = sk
			add(finding(events.FindingImageSkewed, "image_quality", d, fmt.Sprintf("skewed %.2f°, deskewed before OCR", sk)))
			g = Rotate(g, sk)
		}
		text, err := c.OCR(ctx, g)
		if err != nil {
			return res, err
		}
		res.Fields[d] = ParseFields(d, text)
	}
	gst, udy, bill, sig := res.Fields[DocGST], res.Fields[DocUdyam], res.Fields[DocAddress], res.Fields[DocSignatory]

	// ocr: key fields must be readable; a key field printed blank is FIELD_MISSING.
	key := map[string]string{DocGST: "gstin", DocUdyam: "udyamNumber", DocAddress: "billDate", DocSignatory: "name"}
	for _, d := range RequiredDocs {
		f := res.Fields[d]
		if f == nil {
			continue
		}
		v, present := f[key[d]]
		switch {
		case present && strings.TrimSpace(v) == "" && d == DocUdyam:
			add(finding(events.FindingFieldMissing, "ocr", d, "Udyam registration number is blank"))
		case !present || strings.TrimSpace(v) == "",
			d == DocGST && GSTINText(v) == "",
			d == DocAddress && Date(v) == "":
			add(finding(events.FindingOcrLowConfidence, "ocr", d, "could not read "+key[d]))
		}
	}

	// registry: GSTIN (checksum, existence, status) and Udyam number
	if g := GSTINText(gst["gstin"]); g != "" {
		m, err := c.Registry.GSTIN(ctx, g)
		if err != nil {
			return res, err
		}
		switch {
		case m.Record != nil:
			res.GSTIN = m.Record
			switch m.Record.Status {
			case "Cancelled":
				add(finding(events.FindingGstStatusCancelled, "registry", DocGST, "registry status Cancelled for "+logx.MaskID(m.ID)))
			case "Suspended":
				add(finding(events.FindingGstStatusSuspended, "registry", DocGST, "registry status Suspended for "+logx.MaskID(m.ID)))
			}
		case !registry.GSTINChecksumValid(g):
			add(finding(events.FindingGstinChecksumInvalid, "registry", DocGST, fmt.Sprintf("%s fails the checksum; closest registry entries %s", logx.MaskID(g), maskAll(m.Suggestions))))
		default:
			add(finding(events.FindingGstinNotFound, "registry", DocGST, logx.MaskID(g)+" not in the registry"))
		}
	}
	if u := IDText(udy["udyamNumber"]); u != "" {
		m, err := c.Registry.Udyam(ctx, u)
		if err != nil {
			return res, err
		}
		if m.Record != nil {
			res.Udyam = m.Record
		} else {
			add(finding(events.FindingUdyamNotFound, "registry", DocUdyam, logx.MaskID(u)+" not in the Udyam registry"))
		}
	}

	// name_match: every business name against one reference (registry trade name, else the certificate's).
	ref := gst["tradeName"]
	if res.GSTIN != nil {
		ref = res.GSTIN.TradeName
	}
	if ref != "" {
		type src struct{ doc, what, name string }
		_, certBusiness := proprietor(gst["legalName"])
		names := []src{{DocGST, "trade name", gst["tradeName"]}, {DocGST, "legal name", certBusiness},
			{DocUdyam, "enterprise name", udy["enterpriseName"]}, {DocAddress, "billed-to name", bill["billedTo"]},
			{DocSignatory, "business on ID", sig["forBusiness"]}}
		if res.Udyam != nil {
			names = append(names, src{DocUdyam, "Udyam registry enterprise name", res.Udyam.EnterpriseName})
		}
		flagged := map[string]bool{}
		for _, n := range names {
			if n.name == "" || flagged[n.doc] || similar(n.name, ref, 0.85) {
				continue
			}
			flagged[n.doc] = true
			add(finding(events.FindingNameMismatch, "name_match", n.doc, fmt.Sprintf("%s %q differs from %q", n.what, n.name, ref)))
		}
		if res.GSTIN != nil && !flagged[DocGST] {
			certPerson, _ := proprietor(gst["legalName"])
			regPerson, _ := proprietor(res.GSTIN.LegalName)
			if certPerson != "" && regPerson != "" && !similar(certPerson, regPerson, 0.85) {
				add(finding(events.FindingNameMismatch, "name_match", DocGST, fmt.Sprintf("proprietor %q differs from registry %q", certPerson, regPerson)))
			}
		}
	}

	// address_match: certificate and bill against the registry address (else the certificate's);
	// Udyam city and state must appear in it.
	refAddr := gst["address"]
	if res.GSTIN != nil {
		refAddr = res.GSTIN.Address
	}
	if refAddr != "" {
		for _, a := range []struct{ doc, addr string }{{DocGST, gst["address"]}, {DocAddress, bill["address"]}} {
			if a.addr != "" && !sameAddress(a.addr, refAddr) {
				add(finding(events.FindingAddressMismatch, "address_match", a.doc, fmt.Sprintf("address %q differs from %q", a.addr, refAddr)))
			}
		}
		for _, part := range []string{udy["city"], udy["state"]} {
			if part != "" && !strings.Contains(norm(refAddr), norm(part)) {
				add(finding(events.FindingAddressMismatch, "address_match", DocUdyam, fmt.Sprintf("Udyam %q not in address %q", part, refAddr)))
				break
			}
		}
	}

	// document_age: the bill must be dated within MaxBillAgeDays of the reference date.
	if d, err := time.Parse(time.DateOnly, Date(bill["billDate"])); err == nil {
		if age := int(c.ReferenceDate.Sub(d).Hours() / 24); age > c.MaxBillAgeDays {
			add(finding(events.FindingDocStale, "document_age", DocAddress, fmt.Sprintf("bill dated %s is %d days before %s", d.Format(time.DateOnly), age, c.ReferenceDate.Format(time.DateOnly))))
		}
	}

	// signatory_id: valid on the reference date; for a proprietorship the signatory is the proprietor.
	if sig != nil {
		if d, err := time.Parse(time.DateOnly, Date(sig["validTill"])); err == nil && d.Before(c.ReferenceDate) {
			add(finding(events.FindingIdExpired, "signatory_id", DocSignatory, "ID valid till "+d.Format(time.DateOnly)))
		}
		legal, constitution := gst["legalName"], gst["constitution"]
		if res.GSTIN != nil {
			legal, constitution = res.GSTIN.LegalName, res.GSTIN.Constitution
		}
		// ponytail: LLP partners and company directors are not in the mock registry, so only proprietorships
		// are verified; add a partner/director source (MCA) to check the others.
		if person, _ := proprietor(legal); constitution == "Proprietorship" && person != "" && sig["name"] != "" && !similar(sig["name"], person, 0.85) {
			add(finding(events.FindingSignatoryNotAuthorised, "signatory_id", DocSignatory, fmt.Sprintf("signatory %q is not the proprietor %q", sig["name"], person)))
		}
	}

	res.Route = events.RoutePass
	for _, f := range res.Findings {
		if routeRank[f.Route] > routeRank[res.Route] {
			res.Route = f.Route
		}
	}
	return res, nil
}

var propRe = regexp.MustCompile(`^(.*?)\s*\(\s*Prop\.?\s*(.*?)\s*\)\s*$`)

// proprietor splits "Neha Kaur (Prop. Vindhya Tours & Travels)" into person and business; other
// legal names (LLP, company) are the business itself.
func proprietor(legal string) (person, business string) {
	if m := propRe.FindStringSubmatch(legal); m != nil {
		return m[1], m[2]
	}
	return "", legal
}

// norm keeps lowercase letters and digits: OCR punctuation and line wraps do not count.
func norm(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 'a' - 'A'
		}
		return -1
	}, s)
}

// similar compares normalised strings with an edit-distance ratio (tolerates a few OCR slips).
func similar(a, b string, min float64) bool {
	a, b = norm(a), norm(b)
	if a == b {
		return true
	}
	n := max(len(a), len(b))
	return n > 0 && 1-float64(levenshtein(a, b))/float64(n) >= min
}

// sameAddress: every digit (unit number, PIN) must match exactly, since one wrong digit is a different
// place; letters may differ slightly (OCR slips, line wraps).
func sameAddress(a, b string) bool {
	return digits(a) != "" && digits(a) == digits(b) && similar(a, b, 0.9)
}

func digits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func maskAll(ids []string) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = logx.MaskID(id)
	}
	return "[" + strings.Join(out, ", ") + "]"
}
