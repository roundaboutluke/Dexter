package render

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/aymerick/raymond"

	"poraclego/internal/config"
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
	clean := config.StripJSONComments(raw)
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
