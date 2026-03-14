package bot

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
)

func TestShippedDeFrSlashNameKeysCoverCommandInventory(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"de": true, "fr": true},
		},
	})
	factory := i18n.NewFactory(shippedLocaleRoot(t), cfg)
	d := &Discord{manager: &Manager{cfg: cfg, i18n: factory}}
	commands := d.slashCommandDefinitions()

	targets := map[string]discordgo.Locale{
		"de": discordgo.German,
		"fr": discordgo.French,
	}

	for lang, locale := range targets {
		tr := factory.Translator(lang)
		for _, cmd := range commands {
			if cmd == nil {
				continue
			}
			assertSlashNameKeyCoverage(t, tr, locale, "command", cmd.Name, cmd.NameLocalizations)
			walkSlashOptions(cmd.Options, func(opt *discordgo.ApplicationCommandOption) {
				assertSlashOptionNameKeyCoverage(t, tr, locale, opt.Name, opt.NameLocalizations)
			})
		}
	}
}

func TestShippedDeFrSlashMetadataLocalizesDescriptionsAndChoices(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"de": true, "fr": true},
		},
	})
	factory := i18n.NewFactory(shippedLocaleRoot(t), cfg)
	d := &Discord{manager: &Manager{cfg: cfg, i18n: factory}}
	commands := d.slashCommandDefinitions()

	targets := map[string]discordgo.Locale{
		"de": discordgo.German,
		"fr": discordgo.French,
	}

	for lang, locale := range targets {
		tr := factory.Translator(lang)
		for _, cmd := range commands {
			if cmd == nil {
				continue
			}
			if cmd.DescriptionLocalizations == nil || (*cmd.DescriptionLocalizations)[locale] == "" {
				t.Fatalf("locale=%q command=%q missing description localization", lang, cmd.Name)
			}
			walkSlashOptions(cmd.Options, func(opt *discordgo.ApplicationCommandOption) {
				if opt.DescriptionLocalizations == nil || opt.DescriptionLocalizations[locale] == "" {
					t.Fatalf("locale=%q option=%q missing description localization", lang, opt.Name)
				}
				for _, choice := range opt.Choices {
					if choice == nil {
						continue
					}
					translated := tr.Translate(choice.Name, false)
					if translated == choice.Name {
						if choiceMayRemainUnchanged(choice.Name) {
							continue
						}
						t.Fatalf("locale=%q choice=%q missing localization", lang, choice.Name)
					}
					if choice.NameLocalizations == nil || choice.NameLocalizations[locale] != translated {
						t.Fatalf("locale=%q choice=%q missing built localization", lang, choice.Name)
					}
				}
			})
		}
	}
}

func TestShippedDeFrSlashRuntimeKeysCovered(t *testing.T) {
	keys := []string{
		"What do you want to track?",
		"Track a specific monster or everything?",
		"Track raid boss, level, or everything?",
		"Track an egg level?",
		"Track a max battle boss, level, or everything?",
		"Quest filters",
		"Invasion filters",
		"Search Pokemon",
		"Raid boss or level",
		"Select egg level",
		"Select a gym team",
		"Select a weather condition",
		"Select a lure type",
		"What do you want to look up?",
		"Ready to run `{0}`",
		"New Pokemon Alert:",
		"New Pokestop Event Alert:",
		"New Gym Alert:",
		"New Fort Alert:",
		"New Nest Alert:",
		"New Weather Alert:",
		"Confirm Command:",
		"Canceled.",
		"Weather info",
		"Coordinates (lat,lon)",
		"Please enter text to translate.",
		"Please enter coordinates (lat,lon).",
		"Please enter a Pokemon name or ID.",
		"Please pick a specific Pokemon.",
		"No Pokemon matched that search.",
		"Select a Pokemon",
		"Please enter a raid boss name or level.",
		"Please enter quest filters (e.g. reward:items).",
		"Please enter invasion filters (e.g. grunt type).",
		"Please set your location in /profile, or provide a location.",
		"Please pick a weather condition.",
		"Set a saved location in `/profile`, or provide a location with `/weather condition:<condition> location:<place>`.",
		"Current profile: {0}",
		"All profiles",
		"remove all",
		"Target is not registered.",
		"No command to run.",
		"That command is disabled.",
		"Slash command expired. Please run the command again.",
		"Please pick a tracking type.",
		"Please pick a tracking entry.",
		"That tracking entry could not be parsed.",
		"Tracking type changed; please clear the tracking selection and pick again.",
		"That tracking entry could not be removed.",
		"That filter action is no longer available.",
		"That filter action has expired.",
		"That filter action belongs to another user.",
		"No filters were changed.",
		"No filters found in {0}.",
		"Filter not found in {0}.",
		"Filter Added",
		"Filters Added",
		"Filter Removed",
		"Filters Removed",
		"Filter Restored",
		"Filters Restored",
		"Filter",
		"Type",
		"Remove Filter",
		"Remove Filters",
		"Restore Filter",
		"Restore Filters",
		"Saved to {0}.",
		"Removed from {0}.",
		"Restored in {0}.",
		"Rocket",
		"Pokestop Event",
		"And {0} more...",
		"Weather info: use your saved location or enter coordinates?",
		"Use saved location",
		"Enter coordinates",
		"Please pick something to look up.",
		"You're not tracking anything in any profile.",
		"Filter summary across all profiles.",
		"Filter summary across all profiles. Current profile: {0}.",
		"Use `/filters show profile:<profile>` for full details.",
		"Viewing filters for {0}.",
		"Your alerts are currently",
		"Your location is currently set to",
		"You have not set a location yet",
		"Filters for {0} are attached as a file:",
	}

	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"de": true, "fr": true},
		},
	})
	factory := i18n.NewFactory(shippedLocaleRoot(t), cfg)

	for _, lang := range []string{"de", "fr"} {
		tr := factory.Translator(lang)
		for _, key := range keys {
			if got := tr.Translate(key, false); got == key {
				t.Fatalf("locale=%q key=%q fell back to english", lang, key)
			}
		}
	}
}

func TestShippedDeFrProfileAndSchedulerSmoke(t *testing.T) {
	type expectation struct {
		lang       string
		dayLabel   string
		modalTitle string
		modalLabel string
	}

	for _, tc := range []expectation{
		{lang: "de", dayLabel: "Montag", modalTitle: "Standort festlegen", modalLabel: "Adresse oder Koordinaten"},
		{lang: "fr", dayLabel: "Lundi", modalTitle: "Définir la position", modalLabel: "Adresse ou coordonnées"},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			env := newSlashMutationTestEnv(t, map[string][]map[string]any{
				"humans": {{
					"id":       "user-1",
					"language": tc.lang,
				}},
				"profiles": {{
					"id":         "user-1",
					"profile_no": 1,
					"name":       "Home",
				}},
			}, 0)
			cfg := config.New(map[string]any{
				"general": map[string]any{
					"locale":             "en",
					"availableLanguages": map[string]any{"de": true, "fr": true},
				},
			})
			env.discord.manager.cfg = cfg
			env.discord.manager.i18n = i18n.NewFactory(shippedLocaleRoot(t), cfg)

			title, label, _ := env.discord.profileLocationModalText(slashTestInteraction("user-1"))
			if title != tc.modalTitle {
				t.Fatalf("locale=%q modal title=%q, want %q", tc.lang, title, tc.modalTitle)
			}
			if label != tc.modalLabel {
				t.Fatalf("locale=%q modal label=%q, want %q", tc.lang, label, tc.modalLabel)
			}

			i := &discordgo.InteractionCreate{
				Interaction: &discordgo.Interaction{
					Member: &discordgo.Member{
						User: &discordgo.User{ID: "user-1", Username: "tester"},
					},
				},
			}
			_, components, errText := env.discord.buildProfileScheduleDayPayload(i, "1")
			if errText != "" {
				t.Fatalf("locale=%q buildProfileScheduleDayPayload errText=%q", tc.lang, errText)
			}
			row, ok := components[0].(discordgo.ActionsRow)
			if !ok || len(row.Components) == 0 {
				t.Fatalf("locale=%q expected schedule select row", tc.lang)
			}
			menu, ok := row.Components[0].(discordgo.SelectMenu)
			if !ok || len(menu.Options) == 0 {
				t.Fatalf("locale=%q expected schedule select menu", tc.lang)
			}
			if got := menu.Options[0].Label; got != tc.dayLabel {
				t.Fatalf("locale=%q first day label=%q, want %q", tc.lang, got, tc.dayLabel)
			}
		})
	}
}

func TestShippedDeFrTrackedSummarySmoke(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"de": true, "fr": true},
		},
	})
	factory := i18n.NewFactory(shippedLocaleRoot(t), cfg)

	for _, tc := range []struct {
		lang string
		want string
	}{
		{lang: "de", want: "Du verfolgst in keinem Profil etwas."},
		{lang: "fr", want: "Vous ne suivez rien dans aucun profil."},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			d := &Discord{manager: &Manager{cfg: cfg, i18n: factory}}
			selection := slashProfileSelection{
				Mode: slashProfileScopeAll,
				Human: map[string]any{
					"enabled": 1,
				},
			}
			got := d.buildSlashTrackedAllProfilesSummary(selection, factory.Translator(tc.lang))
			if got != tc.want {
				t.Fatalf("locale=%q tracked summary=%q, want %q", tc.lang, got, tc.want)
			}
		})
	}
}

func TestShippedDeFrCanonicalBrandTermsRemainCanonical(t *testing.T) {
	cfg := config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"de": true, "fr": true},
		},
	})
	factory := i18n.NewFactory(shippedLocaleRoot(t), cfg)

	for _, lang := range []string{"de", "fr"} {
		tr := factory.Translator(lang)
		if got := tr.Translate("Pokemon", false); got != "Pokemon" {
			t.Fatalf("locale=%q Pokemon=%q, want Pokemon", lang, got)
		}
		if got := tr.Translate("Pokestop", false); got != "Pokestop" {
			t.Fatalf("locale=%q Pokestop=%q, want Pokestop", lang, got)
		}
		if got := tr.Translate("Track Team Rocket invasions", false); !containsSubstring(got, "Team Rocket") {
			t.Fatalf("locale=%q Team Rocket phrase=%q, want canonical Team Rocket", lang, got)
		}
		if got := tr.Translate("RSVP matching", false); !containsSubstring(got, "RSVP") {
			t.Fatalf("locale=%q RSVP phrase=%q, want canonical RSVP", lang, got)
		}
	}
}

func assertSlashNameKeyCoverage(t *testing.T, tr *i18n.Translator, locale discordgo.Locale, kind, name string, localizations *map[discordgo.Locale]string) {
	t.Helper()
	key := slashLocalizationKey(kind, name)
	got := tr.Translate(key, false)
	if got == key {
		t.Fatalf("locale=%q %s %q missing dedicated slash key", locale, kind, name)
	}
	if !validLocalizedSlashName(got) {
		t.Fatalf("locale=%q %s %q produced invalid slash name %q", locale, kind, name, got)
	}
	if got != name {
		if localizations == nil || (*localizations)[locale] != got {
			t.Fatalf("locale=%q %s %q missing built localization %q", locale, kind, name, got)
		}
	}
}

func assertSlashOptionNameKeyCoverage(t *testing.T, tr *i18n.Translator, locale discordgo.Locale, name string, localizations map[discordgo.Locale]string) {
	t.Helper()
	key := slashLocalizationKey("option", name)
	got := tr.Translate(key, false)
	if got == key {
		t.Fatalf("locale=%q option %q missing dedicated slash key", locale, name)
	}
	if !validLocalizedSlashName(got) {
		t.Fatalf("locale=%q option %q produced invalid slash name %q", locale, name, got)
	}
	if got != name && localizations[locale] != got {
		t.Fatalf("locale=%q option %q missing built localization %q", locale, name, got)
	}
}

func walkSlashOptions(options []*discordgo.ApplicationCommandOption, fn func(*discordgo.ApplicationCommandOption)) {
	for _, opt := range options {
		if opt == nil {
			continue
		}
		fn(opt)
		walkSlashOptions(opt.Options, fn)
	}
}

func choiceMayRemainUnchanged(name string) bool {
	switch name {
	case "Pokemon", "pokemon", "Pokestop", "raid", "egg", "maxbattle", "invasion", "rocket", "pokestop-event", "quest", "gym", "weather", "lure", "nest", "fort", "xxs", "xs", "m", "xl", "xxl":
		return true
	default:
		return false
	}
}

func containsSubstring(value, needle string) bool {
	return strings.Contains(value, needle)
}
