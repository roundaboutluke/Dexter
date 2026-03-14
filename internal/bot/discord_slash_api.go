package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) fetchApplicationCommands(s *discordgo.Session, appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	if d != nil && d.commandFetcher != nil {
		return d.commandFetcher(s, appID, guildID)
	}
	if s == nil {
		return nil, fmt.Errorf("discord session missing")
	}
	return s.ApplicationCommands(appID, guildID)
}

func (d *Discord) createApplicationCommand(s *discordgo.Session, appID, guildID string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
	if d != nil && d.commandCreator != nil {
		return d.commandCreator(s, appID, guildID, cmd)
	}
	if s == nil {
		return nil, fmt.Errorf("discord session missing")
	}
	return s.ApplicationCommandCreate(appID, guildID, cmd)
}

func (d *Discord) editApplicationCommand(s *discordgo.Session, appID, guildID, cmdID string, cmd *discordgo.ApplicationCommand) (*discordgo.ApplicationCommand, error) {
	if d != nil && d.commandEditor != nil {
		return d.commandEditor(s, appID, guildID, cmdID, cmd)
	}
	if s == nil {
		return nil, fmt.Errorf("discord session missing")
	}
	return s.ApplicationCommandEdit(appID, guildID, cmdID, cmd)
}

func (d *Discord) deleteApplicationCommand(s *discordgo.Session, appID, guildID, cmdID string) error {
	if d != nil && d.commandDeleter != nil {
		return d.commandDeleter(s, appID, guildID, cmdID)
	}
	if s == nil {
		return fmt.Errorf("discord session missing")
	}
	return s.ApplicationCommandDelete(appID, guildID, cmdID)
}
