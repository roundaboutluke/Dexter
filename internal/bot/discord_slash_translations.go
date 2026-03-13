package bot

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/i18n"
	"poraclego/internal/logging"
)

type slashLocalizationTarget struct {
	poracle string
	discord discordgo.Locale
	tr      *i18n.Translator
}

var slashDayNames = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

func (d *Discord) configuredDefaultLocale() string {
	if d != nil && d.manager != nil && d.manager.cfg != nil {
		if locale, ok := d.manager.cfg.GetString("general.locale"); ok && strings.TrimSpace(locale) != "" {
			return strings.TrimSpace(locale)
		}
	}
	return "en"
}

func (d *Discord) slashInteractionLanguage(i *discordgo.InteractionCreate) string {
	userID, _ := slashUser(i)
	return d.userLanguage(userID)
}

func (d *Discord) resolvedHumanLanguage(human map[string]any) string {
	if human != nil {
		if lang, ok := human["language"].(string); ok && strings.TrimSpace(lang) != "" {
			return strings.TrimSpace(lang)
		}
	}
	return d.configuredDefaultLocale()
}

func (d *Discord) slashInteractionTranslator(i *discordgo.InteractionCreate) *i18n.Translator {
	return d.slashTranslator(d.slashInteractionLanguage(i))
}

func (d *Discord) slashText(i *discordgo.InteractionCreate, key string) string {
	return d.slashInteractionTranslator(i).Translate(key, false)
}

func (d *Discord) slashTextf(i *discordgo.InteractionCreate, key string, args ...any) string {
	return d.slashInteractionTranslator(i).TranslateFormat(key, args...)
}

func (d *Discord) slashExpiredText(i *discordgo.InteractionCreate) string {
	return d.slashText(i, "Slash command expired. Please run the command again.")
}

func localizedProfileLabel(tr *i18n.Translator, profileNo int) string {
	if profileNo <= 0 {
		return translateOrDefault(tr, "Profile")
	}
	if tr == nil {
		return fmt.Sprintf("Profile %d", profileNo)
	}
	return tr.TranslateFormat("Profile {0}", profileNo)
}

func localizedProfileName(tr *i18n.Translator, row map[string]any) string {
	if row == nil {
		return localizedProfileLabel(tr, 0)
	}
	profileNo := toInt(row["profile_no"], 0)
	name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
	if name != "" {
		return name
	}
	return localizedProfileLabel(tr, profileNo)
}

func localizedProfileDisplayName(tr *i18n.Translator, row map[string]any) string {
	if row == nil {
		return localizedProfileLabel(tr, 0)
	}
	profileNo := toInt(row["profile_no"], 0)
	name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
	if name == "" {
		return localizedProfileLabel(tr, profileNo)
	}
	if profileNo > 0 {
		return fmt.Sprintf("%s (P%d)", name, profileNo)
	}
	return name
}

func localizedDayLabel(tr *i18n.Translator, day int) string {
	if day < 1 || day > len(slashDayNames) {
		return ""
	}
	key := slashDayNames[day-1]
	if tr == nil {
		return key
	}
	return tr.Translate(key, false)
}

func localizedDayList(tr *i18n.Translator, days []int) string {
	labels := make([]string, 0, len(days))
	for _, day := range days {
		label := localizedDayLabel(tr, day)
		if label != "" {
			labels = append(labels, label)
		}
	}
	return strings.Join(labels, ", ")
}

func localizedScheduleEntryLabel(tr *i18n.Translator, entry scheduleEntry) string {
	day := localizedDayLabel(tr, entry.Day)
	start := fmt.Sprintf("%02d:%02d", entry.StartMin/60, entry.StartMin%60)
	if entry.Legacy || entry.EndMin <= 0 {
		return fmt.Sprintf("%s %s (%s)", day, start, translateOrDefault(tr, "switch"))
	}
	end := fmt.Sprintf("%02d:%02d", entry.EndMin/60, entry.EndMin%60)
	return fmt.Sprintf("%s %s-%s", day, start, end)
}

func localizedScheduleRangeLabel(tr *i18n.Translator, day, startMin, endMin int) string {
	return fmt.Sprintf("%s %02d:%02d-%02d:%02d", localizedDayLabel(tr, day), startMin/60, startMin%60, endMin/60, endMin%60)
}

func localizedScheduleTime(tr *i18n.Translator, t time.Time) string {
	return fmt.Sprintf("%s %02d:%02d", localizedDayLabel(tr, slashISOWeekday(t.Weekday())), t.Hour(), t.Minute())
}

func slashISOWeekday(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}

func translateOrDefault(tr *i18n.Translator, key string) string {
	if tr == nil {
		return key
	}
	return tr.Translate(key, false)
}

func (d *Discord) slashLocalizationTargets() []slashLocalizationTarget {
	if d == nil || d.manager == nil || d.manager.i18n == nil {
		return nil
	}
	langs := d.slashLocalizationLanguages()
	targets := make([]slashLocalizationTarget, 0, len(langs))
	for _, lang := range langs {
		locale, ok := poracleDiscordLocale(lang)
		if !ok {
			continue
		}
		targets = append(targets, slashLocalizationTarget{
			poracle: lang,
			discord: locale,
			tr:      d.manager.i18n.Translator(lang),
		})
	}
	return targets
}

func (d *Discord) slashLocalizationLanguages() []string {
	seen := map[string]bool{}
	langs := []string{}
	if d != nil && d.manager != nil && d.manager.cfg != nil {
		if raw, ok := d.manager.cfg.Get("general.availableLanguages"); ok {
			if data, ok := raw.(map[string]any); ok {
				for key := range data {
					locale := strings.ToLower(strings.TrimSpace(key))
					if locale == "" || seen[locale] {
						continue
					}
					seen[locale] = true
					langs = append(langs, locale)
				}
			}
		}
	}
	defaultLocale := strings.ToLower(strings.TrimSpace(d.configuredDefaultLocale()))
	if defaultLocale != "" && !seen[defaultLocale] {
		langs = append(langs, defaultLocale)
	}
	sort.Strings(langs)
	return langs
}

func poracleDiscordLocale(locale string) (discordgo.Locale, bool) {
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "de":
		return discordgo.German, true
	case "fr":
		return discordgo.French, true
	case "it":
		return discordgo.Italian, true
	case "nb-no":
		return discordgo.Norwegian, true
	case "pl":
		return discordgo.Polish, true
	case "ru":
		return discordgo.Russian, true
	case "se":
		return discordgo.Swedish, true
	default:
		return discordgo.Unknown, false
	}
}

func validLocalizedSlashName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 32 {
		return false
	}
	if name != strings.ToLower(name) {
		return false
	}
	for _, r := range name {
		switch r {
		case '-', '_', '\'':
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			continue
		}
		return false
	}
	return true
}

func validLocalizedText(value string, limit int) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if limit <= 0 {
		return true
	}
	return utf8.RuneCountInString(value) <= limit
}

func (d *Discord) logSlashLocalizationSkip(commandName, field, locale, reason, value string) {
	logger := logging.Get().Discord
	if logger == nil {
		return
	}
	logger.Warnf("Slash localization skipped: command=%s field=%s locale=%s reason=%s value=%q", commandName, field, locale, reason, value)
}

func (d *Discord) localizedSlashCommands(commands []*discordgo.ApplicationCommand) []*discordgo.ApplicationCommand {
	targets := d.slashLocalizationTargets()
	if len(targets) == 0 {
		return commands
	}
	used := map[discordgo.Locale]map[string]bool{}
	for _, target := range targets {
		used[target.discord] = map[string]bool{}
		for _, cmd := range commands {
			if cmd == nil {
				continue
			}
			used[target.discord][strings.ToLower(cmd.Name)] = true
		}
	}
	for _, cmd := range commands {
		if cmd == nil {
			continue
		}
		d.applyCommandLocalizations(cmd, targets, used)
	}
	return commands
}

func (d *Discord) applyCommandLocalizations(cmd *discordgo.ApplicationCommand, targets []slashLocalizationTarget, used map[discordgo.Locale]map[string]bool) {
	if cmd == nil {
		return
	}
	for _, target := range targets {
		if target.tr == nil {
			continue
		}
		if translated := target.tr.Translate(cmd.Name, false); translated != cmd.Name {
			nameKey := strings.ToLower(translated)
			switch {
			case !validLocalizedSlashName(translated):
				d.logSlashLocalizationSkip(cmd.Name, "command.name", string(target.discord), "invalid_name", translated)
			case used[target.discord][nameKey]:
				d.logSlashLocalizationSkip(cmd.Name, "command.name", string(target.discord), "duplicate_name", translated)
			default:
				if cmd.NameLocalizations == nil {
					cmd.NameLocalizations = &map[discordgo.Locale]string{}
				}
				(*cmd.NameLocalizations)[target.discord] = translated
				used[target.discord][nameKey] = true
			}
		}
		if translated := target.tr.Translate(cmd.Description, false); translated != cmd.Description {
			if validLocalizedText(translated, 100) {
				if cmd.DescriptionLocalizations == nil {
					cmd.DescriptionLocalizations = &map[discordgo.Locale]string{}
				}
				(*cmd.DescriptionLocalizations)[target.discord] = translated
			} else {
				d.logSlashLocalizationSkip(cmd.Name, "command.description", string(target.discord), "invalid_description", translated)
			}
		}
	}
	d.applyOptionLocalizations(cmd.Name, cmd.Options, targets)
}

func (d *Discord) applyOptionLocalizations(commandName string, options []*discordgo.ApplicationCommandOption, targets []slashLocalizationTarget) {
	if len(options) == 0 {
		return
	}
	used := map[discordgo.Locale]map[string]bool{}
	for _, target := range targets {
		used[target.discord] = map[string]bool{}
		for _, option := range options {
			if option == nil {
				continue
			}
			used[target.discord][strings.ToLower(option.Name)] = true
		}
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		for _, target := range targets {
			if target.tr == nil {
				continue
			}
			if translated := target.tr.Translate(option.Name, false); translated != option.Name {
				nameKey := strings.ToLower(translated)
				switch {
				case !validLocalizedSlashName(translated):
					d.logSlashLocalizationSkip(commandName, "option.name:"+option.Name, string(target.discord), "invalid_name", translated)
				case used[target.discord][nameKey]:
					d.logSlashLocalizationSkip(commandName, "option.name:"+option.Name, string(target.discord), "duplicate_name", translated)
				default:
					if option.NameLocalizations == nil {
						option.NameLocalizations = map[discordgo.Locale]string{}
					}
					option.NameLocalizations[target.discord] = translated
					used[target.discord][nameKey] = true
				}
			}
			if translated := target.tr.Translate(option.Description, false); translated != option.Description {
				if validLocalizedText(translated, 100) {
					if option.DescriptionLocalizations == nil {
						option.DescriptionLocalizations = map[discordgo.Locale]string{}
					}
					option.DescriptionLocalizations[target.discord] = translated
				} else {
					d.logSlashLocalizationSkip(commandName, "option.description:"+option.Name, string(target.discord), "invalid_description", translated)
				}
			}
			for _, choice := range option.Choices {
				if choice == nil {
					continue
				}
				translated := target.tr.Translate(choice.Name, false)
				if translated == choice.Name {
					continue
				}
				if !validLocalizedText(translated, 100) {
					d.logSlashLocalizationSkip(commandName, "choice.name:"+option.Name, string(target.discord), "invalid_choice_name", translated)
					continue
				}
				if choice.NameLocalizations == nil {
					choice.NameLocalizations = map[discordgo.Locale]string{}
				}
				choice.NameLocalizations[target.discord] = translated
			}
		}
		d.applyOptionLocalizations(commandName, option.Options, targets)
	}
}
