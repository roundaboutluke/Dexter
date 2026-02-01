package i18n

import (
	"sync"

	"poraclego/internal/config"
)

// Factory creates translators for configured locales.
type Factory struct {
	root      string
	config    *config.Config
	mu        sync.Mutex
	cache     map[string]*Translator
	cmdCache  []*Translator
}

// NewFactory builds a translator factory.
func NewFactory(root string, cfg *config.Config) *Factory {
	return &Factory{root: root, config: cfg, cache: map[string]*Translator{}}
}

// Default returns the default locale translator.
func (f *Factory) Default() *Translator {
	locale, _ := f.config.GetString("general.locale")
	if locale == "" {
		locale = "en"
	}
	return f.Translator(locale)
}

// AltTranslator returns the secondary locale translator.
func (f *Factory) AltTranslator() *Translator {
	locale, _ := f.config.GetString("locale.language")
	if locale == "" {
		locale = "en"
	}
	return f.Translator(locale)
}

// CommandTranslators returns command translators for available languages.
func (f *Factory) CommandTranslators() []*Translator {
	f.mu.Lock()
	if f.cmdCache != nil {
		result := f.cmdCache
		f.mu.Unlock()
		return result
	}
	f.mu.Unlock()

	langs := f.availableLanguages()
	result := make([]*Translator, 0, len(langs)+2)
	for _, locale := range langs {
		result = append(result, f.Translator(locale))
	}
	if !contains(langs, "en") {
		result = append(result, f.Translator("en"))
	}
	defaultLocale, _ := f.config.GetString("general.locale")
	if defaultLocale != "" && !contains(langs, defaultLocale) && defaultLocale != "en" {
		result = append(result, f.Translator(defaultLocale))
	}

	f.mu.Lock()
	if f.cmdCache == nil {
		f.cmdCache = result
	}
	cached := f.cmdCache
	f.mu.Unlock()
	return cached
}

// ReverseTranslateCommand maps a command from translated to base.
func (f *Factory) ReverseTranslateCommand(key string, toLower bool) string {
	for _, tr := range f.CommandTranslators() {
		reversed := tr.Reverse(key, toLower)
		if reversed != key {
			return reversed
		}
	}
	return key
}

// TranslateCommand returns all translated variants of a command.
func (f *Factory) TranslateCommand(key string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, tr := range f.CommandTranslators() {
		translated := tr.Translate(key, false)
		if !seen[translated] {
			seen[translated] = true
			result = append(result, translated)
		}
	}
	return result
}

// Translator returns a locale translator.
func (f *Factory) Translator(locale string) *Translator {
	f.mu.Lock()
	if tr, ok := f.cache[locale]; ok {
		f.mu.Unlock()
		return tr
	}
	f.mu.Unlock()

	tr, err := NewTranslator(f.root, locale)
	if err != nil {
		tr = &Translator{data: map[string]string{}}
	}

	f.mu.Lock()
	// Double-check in case another goroutine created it while we were loading from disk.
	if existing, ok := f.cache[locale]; ok {
		f.mu.Unlock()
		return existing
	}
	f.cache[locale] = tr
	f.mu.Unlock()
	return tr
}

func (f *Factory) availableLanguages() []string {
	raw, ok := f.config.Get("general.availableLanguages")
	if !ok {
		return []string{}
	}
	if data, ok := raw.(map[string]any); ok {
		langs := make([]string, 0, len(data))
		for key := range data {
			langs = append(langs, key)
		}
		return langs
	}
	return []string{}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
