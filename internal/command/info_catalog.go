package command

import (
	"fmt"
	"sort"
	"strings"

	"poraclego/internal/i18n"
)

type catalogEntry struct {
	Name      string
	Secondary string
}

func infoMoves(ctx *Context) string {
	entries := []catalogEntry{}
	nameCounts := map[string]int{}
	for _, raw := range ctx.Data.Moves {
		if entry, ok := raw.(map[string]any); ok {
			name := fmt.Sprintf("%v", entry["name"])
			entries = append(entries, catalogEntry{Name: name, Secondary: fmt.Sprintf("%v", entry["type"])})
			nameCounts[name]++
		}
	}
	return formatCatalogEntries(ctx.I18n.Translator(ctx.Language), "Recognised moves:", entries, func(entry catalogEntry) bool {
		return entry.Secondary != "" && nameCounts[entry.Name] > 1
	})
}

func infoItems(ctx *Context) string {
	entries := []catalogEntry{}
	for _, raw := range ctx.Data.Items {
		if entry, ok := raw.(map[string]any); ok {
			entries = append(entries, catalogEntry{Name: fmt.Sprintf("%v", entry["name"])})
		}
	}
	return formatCatalogEntries(ctx.I18n.Translator(ctx.Language), "Recognised items:", entries, func(catalogEntry) bool {
		return false
	})
}

func formatCatalogEntries(
	tr *i18n.Translator,
	heading string,
	entries []catalogEntry,
	useSecondary func(catalogEntry) bool,
) string {
	translate := func(text string) string {
		if tr != nil {
			return tr.Translate(text, false)
		}
		return text
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name == entries[j].Name {
			return entries[i].Secondary < entries[j].Secondary
		}
		return entries[i].Name < entries[j].Name
	})
	lines := []string{translate(heading)}
	for _, entry := range entries {
		display := entry.Name
		translated := translate(entry.Name)
		if useSecondary != nil && useSecondary(entry) {
			display = fmt.Sprintf("%s/%s", entry.Name, entry.Secondary)
			translated = fmt.Sprintf("%s/%s", translate(entry.Name), translate(entry.Secondary))
		}
		display = strings.ReplaceAll(display, " ", "\\_")
		translated = strings.ReplaceAll(translated, " ", "\\_")
		if translated != display {
			lines = append(lines, fmt.Sprintf("%s or %s", display, translated))
			continue
		}
		lines = append(lines, display)
	}
	return strings.Join(lines, "\n")
}
