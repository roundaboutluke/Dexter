package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// TrackedCommand lists tracking entries.
type TrackedCommand struct{}

func (c *TrackedCommand) Name() string { return "tracked" }

func (c *TrackedCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "tracked") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	profileNo := result.ProfileNo
	targetID := result.TargetID

	if len(args) > 0 && strings.EqualFold(args[0], "help") {
		help := &HelpCommand{}
		return help.Handle(ctx, []string{"tracked"})
	}

	human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": targetID})
	if err != nil {
		return "", err
	}
	if human == nil {
		return unregisteredMessage(ctx, tr), nil
	}
	if containsWord(args, "area") {
		return tracking.AreaText(tr, ctx.Fences.Fences, parseAreaListFromHuman(human)), nil
	}

	profile, _ := ctx.Query.SelectOneQuery("profiles", map[string]any{"id": targetID, "profile_no": profileNo})
	blocked := tracking.BlockedAlerts(human)

	adminExplanation := ""
	if ctx.IsAdmin {
		adminExplanation = fmt.Sprintf("Tracking details for **%s**\n", result.Target.Name)
	}

	lat := toFloat(human["latitude"])
	lon := toFloat(human["longitude"])
	locationText := ""
	if lat != 0 && lon != 0 {
		mapLink := fmt.Sprintf("https://maps.google.com/maps?q=%f,%f", lat, lon)
		locationText = "\n" + tr.Translate("Your location is currently set to", false) + " " + mapLink
	} else {
		locationText = "\n" + tr.Translate("You have not set a location yet", false)
	}
	restartExplanation := ""
	if toInt(human["enabled"], 0) == 0 {
		restartExplanation = "\n" + tr.TranslateFormat("You can start receiving alerts again using `{0}{1}`", ctx.Prefix, tr.Translate("start", true))
	}
	status := fmt.Sprintf("%s%s **%s**%s%s",
		adminExplanation,
		tr.Translate("Your alerts are currently", false),
		map[bool]string{true: tr.Translate("enabled", false), false: tr.Translate("disabled", false)}[toInt(human["enabled"], 0) != 0],
		restartExplanation,
		locationText,
	)

	areaText := tracking.AreaText(tr, ctx.Fences.Fences, parseAreaListFromHuman(human))
	profileText := ""
	if profile != nil {
		if name := strings.TrimSpace(fmt.Sprintf("%v", profile["name"])); name != "" {
			profileText = fmt.Sprintf("%s %s", tr.Translate("Your profile is currently set to:", false), name)
		}
	}

	sections := []string{status}
	if areaText != "" {
		sections = append(sections, areaText)
	}
	if profileText != "" {
		sections = append(sections, profileText)
	}

	message := strings.Join(sections, "\n\n")
	message = message + "\n\n" + tracking.CategoryDetails(tracking.ListingContext{
		Config:   ctx.Config,
		Query:    ctx.Query,
		Data:     ctx.Data,
		GymNames: ctx.Scanner,
	}, tr, targetID, profileNo, blocked)
	if len(message) < 4000 {
		return message, nil
	}
	if link, err := createHastebinLink(message); err == nil && link != "" {
		return fmt.Sprintf("%s %s", tr.Translate("Tracking list is quite long. Have a look at", false), link), nil
	}
	reply := buildFileReply(fmt.Sprintf("%s.txt", result.Target.Name), tr.Translate("Tracking list is long, but Hastebin is also down. Tracking list made into a file:", false), message)
	if reply != "" {
		return reply, nil
	}
	return message, nil
}
