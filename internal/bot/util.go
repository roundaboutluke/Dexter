package bot

import (
	"encoding/json"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
)

func containsID(cfg *config.Config, path string, id string) bool {
	values, _ := cfg.GetStringSlice(path)
	for _, value := range values {
		if value == id {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stripNonASCII(value string) string {
	if value == "" {
		return ""
	}
	out := make([]rune, 0, len(value))
	for _, r := range value {
		if r <= 0xFF {
			out = append(out, r)
		}
	}
	return string(out)
}

func parseCommandLines(text, prefix string, translator *i18n.Factory) []string {
	lines := strings.Split(text, "\n")
	out := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, prefix) {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if payload == "" {
			continue
		}
		args := splitQuotedArgs(normalizeQuotes(payload))
		if len(args) == 0 {
			continue
		}
		for i, arg := range args {
			arg = strings.ToLower(strings.ReplaceAll(arg, "_", " "))
			if translator != nil {
				arg = translator.ReverseTranslateCommand(arg, true)
			}
			args[i] = arg
		}
		cmd := args[0]
		params := args[1:]
		if cmd == "" {
			continue
		}
		if len(params) == 0 {
			out = append(out, cmd)
			continue
		}
		out = append(out, cmd+" "+joinArgs(params))
	}
	return out
}

func normalizeQuotes(text string) string {
	return strings.NewReplacer(
		"\u2018", "'",
		"\u2019", "'",
		"\u201C", "\"",
		"\u201D", "\"",
	).Replace(text)
}

func splitQuotedArgs(text string) []string {
	args := []string{}
	var buf strings.Builder
	inQuote := false
	for _, r := range text {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				buf.WriteRune(r)
			} else if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args
}

func joinArgs(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t") {
			parts = append(parts, `"`+arg+`"`)
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}

func toJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func splitDiscordMessage(text string, limit int) []string {
	if limit <= 0 {
		limit = 2000
	}
	if len(text) <= limit {
		return []string{text}
	}

	chunks := []string{}
	lines := strings.Split(text, "\n")
	var buf strings.Builder

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		chunks = append(chunks, buf.String())
		buf.Reset()
	}

	appendRaw := func(s string) {
		for len(s) > 0 {
			remaining := limit - buf.Len()
			if remaining <= 0 {
				flush()
				remaining = limit
			}
			if len(s) <= remaining {
				buf.WriteString(s)
				return
			}
			buf.WriteString(s[:remaining])
			s = s[remaining:]
			flush()
		}
	}

	for idx, line := range lines {
		if idx > 0 {
			// Prefer splitting on newlines, but fall back to hard splits if needed.
			if buf.Len()+1 > limit {
				flush()
			}
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
		}
		if len(line) > limit {
			flush()
			appendRaw(line)
			continue
		}
		if buf.Len() > 0 && buf.Len()+len(line) > limit {
			flush()
		}
		buf.WriteString(line)
	}
	flush()
	if len(chunks) == 0 {
		return []string{""}
	}
	return chunks
}
