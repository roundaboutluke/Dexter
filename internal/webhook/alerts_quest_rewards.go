package webhook

import (
	"fmt"
	"strings"

	"encoding/json"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

func questString(p *Processor, hook *Hook, language string, tr *i18n.Translator) string {
	if hook == nil {
		return ""
	}
	if task := getString(hook.Message["quest_task"]); task != "" {
		if !getBoolFromConfig(p.cfg, "general.ignoreMADQuestString", false) {
			return task
		}
	}
	lang := language
	if lang == "" && p != nil && p.cfg != nil {
		if value, ok := p.cfg.GetString("general.locale"); ok && value != "" {
			lang = value
		}
	}
	if lang == "" {
		lang = "en"
	}
	var d *data.GameData
	if p != nil {
		d = p.getData()
	}
	if d != nil && d.Translations != nil {
		if raw, ok := d.Translations[lang]; ok {
			if langMap, ok := raw.(map[string]any); ok {
				if title := getString(hook.Message["title"]); title != "" {
					key := fmt.Sprintf("quest_title_%s", strings.ToLower(title))
					if questTitles, ok := langMap["questTitles"].(map[string]any); ok {
						if text, ok := questTitles[key].(string); ok && text != "" {
							if strings.Contains(strings.ToLower(text), "{{amount_0}}") {
								target := getInt(hook.Message["target"])
								if target == 0 {
									target = getInt(hook.Message["quest_target"])
								}
								if target == 0 {
									target = getInt(hook.Message["target_amount"])
								}
								if target != 0 {
									text = strings.ReplaceAll(text, "{{amount_0}}", fmt.Sprintf("%d", target))
								}
							}
							return text
						}
					}
					if questTypes, ok := langMap["questTypes"].(map[string]any); ok {
						if text, ok := questTypes["quest_0"].(string); ok && text != "" {
							return text
						}
					}
				}
			}
		}
	}
	if title := getString(hook.Message["quest_title"]); title != "" {
		return translateMaybe(tr, title)
	}
	if questType := getInt(hook.Message["quest_type"]); questType > 0 && p != nil && d != nil {
		if raw, ok := d.QuestTypes[fmt.Sprintf("%d", questType)]; ok {
			if m, ok := raw.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					return translateMaybe(tr, text)
				}
			}
		}
	}
	return ""
}

func rewardString(p *Processor, hook *Hook, tr *i18n.Translator) string {
	rewardType := getInt(hook.Message["reward_type"])
	reward := getInt(hook.Message["reward"])
	amount := getInt(hook.Message["amount"])
	if rewardType == 0 {
		rewardType = getInt(hook.Message["reward_type_id"])
	}
	if reward == 0 {
		reward = getInt(hook.Message["pokemon_id"])
	}
	basic := ""
	switch rewardType {
	case 2:
		basic = itemRewardString(p, reward, amount, tr)
	case 3:
		if amount == 0 {
			amount = reward
		}
		if amount > 0 {
			basic = fmt.Sprintf("%d %s", amount, translateMaybe(tr, "Stardust"))
		}
	case 4:
		if amount == 0 {
			amount = 1
		}
		basic = fmt.Sprintf("%d %s", amount, translateMaybe(tr, "Candy"))
	case 7:
		if reward != 0 {
			name := monsterName(p, reward)
			basic = fmt.Sprintf("%s", translateMaybe(tr, name))
		}
	case 12:
		if amount == 0 {
			amount = 1
		}
		basic = fmt.Sprintf("%d %s", amount, translateMaybe(tr, "Mega Energy"))
	}
	if basic != "" {
		return basic
	}
	if hook != nil && hook.Type == "quest" && p != nil {
		rewardData := questRewardData(p, hook)
		temp := map[string]any{}
		applyQuestRewardDetails(p, temp, rewardData, "", tr)
		if text, ok := temp["rewardString"].(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func emptyQuestRewardData() map[string]any {
	return map[string]any{
		"monsters":       []map[string]any{},
		"items":          []map[string]any{},
		"energyMonsters": []map[string]any{},
		"candy":          []map[string]any{},
		"dustAmount":     0,
		"itemAmount":     0,
	}
}

func questRewardDataIsEmpty(rewardData map[string]any) bool {
	if rewardData == nil {
		return true
	}
	if getInt(rewardData["dustAmount"]) > 0 {
		return false
	}
	if items, _ := rewardData["items"].([]map[string]any); len(items) > 0 {
		return false
	}
	if monsters, _ := rewardData["monsters"].([]map[string]any); len(monsters) > 0 {
		return false
	}
	if energy, _ := rewardData["energyMonsters"].([]map[string]any); len(energy) > 0 {
		return false
	}
	if candy, _ := rewardData["candy"].([]map[string]any); len(candy) > 0 {
		return false
	}
	return true
}

func questRewardDataStandard(p *Processor, hook *Hook) map[string]any {
	out := map[string]any{
		"monsters":       []map[string]any{},
		"items":          []map[string]any{},
		"energyMonsters": []map[string]any{},
		"candy":          []map[string]any{},
		"dustAmount":     0,
		"itemAmount":     0,
	}
	if hook == nil {
		return out
	}
	rewards := questRewardsFromHook(hook)
	for _, reward := range rewards {
		rewardType := getInt(reward["type"])
		info, _ := reward["info"].(map[string]any)
		switch rewardType {
		case 2:
			id := getIntFromMap(info, "item_id")
			if id == 0 {
				id = getIntFromMap(info, "id")
			}
			amount := getIntFromMap(info, "amount")
			if id > 0 {
				out["items"] = append(out["items"].([]map[string]any), map[string]any{
					"id":     id,
					"amount": amount,
				})
				if out["itemAmount"].(int) == 0 && amount > 0 {
					out["itemAmount"] = amount
				}
			}
		case 3:
			amount := getIntFromMap(info, "amount")
			if amount > 0 {
				out["dustAmount"] = amount
			}
		case 4:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		case 7:
			pokemonID := getIntFromMap(info, "pokemon_id")
			formID := getIntFromMap(info, "form_id")
			if formID == 0 {
				formID = getIntFromMap(info, "form")
			}
			shiny := getBoolFromMap(info, "shiny")
			if pokemonID > 0 {
				out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
					"pokemonId": pokemonID,
					"formId":    formID,
					"shiny":     shiny,
				})
			}
		case 12:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		}
	}
	if len(rewards) == 0 {
		rewardType := getInt(hook.Message["reward_type"])
		if rewardType == 0 {
			rewardType = getInt(hook.Message["quest_reward_type"])
		}
		if rewardType == 0 {
			rewardType = getInt(hook.Message["reward_type_id"])
		}
		reward := getInt(hook.Message["reward"])
		if reward == 0 {
			if rewardType == 2 {
				reward = getInt(hook.Message["quest_item_id"])
			} else if rewardType == 7 {
				reward = getInt(hook.Message["quest_pokemon_id"])
			}
		}
		if reward == 0 {
			reward = getInt(hook.Message["pokemon_id"])
		}
		amount := getInt(hook.Message["reward_amount"])
		if amount == 0 {
			amount = getInt(hook.Message["quest_reward_amount"])
		}
		if amount == 0 {
			amount = getInt(hook.Message["amount"])
		}
		switch rewardType {
		case 2:
			if reward > 0 {
				out["items"] = append(out["items"].([]map[string]any), map[string]any{
					"id":     reward,
					"amount": amount,
				})
				if amount > 0 {
					out["itemAmount"] = amount
				}
			}
		case 3:
			if amount == 0 {
				amount = reward
			}
			out["dustAmount"] = amount
		case 4:
			out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
				"pokemonId": reward,
				"amount":    amount,
			})
		case 7:
			if reward > 0 {
				out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
					"pokemonId": reward,
					"formId":    getInt(hook.Message["form"]),
					"shiny":     getBool(hook.Message["shiny"]),
				})
			}
		case 12:
			out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
				"pokemonId": reward,
				"amount":    amount,
			})
		}
	}
	return out
}

// questRewardData returns the "No AR" reward payload for quest hooks.
// Some Golbat setups send quest hooks with a `with_ar` boolean and only one reward payload (in `rewards`).
// In that case, the reward belongs to the "With AR" variant, so this should be empty.
func questRewardData(p *Processor, hook *Hook) map[string]any {
	if hook == nil {
		return emptyQuestRewardData()
	}
	withAR := getBool(hook.Message["with_ar"])
	// If the hook includes explicit alternative quest fields, treat it as a combined payload
	// where the standard quest_* fields represent "No AR" regardless of with_ar.
	hasAlternative := len(questRewardsFromHookAR(hook)) > 0 || getInt(hook.Message["alternative_quest_reward_type"]) > 0
	if withAR && !hasAlternative {
		return emptyQuestRewardData()
	}
	return questRewardDataStandard(p, hook)
}

func questRewardsFromHook(hook *Hook) []map[string]any {
	if hook == nil {
		return nil
	}
	raw := firstNonEmpty(
		hook.Message["quest_rewards"],
		hook.Message["rewards"],
		hook.Message["reward"],
		hook.Message["quest_reward"],
	)
	return decodeQuestRewards(raw)
}

func questRewardsFromHookAR(hook *Hook) []map[string]any {
	if hook == nil {
		return nil
	}
	raw := firstNonEmpty(
		hook.Message["alternative_quest_rewards"],
		hook.Message["alternative_rewards"],
		hook.Message["alternative_reward"],
		hook.Message["alt_rewards"],
		hook.Message["rewards_alt"],
		hook.Message["quest_rewards_alt"],
	)
	return decodeQuestRewards(raw)
}

func decodeQuestRewards(raw any) []map[string]any {
	switch v := raw.(type) {
	case nil:
		return nil
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if entry, ok := item.(map[string]any); ok {
				out = append(out, entry)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{v}
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return decodeQuestRewards(string(v))
	case json.RawMessage:
		if len(v) == 0 {
			return nil
		}
		return decodeQuestRewards(string(v))
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var decoded []map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return decoded
		}
		var rawDecoded []any
		if err := json.Unmarshal([]byte(v), &rawDecoded); err == nil {
			out := make([]map[string]any, 0, len(rawDecoded))
			for _, item := range rawDecoded {
				if entry, ok := item.(map[string]any); ok {
					out = append(out, entry)
				}
			}
			return out
		}
	}
	return nil
}

func firstNonEmpty(values ...any) any {
	for _, value := range values {
		switch v := value.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(v) == "" {
				continue
			}
			return v
		default:
			return value
		}
	}
	return nil
}

func questRewardDataFromRewardsAndMessage(p *Processor, rewards []map[string]any, msg map[string]any) map[string]any {
	out := map[string]any{
		"monsters":       []map[string]any{},
		"items":          []map[string]any{},
		"energyMonsters": []map[string]any{},
		"candy":          []map[string]any{},
		"dustAmount":     0,
		"itemAmount":     0,
	}
	for _, reward := range rewards {
		rewardType := getInt(reward["type"])
		info, _ := reward["info"].(map[string]any)
		switch rewardType {
		case 2:
			id := getIntFromMap(info, "item_id")
			if id == 0 {
				id = getIntFromMap(info, "id")
			}
			amount := getIntFromMap(info, "amount")
			if id > 0 {
				out["items"] = append(out["items"].([]map[string]any), map[string]any{
					"id":     id,
					"amount": amount,
				})
				if out["itemAmount"].(int) == 0 && amount > 0 {
					out["itemAmount"] = amount
				}
			}
		case 3:
			amount := getIntFromMap(info, "amount")
			if amount > 0 {
				out["dustAmount"] = amount
			}
		case 4:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		case 7:
			pokemonID := getIntFromMap(info, "pokemon_id")
			formID := getIntFromMap(info, "form_id")
			if formID == 0 {
				formID = getIntFromMap(info, "form")
			}
			shiny := getBoolFromMap(info, "shiny")
			if pokemonID > 0 {
				out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
					"pokemonId": pokemonID,
					"formId":    formID,
					"shiny":     shiny,
				})
			}
		case 12:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		}
	}
	if len(rewards) > 0 || msg == nil {
		return out
	}
	rewardType := getInt(msg["reward_type"])
	if rewardType == 0 {
		rewardType = getInt(msg["reward_type_id"])
	}
	reward := getInt(msg["reward"])
	if reward == 0 {
		reward = getInt(msg["pokemon_id"])
	}
	amount := getInt(msg["reward_amount"])
	if amount == 0 {
		amount = getInt(msg["amount"])
	}
	switch rewardType {
	case 2:
		if reward > 0 {
			out["items"] = append(out["items"].([]map[string]any), map[string]any{
				"id":     reward,
				"amount": amount,
			})
			if amount > 0 {
				out["itemAmount"] = amount
			}
		}
	case 3:
		if amount == 0 {
			amount = reward
		}
		out["dustAmount"] = amount
	case 4:
		out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
			"pokemonId": reward,
			"amount":    amount,
		})
	case 7:
		if reward > 0 {
			out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
				"pokemonId": reward,
				"formId":    getInt(msg["form"]),
				"shiny":     getBool(msg["shiny"]),
			})
		}
	case 12:
		out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
			"pokemonId": reward,
			"amount":    amount,
		})
	}
	return out
}

func questRewardDataAR(p *Processor, hook *Hook) map[string]any {
	if hook == nil {
		return emptyQuestRewardData()
	}
	rewards := questRewardsFromHookAR(hook)
	msg := map[string]any{
		"reward_type":   hook.Message["alternative_quest_reward_type"],
		"reward_amount": hook.Message["alternative_quest_reward_amount"],
		"pokemon_id":    hook.Message["alternative_quest_pokemon_id"],
		"form":          hook.Message["alternative_quest_pokemon_form"],
		"shiny":         hook.Message["alternative_quest_shiny"],
	}
	rewardType := getInt(msg["reward_type"])
	if rewardType == 2 {
		msg["reward"] = hook.Message["alternative_quest_item_id"]
	} else if rewardType == 7 {
		msg["reward"] = hook.Message["alternative_quest_pokemon_id"]
	} else {
		msg["reward"] = hook.Message["alternative_quest_reward"]
	}
	if getInt(msg["reward_amount"]) == 0 {
		msg["reward_amount"] = hook.Message["alternative_quest_amount"]
	}
	out := questRewardDataFromRewardsAndMessage(p, rewards, msg)
	// Fallback: single-quest webhooks with `with_ar=true` use the standard rewards fields.
	if questRewardDataIsEmpty(out) && getBool(hook.Message["with_ar"]) {
		return questRewardDataStandard(p, hook)
	}
	return out
}

func questRewardStringFromData(p *Processor, rewardData map[string]any, tr *i18n.Translator) string {
	if rewardData == nil {
		return ""
	}
	temp := map[string]any{}
	applyQuestRewardDetails(p, temp, rewardData, "", tr)
	if text, ok := temp["rewardString"].(string); ok {
		return text
	}
	return ""
}
