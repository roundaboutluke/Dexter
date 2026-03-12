package bot

import (
	"github.com/bwmarrin/discordgo"
	"poraclego/internal/logging"
)

func (d *Discord) slashCommandDefinitions() []*discordgo.ApplicationCommand {
	maxDistance := 0
	pvpMaxRank := 0
	gymBattleEnabled := false
	hideTemplateOptions := false
	if d != nil && d.manager != nil && d.manager.cfg != nil {
		if value, ok := d.manager.cfg.GetInt("tracking.maxDistance"); ok {
			maxDistance = value
		}
		if value, ok := d.manager.cfg.GetInt("general.pvpFilterMaxRank"); ok {
			pvpMaxRank = value
		}
		if value, ok := d.manager.cfg.GetBool("tracking.enableGymBattle"); ok {
			gymBattleEnabled = value
		}
		if value, ok := d.manager.cfg.GetBool("discord.hideTemplateOptions"); ok {
			hideTemplateOptions = value
		}
		if value, ok := d.manager.cfg.GetBool("discord.slash.hideTemplateOptions"); ok {
			hideTemplateOptions = value
		}
	}
	if pvpMaxRank <= 0 {
		pvpMaxRank = 4096
	}

	distanceOption := func() *discordgo.ApplicationCommandOption {
		opt := &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "distance",
			Description: "Distance away in meters",
			MinValue:    floatPtr(0),
		}
		if maxDistance > 0 {
			opt.MaxValue = float64(maxDistance)
		}
		return opt
	}

	templateOptions := func() []*discordgo.ApplicationCommandOption {
		if hideTemplateOptions {
			return nil
		}
		return []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "template",
				Description:  "Optional template name",
				Autocomplete: true,
			},
		}
	}

	cleanFlagOption := func(description string) *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "clean",
			Description: description,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Enable", Value: "clean"},
			},
		}
	}

	trackOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "pokemon",
			Description:  "Choose a Pokemon, or leave blank for a guided flow",
			Autocomplete: true,
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "form",
			Description:  "Select form (optional)",
			Autocomplete: true,
		},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_iv", Description: "Set minimum IV", MinValue: floatPtr(0), MaxValue: 100},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_iv", Description: "Set maximum IV", MinValue: floatPtr(0), MaxValue: 100},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_atk", Description: "Minimum attack", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_atk", Description: "Maximum attack", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_def", Description: "Minimum defense", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_def", Description: "Maximum defense", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_sta", Description: "Minimum stamina", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_sta", Description: "Maximum stamina", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_cp", Description: "Minimum CP", MinValue: floatPtr(0)},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_cp", Description: "Maximum CP", MinValue: floatPtr(0)},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_level", Description: "Minimum level", MinValue: floatPtr(1), MaxValue: 50},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_level", Description: "Maximum level", MinValue: floatPtr(1), MaxValue: 50},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "size",
			Description: "Select size",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "all", Value: "all"},
				{Name: "xxs", Value: "xxs"},
				{Name: "xs", Value: "xs"},
				{Name: "m", Value: "m"},
				{Name: "xl", Value: "xl"},
				{Name: "xxl", Value: "xxl"},
			},
		},
		distanceOption(),
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "gender",
			Description: "Select gender",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "All", Value: "all"},
				{Name: "Male", Value: "male"},
				{Name: "Female", Value: "female"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "pvp_league",
			Description: "Select pvp league (pvp_ranks required)",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "little", Value: "little"},
				{Name: "great", Value: "great"},
				{Name: "ultra", Value: "ultra"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "pvp_ranks",
			Description: "Set number of pvp ranks (pvp_league required)",
			MinValue:    floatPtr(0),
			MaxValue:    float64(pvpMaxRank),
		},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_time", Description: "Minimum time left in seconds", MinValue: floatPtr(0)},
		cleanFlagOption("Auto delete after despawn"),
	}
	trackOptions = append(trackOptions, templateOptions()...)

	gymOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "team",
			Description: "Choose a team, or leave blank for a guided flow",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Everything", Value: "everything"},
				{Name: "Mystic (Blue)", Value: "mystic"},
				{Name: "Valor (Red)", Value: "valor"},
				{Name: "Instinct (Yellow)", Value: "instinct"},
				{Name: "Uncontested", Value: "uncontested"},
				{Name: "Normal", Value: "normal"},
			},
		},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "slot_changes", Description: "Alert on slot changes"},
		{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	gymOptions = append(gymOptions, templateOptions()...)
	if gymBattleEnabled {
		gymOptions = append(gymOptions, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "battle_changes",
			Description: "Alert on battle changes",
		})
	}

	fortOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "type",
			Description: "Choose Pokestop or Gym, or leave blank for a guided flow",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Everything", Value: "everything"},
				{Name: "Pokestop", Value: "pokestop"},
				{Name: "Gym", Value: "gym"},
			},
		},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "include_empty", Description: "Include empty POIs"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "location", Description: "Track location changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "name", Description: "Track name changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "photo", Description: "Track photo changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "removal", Description: "Track removals"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "new", Description: "Track new POIs"},
		distanceOption(),
	}
	fortOptions = append(fortOptions, templateOptions()...)

	nestOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "pokemon",
			Description:  "Choose a Pokemon, or leave blank for a guided flow",
			Autocomplete: true,
		},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_spawn", Description: "Optional minimum average spawns", MinValue: floatPtr(0)},
		distanceOption(),
		cleanFlagOption("Auto delete after despawn"),
	}
	nestOptions = append(nestOptions, templateOptions()...)

	weatherOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "condition",
			Description:  "Choose weather, or leave blank for a guided flow",
			Autocomplete: true,
		},
		{Type: discordgo.ApplicationCommandOptionString, Name: "location", Description: "Optional location. Leave blank to use your saved location."},
		cleanFlagOption("Auto delete after expiration"),
	}
	weatherOptions = append(weatherOptions, templateOptions()...)

	return []*discordgo.ApplicationCommand{
		{
			Name:        "track",
			Description: "Track Pokemon spawns",
			Options:     trackOptions,
		},
		{
			Name:        "raid",
			Description: "Track raid bosses or raid levels",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Choose a boss or level, or leave blank for a guided flow", Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "team",
					Description: "Select controlling team",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "All", Value: "all"},
						{Name: "Valor (Red)", Value: "red"},
						{Name: "Mystic (Blue)", Value: "blue"},
						{Name: "Instinct (Yellow)", Value: "yellow"},
						{Name: "Uncontested (White)", Value: "white"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "rsvp", Description: "RSVP matching", Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "off", Value: "off"},
					{Name: "on", Value: "on"},
					{Name: "only", Value: "only"},
				}},
				distanceOption(),
				cleanFlagOption("Auto delete after expiration"),
			}, templateOptions()...),
		},
		{
			Name:        "egg",
			Description: "Track raid eggs by level",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "level", Description: "Choose an egg level, or leave blank for a guided flow", Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "team",
					Description: "Select controlling team",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "All", Value: "all"},
						{Name: "Valor (Red)", Value: "red"},
						{Name: "Mystic (Blue)", Value: "blue"},
						{Name: "Instinct (Yellow)", Value: "yellow"},
						{Name: "Uncontested (White)", Value: "white"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "rsvp", Description: "RSVP matching", Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "off", Value: "off"},
					{Name: "on", Value: "on"},
					{Name: "only", Value: "only"},
				}},
				distanceOption(),
				cleanFlagOption("Auto delete after expiration"),
			}, templateOptions()...),
		},
		{
			Name:        "maxbattle",
			Description: "Track max battles by boss or level",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Choose a boss or level, or leave blank for a guided flow", Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "station", Description: "Optional station", Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "gmax_only", Description: "Only Gigantamax battles"},
				distanceOption(),
				cleanFlagOption("Auto delete after expiration"),
			}, templateOptions()...),
		},
		{
			Name:        "quest",
			Description: "Track quest rewards",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Choose a reward, or leave blank for a guided flow", Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_amount", Description: "Optional minimum value for items only", MinValue: floatPtr(0)},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ar",
					Description: "Optional AR mapping filter",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Any", Value: "any"},
						{Name: "No AR", Value: "noar"},
						{Name: "With AR", Value: "ar"},
					},
				},
				distanceOption(),
				cleanFlagOption("Auto delete after expiration"),
			}, templateOptions()...),
		},
		{
			Name:        "invasion",
			Description: "Track Team Rocket invasions",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Choose a grunt or leader type, or leave blank for a guided flow", Autocomplete: true},
				distanceOption(),
				cleanFlagOption("Auto delete after expiration"),
			}, templateOptions()...),
		},
		{
			Name:        "gym",
			Description: "Track gym team or slot changes",
			Options:     gymOptions,
		},
		{
			Name:        "fort",
			Description: "Track Pokestop and Gym changes",
			Options:     fortOptions,
		},
		{
			Name:        "nest",
			Description: "Track nests by Pokemon",
			Options:     nestOptions,
		},
		{
			Name:        "weather",
			Description: "Track weather at your saved location or a place you provide",
			Options:     weatherOptions,
		},
		{
			Name:        "lure",
			Description: "Track lure modules",
			Options: append([]*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Choose a lure type, or leave blank for a guided flow",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Everything", Value: "everything"},
						{Name: "Basic", Value: "basic"},
						{Name: "Glacial", Value: "glacial"},
						{Name: "Mossy", Value: "mossy"},
						{Name: "Magnetic", Value: "magnetic"},
						{Name: "Rainy", Value: "rainy"},
						{Name: "Golden", Value: "sparkly"},
					},
				},
				distanceOption(),
				cleanFlagOption("Auto delete after expiration"),
			}, templateOptions()...),
		},
		{
			Name:        "language",
			Description: "Change language",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "to", Description: "Select language", Autocomplete: true},
			},
		},
		{
			Name:        "profile",
			Description: "Set your saved location, areas, and quiet hours",
		},
		{
			Name:        "tracked",
			Description: "Review what you are tracking",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "profile", Description: "Choose which profile to review", Autocomplete: true},
			},
		},
		{
			Name:        "remove",
			Description: "Remove one or more tracking entries",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Choose what kind of alert to remove",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "pokemon", Value: "pokemon"},
						{Name: "raid", Value: "raid"},
						{Name: "egg", Value: "egg"},
						{Name: "maxbattle", Value: "maxbattle"},
						{Name: "invasion", Value: "invasion"},
						{Name: "quest", Value: "quest"},
						{Name: "gym", Value: "gym"},
						{Name: "weather", Value: "weather"},
						{Name: "lure", Value: "lure"},
						{Name: "nest", Value: "nest"},
						{Name: "fort", Value: "fort"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "profile", Description: "Choose which profile to remove from", Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "tracking", Description: "Choose what to remove", Required: true, Autocomplete: true},
			},
		},
		{
			Name:        "start",
			Description: "Enable alerts",
		},
		{
			Name:        "stop",
			Description: "Disable alerts",
		},
		{
			Name:        "help",
			Description: "Show slash command help",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "command", Description: "Optional command to explain", Autocomplete: true},
			},
		},
		{
			Name:        "info",
			Description: "Look up Pokemon, moves, items, weather, and more",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Choose what to look up, or leave blank for a guided flow",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Pokemon", Value: "pokemon"},
						{Name: "Moves", Value: "moves"},
						{Name: "Items", Value: "items"},
						{Name: "Weather", Value: "weather"},
						{Name: "Rarity", Value: "rarity"},
						{Name: "Shiny", Value: "shiny"},
						{Name: "Translate", Value: "translate"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "pokemon", Description: "Pokemon name (used when type is Pokemon)", Autocomplete: true},
			},
		},
	}
}

func (d *Discord) registerSlashCommands(s *discordgo.Session) {
	if s == nil || s.State == nil || s.State.User == nil {
		return
	}
	appID := s.State.User.ID
	deregisterSlashCommands := false
	slashCommandsDisabled := false
	if d.manager != nil && d.manager.cfg != nil {
		if value, ok := d.manager.cfg.GetBool("discord.slash.deregisterOnStart"); ok {
			deregisterSlashCommands = value
		}
		if value, ok := d.manager.cfg.GetBool("discord.slash.disabled"); ok {
			slashCommandsDisabled = value
		}
	}
	if slashCommandsDisabled {
		// Intentionally do nothing. This is to avoid interfering with other
		// /command providers that use the same bot token / application.
		return
	}
	if deregisterSlashCommands {
		// Remove global slash commands so the next run can re-register from scratch.
		logger := logging.Get().Discord
		deleteFor := func(guildID string) {
			commands, err := s.ApplicationCommands(appID, guildID)
			if err != nil {
				if logger != nil {
					logger.Warnf("Slash command deregistration failed (guild=%q): %v", guildID, err)
				}
				return
			}
			for _, cmd := range commands {
				if cmd == nil || cmd.ID == "" {
					continue
				}
				_ = s.ApplicationCommandDelete(appID, guildID, cmd.ID)
			}
			if logger != nil {
				scope := "global"
				if guildID != "" {
					scope = guildID
				}
				logger.Infof("Slash commands deregistered (%s): %d", scope, len(commands))
			}
		}
		deleteFor("")
		return
	}
	for _, cmd := range d.slashCommandDefinitions() {
		_, _ = s.ApplicationCommandCreate(appID, "", cmd)
	}
}
