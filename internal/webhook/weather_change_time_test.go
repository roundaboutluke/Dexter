package webhook

import "testing"

func TestTrimWeatherChangeTime_MirrorsPoracleJS(t *testing.T) {
	if got := trimWeatherChangeTime("19:00:00"); got != "19:00" {
		t.Fatalf("trimWeatherChangeTime=%q, want %q", got, "19:00")
	}
	if got := trimWeatherChangeTime("09:00"); got != "09" {
		t.Fatalf("trimWeatherChangeTime=%q, want %q", got, "09")
	}
	if got := trimWeatherChangeTime(""); got != "" {
		t.Fatalf("trimWeatherChangeTime=%q, want empty string", got)
	}
}
