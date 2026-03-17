package bot

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func normalizedSlashCommandType(kind discordgo.ApplicationCommandType) discordgo.ApplicationCommandType {
	if kind == 0 {
		return discordgo.ChatApplicationCommand
	}
	return kind
}

func slashCommandKey(cmd *discordgo.ApplicationCommand) string {
	if cmd == nil {
		return ""
	}
	name := strings.ToLower(strings.TrimSpace(cmd.Name))
	if name == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", normalizedSlashCommandType(cmd.Type), name)
}

func slashCommandSignature(cmd *discordgo.ApplicationCommand) string {
	if cmd == nil {
		return ""
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return ""
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	normalizeSlashCommandPayload(payload)
	raw, err = json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func legacySlashCommandName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "track", "egg", "invasion", "tracked", "remove":
		return true
	default:
		return false
	}
}
