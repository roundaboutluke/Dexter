package bot

import (
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/command"
	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/db"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
	"poraclego/internal/scanner"
	"poraclego/internal/stats"
	"poraclego/internal/tz"
	"poraclego/internal/webhook"
)

// Manager coordinates Discord and Telegram bots.
type Manager struct {
	cfg           *config.Config
	query         *db.Query
	data          *data.GameData
	i18n          *i18n.Factory
	templates     []dts.Template
	registry      *command.Registry
	fences        *geofence.Store
	processor     *webhook.Processor
	discordQueue  *dispatch.Queue
	telegramQueue *dispatch.Queue
	webhookQueue  *webhook.Queue
	statsTracker  *stats.Tracker
	weatherData   *webhook.WeatherTracker
	tzLocator     *tz.Locator
	scanner       *scanner.Client
	discordBot    *Discord
	telegramBots  []*Telegram
}

// NewManager creates a bot manager.
func NewManager(cfg *config.Config, query *db.Query, data *data.GameData, i18nFactory *i18n.Factory, templates []dts.Template, fences *geofence.Store, processor *webhook.Processor, discordQueue *dispatch.Queue, telegramQueue *dispatch.Queue, webhookQueue *webhook.Queue, statsTracker *stats.Tracker, weatherData *webhook.WeatherTracker, tzLocator *tz.Locator, scannerClient *scanner.Client) *Manager {
	return &Manager{
		cfg:           cfg,
		query:         query,
		data:          data,
		i18n:          i18nFactory,
		templates:     templates,
		registry:      command.NewRegistry(),
		fences:        fences,
		processor:     processor,
		discordQueue:  discordQueue,
		telegramQueue: telegramQueue,
		webhookQueue:  webhookQueue,
		statsTracker:  statsTracker,
		weatherData:   weatherData,
		tzLocator:     tzLocator,
		scanner:       scannerClient,
	}
}

// Start starts enabled bots.
func (m *Manager) Start() {
	if enabled, _ := m.cfg.GetBool("discord.enabled"); enabled {
		token := ""
		if tokens, ok := m.cfg.GetStringSlice("discord.token"); ok && len(tokens) > 0 {
			token = tokens[0]
		}
		if token == "" {
			token, _ = m.cfg.GetString("discord.token.0")
		}
		if token == "" {
			token, _ = m.cfg.GetString("discord.token")
		}
		if token == "" {
			logger := logging.Get().Discord
			if logger != nil {
				logger.Warnf("discord bot skipped: missing token")
			} else {
				fmt.Fprintln(os.Stderr, "discord bot skipped: missing token")
			}
		} else {
			discord := NewDiscord(m, token)
			if err := discord.Start(); err != nil {
				logger := logging.Get().Discord
				if logger != nil {
					logger.Errorf("discord bot failed: %v", err)
				} else {
					fmt.Fprintf(os.Stderr, "discord bot failed: %v\n", err)
				}
			}
			m.discordBot = discord
		}
	}
	if enabled, _ := m.cfg.GetBool("telegram.enabled"); enabled {
		tokens := []string{}
		if list, ok := m.cfg.GetStringSlice("telegram.token"); ok && len(list) > 0 {
			tokens = append(tokens, list...)
		} else if token, ok := m.cfg.GetString("telegram.token"); ok && token != "" {
			tokens = append(tokens, token)
		}
		if token, ok := m.cfg.GetString("telegram.channelToken"); ok && token != "" {
			tokens = append(tokens, token)
		}
		for _, token := range tokens {
			if token == "" {
				continue
			}
			telegram := NewTelegram(m, token)
			if err := telegram.Start(); err != nil {
				logger := logging.Get().Telegram
				if logger != nil {
					logger.Errorf("telegram bot failed: %v", err)
				} else {
					fmt.Fprintf(os.Stderr, "telegram bot failed: %v\n", err)
				}
				continue
			}
			m.telegramBots = append(m.telegramBots, telegram)
		}
	}
}

// UpdateData replaces the game data used by bot commands and autocompletes.
func (m *Manager) UpdateData(game *data.GameData) {
	if m == nil || game == nil {
		return
	}
	m.data = game
}

// DiscordSession returns the active Discord session, if available.
func (m *Manager) DiscordSession() *discordgo.Session {
	if m == nil || m.discordBot == nil {
		return nil
	}
	return m.discordBot.session
}

// UpdateTemplates refreshes DTS templates for command contexts.
func (m *Manager) UpdateTemplates(templates []dts.Template) {
	if m == nil {
		return
	}
	m.templates = templates
}

// Registry returns the command registry.
func (m *Manager) Registry() *command.Registry {
	return m.registry
}

// RefreshAlertState schedules a full alert-state refresh when a processor is available.
func (m *Manager) RefreshAlertState() {
	if m == nil || m.processor == nil {
		return
	}
	m.processor.RefreshAlertCacheAsync()
}

// Context builds a command context.
func (m *Manager) Context(platform, language, prefix, userID, userName, channelID, channelName string, isDM, isAdmin bool, roles []string, root string) *command.Context {
	return &command.Context{
		Config:            m.cfg,
		Query:             m.query,
		Data:              m.data,
		I18n:              m.i18n,
		Templates:         m.templates,
		Fences:            m.fences,
		DiscordQueue:      m.discordQueue,
		TelegramQueue:     m.telegramQueue,
		WebhookQueue:      m.webhookQueue,
		Stats:             m.statsTracker,
		Weather:           m.weatherData,
		Timezone:          m.tzLocator,
		Scanner:           m.scanner,
		Root:              root,
		Logs:              logging.Get(),
		RefreshAlertCache: m.RefreshAlertState,
		Platform:          platform,
		Language:          language,
		Prefix:            prefix,
		UserID:            userID,
		UserName:          userName,
		ChannelID:         channelID,
		ChannelName:       channelName,
		IsDM:              isDM,
		IsAdmin:           isAdmin,
		Roles:             roles,
	}
}
