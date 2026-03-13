package bot

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

const discordUnknownChannelCode = 10003

type discordFetchErrorInfo struct {
	permanentNotFound bool
	code              int
	status            int
	message           string
}

func classifyDiscordFetchError(err error) discordFetchErrorInfo {
	info := discordFetchErrorInfo{}
	if err == nil {
		return info
	}

	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) {
		if restErr.Response != nil {
			info.status = restErr.Response.StatusCode
		}
		if restErr.Message != nil {
			info.code = restErr.Message.Code
			info.message = restErr.Message.Message
		}
		if info.code == discordUnknownChannelCode || info.status == http.StatusNotFound {
			info.permanentNotFound = true
		}
		return info
	}

	return info
}

func (i discordFetchErrorInfo) summary() string {
	if i.code == 0 && i.status == 0 && i.message == "" {
		return "unknown discord error"
	}
	parts := []string{}
	if i.code != 0 {
		parts = append(parts, fmt.Sprintf("code=%d", i.code))
	}
	if i.status != 0 {
		parts = append(parts, fmt.Sprintf("status=%d", i.status))
	}
	if i.message != "" {
		parts = append(parts, fmt.Sprintf("message=%q", i.message))
	}
	if len(parts) == 0 {
		return "unknown discord error"
	}
	return fmt.Sprintf("%s", joinParts(parts))
}

func joinParts(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += " "
		}
		out += part
	}
	return out
}
