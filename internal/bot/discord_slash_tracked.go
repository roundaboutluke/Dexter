package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/command"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
)

func (d *Discord) handleSlashTracked(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondDeferredEphemeral(s, i)
	reply := d.buildSlashTrackedReply(s, i)
	if reply == "" {
		reply = d.slashText(i, "Done.")
	}
	if d.sendSpecialSlashReply(s, i, reply) {
		return
	}
	d.respondEditMessage(s, i, reply, nil)
}

func (d *Discord) handleSlashStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.executeSlashLineDeferred(s, i, "start")
}

func (d *Discord) handleSlashStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.executeSlashLineDeferred(s, i, "stop")
}

func (d *Discord) handleSlashHelp(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Use a dedicated DTS help template for slash users, so legacy `!help` can stay focused on legacy commands.
	// Falls back to default `help` behaviour if the template does not exist.
	d.respondDeferredEphemeral(s, i)
	options := slashOptions(i.ApplicationCommandData())
	commandName, _ := optionString(options, "command")
	commandName = strings.ToLower(strings.TrimSpace(commandName))
	line := "help slash"
	if commandName != "" {
		line = "help " + commandName
	}
	reply := d.buildSlashReply(s, i, line)
	if strings.TrimSpace(reply) == "🙅" {
		if commandName != "" {
			reply = d.slashTextf(i, "No dedicated help is available for `{0}` yet. Try `/help`.", commandName)
		} else {
			reply = d.buildSlashReply(s, i, "help")
		}
	}
	if reply == "" {
		reply = d.slashText(i, "Done.")
	}
	if d.sendSpecialSlashReply(s, i, reply) {
		return
	}
	d.respondEditMessage(s, i, reply, nil)
}

func (d *Discord) handleSlashInfo(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	data := i.ApplicationCommandData()
	options := slashOptions(data)

	infoType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(infoType) == "" {
		d.respondWithInfoTypeSelect(s, i)
		return
	}
	infoType = strings.ToLower(strings.TrimSpace(infoType))

	switch infoType {
	case "pokemon":
		pokemon, _ := optionString(options, "pokemon")
		pokemon = strings.TrimSpace(pokemon)
		if pokemon == "" {
			d.startSlashGuide(i, "info", "info-pokemon")
			d.respondWithMonsterSearch(s, i)
			return
		}
		if strings.EqualFold(pokemon, "everything") {
			d.respondEphemeral(s, i, tr.Translate("Please pick a specific Pokemon (`Everything` is not supported here).", false))
			return
		}
		d.executeSlashLineDeferred(s, i, "info "+pokemon)
	case "moves", "items", "rarity", "shiny":
		d.executeSlashLineDeferred(s, i, "info "+infoType)
	case "weather":
		d.respondWithButtons(s, i, tr.Translate("Weather info: use your saved location or enter coordinates?", false), []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashInfoWeatherUseSaved, Label: tr.Translate("Use saved location", false), Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: slashInfoWeatherEnterCoordinates, Label: tr.Translate("Enter coordinates", false), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashInfoCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
		})
	case "translate":
		d.respondWithModal(s, i, slashInfoTranslateModal, tr.Translate("Translate", false), tr.Translate("Text", false), "Bonjour")
	default:
		d.executeSlashLineDeferred(s, i, "info")
	}
}

func (d *Discord) handleSlashInfoTypeChoice(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
	tr := d.slashInteractionTranslator(i)
	infoType := strings.ToLower(strings.TrimSpace(value))
	if infoType == "" {
		d.respondEphemeral(s, i, tr.Translate("Please pick something to look up.", false))
		return
	}
	d.logSlashUX(i, "info", "guide_choice", infoType)
	switch infoType {
	case "pokemon":
		d.startSlashGuide(i, "info", "info-pokemon")
		d.respondWithMonsterSearch(s, i)
	case "moves", "items", "rarity", "shiny":
		d.executeSlashLineDeferred(s, i, "info "+infoType)
	case "weather":
		d.respondWithButtons(s, i, tr.Translate("Weather info: use your saved location or enter coordinates?", false), []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashInfoWeatherUseSaved, Label: tr.Translate("Use saved location", false), Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: slashInfoWeatherEnterCoordinates, Label: tr.Translate("Enter coordinates", false), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashInfoCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
		})
	case "translate":
		d.respondWithModal(s, i, slashInfoTranslateModal, tr.Translate("Translate", false), tr.Translate("Text", false), "Bonjour")
	default:
		d.executeSlashLineDeferred(s, i, "info "+infoType)
	}
}

func slashCommandAllowed(ctx *command.Context, commandName string) bool {
	if ctx == nil || ctx.Config == nil {
		return true
	}
	disabled, _ := ctx.Config.GetStringSlice("general.disabledCommands")
	for _, entry := range disabled {
		if strings.EqualFold(entry, commandName) {
			return false
		}
	}
	if ctx.Platform != "discord" {
		return true
	}
	raw, ok := ctx.Config.Get("discord.commandSecurity")
	if !ok {
		return true
	}
	security, ok := raw.(map[string]any)
	if !ok {
		return true
	}
	list, ok := security[commandName]
	if !ok {
		return true
	}
	switch values := list.(type) {
	case []any:
		for _, item := range values {
			value := strings.TrimSpace(fmt.Sprintf("%v", item))
			if value == "" {
				continue
			}
			if value == ctx.UserID {
				return true
			}
			for _, role := range ctx.Roles {
				if value == role {
					return true
				}
			}
		}
		return false
	case []string:
		for _, value := range values {
			if value == ctx.UserID {
				return true
			}
			for _, role := range ctx.Roles {
				if value == role {
					return true
				}
			}
		}
		return false
	default:
		return true
	}
}

func (d *Discord) slashTranslator(locale string) *i18n.Translator {
	if d != nil && d.manager != nil && d.manager.i18n != nil {
		return d.manager.i18n.Translator(locale)
	}
	tr, _ := i18n.NewTranslator(".", locale)
	return tr
}

func (d *Discord) slashTrackedListingContext() tracking.ListingContext {
	if d == nil || d.manager == nil {
		return tracking.ListingContext{}
	}
	return tracking.ListingContext{
		Config:   d.manager.cfg,
		Query:    d.manager.query,
		Data:     d.manager.data,
		GymNames: d.manager.scanner,
	}
}

func (d *Discord) slashTrackedAreaText(tr *i18n.Translator, human map[string]any) string {
	fences := []geofence.Fence(nil)
	if d != nil && d.manager != nil && d.manager.fences != nil {
		fences = d.manager.fences.Fences
	}
	return tracking.AreaTextWithFallback(tr, fences, tracking.ParseAreaList(human))
}

func (d *Discord) slashTrackedBlockedAlerts(human map[string]any) []string {
	return tracking.BlockedAlerts(human)
}

func (d *Discord) slashTrackedCategoryDetails(userID string, profileNo int, blocked []string, tr *i18n.Translator) string {
	return tracking.CategoryDetails(d.slashTrackedListingContext(), tr, userID, profileNo, blocked)
}

func (d *Discord) slashTrackedProfileCounts(userID string, blocked []string) map[int]map[string]int {
	return tracking.ProfileCounts(d.slashTrackedListingContext(), userID, blocked)
}

func (d *Discord) buildSlashTrackedAllProfilesSummary(selection slashProfileSelection, tr *i18n.Translator) string {
	blocked := d.slashTrackedBlockedAlerts(selection.Human)
	countsByProfile := d.slashTrackedProfileCounts(selection.UserID, blocked)
	lines := []string{}
	for _, row := range selection.Profiles {
		profileNo := toInt(row["profile_no"], 0)
		counts := countsByProfile[profileNo]
		if len(counts) == 0 {
			continue
		}
		label := localizedProfileDisplayName(tr, row)
		if profileNo == selection.EffectiveNo {
			label += " (" + translateOrDefault(tr, "current") + ")"
		}
		parts := []string{}
		for _, spec := range tracking.CountSpecs() {
			if count := counts[spec.Key]; count > 0 {
				parts = append(parts, tracking.PluralCount(count, spec.Singular, spec.Plural))
			}
		}
		if len(parts) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, strings.Join(parts, ", ")))
	}
	if len(lines) == 0 {
		return translateOrDefault(tr, "You're not tracking anything in any profile.")
	}
	header := translateOrDefault(tr, "Tracking summary across all profiles.")
	if row := profileRowByNo(selection.Profiles, selection.EffectiveNo); row != nil {
		header = tr.TranslateFormat("Tracking summary across all profiles. Current profile: {0}.", localizedProfileDisplayName(tr, row))
	}
	return header + "\n\n" + strings.Join(lines, "\n") + "\n\n" + translateOrDefault(tr, "Use `/tracked profile:<profile>` for full details.")
}

func (d *Discord) buildSlashTrackedReply(s *discordgo.Session, i *discordgo.InteractionCreate) string {
	if d == nil || d.manager == nil || d.manager.query == nil {
		return d.slashText(i, "Target is not registered.")
	}
	ctx := d.buildSlashContext(s, i)
	if ctx == nil {
		return d.slashText(i, "Target is not registered.")
	}
	lang := d.userLanguage(ctx.UserID)
	tr := d.slashTranslator(lang)
	if ctx.Config != nil {
		if disabled, ok := ctx.Config.GetStringSlice("general.disabledCommands"); ok && containsString(disabled, "tracked") {
			return d.slashText(i, "That command is disabled.")
		}
	}
	if !slashCommandAllowed(ctx, "tracked") {
		return tr.Translate("You do not have permission to execute this command", false)
	}

	options := slashOptions(i.ApplicationCommandData())
	profileToken, _ := optionString(options, "profile")
	selection, errText := d.resolveSlashProfileSelection(i, profileToken)
	if errText != "" {
		return errText
	}
	d.logSlashUX(i, "tracked", "scope", selection.LogValue())

	if selection.Mode == slashProfileScopeAll {
		return d.buildSlashTrackedAllProfilesSummary(selection, tr)
	}

	lines := []string{tr.TranslateFormat("Viewing tracking for {0}.", selection.TargetLabelLocalized(tr))}
	alertStatus := map[bool]string{
		true:  tr.Translate("enabled", false),
		false: tr.Translate("disabled", false),
	}[toInt(selection.Human["enabled"], 0) != 0]
	lines = append(lines, fmt.Sprintf("%s **%s**", tr.Translate("Your alerts are currently", false), alertStatus))
	lat := toFloat(selection.Human["latitude"])
	lon := toFloat(selection.Human["longitude"])
	if lat != 0 && lon != 0 {
		lines = append(lines, fmt.Sprintf("%s https://maps.google.com/maps?q=%f,%f", tr.Translate("Your location is currently set to", false), lat, lon))
	} else {
		lines = append(lines, tr.Translate("You have not set a location yet", false))
	}
	if selection.Mode == slashProfileScopeSpecific && selection.ProfileNo != selection.EffectiveNo {
		if row := profileRowByNo(selection.Profiles, selection.EffectiveNo); row != nil {
			lines = append(lines, tr.TranslateFormat("Current profile: {0}.", localizedProfileDisplayName(tr, row)))
		}
	}
	sections := []string{strings.Join(lines, "\n"), d.slashTrackedAreaText(tr, selection.Human), d.slashTrackedCategoryDetails(selection.UserID, selection.ProfileNo, d.slashTrackedBlockedAlerts(selection.Human), tr)}
	message := strings.Join(sections, "\n\n")
	if len(message) < 4000 {
		return message
	}
	fileName := fmt.Sprintf("tracked-p%d.txt", selection.ProfileNo)
	return command.FileReply(fileName, tr.TranslateFormat("Tracking for {0} is attached as a file:", selection.TargetLabelLocalized(tr)), message)
}

func (d *Discord) handleSlashLanguage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	value, _ := optionString(options, "to")
	value = strings.TrimSpace(value)
	if value == "" {
		d.executeSlashLineDeferred(s, i, "language")
		return
	}
	d.executeSlashLineDeferred(s, i, "language "+value)
}
