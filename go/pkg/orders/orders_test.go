package orders

import (
	"testing"
	"time"
)

func TestTransitions(t *testing.T) {
	ok := [][2]string{{QuoteAccepted, KycPending}, {QuoteAccepted, KycPassed}, {KycPending, KycPassed}, {KycPassed, InstallScheduled},
		{InstallScheduled, Installing}, {Installing, Activated}, {KycPassed, Delayed}, {Delayed, InstallScheduled}, {Installing, Failed}}
	bad := [][2]string{{QuoteAccepted, Activated}, {KycPending, Installing}, {Activated, Installing}, {Failed, KycPending}, {Installing, KycPassed}, {Delayed, Activated}}
	for _, c := range ok {
		if !CanMove(c[0], c[1]) {
			t.Errorf("%s -> %s should be allowed", c[0], c[1])
		}
	}
	for _, c := range bad {
		if CanMove(c[0], c[1]) {
			t.Errorf("%s -> %s should be refused", c[0], c[1])
		}
	}
	for _, s := range append(HappyPath, Delayed, Failed) {
		if !Valid(s) {
			t.Errorf("%s should be valid", s)
		}
	}
	if Valid("shipped") || !Final(Activated) || !Final(Failed) || Final(Delayed) {
		t.Error("Valid/Final wrong")
	}
	if Next(KycPassed) != InstallScheduled || Next(Activated) != "" || Next(Delayed) != "" {
		t.Error("Next wrong")
	}
}

func TestExpectedActivationUsesSlowestProduct(t *testing.T) {
	start := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	if got := ExpectedActivation(start, 21); got != "2026-08-05" {
		t.Errorf("got %s", got)
	}
}

func TestFibreIn(t *testing.T) {
	if got := FibreIn([]string{"P02", "P16", "P09"}); len(got) != 1 || got[0] != "P02" {
		t.Errorf("got %v", got)
	}
}

func TestSteps(t *testing.T) {
	t0 := time.Now()
	entries := []Entry{{Status: QuoteAccepted, At: t0}, {Status: KycPending, At: t0}, {Status: KycPassed, At: t0}, {Status: Delayed, Reason: ReasonFibreNotFeasible, At: t0}}
	states := func(ss []Step) []string {
		out := []string{}
		for _, s := range ss {
			out = append(out, s.State)
		}
		return out
	}
	want := []string{"done", "done", "done", "blocked", "upcoming", "upcoming"}
	if got := states(Steps(Delayed, entries)); !equal(got, want) {
		t.Errorf("delayed: %v", got)
	}
	want = []string{"done", "done", "current", "upcoming", "upcoming", "upcoming"}
	if got := states(Steps(KycPassed, entries[:3])); !equal(got, want) {
		t.Errorf("kyc_passed: %v", got)
	}
	all := append(entries[:3:3], Entry{Status: InstallScheduled, At: t0}, Entry{Status: Installing, At: t0}, Entry{Status: Activated, At: t0})
	for _, s := range Steps(Activated, all) {
		if s.State != "done" || s.At == nil {
			t.Errorf("activated: %+v", s)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
