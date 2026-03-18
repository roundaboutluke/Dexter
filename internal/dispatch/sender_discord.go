package dispatch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"dexter/internal/logging"
)

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
