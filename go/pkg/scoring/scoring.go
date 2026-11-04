// Package scoring serves the lead propensity model trained by ml/train.py. The XGBoost trees are
// exported to model.json and evaluated here in plain Go: the `leaves` library reads only the old
// XGBoost binary format, which XGBoost 3 no longer writes, and walking ~100 depth-3 trees needs no
// sidecar. A parity test checks every probability against Python.
//
// Reasons: each split on a lead's path moves the prediction from the parent node's cover-weighted
// value to the child's; that change is credited to the split's feature (Saabas attribution). The
// credits are summed per input field (all leadSource=... columns count as "leadSource").
package scoring

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
)

//go:embed model.json
var modelJSON []byte

// Lead holds the features known when a lead is created (contracts/profile.schema.json names).
// Pointers distinguish "not known yet" from zero/false.
type Lead struct {
	LeadSource             string   `json:"leadSource,omitempty"`
	Segment                string   `json:"segment,omitempty"`
	PreferredLanguage      string   `json:"preferredLanguage,omitempty"`
	CurrentInternet        string   `json:"currentInternet,omitempty"`
	CurrentProvider        string   `json:"currentProvider,omitempty"`
	CityTier               *float64 `json:"cityTier,omitempty"`
	EmployeeCount          *float64 `json:"employeeCount,omitempty"`
	NumSites               *float64 `json:"numSites,omitempty"`
	AnnualTurnoverBand     string   `json:"annualTurnoverBand,omitempty"`
	HasGSTIN               *bool    `json:"hasGstin,omitempty"`
	HasUdyam               *bool    `json:"hasUdyam,omitempty"`
	MonthlyTelecomSpendInr *float64 `json:"monthlyTelecomSpendInr,omitempty"`
	ProductsOfInterest     []string `json:"productsOfInterest,omitempty"`
}

type feature struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Field   string   `json:"field"`
	Value   string   `json:"value"`
	Default float64  `json:"default"`
	Levels  []string `json:"levels"`
}

type node struct {
	ID   int      `json:"id"`
	Leaf *float64 `json:"leaf"`
	F    int      `json:"f"`
	T    float64  `json:"t"`
	Yes  int      `json:"yes"`
	No   int      `json:"no"`
	V    float64  `json:"v"`
}

type tree struct {
	Nodes []node `json:"nodes"`
	byID  map[int]*node
}

// Model is a loaded propensity model.
type Model struct {
	Version     string                 `json:"version"`
	BaseMargin  float64                `json:"baseMargin"`
	Calibration struct{ A, B float64 } `json:"calibration"`
	Spec        []feature              `json:"spec"`
	Trees       []tree                 `json:"trees"`
	BaseRate    float64                `json:"baseRate"`
	Deciles     []float64              `json:"deciles"` // training-score deciles, for "top 10%" style labels
}

// Load parses the embedded model.
func Load() (*Model, error) {
	var m Model
	if err := json.Unmarshal(modelJSON, &m); err != nil {
		return nil, fmt.Errorf("model.json: %w", err)
	}
	for i := range m.Trees {
		t := &m.Trees[i]
		t.byID = make(map[int]*node, len(t.Nodes))
		for j := range t.Nodes {
			t.byID[t.Nodes[j].ID] = &t.Nodes[j]
		}
	}
	if len(m.Trees) == 0 || len(m.Spec) == 0 {
		return nil, fmt.Errorf("model.json has no trees or features")
	}
	return &m, nil
}

// Encode turns a lead into the model's feature vector (mirror of encode() in ml/train.py).
func (m *Model) Encode(l Lead) []float64 {
	cat := map[string]string{"leadSource": l.LeadSource, "segment": l.Segment, "preferredLanguage": l.PreferredLanguage,
		"currentInternet": l.CurrentInternet, "currentProvider": l.CurrentProvider}
	num := map[string]*float64{"cityTier": l.CityTier, "employeeCount": l.EmployeeCount, "numSites": l.NumSites, "monthlyTelecomSpendInr": l.MonthlyTelecomSpendInr}
	boolean := map[string]*bool{"hasGstin": l.HasGSTIN, "hasUdyam": l.HasUdyam}
	x := make([]float64, len(m.Spec))
	for i, f := range m.Spec {
		switch f.Kind {
		case "onehot":
			if cat[f.Field] == f.Value {
				x[i] = 1
			}
		case "num":
			x[i] = f.Default
			if v := num[f.Field]; v != nil {
				x[i] = *v
			}
		case "ordinal":
			x[i] = f.Default
			if j := slices.Index(f.Levels, l.AnnualTurnoverBand); j >= 0 {
				x[i] = float64(j)
			}
		case "bool":
			x[i] = f.Default
			if v := boolean[f.Field]; v != nil {
				x[i] = 0
				if *v {
					x[i] = 1
				}
			}
		case "count":
			x[i] = float64(len(l.ProductsOfInterest))
		case "interest":
			if slices.Contains(l.ProductsOfInterest, f.Value) {
				x[i] = 1
			}
		}
	}
	return x
}

// Reason is one input field's push on the score, in log-odds units.
type Reason struct {
	Field        string  `json:"field"`
	Value        string  `json:"value"`
	Effect       string  `json:"effect"` // raises | lowers
	Contribution float64 `json:"contribution"`
	Text         string  `json:"text"`
}

// Score is the model's answer for one lead.
type Score struct {
	Probability float64  `json:"probability"`
	Band        string   `json:"band"` // high | medium | low, from the training-score deciles
	Reasons     []Reason `json:"reasons"`
	Model       string   `json:"model"`
}

// Score evaluates the trees, calibrates, and explains the three strongest pushes.
func (m *Model) Score(l Lead) Score {
	x := m.Encode(l)
	margin := m.BaseMargin
	contrib := map[string]float64{}
	for i := range m.Trees {
		t := &m.Trees[i]
		n := t.byID[0]
		for n.Leaf == nil {
			next := t.byID[n.No]
			if x[n.F] < n.T {
				next = t.byID[n.Yes]
			}
			f := m.Spec[n.F]
			key := f.Field
			if f.Kind == "num" || f.Kind == "ordinal" || f.Kind == "bool" || f.Kind == "count" {
				key = f.Name
			}
			contrib[key] += next.V - n.V
			n = next
		}
		margin += *n.Leaf
	}
	p := 1 / (1 + math.Exp(-(m.Calibration.A*margin + m.Calibration.B)))
	band := "low"
	if len(m.Deciles) == 9 {
		switch {
		case p >= m.Deciles[7]:
			band = "high" // top ~20% of training leads
		case p >= m.Deciles[3]:
			band = "medium"
		}
	}
	return Score{Probability: p, Band: band, Reasons: reasons(contrib, l, 3), Model: m.Version}
}

func reasons(contrib map[string]float64, l Lead, k int) []Reason {
	keys := make([]string, 0, len(contrib))
	for f := range contrib {
		keys = append(keys, f)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := math.Abs(contrib[keys[i]]), math.Abs(contrib[keys[j]])
		if a != b {
			return a > b
		}
		return keys[i] < keys[j]
	})
	out := []Reason{}
	for _, f := range keys {
		c := contrib[f]
		if len(out) == k || math.Abs(c) < 0.01 {
			break
		}
		r := Reason{Field: f, Value: valueOf(f, l), Effect: "raises", Contribution: math.Round(c*1000) / 1000}
		if c < 0 {
			r.Effect = "lowers"
		}
		r.Text = fmt.Sprintf("%s: %s (%s the score)", label[f], r.Value, r.Effect)
		out = append(out, r)
	}
	return out
}

var label = map[string]string{
	"leadSource": "Lead source", "segment": "Segment", "preferredLanguage": "Language", "currentInternet": "Current internet",
	"currentProvider": "Current provider", "cityTier": "City tier", "employeeCount": "Employees", "numSites": "Sites",
	"annualTurnoverBand": "Turnover", "hasGstin": "GSTIN", "hasUdyam": "Udyam", "monthlyTelecomSpendInr": "Telecom spend",
	"interestCount": "Products asked about", "productsOfInterest": "Products of interest",
}

func valueOf(field string, l Lead) string {
	f := func(p *float64) string {
		if p == nil {
			return "unknown"
		}
		return fmt.Sprintf("%g", *p)
	}
	b := func(p *bool) string {
		if p == nil {
			return "unknown"
		}
		if *p {
			return "yes"
		}
		return "no"
	}
	s := func(v string) string {
		if v == "" {
			return "unknown"
		}
		return v
	}
	switch field {
	case "leadSource":
		return s(l.LeadSource)
	case "segment":
		return s(l.Segment)
	case "preferredLanguage":
		return s(l.PreferredLanguage)
	case "currentInternet":
		return s(l.CurrentInternet)
	case "currentProvider":
		return s(l.CurrentProvider)
	case "cityTier":
		return f(l.CityTier)
	case "employeeCount":
		return f(l.EmployeeCount)
	case "numSites":
		return f(l.NumSites)
	case "annualTurnoverBand":
		return s(l.AnnualTurnoverBand)
	case "hasGstin":
		return b(l.HasGSTIN)
	case "hasUdyam":
		return b(l.HasUdyam)
	case "monthlyTelecomSpendInr":
		if l.MonthlyTelecomSpendInr == nil {
			return "unknown"
		}
		return fmt.Sprintf("₹%.0f/month", *l.MonthlyTelecomSpendInr)
	case "interestCount", "productsOfInterest":
		if len(l.ProductsOfInterest) == 0 {
			return "none"
		}
		return fmt.Sprint(l.ProductsOfInterest)
	}
	return ""
}
