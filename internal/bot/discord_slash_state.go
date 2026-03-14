package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/i18n"
)

func slashUser(i *discordgo.InteractionCreate) (string, string) {
	if i == nil {
		return "", ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID, i.Member.User.Username
	}
	if i.User != nil {
		return i.User.ID, i.User.Username
	}
	return "", ""
}

func (d *Discord) monsterSearchOptions(query string) []discordgo.SelectMenuOption {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
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
		if name == query || fmt.Sprintf("%d", id) == query {
			candidates = append(candidates, candidate{ID: id, Name: name})
			continue
		}
		if strings.HasPrefix(name, query) || strings.Contains(name, query) {
			candidates = append(candidates, candidate{ID: id, Name: name})
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	if len(candidates) > 25 {
		candidates = candidates[:25]
	}
	options := make([]discordgo.SelectMenuOption, 0, len(candidates))
	for _, mon := range candidates {
		label := fmt.Sprintf("%s (#%d)", d.titleCase(mon.Name), mon.ID)
		options = append(options, discordgo.SelectMenuOption{
			Label: label,
			Value: fmt.Sprintf("%d", mon.ID),
		})
	}
	return options
}

func (d *Discord) raidLevelOptions(tr *i18n.Translator) []discordgo.SelectMenuOption {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
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
	sort.Ints(levels)
	if len(levels) == 0 {
		return nil
	}
	options := make([]discordgo.SelectMenuOption, 0, len(levels))
	for _, level := range levels {
		options = append(options, discordgo.SelectMenuOption{
			Label: d.raidLevelLabel(level, tr),
			Value: fmt.Sprintf("level%d", level),
		})
	}
	if len(options) > 25 {
		options = options[:25]
	}
	return options
}

func (d *Discord) raidLevelLabel(level int, tr *i18n.Translator) string {
	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["raidLevels"].(map[string]any); ok {
			if label, ok := raw[fmt.Sprintf("%d", level)]; ok {
				text := strings.TrimSpace(fmt.Sprintf("%v", label))
				lower := strings.ToLower(text)
				if text != "" {
					if _, err := fmt.Sscanf(text, "%d", new(int)); err == nil {
						if tr != nil {
							return tr.TranslateFormat("Level {0}", level)
						}
						return fmt.Sprintf("Level %d", level)
					}
					if strings.Contains(lower, "level") || strings.Contains(lower, "tier") {
						if tr != nil {
							return tr.TranslateFormat("Level {0}", level)
						}
						return fmt.Sprintf("Level %d", level)
					}
					if !strings.Contains(lower, "raid") {
						raidWord := "raid"
						if tr != nil {
							raidWord = translateOrDefault(tr, "raid")
						}
						text += " " + raidWord
					}
					return d.titleCase(text)
				}
			}
		}
	}
	if tr != nil {
		return tr.TranslateFormat("Level {0}", level)
	}
	return fmt.Sprintf("Level %d", level)
}

func (d *Discord) maxbattleLevelLabel(level int, tr *i18n.Translator) string {
	if level == 90 {
		return translateOrDefault(tr, "All max battle levels")
	}
	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["maxbattleLevels"].(map[string]any); ok {
			if entry, ok := raw[fmt.Sprintf("%d", level)]; ok {
				label := strings.TrimSpace(fmt.Sprintf("%v", entry))
				if label != "" {
					return label
				}
			}
		}
	}
	if tr != nil {
		return tr.TranslateFormat("Level {0} max battle", level)
	}
	return fmt.Sprintf("Level %d max battle", level)
}

func (d *Discord) setSlashState(member *discordgo.Member, user *discordgo.User, state *slashBuilderState) {
	userID := slashUserID(member, user)
	if userID == "" {
		return
	}
	d.slashMu.Lock()
	d.slash[userID] = state
	d.slashMu.Unlock()
}

func (d *Discord) getSlashState(member *discordgo.Member, user *discordgo.User) *slashBuilderState {
	userID := slashUserID(member, user)
	if userID == "" {
		return nil
	}
	d.slashMu.Lock()
	state := d.slash[userID]
	d.slashMu.Unlock()
	return state
}

func (d *Discord) clearSlashState(member *discordgo.Member, user *discordgo.User) {
	userID := slashUserID(member, user)
	if userID == "" {
		return
	}
	d.slashMu.Lock()
	delete(d.slash, userID)
	d.slashMu.Unlock()
}
