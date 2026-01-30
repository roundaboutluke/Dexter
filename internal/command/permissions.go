package command

import "strings"

// commandAllowed checks commandSecurity and disabledCommands.
func commandAllowed(ctx *Context, command string) bool {
	disabled, _ := ctx.Config.GetStringSlice("general.disabledCommands")
	for _, entry := range disabled {
		if strings.EqualFold(entry, command) {
			return false
		}
	}

	if ctx.Platform == "telegram" {
		return true
	}

	if ctx.Platform == "discord" {
		raw, ok := ctx.Config.Get("discord.commandSecurity")
		if !ok {
			return true
		}
		security, ok := raw.(map[string]any)
		if !ok {
			return true
		}
		list, ok := security[command]
		if !ok {
			return true
		}
		switch v := list.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					if s == ctx.UserID {
						return true
					}
					for _, role := range ctx.Roles {
						if s == role {
							return true
						}
					}
				}
			}
			return false
		case []string:
			for _, s := range v {
				if s == ctx.UserID {
					return true
				}
				for _, role := range ctx.Roles {
					if s == role {
						return true
					}
				}
			}
			return false
		default:
			return true
		}
	}

	return true
}
