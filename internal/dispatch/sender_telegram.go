package dispatch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dexter/internal/logging"
)

func (s *Sender) sendTelegram(chatID string, job MessageJob) error {
	token := selectToken(s.cfg, "telegram.token", chatID)
	if token == "" {
		return fmt.Errorf("telegram token missing")
	}
	if job.UpdateExisting && job.UpdateKey != "" {
		if entry, ok := s.updateTelegram.Get(chatID, job.UpdateKey); ok && entry.MessageID != "" {
			payload := map[string]any{
				"chat_id": chatID,
				"text":    job.Message,
			}
			if job.Payload != nil {
				parseMode := "Markdown"
				if raw, ok := job.Payload["parse_mode"].(string); ok && strings.TrimSpace(raw) != "" {
					parseMode = normalizeTelegramParseMode(raw)
				}
				payload["parse_mode"] = parseMode
				if preview, ok := job.Payload["webpage_preview"].(bool); ok {
					payload["disable_web_page_preview"] = !preview
				} else {
					// PoracleJS defaults to disabling link previews when the field is missing.
					payload["disable_web_page_preview"] = true
				}
			}
			if err := s.editTelegramMessage(token, chatID, entry.MessageID, payload); err == nil {
				s.logJobf(logging.LevelDebug, job, "updated existing telegram message %s", entry.MessageID)
				return nil
			}
		}
	}
	messageIDs := []int{}
	sendOrder := telegramSendOrder(job.Payload)
	hasText := containsString(sendOrder, "text")
	for _, entry := range sendOrder {
		switch entry {
		case "sticker":
			sticker := stringFromAny(job.Payload, "sticker")
			if strings.TrimSpace(sticker) == "" {
				continue
			}
			endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendSticker", token)
			payload := map[string]any{
				"chat_id":              chatID,
				"sticker":              sticker,
				"disable_notification": true,
			}
			if id, err := s.postTelegramWithResponse(endpoint, payload); err == nil && id > 0 {
				messageIDs = append(messageIDs, id)
			} else if err != nil {
				if logger := logging.Get().Telegram; logger != nil {
					logger.Infof("telegram sticker send failed (%s): %v", chatID, err)
				}
			}
		case "photo":
			photo := stringFromAny(job.Payload, "photo")
			if strings.TrimSpace(photo) == "" {
				continue
			}
			endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", token)
			payload := map[string]any{
				"chat_id":              chatID,
				"photo":                photo,
				"disable_notification": true,
			}
			if id, err := s.postTelegramWithResponse(endpoint, payload); err == nil && id > 0 {
				messageIDs = append(messageIDs, id)
			} else if err != nil {
				if logger := logging.Get().Telegram; logger != nil {
					logger.Errorf("telegram photo send failed (%s): %v", chatID, err)
				}
			}
		case "text":
			if strings.TrimSpace(job.Message) == "" {
				continue
			}
			endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
			payload := map[string]any{
				"chat_id": chatID,
				"text":    job.Message,
			}
			parseMode := "Markdown"
			if raw := stringFromAny(job.Payload, "parse_mode"); strings.TrimSpace(raw) != "" {
				parseMode = normalizeTelegramParseMode(raw)
			}
			payload["parse_mode"] = parseMode
			if preview, ok := boolFromAny(job.Payload, "webpage_preview"); ok {
				payload["disable_web_page_preview"] = !preview
			} else {
				payload["disable_web_page_preview"] = true
			}
			id, err := s.postTelegramWithResponse(endpoint, payload)
			if err != nil {
				return err
			}
			if id > 0 {
				messageIDs = append(messageIDs, id)
			}
		case "location":
			locationFlag, _ := boolFromAny(job.Payload, "location")
			if !locationFlag || (job.Lat == 0 && job.Lon == 0) {
				continue
			}
			endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendLocation", token)
			payload := map[string]any{
				"chat_id":              chatID,
				"latitude":             job.Lat,
				"longitude":            job.Lon,
				"disable_notification": true,
			}
			if id, err := s.postTelegramWithResponse(endpoint, payload); err == nil && id > 0 {
				messageIDs = append(messageIDs, id)
			} else if err != nil {
				if logger := logging.Get().Telegram; logger != nil {
					logger.Errorf("telegram location send failed (%s): %v", chatID, err)
				}
			}
		case "venue":
			venue := mapStringAnyFromAny(job.Payload["venue"])
			if venue == nil || (job.Lat == 0 && job.Lon == 0) {
				continue
			}
			title := strings.TrimSpace(stringFromAnyMap(venue, "title"))
			address := strings.TrimSpace(stringFromAnyMap(venue, "address"))
			if title == "" && address == "" {
				continue
			}
			endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendVenue", token)
			payload := map[string]any{
				"chat_id":              chatID,
				"latitude":             job.Lat,
				"longitude":            job.Lon,
				"title":                title,
				"address":              address,
				"disable_notification": !hasText,
			}
			if id, err := s.postTelegramWithResponse(endpoint, payload); err == nil && id > 0 {
				messageIDs = append(messageIDs, id)
			} else if err != nil {
				if logger := logging.Get().Telegram; logger != nil {
					logger.Errorf("telegram venue send failed (%s): %v", chatID, err)
				}
			}
		}
	}
	if job.Clean && len(messageIDs) > 0 {
		delay := deletionDelay(job.TTH, 0)
		deleteAt := time.Now().Add(delay)
		for _, msgID := range messageIDs {
			s.scheduleTelegramDelete(token, chatID, msgID, deleteAt, job.UpdateKey)
		}
	}
	if job.UpdateKey != "" && len(messageIDs) > 0 {
		deleteAt := int64(0)
		if job.Clean {
			deleteAt = time.Now().Add(deletionDelay(job.TTH, 0)).UnixMilli()
		}
		s.updateTelegram.Set(updateEntry{
			Key:       job.UpdateKey,
			Target:    chatID,
			MessageID: strconv.Itoa(messageIDs[len(messageIDs)-1]),
			DeleteAt:  deleteAt,
		})
	}
	return nil
}

func telegramSendOrder(payload map[string]any) []string {
	defaultOrder := []string{"sticker", "photo", "text", "location", "venue"}
	allowed := map[string]bool{"sticker": true, "photo": true, "text": true, "location": true, "venue": true}
	if payload == nil {
		return defaultOrder
	}
	raw := payload["send_order"]
	var order []string
	switch v := raw.(type) {
	case []string:
		order = append(order, v...)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				order = append(order, s)
			}
		}
	case string:
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == '|' || r == ';' || r == ' '
		})
		order = append(order, parts...)
	}
	if len(order) == 0 {
		return defaultOrder
	}
	out := make([]string, 0, len(order))
	seen := map[string]bool{}
	for _, entry := range order {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" || !allowed[entry] || seen[entry] {
			continue
		}
		seen[entry] = true
		out = append(out, entry)
	}
	if len(out) == 0 {
		return defaultOrder
	}
	return out
}

func normalizeTelegramParseMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "markdownv2":
		return "MarkdownV2"
	case "html":
		return "HTML"
	default:
		return "Markdown"
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func boolFromAny(payload map[string]any, key string) (bool, bool) {
	if payload == nil {
		return false, false
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		v = strings.TrimSpace(strings.ToLower(v))
		if v == "" {
			return false, false
		}
		return v == "true" || v == "1" || v == "yes" || v == "y", true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	default:
		return false, false
	}
}

func stringFromAny(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
		return ""
	case []string:
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				return s
			}
		}
		return ""
	default:
		return fmt.Sprintf("%v", value)
	}
}

func mapStringAnyFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if entry, ok := value.(map[string]any); ok {
		return entry
	}
	if entry, ok := value.(map[string]interface{}); ok {
		out := map[string]any{}
		for k, v := range entry {
			out[k] = v
		}
		return out
	}
	return nil
}

func stringFromAnyMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (s *Sender) postTelegramJSON(endpoint string, payload map[string]any) error {
	_, err := s.postTelegramWithResponse(endpoint, payload)
	return err
}

func (s *Sender) postTelegramWithResponse(endpoint string, payload map[string]any) (int, error) {
	const maxRetries = 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		status, body, err := s.postJSONRaw(endpoint, payload, nil)
		if err != nil {
			return 0, err
		}
		if status == http.StatusTooManyRequests {
			retryAfter := 30
			var resp struct {
				Parameters struct {
					RetryAfter int `json:"retry_after"`
				} `json:"parameters"`
			}
			if err := json.Unmarshal([]byte(body), &resp); err == nil && resp.Parameters.RetryAfter > 0 {
				retryAfter = resp.Parameters.RetryAfter
			}
			if logger := logging.Get().Telegram; logger != nil {
				logger.Warnf("telegram 429 rate limit endpoint=%s retry_after=%d attempt=%d", endpoint, retryAfter, attempt+1)
			}
			if attempt == maxRetries {
				return 0, fmt.Errorf("telegram rate limited after %d retries", maxRetries)
			}
			time.Sleep(time.Duration(retryAfter) * time.Second)
			continue
		}
		if status < 200 || status >= 300 {
			return 0, fmt.Errorf("http %d: %s", status, strings.TrimSpace(body))
		}
		var resp struct {
			Result struct {
				MessageID int `json:"message_id"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(body), &resp); err == nil && resp.Result.MessageID > 0 {
			return resp.Result.MessageID, nil
		}
		return 0, nil
	}
	return 0, nil
}

func (s *Sender) editTelegramMessage(token, chatID, messageID string, payload map[string]any) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token)
	editPayload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       payload["text"],
	}
	if mode, ok := payload["parse_mode"].(string); ok && mode != "" {
		editPayload["parse_mode"] = mode
	}
	if preview, ok := payload["disable_web_page_preview"].(bool); ok {
		editPayload["disable_web_page_preview"] = preview
	}
	status, body, err := s.postJSONRaw(endpoint, editPayload, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("http %d: %s", status, strings.TrimSpace(body))
	}
	return nil
}
