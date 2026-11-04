// Package catalogue types data/product_catalogue.csv and holds the deterministic eligibility,
// bundle and dependency rules. Contract: contracts/rules-quote.schema.json. Every rule is read
// from a catalogue column; docs/rules.md maps each rule to its source column.
package catalogue

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
)

type Product struct {
	ID                string   `json:"id"`
	Category          string   `json:"category"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Unit              string   `json:"unit"`
	PriceMin          int      `json:"priceMinInr"`
	PriceMax          int      `json:"priceMaxInr"`
	SetupFee          int      `json:"setupFeeInr"`
	MinContractMonths int      `json:"minContractMonths"`
	CityTiers         []int    `json:"cityTiers"`
	MinEmployees      int      `json:"minEmployees"`
	MinSites          int      `json:"minSites"`
	RequiresGSTIN     bool     `json:"requiresGstin"`
	RequiresOneOf     []string `json:"requiresOneOf,omitempty"`
	InstallDays       int      `json:"installDays"`
	Segments          []string `json:"segments"`
}

// Profile mirrors contracts/profile.schema.json. Zero values mean "not known yet".
type Profile struct {
	BusinessName           string   `json:"businessName,omitempty"`
	BusinessType           string   `json:"businessType,omitempty"`
	Segment                string   `json:"segment,omitempty"`
	City                   string   `json:"city,omitempty"`
	CityTier               int      `json:"cityTier,omitempty"`
	PreferredLanguage      string   `json:"preferredLanguage,omitempty"`
	EmployeeCount          int      `json:"employeeCount,omitempty"`
	NumSites               int      `json:"numSites,omitempty"`
	AnnualTurnoverBand     string   `json:"annualTurnoverBand,omitempty"`
	HasGSTIN               *bool    `json:"hasGstin,omitempty"`
	GSTIN                  string   `json:"gstin,omitempty"`
	HasUdyam               *bool    `json:"hasUdyam,omitempty"`
	UdyamNumber            string   `json:"udyamNumber,omitempty"`
	CurrentInternet        string   `json:"currentInternet,omitempty"`
	CurrentProvider        string   `json:"currentProvider,omitempty"`
	MonthlyTelecomSpendInr float64  `json:"monthlyTelecomSpendInr,omitempty"`
	ProductsOfInterest     []string `json:"productsOfInterest,omitempty"`
}

// SegmentCodes maps catalogue target_segments codes to lead segment names.
var SegmentCodes = map[string]string{
	"micro": "Micro retail & services",
	"prof":  "Professional offices",
	"multi": "Small multi-site businesses",
	"mfg":   "Small manufacturers",
}

// Validate checks the profile values the rules depend on (trust boundary).
func (p Profile) Validate() error {
	if p.CityTier != 0 && p.CityTier != 1 && p.CityTier != 2 {
		return fmt.Errorf("cityTier must be 1 or 2")
	}
	if p.EmployeeCount < 0 || p.NumSites < 0 {
		return fmt.Errorf("employeeCount and numSites must be positive")
	}
	if p.Segment != "" {
		ok := false
		for _, name := range SegmentCodes {
			ok = ok || name == p.Segment
		}
		if !ok {
			return fmt.Errorf("unknown segment %q", p.Segment)
		}
	}
	return nil
}

// Parse builds products from catalogue rows (CSV, or the same columns read from Postgres as text).
func Parse(header []string, rows [][]string) ([]Product, error) {
	col := map[string]int{}
	for i, h := range header {
		col[h] = i
	}
	need := []string{"product_id", "category", "product_name", "description", "unit", "price_min_inr_per_month",
		"price_max_inr_per_month", "one_time_setup_inr", "min_contract_months", "available_city_tiers", "min_employees",
		"min_sites", "requires_gstin", "requires_one_of", "typical_install_days", "target_segments"}
	for _, n := range need {
		if _, ok := col[n]; !ok {
			return nil, fmt.Errorf("catalogue: missing column %s", n)
		}
	}
	var out []Product
	for _, r := range rows {
		var err error
		get := func(n string) string { return strings.TrimSpace(r[col[n]]) }
		num := func(n string) int {
			v, e := strconv.Atoi(get(n))
			if e != nil && err == nil {
				err = fmt.Errorf("catalogue %s: %s=%q: %w", get("product_id"), n, get(n), e)
			}
			return v
		}
		list := func(n, sep string) []string {
			if get(n) == "" {
				return nil
			}
			return strings.Split(get(n), sep)
		}
		p := Product{
			ID: get("product_id"), Category: get("category"), Name: get("product_name"), Description: get("description"),
			Unit: get("unit"), PriceMin: num("price_min_inr_per_month"), PriceMax: num("price_max_inr_per_month"),
			SetupFee: num("one_time_setup_inr"), MinContractMonths: num("min_contract_months"),
			MinEmployees: num("min_employees"), MinSites: num("min_sites"),
			RequiresGSTIN: slices.Contains([]string{"Y", "t", "true"}, get("requires_gstin")),
			RequiresOneOf: list("requires_one_of", "|"), InstallDays: num("typical_install_days"),
			Segments: list("target_segments", ";"),
		}
		for _, t := range list("available_city_tiers", ";") {
			v, e := strconv.Atoi(t)
			if e != nil {
				return nil, fmt.Errorf("catalogue %s: bad tier %q", p.ID, t)
			}
			p.CityTiers = append(p.CityTiers, v)
		}
		if err != nil {
			return nil, err
		}
		if p.PriceMin > p.PriceMax {
			return nil, fmt.Errorf("catalogue %s: price_min > price_max", p.ID)
		}
		out = append(out, p)
	}
	return out, nil
}

// ReadCSV parses a catalogue CSV (tests and tools; the rules service reads Postgres).
func ReadCSV(r io.Reader) ([]Product, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("catalogue: empty file")
	}
	return Parse(rows[0], rows[1:])
}

// LoadFile reads a catalogue CSV from disk.
func LoadFile(path string) ([]Product, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadCSV(f)
}

// DefaultQuantity derives a quantity only where the profile or the unit determines it:
// per site = numSites, per user = employeeCount, single-instance units = 1. Anything else
// (lines, cameras, devices, counters, channels, IPs, vehicles) must come from the owner.
func DefaultQuantity(p Product, prof Profile) (int, bool) {
	switch p.Unit {
	case "per site":
		return prof.NumSites, prof.NumSites > 0
	case "per user":
		return prof.EmployeeCount, prof.EmployeeCount > 0
	case "per account", "per pack", "per number":
		return 1, true
	}
	return 0, false
}
