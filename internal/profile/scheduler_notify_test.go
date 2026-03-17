package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/dispatch"
	"poraclego/internal/i18n"
)

func TestNotifySwitchLocalizesMessage(t *testing.T) {
	s := testSchedulerWithLocales(t, map[string]string{
		"I have set your profile to: {0}": "Profil defini sur : {0}",
	})

	s.notifySwitch(map[string]any{
		"type":     "discord",
		"id":       "user-1",
		"name":     "Tester",
		"language": "fr",
	}, map[string]any{
		"name": "Maison",
	})

	jobs := s.discordQueue.Drain()
	if len(jobs) != 1 {
		t.Fatalf("jobs=%d, want 1", len(jobs))
	}
	if jobs[0].Message != "Profil defini sur : Maison" {
		t.Fatalf("message=%q, want %q", jobs[0].Message, "Profil defini sur : Maison")
	}
}

func TestNotifyQuietLocalizesPausedMessage(t *testing.T) {
	s := testSchedulerWithLocales(t, map[string]string{
		"Quiet hours enabled. Alerts paused. Adjust schedules with /profile.": "Heures calmes activees. Alertes en pause. Ajustez les horaires avec /profile.",
	})

	s.notifyQuiet(map[string]any{
		"type":     "discord",
		"id":       "user-1",
		"name":     "Tester",
		"language": "fr",
	}, nil)

	jobs := s.discordQueue.Drain()
	if len(jobs) != 1 {
		t.Fatalf("jobs=%d, want 1", len(jobs))
	}
	if jobs[0].Message != "Heures calmes activees. Alertes en pause. Ajustez les horaires avec /profile." {
		t.Fatalf("message=%q, want translated quiet-hours text", jobs[0].Message)
	}
}

func testSchedulerWithLocales(t *testing.T, entries map[string]string) *Scheduler {
	t.Helper()
	root := t.TempDir()
	localeDir := filepath.Join(root, "locale")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal locale: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, "fr.json"), data, 0o644); err != nil {
		t.Fatalf("write locale file: %v", err)
	}

	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	return &Scheduler{
		cfg:          cfg,
		i18n:         i18n.NewFactory(root, cfg),
		discordQueue: dispatch.NewQueue("discord"),
	}
}
