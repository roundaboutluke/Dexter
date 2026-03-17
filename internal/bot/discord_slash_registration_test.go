package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

func TestSlashCommandDefinitionsDiscoverLocalizationsWhenConfigEmpty(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(shippedLocaleRoot(t), cfg),
	}}

	commands := d.slashCommandDefinitions()
	profile := findSlashCommand(commands, "profile")
	if profile == nil || profile.NameLocalizations == nil {
		t.Fatal("profile command localizations missing")
	}
	if got := (*profile.NameLocalizations)[discordgo.German]; got != "profil" {
		t.Fatalf("profile german localization=%q, want %q", got, "profil")
	}
	if got := (*profile.NameLocalizations)[discordgo.French]; got != "profil" {
		t.Fatalf("profile french localization=%q, want %q", got, "profil")
	}
}

func TestAutocompleteLanguageChoicesDiscoversLocalesWhenConfigEmpty(t *testing.T) {
	root := t.TempDir()
	writeBotTestLocale(t, root, "de")
	writeBotTestLocale(t, root, "fr")

	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(root, cfg),
		data: &data.GameData{
			UtilData: map[string]any{
				"languageNames": map[string]any{
					"de": "german",
					"fr": "french",
				},
			},
		},
	}}

	choices := d.autocompleteLanguageChoices("")
	if !hasChoiceValue(choices, "de") {
		t.Fatal("expected german choice")
	}
	if !hasChoiceValue(choices, "fr") {
		t.Fatal("expected french choice")
	}
}

func TestAutocompleteLanguageChoicesSkipsUtilOnlyLocalesWhenConfigEmpty(t *testing.T) {
	root := t.TempDir()
	writeBotTestLocale(t, root, "de")
	writeBotUtilLocale(t, root, "fr")

	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(root, cfg),
		data: &data.GameData{
			UtilData: map[string]any{
				"languageNames": map[string]any{
					"de": "german",
					"fr": "french",
				},
			},
		},
	}}

	choices := d.autocompleteLanguageChoices("")
	if !hasChoiceValue(choices, "de") {
		t.Fatal("expected german choice")
	}
	if hasChoiceValue(choices, "fr") {
		t.Fatal("did not expect util-only french choice")
	}
}

func TestSlashCommandDefinitionsIncludeDefaultLocaleOutsideAllowlist(t *testing.T) {
	root := t.TempDir()
	writeBotLocaleData(t, filepath.Join(root, "locale", "fr.json"), map[string]string{
		"slash.command.profile": "profil",
	})
	writeBotTestLocale(t, root, "en")

	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "fr",
			"availableLanguages": map[string]any{"en": true},
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(root, cfg),
	}}

	commands := d.slashCommandDefinitions()
	profile := findSlashCommand(commands, "profile")
	if profile == nil || profile.NameLocalizations == nil {
		t.Fatal("profile command localizations missing")
	}
	if got := (*profile.NameLocalizations)[discordgo.French]; got != "profil" {
		t.Fatalf("profile french localization=%q, want %q", got, "profil")
	}
}

func TestRegisterSlashCommandsEditsStaleCommands(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(shippedLocaleRoot(t), cfg),
	}}

	desired := d.slashCommandDefinitions()
	existing := cloneApplicationCommands(t, desired)
	profile := findSlashCommand(existing, "profile")
	if profile == nil {
		t.Fatal("profile command missing from existing set")
	}
	profile.NameLocalizations = nil

	edited := []string{}
	created := []string{}
	d.commandFetcher = func(*discordgo.Session, string, string) ([]*discordgo.ApplicationCommand, error) {
		return existing, nil
	}
	d.commandEditor = func(_ *discordgo.Session, _, _, _ string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		edited = append(edited, cmd.Name)
		return cmd, nil
	}
	d.commandCreator = func(_ *discordgo.Session, _, _ string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		created = append(created, cmd.Name)
		return cmd, nil
	}

	session := &discordgo.Session{State: &discordgo.State{Ready: discordgo.Ready{User: &discordgo.User{ID: "app-1"}}}}
	d.registerSlashCommands(session)

	if len(created) != 0 {
		t.Fatalf("created=%v, want none", created)
	}
	if len(edited) != 1 || edited[0] != "profile" {
		t.Fatalf("edited=%v, want [profile]", edited)
	}
}

func TestRegisterSlashCommandsCreatesMissingAndPreservesUnrelated(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(shippedLocaleRoot(t), cfg),
	}}

	desired := d.slashCommandDefinitions()
	existing := cloneApplicationCommands(t, desired)
	filtered := make([]*discordgo.ApplicationCommand, 0, len(existing))
	for _, cmd := range existing {
		if cmd.Name == "language" {
			continue
		}
		filtered = append(filtered, cmd)
	}
	filtered = append(filtered, &discordgo.ApplicationCommand{
		ID:          "custom-1",
		Type:        discordgo.ChatApplicationCommand,
		Name:        "other",
		Description: "Unrelated command",
	})

	edited := []string{}
	created := []string{}
	d.commandFetcher = func(*discordgo.Session, string, string) ([]*discordgo.ApplicationCommand, error) {
		return filtered, nil
	}
	d.commandEditor = func(_ *discordgo.Session, _, _, _ string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		edited = append(edited, cmd.Name)
		return cmd, nil
	}
	d.commandCreator = func(_ *discordgo.Session, _, _ string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
		created = append(created, cmd.Name)
		return cmd, nil
	}

	session := &discordgo.Session{State: &discordgo.State{Ready: discordgo.Ready{User: &discordgo.User{ID: "app-1"}}}}
	d.registerSlashCommands(session)

	if len(edited) != 0 {
		t.Fatalf("edited=%v, want none", edited)
	}
	if len(created) != 1 || created[0] != "language" {
		t.Fatalf("created=%v, want [language]", created)
	}
}

func TestRegisterSlashCommandsDeletesLegacySlashCommands(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(shippedLocaleRoot(t), cfg),
	}}

	existing := cloneApplicationCommands(t, d.slashCommandDefinitions())
	existing = append(existing,
		&discordgo.ApplicationCommand{ID: "legacy-tracked", Type: discordgo.ChatApplicationCommand, Name: "tracked", Description: "Old tracked command"},
		&discordgo.ApplicationCommand{ID: "legacy-remove", Type: discordgo.ChatApplicationCommand, Name: "remove", Description: "Old remove command"},
		&discordgo.ApplicationCommand{ID: "custom-1", Type: discordgo.ChatApplicationCommand, Name: "other", Description: "Unrelated command"},
	)

	deleted := []string{}
	d.commandFetcher = func(*discordgo.Session, string, string) ([]*discordgo.ApplicationCommand, error) {
		return existing, nil
	}
	d.commandDeleter = func(_ *discordgo.Session, _, _, cmdID string) error {
		deleted = append(deleted, cmdID)
		return nil
	}

	session := &discordgo.Session{State: &discordgo.State{Ready: discordgo.Ready{User: &discordgo.User{ID: "app-1"}}}}
	d.registerSlashCommands(session)

	if strings.Join(deleted, ",") != "legacy-tracked,legacy-remove" {
		t.Fatalf("deleted=%v, want legacy tracked/remove only", deleted)
	}
}

func cloneApplicationCommands(t *testing.T, commands []*discordgo.ApplicationCommand) []*discordgo.ApplicationCommand {
	t.Helper()
	raw, err := json.Marshal(commands)
	if err != nil {
		t.Fatalf("marshal commands: %v", err)
	}
	var cloned []*discordgo.ApplicationCommand
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatalf("unmarshal commands: %v", err)
	}
	for idx, cmd := range cloned {
		if cmd != nil && cmd.ID == "" {
			cmd.ID = cmd.Name + "-id"
		}
		if cmd != nil && cmd.Type == 0 {
			cmd.Type = discordgo.ChatApplicationCommand
		}
		_ = idx
	}
	return cloned
}

func writeBotTestLocale(t *testing.T, root, locale string) {
	t.Helper()
	writeBotLocaleData(t, filepath.Join(root, "locale", locale+".json"), map[string]string{})
}

func writeBotUtilLocale(t *testing.T, root, locale string) {
	t.Helper()
	writeBotLocaleData(t, filepath.Join(root, "util", "locale", locale+".json"), map[string]string{})
}

func writeBotLocaleData(t *testing.T, path string, data map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	payload := []byte(`{}`)
	if len(data) > 0 {
		payload = []byte("{\n")
		first := true
		for key, value := range data {
			line := ""
			if !first {
				line += ",\n"
			}
			line += `  "` + key + `": "` + value + `"`
			payload = append(payload, []byte(line)...)
			first = false
		}
		payload = append(payload, []byte("\n}")...)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write locale file: %v", err)
	}
}
