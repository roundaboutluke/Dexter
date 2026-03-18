package dispatch

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dexter/internal/config"
	"dexter/internal/logging"
)

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
		if _, err := s.postTelegramWithResponse(endpoint, payload); err != nil {
			if logger := logging.Get().Telegram; logger != nil {
				logger.Warnf("telegram clean delete failed for %s/%d: %v", chatID, messageID, err)
			}
		}
		if s.cleanTelegram != nil {
			s.cleanTelegram.Remove(entry)
		}
		if updateKey != "" {
			s.updateTelegram.Remove(chatID, updateKey)
		}
	})
}

// maxCleanRetries caps the number of retry attempts for clean deletes.
const maxCleanRetries = 10

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
		if attempt >= maxCleanRetries {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("discord clean delete giving up after %d attempts (%s/%s): %v", attempt, channelID, entry.MessageID, err)
			}
			if s.cleanDiscord != nil {
				s.cleanDiscord.Remove(entry)
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
		if attempt >= maxCleanRetries {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("discord webhook clean delete giving up after %d attempts (%s): %v", attempt, entry.MessageID, err)
			}
			if s.cleanWebhook != nil {
				s.cleanWebhook.Remove(entry)
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
	logger := logging.Get().General
	logSaveErr := func(name string, err error) {
		if err != nil && logger != nil {
			logger.Warnf("failed to save %s cache: %v", name, err)
		}
	}
	switch kind {
	case "discord":
		logSaveErr("cleanDiscord", s.cleanDiscord.Save())
		logSaveErr("cleanWebhook", s.cleanWebhook.Save())
		logSaveErr("updateDiscord", s.updateDiscord.Save())
		logSaveErr("updateWebhook", s.updateWebhook.Save())
	case "telegram":
		logSaveErr("cleanTelegram", s.cleanTelegram.Save())
		logSaveErr("updateTelegram", s.updateTelegram.Save())
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
