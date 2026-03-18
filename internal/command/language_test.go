package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/db"
	"dexter/internal/i18n"
)

func TestLanguageCommandListsDiscoveredLanguagesWhenConfigEmpty(t *testing.T) {
	env := newLanguageTestEnv(t, map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}, []string{"en", "de", "fr"})

	reply, err := (&LanguageCommand{}).Handle(env.ctx, nil)
	if err != nil {
		t.Fatalf("handle language: %v", err)
	}
	if !strings.Contains(reply, "de, en, fr") {
		t.Fatalf("reply=%q, want discovered language list", reply)
	}
}

func TestLanguageCommandAcceptsDiscoveredLanguageWhenConfigEmpty(t *testing.T) {
	env := newLanguageTestEnv(t, map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}, []string{"en", "de"})

	reply, err := (&LanguageCommand{}).Handle(env.ctx, []string{"de"})
	if err != nil {
		t.Fatalf("handle language: %v", err)
	}
	if !strings.Contains(strings.ToLower(reply), "german") && !strings.Contains(strings.ToLower(reply), "de") {
		t.Fatalf("reply=%q, want language change confirmation", reply)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := strings.ToLower(strings.TrimSpace(row["language"].(string))); got != "de" {
		t.Fatalf("language=%q, want %q", got, "de")
	}
}

func TestLanguageCommandRejectsLanguagesOutsideExplicitAllowlist(t *testing.T) {
	env := newLanguageTestEnv(t, map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"en": true},
		},
	}, []string{"en", "de"})

	reply, err := (&LanguageCommand{}).Handle(env.ctx, []string{"de"})
	if err != nil {
		t.Fatalf("handle language: %v", err)
	}
	if !strings.Contains(reply, "en") || strings.Contains(reply, "de") {
		t.Fatalf("reply=%q, want explicit allowlist rejection", reply)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("select human: %v", err)
	}
	if got := strings.ToLower(strings.TrimSpace(row["language"].(string))); got != "en" {
		t.Fatalf("language=%q, want %q", got, "en")
	}
}

func TestLanguageCommandSkipsUtilOnlyLocalesWhenConfigEmpty(t *testing.T) {
	env := newLanguageTestEnvWithUtilLocales(t, map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}, []string{"en", "de"}, []string{"fr"})

	reply, err := (&LanguageCommand{}).Handle(env.ctx, nil)
	if err != nil {
		t.Fatalf("handle language: %v", err)
	}
	if !strings.Contains(reply, "de, en") {
		t.Fatalf("reply=%q, want discovered runtime language list", reply)
	}
	if strings.Contains(reply, "fr") {
		t.Fatalf("reply=%q, did not expect util-only locale", reply)
	}
}

type languageTestEnv struct {
	ctx   *Context
	query *db.Query
}

func newLanguageTestEnv(t *testing.T, cfgData map[string]any, locales []string) *languageTestEnv {
	t.Helper()
	return newLanguageTestEnvWithUtilLocales(t, cfgData, locales, nil)
}

func newLanguageTestEnvWithUtilLocales(t *testing.T, cfgData map[string]any, locales, utilLocales []string) *languageTestEnv {
	t.Helper()
	root := t.TempDir()
	for _, locale := range locales {
		writeCommandTestLocale(t, root, locale)
	}
	for _, locale := range utilLocales {
		writeCommandTestUtilLocale(t, root, locale)
	}

	cfg := config.New(cfgData)
	sqlDB := openCommunityStubDB(t, map[string][]map[string]any{
		"humans": {{
			"id":       "user-1",
			"name":     "Tester",
			"type":     "discord:user",
			"language": "en",
		}},
	}, 0)
	query := db.NewQuery(sqlDB)
	return &languageTestEnv{
		ctx: &Context{
			Config: cfg,
			Query:  query,
			Data: &data.GameData{
				UtilData: map[string]any{
					"languageNames": map[string]any{
						"en": "english",
						"de": "german",
						"fr": "french",
					},
				},
			},
			I18n:     i18n.NewFactory(root, cfg),
			Language: "en",
			Platform: "discord",
			Prefix:   "!",
			TargetOverride: &Target{
				ID:   "user-1",
				Name: "Tester",
				Type: "discord:user",
			},
		},
		query: query,
	}
}

func writeCommandTestLocale(t *testing.T, root, locale string) {
	t.Helper()
	writeCommandTestLocaleData(t, filepath.Join(root, "locale", locale+".json"))
}

func writeCommandTestUtilLocale(t *testing.T, root, locale string) {
	t.Helper()
	writeCommandTestLocaleData(t, filepath.Join(root, "util", "locale", locale+".json"))
}

func writeCommandTestLocaleData(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write locale file: %v", err)
	}
}
