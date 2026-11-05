package main

import "testing"

func TestMobileNormalisation(t *testing.T) {
	for in, want := range map[string]string{"98765 43210": "9876543210", "+91 98765-43210": "9876543210", "09876543210": "9876543210", "9876543210": "9876543210"} {
		if got := normMobile(in); got != want || !mobileRe.MatchString(got) {
			t.Errorf("normMobile(%q) = %q", in, got)
		}
	}
	for _, bad := range []string{"12345", "5876543210", "98765432101", "abcdefghij"} {
		if mobileRe.MatchString(normMobile(bad)) {
			t.Errorf("%q accepted", bad)
		}
	}
	if m := maskMobile("9876543210"); m != "••••••3210" {
		t.Errorf("mask = %q", m)
	}
	if hash("a") == hash("b") || len(hash("a")) != 64 {
		t.Error("hash")
	}
}
