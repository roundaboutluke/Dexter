package bot

import (
	"fmt"
	"strconv"
	"strings"

	"dexter/internal/uicons"
)

// stripTrailingTrackArgs removes common trailing tracking args (clean, template:X)
// from a condition string.
func stripTrailingTrackArgs(s string) string {
	parts := strings.Fields(s)
	result := []string{}
	for _, p := range parts {
		lower := strings.ToLower(p)
		if lower == "clean" || strings.HasPrefix(lower, "template:") {
			continue
		}
		result = append(result, p)
	}
	return strings.TrimSpace(strings.Join(result, " "))
}

// lureIDFromName returns the lure ID matching the given name, or 0 if not found.
func (d *Discord) lureIDFromName(name string) int {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return 0
	}
	entry, ok := d.manager.data.UtilData["lures"]
	if !ok {
		return 0
	}
	target := strings.ToLower(strings.TrimSpace(name))
	switch v := entry.(type) {
	case []any:
		for i, raw := range v {
			if m, ok := raw.(map[string]any); ok {
				if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", m["name"])), target) {
					return i
				}
			}
		}
	case map[string]any:
		for key, raw := range v {
			if m, ok := raw.(map[string]any); ok {
				if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", m["name"])), target) {
					if id, err := strconv.Atoi(key); err == nil {
						return id
					}
				}
			}
		}
	}
	return 0
}

// weatherIDFromName returns the weather condition ID matching the given name, or 0 if not found.
func (d *Discord) weatherIDFromName(name string) int {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return 0
	}
	raw, ok := d.manager.data.UtilData["weather"].(map[string]any)
	if !ok {
		return 0
	}
	target := strings.ToLower(strings.TrimSpace(name))
	for key, entry := range raw {
		if m, ok := entry.(map[string]any); ok {
			if strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", m["name"])), target) {
				if id, err := strconv.Atoi(key); err == nil {
					return id
				}
			}
		}
	}
	return 0
}

// slashPreviewQuestIconURL resolves an icon for a quest from command line args.
// Args are the parts after the command name, e.g. ["energy:charizard", "d500"].
func (d *Discord) slashPreviewQuestIconURL(client *uicons.Client, args []string) string {
	if len(args) == 0 {
		return ""
	}
	arg := strings.ToLower(args[0])
	// Stardust: "stardust" or "stardust500"
	if strings.HasPrefix(arg, "stardust") {
		url, _ := client.RewardStardustIcon(0)
		return url
	}
	// Mega energy: "energy:charizard"
	if strings.HasPrefix(arg, "energy:") {
		name := strings.TrimPrefix(arg, "energy:")
		name = strings.Trim(name, "\"")
		if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.RewardMegaEnergyIcon(id, 0)
			return url
		}
		return ""
	}
	// Candy: "candy:pikachu"
	if strings.HasPrefix(arg, "candy:") {
		name := strings.TrimPrefix(arg, "candy:")
		name = strings.Trim(name, "\"")
		if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.RewardCandyIcon(id, 0)
			return url
		}
		return ""
	}
	// XL Candy: "xlcandy:pikachu"
	if strings.HasPrefix(arg, "xlcandy:") {
		name := strings.TrimPrefix(arg, "xlcandy:")
		name = strings.Trim(name, "\"")
		if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.RewardXLCandyIcon(id, 0)
			return url
		}
		return ""
	}
	// Pokemon quest reward: try to resolve as a pokemon name
	name := strings.Trim(args[0], "\"")
	if id := d.pokemonIDFromName(name); id > 0 {
		url, _ := client.PokemonIcon(id, 0, 0, 0, 0, 0, false, 0)
		return url
	}
	// Item quest reward: try to resolve as an item name
	if id := d.itemIDFromName(name); id > 0 {
		url, _ := client.RewardItemIcon(id)
		return url
	}
	return ""
}

// slashGymTeamID returns the team ID for a gym team name, or -1 if unknown.
func slashGymTeamID(name string) int {
	switch name {
	case "uncontested", "harmony":
		return 0
	case "mystic", "blue":
		return 1
	case "valor", "valour", "red":
		return 2
	case "instinct", "yellow":
		return 3
	case "everything":
		return 4
	default:
		return -1
	}
}

// itemIDFromName looks up an item ID by name from game data.
func (d *Discord) itemIDFromName(name string) int {
	if d.manager == nil || d.manager.data == nil || d.manager.data.Items == nil {
		return 0
	}
	lower := strings.ToLower(strings.Trim(strings.TrimSpace(name), "\""))
	for key, raw := range d.manager.data.Items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		itemName := strings.ToLower(fmt.Sprintf("%v", item["name"]))
		if itemName == lower {
			id, _ := strconv.Atoi(key)
			return id
		}
	}
	return 0
}

func (d *Discord) pokemonIDFromName(name string) int {
	if d.manager == nil || d.manager.data == nil {
		return 0
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	if id, err := strconv.Atoi(lower); err == nil {
		return id
	}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		monName := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		if monName == lower {
			return toInt(mon["id"], 0)
		}
	}
	return 0
}
