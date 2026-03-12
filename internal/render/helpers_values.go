package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/aymerick/raymond"
)

func jsonHelper(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func uppercaseHelper(value interface{}) string {
	return strings.ToUpper(fmt.Sprintf("%v", value))
}

func lowercaseHelper(value interface{}) string {
	return strings.ToLower(fmt.Sprintf("%v", value))
}

func pvpSlugHelper(value interface{}) string {
	return pvpSlug(fmt.Sprintf("%v", value))
}

func pvpSlug(value string) string {
	slugLower := pvpSlugLower(value)
	if slugLower == "" {
		return ""
	}
	return titleCaseUnderscore(slugLower)
}

func titleCaseUnderscore(slug string) string {
	slug = strings.Trim(slug, "_")
	if slug == "" {
		return ""
	}
	var out strings.Builder
	out.Grow(len(slug))
	startWord := true
	for _, r := range slug {
		if r == '_' {
			out.WriteRune(r)
			startWord = true
			continue
		}
		if startWord {
			out.WriteRune(unicode.ToUpper(r))
			startWord = false
		} else {
			out.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.Trim(out.String(), "_")
}

func pvpSlugLower(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ToLower(value)

	var out strings.Builder
	out.Grow(len(value))
	lastUnderscore := false

	writeUnderscore := func() {
		if out.Len() == 0 || lastUnderscore {
			return
		}
		out.WriteByte('_')
		lastUnderscore = true
	}

	for _, r := range value {
		switch r {
		case '♀':
			if out.Len() > 0 && !lastUnderscore {
				out.WriteByte('_')
			}
			out.WriteString("female")
			lastUnderscore = false
			continue
		case '♂':
			if out.Len() > 0 && !lastUnderscore {
				out.WriteByte('_')
			}
			out.WriteString("male")
			lastUnderscore = false
			continue
		case ' ', '-', '–', '—', '/', '\\', ':':
			writeUnderscore()
			continue
		case '.', '\'', '’', '·':
			continue
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				out.WriteRune(r)
				lastUnderscore = false
				continue
			}
			writeUnderscore()
		}
	}

	slug := strings.Trim(out.String(), "_")
	for strings.Contains(slug, "__") {
		slug = strings.ReplaceAll(slug, "__", "_")
	}
	return slug
}

func capitalizeHelper(value interface{}) string {
	raw := fmt.Sprintf("%v", value)
	if raw == "" {
		return ""
	}
	return strings.ToUpper(raw[:1]) + raw[1:]
}

func exHelper(options *raymond.Options) string {
	if options == nil {
		return ""
	}
	match := truthy(options.Value("ex")) || truthy(options.Value("is_exclusive")) || truthy(options.Value("exclusive"))
	return boolHelper(match, options, "ex")
}
