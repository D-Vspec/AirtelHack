package main

import (
	"regexp"
	"testing"
)

func TestEveryTemplateHasEnglishHindiTamil(t *testing.T) {
	for key, langs := range templates {
		for _, l := range []string{"en", "hi", "ta"} {
			if langs[l] == "" {
				t.Errorf("%s missing %s", key, l)
			}
		}
		// the same placeholders in every language
		ph := regexp.MustCompile(`\{[a-z]+\}`)
		want := len(ph.FindAllString(langs["en"], -1))
		for l, text := range langs {
			if got := len(ph.FindAllString(text, -1)); got != want {
				t.Errorf("%s/%s has %d placeholders, English has %d", key, l, got, want)
			}
		}
	}
}

func TestRender(t *testing.T) {
	body, lang, ok := render("order.status.kyc_passed", "hi", map[string]string{"order": "AB12CD34", "date": "2026-08-05"})
	if !ok || lang != "hi" || body != "ऑर्डर AB12CD34: KYC सत्यापित हो गया। 2026-08-05 तक चालू होने की उम्मीद है।" {
		t.Errorf("got %q %s", body, lang)
	}
	if _, lang, _ := render("order.status.activated", "gu", map[string]string{"order": "X"}); lang != "en" {
		t.Errorf("Gujarati should fall back to English, got %s", lang)
	}
	if _, _, ok := render("no.such.template", "en", nil); ok {
		t.Error("unknown key should fail")
	}
}
