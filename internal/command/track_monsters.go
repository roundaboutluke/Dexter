package command

import (
	"fmt"
	"strings"
)

type trackMonster struct {
	ID     int
	FormID int
}

func resolveMonsterIDs(ctx *Context, args []string, typeFilters []string, genMin int, genMax int, includeAll bool) []int {
	if includeAll {
		return filterMonsterIDs(ctx, allBaseMonsterIDs(ctx), typeFilters, genMin, genMax)
	}
	ids := monsterIDsFromArgs(ctx, args)
	if len(ids) == 0 && len(typeFilters) > 0 {
		ids = allBaseMonsterIDs(ctx)
	}
	return filterMonsterIDs(ctx, ids, typeFilters, genMin, genMax)
}

func isKnownForm(ctx *Context, formName string) bool {
	for _, raw := range ctx.Data.Monsters {
		if mon, ok := raw.(map[string]any); ok {
			if form, ok := mon["form"].(map[string]any); ok {
				if strings.EqualFold(fmt.Sprintf("%v", form["name"]), formName) {
					return true
				}
			}
		}
	}
	return false
}

func containsStringInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func uniqueInts(values []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func buildTrackMonsters(ctx *Context, ids []int, formNames []string, includeAllForms bool) []trackMonster {
	if len(ids) == 0 {
		return nil
	}
	if containsStringInt(ids, 0) && len(formNames) == 0 && !includeAllForms {
		return []trackMonster{{ID: 0, FormID: 0}}
	}
	if includeAllForms && ctx != nil && ctx.Data != nil && !containsStringInt(ids, 0) {
		idSet := map[int]bool{}
		for _, id := range ids {
			idSet[id] = true
		}
		baseForm := map[int]bool{}
		for _, raw := range ctx.Data.Monsters {
			mon, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := toInt(mon["id"], 0)
			if !idSet[id] {
				continue
			}
			form, _ := mon["form"].(map[string]any)
			if toInt(form["id"], 0) == 0 {
				baseForm[id] = true
			}
		}
		out := []trackMonster{}
		for _, raw := range ctx.Data.Monsters {
			mon, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := toInt(mon["id"], 0)
			if !idSet[id] {
				continue
			}
			form, _ := mon["form"].(map[string]any)
			formName := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", form["name"])))
			if baseForm[id] && formName == "normal" && toInt(form["id"], 0) != 0 {
				continue
			}
			out = append(out, trackMonster{ID: id, FormID: toInt(form["id"], 0)})
		}
		if len(out) > 0 {
			return out
		}
	}
	formLookup := map[string]bool{}
	for _, name := range formNames {
		formLookup[strings.ToLower(name)] = true
	}
	out := []trackMonster{}
	if len(formLookup) > 0 && ctx != nil && ctx.Data != nil {
		idSet := map[int]bool{}
		for _, id := range ids {
			idSet[id] = true
		}
		for _, raw := range ctx.Data.Monsters {
			mon, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := toInt(mon["id"], 0)
			if !idSet[id] {
				continue
			}
			form, _ := mon["form"].(map[string]any)
			formName := strings.ToLower(fmt.Sprintf("%v", form["name"]))
			if formLookup[formName] {
				out = append(out, trackMonster{ID: id, FormID: toInt(form["id"], 0)})
			}
		}
		return out
	}
	for _, id := range ids {
		out = append(out, trackMonster{ID: id, FormID: 0})
	}
	return out
}

func allBaseMonsterIDs(ctx *Context) []int {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	ids := []int{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		ids = append(ids, toInt(mon["id"], 0))
	}
	return uniqueInts(ids)
}

func filterMonsterIDs(ctx *Context, ids []int, typeFilters []string, genMin int, genMax int) []int {
	if ctx == nil || ctx.Data == nil {
		return ids
	}
	idSet := map[int]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	typeLookup := map[string]bool{}
	for _, t := range typeFilters {
		typeLookup[strings.ToLower(t)] = true
	}
	filtered := []int{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := toInt(mon["id"], 0)
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		if len(idSet) > 0 && !idSet[id] {
			continue
		}
		if genMin > 0 && genMax > 0 && (id < genMin || id > genMax) {
			continue
		}
		if len(typeLookup) > 0 {
			matches := false
			if types, ok := mon["types"].([]any); ok {
				for _, t := range types {
					if tm, ok := t.(map[string]any); ok {
						name := strings.ToLower(fmt.Sprintf("%v", tm["name"]))
						if typeLookup[name] {
							matches = true
							break
						}
					}
				}
			}
			if !matches {
				continue
			}
		}
		filtered = append(filtered, id)
	}
	return uniqueInts(filtered)
}

func monsterIDsFromArgs(ctx *Context, args []string) []int {
	ids := lookupMonsterIDs(ctx, args)
	for _, arg := range args {
		if !strings.HasSuffix(arg, "+") {
			continue
		}
		base := strings.TrimSuffix(arg, "+")
		base = strings.ToLower(ctx.I18n.ReverseTranslateCommand(base, true))
		if mon := findMonsterByNameOrID(ctx, base); mon != nil {
			id := toInt(mon["id"], 0)
			ids = append(ids, id)
			addEvolutions(ctx, mon, &ids, 0)
		}
	}
	return uniqueInts(ids)
}

func findMonsterByNameOrID(ctx *Context, query string) map[string]any {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := fmt.Sprintf("%v", mon["id"])
		if query == name || query == id {
			return mon
		}
	}
	return nil
}

func addEvolutions(ctx *Context, mon map[string]any, ids *[]int, depth int) {
	if ctx == nil || ctx.Data == nil || mon == nil || depth > 20 {
		return
	}
	raw, ok := mon["evolutions"].([]any)
	if !ok {
		return
	}
	for _, entry := range raw {
		evo, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		evoID := toInt(evo["evoId"], 0)
		formID := toInt(evo["id"], 0)
		if evoID == 0 {
			continue
		}
		*ids = append(*ids, evoID)
		key := fmt.Sprintf("%d_%d", evoID, formID)
		if next, ok := ctx.Data.Monsters[key].(map[string]any); ok {
			addEvolutions(ctx, next, ids, depth+1)
		}
	}
}
