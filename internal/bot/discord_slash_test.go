package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/config"
	"poraclego/internal/dts"
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
