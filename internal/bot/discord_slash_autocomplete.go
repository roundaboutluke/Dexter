package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/scanner"
)

func (d *Discord) autocompletePokemonChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	type candidate struct {
		ID   int
		Name string
	}
	candidates := []candidate{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := toInt(mon["id"], 0)
		if name == "" || id == 0 {
			continue
		}
		if query == "" || name == query || fmt.Sprintf("%d", id) == query || strings.HasPrefix(name, query) || strings.Contains(name, query) {
			candidates = append(candidates, candidate{ID: id, Name: name})
		}
	}
	if len(candidates) == 0 && query != "" {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(candidates)+1)
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
		if len(candidates) > 24 {
			candidates = candidates[:24]
		}
	} else if len(candidates) > 25 {
		candidates = candidates[:25]
	}
	for _, mon := range candidates {
		label := fmt.Sprintf("%s (#%d)", d.titleCase(mon.Name), mon.ID)
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: fmt.Sprintf("%d", mon.ID),
		})
	}
	return choices
}

func (d *Discord) autocompleteInfoPokemonChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	type candidate struct {
		ID   int
		Name string
	}
	candidates := []candidate{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := toInt(mon["id"], 0)
		if name == "" || id == 0 {
			continue
		}
		if query == "" || name == query || fmt.Sprintf("%d", id) == query || strings.HasPrefix(name, query) || strings.Contains(name, query) {
			candidates = append(candidates, candidate{ID: id, Name: name})
		}
	}
	if len(candidates) == 0 && query != "" {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	if len(candidates) > 25 {
		candidates = candidates[:25]
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(candidates))
	for _, mon := range candidates {
		label := fmt.Sprintf("%s (#%d)", d.titleCase(mon.Name), mon.ID)
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: fmt.Sprintf("%d", mon.ID),
		})
	}
	return choices
}

func (d *Discord) autocompletePokemonFormChoices(query, pokemon string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	id := d.pokemonIDFromValue(pokemon)
	if id == 0 {
		return nil
	}
	forms := d.pokemonFormNames(id)
	if len(forms) == 0 {
		return nil
	}
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "All forms",
			Value: "all",
		})
	}
	for _, form := range forms {
		if len(choices) >= 25 {
			break
		}
		lower := strings.ToLower(form)
		if query != "" && !strings.Contains(lower, query) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  d.titleCase(form),
			Value: form,
		})
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteLanguageChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.i18n == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	available := d.manager.i18n.EffectiveLanguages()
	if len(available) == 0 {
		return nil
	}

	languageNames := map[string]string{}
	if d.manager.data != nil && d.manager.data.UtilData != nil {
		if rawNames, ok := d.manager.data.UtilData["languageNames"].(map[string]any); ok {
			for key, value := range rawNames {
				languageNames[strings.ToLower(key)] = strings.TrimSpace(fmt.Sprintf("%v", value))
			}
		}
	}

	type entry struct {
		key   string
		label string
	}
	entries := make([]entry, 0, len(available))
	for _, key := range available {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" {
			continue
		}
		name := languageNames[k]
		label := k
		if name != "" {
			label = fmt.Sprintf("%s (%s)", name, k)
		}
		entries = append(entries, entry{key: k, label: label})
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].label) < strings.ToLower(entries[j].label) })

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(entries))
	for _, e := range entries {
		if len(choices) >= 25 {
			break
		}
		if query != "" {
			if !strings.Contains(strings.ToLower(e.key), query) && !strings.Contains(strings.ToLower(e.label), query) {
				continue
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  e.label,
			Value: e.key,
		})
	}
	return choices
}

func (d *Discord) autocompleteWeatherChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	raw, ok := d.manager.data.UtilData["weather"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}

	type entry struct {
		id   int
		name string
	}
	entries := make([]entry, 0, len(raw))
	for key, value := range raw {
		weatherID := toInt(key, 0)
		if weatherID <= 0 {
			continue
		}
		m, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", m["name"]))
		if name == "" {
			continue
		}
		entries = append(entries, entry{id: weatherID, name: name})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })

	choices := []*discordgo.ApplicationCommandOptionChoice{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
	}
	for _, e := range entries {
		if len(choices) >= 25 {
			break
		}
		label := fmt.Sprintf("%s (#%d)", e.name, e.id)
		value := fmt.Sprintf("%d", e.id)
		if query != "" && !strings.Contains(strings.ToLower(label), query) && !strings.Contains(value, query) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: value,
		})
	}
	return choices
}

func (d *Discord) autocompleteRaidTypeChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	seen := map[string]bool{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
		seen["everything"] = true
	}

	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["raidLevels"].(map[string]any); ok {
			levels := []int{}
			for key := range raw {
				if value := toInt(key, 0); value > 0 {
					levels = append(levels, value)
				}
			}
			sort.Ints(levels)
			for _, level := range levels {
				value := fmt.Sprintf("level%d", level)
				if query == "" || strings.Contains(value, query) {
					seen[value] = true
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  d.raidLevelLabel(level),
						Value: value,
					})
				}
			}
		}
	}

	for _, choice := range d.autocompletePokemonChoices(query) {
		if len(choices) >= 25 {
			break
		}
		value := fmt.Sprintf("%v", choice.Value)
		if seen[value] {
			continue
		}
		seen[value] = true
		choices = append(choices, choice)
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteMaxbattleTypeChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	seen := map[string]bool{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
		seen["everything"] = true
	}

	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["maxbattleLevels"].(map[string]any); ok {
			levels := []int{}
			for key := range raw {
				if value := toInt(key, 0); value > 0 {
					levels = append(levels, value)
				}
			}
			sort.Ints(levels)
			for _, level := range levels {
				value := fmt.Sprintf("level%d", level)
				if query == "" || strings.Contains(value, query) {
					seen[value] = true
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  d.maxbattleLevelLabel(level),
						Value: value,
					})
				}
				if len(choices) >= 25 {
					break
				}
			}
		}
	}

	for _, choice := range d.autocompletePokemonChoices(query) {
		if len(choices) >= 25 {
			break
		}
		value := fmt.Sprintf("%v", choice.Value)
		if seen[value] {
			continue
		}
		seen[value] = true
		choices = append(choices, choice)
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteRaidLevelChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	raw, ok := d.manager.data.UtilData["raidLevels"].(map[string]any)
	if !ok {
		return nil
	}
	levels := []int{}
	for key := range raw {
		if value := toInt(key, 0); value > 0 {
			levels = append(levels, value)
		}
	}
	if len(levels) == 0 {
		return nil
	}
	sort.Ints(levels)
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(levels)+1)
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
	}
	for _, level := range levels {
		value := fmt.Sprintf("level%d", level)
		label := d.raidLevelLabel(level)
		if query == "" || strings.Contains(strings.ToLower(value), query) || strings.Contains(strings.ToLower(label), query) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  label,
				Value: value,
			})
		}
		if len(choices) >= 25 {
			break
		}
	}
	return choices
}

func (d *Discord) autocompleteGymChoices(i *discordgo.InteractionCreate, query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.scanner == nil {
		return nil
	}
	query = strings.TrimSpace(query)
	var entries []scanner.GymEntry
	var err error
	if query == "" {
		userID, _ := slashUser(i)
		if d.manager.query != nil && userID != "" {
			if row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID}); err == nil && row != nil {
				lat := toFloat(row["latitude"])
				lon := toFloat(row["longitude"])
				if lat != 0 || lon != 0 {
					entries, err = d.manager.scanner.SearchGymsNearby(lat, lon, 25)
				} else if d.manager.fences != nil {
					areas := parseAreaListFromHuman(row)
					if len(areas) > 0 {
						target := strings.ToLower(strings.TrimSpace(areas[0]))
						for _, fence := range d.manager.fences.Fences {
							if strings.EqualFold(strings.TrimSpace(fence.Name), target) {
								if centerLat, centerLon, ok := fenceCentroid(fence); ok {
									entries, err = d.manager.scanner.SearchGymsNearby(centerLat, centerLon, 25)
								}
								break
							}
						}
					}
				}
			}
		}
	}
	if entries == nil || len(entries) == 0 {
		entries, err = d.manager.scanner.SearchGyms(query, 25)
	}
	if err != nil {
		return nil
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" || entry.ID == "" {
			continue
		}
		if entry.HasCoords && d.manager != nil && d.manager.fences != nil {
			areas := d.manager.fences.MatchedAreas([]float64{entry.Latitude, entry.Longitude})
			if len(areas) > 0 && areas[0].Name != "" {
				name = fmt.Sprintf("%s (%s)", name, areas[0].Name)
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: entry.ID,
		})
	}
	return choices
}

func (d *Discord) autocompleteStationChoices(i *discordgo.InteractionCreate, query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.scanner == nil {
		return nil
	}
	query = strings.TrimSpace(query)
	var entries []scanner.StationEntry
	var err error
	if query == "" {
		userID, _ := slashUser(i)
		if d.manager.query != nil && userID != "" {
			if row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID}); err == nil && row != nil {
				lat := toFloat(row["latitude"])
				lon := toFloat(row["longitude"])
				if lat != 0 || lon != 0 {
					entries, err = d.manager.scanner.SearchStationsNearby(lat, lon, 25)
				} else if d.manager.fences != nil {
					areas := parseAreaListFromHuman(row)
					if len(areas) > 0 {
						target := strings.ToLower(strings.TrimSpace(areas[0]))
						for _, fence := range d.manager.fences.Fences {
							if strings.EqualFold(strings.TrimSpace(fence.Name), target) {
								if centerLat, centerLon, ok := fenceCentroid(fence); ok {
									entries, err = d.manager.scanner.SearchStationsNearby(centerLat, centerLon, 25)
								}
								break
							}
						}
					}
				}
			}
		}
	}
	if entries == nil || len(entries) == 0 {
		entries, err = d.manager.scanner.SearchStations(query, 25)
	}
	if err != nil {
		return nil
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" || entry.ID == "" {
			continue
		}
		if entry.HasCoords && d.manager != nil && d.manager.fences != nil {
			areas := d.manager.fences.MatchedAreas([]float64{entry.Latitude, entry.Longitude})
			if len(areas) > 0 && areas[0].Name != "" {
				name = fmt.Sprintf("%s (%s)", name, areas[0].Name)
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: entry.ID,
		})
	}
	return choices
}
