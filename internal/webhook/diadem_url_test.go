package webhook

import "testing"

func TestDiademURL_EmptyBaseReturnsEmpty(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{
		"general": map[string]any{
			"diademURL": "",
		},
	})
	hook := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "123"}}
	if got := diademURL(cfg, hook); got != "" {
		t.Fatalf("expected empty diadem URL, got %q", got)
	}
}

func TestDiademURL_NormalizesTrailingSlash(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{
		"general": map[string]any{
			"diademURL": "https://map.example.com",
		},
	})
	hook := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "123"}}
	if got := diademURL(cfg, hook); got != "https://map.example.com/pokemon/123" {
		t.Fatalf("unexpected diadem URL: %q", got)
	}
}

func TestDiademURL_Pokemon(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{"general": map[string]any{"diademURL": "https://map/"}})
	hook := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "e"}}
	if got := diademURL(cfg, hook); got != "https://map/pokemon/e" {
		t.Fatalf("unexpected diadem URL: %q", got)
	}
}

func TestDiademURL_RaidAndEggUseGym(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{"general": map[string]any{"diademURL": "https://map/"}})
	for _, hookType := range []string{"raid", "egg", "gym", "gym_details"} {
		hook := &Hook{Type: hookType, Message: map[string]any{"gym_id": "g"}}
		if got := diademURL(cfg, hook); got != "https://map/gym/g" {
			t.Fatalf("type %s: unexpected diadem URL: %q", hookType, got)
		}
	}
}

func TestDiademURL_QuestStopAndStation(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{"general": map[string]any{"diademURL": "https://map/"}})

	hook := &Hook{Type: "quest", Message: map[string]any{"pokestop_id": "s"}}
	if got := diademURL(cfg, hook); got != "https://map/pokestop/s" {
		t.Fatalf("unexpected diadem URL: %q", got)
	}

	hook = &Hook{Type: "max_battle", Message: map[string]any{"id": "st"}}
	if got := diademURL(cfg, hook); got != "https://map/station/st" {
		t.Fatalf("unexpected diadem URL: %q", got)
	}
}
