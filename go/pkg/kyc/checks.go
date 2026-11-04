package kyc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"udyam/pkg/events"
	"udyam/pkg/logx"
	"udyam/pkg/registry"
)

// Registry looks identifiers up (GET /gstin/{id}, GET /udyam/{id} on the registry service).
type Registry interface {
	GSTIN(ctx context.Context, id string) (registry.GSTINMatch, error)
	Udyam(ctx context.Context, id string) (registry.Match[registry.Udyam], error)
}

// Finding is one typed result of a pipeline stage.
type Finding struct {
	Code       string            `json:"code"`
	Severity   string            `json:"severity"`
	Route      string            `json:"route"`
	Stage      string            `json:"stage"`
	Document   string            `json:"document,omitempty"`
	MessageKey string            `json:"messageKey"`       // kyc.finding.<CODE>; text per language in contracts/kyc-messages.json
	Params     map[string]string `json:"params,omitempty"` // values for the message; identifiers are masked
	Detail     string            `json:"detail,omitempty"` // English explanation for reviewers
}

// Stages in pipeline order.
const (
	StageCompleteness = "completeness"
	StageQuality      = "image_quality"
	StageOCR          = "ocr"
	StageIdentifiers  = "identifiers"
	StageRegistry     = "registry_status"
	StageNames        = "name_match"
	StageAddress      = "address_match"
	StageAge          = "document_age"
	StageID           = "signatory_id"
)

// Stages lists every stage in order.
var Stages = []string{StageCompleteness, StageQuality, StageOCR, StageIdentifiers, StageRegistry, StageNames, StageAddress, StageAge, StageID}

func newFinding(code, stage, doc, detail string, params map[string]string) Finding {
	return Finding{
		Code: code, Severity: events.DefaultSeverity[code], Route: events.DefaultRoute[code], Stage: stage,
		Document: doc, MessageKey: "kyc.finding." + code, Params: params, Detail: detail,
	}
}

// Policy holds the tunable rules.
type Policy struct {
	ReferenceDate  time.Time // KYC_REFERENCE_DATE; documents are aged against it
	MaxBillAgeDays int       // address proof must be at most this old
	NameSimilarity float64   // below this two names differ (tolerates OCR slips)
	AddrSimilarity float64
}

// DefaultPolicy uses the case's reference date and the 90-day rule.
func DefaultPolicy(ref time.Time) Policy {
	return Policy{ReferenceDate: ref, MaxBillAgeDays: 90, NameSimilarity: 0.85, AddrSimilarity: 0.9}
}

// Verified is what the checks established, used for prefill and the reviewer view.
type Verified struct {
	GSTIN          string          `json:"gstin,omitempty"` // masked in API responses
	GSTINMatchType string          `json:"gstinMatchType,omitempty"`
	GSTINRecord    *registry.GSTIN `json:"gstinRecord,omitempty"`
	UdyamNumber    string          `json:"udyamNumber,omitempty"`
	UdyamMatchType string          `json:"udyamMatchType,omitempty"`
	UdyamRecord    *registry.Udyam `json:"udyamRecord,omitempty"`
}

// Result is the outcome of the whole pipeline for one applicant.
type Result struct {
	Route     string               `json:"route"`
	Findings  []Finding            `json:"findings"`
	Documents map[string]DocResult `json:"documents"`
	Verified  Verified             `json:"verified"`
	CheckedAt time.Time            `json:"checkedAt"`
}

// Codes returns the finding codes, sorted, without duplicates.
func (r Result) Codes() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range r.Findings {
		if !seen[f.Code] {
			seen[f.Code] = true
			out = append(out, f.Code)
		}
	}
	sort.Strings(out)
	return out
}

// routeRank orders routes for the final decision. Reject wins; a re-upload comes before manual
// review because a reviewer should not spend time on an incomplete or unreadable file.
var routeRank = map[string]int{events.RoutePass: 0, events.RouteManualReview: 1, events.RouteReupload: 2, events.RouteReject: 3}

// FinalRoute picks the strongest route among the findings.
func FinalRoute(fs []Finding) string {
	route := events.RoutePass
	for _, f := range fs {
		if routeRank[f.Route] > routeRank[route] {
			route = f.Route
		}
	}
	return route
}

// required fields per document; a printed label with a blank value is FIELD_MISSING.
var required = map[string][]string{
	DocGST:       {FGSTIN, FLegalName, FTradeName, FAddress},
	DocUdyam:     {FUdyamNumber, FEnterprise},
	DocAddress:   {FBilledTo, FAddress, FBillDate},
	DocSignatory: {FName, FIDNo, FValidTill},
}

// Check runs every stage after OCR. docs holds the analysed documents that were submitted.
func Check(ctx context.Context, reg Registry, p Policy, docs map[string]DocResult) (Result, error) {
	res := Result{Documents: docs, Findings: []Finding{}}
	add := func(f Finding) { res.Findings = append(res.Findings, f) }

	// 1. completeness
	for _, d := range DocTypes {
		if _, ok := docs[d]; !ok {
			add(newFinding(events.FindingDocMissing, StageCompleteness, d, d+" was not submitted", map[string]string{"document": d}))
		}
	}
	// 2. image quality (blurry pages are not OCR'd); 3. OCR required fields
	readable := map[string]Extracted{}
	for _, d := range DocTypes {
		dr, ok := docs[d]
		if !ok {
			continue
		}
		for _, f := range (QualityResult{Sharpness: dr.Sharpness, SkewDeg: dr.SkewDeg, Blurry: dr.Blurry}).Findings(d) {
			add(f)
		}
		if dr.Blurry {
			continue
		}
		ex := dr.Extracted
		if ex.Fields == nil {
			ex = newExtracted()
		}
		readable[d] = ex
		for _, f := range required[d] {
			if strings.TrimSpace(ex.Fields[f]) == "" {
				add(newFinding(events.FindingFieldMissing, StageOCR, d, f+" is blank or unreadable", map[string]string{"document": d, "field": f}))
			}
		}
	}

	gst, haveGST := readable[DocGST]
	udy, haveUdyam := readable[DocUdyam]
	bill, haveBill := readable[DocAddress]
	sig, haveSig := readable[DocSignatory]

	// 4. identifiers: checksum, then registry (exact, OCR-corrected, or one-character near miss)
	var gRec *registry.GSTIN
	if haveGST && gst.Fields[FGSTIN] != "" {
		m, err := resolveGSTIN(ctx, reg, gst.Fields[FGSTIN])
		if err != nil {
			return res, err
		}
		res.Verified.GSTINMatchType = m.MatchType
		switch {
		case m.MatchType != "none":
			gRec = m.Record
			res.Verified.GSTIN, res.Verified.GSTINRecord = m.ID, m.Record
		case !m.ChecksumValid:
			res.Verified.GSTIN = m.Query
			params := map[string]string{"gstin": logx.MaskID(m.Query)}
			detail := "GSTIN " + logx.MaskID(m.Query) + " fails the mod-36 checksum"
			if len(m.Suggestions) > 0 {
				params["closest"] = logx.MaskID(m.Suggestions[0])
				detail += "; closest registry GSTIN differs by one character (" + logx.MaskID(m.Suggestions[0]) + ")"
			}
			if len(m.Query) != 15 {
				detail = "GSTIN read as " + fmt.Sprint(len(m.Query)) + " characters; " + detail
			}
			add(newFinding(events.FindingGstinChecksumInvalid, StageIdentifiers, DocGST, detail, params))
		default:
			res.Verified.GSTIN = m.Query
			add(newFinding(events.FindingGstinNotFound, StageIdentifiers, DocGST, "GSTIN "+logx.MaskID(m.Query)+" passes the checksum but is not in the registry", map[string]string{"gstin": logx.MaskID(m.Query)}))
		}
	}
	var uRec *registry.Udyam
	if haveUdyam && udy.Fields[FUdyamNumber] != "" {
		m, err := reg.Udyam(ctx, udy.Fields[FUdyamNumber])
		if err != nil {
			return res, err
		}
		res.Verified.UdyamNumber, res.Verified.UdyamMatchType = m.ID, m.MatchType
		if m.MatchType == "none" {
			res.Verified.UdyamNumber = m.Query
			add(newFinding(events.FindingUdyamNotFound, StageIdentifiers, DocUdyam, "Udyam number "+logx.MaskID(m.Query)+" is not in the registry", map[string]string{"udyam": logx.MaskID(m.Query)}))
		} else {
			uRec = m.Record
			res.Verified.UdyamRecord = m.Record
		}
	}

	// 5. registry status
	if gRec != nil {
		switch strings.ToLower(gRec.Status) {
		case "cancelled":
			add(newFinding(events.FindingGstStatusCancelled, StageRegistry, DocGST, "GST registration is Cancelled in the registry", nil))
		case "suspended":
			add(newFinding(events.FindingGstStatusSuspended, StageRegistry, DocGST, "GST registration is Suspended in the registry", nil))
		}
	}

	// 6. names. The reference business name is the registry trade name; without a registry
	// match, the certificate trade name.
	refBiz, refOwner := "", ""
	constitution := ""
	if gRec != nil {
		refBiz = gRec.TradeName
		refOwner, _ = splitLegalName(gRec.LegalName)
		constitution = gRec.Constitution
	} else if haveGST {
		refBiz = gst.Fields[FTradeName]
		refOwner, _ = splitLegalName(gst.Fields[FLegalName])
		constitution = gst.Fields[FConstitution]
	}
	if refBiz != "" {
		type src struct{ doc, label, name string }
		var names []src
		if haveGST {
			owner, biz := splitLegalName(gst.Fields[FLegalName])
			names = append(names, src{DocGST, "legal name", biz}, src{DocGST, "trade name", gst.Fields[FTradeName]})
			if owner != "" && refOwner != "" && !SameName(owner, refOwner, p.NameSimilarity) {
				add(newFinding(events.FindingNameMismatch, StageNames, DocGST, fmt.Sprintf("proprietor on certificate %q differs from registry %q", owner, refOwner),
					map[string]string{"document": DocGST, "found": owner, "expected": refOwner}))
			}
		}
		if haveUdyam {
			names = append(names, src{DocUdyam, "enterprise name", udy.Fields[FEnterprise]})
		}
		if uRec != nil {
			names = append(names, src{DocUdyam, "Udyam registry enterprise name", uRec.EnterpriseName})
		}
		if haveBill {
			names = append(names, src{DocAddress, "billed to", bill.Fields[FBilledTo]})
		}
		if haveSig {
			names = append(names, src{DocSignatory, "business", sig.Fields[FBusiness]})
		}
		for _, n := range names {
			if n.name != "" && !SameName(n.name, refBiz, p.NameSimilarity) {
				add(newFinding(events.FindingNameMismatch, StageNames, n.doc, fmt.Sprintf("%s %q differs from registered name %q", n.label, n.name, refBiz),
					map[string]string{"document": n.doc, "found": n.name, "expected": refBiz}))
			}
		}
		// signatory: for a proprietorship the signatory must be the proprietor. The registry has
		// no partner or director list, so for LLPs and companies the name cannot be verified here
		// (see docs/kyc-findings.md).
		if haveSig && strings.EqualFold(strings.TrimSpace(constitution), "Proprietorship") && refOwner != "" && sig.Fields[FName] != "" &&
			!SameName(sig.Fields[FName], refOwner, p.NameSimilarity) {
			add(newFinding(events.FindingSignatoryNotAuthorised, StageNames, DocSignatory, fmt.Sprintf("signatory %q is not the proprietor %q", sig.Fields[FName], refOwner),
				map[string]string{"found": sig.Fields[FName], "expected": refOwner}))
		}
	}

	// 7. address: certificate and bill against the registry address; Udyam city/state against it
	refAddr := ""
	if gRec != nil {
		refAddr = gRec.Address
	} else if haveGST {
		refAddr = gst.Fields[FAddress]
	}
	if refAddr != "" {
		for _, a := range []struct{ doc, addr string }{{DocGST, gst.Fields[FAddress]}, {DocAddress, bill.Fields[FAddress]}} {
			if a.addr != "" && Similarity(addrKey(a.addr), addrKey(refAddr)) < p.AddrSimilarity {
				add(newFinding(events.FindingAddressMismatch, StageAddress, a.doc, fmt.Sprintf("address %q differs from registered %q", a.addr, refAddr),
					map[string]string{"document": a.doc}))
			}
		}
		key := addrKey(refAddr)
		for _, c := range []struct{ label, v string }{{"Udyam certificate city", udy.Fields[FCity]}, {"Udyam certificate state", udy.Fields[FState]}} {
			if c.v != "" && !strings.Contains(key, addrKey(c.v)) {
				add(newFinding(events.FindingAddressMismatch, StageAddress, DocUdyam, fmt.Sprintf("%s %q is not in the registered address", c.label, c.v), map[string]string{"document": DocUdyam}))
			}
		}
		if uRec != nil && !strings.Contains(key, addrKey(uRec.City)) {
			add(newFinding(events.FindingAddressMismatch, StageAddress, DocUdyam, fmt.Sprintf("Udyam registry city %q is not in the registered address", uRec.City), map[string]string{"document": DocUdyam}))
		}
	}

	// 8. document age
	if haveBill && bill.Fields[FBillDate] != "" {
		d, err := time.Parse("2006-01-02", bill.Fields[FBillDate])
		if err != nil {
			add(newFinding(events.FindingOcrLowConfidence, StageAge, DocAddress, "bill date unreadable: "+bill.Fields[FBillDate], map[string]string{"document": DocAddress, "field": FBillDate}))
		} else if age := int(p.ReferenceDate.Sub(d).Hours() / 24); age > p.MaxBillAgeDays {
			add(newFinding(events.FindingDocStale, StageAge, DocAddress, fmt.Sprintf("bill dated %s is %d days old at %s (limit %d)", bill.Fields[FBillDate], age, p.ReferenceDate.Format("2006-01-02"), p.MaxBillAgeDays),
				map[string]string{"date": bill.Fields[FBillDate], "days": fmt.Sprint(age), "limit": fmt.Sprint(p.MaxBillAgeDays)}))
		}
	}

	// 9. signatory ID validity
	if haveSig && sig.Fields[FValidTill] != "" {
		d, err := time.Parse("2006-01-02", sig.Fields[FValidTill])
		if err != nil {
			add(newFinding(events.FindingOcrLowConfidence, StageID, DocSignatory, "ID expiry date unreadable: "+sig.Fields[FValidTill], map[string]string{"document": DocSignatory, "field": FValidTill}))
		} else if d.Before(p.ReferenceDate) {
			add(newFinding(events.FindingIdExpired, StageID, DocSignatory, "signatory ID expired on "+sig.Fields[FValidTill], map[string]string{"date": sig.Fields[FValidTill]}))
		}
	}

	res.Route = FinalRoute(res.Findings)
	return res, nil
}

// resolveGSTIN tries the GSTIN as read, then a format repair: positions 1-13 as read, then the
// fixed 'Z', then the last character read as the check digit. Full-page OCR of the monospace
// certificate font turns "1Z" into "17Z", "172" or "171", which this undoes. The repair is
// only accepted when the registry knows it; otherwise the 15-character reading is reported.
func resolveGSTIN(ctx context.Context, reg Registry, raw string) (registry.GSTINMatch, error) {
	cands := []string{raw}
	if len(raw) >= 15 && len(raw) <= 17 {
		if rep := raw[:13] + "Z" + raw[len(raw)-1:]; rep != raw {
			cands = append(cands, rep)
		}
	}
	var fallback *registry.GSTINMatch
	for _, c := range cands {
		m, err := reg.GSTIN(ctx, c)
		if err != nil {
			return m, err
		}
		if m.MatchType != "none" {
			return m, nil
		}
		if fallback == nil || len(c) == 15 {
			mm := m
			fallback = &mm
		}
	}
	return *fallback, nil
}

// splitLegalName splits "Owner Name (Prop. Business)" into owner and business. Other legal
// names are the business itself.
func splitLegalName(s string) (owner, business string) {
	s = strings.TrimSpace(s)
	i := strings.Index(s, "(")
	if i < 0 {
		return "", s
	}
	owner = strings.TrimSpace(s[:i])
	business = strings.TrimSuffix(strings.TrimSpace(s[i+1:]), ")")
	business = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(business, "Prop."), "Prop"))
	return owner, business
}

// NameKey lowercases, treats & as "and" and keeps letters, digits and single spaces.
func NameKey(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "&", " and ")
	var b strings.Builder
	space := true
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// SameName compares two names, tolerating small OCR slips.
func SameName(a, b string, min float64) bool {
	return Similarity(NameKey(a), NameKey(b)) >= min
}

// addrKey keeps only letters and digits: wrapped lines split words ("Maharashtr a"), so
// spacing and punctuation are ignored.
func addrKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
