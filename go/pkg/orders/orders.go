// Package orders is the order lifecycle state machine (contracts/orders.schema.json).
package orders

import (
	"slices"
	"strings"
	"time"
)

// Statuses.
const (
	QuoteAccepted    = "quote_accepted"
	KycPending       = "kyc_pending"
	KycPassed        = "kyc_passed"
	InstallScheduled = "install_scheduled"
	Installing       = "installing"
	Activated        = "activated"
	Delayed          = "delayed"
	Failed           = "failed"
)

// Reasons with special handling.
const (
	ReasonFibreNotFeasible = "fibre_not_feasible"
	ReasonKycRejected      = "kyc_rejected"
	ReasonFallbackAccepted = "fallback_5g_accepted"
)

// HappyPath is the order a progress bar shows.
var HappyPath = []string{QuoteAccepted, KycPending, KycPassed, InstallScheduled, Installing, Activated}

// Allowed lists the legal moves. activated and failed are final. quote_accepted may jump to
// kyc_passed when the owner finished KYC before accepting the quote.
var Allowed = map[string][]string{
	QuoteAccepted:    {KycPending, KycPassed, Failed},
	KycPending:       {KycPassed, Failed},
	KycPassed:        {InstallScheduled, Delayed, Failed},
	InstallScheduled: {Installing, Delayed, Failed},
	Installing:       {Activated, Delayed, Failed},
	Delayed:          {InstallScheduled, Installing, Failed},
}

// CanMove reports whether from -> to is a legal transition.
func CanMove(from, to string) bool { return slices.Contains(Allowed[from], to) }

// CanMoveFrom is CanMove plus the one way out of failed: an order that failed only because KYC
// rejected it reopens at kyc_passed when the owner later passes KYC (for example after uploading
// the right documents, or a restored GST registration). Every other failure stays final.
func CanMoveFrom(from, reason, to string) bool {
	return CanMove(from, to) || (from == Failed && reason == ReasonKycRejected && to == KycPassed)
}

// Valid reports whether s is a known status.
func Valid(s string) bool { return s == Activated || s == Failed || Allowed[s] != nil }

// Final reports whether no further moves are possible.
func Final(s string) bool { return s == Activated || s == Failed }

// Next is the next happy-path status, or "" at the end (used by the demo simulator).
func Next(s string) string {
	i := slices.Index(HappyPath, s)
	if i < 0 || i == len(HappyPath)-1 {
		return ""
	}
	return HappyPath[i+1]
}

// Fibre products and their 5G fallback (data/product_catalogue.csv: P01-P03 Biz Fibre,
// P04 Biz 5G AirFiber 100).
var (
	FibreProducts   = []string{"P01", "P02", "P03"}
	FallbackProduct = "P04"
)

// FibreIn returns the fibre products in an order.
func FibreIn(productIDs []string) []string {
	var out []string
	for _, p := range productIDs {
		if slices.Contains(FibreProducts, p) {
			out = append(out, p)
		}
	}
	return out
}

// ExpectedActivation is start plus the slowest product's install days, as a date.
func ExpectedActivation(start time.Time, installDays int) string {
	return start.UTC().AddDate(0, 0, installDays).Format("2006-01-02")
}

// Entry is one timeline row.
type Entry struct {
	Status string    `json:"status"`
	Reason string    `json:"reason,omitempty"`
	Note   string    `json:"note,omitempty"`
	At     time.Time `json:"at"`
}

// Step is one happy-path stage for a progress bar.
type Step struct {
	Status string     `json:"status"`
	State  string     `json:"state"` // done | current | upcoming | blocked
	At     *time.Time `json:"at,omitempty"`
}

// Steps maps the timeline onto the happy path. A delayed order shows its last happy-path
// status as blocked; a failed order shows the step it failed at as blocked.
func Steps(current string, entries []Entry) []Step {
	reached := map[string]time.Time{}
	last := ""
	for _, e := range entries {
		if slices.Contains(HappyPath, e.Status) {
			reached[e.Status] = e.At
			last = e.Status
		}
	}
	if slices.Contains(HappyPath, current) {
		last = current
	}
	li := slices.Index(HappyPath, last)
	steps := make([]Step, len(HappyPath))
	for i, s := range HappyPath {
		st := Step{Status: s, State: "upcoming"}
		if at, ok := reached[s]; ok {
			at := at
			st.At = &at
		}
		switch {
		case i < li:
			st.State = "done"
		case i == li && current == s:
			st.State = "current"
			if s == Activated {
				st.State = "done"
			}
		case i == li:
			st.State = "done"
		case i == li+1 && (current == Delayed || current == Failed):
			st.State = "blocked"
		}
		steps[i] = st
	}
	return steps
}

// ShortID is a readable order reference for messages.
func ShortID(id string) string {
	return strings.ToUpper(strings.ReplaceAll(id, "-", "")[:8])
}
