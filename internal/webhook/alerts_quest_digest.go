package webhook

import (
	"fmt"
	"time"
)

func questRewardSummary(hook *Hook) string {
	rewardType := getInt(hook.Message["reward_type"])
	reward := getInt(hook.Message["reward"])
	if rewardType == 0 {
		rewardType = getInt(hook.Message["reward_type_id"])
	}
	if reward == 0 {
		reward = getInt(hook.Message["pokemon_id"])
	}
	return fmt.Sprintf("%d:%d", rewardType, reward)
}

func questDigestTime(hook *Hook) time.Time {
	if hook == nil {
		return time.Now()
	}
	updated := getInt(hook.Message["updated"])
	if updated == 0 {
		return time.Now()
	}
	return time.Unix(int64(updated), 0)
}

func questDigestKey(hook *Hook) string {
	if hook == nil {
		return ""
	}
	if id := getString(hook.Message["pokestop_id"]); id != "" {
		return id
	}
	if id := getString(hook.Message["id"]); id != "" {
		return id
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat != 0 || lon != 0 {
		return fmt.Sprintf("%.5f,%.5f", lat, lon)
	}
	return ""
}

func questDigestStopText(hook *Hook) string {
	if hook == nil {
		return ""
	}
	name := getString(hook.Message["pokestop_name"])
	if name == "" {
		name = getString(hook.Message["name"])
	}
	return name
}
