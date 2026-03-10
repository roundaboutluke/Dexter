package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"poraclego/internal/config"
	"poraclego/internal/logging"
)

// Sender delivers MessageJob payloads to Discord/Telegram/webhook targets.
type Sender struct {
	cfg            *config.Config
	client         *http.Client
	root           string
	cleanDiscord   *cleanCache
	cleanWebhook   *cleanCache
	cleanTelegram  *cleanCache
	updateDiscord  *updateCache
	updateWebhook  *updateCache
	updateTelegram *updateCache
}

func (s *Sender) loggerForType(targetType string) *logging.Logger {
	switch {
	case strings.HasPrefix(targetType, "telegram"):
		return logging.Get().Telegram
	case strings.HasPrefix(targetType, "discord"), targetType == "webhook":
		return logging.Get().Discord
	default:
		return logging.Get().General
	}
}

func (s *Sender) logf(level logging.Level, targetType, format string, args ...any) {
	logger := s.loggerForType(targetType)
	if logger == nil || !logger.Enabled(level) {
		return
	}
	logger.Logf(level, format, args...)
}

func (s *Sender) logJobf(level logging.Level, job MessageJob, format string, args ...any) {
	ref := strings.TrimSpace(job.LogReference)
	if ref == "" {
		ref = "Unknown"
	}
	prefixArgs := []any{ref, job.Name, job.Target}
	prefixArgs = append(prefixArgs, args...)
	s.logf(level, job.Type, "%s: %s %s "+format, prefixArgs...)
}

// NewSender constructs a sender with a shared HTTP client.
func NewSender(cfg *config.Config, root string) *Sender {
	if root == "" {
		root = "."
	}
	cacheDir := filepath.Join(root, ".cache")
	_ = os.MkdirAll(cacheDir, 0o755)
	timeout := 10 * time.Second
	if cfg != nil {
		if ms, ok := cfg.GetInt("tuning.discordTimeout"); ok && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	return &Sender{
		cfg:            cfg,
		client:         &http.Client{Timeout: timeout},
		root:           root,
		cleanDiscord:   newCleanCache(filepath.Join(cacheDir, "cleancache-discord.json")),
		cleanWebhook:   newCleanCache(filepath.Join(cacheDir, "cleancache-webhook.json")),
		cleanTelegram:  newCleanCache(filepath.Join(cacheDir, "cleancache-telegram.json")),
		updateDiscord:  newUpdateCache(filepath.Join(cacheDir, "updatecache-discord.json")),
		updateWebhook:  newUpdateCache(filepath.Join(cacheDir, "updatecache-webhook.json")),
		updateTelegram: newUpdateCache(filepath.Join(cacheDir, "updatecache-telegram.json")),
	}
}

// Send dispatches a message to the configured platform.
func (s *Sender) Send(job MessageJob) error {
	start := time.Now()
	s.logJobf(logging.LevelInfo, job, "sending message")
	switch job.Type {
	case "webhook":
		err := s.sendDiscordWebhook(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "WEBHOOK (%d ms)", time.Since(start).Milliseconds())
		}
		return err
	case "discord:channel":
		err := s.sendDiscordChannel(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "CHANNEL (%d ms)", time.Since(start).Milliseconds())
		}
		return err
	case "discord:user":
		err := s.sendDiscordUser(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "USER (%d ms)", time.Since(start).Milliseconds())
		}
		return err
	case "telegram:user", "telegram:channel":
		err := s.sendTelegram(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "TELEGRAM (%d ms)", time.Since(start).Milliseconds())
		}
		return err
	case "telegram:group":
		err := s.sendTelegram(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "TELEGRAM (%d ms)", time.Since(start).Milliseconds())
		}
		return err
	default:
		return fmt.Errorf("unsupported dispatch type %s", job.Type)
	}
}

func (s *Sender) sendDiscordWebhook(url string, job MessageJob) error {
	if url == "" {
		return fmt.Errorf("missing webhook url")
	}
	payload := s.discordPayload(job)
	s.maybeDeletePreviousForMonsterChangeWebhook(url, job)
	if job.UpdateExisting && job.UpdateKey != "" {
		if entry, ok := s.updateWebhook.Get(url, job.UpdateKey); ok {
			s.logJobf(logging.LevelDebug, job, "attempting webhook edit for message %s", entry.MessageID)
			if err := s.patchDiscordWebhookMessage(url, entry.MessageID, payload); err == nil {
				s.logJobf(logging.LevelDebug, job, "updated existing webhook message %s", entry.MessageID)
				return nil
			}
		}
	}
	waitURL := url
	if job.Clean || job.UpdateKey != "" {
		waitURL = url + "?wait=true"
	}
	resp, err := s.postDiscordPayload(waitURL, payload, nil, true)
	if err != nil {
		return err
	}
	if job.Clean || job.UpdateKey != "" {
		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(resp, &result); err == nil && result.ID != "" {
			deleteAt := time.Now().Add(deletionDelay(job.TTH, 0))
			if job.Clean {
				s.scheduleDiscordWebhookDelete(url, result.ID, deleteAt, job.UpdateKey)
			}
			if job.UpdateKey != "" {
				entry := updateEntry{
					Key:       job.UpdateKey,
					Target:    url,
					MessageID: result.ID,
					DeleteAt:  deleteAt.UnixMilli(),
				}
				s.updateWebhook.Set(entry)
			}
		}
	}
	return nil
}

func (s *Sender) sendDiscordChannel(channelID string, job MessageJob) error {
	token := selectToken(s.cfg, "discord.token", channelID)
	if token == "" {
		return fmt.Errorf("discord token missing")
	}
	endpoint := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	headers := map[string]string{"Authorization": "Bot " + token}
	payload := s.discordPayload(job)
	s.maybeDeletePreviousForMonsterChangeDiscord(channelID, token, job)
	if job.UpdateExisting && job.UpdateKey != "" {
		if entry, ok := s.updateDiscord.Get(channelID, job.UpdateKey); ok {
			editChannel := entry.ChannelID
			if editChannel == "" {
				editChannel = channelID
			}
			s.logJobf(logging.LevelDebug, job, "attempting discord channel edit for message %s", entry.MessageID)
			if err := s.patchDiscordChannelMessage(editChannel, entry.MessageID, token, payload); err == nil {
				s.logJobf(logging.LevelDebug, job, "updated existing discord channel message %s", entry.MessageID)
				return nil
			}
		}
	}
	resp, err := s.postDiscordPayload(endpoint, payload, headers, false)
	if err != nil {
		return err
	}
	if job.Clean || job.UpdateKey != "" {
		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(resp, &result); err == nil && result.ID != "" {
			delay := deletionDelay(job.TTH, getIntConfig(s.cfg, "discord.messageDeleteDelay", 0))
			deleteAt := time.Now().Add(delay)
			if job.Clean {
				s.scheduleDiscordDelete(channelID, result.ID, token, deleteAt, "discord:channel", channelID, job.UpdateKey)
			}
			if job.UpdateKey != "" {
				entry := updateEntry{
					Key:       job.UpdateKey,
					Target:    channelID,
					MessageID: result.ID,
					ChannelID: channelID,
					DeleteAt:  deleteAt.UnixMilli(),
				}
				s.updateDiscord.Set(entry)
			}
		}
	}
	return nil
}

func (s *Sender) sendDiscordUser(userID string, job MessageJob) error {
	token := selectToken(s.cfg, "discord.token", userID)
	if token == "" {
		return fmt.Errorf("discord token missing")
	}
	channelID, err := s.ensureDiscordDM(userID, token)
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	headers := map[string]string{"Authorization": "Bot " + token}
	payload := s.discordPayload(job)
	s.maybeDeletePreviousForMonsterChangeDiscord(userID, token, job)
	if job.UpdateExisting && job.UpdateKey != "" {
		if entry, ok := s.updateDiscord.Get(userID, job.UpdateKey); ok {
			editChannel := entry.ChannelID
			if editChannel == "" {
				editChannel = channelID
			}
			s.logJobf(logging.LevelDebug, job, "attempting discord DM edit for message %s", entry.MessageID)
			if err := s.patchDiscordChannelMessage(editChannel, entry.MessageID, token, payload); err == nil {
				s.logJobf(logging.LevelDebug, job, "updated existing discord DM message %s", entry.MessageID)
				return nil
			}
		}
	}
	resp, err := s.postDiscordPayload(endpoint, payload, headers, false)
	if err != nil {
		return err
	}
	if job.Clean || job.UpdateKey != "" {
		var result struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(resp, &result); err == nil && result.ID != "" {
			// PoracleJS only applies discord.messageDeleteDelay to channel alerts (not DMs).
			delay := deletionDelay(job.TTH, 0)
			deleteAt := time.Now().Add(delay)
			if job.Clean {
				s.scheduleDiscordDelete(channelID, result.ID, token, deleteAt, "discord:user", userID, job.UpdateKey)
			}
			if job.UpdateKey != "" {
				entry := updateEntry{
					Key:       job.UpdateKey,
					Target:    userID,
					MessageID: result.ID,
					ChannelID: channelID,
					DeleteAt:  deleteAt.UnixMilli(),
				}
				s.updateDiscord.Set(entry)
			}
		}
	}
	return nil
}

func (s *Sender) maybeDeletePreviousForMonsterChangeWebhook(url string, job MessageJob) {
	if s == nil || job.LogReference != "MonsterChange" || !job.Clean || job.UpdateKey == "" || job.UpdateExisting {
		return
	}
	if s.updateWebhook == nil {
		return
	}
	entry, ok := s.updateWebhook.Get(url, job.UpdateKey)
	if !ok || entry.MessageID == "" {
		return
	}
	// Remove mapping before deleting so we don't race with a new message reusing the same key.
	s.updateWebhook.Remove(url, job.UpdateKey)
	s.scheduleDiscordWebhookDelete(url, entry.MessageID, time.Now().Add(10*time.Millisecond), "")
}

func (s *Sender) maybeDeletePreviousForMonsterChangeDiscord(targetID, token string, job MessageJob) {
	if s == nil || job.LogReference != "MonsterChange" || !job.Clean || job.UpdateKey == "" || job.UpdateExisting {
		return
	}
	if s.updateDiscord == nil {
		return
	}
	entry, ok := s.updateDiscord.Get(targetID, job.UpdateKey)
	if !ok || entry.MessageID == "" {
		return
	}
	channelID := entry.ChannelID
	if channelID == "" {
		return
	}
	// Remove mapping before deleting so we don't race with a new message reusing the same key.
	s.updateDiscord.Remove(targetID, job.UpdateKey)
	s.scheduleDiscordDelete(channelID, entry.MessageID, token, time.Now().Add(10*time.Millisecond), job.Type, targetID, "")
}

func (s *Sender) ensureDiscordDM(userID, token string) (string, error) {
	endpoint := "https://discord.com/api/v10/users/@me/channels"
	headers := map[string]string{"Authorization": "Bot " + token}
	payload := map[string]any{"recipient_id": userID}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	resp, err := s.discordRequestWithRetries(http.MethodPost, endpoint, raw, "application/json", headers, 10)
	if err != nil {
		return "", err
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	if result.ID == "" {
		return "", fmt.Errorf("discord dm channel missing id")
	}
	return result.ID, nil
}

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

func deletionDelay(tth TimeToHide, extraMs int) time.Duration {
	total := (tth.Hours*3600 + tth.Minutes*60 + tth.Seconds) * 1000
	total += extraMs
	if total <= 0 {
		return 0
	}
	return time.Duration(total) * time.Millisecond
}

func (s *Sender) scheduleDiscordDelete(channelID, messageID, token string, deleteAt time.Time, entryType, entryTarget, updateKey string) {
	if deleteAt.IsZero() {
		return
	}
	entry := cleanEntry{
		Type:      entryType,
		Target:    entryTarget,
		MessageID: messageID,
		DeleteAt:  deleteAt.UnixMilli(),
	}
	if s.cleanDiscord != nil {
		s.cleanDiscord.Add(entry)
	}
	s.scheduleDiscordDeleteAttempt(entry, channelID, token, updateKey, 0)
}

func (s *Sender) scheduleDiscordWebhookDelete(url, messageID string, deleteAt time.Time, updateKey string) {
	if deleteAt.IsZero() {
		return
	}
	entry := cleanEntry{
		Type:      "webhook",
		Target:    url,
		MessageID: messageID,
		DeleteAt:  deleteAt.UnixMilli(),
	}
	if s.cleanWebhook != nil {
		s.cleanWebhook.Add(entry)
	}
	s.scheduleDiscordWebhookDeleteAttempt(entry, url, updateKey, 0)
}

func (s *Sender) scheduleTelegramDelete(token, chatID string, messageID int, deleteAt time.Time, updateKey string) {
	if deleteAt.IsZero() {
		return
	}
	entry := cleanEntry{
		Type:      "telegram",
		Target:    chatID,
		MessageID: strconv.Itoa(messageID),
		DeleteAt:  deleteAt.UnixMilli(),
	}
	if s.cleanTelegram != nil {
		s.cleanTelegram.Add(entry)
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", token)
	payload := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	delay := time.Until(deleteAt)
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		_, _ = s.postTelegramWithResponse(endpoint, payload)
		if s.cleanTelegram != nil {
			s.cleanTelegram.Remove(entry)
		}
		if updateKey != "" {
			s.updateTelegram.Remove(chatID, updateKey)
		}
	})
}

func (s *Sender) scheduleDiscordDeleteAttempt(entry cleanEntry, channelID, token, updateKey string, attempt int) {
	deleteAt := time.Unix(0, entry.DeleteAt*int64(time.Millisecond))
	delay := time.Until(deleteAt)
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		retryAfter, terminal, err := s.deleteDiscordChannelMessage(channelID, entry.MessageID, token)
		if terminal && err == nil {
			if s.cleanDiscord != nil {
				s.cleanDiscord.Remove(entry)
			}
			if updateKey != "" && s.updateDiscord != nil {
				s.updateDiscord.Remove(entry.Target, updateKey)
			}
			return
		}
		if logger := logging.Get().Discord; logger != nil && err != nil {
			logger.Warnf("discord clean delete failed (%s/%s), retrying: %v", channelID, entry.MessageID, err)
		}
		nextAt := time.Now().Add(cleanRetryDelay(entry, attempt, retryAfter))
		entry.DeleteAt = nextAt.UnixMilli()
		if s.cleanDiscord != nil {
			s.cleanDiscord.Add(entry)
		}
		s.scheduleDiscordDeleteAttempt(entry, channelID, token, updateKey, attempt+1)
	})
}

func (s *Sender) scheduleDiscordWebhookDeleteAttempt(entry cleanEntry, url, updateKey string, attempt int) {
	deleteAt := time.Unix(0, entry.DeleteAt*int64(time.Millisecond))
	delay := time.Until(deleteAt)
	if delay < 0 {
		delay = 0
	}
	time.AfterFunc(delay, func() {
		retryAfter, terminal, err := s.deleteDiscordWebhookMessage(url, entry.MessageID)
		if terminal && err == nil {
			if s.cleanWebhook != nil {
				s.cleanWebhook.Remove(entry)
			}
			if updateKey != "" && s.updateWebhook != nil {
				s.updateWebhook.Remove(url, updateKey)
			}
			return
		}
		if logger := logging.Get().Discord; logger != nil && err != nil {
			logger.Warnf("discord webhook clean delete failed (%s), retrying: %v", entry.MessageID, err)
		}
		nextAt := time.Now().Add(cleanRetryDelay(entry, attempt, retryAfter))
		entry.DeleteAt = nextAt.UnixMilli()
		if s.cleanWebhook != nil {
			s.cleanWebhook.Add(entry)
		}
		s.scheduleDiscordWebhookDeleteAttempt(entry, url, updateKey, attempt+1)
	})
}

func (s *Sender) deleteDiscordChannelMessage(channelID, messageID, token string) (time.Duration, bool, error) {
	endpoint := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s", channelID, messageID)
	headers := map[string]string{"Authorization": "Bot " + token}
	status, respHeaders, respBody, err := s.doRawRequest(http.MethodDelete, endpoint, nil, "", headers)
	if err != nil {
		return 0, false, err
	}
	return classifyDiscordDeleteResponse(status, respHeaders, respBody)
}

func (s *Sender) deleteDiscordWebhookMessage(url, messageID string) (time.Duration, bool, error) {
	endpoint := fmt.Sprintf("%s/messages/%s", url, messageID)
	status, respHeaders, respBody, err := s.doRawRequest(http.MethodDelete, endpoint, nil, "", nil)
	if err != nil {
		return 0, false, err
	}
	return classifyDiscordDeleteResponse(status, respHeaders, respBody)
}

func classifyDiscordDeleteResponse(status int, headers http.Header, body []byte) (time.Duration, bool, error) {
	switch {
	case status >= 200 && status < 300:
		return 0, true, nil
	case status == http.StatusTooManyRequests:
		return discordRetryAfter(headers, body), false, fmt.Errorf("http %d: rate limited", status)
	case status == http.StatusNotFound:
		// Treat "already gone" as success for cleanup.
		return 0, true, nil
	case status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden:
		// Permanent failure classes for cleanup delete; avoid retry loops.
		return 0, true, nil
	default:
		return 0, false, fmt.Errorf("http %d: %s", status, strings.TrimSpace(string(body)))
	}
}

func cleanRetryDelay(entry cleanEntry, attempt int, retryAfter time.Duration) time.Duration {
	base := retryAfter
	if base <= 0 {
		backoff := []time.Duration{
			5 * time.Minute,
			15 * time.Minute,
			30 * time.Minute,
			60 * time.Minute,
			2 * time.Hour,
		}
		if attempt < 0 {
			attempt = 0
		}
		if attempt >= len(backoff) {
			base = backoff[len(backoff)-1]
		} else {
			base = backoff[attempt]
		}
	}
	// Add deterministic jitter to avoid synchronized retry spikes.
	h := fnv.New32a()
	_, _ = h.Write([]byte(entry.Type))
	_, _ = h.Write([]byte(entry.Target))
	_, _ = h.Write([]byte(entry.MessageID))
	_, _ = h.Write([]byte{byte(attempt)})
	jitter := time.Duration(h.Sum32()%15000) * time.Millisecond
	return base + jitter
}

func (s *Sender) postJSONRaw(endpoint string, payload any, headers map[string]string) (int, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	return resp.StatusCode, string(bodyBytes), nil
}

func (s *Sender) patchDiscordChannelMessage(channelID, messageID, token string, payload map[string]any) error {
	endpoint := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s", channelID, messageID)
	headers := map[string]string{"Authorization": "Bot " + token}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.discordRequestWithRetries(http.MethodPatch, endpoint, raw, "application/json", headers, 10)
	if err != nil {
		return err
	}
	return nil
}

func (s *Sender) patchDiscordWebhookMessage(url, messageID string, payload map[string]any) error {
	endpoint := fmt.Sprintf("%s/messages/%s", url, messageID)
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.discordRequestWithRetries(http.MethodPatch, endpoint, raw, "application/json", nil, 10)
	if err != nil {
		return err
	}
	return nil
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

func (s *Sender) discordPayload(job MessageJob) map[string]any {
	if job.Payload != nil {
		payload := sanitizeDiscordPayload(job.Payload)
		if _, hasEmbeds := payload["embeds"]; !hasEmbeds {
			if content, ok := payload["content"].(string); ok {
				if content == "" {
					payload["content"] = job.Message
				}
			} else {
				payload["content"] = job.Message
			}
		}
		return payload
	}
	return map[string]any{"content": job.Message}
}

func sanitizeDiscordPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return payload
	}

	// Normalize legacy single-embed payloads and malformed embeds objects so all callsites
	// can rely on `embeds` being the canonical list shape.
	if rawEmbed, ok := payload["embed"]; ok {
		if _, hasEmbeds := payload["embeds"]; !hasEmbeds {
			if embed, ok := rawEmbed.(map[string]any); ok {
				payload["embeds"] = []any{embed}
			}
		}
		delete(payload, "embed")
	}
	if rawEmbeds, ok := payload["embeds"]; ok {
		if embed, ok := rawEmbeds.(map[string]any); ok {
			payload["embeds"] = []any{embed}
		}
	}
	if embeds, ok := payload["embeds"].([]any); ok {
		for _, raw := range embeds {
			if embed, ok := raw.(map[string]any); ok {
				sanitizeEmbedColor(embed)
			}
		}
	}
	return payload
}

func (s *Sender) patchJSONRaw(endpoint string, payload any, headers map[string]string) (int, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", err
	}
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	return resp.StatusCode, string(bodyBytes), nil
}
func sanitizeEmbedColor(embed map[string]any) {
	if embed == nil {
		return
	}
	raw, ok := embed["color"]
	if !ok {
		return
	}
	switch v := raw.(type) {
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			delete(embed, "color")
			return
		}
		hexCandidate := strings.TrimPrefix(value, "#")
		isHex := strings.HasPrefix(value, "#") || looksLikeHexColor(hexCandidate)
		// Avoid treating 8-digit *decimal* colors (e.g. "13369344") as ARGB hex.
		// These are commonly produced by templates like `"color": "{{color}}"` where `color` is an int.
		if !strings.HasPrefix(value, "#") && len(hexCandidate) == 8 && isAllDigits(hexCandidate) {
			isHex = false
		}
		if isHex {
			if len(hexCandidate) == 8 {
				hexCandidate = hexCandidate[2:]
			}
			parsed, err := strconv.ParseInt(hexCandidate, 16, 32)
			if err != nil {
				delete(embed, "color")
				return
			}
			if normalized, ok := normalizeEmbedColor(parsed); ok {
				embed["color"] = int(normalized)
			} else {
				delete(embed, "color")
			}
			return
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			delete(embed, "color")
			return
		}
		if normalized, ok := normalizeEmbedColor(int64(parsed)); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	case float64:
		if normalized, ok := normalizeEmbedColor(int64(v)); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	case int:
		if normalized, ok := normalizeEmbedColor(int64(v)); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	case int64:
		if normalized, ok := normalizeEmbedColor(v); ok {
			embed["color"] = int(normalized)
		} else {
			delete(embed, "color")
		}
	}
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeEmbedColor(value int64) (int64, bool) {
	if value < 0 {
		return 0, false
	}
	if value <= 0xFFFFFF {
		return value, true
	}
	if value <= 0xFFFFFFFF {
		return value & 0xFFFFFF, true
	}
	return 0, false
}

func looksLikeHexColor(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return len(value) == 3 || len(value) == 6 || len(value) == 8
}

func (s *Sender) postJSON(url string, payload any, headers map[string]string) error {
	_, err := s.postJSONWithResponse(url, payload, headers)
	return err
}

func (s *Sender) postJSONWithResponse(url string, payload any, headers map[string]string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return bodyBytes, nil
}

func (s *Sender) postRawWithResponse(url string, body io.Reader, contentType string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return bodyBytes, nil
}

func (s *Sender) discordRequestWithRetries(method, endpoint string, body []byte, contentType string, headers map[string]string, maxRetries int) ([]byte, error) {
	const maxTimeoutRetries = 5
	for attempt := 0; attempt <= maxRetries; attempt++ {
		status, respHeaders, respBody, err := s.doRawRequest(method, endpoint, body, contentType, headers)
		if err != nil {
			if isTimeoutErr(err) && attempt < maxTimeoutRetries {
				if logger := logging.Get().Discord; logger != nil {
					logger.Warnf("discord timeout endpoint=%s attempt=%d", endpoint, attempt+1)
				}
				time.Sleep(discordTimeoutBackoff(endpoint, attempt))
				continue
			}
			return nil, err
		}
		if status == http.StatusTooManyRequests {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("discord 429 rate limit endpoint=%s attempt=%d retry_after=%s", endpoint, attempt+1, discordRetryAfter(respHeaders, respBody))
			}
			if attempt == maxRetries {
				return nil, fmt.Errorf("discord rate limited after %d retries", maxRetries)
			}
			delay := discordRetryAfter(respHeaders, respBody)
			time.Sleep(delay + discordRetryJitter(endpoint, attempt))
			continue
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("http %d: %s", status, strings.TrimSpace(string(respBody)))
		}
		return respBody, nil
	}
	return nil, fmt.Errorf("discord request exceeded retries")
}

func (s *Sender) doRawRequest(method, endpoint string, body []byte, contentType string, headers map[string]string) (int, http.Header, []byte, error) {
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioReadAll(resp.Body)
	return resp.StatusCode, resp.Header, bodyBytes, nil
}

func discordRetryAfter(headers http.Header, body []byte) time.Duration {
	type discordRate struct {
		RetryAfter float64 `json:"retry_after"`
		Parameters struct {
			RetryAfter float64 `json:"retry_after"`
		} `json:"parameters"`
	}
	delaySeconds := 0.0
	var parsed discordRate
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err == nil {
			if parsed.RetryAfter > 0 {
				delaySeconds = parsed.RetryAfter
			} else if parsed.Parameters.RetryAfter > 0 {
				delaySeconds = parsed.Parameters.RetryAfter
			}
		}
	}
	if delaySeconds > 0 {
		return time.Duration(delaySeconds * float64(time.Second))
	}

	raw := strings.TrimSpace(headers.Get("Retry-After"))
	if raw == "" {
		raw = strings.TrimSpace(headers.Get("retry-after"))
	}
	if raw == "" {
		return 5 * time.Second
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
		if v > 1000 {
			return time.Duration(v) * time.Millisecond
		}
		return time.Duration(v * float64(time.Second))
	}
	return 5 * time.Second
}

func discordRetryJitter(endpoint string, attempt int) time.Duration {
	h := fnv.New32a()
	_, _ = h.Write([]byte(endpoint))
	_, _ = h.Write([]byte{byte(attempt)})
	return time.Duration(h.Sum32()%5000) * time.Millisecond
}

func discordTimeoutBackoff(endpoint string, attempt int) time.Duration {
	base := 2500*time.Millisecond + discordRetryJitter(endpoint, attempt)
	extra := time.Duration((attempt%3)*2500) * time.Millisecond
	return base + extra
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (s *Sender) postDiscordPayload(url string, payload map[string]any, headers map[string]string, webhook bool) ([]byte, error) {
	const maxRetries = 10
	if s.shouldUploadEmbedImages() {
		if body, contentType, used, err := s.buildDiscordMultipartBytes(payload, webhook); err == nil && used {
			return s.discordRequestWithRetries(http.MethodPost, url, body, contentType, headers, maxRetries)
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return s.discordRequestWithRetries(http.MethodPost, url, raw, "application/json", headers, maxRetries)
}

func (s *Sender) shouldUploadEmbedImages() bool {
	if s.cfg == nil {
		return false
	}
	value, ok := s.cfg.GetBool("discord.uploadEmbedImages")
	return ok && value
}

func (s *Sender) buildDiscordMultipartBytes(payload map[string]any, webhook bool) ([]byte, string, bool, error) {
	imageURL := extractEmbedImageURL(payload)
	if imageURL == "" {
		return nil, "", false, nil
	}
	clone, err := clonePayload(payload)
	if err != nil {
		return nil, "", false, err
	}
	if !setEmbedImageURL(clone, "attachment://map.png") {
		return nil, "", false, nil
	}
	resp, err := s.client.Get(imageURL)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", false, fmt.Errorf("image http %d", resp.StatusCode)
	}
	imageBytes, err := ioReadAll(resp.Body)
	if err != nil {
		return nil, "", false, err
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	payloadJSON, err := json.Marshal(clone)
	if err != nil {
		return nil, "", false, err
	}
	if err := writer.WriteField("payload_json", string(payloadJSON)); err != nil {
		return nil, "", false, err
	}
	fieldName := "file"
	if !webhook {
		fieldName = "files[0]"
	}
	part, err := writer.CreateFormFile(fieldName, "map.png")
	if err != nil {
		return nil, "", false, err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, "", false, err
	}
	if err := writer.Close(); err != nil {
		return nil, "", false, err
	}
	return buf.Bytes(), writer.FormDataContentType(), true, nil
}

func extractEmbedImageURL(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if embeds, ok := payload["embeds"].([]any); ok && len(embeds) > 0 {
		if embed, ok := embeds[0].(map[string]any); ok {
			if url := extractImageURL(embed); url != "" {
				return url
			}
		}
	}
	if embed, ok := payload["embed"].(map[string]any); ok {
		return extractImageURL(embed)
	}
	return ""
}

func extractImageURL(embed map[string]any) string {
	if embed == nil {
		return ""
	}
	image, ok := embed["image"].(map[string]any)
	if !ok {
		return ""
	}
	url, _ := image["url"].(string)
	return url
}

func setEmbedImageURL(payload map[string]any, url string) bool {
	if payload == nil {
		return false
	}
	if embeds, ok := payload["embeds"].([]any); ok && len(embeds) > 0 {
		if embed, ok := embeds[0].(map[string]any); ok {
			setEmbedImage(embed, url)
			return true
		}
	}
	if embed, ok := payload["embed"].(map[string]any); ok {
		setEmbedImage(embed, url)
		return true
	}
	return false
}

func setEmbedImage(embed map[string]any, url string) {
	if embed == nil {
		return
	}
	image, ok := embed["image"].(map[string]any)
	if !ok {
		image = map[string]any{}
		embed["image"] = image
	}
	image["url"] = url
}

func clonePayload(payload map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var clone map[string]any
	if err := json.Unmarshal(raw, &clone); err != nil {
		return nil, err
	}
	return clone, nil
}

func (s *Sender) LoadCleanCaches(kind string) {
	switch kind {
	case "discord":
		s.loadDiscordCleanCache()
		s.loadWebhookCleanCache()
		s.loadDiscordUpdateCache()
		s.loadWebhookUpdateCache()
	case "telegram":
		s.loadTelegramCleanCache()
		s.loadTelegramUpdateCache()
	}
}

func (s *Sender) SaveCleanCaches(kind string) {
	switch kind {
	case "discord":
		_ = s.cleanDiscord.Save()
		_ = s.cleanWebhook.Save()
		_ = s.updateDiscord.Save()
		_ = s.updateWebhook.Save()
	case "telegram":
		_ = s.cleanTelegram.Save()
		_ = s.updateTelegram.Save()
	}
}

func (s *Sender) loadDiscordCleanCache() {
	entries, err := s.cleanDiscord.Load()
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		deleteAt := time.Unix(0, entry.DeleteAt*int64(time.Millisecond))
		if deleteAt.Before(now) {
			deleteAt = now
		}
		if entry.Type == "discord:user" {
			token := selectToken(s.cfg, "discord.token", entry.Target)
			if token == "" {
				continue
			}
			channelID, err := s.ensureDiscordDM(entry.Target, token)
			if err != nil {
				continue
			}
			s.scheduleDiscordDelete(channelID, entry.MessageID, token, deleteAt, entry.Type, entry.Target, "")
			continue
		}
		if entry.Type == "discord:channel" {
			token := selectToken(s.cfg, "discord.token", entry.Target)
			if token == "" {
				continue
			}
			s.scheduleDiscordDelete(entry.Target, entry.MessageID, token, deleteAt, entry.Type, entry.Target, "")
		}
	}
}

func (s *Sender) loadWebhookCleanCache() {
	entries, err := s.cleanWebhook.Load()
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		deleteAt := time.Unix(0, entry.DeleteAt*int64(time.Millisecond))
		if deleteAt.Before(now) {
			deleteAt = now
		}
		s.scheduleDiscordWebhookDelete(entry.Target, entry.MessageID, deleteAt, "")
	}
}

func (s *Sender) loadTelegramCleanCache() {
	entries, err := s.cleanTelegram.Load()
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		token := selectToken(s.cfg, "telegram.token", entry.Target)
		if token == "" {
			continue
		}
		messageID, err := strconv.Atoi(entry.MessageID)
		if err != nil {
			continue
		}
		deleteAt := time.Unix(0, entry.DeleteAt*int64(time.Millisecond))
		if deleteAt.Before(now) {
			deleteAt = now
		}
		s.scheduleTelegramDelete(token, entry.Target, messageID, deleteAt, "")
	}
}

func (s *Sender) loadDiscordUpdateCache() {
	if s.updateDiscord == nil {
		return
	}
	_ = s.updateDiscord.Load()
}

func (s *Sender) loadWebhookUpdateCache() {
	if s.updateWebhook == nil {
		return
	}
	_ = s.updateWebhook.Load()
}

func (s *Sender) loadTelegramUpdateCache() {
	if s.updateTelegram == nil {
		return
	}
	_ = s.updateTelegram.Load()
}

func selectToken(cfg *config.Config, path, target string) string {
	if cfg == nil {
		return ""
	}
	values, ok := cfg.GetStringSlice(path)
	if ok && len(values) > 0 {
		if len(values) == 1 || target == "" {
			return strings.TrimSpace(values[0])
		}
		index := int(hashString(target)) % len(values)
		return strings.TrimSpace(values[index])
	}
	value, ok := cfg.GetString(path)
	if ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func hashString(value string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return h.Sum32()
}

func ioReadAll(r io.Reader) ([]byte, error) {
	buf := &bytes.Buffer{}
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

func getIntConfig(cfg *config.Config, path string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	if value, ok := cfg.GetInt(path); ok {
		return value
	}
	return fallback
}
