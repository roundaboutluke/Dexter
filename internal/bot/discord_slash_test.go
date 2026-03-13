package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/command"
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

func TestBuildSlashExecutionResultEmptyCommandUsesConfiguredFallbackLocale(t *testing.T) {
	root := writeTestLocale(t, "fr", map[string]string{
		"No command to run.": "Aucune commande a executer.",
	})
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id": "user-1",
		}},
	}, 0)
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "fr",
		},
	})
	env.discord.manager.cfg = cfg
	env.discord.manager.i18n = i18n.NewFactory(root, cfg)
	env.discord.manager.registry = command.NewRegistry()

	result := env.discord.buildSlashExecutionResult(nil, slashTestInteraction("user-1"), "")
	if result.Status != slashExecutionBlocked {
		t.Fatalf("status=%q, want %q", result.Status, slashExecutionBlocked)
	}
	if result.Reply != "Aucune commande a executer." {
		t.Fatalf("reply=%q, want %q", result.Reply, "Aucune commande a executer.")
	}
}

func TestBuildSlashExecutionResultDisabledCommandLocalized(t *testing.T) {
	root := writeTestLocale(t, "fr", map[string]string{
		"That command is disabled.": "Cette commande est desactivee.",
	})
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":           "fr",
			"disabledCommands": []string{"noop"},
		},
	})
	registry := command.NewRegistry()
	handler := &slashStubHandler{name: "noop", reply: "unexpected"}
	registry.Register(handler)
	d := &Discord{manager: &Manager{
		cfg:      cfg,
		i18n:     i18n.NewFactory(root, cfg),
		registry: registry,
	}}

	result := d.buildSlashExecutionResult(nil, slashTestInteraction("user-1"), "noop")
	if result.Status != slashExecutionBlocked {
		t.Fatalf("status=%q, want %q", result.Status, slashExecutionBlocked)
	}
	if result.Reply != "Cette commande est desactivee." {
		t.Fatalf("reply=%q, want %q", result.Reply, "Cette commande est desactivee.")
	}
	if handler.calls != 0 {
		t.Fatalf("handler calls=%d, want 0", handler.calls)
	}
}

func TestBuildSlashExecutionResultEmptyReplyUsesTranslatedDone(t *testing.T) {
	root := writeTestLocale(t, "fr", map[string]string{
		"Done.": "Termine.",
	})
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "fr",
		},
	})
	registry := command.NewRegistry()
	registry.Register(&slashStubHandler{name: "noop"})
	d := &Discord{manager: &Manager{
		cfg:      cfg,
		i18n:     i18n.NewFactory(root, cfg),
		registry: registry,
	}}

	result := d.buildSlashExecutionResult(nil, slashTestInteraction("user-1"), "noop")
	if result.Status != slashExecutionSuccess {
		t.Fatalf("status=%q, want %q", result.Status, slashExecutionSuccess)
	}
	if result.Reply != "Termine." {
		t.Fatalf("reply=%q, want %q", result.Reply, "Termine.")
	}
}

func TestProfileDeleteOutcomeSuppressesTranslatedSuccess(t *testing.T) {
	root := writeTestLocale(t, "fr", map[string]string{
		"Profile removed.": "Profil supprime.",
	})
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":       "user-1",
			"language": "fr",
		}},
		"profiles": {
			{
				"id":         "user-1",
				"profile_no": 1,
				"name":       "Maison",
			},
			{
				"id":         "user-1",
				"profile_no": 2,
				"name":       "Travail",
			},
		},
	}, 0)
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	env.discord.manager.cfg = cfg
	env.discord.manager.i18n = i18n.NewFactory(root, cfg)
	env.discord.manager.registry = command.NewRegistry()

	result := env.discord.buildSlashExecutionResult(nil, slashTestInteraction("user-1"), "profile remove 2")
	if result.Status != slashExecutionSuccess {
		t.Fatalf("status=%q, want %q", result.Status, slashExecutionSuccess)
	}
	if result.Reply != "Profil supprime." {
		t.Fatalf("reply=%q, want %q", result.Reply, "Profil supprime.")
	}
	profiles, err := env.query.SelectAllQuery("profiles", map[string]any{"id": "user-1"})
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	refreshProfile, message := profileDeleteOutcome(profiles, "2", result)
	if !refreshProfile {
		t.Fatal("expected removed profile state to refresh profile payload")
	}
	if message != "" {
		t.Fatalf("message=%q, want empty", message)
	}
}

func TestProfileLocationModalTextLocalized(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":       "user-1",
			"language": "fr",
		}},
	}, 0)
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	env.discord.manager.cfg = cfg
	env.discord.manager.i18n = i18n.NewFactory("/Users/pbx/PoracleJS/PoracleGo", cfg)

	title, label, placeholder := env.discord.profileLocationModalText(slashTestInteraction("user-1"))
	if title != "Définir la position" {
		t.Fatalf("title=%q, want %q", title, "Définir la position")
	}
	if label != "Adresse ou coordonnées" {
		t.Fatalf("label=%q, want %q", label, "Adresse ou coordonnées")
	}
	if placeholder != "51.5,-0.12" {
		t.Fatalf("placeholder=%q, want %q", placeholder, "51.5,-0.12")
	}
}

func TestProfileCreateModalTextLocalized(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":       "user-1",
			"language": "fr",
		}},
	}, 0)
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	env.discord.manager.cfg = cfg
	env.discord.manager.i18n = i18n.NewFactory("/Users/pbx/PoracleJS/PoracleGo", cfg)

	title, label, placeholder := env.discord.profileCreateModalText(slashTestInteraction("user-1"))
	if title != "Nouveau profil" {
		t.Fatalf("title=%q, want %q", title, "Nouveau profil")
	}
	if label != "Nom du profil" {
		t.Fatalf("label=%q, want %q", label, "Nom du profil")
	}
	if placeholder != "home" {
		t.Fatalf("placeholder=%q, want %q", placeholder, "home")
	}
}

func TestShippedLocalesCoverReviewedProfileKeys(t *testing.T) {
	keys := []string{
		"Delete",
		"That is not a valid profile name.",
		"That profile name already exists.",
		"Schedule entry not found.",
	}
	locales := []string{"de", "fr", "it", "nb-no", "pl", "ru", "se"}

	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	factory := i18n.NewFactory("/Users/pbx/PoracleJS/PoracleGo", cfg)

	for _, locale := range locales {
		tr := factory.Translator(locale)
		for _, key := range keys {
			got := strings.TrimSpace(tr.Translate(key, false))
			if got == "" {
				t.Fatalf("locale=%q key=%q translated to empty string", locale, key)
			}
			if got == key {
				t.Fatalf("locale=%q key=%q fell back to english", locale, key)
			}
		}
	}
}

func TestSlashExpiredTextLocalized(t *testing.T) {
	root := writeTestLocale(t, "fr", map[string]string{
		"Slash command expired. Please run the command again.": "La commande slash a expire. Veuillez relancer la commande.",
	})
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "fr",
		},
	})
	d := &Discord{manager: &Manager{
		cfg:  cfg,
		i18n: i18n.NewFactory(root, cfg),
	}}

	if got := d.slashExpiredText(slashTestInteraction("user-1")); got != "La commande slash a expire. Veuillez relancer la commande." {
		t.Fatalf("slashExpiredText()=%q, want %q", got, "La commande slash a expire. Veuillez relancer la commande.")
	}
}

func TestProfileLocationOutcome(t *testing.T) {
	successRefresh, successMessage := profileLocationOutcome(map[string]any{
		"latitude":  51.5,
		"longitude": -0.12,
	}, "51.5,-0.12", slashExecutionResult{
		Status: slashExecutionSuccess,
		Reply:  "Done.",
	})
	if !successRefresh {
		t.Fatal("expected success to refresh profile payload")
	}
	if successMessage != "" {
		t.Fatalf("success message=%q, want empty", successMessage)
	}

	blockedRefresh, blockedMessage := profileLocationOutcome(map[string]any{
		"latitude":  0,
		"longitude": 0,
	}, "51.5,-0.12", slashExecutionResult{
		Status: slashExecutionBlocked,
		Reply:  "blocked message",
	})
	if blockedRefresh {
		t.Fatal("expected blocked outcome to skip profile refresh")
	}
	if blockedMessage != "blocked message" {
		t.Fatalf("blocked message=%q, want %q", blockedMessage, "blocked message")
	}

	errorRefresh, errorMessage := profileLocationOutcome(map[string]any{
		"latitude":  51.4,
		"longitude": -0.11,
	}, "51.5,-0.12", slashExecutionResult{
		Status: slashExecutionError,
		Reply:  "error message",
	})
	if errorRefresh {
		t.Fatal("expected error outcome to skip profile refresh")
	}
	if errorMessage != "error message" {
		t.Fatalf("error message=%q, want %q", errorMessage, "error message")
	}
}

func TestProfileLocationModalRemoveOutcome(t *testing.T) {
	refreshProfile, message, clearState := profileLocationModalRemoveOutcome(map[string]any{
		"latitude":  0,
		"longitude": 0,
	}, slashExecutionResult{
		Status: slashExecutionSuccess,
		Reply:  "Done.",
	})
	if !refreshProfile {
		t.Fatal("expected cleared location state to refresh profile payload")
	}
	if message != "" {
		t.Fatalf("message=%q, want empty", message)
	}
	if !clearState {
		t.Fatal("expected modal remove path to clear slash state")
	}

	refreshProfile, message, clearState = profileLocationModalRemoveOutcome(map[string]any{
		"latitude":  12.34,
		"longitude": 56.78,
	}, slashExecutionResult{
		Status: slashExecutionBlocked,
		Reply:  "blocked message",
	})
	if refreshProfile {
		t.Fatal("expected blocked modal remove result to skip profile refresh")
	}
	if message != "blocked message" {
		t.Fatalf("message=%q, want %q", message, "blocked message")
	}
	if !clearState {
		t.Fatal("expected modal remove path to clear slash state after blocked result")
	}
}

func TestBuildProfileDeletePayloadLocalizesDeleteButton(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":       "user-1",
			"language": "fr",
		}},
		"profiles": {
			{
				"id":         "user-1",
				"profile_no": 1,
				"name":       "Maison",
			},
			{
				"id":         "user-1",
				"profile_no": 2,
				"name":       "Travail",
			},
		},
	}, 0)
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	})
	env.discord.manager.cfg = cfg
	env.discord.manager.i18n = i18n.NewFactory("/Users/pbx/PoracleJS/PoracleGo", cfg)

	_, components, errText := env.discord.buildProfileDeletePayload(slashTestInteraction("user-1"), "2")
	if errText != "" {
		t.Fatalf("buildProfileDeletePayload errText=%q", errText)
	}
	if len(components) == 0 {
		t.Fatal("expected components")
	}
	row, ok := components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) == 0 {
		t.Fatalf("expected action row, got %#v", components[0])
	}
	button, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("expected button, got %#v", row.Components[0])
	}
	if button.Label == "Delete" {
		t.Fatal("delete button label fell back to english")
	}
	if button.Label != "Supprimer" {
		t.Fatalf("delete button label=%q, want %q", button.Label, "Supprimer")
	}
}

func TestProfileCreateOutcomeUsesPostState(t *testing.T) {
	refreshProfile, message := profileCreateOutcome([]map[string]any{{
		"profile_no": 2,
		"name":       "Work",
	}}, "Work", slashExecutionResult{
		Status: slashExecutionSuccess,
		Reply:  "Profile added.",
	})
	if !refreshProfile {
		t.Fatal("expected created profile state to refresh profile payload")
	}
	if message != "" {
		t.Fatalf("message=%q, want empty", message)
	}

	refreshProfile, message = profileCreateOutcome([]map[string]any{{
		"profile_no": 1,
		"name":       "Home",
	}}, "Work", slashExecutionResult{
		Status: slashExecutionSuccess,
		Reply:  "You do not have permission to execute this command",
	})
	if refreshProfile {
		t.Fatal("expected missing profile state to skip profile refresh")
	}
	if message != "You do not have permission to execute this command" {
		t.Fatalf("message=%q, want permission reply", message)
	}
}

func TestDirectClearLocationUsesBlockedOutcomeMessage(t *testing.T) {
	root := writeTestLocale(t, "fr", map[string]string{
		"That command is disabled.": "Cette commande est desactivee.",
	})
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":           "fr",
			"disabledCommands": []string{"location"},
		},
	})
	d := &Discord{manager: &Manager{
		cfg:      cfg,
		i18n:     i18n.NewFactory(root, cfg),
		registry: command.NewRegistry(),
	}}

	result := d.buildSlashExecutionResult(nil, slashTestInteraction("user-1"), "location remove")
	refreshProfile, message := profileLocationOutcome(map[string]any{
		"latitude":  12.34,
		"longitude": 56.78,
	}, "remove", result)
	if refreshProfile {
		t.Fatal("expected clear-location blocked result to skip profile refresh")
	}
	if message != "Cette commande est desactivee." {
		t.Fatalf("message=%q, want %q", message, "Cette commande est desactivee.")
	}
}

func TestAreaToggleOutcomeUsesDesiredPostState(t *testing.T) {
	refreshArea, message := areaToggleOutcome([]string{"home"}, "home", true, slashExecutionResult{
		Status: slashExecutionSuccess,
		Reply:  "Added areas: home",
	})
	if !refreshArea {
		t.Fatal("expected enabled area state to refresh area payload")
	}
	if message != "" {
		t.Fatalf("message=%q, want empty", message)
	}

	refreshArea, message = areaToggleOutcome([]string{"home"}, "home", false, slashExecutionResult{
		Status: slashExecutionSuccess,
		Reply:  "Removed areas: home",
	})
	if refreshArea {
		t.Fatal("expected still-enabled area state to skip area refresh on remove")
	}
	if message != "Removed areas: home" {
		t.Fatalf("message=%q, want original reply", message)
	}

	refreshArea, message = areaToggleOutcome([]string{}, "home", false, slashExecutionResult{
		Status: slashExecutionSuccess,
		Reply:  "Removed areas: home",
	})
	if !refreshArea {
		t.Fatal("expected missing area after remove to refresh area payload")
	}
	if message != "" {
		t.Fatalf("message=%q, want empty", message)
	}
}

func TestSlashConfirmCloseoutPayloadPreservesMessageAndClearsButtons(t *testing.T) {
	embed := &discordgo.MessageEmbed{Title: "Ready"}
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Message: &discordgo.Message{
				Content: "Run this command",
				Embeds:  []*discordgo.MessageEmbed{embed},
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{
						discordgo.Button{CustomID: "confirm", Label: "Verify", Style: discordgo.SuccessButton},
					}},
				},
			},
		},
	}

	text, embeds, components := slashConfirmCloseoutPayload(i)
	if text != "Run this command" {
		t.Fatalf("text=%q, want %q", text, "Run this command")
	}
	if len(embeds) != 1 || embeds[0] != embed {
		t.Fatalf("embeds=%v, want preserved embed", embeds)
	}
	if len(components) != 0 {
		t.Fatalf("components=%v, want empty", components)
	}
	if text == "Confirmed ✅" {
		t.Fatal("closeout payload should not inject success text")
	}
}

func TestSlashConfirmCloseoutPayloadFallsBackWithoutMessage(t *testing.T) {
	text, embeds, components := slashConfirmCloseoutPayload(slashTestInteraction("user-1"))
	if text != "" {
		t.Fatalf("text=%q, want empty", text)
	}
	if embeds != nil {
		t.Fatalf("embeds=%v, want nil", embeds)
	}
	if len(components) != 0 {
		t.Fatalf("components=%v, want empty", components)
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

type slashStubHandler struct {
	name  string
	reply string
	err   error
	calls int
}

func (h *slashStubHandler) Name() string { return h.name }

func (h *slashStubHandler) Handle(_ *command.Context, _ []string) (string, error) {
	h.calls++
	if h.err != nil {
		return "", h.err
	}
	return h.reply, nil
}

func slashTestInteraction(userID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Member: &discordgo.Member{
				User: &discordgo.User{ID: userID, Username: "tester"},
			},
		},
	}
}

func writeTestLocale(t *testing.T, locale string, entries map[string]string) string {
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
	if err := os.WriteFile(filepath.Join(localeDir, locale+".json"), data, 0o644); err != nil {
		t.Fatalf("write locale file: %v", err)
	}
	return root
}
