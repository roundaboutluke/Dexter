package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) resolveSlashAddProfileSelection(i *discordgo.InteractionCreate, options []*discordgo.ApplicationCommandInteractionDataOption) (slashProfileSelection, string) {
	profileToken, _ := optionString(options, "profile")
	return d.resolveSlashTrackingProfileSelection(i, profileToken)
}

func appendRaidSharedSlashArgs(args []string, options []*discordgo.ApplicationCommandInteractionDataOption) []string {
	if value, ok := optionString(options, "team"); ok {
		switch strings.ToLower(value) {
		case "blue", "red", "yellow", "white":
			args = append(args, strings.ToLower(value))
		}
	}
	if value, ok := optionString(options, "rsvp"); ok {
		switch strings.ToLower(value) {
		case "on":
			args = append(args, "rsvp")
		case "only":
			args = append(args, "rsvp", "only")
		case "off":
			args = append(args, "no", "rsvp")
		}
	}
	if value, ok := optionString(options, "gym"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "gym:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	return args
}

func appendMaxbattleSharedSlashArgs(args []string, options []*discordgo.ApplicationCommandInteractionDataOption) []string {
	if value, ok := optionString(options, "station"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "station:"+strings.TrimSpace(value))
	}
	if value, ok := optionBool(options, "gmax_only"); ok && value {
		args = append(args, "gmax")
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	return args
}

func appendQuestSharedSlashArgs(args []string, options []*discordgo.ApplicationCommandInteractionDataOption) []string {
	if value, ok := optionString(options, "ar"); ok {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "ar":
			args = append(args, "ar")
		case "noar":
			args = append(args, "noar")
		}
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	return args
}

func appendInvasionLikeSlashArgs(args []string, options []*discordgo.ApplicationCommandInteractionDataOption) []string {
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	return args
}

func quoteSlashCommandValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, " \t") {
		return strconv.Quote(value)
	}
	return value
}

func prefixedQuestArg(prefix, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return prefix
	}
	// Autocomplete values may already include the prefix (e.g. "energy:charizard").
	// Avoid double-prefixing like "energy:energy:charizard".
	if strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix)+":") {
		return value
	}
	return prefix + ":" + quoteSlashCommandValue(value)
}
