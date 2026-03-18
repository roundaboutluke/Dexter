package command

import (
	"fmt"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/db"
	"dexter/internal/dispatch"
	"dexter/internal/dts"
	"dexter/internal/geofence"
	"dexter/internal/i18n"
	"dexter/internal/logging"
	"dexter/internal/render"
	"dexter/internal/scanner"
	"dexter/internal/stats"
	"dexter/internal/tz"
	"dexter/internal/version"
	"dexter/internal/webhook"
)

// Context holds command execution dependencies.
type Context struct {
	Config        *config.Config
	Query         *db.Query
	Data          *data.GameData
	I18n          *i18n.Factory
	Templates     []dts.Template
	Fences        *geofence.Store
	DiscordQueue  *dispatch.Queue
	TelegramQueue *dispatch.Queue
	WebhookQueue  *webhook.Queue
	Stats         *stats.Tracker
	Weather       *webhook.WeatherTracker
	Timezone      *tz.Locator
	Root          string
	Scanner       *scanner.Client
	Logs          logging.Loggers
	// RefreshAlertCache triggers a reload of the full in-memory alert state snapshot.
	RefreshAlertCache func()

	alertStateDirty            bool
	alertStateRefreshRequested bool

	Platform    string
	Language    string
	Prefix      string
	RawLine     string
	Ping        string
	UserID      string
	UserName    string
	ChannelID   string
	ChannelName string
	IsDM        bool
	IsAdmin     bool
	Roles       []string

	TargetOverride  *Target
	ProfileOverride int
}

// RenderTemplate renders a DTS template by type/platform/language/id.
func (c *Context) RenderTemplate(templateType, templateID string, payload any) (string, error) {
	for _, tpl := range c.Templates {
		if tpl.Hidden || tpl.Type != templateType || tpl.Platform != c.Platform {
			continue
		}
		if templateID != "" && fmt.Sprintf("%v", tpl.ID) != templateID {
			continue
		}
		if c.Language != "" && tpl.Language != nil && *tpl.Language != c.Language {
			continue
		}
		template, ok := tpl.Template.(map[string]any)
		if !ok {
			continue
		}
		content, ok := template["content"].(string)
		if !ok || content == "" {
			continue
		}
		meta := map[string]any{
			"language": c.Language,
			"platform": c.Platform,
		}
		return render.RenderHandlebars(content, payload, meta)
	}
	return "", nil
}

// Version returns the current Dexter version.
func (c *Context) Version() string {
	return version.Read(c.Root)
}

// CommandLogger returns the shared command logger when available.
func (c *Context) CommandLogger() *logging.Logger {
	if c == nil {
		return nil
	}
	if c.Logs.Commands != nil {
		return c.Logs.Commands
	}
	return c.Logs.Command
}

// TransportLogger returns the platform-specific transport logger when available.
func (c *Context) TransportLogger() *logging.Logger {
	if c == nil {
		return nil
	}
	switch c.Platform {
	case "discord":
		return c.Logs.Discord
	case "telegram":
		return c.Logs.Telegram
	default:
		return c.Logs.General
	}
}

// MarkAlertStateDirty marks the current command execution as changing alert-state inputs.
func (c *Context) MarkAlertStateDirty() {
	if c == nil {
		return
	}
	c.alertStateDirty = true
}

func (c *Context) resetAlertStateTracking() {
	if c == nil {
		return
	}
	c.alertStateDirty = false
	c.alertStateRefreshRequested = false
}

func (c *Context) shouldRefreshAlertState() bool {
	return c != nil && c.alertStateDirty
}

// RequestAlertStateRefresh triggers the refresh callback once for the current command execution.
func (c *Context) RequestAlertStateRefresh() {
	if c == nil || c.RefreshAlertCache == nil || c.alertStateRefreshRequested {
		return
	}
	c.alertStateRefreshRequested = true
	c.RefreshAlertCache()
}
