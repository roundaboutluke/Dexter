package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Translator handles locale translations.
type Translator struct {
	data map[string]string
}

// NewTranslator loads locale data from disk.
func NewTranslator(root string, region string) (*Translator, error) {
	data := map[string]string{}
	loadLocaleFile(data, filepath.Join(root, "util", "locale", fmt.Sprintf("%s.json", region)))
	loadLocaleFile(data, filepath.Join(root, "locale", fmt.Sprintf("%s.json", region)))
	loadLocaleFile(data, filepath.Join(root, "locale", "slash", fmt.Sprintf("%s.json", region)))
	loadLocaleFile(data, filepath.Join(root, "config", fmt.Sprintf("custom.%s.json", region)))

	for key, value := range data {
		if strings.EqualFold(key, value) {
			delete(data, key)
		}
	}

	return &Translator{data: data}, nil
}

// Translate returns the translated string.
func (t *Translator) Translate(bit string, lowercase bool) string {
	if !lowercase {
		if value, ok := t.data[bit]; ok {
			return value
		}
		return bit
	}

	lower := strings.ToLower(bit)
	for key, value := range t.data {
		if strings.ToLower(key) == lower {
			return value
		}
	}
	return lower
}

// TranslateFormat applies positional formatting after translation.
func (t *Translator) TranslateFormat(bit string, args ...any) string {
	return formatString(t.Translate(bit, false), args...)
}

// Reverse returns the key for a translated value.
func (t *Translator) Reverse(bit string, lowercase bool) string {
	if !lowercase {
		for key, value := range t.data {
			if value == bit {
				return key
			}
		}
		return bit
	}

	lower := strings.ToLower(bit)
	for key, value := range t.data {
		if strings.ToLower(value) == lower && strings.ToLower(key) != lower {
			return strings.ToLower(key)
		}
	}
	return bit
}

func loadLocaleFile(dst map[string]string, path string) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var raw map[string]string
	if err := json.Unmarshal(payload, &raw); err != nil {
		return
	}
	for key, value := range raw {
		dst[key] = value
	}
}

func formatString(input string, args ...any) string {
	result := input
	for i := len(args) - 1; i >= 0; i-- {
		re := regexp.MustCompile(fmt.Sprintf("\\{%d\\}", i))
		result = re.ReplaceAllString(result, fmt.Sprintf("%v", args[i]))
	}
	return result
}
