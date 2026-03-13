package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/command"
	"poraclego/internal/config"
	"poraclego/internal/dts"
	"poraclego/internal/geofence"
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
	removeTracking := findSlashOption(remove.Options, "tracking")
	if removeTracking == nil || !removeTracking.Autocomplete || !removeTracking.Required {
		t.Fatal("remove tracking option should exist, autocomplete, and remain required")
	}
	if len(remove.Options) < 3 || remove.Options[1].Name != "tracking" || remove.Options[2].Name != "profile" {
		t.Fatal("remove command should place required tracking before optional profile")
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

func TestAreaMapRequestUsesFencePathHash(t *testing.T) {
	d := &Discord{manager: &Manager{
		fences: &geofence.Store{Fences: []geofence.Fence{
			{Name: "Home", Path: [][]float64{{51.5, -0.1}, {51.6, -0.1}, {51.6, -0.2}}},
			{Name: "Work", Path: [][]float64{{40.0, -73.0}, {40.1, -73.0}, {40.1, -73.1}}},
		}},
	}}

	homeA := d.areaMapRequest("Home")
	homeB := d.areaMapRequest("home")
	work := d.areaMapRequest("Work")
	if homeA == nil || homeB == nil || work == nil {
		t.Fatal("expected map requests for known fences")
	}
	if homeA.Key != homeB.Key {
		t.Fatalf("same fence key mismatch: %q vs %q", homeA.Key, homeB.Key)
	}
	if homeA.Key == work.Key {
		t.Fatalf("different fence paths should not share cache key: %q", homeA.Key)
	}
}

func TestBuildAreaShowPayloadStateUsesCacheAndMissesWithoutImage(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id":   "user-1",
			"area": `["Home"]`,
		}},
	}, 0)
	env.discord.manager.cfg = config.New(map[string]any{
		"geocoding": map[string]any{
			"staticProvider":    "tileservercache",
			"staticProviderURL": "https://tiles.example",
		},
	})
	env.discord.manager.fences = &geofence.Store{Fences: []geofence.Fence{{
		Name: "Home",
		Path: [][]float64{{51.5, -0.1}, {51.6, -0.1}, {51.6, -0.2}},
	}}}

	embed, _, mapReq, errText := env.discord.buildAreaShowPayloadState(slashTestInteraction("user-1"), "Home")
	if errText != "" {
		t.Fatalf("buildAreaShowPayloadState errText=%q", errText)
	}
	if mapReq == nil {
		t.Fatal("expected map request for known area")
	}
	if embed.Image != nil {
		t.Fatalf("expected uncached area render to omit image, got %#v", embed.Image)
	}

	env.discord.ensureSlashMapState()
	env.discord.mapCache[mapReq.Key] = slashMapCacheEntry{
		URL:       "https://tiles.example/pregenerated/home.png",
		ExpiresAt: time.Now().Add(slashAreaMapSuccessTTL),
	}
	embed, _, _, errText = env.discord.buildAreaShowPayloadState(slashTestInteraction("user-1"), "Home")
	if errText != "" {
		t.Fatalf("cached buildAreaShowPayloadState errText=%q", errText)
	}
	if embed.Image == nil || embed.Image.URL != "https://tiles.example/pregenerated/home.png" {
		t.Fatalf("cached area image=%#v, want pregenerated URL", embed.Image)
	}
}

func TestBuildProfilePayloadStateUsesCacheAndMissesWithoutImage(t *testing.T) {
	env := newSlashMutationTestEnv(t, map[string][]map[string]any{
		"humans": {{
			"id": "user-1",
		}},
		"profiles": {{
			"id":         "user-1",
			"profile_no": 1,
			"name":       "Home",
			"area":       `["Home"]`,
		}},
	}, 0)
	env.discord.manager.cfg = config.New(map[string]any{
		"geocoding": map[string]any{
			"staticProvider":    "tileservercache",
			"staticProviderURL": "https://tiles.example",
		},
	})
	env.discord.manager.fences = &geofence.Store{Fences: []geofence.Fence{{
		Name: "Home",
		Path: [][]float64{{51.5, -0.1}, {51.6, -0.1}, {51.6, -0.2}},
	}}}

	embed, _, mapReq, errText := env.discord.buildProfilePayloadState(slashTestInteraction("user-1"), "1")
	if errText != "" {
		t.Fatalf("buildProfilePayloadState errText=%q", errText)
	}
	if mapReq == nil {
		t.Fatal("expected profile map request")
	}
	if embed.Image != nil {
		t.Fatalf("expected uncached profile render to omit image, got %#v", embed.Image)
	}

	env.discord.ensureSlashMapState()
	env.discord.mapCache[mapReq.Key] = slashMapCacheEntry{
		URL:       "https://tiles.example/pregenerated/profile.png",
		ExpiresAt: time.Now().Add(slashAreaMapSuccessTTL),
	}
	embed, _, _, errText = env.discord.buildProfilePayloadState(slashTestInteraction("user-1"), "1")
	if errText != "" {
		t.Fatalf("cached buildProfilePayloadState errText=%q", errText)
	}
	if embed.Image == nil || embed.Image.URL != "https://tiles.example/pregenerated/profile.png" {
		t.Fatalf("cached profile image=%#v, want pregenerated URL", embed.Image)
	}
}

func TestQueueSlashMapResolutionCoalescesInFlightRequests(t *testing.T) {
	d := &Discord{manager: &Manager{
		cfg: config.New(map[string]any{
			"geocoding": map[string]any{
				"staticProvider":    "tileservercache",
				"staticProviderURL": "https://tiles.example",
			},
		}),
	}}
	gen := &slashMapStubGenerator{
		started: make(chan string, 2),
		release: make(chan struct{}),
		urls: map[string]string{
			"area:home": "https://tiles.example/pregenerated/home.png",
		},
	}
	d.mapGenerator = gen

	req := &slashMapRequest{Key: "area:home", Kind: "area", Area: "Home"}
	d.queueSlashMapResolution(nil, req)
	d.queueSlashMapResolution(nil, req)

	select {
	case got := <-gen.started:
		if got != req.Key {
			t.Fatalf("started key=%q, want %q", got, req.Key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for slash map job start")
	}
	if calls := gen.callCount(); calls != 1 {
		t.Fatalf("generator calls=%d, want 1", calls)
	}

	close(gen.release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if url, ok := d.slashMapCacheStatus(req); ok && url == "https://tiles.example/pregenerated/home.png" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected resolved map URL to be cached")
}

func TestQueueSlashMapResolutionAppliesFailureBackoff(t *testing.T) {
	d := &Discord{manager: &Manager{
		cfg: config.New(map[string]any{
			"geocoding": map[string]any{
				"staticProvider":    "tileservercache",
				"staticProviderURL": "https://tiles.example",
			},
		}),
	}}
	gen := &slashMapStubGenerator{
		started: make(chan string, 2),
		release: make(chan struct{}),
		errs: map[string]error{
			"area:home": errSlashMapStubFailure,
		},
	}
	d.mapGenerator = gen

	req := &slashMapRequest{Key: "area:home", Kind: "area", Area: "Home"}
	d.queueSlashMapResolution(nil, req)
	select {
	case <-gen.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for failing slash map job start")
	}
	close(gen.release)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		d.ensureSlashMapState()
		d.mapMu.Lock()
		entry := d.mapCache[req.Key]
		d.mapMu.Unlock()
		if !entry.RetryAfter.IsZero() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	before := gen.callCount()
	d.queueSlashMapResolution(nil, req)
	time.Sleep(50 * time.Millisecond)
	if after := gen.callCount(); after != before {
		t.Fatalf("expected backoff to suppress retry, calls=%d -> %d", before, after)
	}
}

func TestQueueSlashMapResolutionRegeneratesExpiredSuccess(t *testing.T) {
	d := &Discord{manager: &Manager{
		cfg: config.New(map[string]any{
			"geocoding": map[string]any{
				"staticProvider":    "tileservercache",
				"staticProviderURL": "https://tiles.example",
			},
		}),
	}}
	gen := &slashMapStubGenerator{
		started: make(chan string, 1),
		release: make(chan struct{}),
		urls: map[string]string{
			"location:51.5,-0.12": "https://tiles.example/pregenerated/location.png",
		},
	}
	d.mapGenerator = gen
	req := &slashMapRequest{Key: "location:51.5,-0.12", Kind: "location", Latitude: 51.5, Longitude: -0.12}

	d.ensureSlashMapState()
	d.mapCache[req.Key] = slashMapCacheEntry{
		URL:       "https://tiles.example/pregenerated/stale-location.png",
		ExpiresAt: time.Now().Add(-time.Second),
	}

	d.queueSlashMapResolution(nil, req)
	select {
	case got := <-gen.started:
		if got != req.Key {
			t.Fatalf("started key=%q, want %q", got, req.Key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for regeneration after expired cache entry")
	}
	close(gen.release)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if url, ok := d.slashMapCacheStatus(req); ok && url == "https://tiles.example/pregenerated/location.png" {
			d.mapMu.Lock()
			entry := d.mapCache[req.Key]
			d.mapMu.Unlock()
			if !entry.ExpiresAt.After(time.Now()) {
				t.Fatal("expected regenerated cache entry to have a fresh expiry")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected expired success entry to regenerate")
}

func TestSlashMapCacheStatusPrunesExpiredEntries(t *testing.T) {
	d := &Discord{}
	req := &slashMapRequest{Key: "area:home", Kind: "area", Area: "Home"}
	d.ensureSlashMapState()
	d.mapCache[req.Key] = slashMapCacheEntry{
		URL:       "https://tiles.example/pregenerated/home.png",
		ExpiresAt: time.Now().Add(-time.Second),
	}
	d.mapCache["failed"] = slashMapCacheEntry{
		RetryAfter: time.Now().Add(-time.Second),
	}

	if url, ok := d.slashMapCacheStatus(req); ok || url != "" {
		t.Fatalf("expired success entry should miss, got (%q, %v)", url, ok)
	}
	d.mapMu.Lock()
	_, hasSuccess := d.mapCache[req.Key]
	_, hasFailure := d.mapCache["failed"]
	d.mapMu.Unlock()
	if hasSuccess || hasFailure {
		t.Fatalf("expected expired cache entries to be pruned, success=%v failure=%v", hasSuccess, hasFailure)
	}
}

func TestPrepareSlashMapPatchSkipsStaleRevision(t *testing.T) {
	d := &Discord{}
	target := channelRenderTarget("chan-1", "msg-1")
	embedA := &discordgo.MessageEmbed{Title: "Area A"}
	embedB := &discordgo.MessageEmbed{Title: "Area B"}

	revA := d.captureSlashRender(target, &slashMapRequest{Key: "area:a"}, embedA)
	revB := d.captureSlashRender(target, &slashMapRequest{Key: "area:b"}, embedB)

	if _, ok := d.prepareSlashMapPatch(target, "area:a", revA, "https://tiles.example/a.png"); ok {
		t.Fatal("expected stale map patch to be rejected")
	}
	patch, ok := d.prepareSlashMapPatch(target, "area:b", revB, "https://tiles.example/b.png")
	if !ok {
		t.Fatal("expected latest map patch to be accepted")
	}
	if patch == nil || patch.ChannelEdit == nil || patch.ChannelEdit.Embeds == nil || len(*patch.ChannelEdit.Embeds) != 1 {
		t.Fatalf("unexpected edit payload: %#v", patch)
	}
	if got := (*patch.ChannelEdit.Embeds)[0].Image.URL; got != "https://tiles.example/b.png" {
		t.Fatalf("patched image=%q, want %q", got, "https://tiles.example/b.png")
	}
}

func TestPrepareSlashMapPatchSupportsOriginalInteractionTarget(t *testing.T) {
	d := &Discord{}
	msg := &discordgo.Message{ID: "msg-1"}
	target := originalInteractionRenderTarget(slashTestInteraction("user-1"), msg)
	rev := d.captureSlashRender(target, &slashMapRequest{Key: "location:51.5,-0.12"}, &discordgo.MessageEmbed{Title: "Confirm location"})

	patch, ok := d.prepareSlashMapPatch(target, "location:51.5,-0.12", rev, "https://tiles.example/pregenerated/location.png")
	if !ok {
		t.Fatal("expected original interaction patch to be accepted")
	}
	if patch == nil || patch.WebhookEdit == nil || patch.ChannelEdit != nil {
		t.Fatalf("unexpected original interaction patch: %#v", patch)
	}
	if got := (*patch.WebhookEdit.Embeds)[0].Image.URL; got != "https://tiles.example/pregenerated/location.png" {
		t.Fatalf("patched image=%q, want pregenerated location URL", got)
	}
}

func TestRegisterSuccessfulSlashRenderSkipsFailedPrimaryEdit(t *testing.T) {
	d := &Discord{}
	ok := d.registerSuccessfulSlashRender(nil, &discordgo.Message{ChannelID: "chan-1", ID: "msg-1"}, errSlashMapStubFailure, &slashMapRequest{Key: "area:a"}, &discordgo.MessageEmbed{Title: "Area A"})
	if ok {
		t.Fatal("expected failed primary edit not to register render state")
	}
	d.renderMu.Lock()
	renderCount := len(d.renderState)
	d.renderMu.Unlock()
	d.mapMu.Lock()
	jobCount := len(d.mapJobs)
	d.mapMu.Unlock()
	if renderCount != 0 || jobCount != 0 {
		t.Fatalf("expected no render state or jobs after failed edit, renders=%d jobs=%d", renderCount, jobCount)
	}
}

func TestBuildLocationConfirmPayloadStateUsesCacheAndMissesWithoutImage(t *testing.T) {
	d := &Discord{manager: &Manager{
		cfg: config.New(map[string]any{
			"geocoding": map[string]any{
				"staticProvider":    "tileservercache",
				"staticProviderURL": "https://tiles.example",
			},
		}),
	}}
	embed, _, mapReq := d.buildLocationConfirmPayloadState(slashTestInteraction("user-1"), 51.5, -0.12, "")
	if mapReq == nil {
		t.Fatal("expected location map request")
	}
	if embed.Image != nil {
		t.Fatalf("expected uncached location preview to omit image, got %#v", embed.Image)
	}

	d.ensureSlashMapState()
	d.mapCache[mapReq.Key] = slashMapCacheEntry{
		URL:       "https://tiles.example/pregenerated/location.png",
		ExpiresAt: time.Now().Add(slashLocationMapTTL),
	}
	embed, _, _ = d.buildLocationConfirmPayloadState(slashTestInteraction("user-1"), 51.5, -0.12, "")
	if embed.Image == nil || embed.Image.URL != "https://tiles.example/pregenerated/location.png" {
		t.Fatalf("expected cached location preview image, got %#v", embed.Image)
	}
}

func TestClearSlashRenderMessageRemovesOriginalInteractionRender(t *testing.T) {
	d := &Discord{}
	msg := &discordgo.Message{ID: "msg-1"}
	target := originalInteractionRenderTarget(slashTestInteraction("user-1"), msg)
	rev := d.captureSlashRender(target, &slashMapRequest{Key: "location:51.5,-0.12"}, &discordgo.MessageEmbed{Title: "Confirm location"})
	if rev == 0 {
		t.Fatal("expected preview render state to be captured")
	}
	d.clearSlashRenderMessage(msg)
	if _, ok := d.prepareSlashMapPatch(target, "location:51.5,-0.12", rev, "https://tiles.example/pregenerated/location.png"); ok {
		t.Fatal("expected cleared preview render state to reject late patch")
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

var errSlashMapStubFailure = &slashMapStubError{text: "slash map stub failure"}

type slashMapStubError struct {
	text string
}

func (e *slashMapStubError) Error() string {
	return e.text
}

type slashMapStubGenerator struct {
	mu      sync.Mutex
	calls   int
	started chan string
	release chan struct{}
	urls    map[string]string
	errs    map[string]error
}

func (g *slashMapStubGenerator) Generate(req *slashMapRequest) (string, error) {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	if g.started != nil {
		g.started <- req.Key
	}
	if g.release != nil {
		<-g.release
	}
	if g.errs != nil {
		if err := g.errs[req.Key]; err != nil {
			return "", err
		}
	}
	if g.urls != nil {
		return g.urls[req.Key], nil
	}
	return "", nil
}

func (g *slashMapStubGenerator) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
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
			AppID: "app-1",
			Token: "token-1",
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
