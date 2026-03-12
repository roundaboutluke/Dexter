package dispatch

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
