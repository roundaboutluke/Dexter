package webhook

import "testing"

func TestNormalizeHookCoordinatesFortUpdatePromotesNestedIDAndType(t *testing.T) {
	hook := &Hook{
		Type: "fort_update",
		Message: map[string]any{
			"new": map[string]any{
				"id":   "0b427e88a3254eeab442d425412e4505.16",
				"type": "pokestop",
				"location": map[string]any{
					"lat": 50.982116,
					"lon": 6.933164,
				},
			},
		},
	}

	normalizeHookCoordinates(hook)

	if got := getString(hook.Message["id"]); got != "0b427e88a3254eeab442d425412e4505.16" {
		t.Fatalf("id=%q, want %q", got, "0b427e88a3254eeab442d425412e4505.16")
	}
	if got := getString(hook.Message["fort_type"]); got != "pokestop" {
		t.Fatalf("fort_type=%q, want %q", got, "pokestop")
	}
	if got := getFloat(hook.Message["latitude"]); got == 0 {
		t.Fatalf("latitude was not promoted from nested location")
	}
	if got := getFloat(hook.Message["longitude"]); got == 0 {
		t.Fatalf("longitude was not promoted from nested location")
	}
}
