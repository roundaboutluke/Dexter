package webhook

import "fmt"

// BuildRenderDataForTool is a small helper for internal tooling (e.g. ./tools/renderdata_dump).
// It intentionally avoids exposing alertMatch/alertTarget types outside this package while still
// allowing fixture-based parity checks of the render payload used by templates.
func BuildRenderDataForTool(p *Processor, hook *Hook, platform, language, targetID string, row map[string]any) (map[string]any, error) {
	if p == nil || hook == nil {
		return nil, fmt.Errorf("missing processor or hook")
	}
	if row == nil {
		row = map[string]any{}
	}
	if platform == "" {
		platform = "discord"
	}
	match := alertMatch{
		Target: alertTarget{
			ID:       targetID,
			Platform: platform,
			Language: language,
		},
		Row: row,
	}
	return buildRenderData(p, hook, match), nil
}
