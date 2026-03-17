package webhook

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"poraclego/internal/logging"
	"poraclego/internal/util"
)

func normalizeHook(item any) (*Hook, bool) {
	raw, ok := item.(map[string]any)
	if !ok {
		return nil, false
	}
	hookType, ok := raw["type"].(string)
	if !ok || hookType == "" {
		return nil, false
	}
	message := map[string]any{}
	switch v := raw["message"].(type) {
	case map[string]any:
		message = v
	default:
		if logger := logging.Get().Webhooks; logger != nil {
			logger.Debugf("normalizeHook: message field is not map[string]any for type %s, using flat fallback", hookType)
		}
		for key, value := range raw {
			if key == "type" {
				continue
			}
			message[key] = value
		}
	}
	return &Hook{Type: hookType, Message: message}, true
}

func expiryTTL(expireUnix int64, buffer time.Duration) time.Duration {
	if expireUnix == 0 {
		return 0
	}
	remaining := time.Until(time.Unix(expireUnix, 0))
	if remaining < 0 {
		remaining = 0
	}
	return remaining + buffer
}

var getString = util.GetString

func getInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return int(parsed)
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return int(parsed)
		}
	}
	return 0
}

func getInt64(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

func getBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v == "true" || v == "1"
	case []byte:
		s := strings.TrimSpace(string(v))
		return s == "true" || s == "1"
	}
	return false
}

// gymInBattle mirrors PoracleJS gym handling:
// `const inBattle = hook.message.is_in_battle ?? hook.message.in_battle ?? 0`
// followed by normal JS truthiness checks.
func gymInBattle(message map[string]any) bool {
	if message == nil {
		return false
	}
	if raw, ok := message["is_in_battle"]; ok && raw != nil {
		return jsTruthy(raw)
	}
	if raw, ok := message["in_battle"]; ok && raw != nil {
		return jsTruthy(raw)
	}
	return false
}

func jsTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case float32:
		return v != 0
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed != 0
		}
		return false
	case string:
		return v != ""
	case []byte:
		return len(v) > 0
	default:
		// Non-null objects/arrays are truthy in JS.
		return true
	}
}

func teamFromHookMessage(message map[string]any) int {
	if message == nil {
		return 0
	}
	// Match PoracleJS nullish coalescing behavior: `team_id ?? team` (0 is a real value).
	if raw, ok := message["team_id"]; ok && raw != nil {
		return getInt(raw)
	}
	if raw, ok := message["team"]; ok && raw != nil {
		return getInt(raw)
	}
	return 0
}

func getFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return parsed
		}
	}
	return 0
}
