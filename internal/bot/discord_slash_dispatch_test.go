package bot

import (
	"strings"
	"testing"
)

// TestDispatchTableNoNilHandlers verifies every entry in the exact and prefix
// dispatch tables has a non-nil handler function.
func TestDispatchTableNoNilHandlers(t *testing.T) {
	t.Run("ComponentExact", func(t *testing.T) {
		for key, handler := range slashComponentExactHandlers {
			if handler == nil {
				t.Errorf("slashComponentExactHandlers[%q] has nil handler", key)
			}
		}
	})
	t.Run("ComponentPrefix", func(t *testing.T) {
		for _, entry := range slashComponentPrefixHandlers {
			if entry.handler == nil {
				t.Errorf("slashComponentPrefixHandlers prefix %q has nil handler", entry.prefix)
			}
		}
	})
	t.Run("ModalExact", func(t *testing.T) {
		for key, handler := range slashModalExactHandlers {
			if handler == nil {
				t.Errorf("slashModalExactHandlers[%q] has nil handler", key)
			}
		}
	})
	t.Run("ModalPrefix", func(t *testing.T) {
		for _, entry := range slashModalPrefixHandlers {
			if entry.handler == nil {
				t.Errorf("slashModalPrefixHandlers prefix %q has nil handler", entry.prefix)
			}
		}
	})
}

// TestDispatchTableNoDuplicatePrefixes verifies that no prefix appears more
// than once in the ordered prefix handler slices.
func TestDispatchTableNoDuplicatePrefixes(t *testing.T) {
	t.Run("ComponentPrefix", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, entry := range slashComponentPrefixHandlers {
			if seen[entry.prefix] {
				t.Errorf("duplicate prefix %q in slashComponentPrefixHandlers", entry.prefix)
			}
			seen[entry.prefix] = true
		}
	})
	t.Run("ModalPrefix", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, entry := range slashModalPrefixHandlers {
			if seen[entry.prefix] {
				t.Errorf("duplicate prefix %q in slashModalPrefixHandlers", entry.prefix)
			}
			seen[entry.prefix] = true
		}
	})
}

// TestDispatchTablePrefixOrderingSafe verifies that if prefix A is a substring
// of prefix B, then B comes before A in the slice. This ensures the longer
// (more specific) prefix matches first.
func TestDispatchTablePrefixOrderingSafe(t *testing.T) {
	checkOrdering := func(t *testing.T, name string, prefixes []string) {
		t.Helper()
		for i := 0; i < len(prefixes); i++ {
			for j := i + 1; j < len(prefixes); j++ {
				a := prefixes[i]
				b := prefixes[j]
				// If a is a prefix of b, b should come first (before a), but
				// here a is at index i < j, which means a comes first.
				// That is wrong: the longer string b should come first.
				if a != b && strings.HasPrefix(b, a) {
					t.Errorf("%s: prefix %q (index %d) is a prefix of %q (index %d), but the longer prefix should come first", name, a, i, b, j)
				}
			}
		}
	}

	t.Run("ComponentPrefix", func(t *testing.T) {
		var prefixes []string
		for _, entry := range slashComponentPrefixHandlers {
			prefixes = append(prefixes, entry.prefix)
		}
		checkOrdering(t, "slashComponentPrefixHandlers", prefixes)
	})
	t.Run("ModalPrefix", func(t *testing.T) {
		var prefixes []string
		for _, entry := range slashModalPrefixHandlers {
			prefixes = append(prefixes, entry.prefix)
		}
		checkOrdering(t, "slashModalPrefixHandlers", prefixes)
	})
}

// TestDispatchTableAllConstantsCovered verifies that every customID constant
// declared in the const block of discord_slash.go is covered by either the
// exact/prefix dispatch tables or the stateful switch blocks in
// handleSlashComponent / handleSlashModal.
func TestDispatchTableAllConstantsCovered(t *testing.T) {
	// Build a set of all constants that should be dispatched somewhere.
	allConstants := map[string]string{
		slashTrackTypeSelect:             "slashTrackTypeSelect",
		slashMonsterSearch:               "slashMonsterSearch",
		slashMonsterSelect:               "slashMonsterSelect",
		slashRaidInput:                   "slashRaidInput",
		slashRaidLevelSelect:             "slashRaidLevelSelect",
		slashEggLevelSelect:              "slashEggLevelSelect",
		slashQuestInput:                  "slashQuestInput",
		slashInvasionInput:               "slashInvasionInput",
		slashGymTeamSelect:               "slashGymTeamSelect",
		slashFortTypeSelect:              "slashFortTypeSelect",
		slashWeatherConditionSelect:      "slashWeatherConditionSelect",
		slashLureTypeSelect:              "slashLureTypeSelect",
		slashFiltersModal:                "slashFiltersModal",
		slashConfirmButton:               "slashConfirmButton",
		slashCancelButton:                "slashCancelButton",
		slashChooseEverything:            "slashChooseEverything",
		slashChooseSearch:                "slashChooseSearch",
		slashAreaShowSelect:              "slashAreaShowSelect",
		slashAreaShowAdd:                 "slashAreaShowAdd",
		slashAreaShowRemove:              "slashAreaShowRemove",
		slashProfileSelect:               "slashProfileSelect",
		slashProfileSet:                  "slashProfileSet",
		slashProfileCreate:               "slashProfileCreate",
		slashProfileCreateMod:            "slashProfileCreateMod",
		slashProfileScheduleAdd:          "slashProfileScheduleAdd",
		slashProfileScheduleOverview:     "slashProfileScheduleOverview",
		slashProfileScheduleAddGlobal:    "slashProfileScheduleAddGlobal",
		slashProfileScheduleDay:          "slashProfileScheduleDay",
		slashProfileScheduleDayGlobal:    "slashProfileScheduleDayGlobal",
		slashProfileScheduleTime:         "slashProfileScheduleTime",
		slashProfileScheduleAssign:       "slashProfileScheduleAssign",
		slashProfileScheduleEditGlobal:   "slashProfileScheduleEditGlobal",
		slashProfileScheduleEditDay:      "slashProfileScheduleEditDay",
		slashProfileScheduleEditTime:     "slashProfileScheduleEditTime",
		slashProfileScheduleEditAssign:   "slashProfileScheduleEditAssign",
		slashProfileScheduleBack:         "slashProfileScheduleBack",
		slashProfileScheduleClear:        "slashProfileScheduleClear",
		slashProfileScheduleRemove:       "slashProfileScheduleRemove",
		slashProfileScheduleRemoveGlobal: "slashProfileScheduleRemoveGlobal",
		slashProfileScheduleToggle:       "slashProfileScheduleToggle",
		slashProfileLocation:             "slashProfileLocation",
		slashProfileLocationClear:        "slashProfileLocationClear",
		slashProfileLocationMod:          "slashProfileLocationMod",
		slashProfileArea:                 "slashProfileArea",
		slashProfileAreaBack:             "slashProfileAreaBack",
		slashProfileBack:                 "slashProfileBack",
		slashProfileDelete:               "slashProfileDelete",
		slashProfileDeleteConfirm:        "slashProfileDeleteConfirm",
		slashProfileDeleteCancel:         "slashProfileDeleteCancel",
		slashInfoCancelButton:            "slashInfoCancelButton",
		slashInfoTypeSelect:              "slashInfoTypeSelect",
		slashInfoTranslateModal:          "slashInfoTranslateModal",
		slashInfoWeatherModal:            "slashInfoWeatherModal",
		slashInfoWeatherUseSaved:         "slashInfoWeatherUseSaved",
		slashInfoWeatherEnterCoordinates: "slashInfoWeatherEnterCoordinates",
	}

	// Collect all keys/prefixes from the dispatch tables.
	covered := make(map[string]bool)

	for key := range slashComponentExactHandlers {
		covered[key] = true
	}
	for _, entry := range slashComponentPrefixHandlers {
		covered[entry.prefix] = true
	}
	for key := range slashModalExactHandlers {
		covered[key] = true
	}
	for _, entry := range slashModalPrefixHandlers {
		covered[entry.prefix] = true
	}

	// Constants handled in the stateful switch block of handleSlashComponent.
	componentStatefulCases := []string{
		slashTrackTypeSelect,
		slashChooseEverything,
		slashChooseSearch,
		slashMonsterSelect,
		slashRaidLevelSelect,
		slashEggLevelSelect,
		slashGymTeamSelect,
		slashFortTypeSelect,
		slashWeatherConditionSelect,
		slashLureTypeSelect,
		slashConfirmButton,
		slashCancelButton,
		slashFiltersModal,
	}
	for _, c := range componentStatefulCases {
		covered[c] = true
	}

	// Constants handled in the stateful switch block of handleSlashModal.
	modalStatefulCases := []string{
		slashMonsterSearch,
		slashRaidInput,
		slashQuestInput,
		slashInvasionInput,
		slashFiltersModal,
	}
	for _, c := range modalStatefulCases {
		covered[c] = true
	}

	for value, name := range allConstants {
		t.Run(name, func(t *testing.T) {
			if covered[value] {
				return
			}
			// Check if this constant is covered as a prefix match (its value
			// starts with one of the registered prefixes).
			for prefix := range covered {
				if strings.HasPrefix(value, prefix) && prefix != value {
					return
				}
			}
			t.Errorf("constant %s (%q) is not covered by any dispatch table or stateful switch case", name, value)
		})
	}
}
