package command

import (
	"fmt"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/db"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/render"
	"poraclego/internal/scanner"
	"poraclego/internal/stats"
	"poraclego/internal/tz"
	"poraclego/internal/version"
	"poraclego/internal/webhook"
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
	// RefreshAlertCache triggers a reload of in-memory alert caches (e.g. fastMonsters).
	RefreshAlertCache func()

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

	TargetOverride *Target
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

// Version returns the current PoracleGo version.
func (c *Context) Version() string {
	return version.Read(c.Root)
}
