package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds Dexter configuration data.
type Config struct {
	data map[string]any
}

// New creates a Config from a map. Primarily used for tests and tooling.
func New(data map[string]any) *Config {
	if data == nil {
		data = map[string]any{}
	}
	return &Config{data: data}
}

// Load reads default config, environment overrides, and local config.
func Load(root string) (*Config, error) {
	base := filepath.Join(root, "config")
	if dir := os.Getenv("NODE_CONFIG_DIR"); dir != "" {
		if filepath.IsAbs(dir) {
			base = dir
		} else {
			base = filepath.Join(root, dir)
		}
	}
	cfg := map[string]any{}

	defaultPath := filepath.Join(base, "default.json")
	if err := loadJSONFileInto(cfg, defaultPath); err != nil {
		return nil, fmt.Errorf("load default config: %w", err)
	}

	envName := os.Getenv("NODE_CONFIG_ENV")
	if envName == "" {
		envName = os.Getenv("NODE_ENV")
	}
	if envName == "" {
		envName = "production"
	}
	envPath := filepath.Join(base, fmt.Sprintf("%s.json", envName))
	if fileExists(envPath) {
		if err := loadJSONFileInto(cfg, envPath); err != nil {
			return nil, fmt.Errorf("load env config: %w", err)
		}
	}

	localPath := filepath.Join(base, "local.json")
	if fileExists(localPath) {
		if err := loadJSONFileInto(cfg, localPath); err != nil {
			return nil, fmt.Errorf("load local config: %w", err)
		}
	}
	rootLocalPath := filepath.Join(root, "local.json")
	if rootLocalPath != localPath && fileExists(rootLocalPath) {
		if err := loadJSONFileInto(cfg, rootLocalPath); err != nil {
			return nil, fmt.Errorf("load root local config: %w", err)
		}
	}
	parent := filepath.Dir(root)
	if parent != root {
		parentConfigLocal := filepath.Join(parent, "config", "local.json")
		if parentConfigLocal != localPath && fileExists(parentConfigLocal) {
			if err := loadJSONFileInto(cfg, parentConfigLocal); err != nil {
				return nil, fmt.Errorf("load parent config local: %w", err)
			}
		}
		parentLocal := filepath.Join(parent, "local.json")
		if parentLocal != localPath && parentLocal != rootLocalPath && fileExists(parentLocal) {
			if err := loadJSONFileInto(cfg, parentLocal); err != nil {
				return nil, fmt.Errorf("load parent local: %w", err)
			}
		}
	}

	envMapPath := filepath.Join(base, "custom-environment-variables.json")
	if fileExists(envMapPath) {
		mapping, err := loadJSONFile(envMapPath)
		if err != nil {
			return nil, fmt.Errorf("load env mapping: %w", err)
		}
		if err := applyEnvOverrides(cfg, mapping); err != nil {
			return nil, fmt.Errorf("apply env overrides: %w", err)
		}
	}

	// Node-config supports overriding configuration via NODE_CONFIG, which contains a JSON object.
	// Apply it last so it acts as the highest-priority overlay.
	if raw := strings.TrimSpace(os.Getenv("NODE_CONFIG")); raw != "" {
		var overlay map[string]any
		if err := json.Unmarshal([]byte(raw), &overlay); err != nil {
			return nil, fmt.Errorf("parse NODE_CONFIG: %w", err)
		}
		mergeMaps(cfg, overlay)
	}

	return &Config{data: cfg}, nil
}

// Get returns the raw config value at a dotted path.
func (c *Config) Get(path string) (any, bool) {
	parts := strings.Split(path, ".")
	cur := any(c.data)
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// GetString returns a string value at a dotted path.
func (c *Config) GetString(path string) (string, bool) {
	val, ok := c.Get(path)
	if !ok {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}

// GetBool returns a bool value at a dotted path.
func (c *Config) GetBool(path string) (bool, bool) {
	val, ok := c.Get(path)
	if !ok {
		return false, false
	}
	switch v := val.(type) {
	case bool:
		return v, true
	case string:
		parsed, err := strconv.ParseBool(v)
		return parsed, err == nil
	case float64:
		return v != 0, true
	default:
		return false, false
	}
}

// GetInt returns an int value at a dotted path.
func (c *Config) GetInt(path string) (int, bool) {
	val, ok := c.Get(path)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		parsed, err := strconv.Atoi(v)
		return parsed, err == nil
	default:
		return 0, false
	}
}

// GetStringSlice returns a string slice at a dotted path.
func (c *Config) GetStringSlice(path string) ([]string, bool) {
	val, ok := c.Get(path)
	if !ok {
		return nil, false
	}
	switch v := val.(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch s := item.(type) {
			case string:
				out = append(out, s)
			default:
				out = append(out, fmt.Sprintf("%v", s))
			}
		}
		return out, true
	case string:
		if v == "" {
			return []string{}, true
		}
		return []string{v}, true
	default:
		return nil, false
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadJSONFileInto(dst map[string]any, path string) error {
	data, err := loadJSONFile(path)
	if err != nil {
		return err
	}
	mergeMaps(dst, data)
	return nil
}

func loadJSONFile(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	clean := stripJSONComments(raw)
	var decoded map[string]any
	if err := json.Unmarshal(clean, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func mergeMaps(dst map[string]any, src map[string]any) {
	for key, value := range src {
		srcMap, ok := value.(map[string]any)
		if !ok {
			dst[key] = value
			continue
		}
		dstMap, ok := dst[key].(map[string]any)
		if !ok {
			dst[key] = cloneMap(srcMap)
			continue
		}
		mergeMaps(dstMap, srcMap)
	}
}

func cloneMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func applyEnvOverrides(cfg map[string]any, mapping map[string]any) error {
	for key, raw := range mapping {
		if err := applyEnvOverride(cfg, []string{key}, raw); err != nil {
			return err
		}
	}
	return nil
}

func applyEnvOverride(cfg map[string]any, path []string, raw any) error {
	switch val := raw.(type) {
	case map[string]any:
		name, hasName := val["__name"].(string)
		format, _ := val["__format"].(string)
		if hasName {
			envVal, ok := os.LookupEnv(name)
			if !ok {
				return nil
			}
			parsed, err := parseEnvValue(envVal, format)
			if err != nil {
				return fmt.Errorf("parse env %s: %w", name, err)
			}
			setAtPath(cfg, path, parsed)
			return nil
		}
		for k, v := range val {
			if strings.HasPrefix(k, "__") {
				continue
			}
			if err := applyEnvOverride(cfg, append(path, k), v); err != nil {
				return err
			}
		}
		return nil
	case string:
		envVal, ok := os.LookupEnv(val)
		if !ok {
			return nil
		}
		parsed, err := parseEnvValue(envVal, "")
		if err != nil {
			return fmt.Errorf("parse env %s: %w", val, err)
		}
		setAtPath(cfg, path, parsed)
		return nil
	default:
		return errors.New("invalid env mapping")
	}
}

func parseEnvValue(value string, format string) (any, error) {
	if format == "json" {
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	}
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed, nil
	}
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		return parsed, nil
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return parsed, nil
	}
	return value, nil
}

func setAtPath(cfg map[string]any, path []string, value any) {
	cur := cfg
	for i, part := range path {
		if i == len(path)-1 {
			cur[part] = value
			return
		}
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
}
