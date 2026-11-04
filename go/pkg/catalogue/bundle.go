package catalogue

import "slices"

// Slot is one position in a bundle template: the first eligible option is used.
type Slot struct {
	Options  []string
	Optional bool
	Why      string
}

// Template is a segment bundle. Product choices are sales design; eligibility, dependencies,
// prices and quantities still come from the catalogue and the profile.
type Template struct {
	ID, Segment, Name, Rationale string
	Slots                        []Slot
}

const AnySegment = "any"

var Templates = []Template{
	{ID: "micro-essentials", Segment: SegmentCodes["micro"], Name: "Shop essentials",
		Rationale: "Micro shops need a cheap, quick setup: one internet connection, GST-ready billing with UPI, and a business mobile line. Kept short because quote complexity loses owners.",
		Slots: []Slot{
			{Options: []string{"P01", "P04"}, Why: "Internet for billing, UPI and orders"},
			{Options: []string{"P19"}, Why: "GST-ready billing and UPI/card acceptance on a tablet"},
			{Options: []string{"P07"}, Why: "Business mobile line"},
			{Options: []string{"P15"}, Optional: true, Why: "Verified WhatsApp channel and catalogue for orders"},
			{Options: []string{"P22"}, Optional: true, Why: "Separate Wi-Fi for customers"},
		}},
	{ID: "prof-office", Segment: SegmentCodes["prof"], Name: "Office ready",
		Rationale: "Professional offices convert best and value reliability and data safety: fast fibre, email for every staff member, backup and device protection, with failover as an option.",
		Slots: []Slot{
			{Options: []string{"P02", "P01", "P04"}, Why: "Fast office internet"},
			{Options: []string{"P16"}, Why: "Business email and documents for every staff member"},
			{Options: []string{"P17"}, Why: "Automatic backup of PCs and phones"},
			{Options: []string{"P20"}, Why: "Antivirus and ransomware protection"},
			{Options: []string{"P09"}, Optional: true, Why: "Stays online when the main link is down"},
			{Options: []string{"P12"}, Optional: true, Why: "Business phone lines with IVR"},
		}},
	{ID: "multi-sites", Segment: SegmentCodes["multi"], Name: "All sites, one bill",
		Rationale: "Multi-site businesses need the same connectivity at every site with failover, cameras and one bill; SD-WAN is offered only when the size rules are met.",
		Slots: []Slot{
			{Options: []string{"P02", "P01", "P04"}, Why: "Internet at every site"},
			{Options: []string{"P09"}, Why: "Failover so no site goes offline"},
			{Options: []string{"P23"}, Why: "Cloud CCTV across sites on one app"},
			{Options: []string{"P10"}, Optional: true, Why: "Central management of links across sites"},
			{Options: []string{"P19"}, Optional: true, Why: "Billing at every counter"},
		}},
	{ID: "mfg-factory", Segment: SegmentCodes["mfg"], Name: "Factory connect",
		Rationale: "Manufacturers need dependable dedicated internet and machine uptime data; leased lines take 21-30 days to install, so the quote says so up front.",
		Slots: []Slot{
			{Options: []string{"P05", "P03", "P02", "P01"}, Why: "Dependable dedicated internet for the plant"},
			{Options: []string{"P24"}, Why: "Machine uptime and energy monitoring"},
			{Options: []string{"P20"}, Why: "Protect office PCs and phones"},
			{Options: []string{"P16"}, Optional: true, Why: "Business email for staff"},
			{Options: []string{"P21"}, Optional: true, Why: "Managed firewall for the plant network"},
		}},
	{ID: "mobile-only", Segment: AnySegment, Name: "Just a mobile plan",
		Rationale: "For owners who only want a business mobile line: 1-month contract, no bundle.",
		Slots:     []Slot{{Options: []string{"P07", "P08"}, Why: "Business mobile line"}}},
}

type BundleItem struct {
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	Unit        string `json:"unit"`
	Quantity    *int   `json:"quantity"`
	Optional    bool   `json:"optional"`
	Why         string `json:"why"`
	AddedFor    string `json:"addedFor,omitempty"`
}

type Bundle struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Segment   string       `json:"segment"`
	Rationale string       `json:"rationale"`
	Items     []BundleItem `json:"items"`
	Omitted   []Exclusion  `json:"omitted,omitempty"`
}

// Dependency is a product added because another product requires one of a set.
type Dependency struct{ ID, For string }

// CompleteDependencies adds, for every chosen product whose requires_one_of is not yet met,
// the first eligible option in catalogue order. Callers ensure chosen products are eligible
// (Evaluate already excludes a product none of whose dependencies is eligible).
func CompleteDependencies(chosen []string, eligible map[string]Product) []Dependency {
	have := slices.Clone(chosen)
	var added []Dependency
	for _, id := range chosen {
		p := eligible[id]
		if len(p.RequiresOneOf) == 0 || slices.ContainsFunc(p.RequiresOneOf, func(d string) bool { return slices.Contains(have, d) }) {
			continue
		}
		for _, d := range p.RequiresOneOf {
			if _, ok := eligible[d]; ok {
				added = append(added, Dependency{ID: d, For: id})
				have = append(have, d)
				break
			}
		}
	}
	return added
}

// Bundles builds the templates for the profile's segment (all segments when unknown) plus the
// mobile-only quick path, using eligible products only.
func Bundles(prof Profile, el Eligibility) []Bundle {
	eligible := el.EligibleByID()
	excluded := map[string]Exclusion{}
	for _, x := range el.Excluded {
		excluded[x.ProductID] = x
	}
	qty := func(p Product) *int {
		if q, ok := DefaultQuantity(p, prof); ok {
			return &q
		}
		return nil
	}
	var out []Bundle
	for _, t := range Templates {
		if prof.Segment != "" && t.Segment != prof.Segment && t.Segment != AnySegment {
			continue
		}
		b := Bundle{ID: t.ID, Name: t.Name, Segment: t.Segment, Rationale: t.Rationale, Items: []BundleItem{}}
		var chosen []string
		for _, s := range t.Slots {
			picked := false
			for _, id := range s.Options {
				if p, ok := eligible[id]; ok {
					b.Items = append(b.Items, BundleItem{p.ID, p.Name, p.Unit, qty(p), s.Optional, s.Why, ""})
					chosen = append(chosen, id)
					picked = true
					break
				}
			}
			if !picked {
				b.Omitted = append(b.Omitted, excluded[s.Options[0]])
			}
		}
		for _, d := range CompleteDependencies(chosen, eligible) {
			p := eligible[d.ID]
			b.Items = append(b.Items, BundleItem{p.ID, p.Name, p.Unit, qty(p), false,
				"Added automatically: " + eligible[d.For].Name + " needs it", d.For})
		}
		out = append(out, b)
	}
	return out
}
