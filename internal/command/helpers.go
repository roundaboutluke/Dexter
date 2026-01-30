package command

import (
	"encoding/json"
)

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func toJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func nullableString(value string) any {
	if value == "" || value == "[]" || value == "{}" {
		return nil
	}
	return value
}
