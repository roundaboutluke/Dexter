package command

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"poraclego/internal/i18n"
)

// InfoCommand provides basic information queries.
type InfoCommand struct{}

func (c *InfoCommand) Name() string { return "info" }

func (c *InfoCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	if !result.CanContinue {
		return result.Message, nil
	}
	tr := ctx.I18n.Translator(result.Language)
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	if len(args) == 0 {
		return infoDefaultMessage(ctx, result, tr), nil
	}
	switch strings.ToLower(args[0]) {
	case "poracle":
		return infoPoracle(ctx), nil
	case "translate":
		return infoTranslate(tr, args), nil
	case "dts":
		return infoDTS(ctx), nil
	case "moves":
		return infoMoves(ctx), nil
	case "items":
		return infoItems(ctx), nil
	case "weather":
		return infoWeather(ctx, result.TargetID, args[1:], re), nil
	case "rarity":
		return infoRarity(ctx), nil
	case "shiny":
		return infoShiny(ctx), nil
	default:
		aliasArgs := expandPokemonAliases(ctx, args)
		if msg, ok := infoPokemon(ctx, aliasArgs, re); ok {
			return msg, nil
		}
		lines := []string{tr.TranslateFormat("Valid commands are `{0}info rarity`, `{0}info weather`, `{0}info moves`, `{0}info items`, `{0}info bulbasaur`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "info", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}
}

var startedAt = time.Now()

func infoDefaultMessage(ctx *Context, result TargetResult, tr *i18n.Translator) string {
	lines := []string{tr.Translate("Valid commands include `info moves`, `info items`, `info translate <text>`, `info dts`", false)}
	if helpLine := singleLineHelpText(ctx, "info", result.Language, result.Target); helpLine != "" {
		lines = append(lines, helpLine)
	}
	return strings.Join(lines, "\n")
}

func infoPoracle(ctx *Context) string {
	if !ctx.IsAdmin {
		return "🙅"
	}
	uptime := time.Since(startedAt).Truncate(time.Second)
	webhookLen := 0
	if ctx.WebhookQueue != nil {
		webhookLen = ctx.WebhookQueue.Len()
	}
	discordLen := 0
	discordSummary := map[string]int{}
	if ctx.DiscordQueue != nil {
		discordLen = ctx.DiscordQueue.Len()
		for target, count := range ctx.DiscordQueue.TargetCounts() {
			discordSummary[target] = count
		}
	}
	telegramLen := 0
	telegramSummary := map[string]int{}
	if ctx.TelegramQueue != nil {
		telegramLen = ctx.TelegramQueue.Len()
		for target, count := range ctx.TelegramQueue.TargetCounts() {
			telegramSummary[target] = count
		}
	}
	queueInfo := fmt.Sprintf("Inbound webhook %d | Discord %d | Telegram %d", webhookLen, discordLen, telegramLen)
	queueSummaryPayload := map[string]map[string]int{
		"discord":  discordSummary,
		"telegram": telegramSummary,
	}
	queueSummary := ""
	if payload, err := json.MarshalIndent(queueSummaryPayload, "", " "); err == nil {
		queueSummary = string(payload)
	} else {
		queueSummary = fmt.Sprintf("%v", queueSummaryPayload)
	}
	return fmt.Sprintf("Queue info: %s\nQueue summary: %s\nPoracle has been up for %s", queueInfo, queueSummary, formatUptime(uptime))
}

func infoTranslate(tr *i18n.Translator, args []string) string {
	reverse := []string{}
	forward := []string{}
	for _, arg := range args {
		reverse = append(reverse, fmt.Sprintf("\"%s\" ", arg))
		forward = append(forward, fmt.Sprintf("\"%s\" ", tr.Translate(arg, false)))
	}
	return fmt.Sprintf("Reverse translation: %s\nForward translation: %s", strings.Join(reverse, ","), strings.Join(forward, ","))
}

func infoDTS(ctx *Context) string {
	if !ctx.IsAdmin {
		return "🙅"
	}
	lines := []string{"Your loaded DTS looks like this:"}
	for _, tpl := range ctx.Templates {
		lines = append(lines, fmt.Sprintf("type: %s platform: %s id: %v language: %v", tpl.Type, tpl.Platform, tpl.ID, tpl.Language))
	}
	return strings.Join(lines, "\n")
}
