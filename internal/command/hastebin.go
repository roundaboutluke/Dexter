package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type hasteResponse struct {
	Key string `json:"key"`
}

func createHastebinLink(content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("empty content")
	}
	req, err := http.NewRequest(http.MethodPost, "https://hastebin.com/documents", bytes.NewBufferString(content))
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("hastebin http %d", resp.StatusCode)
	}
	var payload hasteResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Key == "" {
		return "", fmt.Errorf("hastebin empty key")
	}
	return "https://hastebin.com/" + payload.Key, nil
}
