package command

import (
	"slices"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
)

func TestParseProfileTimeTokensSupportsSwitchesAndRanges(t *testing.T) {
	factory := i18n.NewFactory("/Users/pbx/PoracleJS/PoracleGo", config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"en": true},
		},
	}))

	legacy, ranges := parseProfileTimeTokens(factory, []string{"mon0900", "weekday:18:00-23:00"})
	if len(legacy) != 1 {
		t.Fatalf("legacy count=%d, want 1", len(legacy))
	}
	if got := toInt(legacy[0]["day"], 0); got != 1 {
		t.Fatalf("legacy day=%d, want 1", got)
	}
	if len(ranges) != 5 {
		t.Fatalf("range count=%d, want 5", len(ranges))
	}
}

func TestParseClockMinutesFlexible(t *testing.T) {
	cases := []struct {
		input string
		want  int
		ok    bool
	}{
		{input: "9", want: 9 * 60, ok: true},
		{input: "930", want: 9*60 + 30, ok: true},
		{input: "09:45", want: 9*60 + 45, ok: true},
		{input: "2400", ok: false},
	}
	for _, tc := range cases {
		got, ok := parseClockMinutesFlexible(tc.input)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("parseClockMinutesFlexible(%q)=(%d,%v), want (%d,%v)", tc.input, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFormatProfileTimesAndParseScheduleRanges(t *testing.T) {
	tr := testCommandTranslator(t)
	raw := `[{"day":1,"hours":9,"mins":0},{"day":2,"start_hours":18,"start_mins":0,"end_hours":23,"end_mins":0}]`

	got := formatProfileTimes(tr, raw)
	want := []string{
		"    Monday 9:00 (switch)",
		"    Tuesday 18:00-23:00",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("formatProfileTimes()=%v, want %v", got, want)
	}

	ranges := parseScheduleRangesFromRaw(raw)
	if len(ranges) != 1 {
		t.Fatalf("range len=%d, want 1", len(ranges))
	}
	if ranges[0].Day != 2 || ranges[0].StartMin != 18*60 || ranges[0].EndMin != 23*60 {
		t.Fatalf("unexpected parsed range: %#v", ranges[0])
	}
}

func TestFormatScheduleRangeLabelLocalized(t *testing.T) {
	root := "/Users/pbx/PoracleJS/PoracleGo"
	tr, err := i18n.NewTranslator(root, "fr")
	if err != nil {
		t.Fatalf("new translator: %v", err)
	}

	if got := formatScheduleRangeLabel(tr, 1, 9*60, 10*60); got != "Lundi 09:00-10:00" {
		t.Fatalf("formatScheduleRangeLabel()=%q, want %q", got, "Lundi 09:00-10:00")
	}
}

func testCommandTranslator(t *testing.T) *i18n.Translator {
	t.Helper()
	root := "/Users/pbx/PoracleJS/PoracleGo"
	tr, err := i18n.NewTranslator(root, "en")
	if err != nil {
		t.Fatalf("new translator: %v", err)
	}
	return tr
}
