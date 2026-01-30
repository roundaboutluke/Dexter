package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	aliasOnce sync.Once
	aliasMap  map[string][]string
)

func loadPokemonAliases(root string) map[string][]string {
	aliasOnce.Do(func() {
		aliasMap = map[string][]string{}
		paths := []string{
			filepath.Join(root, "config", "pokemonAlias.json"),
			filepath.Join(root, "config", "defaults", "pokemonAlias.json"),
		}
		for _, path := range paths {
			payload, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var raw map[string]any
			if err := json.Unmarshal(payload, &raw); err != nil {
				continue
			}
			for key, value := range raw {
				switch v := value.(type) {
				case []any:
					values := make([]string, 0, len(v))
					for _, item := range v {
						if s, ok := item.(string); ok {
							values = append(values, s)
						} else {
							values = append(values, strings.TrimSpace(strings.ToLower(toString(item))))
						}
					}
					if len(values) > 0 {
						aliasMap[strings.ToLower(key)] = values
					}
				case string:
					if v != "" {
						aliasMap[strings.ToLower(key)] = []string{v}
					}
				default:
					if s := strings.TrimSpace(toString(value)); s != "" {
						aliasMap[strings.ToLower(key)] = []string{s}
					}
				}
			}
			if len(aliasMap) > 0 {
				break
			}
		}
	})
	return aliasMap
}

func expandPokemonAliases(ctx *Context, args []string) []string {
	aliases := loadPokemonAliases(ctx.Root)
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if replacements, ok := aliases[strings.ToLower(arg)]; ok {
			out = append(out, replacements...)
			continue
		}
		out = append(out, arg)
	}
	return out
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}
