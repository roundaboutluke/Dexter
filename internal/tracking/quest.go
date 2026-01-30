package tracking

import (
	"fmt"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

// QuestRowText mirrors PoracleJS quest tracking formatting.
func QuestRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any) string {
	rewardType := intFromAny(row["reward_type"])
	reward := intFromAny(row["reward"])
	form := intFromAny(row["form"])
	amount := intFromAny(row["amount"])
	arMode := intFromAny(row["ar"])
	rewardThing := ""

	switch rewardType {
	case 7:
		mon := findMonster(game, reward, form)
		if mon != nil {
			rewardThing = tr.Translate(getString(mon["name"]), false)
			formName := getString(getMap(mon["form"])["name"])
			if form != 0 && formName != "" {
				rewardThing = rewardThing + " " + tr.Translate(formName, false)
			}
		} else {
			rewardThing = fmt.Sprintf("%s %d %d", tr.Translate("Unknown monster", false), reward, form)
		}
	case 3:
		if reward > 0 {
			rewardThing = fmt.Sprintf("%d %s", reward, tr.Translate("or more stardust", false))
		} else {
			rewardThing = tr.Translate("stardust", false)
		}
	case 2:
		if item := getItem(game.Items, reward); item != "" {
			rewardThing = tr.Translate(item, false)
		} else {
			rewardThing = fmt.Sprintf("%s %d", tr.Translate("Unknown item", false), reward)
		}
	case 12:
		if reward == 0 {
			rewardThing = tr.Translate("mega energy", false)
		} else {
			mon := findMonster(game, reward, 0)
			monsterName := tr.Translate("Unknown monster", false)
			if mon != nil {
				monsterName = tr.Translate(getString(mon["name"]), false)
			}
			rewardThing = fmt.Sprintf("%s %s", tr.Translate("mega energy", false), monsterName)
		}
	case 4:
		if reward == 0 {
			rewardThing = tr.Translate("Rare Candy", false)
		} else {
			mon := findMonster(game, reward, 0)
			monsterName := tr.Translate("Unknown monster", false)
			if mon != nil {
				monsterName = tr.Translate(getString(mon["name"]), false)
			}
			rewardThing = fmt.Sprintf("%s Candy", monsterName)
		}
	case 9:
		if reward == 0 {
			rewardThing = tr.Translate("Rare Candy XL", false)
		} else {
			mon := findMonster(game, reward, 0)
			monsterName := tr.Translate("Unknown monster", false)
			if mon != nil {
				monsterName = tr.Translate(getString(mon["name"]), false)
			}
			rewardThing = fmt.Sprintf("%s XL Candy", monsterName)
		}
	case 1:
		rewardThing = tr.Translate("experience", false)
	default:
		rewardThing = tr.Translate("Unknown reward", false)
	}

	distance := intFromAny(row["distance"])
	parts := []string{
		fmt.Sprintf("%s: **%s**", titleCase(tr.Translate("reward", false)), rewardThing),
	}
	if amount > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", tr.Translate("minimum", false), amount))
	}
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	if arMode == 1 {
		parts = append(parts, fmt.Sprintf("| %s: %s", titleCase(tr.Translate("ar", false)), tr.Translate("No AR", false)))
	} else if arMode == 2 {
		parts = append(parts, fmt.Sprintf("| %s: %s", titleCase(tr.Translate("ar", false)), tr.Translate("With AR", false)))
	}
	parts = append(parts, standardText(cfg, tr, row))
	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}

func getItem(items map[string]any, id int) string {
	if items == nil {
		return ""
	}
	key := fmt.Sprintf("%d", id)
	if item, ok := items[key]; ok {
		if m, ok := item.(map[string]any); ok {
			return getString(m["name"])
		}
	}
	return ""
}
