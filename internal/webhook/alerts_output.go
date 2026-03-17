package webhook

import (
	"fmt"
	"strings"

	"poraclego/internal/data"
	"poraclego/internal/dispatch"
	"poraclego/internal/render"
)

func (p *Processor) enqueue(job dispatch.MessageJob) {
	if strings.HasPrefix(job.Type, "telegram") {
		if p.telegramQueue != nil {
			p.telegramQueue.Push(job)
		}
		return
	}
	if p.discordQueue != nil {
		p.discordQueue.Push(job)
	}
}

func (p *Processor) formatMessage(hook *Hook, match alertMatch) string {
	template := selectTemplate(p, match.Target, hook)
	data := buildRenderData(p, hook, match)
	meta := renderMeta(match.Target)
	if template != "" {
		rendered, err := render.RenderHandlebars(template, data, meta)
		if err == nil && strings.TrimSpace(rendered) != "" {
			return rendered
		}
		if logger := p.controllerLogger(); logger != nil {
			if err != nil {
				logger.Errorf("%s: error rendering message template for %s/%s/%s: %v", hookLogReference(hook), match.Target.Platform, templateTypeForHook(hook), match.Target.Template, err)
			} else {
				logger.Warnf("%s: invalid or empty message template for %s/%s/%s", hookLogReference(hook), match.Target.Platform, templateTypeForHook(hook), match.Target.Template)
			}
		}
	} else if logger := p.controllerLogger(); logger != nil {
		logger.Warnf("%s: cannot find DTS template for %s/%s/%s", hookLogReference(hook), match.Target.Platform, templateTypeForHook(hook), match.Target.Template)
	}
	tr := defaultTranslator(p, hook)
	switch hook.Type {
	case "pokemon":
		return fmt.Sprintf("%s %s (%s)", tr.Translate("Pokemon", false), nameOrID(p, hook, "pokemon_id"), getString(hook.Message["pokemon_id"]))
	case "raid":
		return fmt.Sprintf("%s L%d %s", tr.Translate("Raid", false), getInt(hook.Message["level"]), nameOrID(p, hook, "pokemon_id"))
	case "egg":
		return fmt.Sprintf("%s L%d", tr.Translate("Egg", false), getInt(hook.Message["level"]))
	case "max_battle":
		return fmt.Sprintf("%s L%d %s", tr.Translate("Max Battle", false), getInt(hook.Message["level"]), nameOrID(p, hook, "pokemon_id"))
	case "quest":
		return fmt.Sprintf("%s %s", tr.Translate("Quest", false), questRewardSummary(hook))
	case "invasion":
		return fmt.Sprintf("%s %s", tr.Translate("Invasion", false), getString(hook.Message["grunt_type"]))
	case "lure":
		return fmt.Sprintf("%s %s", tr.Translate("Lure", false), getString(hook.Message["lure_id"]))
	case "nest":
		return fmt.Sprintf("%s %s", tr.Translate("Nest", false), nameOrID(p, hook, "pokemon_id"))
	case "gym", "gym_details":
		return fmt.Sprintf("%s %s", tr.Translate("Gym", false), getString(hook.Message["id"]))
	case "weather":
		return fmt.Sprintf("%s %s", tr.Translate("Weather", false), getString(hook.Message["condition"]))
	case "fort_update":
		return fmt.Sprintf("%s %s", tr.Translate("Fort update", false), getString(hook.Message["id"]))
	default:
		return fmt.Sprintf("%s", hook.Type)
	}
}

func (p *Processor) formatPayload(hook *Hook, match alertMatch) (map[string]any, string) {
	template := selectTemplatePayload(p, match.Target, hook)
	data := buildRenderData(p, hook, match)
	meta := renderMeta(match.Target)
	payload := map[string]any{}
	message := ""
	ping := getString(match.Row["ping"])
	if template != nil {
		rendered := renderAny(template, data, meta, p)
		if renderedMap, ok := rendered.(map[string]any); ok {
			payload = renderedMap
			embed, hasEmbed := payload["embed"].(map[string]any)
			if hasEmbed {
				payload["embeds"] = []any{embed}
				delete(payload, "embed")
			}
			if content, ok := payload["content"].(string); ok {
				message = content
			} else if hasEmbed {
				if desc, ok := embed["description"].(string); ok {
					message = desc
				}
			}
		} else if renderedString, ok := rendered.(string); ok {
			message = renderedString
		}
	} else if logger := p.controllerLogger(); logger != nil {
		logger.Warnf("%s: cannot find payload template for %s/%s/%s", hookLogReference(hook), match.Target.Platform, templateTypeForHook(hook), match.Target.Template)
	}
	if ping != "" {
		if content, ok := payload["content"].(string); ok {
			payload["content"] = content + ping
		} else if len(payload) > 0 {
			payload["content"] = ping
		}
		if message != "" {
			message += ping
		} else if ping != "" {
			message = ping
		}
	}
	if message == "" {
		message = p.formatMessage(hook, match)
	}
	return payload, message
}

func renderMeta(target alertTarget) map[string]any {
	return map[string]any{
		"language": target.Language,
		"platform": target.Platform,
	}
}

func nameOrID(p *Processor, hook *Hook, key string) string {
	d := p.getData()
	if d == nil {
		return getString(hook.Message[key])
	}
	id := getString(hook.Message[key])
	if id == "" {
		return ""
	}
	form := getInt(hook.Message["form"])
	if form == 0 {
		form = getInt(hook.Message["form_id"])
	}
	if form == 0 {
		form = getInt(hook.Message["pokemon_form"])
	}
	keyWithForm := fmt.Sprintf("%s_%d", id, form)
	if name := lookupMonsterName(d, keyWithForm); name != "" {
		return name
	}
	if name := lookupMonsterName(d, fmt.Sprintf("%s_0", id)); name != "" {
		return name
	}
	if name := lookupMonsterName(d, id); name != "" {
		return name
	}
	return id
}

func lookupMonsterName(data *data.GameData, key string) string {
	if data == nil || data.Monsters == nil {
		return ""
	}
	monster, ok := data.Monsters[key]
	if !ok {
		return ""
	}
	if m, ok := monster.(map[string]any); ok {
		if name, ok := m["name"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}
