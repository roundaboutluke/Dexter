package bot

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"poraclego/internal/command"
	"poraclego/internal/i18n"
)

// Telegram bot wrapper.
type Telegram struct {
	manager *Manager
	token   string
	bot     *tgbotapi.BotAPI
}

// NewTelegram constructs a Telegram bot.
func NewTelegram(manager *Manager, token string) *Telegram {
	return &Telegram{manager: manager, token: token}
}

// Start begins polling updates.
func (t *Telegram) Start() error {
	bot, err := tgbotapi.NewBotAPI(t.token)
	if err != nil {
		return err
	}
	t.bot = bot
	updateCfg := tgbotapi.NewUpdate(0)
	updateCfg.Timeout = 30
	updates := bot.GetUpdatesChan(updateCfg)
	go func() {
		for update := range updates {
			msg := update.Message
			if msg == nil {
				msg = update.ChannelPost
			}
			if msg == nil {
				continue
			}
			if msg.Location != nil {
				msg.Text = fmt.Sprintf("/location %f,%f", msg.Location.Latitude, msg.Location.Longitude)
			}
			prefix, _ := t.manager.cfg.GetString("telegram.prefix")
			if prefix == "" {
				prefix = "/"
			}
			text := msg.Text
			if !strings.HasPrefix(text, prefix) {
				continue
			}
			if strings.HasPrefix(text, prefix+"identify") {
				if msg.Chat != nil && msg.Chat.IsPrivate() && msg.From != nil {
					reply := fmt.Sprintf("This is a private message and your id is: [ %d ]", msg.From.ID)
					_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, reply))
				} else if msg.Chat != nil {
					author := "unknown - this is a channel (and can't be used for bot registration)"
					if msg.From != nil {
						author = fmt.Sprintf("[ %d ]", msg.From.ID)
					}
					reply := fmt.Sprintf("This channel is id: [ %d ] and your id is: %s", msg.Chat.ID, author)
					_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, reply))
				}
				continue
			}
			channelName := ""
			isDM := msg.Chat != nil && msg.Chat.IsPrivate()
			if msg.Chat != nil {
				if msg.Chat.Title != "" {
					channelName = msg.Chat.Title
				} else if msg.Chat.UserName != "" {
					channelName = msg.Chat.UserName
				}
			}
			if msg.From == nil {
				continue
			}
			userName := telegramDisplayName(msg.From)
			isAdmin := containsID(t.manager.cfg, "telegram.admins", fmt.Sprintf("%d", msg.From.ID))
			channelID := ""
			if msg.Chat != nil {
				channelID = fmt.Sprintf("%d", msg.Chat.ID)
			}
			ctx := t.manager.Context("telegram", "", prefix, fmt.Sprintf("%d", msg.From.ID), userName, channelID, channelName, isDM, isAdmin, nil, ".")
			lines := parseTelegramCommandLines(text, prefix, t.manager.i18n)
			recognized := false
			for _, line := range lines {
				if line == "" {
					continue
				}
				tokens := splitQuotedArgs(line)
				if len(tokens) == 0 {
					continue
				}
				if tokens[0] == "start" {
					if isDM {
						if enabled, _ := t.manager.cfg.GetBool("telegram.registerOnStart"); enabled {
							t.syncTelegramUser(msg.From.ID, true, false)
						}
					}
				}
				recognized = true
				reply, err := t.manager.Registry().Execute(ctx, line)
				if err != nil {
					if err.Error() == "unknown command" {
						if isDM {
							if unrec, ok := t.manager.cfg.GetString("telegram.unrecognisedCommandMessage"); ok && unrec != "" {
								_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, unrec))
								continue
							}
						}
					}
					_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, err.Error()))
					continue
				}
				if reply == "" {
					continue
				}
				if handled := sendTelegramSpecialReply(bot, msg.Chat.ID, reply); handled {
					continue
				}
				_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, reply))
			}
			if isDM && !recognized {
				if unrec, ok := t.manager.cfg.GetString("telegram.unrecognisedCommandMessage"); ok && unrec != "" {
					_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, unrec))
				}
			}
		}
	}()
	t.startReconciliation()
	return nil
}

func parseTelegramCommandLines(text, prefix string, translator *i18n.Factory) []string {
	commandRe := regexp.MustCompile(`^/([^@\s]+)@?(?:\S+)?\s*([\s\S]*)$`)
	matches := commandRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		return nil
	}
	command := matches[1]
	args := matches[2]
	line := strings.TrimSpace(prefix + command + " " + args)
	return parseCommandLines(line, prefix, translator)
}

func telegramDisplayName(user *tgbotapi.User) string {
	if user == nil {
		return ""
	}
	name := strings.TrimSpace(strings.TrimSpace(user.FirstName + " " + user.LastName))
	if user.UserName != "" {
		if name == "" {
			name = user.UserName
		} else {
			name = name + " [" + user.UserName + "]"
		}
	}
	return strings.TrimSpace(name)
}

func sendTelegramSpecialReply(bot *tgbotapi.BotAPI, chatID int64, reply string) bool {
	if strings.HasPrefix(reply, command.FileReplyPrefix) {
		raw := strings.TrimPrefix(reply, command.FileReplyPrefix)
		var payload struct {
			Name    string `json:"name"`
			Message string `json:"message"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		if payload.Name == "" || payload.Content == "" {
			return false
		}
		if payload.Message != "" {
			_, _ = bot.Send(tgbotapi.NewMessage(chatID, payload.Message))
		}
		file := tgbotapi.FileBytes{
			Name:  payload.Name,
			Bytes: []byte(payload.Content),
		}
		_, _ = bot.Send(tgbotapi.NewDocument(chatID, file))
		return true
	}
	if !strings.HasPrefix(reply, command.TelegramMultiPrefix) {
		return false
	}
	raw := strings.TrimPrefix(reply, command.TelegramMultiPrefix)
	var messages []string
	if err := json.Unmarshal([]byte(raw), &messages); err != nil {
		return false
	}
	for _, message := range messages {
		if message == "" {
			continue
		}
		_, _ = bot.Send(tgbotapi.NewMessage(chatID, message))
	}
	return true
}
