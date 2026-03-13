package bot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/config"
	"poraclego/internal/dts"
	"poraclego/internal/i18n"
)

func TestSlashCommandDefinitionsGuidedEntryOptionsAreOptional(t *testing.T) {
	d := &Discord{manager: &Manager{cfg: config.New(nil)}}
	commands := d.slashCommandDefinitions()

	cases := []struct {
		command string
		option  string
	}{
		{command: "track", option: "pokemon"},
		{command: "raid", option: "type"},
		{command: "egg", option: "level"},
		{command: "maxbattle", option: "type"},
		{command: "quest", option: "type"},
		{command: "invasion", option: "type"},
		{command: "gym", option: "team"},
		{command: "fort", option: "type"},
		{command: "nest", option: "pokemon"},
		{command: "weather", option: "condition"},
		{command: "lure", option: "type"},
		{command: "info", option: "type"},
	}

	for _, tc := range cases {
		cmd := findSlashCommand(commands, tc.command)
		if cmd == nil {
			t.Fatalf("command %q not found", tc.command)
		}
		option := findSlashOption(cmd.Options, tc.option)
		if option == nil {
			t.Fatalf("option %q not found on command %q", tc.option, tc.command)
		}
		if option.Required {
			t.Fatalf("option %q on command %q should be optional", tc.option, tc.command)
		}
	}
}

func TestSlashCommandDefinitionsProfileAndHelpOptions(t *testing.T) {
	d := &Discord{manager: &Manager{cfg: config.New(nil)}}
	commands := d.slashCommandDefinitions()

	tracked := findSlashCommand(commands, "tracked")
	if tracked == nil {
		t.Fatal("tracked command not found")
	}
	trackedProfile := findSlashOption(tracked.Options, "profile")
	if trackedProfile == nil || !trackedProfile.Autocomplete || trackedProfile.Required {
		t.Fatal("tracked profile option should exist, autocomplete, and remain optional")
	}

	remove := findSlashCommand(commands, "remove")
	if remove == nil {
		t.Fatal("remove command not found")
	}
	removeProfile := findSlashOption(remove.Options, "profile")
	if removeProfile == nil || !removeProfile.Autocomplete || removeProfile.Required {
		t.Fatal("remove profile option should exist, autocomplete, and remain optional")
	}

	help := findSlashCommand(commands, "help")
	if help == nil {
		t.Fatal("help command not found")
	}
	helpCommand := findSlashOption(help.Options, "command")
	if helpCommand == nil || !helpCommand.Autocomplete || helpCommand.Required {
		t.Fatal("help command option should exist, autocomplete, and remain optional")
	}
}

func TestAutocompleteHelpCommandChoicesUsesDiscordTemplates(t *testing.T) {
	lang := "en"
	d := &Discord{
		manager: &Manager{
			cfg: config.New(nil),
			templates: []dts.Template{
				{Type: "help", Platform: "discord", Language: &lang, ID: "tracked"},
				{Type: "help", Platform: "discord", Language: &lang, ID: "remove"},
				{Type: "help", Platform: "discord", Language: &lang, ID: "slash"},
				{Type: "help", Platform: "telegram", Language: &lang, ID: "profile"},
			},
		},
	}

	choices := d.autocompleteHelpCommandChoices("")
	if !hasChoiceValue(choices, "tracked") {
		t.Fatal("expected tracked help choice")
	}
	if !hasChoiceValue(choices, "remove") {
		t.Fatal("expected remove help choice")
	}
	if hasChoiceValue(choices, "slash") {
		t.Fatal("did not expect slash greeting help choice")
	}
	if hasChoiceValue(choices, "profile") {
		t.Fatal("did not expect telegram-only help choice")
	}
}

func TestProfileDisplayName(t *testing.T) {
	if got := profileDisplayName(map[string]any{"profile_no": 2, "name": "Work"}); got != "Work (P2)" {
		t.Fatalf("unexpected profile display name: %q", got)
	}
	if got := profileDisplayName(map[string]any{"profile_no": 3}); got != "Profile 3" {
		t.Fatalf("unexpected fallback profile display name: %q", got)
	}
}

func TestEffectiveProfileNoFromHuman(t *testing.T) {
	if got := effectiveProfileNoFromHuman(map[string]any{"current_profile_no": 2, "preferred_profile_no": 1}); got != 2 {
		t.Fatalf("expected current profile to win, got %d", got)
	}
	if got := effectiveProfileNoFromHuman(map[string]any{"current_profile_no": 0, "preferred_profile_no": 4}); got != 4 {
		t.Fatalf("expected preferred profile fallback, got %d", got)
	}
	if got := effectiveProfileNoFromHuman(map[string]any{}); got != 1 {
		t.Fatalf("expected profile fallback to 1, got %d", got)
	}
}

func TestSlashCommandDefinitionsUseConfiguredLocalizations(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"en": true, "fr": true, "xx": true},
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory("/Users/pbx/PoracleJS/PoracleGo", cfg),
	}}

	commands := d.slashCommandDefinitions()

	profile := findSlashCommand(commands, "profile")
	if profile == nil {
		t.Fatal("profile command not found")
	}
	if profile.NameLocalizations == nil {
		t.Fatal("profile name localizations missing")
	}
	if got := (*profile.NameLocalizations)[discordgo.French]; got != "profil" {
		t.Fatalf("profile french localization=%q, want %q", got, "profil")
	}
	if _, ok := (*profile.NameLocalizations)[discordgo.EnglishUS]; ok {
		t.Fatal("did not expect english localization entry when translated value matches default")
	}

	track := findSlashCommand(commands, "track")
	if track == nil {
		t.Fatal("track command not found")
	}
	gender := findSlashOption(track.Options, "gender")
	if gender == nil {
		t.Fatal("track gender option not found")
	}
	if len(gender.Choices) < 3 {
		t.Fatalf("gender choices=%d, want at least 3", len(gender.Choices))
	}
	if got := gender.Choices[1].NameLocalizations[discordgo.French]; got != "Mâle" {
		t.Fatalf("male french localization=%q, want %q", got, "Mâle")
	}
	if got := gender.Choices[2].NameLocalizations[discordgo.French]; got != "Femelle" {
		t.Fatalf("female french localization=%q, want %q", got, "Femelle")
	}
}

func TestLocalizedSlashCommandsSkipInvalidNameAndKeepDescription(t *testing.T) {
	root := t.TempDir()
	localeDir := filepath.Join(root, "locale")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	payload := `{"language":"langue invalide","Change language":"Changer de langue"}`
	if err := os.WriteFile(filepath.Join(localeDir, "fr.json"), []byte(payload), 0o644); err != nil {
		t.Fatalf("write locale file: %v", err)
	}

	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"fr": true},
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(root, cfg),
	}}

	commands := d.localizedSlashCommands([]*discordgo.ApplicationCommand{{
		Name:        "language",
		Description: "Change language",
	}})
	cmd := commands[0]
	if cmd.NameLocalizations != nil && len(*cmd.NameLocalizations) > 0 {
		t.Fatalf("expected invalid localized name to be skipped, got %v", *cmd.NameLocalizations)
	}
	if cmd.DescriptionLocalizations == nil {
		t.Fatal("description localizations missing")
	}
	if got := (*cmd.DescriptionLocalizations)[discordgo.French]; got != "Changer de langue" {
		t.Fatalf("description french localization=%q, want %q", got, "Changer de langue")
	}
}

func TestUserLanguageFallsBackToConfiguredLocale(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id": "user-1",
		}},
	}, 0)
	env.discord.manager.cfg = config.New(map[string]any{
		"general": map[string]any{
			"locale": "fr",
		},
	})

	if got := env.discord.userLanguage("user-1"); got != "fr" {
		t.Fatalf("userLanguage()=%q, want %q", got, "fr")
	}
}

func TestBuildProfileScheduleDayPayloadLocalizesDayOptions(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":       "user-1",
			"language": "fr",
		}},
		"profiles": {{
			"id":         "user-1",
			"profile_no": 1,
			"name":       "Maison",
		}},
	}, 0)
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"fr": true},
		},
	})
	env.discord.manager.cfg = cfg
	env.discord.manager.i18n = i18n.NewFactory("/Users/pbx/PoracleJS/PoracleGo", cfg)

	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: &discordgo.Member{
				User: &discordgo.User{ID: "user-1", Username: "tester"},
			},
		},
	}

	_, components, errText := env.discord.buildProfileScheduleDayPayload(i, "1")
	if errText != "" {
		t.Fatalf("buildProfileScheduleDayPayload errText=%q", errText)
	}
	if len(components) == 0 {
		t.Fatal("expected components")
	}
	row, ok := components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) == 0 {
		t.Fatalf("expected select row, got %#v", components[0])
	}
	menu, ok := row.Components[0].(discordgo.SelectMenu)
	if !ok {
		t.Fatalf("expected select menu, got %#v", row.Components[0])
	}
	if got := menu.Options[0].Label; got != "Lundi" {
		t.Fatalf("first day label=%q, want %q", got, "Lundi")
	}
	if got := menu.Options[1].Label; got != "Mardi" {
		t.Fatalf("second day label=%q, want %q", got, "Mardi")
	}
}

func findSlashCommand(commands []*discordgo.ApplicationCommand, name string) *discordgo.ApplicationCommand {
	for _, cmd := range commands {
		if cmd != nil && cmd.Name == name {
			return cmd
		}
	}
	return nil
}

func findSlashOption(options []*discordgo.ApplicationCommandOption, name string) *discordgo.ApplicationCommandOption {
	for _, option := range options {
		if option != nil && option.Name == name {
			return option
		}
	}
	return nil
}

func hasChoiceValue(choices []*discordgo.ApplicationCommandOptionChoice, value string) bool {
	for _, choice := range choices {
		if choice != nil && choice.Value == value {
			return true
		}
	}
	return false
}
