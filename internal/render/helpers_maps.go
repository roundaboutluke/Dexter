package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unsafe"

	"github.com/aymerick/raymond"
	"github.com/aymerick/raymond/ast"
)

// isCurrentBlockHelper returns true when the helper currently executing is the helper that owns the
// active Handlebars block.
func isCurrentBlockHelper(options *raymond.Options, expected string) bool {
	if options == nil {
		return false
	}
	opt := reflect.ValueOf(options)
	if opt.Kind() != reflect.Pointer || opt.IsNil() {
		return false
	}
	optElem := opt.Elem()
	if optElem.Kind() != reflect.Struct {
		return false
	}
	evalVal := optElem.FieldByName("eval")
	if !evalVal.IsValid() || evalVal.Kind() != reflect.Pointer || evalVal.IsNil() {
		return false
	}
	evalElem := evalVal.Elem()
	if evalElem.Kind() != reflect.Struct {
		return false
	}
	blocksVal := evalElem.FieldByName("blocks")
	if !blocksVal.IsValid() || blocksVal.Kind() != reflect.Slice || !blocksVal.CanAddr() {
		return false
	}
	blocks := *(*[]*ast.BlockStatement)(unsafe.Pointer(blocksVal.UnsafeAddr()))
	if len(blocks) == 0 {
		return false
	}
	block := blocks[len(blocks)-1]
	if block == nil || block.Expression == nil {
		return false
	}
	return block.Expression.HelperName() == expected
}

func mapHelper(name interface{}, value interface{}, options *raymond.Options) string {
	language := ""
	if options != nil {
		language = options.DataStr("language")
	}
	entry := findMapEntry(fmt.Sprintf("%v", name), language)
	if entry == nil {
		return ""
	}
	mapping, _ := entry["map"].(map[string]any)
	result := mapping[fmt.Sprintf("%v", value)]
	if result == nil {
		return ""
	}
	if isCurrentBlockHelper(options, "map") {
		return options.FnWith(result)
	}
	return fmt.Sprintf("%v", result)
}

func map2Helper(name interface{}, value interface{}, value2 interface{}, options *raymond.Options) string {
	language := ""
	if options != nil {
		language = options.DataStr("language")
	}
	entry := findMapEntry(fmt.Sprintf("%v", name), language)
	if entry == nil {
		return ""
	}
	mapping, _ := entry["map"].(map[string]any)
	result := mapping[fmt.Sprintf("%v", value)]
	if result == nil {
		result = mapping[fmt.Sprintf("%v", value2)]
	}
	if result == nil {
		return ""
	}
	if isCurrentBlockHelper(options, "map2") {
		return options.FnWith(result)
	}
	return fmt.Sprintf("%v", result)
}

func concatHelper(params ...interface{}) string {
	if len(params) == 0 {
		return ""
	}
	if len(params) > 0 {
		if _, ok := params[len(params)-1].(*raymond.Options); ok {
			params = params[:len(params)-1]
		}
	}
	parts := make([]string, 0, len(params))
	for _, item := range params {
		parts = append(parts, fmt.Sprintf("%v", item))
	}
	return strings.Join(parts, "")
}

func loadCustomMaps(root string) []map[string]any {
	dir := filepath.Join(configDir(root), "customMaps")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []map[string]any{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var payload any
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}
		switch v := payload.(type) {
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					out = append(out, m)
				}
			}
		case map[string]any:
			out = append(out, v)
		}
	}
	return out
}

func loadCustomEmoji(root string) map[string]map[string]string {
	path := filepath.Join(configDir(root), "emoji.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	clean := stripJSONComments(raw)
	var payload map[string]map[string]string
	if err := json.Unmarshal(clean, &payload); err != nil {
		return nil
	}
	return payload
}

func findMapEntry(name, language string) map[string]any {
	for _, entry := range customMaps {
		if getString(entry["name"]) == name && getString(entry["language"]) == language {
			return entry
		}
	}
	for _, entry := range customMaps {
		if getString(entry["name"]) == name {
			return entry
		}
	}
	return nil
}
