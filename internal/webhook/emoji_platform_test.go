package webhook

import (
	"testing"

	"poraclego/internal/data"
)

func TestWeatherInfoUsesPlatformEmojiOverrides(t *testing.T) {
	p := &Processor{
		data: &data.GameData{
			UtilData: map[string]any{
				"weather": map[string]any{
					"1": map[string]any{"name": "Clear", "emoji": "weather-clear"},
				},
				"emojis": map[string]any{
					"weather-clear": "DEFAULT",
				},
			},
		},
		customEmoji: map[string]map[string]string{
			"telegram": {"weather-clear": "TG"},
			"discord":  {"weather-clear": "DC"},
		},
	}

	_, emoji := weatherInfo(p, 1, "telegram", nil)
	if emoji != "TG" {
		t.Fatalf("telegram emoji=%q, want %q", emoji, "TG")
	}

	_, emoji = weatherInfo(p, 1, "discord", nil)
	if emoji != "DC" {
		t.Fatalf("discord emoji=%q, want %q", emoji, "DC")
	}
}

func TestGenderDataUsesPlatformEmojiOverrides(t *testing.T) {
	p := &Processor{
		data: &data.GameData{
			UtilData: map[string]any{
				"genders": map[string]any{
					"1": map[string]any{"name": "male", "emoji": "gender-male"},
				},
				"emojis": map[string]any{
					"gender-male": "DEFAULT",
				},
			},
		},
		customEmoji: map[string]map[string]string{
			"telegram": {"gender-male": "TG"},
		},
	}

	res := genderData(p, 1, "telegram", nil)
	if got := res["emoji"]; got != "TG" {
		t.Fatalf("gender emoji=%v, want %q", got, "TG")
	}
}

func TestGruntUnknownEmojiUsesPlatformOverrides(t *testing.T) {
	p := &Processor{
		data: &data.GameData{
			UtilData: map[string]any{
				"emojis": map[string]any{
					"grunt-unknown": "DEFAULT",
				},
			},
		},
		customEmoji: map[string]map[string]string{
			"telegram": {"grunt-unknown": "TG"},
		},
	}

	if got := gruntTypeEmoji(p, "whatever", "telegram"); got != "TG" {
		t.Fatalf("grunt-unknown emoji=%q, want %q", got, "TG")
	}
}
