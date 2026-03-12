package profile

import "testing"

func TestNewIncludesMaxbattleTrackingCategory(t *testing.T) {
	logic := New(nil, "user-1")
	found := false
	for _, category := range logic.categories {
		if category == "maxbattle" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("maxbattle missing from profile categories: %#v", logic.categories)
	}
}
