package dispatch

import "testing"

func TestPlatformInfo(t *testing.T) {
	tests := []struct {
		jobType      string
		wantPlatform string
		wantSubType  string
	}{
		{"webhook", "discord", "webhook"},
		{"discord:channel", "discord", "channel"},
		{"discord:user", "discord", "user"},
		{"telegram:user", "telegram", "user"},
		{"telegram:channel", "telegram", "channel"},
		{"telegram:group", "telegram", "group"},
		{"unknown", "unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.jobType, func(t *testing.T) {
			platform, subType := platformInfo(tt.jobType)
			if platform != tt.wantPlatform {
				t.Errorf("platform = %q, want %q", platform, tt.wantPlatform)
			}
			if subType != tt.wantSubType {
				t.Errorf("subType = %q, want %q", subType, tt.wantSubType)
			}
		})
	}
}
