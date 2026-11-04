package catalogue

import (
	"fmt"
	"math"
	"slices"
	"time"
)

// Quote error codes (contracts/rules-quote.schema.json QuoteErrorCode).
const (
	ErrInvalidRequest   = "INVALID_REQUEST"
	ErrUnknownProduct   = "UNKNOWN_PRODUCT"
	ErrNotEligible      = "NOT_ELIGIBLE"
	ErrQuantity         = "QUANTITY_REQUIRED"
	ErrContract         = "INVALID_CONTRACT"
	ErrDependency       = "DEPENDENCY_REQUIRED"
	ErrQuoteNotFound    = "QUOTE_NOT_FOUND"
	ErrQuoteAccepted    = "QUOTE_ACCEPTED"
	ErrRulesUnavailable = "RULES_UNAVAILABLE"
)

// ContractOptions are the contract lengths offered (the distinct min_contract_months values).
var ContractOptions = []int{1, 3, 6, 12, 24}

type QuoteItem struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity,omitempty"`
}

type QuoteRequest struct {
	LeadID         string      `json:"leadId,omitempty"`
	Profile        Profile     `json:"profile"`
	Items          []QuoteItem `json:"items"`
	ContractMonths int         `json:"contractMonths,omitempty"`
}

type QuoteLine struct {
	ProductID      string `json:"productId"`
	ProductName    string `json:"productName"`
	Unit           string `json:"unit"`
	Quantity       int    `json:"quantity"`
	UnitPriceInr   int    `json:"unitPriceInr"`
	MonthlyInr     int    `json:"monthlyInr"`
	OneTimeInr     int    `json:"oneTimeInr"`
	ContractMonths int    `json:"contractMonths"`
	InstallDays    int    `json:"installDays"`
	AddedFor       string `json:"addedFor,omitempty"`
	Note           string `json:"note,omitempty"`
}

type Quote struct {
	ID                    string      `json:"id"`
	LeadID                string      `json:"leadId,omitempty"`
	Status                string      `json:"status"`
	ContractMonths        int         `json:"contractMonths"`
	Lines                 []QuoteLine `json:"lines"`
	MonthlyTotalInr       int         `json:"monthlyTotalInr"`
	OneTimeTotalInr       int         `json:"oneTimeTotalInr"`
	TotalContractValueInr int         `json:"totalContractValueInr"`
	ExpectedInstallDays   int         `json:"expectedInstallDays"`
	Notes                 []string    `json:"notes"`
	CreatedAt             time.Time   `json:"createdAt"`
	AcceptedAt            *time.Time  `json:"acceptedAt,omitempty"`
}

type QuoteError struct {
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Excluded []Exclusion `json:"excluded,omitempty"`
}

func (e *QuoteError) Error() string { return e.Code + ": " + e.Message }

// UnitPrice is deterministic within the catalogue range: it starts at price_max and moves toward
// price_min by half for contract length (months/24) and half for volume ((quantity-1)/9, capped).
// 24 months and 10+ units reach price_min; a 1-month single unit pays close to price_max.
func UnitPrice(p Product, quantity, months int) int {
	c := math.Min(float64(months), 24) / 24
	s := math.Min(float64(quantity-1), 9) / 9
	return int(math.Round(float64(p.PriceMax) - float64(p.PriceMax-p.PriceMin)*(0.5*c+0.5*s)))
}

// BuildQuote prices a request against the eligibility result for its profile. It adds missing
// dependencies, derives quantities only from the profile or unit, and raises a line's contract to
// the product's minimum. ID, status and timestamps are set by the caller.
func BuildQuote(req QuoteRequest, el Eligibility, all []Product) (Quote, *QuoteError) {
	months := req.ContractMonths
	if months == 0 {
		months = 12
	}
	if !slices.Contains(ContractOptions, months) {
		return Quote{}, &QuoteError{Code: ErrContract, Message: fmt.Sprintf("contractMonths must be one of %v", ContractOptions)}
	}
	if len(req.Items) == 0 {
		return Quote{}, &QuoteError{Code: ErrInvalidRequest, Message: "items is empty"}
	}
	known := map[string]bool{}
	for _, p := range all {
		known[p.ID] = true
	}
	eligible := el.EligibleByID()
	qty := map[string]int{}
	var ids []string
	var notEligible []Exclusion
	for _, it := range req.Items {
		if !known[it.ProductID] {
			return Quote{}, &QuoteError{Code: ErrUnknownProduct, Message: "unknown product " + it.ProductID}
		}
		if it.Quantity < 0 {
			return Quote{}, &QuoteError{Code: ErrInvalidRequest, Message: "quantity must be positive"}
		}
		if slices.Contains(ids, it.ProductID) {
			return Quote{}, &QuoteError{Code: ErrInvalidRequest, Message: "duplicate product " + it.ProductID}
		}
		if _, ok := eligible[it.ProductID]; !ok {
			for _, x := range el.Excluded {
				if x.ProductID == it.ProductID {
					notEligible = append(notEligible, x)
				}
			}
			continue
		}
		ids = append(ids, it.ProductID)
		qty[it.ProductID] = it.Quantity
	}
	if len(notEligible) > 0 {
		return Quote{}, &QuoteError{Code: ErrNotEligible, Message: "some products are not available for this business", Excluded: notEligible}
	}

	q := Quote{Status: "draft", LeadID: req.LeadID, ContractMonths: months, Lines: []QuoteLine{}, Notes: []string{}}
	addedFor := map[string]string{}
	for _, d := range CompleteDependencies(ids, eligible) {
		ids = append(ids, d.ID)
		addedFor[d.ID] = d.For
	}
	for _, id := range ids {
		p := eligible[id]
		n := qty[id]
		if n == 0 {
			var ok bool
			if n, ok = DefaultQuantity(p, req.Profile); !ok {
				return Quote{}, &QuoteError{Code: ErrQuantity, Message: fmt.Sprintf("how many for %s (%s)?", p.Name, p.Unit)}
			}
		}
		line := QuoteLine{ProductID: id, ProductName: p.Name, Unit: p.Unit, Quantity: n,
			ContractMonths: max(months, p.MinContractMonths), InstallDays: p.InstallDays, AddedFor: addedFor[id]}
		line.UnitPriceInr = UnitPrice(p, n, line.ContractMonths)
		line.MonthlyInr = line.UnitPriceInr * n
		line.OneTimeInr = p.SetupFee * n
		if f := addedFor[id]; f != "" {
			line.Note = fmt.Sprintf("Added because %s needs one of: %s", eligible[f].Name, names(eligible, filterKnown(eligible[f].RequiresOneOf, eligible)))
			q.Notes = append(q.Notes, fmt.Sprintf("%s added automatically: %s needs an internet connection.", p.Name, eligible[f].Name))
		}
		if line.ContractMonths > months {
			line.Note = joinNote(line.Note, fmt.Sprintf("Minimum contract for %s is %d months.", p.Name, p.MinContractMonths))
		}
		q.Lines = append(q.Lines, line)
		q.MonthlyTotalInr += line.MonthlyInr
		q.OneTimeTotalInr += line.OneTimeInr
		q.TotalContractValueInr += line.MonthlyInr*line.ContractMonths + line.OneTimeInr
		q.ExpectedInstallDays = max(q.ExpectedInstallDays, p.InstallDays)
	}
	return q, nil
}

// Toggle is one add-on switch on an existing quote.
type Toggle struct {
	ProductID string `json:"productId"`
	Enabled   bool   `json:"enabled"`
	Quantity  int    `json:"quantity,omitempty"`
}

// ApplyToggles returns the new item list. Removing a product that another item still depends on
// (and that is its only satisfied dependency) is refused with an explanation.
func ApplyToggles(lines []QuoteLine, toggles []Toggle, all []Product) ([]QuoteItem, *QuoteError) {
	byID := map[string]Product{}
	for _, p := range all {
		byID[p.ID] = p
	}
	var items []QuoteItem
	for _, l := range lines {
		items = append(items, QuoteItem{ProductID: l.ProductID, Quantity: l.Quantity})
	}
	for _, t := range toggles {
		i := slices.IndexFunc(items, func(x QuoteItem) bool { return x.ProductID == t.ProductID })
		switch {
		case t.Enabled && i >= 0:
			if t.Quantity > 0 {
				items[i].Quantity = t.Quantity
			}
		case t.Enabled:
			items = append(items, QuoteItem{ProductID: t.ProductID, Quantity: t.Quantity})
		case i >= 0:
			items = slices.Delete(items, i, i+1)
		}
	}
	have := map[string]bool{}
	for _, it := range items {
		have[it.ProductID] = true
	}
	for _, t := range toggles {
		if t.Enabled {
			continue
		}
		for _, it := range items {
			deps := byID[it.ProductID].RequiresOneOf
			if slices.Contains(deps, t.ProductID) && !slices.ContainsFunc(deps, func(d string) bool { return have[d] }) {
				return nil, &QuoteError{Code: ErrDependency, Message: fmt.Sprintf("%s cannot be removed: %s needs one of %s. Remove %s first or keep it.",
					byID[t.ProductID].Name, byID[it.ProductID].Name, names(byID, deps), byID[it.ProductID].Name)}
			}
		}
	}
	return items, nil
}

func filterKnown(ids []string, m map[string]Product) []string {
	var out []string
	for _, id := range ids {
		if _, ok := m[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + " " + b
}
