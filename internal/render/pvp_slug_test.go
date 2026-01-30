package render

import "testing"

func TestPvpSlugHelper_CommonPokemonNames(t *testing.T) {
	Init(t.TempDir(), nil, nil, nil)

	cases := map[string]string{
		"Mr. Mime":    "Mr_Mime",
		"Mime Jr.":    "Mime_Jr",
		"Ho-Oh":       "Ho_Oh",
		"Farfetch'd":  "Farfetchd",
		"Nidoran♀":    "Nidoran_Female",
		"Nidoran♂":    "Nidoran_Male",
		"Porygon-Z":   "Porygon_Z",
		"Tapu Koko":   "Tapu_Koko",
		"Mr Rime":     "Mr_Rime",
		"  Mr. Mime ": "Mr_Mime",
	}

	for input, want := range cases {
		out, err := RenderHandlebars("{{pvpSlug name}}", map[string]any{"name": input}, map[string]any{"language": "en"})
		if err != nil {
			t.Fatalf("RenderHandlebars error for %q: %v", input, err)
		}
		if out != want {
			t.Fatalf("pvpSlug(%q)=%q, want %q", input, out, want)
		}
	}
}

func TestPvpSlugHelper_LowercaseSubexpressionSupportsPvpoke(t *testing.T) {
	Init(t.TempDir(), nil, nil, nil)

	out, err := RenderHandlebars("{{lowercase (pvpSlug name)}}", map[string]any{"name": "Mr. Mime"}, map[string]any{"language": "en"})
	if err != nil {
		t.Fatalf("RenderHandlebars error: %v", err)
	}
	if out != "mr_mime" {
		t.Fatalf("unexpected output: %q", out)
	}
}
