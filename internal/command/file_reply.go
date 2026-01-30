package command

import "encoding/json"

type fileReply struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Content string `json:"content"`
}

// FileReply builds a special reply payload that instructs platform handlers to send a text file.
// This is useful for platforms with strict message length limits (e.g. Discord interactions).
func FileReply(name, message, content string) string {
	return buildFileReply(name, message, content)
}

func buildFileReply(name, message, content string) string {
	payload, err := json.Marshal(fileReply{
		Name:    name,
		Message: message,
		Content: content,
	})
	if err != nil {
		return ""
	}
	return FileReplyPrefix + string(payload)
}
