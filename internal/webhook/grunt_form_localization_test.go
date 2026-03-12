package webhook

import (
	"os"
	"path/filepath"
	"testing"

	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

func TestGruntRewardListsTreatNormalFormAsEmptyBeforeTranslation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "util", "locale"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Force a translation that would break "Normal" checks if we compared the translated value.
	if err := os.WriteFile(
		filepath.Join(root, "util", "locale", "xx.json"),
		[]byte(`{"Normal":"Normale","Testmon":"Translated"}`),
		0o644,
	); err != nil {
		t.Fatalf("write locale: %v", err)
	}
	tr, err := i18n.NewTranslator(root, "xx")
	if err != nil {
		t.Fatalf("translator: %v", err)
	}

	p := &Processor{
		data: &data.GameData{
			Monsters: map[string]any{
				"1_0": map[string]any{
					"name": "Testmon",
					"form": map[string]any{"name": "Normal"},
				},
			},
		},
	}

	rewards := rewardListFromEncountersDetailed(p, []any{map[string]any{"id": 1, "form": 0}}, tr)
	if len(rewards) != 1 {
		t.Fatalf("rewards=%d, want 1", len(rewards))
	}
	if got := rewards[0]["fullName"]; got != "Translated" {
		t.Fatalf("reward fullName=%v, want %q", got, "Translated")
	}

	lineup := buildGruntLineupList(p, []any{map[string]any{"pokemon_id": 1, "form": 0}}, tr)
	monsters, _ := lineup["monsters"].([]map[string]any)
	if len(monsters) != 1 {
		t.Fatalf("lineup monsters=%d, want 1", len(monsters))
	}
	if got := monsters[0]["fullName"]; got != "Translated" {
		t.Fatalf("lineup fullName=%v, want %q", got, "Translated")
	}
}
