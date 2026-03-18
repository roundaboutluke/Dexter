package webhook

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dexter/internal/i18n"
)

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func parseNumber(value string) (any, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, false
	}
	if i, err := strconv.Atoi(trimmed); err == nil {
		return i, true
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f, true
	}
	return nil, false
}

func ivColor(value any) int {
	iv := toFloat(value)
	switch {
	case iv >= 90:
		return 0x00FF00
	case iv >= 80:
		return 0x7FFF00
	case iv >= 66:
		return 0xFFFF00
	case iv >= 50:
		return 0xFFA500
	default:
		return 0xFF0000
	}
}

func moveName(p *Processor, moveID int) string {
	if moveID == 0 || p == nil {
		return ""
	}
	d := p.getData()
	if d == nil {
		return ""
	}
	raw, ok := d.Moves[fmt.Sprintf("%d", moveID)]
	if !ok {
		return ""
	}
	if m, ok := raw.(map[string]any); ok {
		if name, ok := m["name"].(string); ok {
			return name
		}
	}
	return ""
}

func moveEmoji(p *Processor, moveID int, platform string, tr *i18n.Translator) string {
	if moveID == 0 || p == nil {
		return ""
	}
	d := p.getData()
	if d == nil {
		return ""
	}
	raw, ok := d.Moves[fmt.Sprintf("%d", moveID)]
	if !ok {
		return ""
	}
	move, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	typeName := getString(move["type"])
	if typeName == "" {
		return ""
	}
	_, emojiKey := typeStyle(p, typeName)
	if emojiKey == "" {
		return ""
	}
	emoji := lookupEmojiForPlatform(p, emojiKey, platform)
	if emoji == "" {
		return ""
	}
	return translateMaybe(tr, emoji)
}

func gymName(hook *Hook) string {
	if name := getString(hook.Message["gym_name"]); name != "" {
		return name
	}
	if name := getString(hook.Message["name"]); name != "" {
		return name
	}
	return ""
}

func campfireLink(lat, lon float64, gymID any, gymName any, gymURL any) string {
	if lat == 0 && lon == 0 {
		return ""
	}
	marker := getString(gymID)
	if marker == "" {
		marker = generateMarkerID()
	}
	latStr := strconv.FormatFloat(lat, 'f', -1, 64)
	lonStr := strconv.FormatFloat(lon, 'f', -1, 64)
	deepLinkData := fmt.Sprintf("r=map&lat=%s&lng=%s&m=%s&g=PGO", latStr, lonStr, marker)
	encodedData := base64.StdEncoding.EncodeToString([]byte(deepLinkData))

	title := getString(gymName)
	if title == "" {
		title = "Gym"
	}
	image := getString(gymURL)
	if image == "" {
		image = "https://social.nianticlabs.com/images/gym-link-social-preview.png"
	}

	return fmt.Sprintf("https://campfire.onelink.me/eBr8?af_dp=campfire://&af_force_deeplink=true&deep_link_sub1=%s&af_og_title=%s&af_og_description=%%20&af_og_image=%s",
		encodedData,
		encodeURIComponent(title),
		encodeURIComponent(image),
	)
}

func generateMarkerID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		raw[0], raw[1], raw[2], raw[3],
		raw[4], raw[5],
		raw[6], raw[7],
		raw[8], raw[9],
		raw[10], raw[11], raw[12], raw[13], raw[14], raw[15],
	)
}

func teamInfo(teamID int) (string, int) {
	if teamID < 0 {
		return "", 0
	}
	switch teamID {
	case 1:
		return "Mystic", 0x1E90FF
	case 2:
		return "Valor", 0xFF0000
	case 3:
		return "Instinct", 0xFFFF00
	default:
		return "Neutral", 0x808080
	}
}

func encodeURIComponent(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	replacer := strings.NewReplacer(
		"%21", "!",
		"%27", "'",
		"%28", "(",
		"%29", ")",
		"%2A", "*",
		"%7E", "~",
	)
	return replacer.Replace(escaped)
}

func normalizeCampfireMarker(marker string) string {
	if marker == "" {
		return marker
	}
	if dot := strings.Index(marker, "."); dot > 0 {
		marker = marker[:dot]
	}
	lower := strings.ToLower(marker)
	if len(lower) != 32 {
		return marker
	}
	for _, r := range lower {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return marker
		}
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", lower[0:8], lower[8:12], lower[12:16], lower[16:20], lower[20:32])
}

func weatherInfo(p *Processor, weatherID int, platform string, tr *i18n.Translator) (string, string) {
	if p == nil {
		return "", ""
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return "", ""
	}
	weatherRaw, ok := d.UtilData["weather"].(map[string]any)
	if !ok {
		return "", ""
	}
	entry, ok := weatherRaw[fmt.Sprintf("%d", weatherID)].(map[string]any)
	if !ok {
		return "", ""
	}
	name := getString(entry["name"])
	emojiKey := getString(entry["emoji"])
	emoji := lookupEmojiForPlatform(p, emojiKey, platform)
	if tr != nil {
		name = tr.Translate(name, false)
		if emoji != "" {
			emoji = tr.Translate(emoji, false)
		}
	}
	return name, emoji
}

func weatherEntry(p *Processor, weatherID int) (string, string) {
	if p == nil {
		return "", ""
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return "", ""
	}
	weatherRaw, ok := d.UtilData["weather"].(map[string]any)
	if !ok {
		return "", ""
	}
	entry, ok := weatherRaw[fmt.Sprintf("%d", weatherID)].(map[string]any)
	if !ok {
		return "", ""
	}
	return getString(entry["name"]), getString(entry["emoji"])
}

func teamDetails(p *Processor, teamID int) (string, string, int) {
	name, color := teamInfo(teamID)
	emojiKey := ""
	if p != nil {
		d := p.getData()
		if d != nil && d.UtilData != nil {
			if teams, ok := d.UtilData["teams"].(map[string]any); ok {
				if entry, ok := teams[strconv.Itoa(teamID)].(map[string]any); ok {
					if entryName := getString(entry["name"]); entryName != "" {
						name = entryName
					}
					if entryEmoji := getString(entry["emoji"]); entryEmoji != "" {
						emojiKey = entryEmoji
					}
					if entryColor := getString(entry["color"]); entryColor != "" {
						if parsed, err := strconv.ParseInt(strings.TrimPrefix(entryColor, "#"), 16, 32); err == nil {
							color = int(parsed)
						}
					}
				}
			}
		}
	}
	return name, emojiKey, color
}

func raidLevelName(p *Processor, level int) string {
	if p == nil {
		return fmt.Sprintf("Level %d", level)
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return fmt.Sprintf("Level %d", level)
	}
	raw, ok := d.UtilData["raidLevels"].(map[string]any)
	if !ok {
		return fmt.Sprintf("Level %d", level)
	}
	if entry, ok := raw[strconv.Itoa(level)]; ok {
		if name := fmt.Sprintf("%v", entry); name != "" {
			return name
		}
	}
	return fmt.Sprintf("Level %d", level)
}

func maxbattleLevelName(p *Processor, level int) string {
	if p == nil {
		return fmt.Sprintf("Level %d", level)
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return fmt.Sprintf("Level %d", level)
	}
	raw, ok := d.UtilData["maxbattleLevels"].(map[string]any)
	if !ok {
		return fmt.Sprintf("Level %d", level)
	}
	if entry, ok := raw[strconv.Itoa(level)]; ok {
		if name := fmt.Sprintf("%v", entry); name != "" {
			return name
		}
	}
	return fmt.Sprintf("Level %d", level)
}

func evolutionName(p *Processor, evolutionID int) string {
	if p == nil || evolutionID == 0 {
		return ""
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return ""
	}
	raw, ok := d.UtilData["evolution"].(map[string]any)
	if !ok {
		return ""
	}
	entry, ok := raw[strconv.Itoa(evolutionID)].(map[string]any)
	if !ok {
		return ""
	}
	return getString(entry["name"])
}

func megaNameFormat(p *Processor, evolutionID int) string {
	if p == nil || evolutionID == 0 {
		return ""
	}
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return ""
	}
	raw, ok := d.UtilData["megaName"].(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", raw[strconv.Itoa(evolutionID)])
}

func translatorFor(p *Processor, language string) *i18n.Translator {
	if p == nil || p.i18n == nil {
		return nil
	}
	if language == "" && p.cfg != nil {
		if val, ok := p.cfg.GetString("general.locale"); ok {
			language = val
		}
	}
	return p.i18n.Translator(language)
}

func translateMaybe(tr *i18n.Translator, value string) string {
	if tr == nil || value == "" {
		return value
	}
	return tr.Translate(value, false)
}
