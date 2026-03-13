package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) fetchChannelByID(s *discordgo.Session, channelID string) (*discordgo.Channel, error) {
	if d != nil && d.channelFetcher != nil {
		return d.channelFetcher(s, channelID)
	}
	if s == nil {
		return nil, fmt.Errorf("discord session missing")
	}
	return s.Channel(channelID)
}

func (d *Discord) fetchGuildMemberByID(s *discordgo.Session, guildID, userID string) (*discordgo.Member, error) {
	if d != nil && d.guildMemberFetcher != nil {
		return d.guildMemberFetcher(s, guildID, userID)
	}
	if s == nil {
		return nil, fmt.Errorf("discord session missing")
	}
	return s.GuildMember(guildID, userID)
}

func (d *Discord) fetchGuildMembersForGuild(s *discordgo.Session, guildID string) ([]*discordgo.Member, error) {
	if d != nil && d.guildMembersLoader != nil {
		return d.guildMembersLoader(s, guildID)
	}
	return d.loadGuildMembersREST(s, guildID)
}
