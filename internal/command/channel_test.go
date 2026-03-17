package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/db"
	"poraclego/internal/i18n"
)

func TestChannelAddPersistsDiscoveredRuntimeLanguage(t *testing.T) {
	env := newChannelTestEnv(t, map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}, []string{"en", "de"}, nil)

	reply, err := (&ChannelCommand{}).Handle(env.ctx, []string{"add", "language:de"})
	if err != nil {
		t.Fatalf("handle channel add: %v", err)
	}
	if !strings.Contains(strings.ToLower(reply), "german") && !strings.Contains(strings.ToLower(reply), "de") {
		t.Fatalf("reply=%q, want language confirmation", reply)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": env.ctx.ChannelID})
	if err != nil {
		t.Fatalf("select channel: %v", err)
	}
	if got := strings.ToLower(strings.TrimSpace(row["language"].(string))); got != "de" {
		t.Fatalf("language=%q, want %q", got, "de")
	}
}

func TestChannelAddSkipsUtilOnlyLanguageWhenConfigEmpty(t *testing.T) {
	env := newChannelTestEnv(t, map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}, []string{"en", "de"}, []string{"fr"})

	if _, err := (&ChannelCommand{}).Handle(env.ctx, []string{"add", "language:fr"}); err != nil {
		t.Fatalf("handle channel add: %v", err)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": env.ctx.ChannelID})
	if err != nil {
		t.Fatalf("select channel: %v", err)
	}
	if value, ok := row["language"]; ok && value != nil {
		t.Fatalf("language=%v, want nil", value)
	}
}

func TestChannelAddRespectsExplicitLanguageAllowlist(t *testing.T) {
	env := newChannelTestEnv(t, map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"en": true},
		},
	}, []string{"en", "de"}, nil)

	if _, err := (&ChannelCommand{}).Handle(env.ctx, []string{"add", "language:de"}); err != nil {
		t.Fatalf("handle channel add: %v", err)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": env.ctx.ChannelID})
	if err != nil {
		t.Fatalf("select channel: %v", err)
	}
	if value, ok := row["language"]; ok && value != nil {
		t.Fatalf("language=%v, want nil", value)
	}
}

func TestChannelAddResolvesHumanReadableLanguageLabel(t *testing.T) {
	env := newChannelTestEnv(t, map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}, []string{"en", "de"}, nil)

	if _, err := (&ChannelCommand{}).Handle(env.ctx, []string{"add", "language:german"}); err != nil {
		t.Fatalf("handle channel add: %v", err)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": env.ctx.ChannelID})
	if err != nil {
		t.Fatalf("select channel: %v", err)
	}
	if got := strings.ToLower(strings.TrimSpace(row["language"].(string))); got != "de" {
		t.Fatalf("language=%q, want %q", got, "de")
	}
}

func TestChannelAddFallsBackToLocaleCodeForCustomLanguageLabel(t *testing.T) {
	env := newChannelTestEnvWithCustomLocales(t, map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}, []string{"en"}, nil, []string{"xx"})

	reply, err := (&ChannelCommand{}).Handle(env.ctx, []string{"add", "language:xx"})
	if err != nil {
		t.Fatalf("handle channel add: %v", err)
	}
	if !strings.Contains(reply, "xx") {
		t.Fatalf("reply=%q, want locale code fallback", reply)
	}
	if !strings.Contains(strings.ToLower(reply), "language xx") {
		t.Fatalf("reply=%q, want explicit locale code label", reply)
	}

	row, err := env.query.SelectOneQuery("humans", map[string]any{"id": env.ctx.ChannelID})
	if err != nil {
		t.Fatalf("select channel: %v", err)
	}
	if got := strings.ToLower(strings.TrimSpace(row["language"].(string))); got != "xx" {
		t.Fatalf("language=%q, want %q", got, "xx")
	}
}

type channelTestEnv struct {
	ctx   *Context
	query *db.Query
}

func newChannelTestEnv(t *testing.T, cfgData map[string]any, locales, utilLocales []string) *channelTestEnv {
	t.Helper()
	return newChannelTestEnvWithCustomLocales(t, cfgData, locales, utilLocales, nil)
}

func newChannelTestEnvWithCustomLocales(t *testing.T, cfgData map[string]any, locales, utilLocales, customLocales []string) *channelTestEnv {
	t.Helper()
	root := t.TempDir()
	for _, locale := range locales {
		writeCommandTestLocale(t, root, locale)
	}
	for _, locale := range utilLocales {
		writeCommandTestUtilLocale(t, root, locale)
	}
	for _, locale := range customLocales {
		writeChannelTestCustomLocale(t, root, locale)
	}

	cfg := config.New(cfgData)
	sqlDB := openCommunityStubDB(t, map[string][]map[string]any{
		"humans": {},
	}, 0)
	query := db.NewQuery(sqlDB)
	return &channelTestEnv{
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
			I18n:        i18n.NewFactory(root, cfg),
			Language:    "en",
			Platform:    "discord",
			Prefix:      "!",
			UserID:      "admin",
			UserName:    "Admin",
			ChannelID:   "chan-1",
			ChannelName: "alerts",
			IsAdmin:     true,
		},
		query: query,
	}
}

func writeChannelTestCustomLocale(t *testing.T, root, locale string) {
	t.Helper()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "custom."+locale+".json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write custom locale file: %v", err)
	}
}
