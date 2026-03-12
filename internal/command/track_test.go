package command

import (
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/data"
)

func TestBuildTrackMonstersIncludeAllFormsKeepsDistinctForms(t *testing.T) {
	ctx := &Context{
		Data: &data.GameData{
			Monsters: map[string]any{
				"25": map[string]any{
					"id":   25,
					"name": "Pikachu",
					"form": map[string]any{"id": 0, "name": "Normal"},
				},
				"25_61": map[string]any{
					"id":   25,
					"name": "Pikachu",
					"form": map[string]any{"id": 61, "name": "Pop Star"},
				},
			},
		},
	}

	got := buildTrackMonsters(ctx, []int{25}, nil, true)
	if len(got) != 2 {
		t.Fatalf("len(buildTrackMonsters)=%d, want 2", len(got))
	}
	if got[0].ID != 25 || got[1].ID != 25 {
		t.Fatalf("unexpected IDs: %#v", got)
	}
}

func TestEverythingModeAndTrackDefaults(t *testing.T) {
	ctx := &Context{
		Config: config.New(map[string]any{
			"tracking": map[string]any{
				"everythingFlagPermissions":   "allow-and-ignore-individually",
				"defaultUserTrackingLevelCap": 50,
			},
			"pvp": map[string]any{
				"pvpFilterMaxRank":     200,
				"pvpFilterGreatMinCP":  1000,
				"pvpFilterUltraMinCP":  2000,
				"pvpFilterLittleMinCP": 300,
			},
		}),
	}

	mode := everythingMode(ctx)
	if !mode.ignoreIndividually || mode.individuallyAllowed {
		t.Fatalf("unexpected everything mode: %#v", mode)
	}

	opt := trackOptions{
		MinIV:     -5,
		MaxIV:     101,
		PvpLeague: 1500,
		PvpBest:   250,
		PvpWorst:  500,
		PvpMinCP:  0,
	}
	applyTrackDefaults(ctx, &opt)

	if opt.MinIV != -1 || opt.MaxIV != 100 {
		t.Fatalf("unexpected iv range: %#v", opt)
	}
	if opt.PvpBest != 200 || opt.PvpWorst != 250 {
		t.Fatalf("unexpected pvp rank normalization: %#v", opt)
	}
	if opt.PvpMinCP != 1000 || opt.PvpCap != 50 {
		t.Fatalf("unexpected pvp defaults: %#v", opt)
	}
}
