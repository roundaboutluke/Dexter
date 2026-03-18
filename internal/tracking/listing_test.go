package tracking

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/geofence"
	"dexter/internal/i18n"
)

type stubRowSource struct {
	tables map[string][]map[string]any
}

func (s stubRowSource) SelectAllQuery(table string, where map[string]any) ([]map[string]any, error) {
	rows := s.tables[table]
	filtered := []map[string]any{}
	for _, row := range rows {
		if !matchesWhere(row, where) {
			continue
		}
		filtered = append(filtered, CloneRow(row))
	}
	return filtered, nil
}

func (s stubRowSource) CountGroupedQuery(table string, conditions map[string]any, groupBy string) (map[int]int64, error) {
	rows, err := s.SelectAllQuery(table, conditions)
	if err != nil {
		return nil, err
	}
	result := map[int]int64{}
	for _, row := range rows {
		key := 0
		if v, ok := row[groupBy]; ok {
			switch n := v.(type) {
			case int:
				key = n
			case int64:
				key = int(n)
			case float64:
				key = int(n)
			}
		}
		result[key]++
	}
	return result, nil
}

func matchesWhere(row, where map[string]any) bool {
	for key, want := range where {
		if fmt.Sprintf("%v", row[key]) != fmt.Sprintf("%v", want) {
			return false
		}
	}
	return true
}

func TestAreaTextAndRowFieldParsing(t *testing.T) {
	human := map[string]any{
		"area":           `["london"]`,
		"blocked_alerts": `["monster","quest"]`,
	}

	areas := ParseAreaList(human)
	if len(areas) != 1 || areas[0] != "london" {
		t.Fatalf("areas=%v, want [london]", areas)
	}

	blocked := BlockedAlerts(human)
	if len(blocked) != 2 || blocked[0] != "monster" || blocked[1] != "quest" {
		t.Fatalf("blocked=%v, want [monster quest]", blocked)
	}

	fences := []geofence.Fence{{Name: "London"}}
	if got := AreaText(nil, fences, areas); got != "You are currently set to receive alarms in London" {
		t.Fatalf("AreaText()=%q", got)
	}
	if got := AreaText(nil, nil, []string{"custom"}); got != "You have not selected any area yet" {
		t.Fatalf("AreaText() missing-fence=%q", got)
	}
	if got := AreaTextWithFallback(nil, nil, []string{"custom"}); got != "You are currently set to receive alarms in custom" {
		t.Fatalf("AreaTextWithFallback()=%q", got)
	}
}

func TestCategoryDetailsUsesSharedMonsterListing(t *testing.T) {
	tr := testTranslator(t)
	ctx := ListingContext{
		Config: config.New(map[string]any{
			"general": map[string]any{
				"defaultTemplateName": "1",
			},
		}),
		Query: stubRowSource{
			tables: map[string][]map[string]any{
				"monsters": {
					{
						"id":         "user-1",
						"profile_no": 1,
						"pokemon_id": 25,
						"form":       0,
						"distance":   100,
						"template":   "1",
						"clean":      0,
						"min_iv":     0,
						"max_iv":     100,
						"min_cp":     10,
						"max_cp":     5000,
						"min_level":  1,
						"max_level":  35,
						"atk":        0,
						"def":        0,
						"sta":        0,
						"max_atk":    15,
						"max_def":    15,
						"max_sta":    15,
						"size":       1,
						"max_size":   5,
						"rarity":     1,
						"max_rarity": 5,
					},
				},
			},
		},
		Data: testGameData(),
	}

	got := CategoryDetails(ctx, tr, "user-1", 1, []string{"pvp"})
	if !strings.Contains(got, "You're tracking the following monsters:") {
		t.Fatalf("CategoryDetails() missing heading: %q", got)
	}
	if !strings.Contains(got, "**Pikachu**") {
		t.Fatalf("CategoryDetails() missing monster row: %q", got)
	}
	if !strings.Contains(got, "PVP tracking") {
		t.Fatalf("CategoryDetails() missing pvp warning: %q", got)
	}
}

func TestProfileCountsSkipDisabledAndBlockedCategories(t *testing.T) {
	ctx := ListingContext{
		Config: config.New(map[string]any{
			"general": map[string]any{
				"disableRaid": true,
			},
		}),
		Query: stubRowSource{
			tables: map[string][]map[string]any{
				"monsters": {
					{"id": "user-1", "profile_no": 1},
				},
				"raid": {
					{"id": "user-1", "profile_no": 2},
				},
				"quest": {
					{"id": "user-1", "profile_no": 2},
				},
			},
		},
	}

	got := ProfileCounts(ctx, "user-1", []string{"quest"})
	if got[1]["monsters"] != 1 {
		t.Fatalf("monster count=%v, want 1", got)
	}
	if _, ok := got[2]; ok {
		t.Fatalf("unexpected disabled/blocked counts: %v", got[2])
	}
}

func testTranslator(t *testing.T) *i18n.Translator {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Dir(filepath.Dir(wd))
	tr, err := i18n.NewTranslator(root, "en")
	if err != nil {
		t.Fatalf("translator: %v", err)
	}
	return tr
}

func testGameData() *data.GameData {
	return &data.GameData{
		Monsters: map[string]any{
			"25": map[string]any{
				"id":   25,
				"name": "Pikachu",
				"form": map[string]any{
					"id":   0,
					"name": "Normal",
				},
			},
		},
		UtilData: map[string]any{
			"size": []any{"", "Tiny", "Small", "Normal", "Large", "Huge", "XXL"},
			"rarity": []any{
				"",
				"Common",
				"Uncommon",
				"Rare",
				"Epic",
				"Legendary",
				"Mythic",
			},
			"genders": []any{
				map[string]any{"emoji": ""},
				map[string]any{"emoji": "♂"},
				map[string]any{"emoji": "♀"},
			},
		},
	}
}
