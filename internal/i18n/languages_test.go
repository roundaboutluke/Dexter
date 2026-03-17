package i18n

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"poraclego/internal/config"
)

func TestEffectiveLanguagesDiscoversShippedLocalesWhenConfigEmpty(t *testing.T) {
	root := t.TempDir()
	writeLocaleFile(t, root, "fr")
	writeLocaleFile(t, root, "de")
	writeUtilLocaleFile(t, root, "es")
	writeCustomLocaleFile(t, root, "it")

	factory := NewFactory(root, config.New(map[string]any{
		"general": map[string]any{
			"locale": "en",
		},
	}))

	got := factory.EffectiveLanguages()
	want := []string{"de", "fr", "it"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveLanguages()=%v, want %v", got, want)
	}
}

func TestEffectiveLanguagesUsesExplicitAllowlist(t *testing.T) {
	root := t.TempDir()
	writeLocaleFile(t, root, "fr")
	writeLocaleFile(t, root, "de")

	factory := NewFactory(root, config.New(map[string]any{
		"general": map[string]any{
			"locale":             "en",
			"availableLanguages": map[string]any{"fr": true},
		},
	}))

	got := factory.EffectiveLanguages()
	want := []string{"fr"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveLanguages()=%v, want %v", got, want)
	}
}

func TestCommandTranslatorsIncludeDefaultLocaleOutsideAllowlist(t *testing.T) {
	root := t.TempDir()
	writeLocaleDataFile(t, filepath.Join(root, "locale", "en.json"), map[string]string{
		"help": "help",
	})
	writeLocaleDataFile(t, filepath.Join(root, "locale", "fr.json"), map[string]string{
		"help": "aide",
	})

	factory := NewFactory(root, config.New(map[string]any{
		"general": map[string]any{
			"locale":             "fr",
			"availableLanguages": map[string]any{"en": true},
		},
	}))

	translated := factory.TranslateCommand("help")
	if !reflect.DeepEqual(translated, []string{"help", "aide"}) {
		t.Fatalf("TranslateCommand(help)=%v, want %v", translated, []string{"help", "aide"})
	}
	if got := factory.ReverseTranslateCommand("aide", true); got != "help" {
		t.Fatalf("ReverseTranslateCommand(aide)=%q, want %q", got, "help")
	}
}

func writeLocaleFile(t *testing.T, root, locale string) {
	t.Helper()
	writeLocaleDataFile(t, filepath.Join(root, "locale", locale+".json"), map[string]string{})
}

func writeUtilLocaleFile(t *testing.T, root, locale string) {
	t.Helper()
	writeLocaleDataFile(t, filepath.Join(root, "util", "locale", locale+".json"), map[string]string{})
}

func writeCustomLocaleFile(t *testing.T, root, locale string) {
	t.Helper()
	writeLocaleDataFile(t, filepath.Join(root, "config", "custom."+locale+".json"), map[string]string{})
}

func writeLocaleDataFile(t *testing.T, path string, data map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	payload := []byte(`{}`)
	if len(data) > 0 {
		payload = []byte("{\n")
	}
	if len(data) > 0 {
		first := true
		for key, value := range data {
			line := ""
			if !first {
				line += ",\n"
			}
			line += `  "` + key + `": "` + value + `"`
			payload = append(payload, []byte(line)...)
			first = false
		}
		payload = append(payload, []byte("\n}")...)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write locale file: %v", err)
	}
}
