package dispatch

import (
	"encoding/json"
	"strconv"
	"strings"
)

func (s *Sender) discordPayload(job MessageJob) map[string]any {
	if job.Payload != nil {
		payload := sanitizeDiscordPayload(job.Payload)
		if _, hasEmbeds := payload["embeds"]; !hasEmbeds {
			if content, ok := payload["content"].(string); ok {
				if content == "" {
					payload["content"] = job.Message
				}
			} else {
				payload["content"] = job.Message
			}
		}
		return payload
	}
	return map[string]any{"content": job.Message}
}

func sanitizeDiscordPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return payload
	}

	// Normalize legacy single-embed payloads and malformed embeds objects so all callsites
	// can rely on `embeds` being the canonical list shape.
	if rawEmbed, ok := payload["embed"]; ok {
		if _, hasEmbeds := payload["embeds"]; !hasEmbeds {
			if embed, ok := rawEmbed.(map[string]any); ok {
				payload["embeds"] = []any{embed}
			}
		}
		delete(payload, "embed")
	}
	if rawEmbeds, ok := payload["embeds"]; ok {
		if embed, ok := rawEmbeds.(map[string]any); ok {
			payload["embeds"] = []any{embed}
		}
	}
	if embeds, ok := payload["embeds"].([]any); ok {
		for _, raw := range embeds {
			if embed, ok := raw.(map[string]any); ok {
				sanitizeEmbedColor(embed)
			}
		}
	}
	return payload
}

func sanitizeEmbedColor(embed map[string]any) {
	if embed == nil {
		return
	}
	raw, ok := embed["color"]
	if !ok {
		return
	}
	switch v := raw.(type) {
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			delete(embed, "color")
			return
		}
		hexCandidate := strings.TrimPrefix(value, "#")
		isHex := strings.HasPrefix(value, "#") || looksLikeHexColor(hexCandidate)
		// Avoid treating 8-digit *decimal* colors (e.g. "13369344") as ARGB hex.
		// These are commonly produced by templates like `"color": "{{color}}"` where `color` is an int.
		if !strings.HasPrefix(value, "#") && len(hexCandidate) == 8 && isAllDigits(hexCandidate) {
			isHex = false
		}
		if isHex {
			if len(hexCandidate) == 8 {
				hexCandidate = hexCandidate[2:]
			}
			parsed, err := strconv.ParseInt(hexCandidate, 16, 32)
			if err != nil {
				delete(embed, "color")
				return
			}
			if normalized, ok := normalizeEmbedColor(parsed); ok {
				embed["color"] = int(normalized)
			} else {
				delete(embed, "color")
			}
			return
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			delete(embed, "color")
			return
		}
		if normalized, ok := normalizeEmbedColor(int64(parsed)); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	case float64:
		if normalized, ok := normalizeEmbedColor(int64(v)); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	case int:
		if normalized, ok := normalizeEmbedColor(int64(v)); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	case int64:
		if normalized, ok := normalizeEmbedColor(v); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	}
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeEmbedColor(value int64) (int64, bool) {
	if value < 0 {
		return 0, false
	}
	if value <= 0xFFFFFF {
		return value, true
	}
	if value <= 0xFFFFFFFF {
		return value & 0xFFFFFF, true
	}
	return 0, false
}

func looksLikeHexColor(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return len(value) == 3 || len(value) == 6 || len(value) == 8
}

func extractEmbedImageURL(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if embeds, ok := payload["embeds"].([]any); ok && len(embeds) > 0 {
		if embed, ok := embeds[0].(map[string]any); ok {
			if url := extractImageURL(embed); url != "" {
				return url
			}
		}
	}
	if embed, ok := payload["embed"].(map[string]any); ok {
		return extractImageURL(embed)
	}
	return ""
}

func extractImageURL(embed map[string]any) string {
	if embed == nil {
		return ""
	}
	image, ok := embed["image"].(map[string]any)
	if !ok {
		return ""
	}
	url, _ := image["url"].(string)
	return url
}

func setEmbedImageURL(payload map[string]any, url string) bool {
	if payload == nil {
		return false
	}
	if embeds, ok := payload["embeds"].([]any); ok && len(embeds) > 0 {
		if embed, ok := embeds[0].(map[string]any); ok {
			setEmbedImage(embed, url)
			return true
		}
	}
	if embed, ok := payload["embed"].(map[string]any); ok {
		setEmbedImage(embed, url)
		return true
	}
	return false
}

func setEmbedImage(embed map[string]any, url string) {
	if embed == nil {
		return
	}
	image, ok := embed["image"].(map[string]any)
	if !ok {
		image = map[string]any{}
		embed["image"] = image
	}
	image["url"] = url
}

func clonePayload(payload map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var clone map[string]any
	if err := json.Unmarshal(raw, &clone); err != nil {
		return nil, err
	}
	return clone, nil
}
