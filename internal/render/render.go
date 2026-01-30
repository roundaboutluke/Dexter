package render

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aymerick/raymond"
)

// RenderHandlebars renders a Handlebars template with the given data and meta.
func RenderHandlebars(template string, data any, meta map[string]any) (string, error) {
	tpl, err := raymond.Parse(template)
	if err != nil {
		return "", err
	}
	df := raymond.NewDataFrame()
	for key, value := range meta {
		df.Set(key, value)
	}
	return tpl.ExecWith(data, df)
}

// RegisterPartials loads partials.json and registers them globally.
func RegisterPartials(root string) error {
	path := filepath.Join(configDir(root), "partials.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	clean := stripJSONComments(raw)
	var partials map[string]string
	if err := json.Unmarshal(clean, &partials); err != nil {
		return err
	}
	for name, tpl := range partials {
		raymond.RegisterPartial(name, tpl)
	}
	return nil
}

func configDir(root string) string {
	base := filepath.Join(root, "config")
	if dir := os.Getenv("NODE_CONFIG_DIR"); dir != "" {
		if filepath.IsAbs(dir) {
			base = dir
		} else {
			base = filepath.Join(root, dir)
		}
	}
	return base
}

func stripJSONComments(input []byte) []byte {
	out := make([]byte, 0, len(input))
	inString := false
	inSingleLine := false
	inMultiLine := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if inSingleLine {
			if c == '\n' {
				inSingleLine = false
				out = append(out, c)
			}
			continue
		}

		if inMultiLine {
			if c == '*' && i+1 < len(input) && input[i+1] == '/' {
				inMultiLine = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}

		if c == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				inSingleLine = true
				i++
				continue
			}
			if next == '*' {
				inMultiLine = true
				i++
				continue
			}
		}

		out = append(out, c)
	}

	return out
}
