package command

import (
	"errors"
	"fmt"
	"strings"
)

// Handler processes a command.
type Handler interface {
	Name() string
	Handle(ctx *Context, args []string) (string, error)
}

// Registry stores command handlers.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry builds a registry with default handlers.
func NewRegistry() *Registry {
	r := &Registry{handlers: map[string]Handler{}}
	r.Register(&VersionCommand{})
	r.Register(&HelpCommand{})
	r.Register(&StartCommand{})
	r.Register(&StopCommand{})
	r.Register(&PoracleCommand{})
	r.Register(&ApplyCommand{})
	r.Register(&UnregisterCommand{})
	r.Register(&TrackCommand{})
	r.Register(&UntrackCommand{})
	r.Register(&TrackedCommand{})
	r.Register(&LocationCommand{})
	r.Register(&LanguageCommand{})
	r.Register(&ProfileCommand{})
	r.Register(&AreaCommand{})
	r.Register(&EnableCommand{})
	r.Register(&DisableCommand{})
	r.Register(&UserListCommand{})
	r.Register(&InfoCommand{})
	r.Register(&BroadcastCommand{})
	r.Register(&CommunityCommand{})
	r.Register(&BackupCommand{})
	r.Register(&RestoreCommand{})
	r.Register(&ScriptCommand{})
	r.Register(&FortCommand{})
	r.Register(&ChannelCommand{})
	r.Register(&WebhookCommand{})
	r.Register(&PoracleTestCommand{})
	r.Register(&RaidCommand{})
	r.Register(&EggCommand{})
	r.Register(&QuestCommand{})
	r.Register(&InvasionCommand{})
	r.Register(&IncidentCommand{})
	r.Register(&LureCommand{})
	r.Register(&GymCommand{})
	r.Register(&NestCommand{})
	r.Register(&WeatherCommand{})
	r.Register(&MaxbattleCommand{})
	return r
}

// Register adds a handler.
func (r *Registry) Register(handler Handler) {
	r.handlers[strings.ToLower(handler.Name())] = handler
}

// Execute runs a command by name.
func (r *Registry) Execute(ctx *Context, line string) (string, error) {
	parts := parseCommandArgs(line)
	if len(parts) == 0 {
		return "", errors.New("empty command")
	}
	if ctx != nil {
		ctx.RawLine = line
	}
	name := strings.ToLower(parts[0])
	logLabel := commandLogLabel(ctx, line)
	handler, ok := r.handlers[name]
	aliasLang := ""
	if !ok && ctx != nil {
		if alias, lang := resolveCommandAlias(ctx, name); alias != "" {
			if alt, okAlt := r.handlers[alias]; okAlt {
				handler = alt
				ok = true
				aliasLang = lang
			}
		}
	}
	if !ok {
		if logger := ctx.CommandLogger(); logger != nil {
			logger.Warnf("%s unknown command", logLabel)
		}
		return "", errors.New("unknown command")
	}
	if logger := ctx.CommandLogger(); logger != nil {
		logger.Infof("%s", logLabel)
	}
	if name != "help" && len(parts) > 1 && strings.EqualFold(parts[1], "help") {
		if helpHandler, okHelp := r.handlers["help"]; okHelp {
			if aliasLang != "" && ctx != nil {
				clone := *ctx
				clone.Language = aliasLang
				reply, err := helpHandler.Handle(&clone, []string{name})
				logCommandResult(ctx, logLabel, err)
				return reply, err
			}
			reply, err := helpHandler.Handle(ctx, []string{name})
			logCommandResult(ctx, logLabel, err)
			return reply, err
		}
	}
	if aliasLang != "" {
		clone := *ctx
		clone.Language = aliasLang
		reply, err := handler.Handle(&clone, parts[1:])
		logCommandResult(&clone, logLabel, err)
		return reply, err
	}
	reply, err := handler.Handle(ctx, parts[1:])
	logCommandResult(ctx, logLabel, err)
	return reply, err
}

func parseCommandArgs(line string) []string {
	args := []string{}
	var buf strings.Builder
	inQuote := false
	for _, r := range line {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t', '\n':
			if inQuote {
				buf.WriteRune(r)
			} else if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args
}

func resolveCommandAlias(ctx *Context, name string) (string, string) {
	if ctx == nil || ctx.Config == nil {
		return "", ""
	}
	raw, ok := ctx.Config.Get("general.availableLanguages")
	if !ok {
		return "", ""
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return "", ""
	}
	for lang, entry := range entries {
		config, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if value, ok := config["poracle"].(string); ok && value != "" {
			if strings.EqualFold(name, value) {
				return "poracle", lang
			}
		}
		if value, ok := config["help"].(string); ok && value != "" {
			if strings.EqualFold(name, value) {
				return "help", lang
			}
		}
	}
	return "", ""
}

func commandLogLabel(ctx *Context, line string) string {
	if ctx == nil {
		return line
	}
	targetType := ctx.Platform + ":channel"
	targetID := ctx.ChannelID
	targetName := strings.TrimSpace(ctx.ChannelName)
	if ctx.IsDM || ctx.ChannelID == "" {
		targetType = ctx.Platform + ":user"
		targetID = ctx.UserID
		targetName = strings.TrimSpace(ctx.UserName)
	}
	if targetName == "" {
		targetName = ctx.UserName
	}
	if targetName == "" {
		targetName = targetID
	}
	return fmt.Sprintf("%s/%s-%s: %s", targetName, targetType, targetID, line)
}

func logCommandResult(ctx *Context, label string, err error) {
	logger := ctx.CommandLogger()
	if logger == nil {
		return
	}
	if err != nil {
		logger.Errorf("%s unhappy: %v", label, err)
		return
	}
	logger.Debugf("%s completed", label)
}
