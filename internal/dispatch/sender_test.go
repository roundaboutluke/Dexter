package dispatch

import "testing"

func TestSanitizeEmbedColor_ParsesDecimalEightDigit(t *testing.T) {
	embed := map[string]any{"color": "13369344"} // 0xCC0000 (Valor) in decimal
	sanitizeEmbedColor(embed)
	if got, ok := embed["color"].(int); !ok || got != 13369344 {
		t.Fatalf("expected decimal color 13369344, got %#v", embed["color"])
	}
}

func TestSanitizeEmbedColor_ParsesHexSixDigitNoHash(t *testing.T) {
	embed := map[string]any{"color": "FFDE00"} // 0xFFDE00 (Instinct) in hex
	sanitizeEmbedColor(embed)
	if got, ok := embed["color"].(int); !ok || got != 0xFFDE00 {
		t.Fatalf("expected hex color 0xFFDE00, got %#v", embed["color"])
	}
}

func TestSanitizeEmbedColor_ParsesARGBHash(t *testing.T) {
	embed := map[string]any{"color": "#80FFDE00"} // alpha + 0xFFDE00
	sanitizeEmbedColor(embed)
	if got, ok := embed["color"].(int); !ok || got != 0xFFDE00 {
		t.Fatalf("expected ARGB-stripped color 0xFFDE00, got %#v", embed["color"])
	}
}

func TestSanitizeEmbedColor_ParsesHexSixDigitDigitsOnlyNoHash(t *testing.T) {
	embed := map[string]any{"color": "808080"} // grey in hex
	sanitizeEmbedColor(embed)
	if got, ok := embed["color"].(int); !ok || got != 0x808080 {
		t.Fatalf("expected hex color 0x808080, got %#v", embed["color"])
	}
}

func TestDiscordPayload_NormalizesSingleEmbedAndSkipsContentAutofill(t *testing.T) {
	sender := &Sender{}
	job := MessageJob{
		Message: "fallback",
		Payload: map[string]any{
			"embed": map[string]any{
				"title":       "Quest Digest",
				"description": "desc",
				"color":       "FFDE00",
			},
		},
	}

	got := sender.discordPayload(job)

	if _, ok := got["embed"]; ok {
		t.Fatalf("expected legacy embed key to be removed")
	}
	if _, ok := got["content"]; ok {
		t.Fatalf("expected no content autofill when embeds are present, got %#v", got["content"])
	}
	embeds, ok := got["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected embeds array of len 1, got %#v", got["embeds"])
	}
	first, ok := embeds[0].(map[string]any)
	if !ok {
		t.Fatalf("expected embed element to be map, got %#v", embeds[0])
	}
	if color, ok := first["color"].(int); !ok || color != 0xFFDE00 {
		t.Fatalf("expected normalized color 0xFFDE00, got %#v", first["color"])
	}
}

func TestDiscordPayload_NormalizesEmbedsObjectAndSkipsContentAutofill(t *testing.T) {
	sender := &Sender{}
	job := MessageJob{
		Message: "fallback",
		Payload: map[string]any{
			"embeds": map[string]any{
				"title":       "Quest Digest",
				"description": "desc",
			},
		},
	}

	got := sender.discordPayload(job)

	if _, ok := got["content"]; ok {
		t.Fatalf("expected no content autofill when embeds are present, got %#v", got["content"])
	}
	embeds, ok := got["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected embeds array of len 1, got %#v", got["embeds"])
	}
	if _, ok := embeds[0].(map[string]any); !ok {
		t.Fatalf("expected embed element to be map, got %#v", embeds[0])
	}
}
