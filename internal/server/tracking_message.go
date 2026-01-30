package server

import (
	"strings"

	"poraclego/internal/dispatch"
)

func sendTrackingMessage(s *Server, human map[string]any, message, language string) {
	if s == nil || human == nil {
		return
	}
	targetID := getString(human["id"])
	targetType := getString(human["type"])
	if targetID == "" || targetType == "" {
		return
	}
	job := dispatch.MessageJob{
		Lat:          0,
		Lon:          0,
		Message:      message,
		Target:       targetID,
		Type:         targetType,
		Name:         getString(human["name"]),
		TTH:          dispatch.TimeToHide{Hours: 1, Minutes: 0, Seconds: 0},
		Clean:        false,
		Emoji:        "",
		LogReference: "WebApi",
		Language:     language,
	}

	switch {
	case targetType == "webhook" || strings.HasPrefix(targetType, "discord:"):
		if s.discordQueue != nil {
			s.discordQueue.Push(job)
		}
	case strings.HasPrefix(targetType, "telegram:"):
		if s.telegramQueue != nil {
			s.telegramQueue.Push(job)
		}
	}
}
