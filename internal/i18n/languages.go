package i18n

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dexter/internal/config"
)

// EffectiveLanguages returns the configured language allowlist, or auto-discovers
// shipped locale files when no explicit allowlist is set.
func (f *Factory) EffectiveLanguages() []string {
	if f == nil {
		return nil
	}
	return effectiveLanguages(f.root, f.config)
}

// HasRuntimeLocale reports whether a locale has a full runtime locale pack.
func (f *Factory) HasRuntimeLocale(locale string) bool {
	if f == nil {
		return false
	}
	return hasDiscoveredLanguage(f.root, locale)
}

func effectiveLanguages(root string, cfg *config.Config) []string {
	if configured := configuredLanguages(cfg); len(configured) > 0 {
		return configured
	}
	return discoveredLanguages(root)
}

func configuredLanguages(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	raw, ok := cfg.Get("general.availableLanguages")
	if !ok {
		return nil
	}
	return normalizeLanguages(raw)
}

func normalizeLanguages(raw any) []string {
	seen := map[string]bool{}
	langs := []string{}
	add := func(value string) {
		locale := strings.ToLower(strings.TrimSpace(value))
		if locale == "" || seen[locale] {
			return
		}
		seen[locale] = true
		langs = append(langs, locale)
	}

	switch v := raw.(type) {
	case map[string]any:
		for key := range v {
			add(key)
		}
	case []string:
		for _, value := range v {
			add(value)
		}
	case []any:
		for _, value := range v {
			if text, ok := value.(string); ok {
				add(text)
			}
		}
	}

	sort.Strings(langs)
	return langs
}

func discoveredLanguages(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	seen := map[string]bool{}
	langs := []string{}
	langs = appendLocaleDirLanguages(langs, seen, filepath.Join(root, "locale"))
	langs = appendCustomLocaleLanguages(langs, seen, filepath.Join(root, "config"))
	sort.Strings(langs)
	return langs
}

func hasDiscoveredLanguage(root, locale string) bool {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "" {
		return false
	}
	for _, discovered := range discoveredLanguages(root) {
		if discovered == locale {
			return true
		}
	}
	return false
}

func appendLocaleDirLanguages(langs []string, seen map[string]bool, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return langs
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		locale := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		if locale == "" || seen[locale] {
			continue
		}
		seen[locale] = true
		langs = append(langs, locale)
	}
	return langs
}

func appendCustomLocaleLanguages(langs []string, seen map[string]bool, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return langs
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		locale := customLocaleName(entry.Name())
		if locale == "" || seen[locale] {
			continue
		}
		seen[locale] = true
		langs = append(langs, locale)
	}
	return langs
}

func customLocaleName(name string) string {
	if !strings.HasPrefix(name, "custom.") || filepath.Ext(name) != ".json" {
		return ""
	}
	locale := strings.TrimPrefix(name, "custom.")
	locale = strings.TrimSuffix(locale, filepath.Ext(name))
	return strings.ToLower(strings.TrimSpace(locale))
}
