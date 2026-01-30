package render

import "testing"

func TestMapHelperInlineInsideIfDoesNotRecurse(t *testing.T) {
	Init(t.TempDir(), nil, nil, nil)
	customMaps = []map[string]any{
		{
			"name":     "timeEmoji",
			"language": "en",
			"map": map[string]any{
				"1": "🕐",
			},
		},
	}

	out, err := RenderHandlebars("{{#if true}}{{map 'timeEmoji' 1}}{{/if}}", map[string]any{}, map[string]any{"language": "en"})
	if err != nil {
		t.Fatalf("RenderHandlebars returned error: %v", err)
	}
	if out != "🕐" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestMapHelperBlockUsesMappedValueAsContext(t *testing.T) {
	Init(t.TempDir(), nil, nil, nil)
	customMaps = []map[string]any{
		{
			"name":     "timeEmoji",
			"language": "en",
			"map": map[string]any{
				"1": "🕐",
			},
		},
	}

	out, err := RenderHandlebars("{{#map 'timeEmoji' 1}}X{{this}}Y{{/map}}", map[string]any{}, map[string]any{"language": "en"})
	if err != nil {
		t.Fatalf("RenderHandlebars returned error: %v", err)
	}
	if out != "X🕐Y" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestMapHelperMissingKeyRendersEmptyString(t *testing.T) {
	Init(t.TempDir(), nil, nil, nil)
	customMaps = []map[string]any{
		{
			"name":     "timeEmoji",
			"language": "en",
			"map": map[string]any{
				"1": "🕐",
			},
		},
	}

	out, err := RenderHandlebars("A{{map 'timeEmoji' 2}}B", map[string]any{}, map[string]any{"language": "en"})
	if err != nil {
		t.Fatalf("RenderHandlebars returned error: %v", err)
	}
	if out != "AB" {
		t.Fatalf("unexpected output: %q", out)
	}
}
