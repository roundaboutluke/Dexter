package command

import (
	"fmt"

	"poraclego/internal/i18n"
)

func defaultTemplateName(ctx *Context) string {
	if value, ok := ctx.Config.GetString("general.defaultTemplateName"); ok && value != "" {
		return value
	}
	return "1"
}

func applyDistanceDefaults(ctx *Context, tr *i18n.Translator, distance int, result TargetResult, remove bool, allowZeroWithoutArea bool) (int, string, string) {
	if remove {
		return distance, "", ""
	}
	defaultDistance := 0
	if def, ok := ctx.Config.GetInt("tracking.defaultDistance"); ok {
		defaultDistance = def
	}
	if distance == 0 {
		if allowZeroWithoutArea {
			// Keep distance disabled (0) when tracking is narrowed by a specific entity
			// (e.g. a specific gym/station), even if the user has no location/area set.
		} else if defaultDistance > 0 && !ctx.IsAdmin {
			distance = defaultDistance
		}
	}
	if max, ok := ctx.Config.GetInt("tracking.maxDistance"); ok && max > 0 && distance > max && !ctx.IsAdmin {
		distance = max
	}
	if distance > 0 && !result.UserHasLocation {
		text := "Oops, a distance was set in command but no location is defined for your tracking - check the"
		if tr != nil {
			return distance, "", fmt.Sprintf("%s `%s%s`", tr.Translate(text, false), ctx.Prefix, tr.Translate("help", true))
		}
		return distance, "", fmt.Sprintf("%s `%shelp`", text, ctx.Prefix)
	}
	if distance == 0 && !result.UserHasArea && !ctx.IsAdmin {
		if allowZeroWithoutArea {
			return distance, "", ""
		}
		text := "Oops, no distance was set in command and no area is defined for your tracking - check the"
		if tr != nil {
			return distance, "", fmt.Sprintf("%s `%s%s`", tr.Translate(text, false), ctx.Prefix, tr.Translate("help", true))
		}
		return distance, "", fmt.Sprintf("%s `%shelp`", text, ctx.Prefix)
	}
	if distance == 0 && !result.UserHasArea && ctx.IsAdmin {
		if allowZeroWithoutArea {
			return distance, "", ""
		}
		distance = defaultDistance
		warnText := "Warning: Admin command detected without distance set - using default distance"
		if tr != nil {
			warnText = tr.Translate(warnText, false)
		}
		return distance, fmt.Sprintf("%s %d", warnText, defaultDistance), ""
	}
	return distance, "", ""
}

func prependWarning(warning, message string) string {
	if warning == "" {
		return message
	}
	if message == "" {
		return warning
	}
	return warning + "\n" + message
}
