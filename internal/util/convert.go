// Package util provides shared conversion helpers used across multiple packages.
package util

import (
	"fmt"
	"strconv"
	"strings"
)

// ToInt converts an arbitrary value to int, returning fallback if conversion fails.
// Handles int, int64, float64, float32, string (with TrimSpace), and []byte.
func ToInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.Atoi(strings.TrimSpace(string(v))); err == nil {
			return parsed
		}
	}
	return fallback
}

// ToFloat converts an arbitrary value to float64, returning fallback if conversion fails.
func ToFloat(value any, fallback float64) float64 {
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
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

// GetString converts an arbitrary value to string.
// Returns "" for nil, the value itself for string/[]byte, or fmt.Sprintf for other types.
func GetString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
