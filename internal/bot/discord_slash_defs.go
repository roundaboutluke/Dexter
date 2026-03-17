package bot

import (
	"strings"

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

	subcommand := func(name, description string, options []*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        name,
			Description: description,
			Options:     options,
		}
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

	profileOption := func() *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "profile",
			Description:  "Choose which profile to add this filter to",
			Autocomplete: true,
		}
	}

	appendCreationOptions := func(options []*discordgo.ApplicationCommandOption) []*discordgo.ApplicationCommandOption {
		options = append(options, templateOptions()...)
		return append(options, profileOption())
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

	raidTeamOption := func() *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
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
		}
	}

	rsvpOption := func() *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "rsvp",
			Description: "RSVP matching",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "off", Value: "off"},
				{Name: "on", Value: "on"},
				{Name: "only", Value: "only"},
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
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_cp", Description: "Minimum CP", MinValue: floatPtr(0)},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_cp", Description: "Maximum CP", MinValue: floatPtr(0)},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_level", Description: "Minimum level", MinValue: floatPtr(1), MaxValue: 50},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_level", Description: "Maximum level", MinValue: floatPtr(1), MaxValue: 50},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_atk", Description: "Minimum attack", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_atk", Description: "Maximum attack", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_def", Description: "Minimum defense", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_def", Description: "Maximum defense", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_sta", Description: "Minimum stamina", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_sta", Description: "Maximum stamina", MinValue: floatPtr(0), MaxValue: 15},
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
		distanceOption(),
		cleanFlagOption("Auto delete after despawn"),
	}
	trackOptions = appendCreationOptions(trackOptions)

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
		{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "slot_changes", Description: "Alert on slot changes"},
	}
	if gymBattleEnabled {
		gymOptions = append(gymOptions, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "battle_changes",
			Description: "Alert on battle changes",
		})
	}
	gymOptions = append(gymOptions,
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	)
	gymOptions = appendCreationOptions(gymOptions)

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
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "new", Description: "Track new POIs"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "removal", Description: "Track removals"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "name", Description: "Track name changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "photo", Description: "Track photo changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "location", Description: "Track location changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "include_empty", Description: "Include empty POIs"},
		distanceOption(),
	}
	fortOptions = appendCreationOptions(fortOptions)

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
	nestOptions = appendCreationOptions(nestOptions)

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
	weatherOptions = appendCreationOptions(weatherOptions)

	raidBossOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "pokemon", Description: "Choose a Pokemon, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
		raidTeamOption(),
		rsvpOption(),
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	raidBossOptions = appendCreationOptions(raidBossOptions)

	raidLevelOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "level", Description: "Choose a raid level, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
		raidTeamOption(),
		rsvpOption(),
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	raidLevelOptions = appendCreationOptions(raidLevelOptions)

	eggOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "level", Description: "Choose an egg level, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
		raidTeamOption(),
		rsvpOption(),
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	eggOptions = appendCreationOptions(eggOptions)

	maxbattleBossOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "pokemon", Description: "Choose a Pokemon, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: "station", Description: "Optional station", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "gmax_only", Description: "Only Gigantamax battles"},
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	maxbattleBossOptions = appendCreationOptions(maxbattleBossOptions)

	maxbattleLevelOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "level", Description: "Choose a max battle level, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: "station", Description: "Optional station", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "gmax_only", Description: "Only Gigantamax battles"},
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	maxbattleLevelOptions = appendCreationOptions(maxbattleLevelOptions)

	questAROption := &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "ar",
		Description: "Optional AR mapping filter",
		Choices: []*discordgo.ApplicationCommandOptionChoice{
			{Name: "Any", Value: "any"},
			{Name: "No AR", Value: "noar"},
			{Name: "With AR", Value: "ar"},
		},
	}
	questPokemonOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "pokemon", Description: "Choose a Pokemon, or leave blank for a guided flow", Autocomplete: true},
		questAROption,
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	questPokemonOptions = appendCreationOptions(questPokemonOptions)

	questItemOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "item", Description: "Choose an item, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_amount", Description: "Optional minimum value for items only", MinValue: floatPtr(0)},
		questAROption,
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	questItemOptions = appendCreationOptions(questItemOptions)

	questStardustOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_amount", Description: "Optional minimum value for stardust only", MinValue: floatPtr(0)},
		questAROption,
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	questStardustOptions = appendCreationOptions(questStardustOptions)

	questCandyOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "pokemon", Description: "Choose a Pokemon, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_amount", Description: "Optional minimum value for candy only", MinValue: floatPtr(0)},
		questAROption,
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	questCandyOptions = appendCreationOptions(questCandyOptions)

	questMegaEnergyOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "pokemon", Description: "Choose a Pokemon, or leave blank for a guided flow", Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_amount", Description: "Optional minimum value for mega energy only", MinValue: floatPtr(0)},
		questAROption,
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	questMegaEnergyOptions = appendCreationOptions(questMegaEnergyOptions)

	rocketOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Choose a grunt or leader type, or leave blank for a guided flow", Autocomplete: true},
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	rocketOptions = appendCreationOptions(rocketOptions)

	incidentOptions := []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Choose a Pokestop event type, or leave blank for a guided flow", Autocomplete: true},
		distanceOption(),
		cleanFlagOption("Auto delete after expiration"),
	}
	incidentOptions = appendCreationOptions(incidentOptions)

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "pokemon",
			Description: "Track Pokemon spawns",
			Options:     trackOptions,
		},
		{
			Name:        "raid",
			Description: "Track raid bosses, levels, or eggs",
			Options: []*discordgo.ApplicationCommandOption{
				subcommand("boss", "Track a raid boss", raidBossOptions),
				subcommand("level", "Track raid levels", raidLevelOptions),
				subcommand("egg", "Track raid eggs by level", eggOptions),
			},
		},
		{
			Name:        "maxbattle",
			Description: "Track max battles by boss or level",
			Options: []*discordgo.ApplicationCommandOption{
				subcommand("boss", "Track max battles by boss", maxbattleBossOptions),
				subcommand("level", "Track max battles by level", maxbattleLevelOptions),
			},
		},
		{
			Name:        "quest",
			Description: "Track quest rewards",
			Options: []*discordgo.ApplicationCommandOption{
				subcommand("pokemon", "Track Pokemon quest rewards", questPokemonOptions),
				subcommand("item", "Track item quest rewards", questItemOptions),
				subcommand("stardust", "Track stardust quest rewards", questStardustOptions),
				subcommand("candy", "Track candy quest rewards", questCandyOptions),
				subcommand("mega-energy", "Track mega energy quest rewards", questMegaEnergyOptions),
			},
		},
		{
			Name:        "rocket",
			Description: "Track Team Rocket invasions",
			Options:     rocketOptions,
		},
		{
			Name:        "pokestop-event",
			Description: "Track Pokestop events",
			Options:     incidentOptions,
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
			Options: appendCreationOptions([]*discordgo.ApplicationCommandOption{
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
			}),
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
			Description: "Manage your active profile, saved location, areas, and quiet hours",
		},
		{
			Name:        "filters",
			Description: "Show or remove filters",
			Options: []*discordgo.ApplicationCommandOption{
				subcommand("show", "Review your filters", []*discordgo.ApplicationCommandOption{
					{Type: discordgo.ApplicationCommandOptionString, Name: "profile", Description: "Choose which profile to review", Autocomplete: true},
				}),
				subcommand("remove", "Remove one or more filters", []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "type",
						Description: "Choose what kind of filter to remove",
						Required:    true,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "pokemon", Value: "pokemon"},
							{Name: "raid", Value: "raid"},
							{Name: "egg", Value: "egg"},
							{Name: "maxbattle", Value: "maxbattle"},
							{Name: "rocket", Value: "rocket"},
							{Name: "pokestop-event", Value: "pokestop-event"},
							{Name: "quest", Value: "quest"},
							{Name: "gym", Value: "gym"},
							{Name: "weather", Value: "weather"},
							{Name: "lure", Value: "lure"},
							{Name: "nest", Value: "nest"},
							{Name: "fort", Value: "fort"},
						},
					},
					{Type: discordgo.ApplicationCommandOptionString, Name: "tracking", Description: "Choose what to remove", Required: true, Autocomplete: true},
					{Type: discordgo.ApplicationCommandOptionString, Name: "profile", Description: "Choose which profile to remove from", Autocomplete: true},
				}),
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
	return d.localizedSlashCommands(commands)
}

type slashCommandSyncStats struct {
	Created   int
	Updated   int
	Unchanged int
	Deleted   int
	Failed    int
}

func normalizeSlashCommandPayload(payload map[string]any) {
	normalizeSlashCommandMap(payload)
	for _, key := range []string{
		"id",
		"application_id",
		"guild_id",
		"version",
		"default_permission",
		"default_member_permissions",
		"dm_permission",
		"nsfw",
	} {
		delete(payload, key)
	}
	if kind, ok := payload["type"].(float64); !ok || kind == 0 {
		payload["type"] = float64(discordgo.ChatApplicationCommand)
	}
}

func normalizeSlashCommandMap(payload map[string]any) {
	for key, value := range payload {
		cleaned, keep := normalizeSlashCommandPayloadValue(key, value)
		if !keep {
			delete(payload, key)
			continue
		}
		payload[key] = cleaned
	}
}

func normalizeSlashCommandPayloadValue(key string, value any) (any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case map[string]any:
		normalizeSlashCommandMap(v)
		if len(v) == 0 {
			return nil, false
		}
		return v, true
	case []any:
		out := make([]any, 0, len(v))
		for _, entry := range v {
			cleaned, keep := normalizeSlashCommandPayloadValue("", entry)
			if keep {
				out = append(out, cleaned)
			}
		}
		if len(out) == 0 && (key == "options" || key == "choices" || key == "channel_types") {
			return nil, false
		}
		return out, true
	default:
		return value, true
	}
}

func (d *Discord) syncSlashCommands(s *discordgo.Session, appID, guildID string, commands []*discordgo.ApplicationCommand) slashCommandSyncStats {
	stats := slashCommandSyncStats{}
	if len(commands) == 0 {
		return stats
	}
	logger := logging.Get().Discord
	existing, err := d.fetchApplicationCommands(s, appID, guildID)
	if err != nil {
		stats.Failed = len(commands)
		if logger != nil {
			logger.Warnf("Slash command fetch failed (guild=%q): %v", guildID, err)
		}
		return stats
	}
	existingByKey := map[string]*discordgo.ApplicationCommand{}
	for _, cmd := range existing {
		if key := slashCommandKey(cmd); key != "" {
			existingByKey[key] = cmd
		}
	}
	desiredKeys := map[string]bool{}

	for _, cmd := range commands {
		key := slashCommandKey(cmd)
		if key == "" {
			continue
		}
		desiredKeys[key] = true
		current, ok := existingByKey[key]
		if !ok {
			if _, err := d.createApplicationCommand(s, appID, guildID, cmd); err != nil {
				stats.Failed++
				if logger != nil {
					logger.Warnf("Slash command registration failed (command=%s): %v", cmd.Name, err)
				}
				continue
			}
			stats.Created++
			continue
		}
		if current.ID != "" && slashCommandSignature(current) == slashCommandSignature(cmd) {
			stats.Unchanged++
			continue
		}
		if _, err := d.editApplicationCommand(s, appID, guildID, current.ID, cmd); err != nil {
			stats.Failed++
			if logger != nil {
				logger.Warnf("Slash command update failed (command=%s): %v", cmd.Name, err)
			}
			continue
		}
		stats.Updated++
	}

	for _, cmd := range existing {
		key := slashCommandKey(cmd)
		if key == "" || desiredKeys[key] || !legacySlashCommandName(cmd.Name) {
			continue
		}
		if err := d.deleteApplicationCommand(s, appID, guildID, cmd.ID); err != nil {
			stats.Failed++
			if logger != nil {
				logger.Warnf("Slash command delete failed (command=%s): %v", cmd.Name, err)
			}
			continue
		}
		stats.Deleted++
	}

	return stats
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
			commands, err := d.fetchApplicationCommands(s, appID, guildID)
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
				_ = d.deleteApplicationCommand(s, appID, guildID, cmd.ID)
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
	logger := logging.Get().Discord
	if logger != nil {
		targets := d.slashLocalizationTargets()
		langs := make([]string, 0, len(targets))
		for _, target := range targets {
			langs = append(langs, target.poracle)
		}
		if len(langs) == 0 {
			logger.Infof("Slash localization locales: none")
		} else {
			logger.Infof("Slash localization locales: %s", strings.Join(langs, ", "))
		}
	}
	stats := d.syncSlashCommands(s, appID, "", d.slashCommandDefinitions())
	if logger != nil {
		logger.Infof("Slash commands synced (global): created=%d updated=%d unchanged=%d deleted=%d failed=%d", stats.Created, stats.Updated, stats.Unchanged, stats.Deleted, stats.Failed)
	}
}
