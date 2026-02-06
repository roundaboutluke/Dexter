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
