package main

import "testing"

func TestParseBarOpacity(t *testing.T) {
	for raw, want := range map[string]float64{
		"0":    0,
		"0.68": 0.68,
		"1":    1,
	} {
		got, err := parseBarOpacity(raw)
		if err != nil {
			t.Errorf("parseBarOpacity(%q): %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("parseBarOpacity(%q) = %v; esperado %v", raw, got, want)
		}
	}

	for _, raw := range []string{"", "no", "NaN", "+Inf", "-0.01", "1.01"} {
		if got, err := parseBarOpacity(raw); err == nil {
			t.Errorf("parseBarOpacity(%q) = %v; debía rechazarlo", raw, got)
		}
	}
}
