package webhook

import (
	"testing"
	"time"

	"poraclego/internal/data"
)

func TestNestAddsResetTimeAndDisappearDate(t *testing.T) {
	p := &Processor{}
	reset := int64(1700000000)
	hook := &Hook{
		Type: "nest",
		Message: map[string]any{
			"latitude":   1.0,
			"longitude":  1.0,
			"reset_time": reset,
		},
	}

	payload := buildRenderData(p, hook, alertMatch{Target: alertTarget{Platform: "discord"}})

	if got := getString(payload["resetTime"]); got != time.Unix(reset, 0).Format("15:04:05") {
		t.Fatalf("resetTime=%q, want %q", got, time.Unix(reset, 0).Format("15:04:05"))
	}
	expire := reset + 7*24*60*60
	if got := getString(payload["disappearDate"]); got != time.Unix(expire, 0).Format("2006-01-02") {
		t.Fatalf("disappearDate=%q, want %q", got, time.Unix(expire, 0).Format("2006-01-02"))
	}
}

func TestFortUpdateAddsChangeTypeEditTypesResetTimeAndDisappearDate(t *testing.T) {
	p := &Processor{}
	reset := int64(1700000000)
	hook := &Hook{
		Type: "fort_update",
		Message: map[string]any{
			"latitude":     1.0,
			"longitude":    1.0,
			"reset_time":   reset,
			"change_type":  "edit",
			"edit_types":   []any{"name"},
			"old":          map[string]any{"name": "", "description": ""},
			"new":          map[string]any{"name": "x", "description": "y", "type": "pokestop"},
			"fort_type":    "pokestop",
			"pokestop_id":  "stop",
			"pokestop_url": "url",
		},
	}

	payload := buildRenderData(p, hook, alertMatch{Target: alertTarget{Platform: "discord"}})

	if got := getString(payload["change_type"]); got != "new" {
		t.Fatalf("change_type=%q, want %q", got, "new")
	}
	if _, ok := payload["edit_types"]; !ok {
		t.Fatalf("edit_types missing, want present (nil)")
	}
	if payload["edit_types"] != nil {
		t.Fatalf("edit_types=%v, want nil", payload["edit_types"])
	}
	if got := getString(payload["resetTime"]); got != time.Unix(reset, 0).Format("15:04:05") {
		t.Fatalf("resetTime=%q, want %q", got, time.Unix(reset, 0).Format("15:04:05"))
	}
	expire := reset + 7*24*60*60
	if got := getString(payload["disappearDate"]); got != time.Unix(expire, 0).Format("2006-01-02") {
		t.Fatalf("disappearDate=%q, want %q", got, time.Unix(expire, 0).Format("2006-01-02"))
	}
}

func TestTemplateTypeForHookFortUpdateUsesFortUpdateTemplate(t *testing.T) {
	hook := &Hook{
		Type:    "fort_update",
		Message: map[string]any{},
	}

	if got := templateTypeForHook(hook); got != "fort-update" {
		t.Fatalf("templateTypeForHook=%q, want %q", got, "fort-update")
	}
}

func TestWeatherAddsConditionAlias(t *testing.T) {
	p := &Processor{}
	hook := &Hook{
		Type: "weather",
		Message: map[string]any{
			"latitude":           1.0,
			"longitude":          1.0,
			"gameplay_condition": 2,
		},
	}

	payload := buildRenderData(p, hook, alertMatch{Target: alertTarget{Platform: "discord"}})
	if got := getInt(payload["condition"]); got != 2 {
		t.Fatalf("condition=%d, want %d", got, 2)
	}
}

func TestWeatherOmitsActivePokemonsWhenAlteredPokemonDisabled(t *testing.T) {
	p := &Processor{}
	hook := &Hook{
		Type: "weather",
		Message: map[string]any{
			"latitude":           1.0,
			"longitude":          1.0,
			"gameplay_condition": 2,
		},
	}

	payload := buildRenderData(p, hook, alertMatch{Target: alertTarget{Platform: "discord", ID: "target"}})
	if _, ok := payload["activePokemons"]; ok {
		t.Fatalf("activePokemons present, want omitted when showAlteredPokemon is disabled")
	}
	if got := getString(payload["id"]); got != "target" {
		t.Fatalf("id=%q, want %q", got, "target")
	}
}

func TestRaidAddsGymNameTeamIDAndEvolutionName(t *testing.T) {
	p := &Processor{
		data: &data.GameData{
			UtilData: map[string]any{
				"evolution": map[string]any{
					"1": map[string]any{"name": "Mega"},
				},
			},
		},
	}
	hook := &Hook{
		Type: "raid",
		Message: map[string]any{
			"latitude":     1.0,
			"longitude":    1.0,
			"pokemon_id":   1,
			"form":         0,
			"team":         2,
			"evolution_id": 1,
		},
	}

	payload := buildRenderData(p, hook, alertMatch{Target: alertTarget{Platform: "discord"}})
	if got := getInt(payload["team_id"]); got != 2 {
		t.Fatalf("team_id=%d, want %d", got, 2)
	}
	if _, ok := payload["gym_name"]; !ok {
		t.Fatalf("gym_name missing, want present")
	}
	if got := getString(payload["evolutionName"]); got != "Mega" {
		t.Fatalf("evolutionName=%q, want %q", got, "Mega")
	}
}
