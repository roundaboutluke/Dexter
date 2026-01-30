package render

import "testing"

func TestPvpSlugHelper_CommonPokemonNames(t *testing.T) {
	Init(t.TempDir(), nil, nil, nil)

	cases := map[string]string{
		"Mr. Mime":    "mr_mime",
		"Mime Jr.":    "mime_jr",
		"Ho-Oh":       "ho_oh",
		"Farfetch'd":  "farfetchd",
		"Nidoran♀":    "nidoran_female",
		"Nidoran♂":    "nidoran_male",
		"Porygon-Z":   "porygon_z",
		"Tapu Koko":   "tapu_koko",
		"Mr Rime":     "mr_rime",
		"  Mr. Mime ": "mr_mime",
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

