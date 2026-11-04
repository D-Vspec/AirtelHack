package logx

import "testing"

func TestMaskID(t *testing.T) {
	if got := MaskID("37IHKXV5167L1ZS"); got != "***********L1ZS" {
		t.Fatalf("got %q", got)
	}
	if got := MaskID("abc"); got != "***" {
		t.Fatalf("got %q", got)
	}
}
