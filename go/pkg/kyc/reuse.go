package kyc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"udyam/pkg/events"
)

// ReusePolicy decides when a returning customer's earlier verification covers a new order, so
// they are not asked for the same four documents again. Deterministic: the owner's declaration,
// the stored verified facts, today's date and a live registry re-check. Anything that changed
// sends the owner back through the full pipeline.
type ReusePolicy struct {
	MaxAgeDays int // how long a verification stays reusable
}

func DefaultReusePolicy() ReusePolicy { return ReusePolicy{MaxAgeDays: 365} }

// Reuse reason codes (owner-facing text lives in the clients).
const (
	ReuseNoVerification  = "NO_VERIFICATION" // no passed verification on the account
	ReuseOwnerChanged    = "OWNER_CHANGED"   // owner said business details changed
	ReuseTooOld          = "VERIFICATION_EXPIRED"
	ReuseIDExpired       = "SIGNATORY_ID_EXPIRED"
	ReuseGstinNotActive  = "GSTIN_NOT_ACTIVE"
	ReuseRegistryChanged = "REGISTRY_DETAILS_CHANGED" // legal name or address differs from what we verified
	ReuseUdyamNotFound   = "UDYAM_NOT_FOUND"
	ReuseRegistryDown    = "REGISTRY_UNAVAILABLE"
)

// ReuseDecision is the answer: Reusable, or the reasons it is not.
type ReuseDecision struct {
	Reusable bool     `json:"reusable"`
	Reasons  []string `json:"reasons"`
}

// Decide checks a stored passed result (verifiedAt = when it passed) for reuse on a new order.
func (p ReusePolicy) Decide(ctx context.Context, reg Registry, prev *Result, verifiedAt time.Time, ownerSaysUnchanged bool, now time.Time) ReuseDecision {
	var reasons []string
	add := func(code string) { reasons = append(reasons, code) }
	if prev == nil {
		return ReuseDecision{Reasons: []string{ReuseNoVerification}}
	}
	if !ownerSaysUnchanged {
		add(ReuseOwnerChanged)
	}
	if now.Sub(verifiedAt) > time.Duration(p.MaxAgeDays)*24*time.Hour {
		add(ReuseTooOld)
	}
	if sig, ok := prev.Documents[DocSignatory]; ok {
		if v := sig.Extracted.Fields[FValidTill]; v != "" {
			if d, err := time.Parse("2006-01-02", v); err == nil && d.Before(now.Truncate(24*time.Hour)) {
				add(ReuseIDExpired)
			}
		}
	}
	if g := prev.Verified.GSTIN; g != "" {
		m, err := reg.GSTIN(ctx, g)
		switch {
		case err != nil:
			add(ReuseRegistryDown)
		case m.Record == nil || m.Record.Status != "Active":
			add(ReuseGstinNotActive)
		case prev.Verified.GSTINRecord != nil && (!sameText(m.Record.LegalName, prev.Verified.GSTINRecord.LegalName) || !sameText(m.Record.Address, prev.Verified.GSTINRecord.Address)):
			add(ReuseRegistryChanged)
		}
	}
	if u := prev.Verified.UdyamNumber; u != "" {
		m, err := reg.Udyam(ctx, u)
		switch {
		case err != nil:
			if !contains(reasons, ReuseRegistryDown) {
				add(ReuseRegistryDown)
			}
		case m.Record == nil:
			add(ReuseUdyamNotFound)
		}
	}
	return ReuseDecision{Reusable: len(reasons) == 0, Reasons: nonNil(reasons)}
}

// Reused is the result stored for the new order: the earlier verified facts, marked as reused.
func Reused(prev Result, now time.Time) Result {
	out := prev
	out.Route = events.RoutePass
	out.Findings = []Finding{}
	out.CheckedAt = now
	return out
}

func sameText(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}

// String is for logs.
func (d ReuseDecision) String() string {
	return fmt.Sprintf("reusable=%v reasons=%v", d.Reusable, d.Reasons)
}
