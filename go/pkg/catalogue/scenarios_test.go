package catalogue

import (
	"slices"
	"testing"
)

func load(t *testing.T) []Product {
	t.Helper()
	ps, err := LoadFile("../../../data/product_catalogue.csv")
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 25 {
		t.Fatalf("want 25 products, got %d", len(ps))
	}
	return ps
}

func b(v bool) *bool { return &v }

var (
	micro = SegmentCodes["micro"]
	prof  = SegmentCodes["prof"]
	multi = SegmentCodes["multi"]
	mfg   = SegmentCodes["mfg"]

	kirana    = Profile{BusinessType: "Kirana store", Segment: micro, CityTier: 2, EmployeeCount: 2, NumSites: 1, HasGSTIN: b(false), PreferredLanguage: "Hindi"}
	kiranaGST = Profile{BusinessType: "Kirana store", Segment: micro, CityTier: 2, EmployeeCount: 2, NumSites: 1, HasGSTIN: b(true)}
	clinic    = Profile{BusinessType: "Dental clinic", Segment: prof, CityTier: 1, EmployeeCount: 6, NumSites: 1, HasGSTIN: b(true), PreferredLanguage: "Tamil"}
	caFirm    = Profile{BusinessType: "CA firm", Segment: prof, CityTier: 1, EmployeeCount: 8, NumSites: 1, HasGSTIN: b(true)}
	caFirm10  = Profile{BusinessType: "CA firm", Segment: prof, CityTier: 1, EmployeeCount: 10, NumSites: 1, HasGSTIN: b(true)}
	tier2Off  = Profile{Segment: prof, CityTier: 2, EmployeeCount: 12, NumSites: 1, HasGSTIN: b(true)}
	tier1Off  = Profile{Segment: prof, CityTier: 1, EmployeeCount: 12, NumSites: 1, HasGSTIN: b(true)}
	chain     = Profile{BusinessType: "Restaurant chain", Segment: multi, CityTier: 1, EmployeeCount: 30, NumSites: 5, HasGSTIN: b(true)}
	chain2    = Profile{Segment: multi, CityTier: 1, EmployeeCount: 30, NumSites: 2, HasGSTIN: b(true)}
	chainTiny = Profile{Segment: multi, CityTier: 1, EmployeeCount: 6, NumSites: 5, HasGSTIN: b(true)}
	chainNoGS = Profile{Segment: multi, CityTier: 1, EmployeeCount: 30, NumSites: 5, HasGSTIN: b(false)}
	factory   = Profile{BusinessType: "Auto parts unit", Segment: mfg, CityTier: 2, EmployeeCount: 40, NumSites: 1, HasGSTIN: b(true)}
	workshop  = Profile{Segment: mfg, CityTier: 2, EmployeeCount: 5, NumSites: 1, HasGSTIN: b(false)}
	unknown   = Profile{}
)

type scenario struct {
	name        string
	prof        Profile
	eligible    []string          // must be eligible
	excluded    map[string]string // product -> a reason code it must carry
	alternative map[string]string // excluded product -> first alternative
	bundle      string            // template id that must be offered
	bundleHas   []string
	bundleLacks []string
	bundleQty   map[string]int // product -> quantity in that bundle (0 = must be null/ask)
	items       []QuoteItem
	months      int
	quoteErr    string
	lines       []string
	addedFor    map[string]string
	lineQty     map[string]int
	lineMonths  map[string]int
	installDays int
	toggles     []Toggle
	toggleErr   string
}

func TestScenarios(t *testing.T) {
	all := load(t)
	cases := []scenario{
		// 1. Kirana, Hindi, no GSTIN, wants UPI/WhatsApp.
		{name: "S1 no GSTIN excludes every GSTIN product", prof: kirana,
			excluded: map[string]string{"P14": CodeGSTIN, "P15": CodeGSTIN, "P13": CodeGSTIN, "P05": CodeGSTIN, "P12": CodeGSTIN, "P24": CodeGSTIN}},
		{name: "S1 bundle offers GSTIN-free connectivity and cloud POS", prof: kirana, bundle: "micro-essentials",
			bundleHas: []string{"P01", "P19", "P07"}, bundleLacks: []string{"P15"}, bundleQty: map[string]int{"P01": 1, "P19": 0}},
		{name: "S1 GSTIN unlocks WhatsApp API and SMS", prof: kiranaGST, eligible: []string{"P15", "P14"}},
		{name: "S1 quoting WhatsApp API without GSTIN is refused", prof: kirana, items: []QuoteItem{{ProductID: "P15"}}, quoteErr: ErrNotEligible},
		{name: "S1 quote connectivity plus POS", prof: kirana, items: []QuoteItem{{ProductID: "P01"}, {ProductID: "P19", Quantity: 1}},
			lines: []string{"P01", "P19"}, installDays: 5},
		// 2. Clinic, Tier 1, 6 staff, Tamil.
		{name: "S2 clinic bundle: fibre, email x6, backup, endpoint, optional failover", prof: clinic, bundle: "prof-office",
			bundleHas: []string{"P02", "P16", "P17", "P20", "P09"}, bundleQty: map[string]int{"P16": 6, "P02": 1, "P20": 0}},
		{name: "S2 clinic quote is one monthly price", prof: clinic,
			items: []QuoteItem{{ProductID: "P02"}, {ProductID: "P16"}, {ProductID: "P17"}, {ProductID: "P20", Quantity: 6}, {ProductID: "P09"}},
			lines: []string{"P02", "P16", "P17", "P20", "P09"}, lineQty: map[string]int{"P16": 6, "P20": 6}},
		{name: "S2 failover alone pulls in fibre", prof: clinic, items: []QuoteItem{{ProductID: "P09"}},
			lines: []string{"P09", "P01"}, addedFor: map[string]string{"P01": "P09"}},
		// 3. CA firm, 8 employees, asks for a managed firewall.
		{name: "S3 firewall excluded below 10 employees, endpoint offered", prof: caFirm,
			excluded: map[string]string{"P21": CodeEmployees}, eligible: []string{"P20"}},
		{name: "S3 quoting the firewall is refused", prof: caFirm, items: []QuoteItem{{ProductID: "P21"}, {ProductID: "P02"}}, quoteErr: ErrNotEligible},
		{name: "S3 at 10 employees the firewall is eligible", prof: caFirm10, eligible: []string{"P21"}},
		// 4. Tier 2 owner asks for a Tier 1 only product.
		{name: "S4 Tier 2 cannot buy Biz Fibre 1G; closest is Fibre 300", prof: tier2Off,
			excluded: map[string]string{"P03": CodeCityTier}, alternative: map[string]string{"P03": "P02"}},
		{name: "S4 same office in Tier 1 can", prof: tier1Off, eligible: []string{"P03"}},
		{name: "S4 quoting Fibre 1G in Tier 2 is refused", prof: tier2Off, items: []QuoteItem{{ProductID: "P03"}}, quoteErr: ErrNotEligible},
		// 5. Restaurant chain, 5 sites, Tier 1.
		{name: "S5 chain bundle per site with failover, CCTV and SD-WAN", prof: chain, bundle: "multi-sites",
			bundleHas: []string{"P02", "P09", "P23", "P10"}, bundleQty: map[string]int{"P02": 5, "P09": 5, "P23": 0}},
		{name: "S5 chain quote: one bill, per-site quantities", prof: chain,
			items: []QuoteItem{{ProductID: "P02"}, {ProductID: "P09"}, {ProductID: "P23", Quantity: 10}},
			lines: []string{"P02", "P09", "P23"}, lineQty: map[string]int{"P02": 5, "P09": 5, "P23": 10}},
		{name: "S5 SD-WAN refused below 3 sites", prof: chain2, excluded: map[string]string{"P10": CodeSites}},
		{name: "S5 SD-WAN refused below 10 employees", prof: chainTiny, excluded: map[string]string{"P10": CodeEmployees}},
		{name: "S5 SD-WAN refused without GSTIN", prof: chainNoGS, excluded: map[string]string{"P10": CodeGSTIN}},
		{name: "S5 bundle without SD-WAN when size rules fail", prof: chain2, bundle: "multi-sites", bundleLacks: []string{"P10"}},
		// 6. Small manufacturer, 40 staff.
		{name: "S6 factory bundle: leased line and machine monitoring", prof: factory, bundle: "mfg-factory",
			bundleHas: []string{"P05", "P24", "P20"}},
		{name: "S6 leased line quote is honest about a 21-day install", prof: factory,
			items: []QuoteItem{{ProductID: "P05"}, {ProductID: "P24", Quantity: 8}}, lines: []string{"P05", "P24"}, installDays: 21},
		{name: "S6 Leased Line 100 means 30 days", prof: factory, items: []QuoteItem{{ProductID: "P06"}}, installDays: 30,
			lineMonths: map[string]int{"P06": 24}},
		{name: "S6 5-person workshop without GSTIN falls back to fibre", prof: workshop, bundle: "mfg-factory",
			bundleHas: []string{"P02"}, bundleLacks: []string{"P05", "P03", "P24"}},
		// 7. CCTV without internet.
		{name: "S7 CCTV without internet auto-adds connectivity", prof: kirana, items: []QuoteItem{{ProductID: "P23", Quantity: 4}},
			lines: []string{"P23", "P01"}, addedFor: map[string]string{"P01": "P23"}, lineQty: map[string]int{"P01": 1}},
		{name: "S7 removing the only connectivity under CCTV is blocked", prof: kirana,
			items: []QuoteItem{{ProductID: "P23", Quantity: 4}}, toggles: []Toggle{{ProductID: "P01", Enabled: false}}, toggleErr: ErrDependency},
		{name: "S7 removing CCTV, then connectivity, is allowed", prof: kirana,
			items:   []QuoteItem{{ProductID: "P23", Quantity: 4}, {ProductID: "P07", Quantity: 1}},
			toggles: []Toggle{{ProductID: "P23", Enabled: false}, {ProductID: "P01", Enabled: false}}},
		{name: "S7 swapping connectivity keeps CCTV valid", prof: kirana,
			items: []QuoteItem{{ProductID: "P23", Quantity: 4}}, toggles: []Toggle{{ProductID: "P04", Enabled: true}, {ProductID: "P01", Enabled: false}}},
		// 8. Only a mobile plan.
		{name: "S8 mobile plan only: 1-month contract, nothing forced", prof: kirana, items: []QuoteItem{{ProductID: "P07", Quantity: 1}}, months: 1,
			lines: []string{"P07"}, lineMonths: map[string]int{"P07": 1}, installDays: 1},
		{name: "S8 mobile-only quick path is offered to every segment", prof: factory, bundle: "mobile-only", bundleHas: []string{"P07"}},
		{name: "S8 mobile-only for micro too", prof: kirana, bundle: "mobile-only", bundleHas: []string{"P07"}},
		// 9 is TestTooExpensiveCheaperPath; 10 (code-mixing) is the advisor's job.
		// Contracts, quantities and missing information.
		{name: "contract below a product minimum is raised", prof: kirana, items: []QuoteItem{{ProductID: "P01"}}, months: 1,
			lineMonths: map[string]int{"P01": 12}},
		{name: "contract length must be an offered option", prof: kirana, items: []QuoteItem{{ProductID: "P07", Quantity: 1}}, months: 7, quoteErr: ErrContract},
		{name: "unknown product is rejected", prof: kirana, items: []QuoteItem{{ProductID: "P99"}}, quoteErr: ErrUnknownProduct},
		{name: "duplicate product is rejected", prof: kirana, items: []QuoteItem{{ProductID: "P07", Quantity: 1}, {ProductID: "P07", Quantity: 2}}, quoteErr: ErrInvalidRequest},
		{name: "cameras are never guessed", prof: kirana, items: []QuoteItem{{ProductID: "P23"}}, quoteErr: ErrQuantity},
		{name: "empty profile: tier-1-only and size-gated products need info", prof: unknown,
			excluded: map[string]string{"P03": CodeMissing, "P05": CodeMissing, "P10": CodeMissing}, eligible: []string{"P01", "P07", "P16"}},
		{name: "unknown GSTIN status asks rather than refuses", prof: Profile{Segment: micro, CityTier: 1, EmployeeCount: 3, NumSites: 1},
			excluded: map[string]string{"P15": CodeMissing}},
		{name: "per-site product needs the number of sites", prof: Profile{CityTier: 1, HasGSTIN: b(true)},
			items: []QuoteItem{{ProductID: "P01"}}, quoteErr: ErrQuantity},
	}
	if len(cases) < 30 {
		t.Fatalf("need at least 30 scenarios, have %d", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runScenario(t, c, all) })
	}
}

func runScenario(t *testing.T, c scenario, all []Product) {
	el := Evaluate(c.prof, all)
	elig := el.EligibleByID()
	for _, id := range c.eligible {
		if _, ok := elig[id]; !ok {
			t.Errorf("%s should be eligible", id)
		}
	}
	for id, code := range c.excluded {
		x := findExclusion(el, id)
		if x == nil || !slices.ContainsFunc(x.Reasons, func(r Reason) bool { return r.Code == code }) {
			t.Errorf("%s should be excluded with %s, got %+v", id, code, x)
		}
	}
	for id, alt := range c.alternative {
		if x := findExclusion(el, id); x == nil || len(x.Alternatives) == 0 || x.Alternatives[0] != alt {
			t.Errorf("%s first alternative should be %s, got %+v", id, alt, x)
		}
	}
	if c.bundle != "" {
		bs := Bundles(c.prof, el)
		i := slices.IndexFunc(bs, func(x Bundle) bool { return x.ID == c.bundle })
		if i < 0 {
			t.Fatalf("bundle %s not offered", c.bundle)
		}
		bd := bs[i]
		has := map[string]BundleItem{}
		for _, it := range bd.Items {
			if _, ok := elig[it.ProductID]; !ok {
				t.Errorf("bundle contains ineligible %s", it.ProductID)
			}
			has[it.ProductID] = it
		}
		for _, id := range c.bundleHas {
			if _, ok := has[id]; !ok {
				t.Errorf("bundle should contain %s, has %v", id, keys(has))
			}
		}
		for _, id := range c.bundleLacks {
			if _, ok := has[id]; ok {
				t.Errorf("bundle should not contain %s", id)
			}
		}
		for id, q := range c.bundleQty {
			got := has[id].Quantity
			if (q == 0) != (got == nil) || (got != nil && *got != q) {
				t.Errorf("bundle quantity for %s = %v, want %d (0 = ask)", id, got, q)
			}
		}
	}
	if c.items == nil {
		return
	}
	q, err := BuildQuote(QuoteRequest{Profile: c.prof, Items: c.items, ContractMonths: c.months}, el, all)
	if c.quoteErr != "" {
		if err == nil || err.Code != c.quoteErr {
			t.Fatalf("want quote error %s, got %v", c.quoteErr, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("quote failed: %v", err)
	}
	checkQuote(t, q, elig)
	got := map[string]QuoteLine{}
	for _, l := range q.Lines {
		got[l.ProductID] = l
	}
	if c.lines != nil && len(c.lines) != len(q.Lines) {
		t.Errorf("lines = %v, want %v", keys(got), c.lines)
	}
	for _, id := range c.lines {
		if _, ok := got[id]; !ok {
			t.Errorf("quote missing line %s", id)
		}
	}
	for id, f := range c.addedFor {
		if got[id].AddedFor != f || got[id].Note == "" {
			t.Errorf("%s should be added for %s with a note, got %+v", id, f, got[id])
		}
	}
	for id, n := range c.lineQty {
		if got[id].Quantity != n {
			t.Errorf("%s quantity %d, want %d", id, got[id].Quantity, n)
		}
	}
	for id, m := range c.lineMonths {
		if got[id].ContractMonths != m {
			t.Errorf("%s contract %d, want %d", id, got[id].ContractMonths, m)
		}
	}
	if c.installDays != 0 && q.ExpectedInstallDays != c.installDays {
		t.Errorf("install days %d, want %d", q.ExpectedInstallDays, c.installDays)
	}
	if c.toggles == nil {
		return
	}
	items, terr := ApplyToggles(q.Lines, c.toggles, all)
	if c.toggleErr != "" {
		if terr == nil || terr.Code != c.toggleErr {
			t.Fatalf("want toggle error %s, got %v", c.toggleErr, terr)
		}
		return
	}
	if terr != nil {
		t.Fatalf("toggle failed: %v", terr)
	}
	if len(items) == 0 {
		return // everything removed
	}
	q2, err := BuildQuote(QuoteRequest{Profile: c.prof, Items: items, ContractMonths: c.months}, el, all)
	if err != nil {
		t.Fatalf("re-quote after toggles failed: %v", err)
	}
	checkQuote(t, q2, elig)
}

// checkQuote asserts the invariants every quote must hold.
func checkQuote(t *testing.T, q Quote, elig map[string]Product) {
	t.Helper()
	monthly, oneTime, tcv := 0, 0, 0
	have := map[string]bool{}
	for _, l := range q.Lines {
		have[l.ProductID] = true
	}
	for _, l := range q.Lines {
		p, ok := elig[l.ProductID]
		if !ok {
			t.Errorf("quote line %s is not eligible", l.ProductID)
			continue
		}
		if l.UnitPriceInr < p.PriceMin || l.UnitPriceInr > p.PriceMax {
			t.Errorf("%s unit price %d outside %d-%d", p.ID, l.UnitPriceInr, p.PriceMin, p.PriceMax)
		}
		if l.ContractMonths < p.MinContractMonths {
			t.Errorf("%s contract %d below minimum %d", p.ID, l.ContractMonths, p.MinContractMonths)
		}
		if len(p.RequiresOneOf) > 0 && !slices.ContainsFunc(p.RequiresOneOf, func(d string) bool { return have[d] }) {
			t.Errorf("%s dependency not satisfied", p.ID)
		}
		monthly += l.MonthlyInr
		oneTime += l.OneTimeInr
		tcv += l.MonthlyInr*l.ContractMonths + l.OneTimeInr
	}
	if monthly != q.MonthlyTotalInr || oneTime != q.OneTimeTotalInr || tcv != q.TotalContractValueInr {
		t.Errorf("totals do not add up: %+v", q)
	}
}

// 9. "Too expensive": a cheaper tier on a shorter contract lowers the total cost, shown up front.
func TestTooExpensiveCheaperPath(t *testing.T) {
	all := load(t)
	el := Evaluate(kirana, all)
	full, err := BuildQuote(QuoteRequest{Profile: kirana, Items: []QuoteItem{{ProductID: "P02"}, {ProductID: "P19", Quantity: 1}}, ContractMonths: 24}, el, all)
	if err != nil {
		t.Fatal(err)
	}
	cheap, err := BuildQuote(QuoteRequest{Profile: kirana, Items: []QuoteItem{{ProductID: "P04"}}, ContractMonths: 6}, el, all)
	if err != nil {
		t.Fatal(err)
	}
	if cheap.TotalContractValueInr >= full.TotalContractValueInr || cheap.MonthlyTotalInr >= full.MonthlyTotalInr {
		t.Errorf("cheaper path not cheaper: %d/%d vs %d/%d", cheap.MonthlyTotalInr, cheap.TotalContractValueInr, full.MonthlyTotalInr, full.TotalContractValueInr)
	}
}

func TestUnitPriceStaysInRangeAndFallsWithCommitment(t *testing.T) {
	for _, p := range load(t) {
		prev := p.PriceMax + 1
		for _, m := range ContractOptions {
			for _, n := range []int{1, 2, 5, 10, 50} {
				u := UnitPrice(p, n, m)
				if u < p.PriceMin || u > p.PriceMax {
					t.Errorf("%s n=%d m=%d price %d outside range", p.ID, n, m, u)
				}
			}
			u := UnitPrice(p, 1, m)
			if u > prev {
				t.Errorf("%s price rose with a longer contract", p.ID)
			}
			prev = u
		}
		if UnitPrice(p, 10, 24) != p.PriceMin {
			t.Errorf("%s: 24 months x 10 units should reach price_min", p.ID)
		}
	}
}

func TestTemplatesReferenceRealProducts(t *testing.T) {
	ids := map[string]bool{}
	for _, p := range load(t) {
		ids[p.ID] = true
	}
	for _, tp := range Templates {
		for _, s := range tp.Slots {
			for _, id := range s.Options {
				if !ids[id] {
					t.Errorf("template %s references unknown %s", tp.ID, id)
				}
			}
		}
	}
}

func TestParseRejectsBadCatalogue(t *testing.T) {
	if _, err := Parse([]string{"product_id"}, nil); err == nil {
		t.Error("missing columns should fail")
	}
}

func findExclusion(el Eligibility, id string) *Exclusion {
	for i := range el.Excluded {
		if el.Excluded[i].ProductID == id {
			return &el.Excluded[i]
		}
	}
	return nil
}

func keys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
