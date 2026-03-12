package webhook

import "testing"

func TestMatchQuestWithDataUnsupportedRewardTypeMatchesNone(t *testing.T) {
	hook := &Hook{
		Type: "quest",
		Message: map[string]any{
			"reward_type": 99,
			"reward":      123,
		},
	}

	row := map[string]any{
		"reward_type": 99,
		"reward":      123,
		"amount":      0,
		"form":        0,
		"shiny":       0,
	}

	rewardData := questRewardData(nil, hook)
	if got := matchQuestWithData(hook, row, rewardData); got {
		t.Fatalf("matchQuestWithData=true, want false")
	}
}
