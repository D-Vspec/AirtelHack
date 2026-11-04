package catalogue

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Exclusion reason codes (contracts/rules-quote.schema.json ExclusionCode).
const (
	CodeCityTier   = "CITY_TIER_UNAVAILABLE"
	CodeEmployees  = "BELOW_MIN_EMPLOYEES"
	CodeSites      = "BELOW_MIN_SITES"
	CodeGSTIN      = "GSTIN_REQUIRED"
	CodeDependency = "DEPENDENCY_UNAVAILABLE"
	CodeMissing    = "MISSING_INFO"
)

type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Exclusion struct {
	ProductID    string   `json:"productId"`
	ProductName  string   `json:"productName"`
	Reasons      []Reason `json:"reasons"`
	Alternatives []string `json:"alternatives,omitempty"`
}

type Eligibility struct {
	Eligible []Product   `json:"eligible"`
	Excluded []Exclusion `json:"excluded"`
}

// EligibleByID indexes the eligible products.
func (e Eligibility) EligibleByID() map[string]Product {
	m := make(map[string]Product, len(e.Eligible))
	for _, p := range e.Eligible {
		m[p.ID] = p
	}
	return m
}

// Evaluate applies every eligibility column to the profile. A product is eligible only when
// every constraint is satisfied by a known value; unknown values give MISSING_INFO, never a guess.
func Evaluate(prof Profile, products []Product) Eligibility {
	reasons := map[string][]Reason{}
	for _, p := range products {
		var rs []Reason
		allTiers := slices.Contains(p.CityTiers, 1) && slices.Contains(p.CityTiers, 2)
		if !allTiers && !slices.Contains(p.CityTiers, prof.CityTier) {
			if prof.CityTier == 0 {
				rs = append(rs, Reason{CodeMissing, fmt.Sprintf("We need your city to check %s (sold in Tier %s cities).", p.Name, tiers(p.CityTiers))})
			} else {
				rs = append(rs, Reason{CodeCityTier, fmt.Sprintf("%s is only sold in Tier %s cities; your city is Tier %d.", p.Name, tiers(p.CityTiers), prof.CityTier)})
			}
		}
		if p.MinEmployees > 1 {
			if prof.EmployeeCount == 0 {
				rs = append(rs, Reason{CodeMissing, fmt.Sprintf("We need your number of employees to check %s (needs at least %d).", p.Name, p.MinEmployees)})
			} else if prof.EmployeeCount < p.MinEmployees {
				rs = append(rs, Reason{CodeEmployees, fmt.Sprintf("%s needs at least %d employees; you have %d.", p.Name, p.MinEmployees, prof.EmployeeCount)})
			}
		}
		if p.MinSites > 1 {
			if prof.NumSites == 0 {
				rs = append(rs, Reason{CodeMissing, fmt.Sprintf("We need your number of sites to check %s (needs at least %d).", p.Name, p.MinSites)})
			} else if prof.NumSites < p.MinSites {
				rs = append(rs, Reason{CodeSites, fmt.Sprintf("%s needs at least %d sites; you have %d.", p.Name, p.MinSites, prof.NumSites)})
			}
		}
		if p.RequiresGSTIN {
			if prof.HasGSTIN == nil {
				rs = append(rs, Reason{CodeMissing, fmt.Sprintf("We need to know if you have a GSTIN to check %s.", p.Name)})
			} else if !*prof.HasGSTIN {
				rs = append(rs, Reason{CodeGSTIN, fmt.Sprintf("%s needs an active GST registration. Registering for GST unlocks it.", p.Name)})
			}
		}
		if len(rs) > 0 {
			reasons[p.ID] = rs
		}
	}
	// Dependency (requires_one_of): at least one of the listed products must itself be eligible.
	byID := map[string]Product{}
	for _, p := range products {
		byID[p.ID] = p
	}
	for _, p := range products {
		if len(p.RequiresOneOf) == 0 || reasons[p.ID] != nil {
			continue
		}
		ok := slices.ContainsFunc(p.RequiresOneOf, func(id string) bool { _, bad := reasons[id]; return !bad && byID[id].ID != "" })
		if !ok {
			reasons[p.ID] = []Reason{{CodeDependency, fmt.Sprintf("%s works on top of %s, and none of those is available to you.", p.Name, names(byID, p.RequiresOneOf))}}
		}
	}

	var out Eligibility
	out.Eligible, out.Excluded = []Product{}, []Exclusion{}
	for _, p := range products {
		if rs, bad := reasons[p.ID]; bad {
			out.Excluded = append(out.Excluded, Exclusion{ProductID: p.ID, ProductName: p.Name, Reasons: rs})
		} else {
			out.Eligible = append(out.Eligible, p)
		}
	}
	for i, x := range out.Excluded {
		out.Excluded[i].Alternatives = alternatives(byID[x.ProductID], out.Eligible)
	}
	return out
}

// alternatives: eligible products in the same category, closest monthly price first, max 3.
func alternatives(p Product, eligible []Product) []string {
	var c []Product
	for _, e := range eligible {
		if e.Category == p.Category {
			c = append(c, e)
		}
	}
	mid := func(x Product) int { return x.PriceMin + x.PriceMax }
	dist := func(x Product) int {
		d := mid(x) - mid(p)
		if d < 0 {
			return -d
		}
		return d
	}
	sort.SliceStable(c, func(i, j int) bool { return dist(c[i]) < dist(c[j]) })
	var ids []string
	for i := 0; i < len(c) && i < 3; i++ {
		ids = append(ids, c[i].ID)
	}
	return ids
}

func tiers(t []int) string {
	s := make([]string, len(t))
	for i, v := range t {
		s[i] = fmt.Sprint(v)
	}
	return strings.Join(s, " and ")
}

func names(byID map[string]Product, ids []string) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = byID[id].Name
	}
	return strings.Join(s, ", ")
}
