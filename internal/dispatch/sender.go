package dispatch

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dexter/internal/circuitbreaker"
	"dexter/internal/config"
	"dexter/internal/logging"
	"dexter/internal/metrics"
)

// Sender delivers MessageJob payloads to Discord/Telegram/webhook targets.
type Sender struct {
	cfg            *config.Config
	client         *http.Client
	root           string
	breakers       map[string]*circuitbreaker.Breaker
	cleanDiscord   *cleanCache
	cleanWebhook   *cleanCache
	cleanTelegram  *cleanCache
	updateDiscord  *updateCache
	updateWebhook  *updateCache
	updateTelegram *updateCache
	dmChannels     sync.Map // userID -> channelID cache for Discord DMs
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
// breakers is an optional map of circuit breakers keyed by platform name (e.g. "discord", "telegram").
func NewSender(cfg *config.Config, root string, breakers map[string]*circuitbreaker.Breaker) *Sender {
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
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
		breakers:       breakers,
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

	platform, subType := platformInfo(job.Type)
	if b := s.breakers[platform]; b != nil && !b.Allow() {
		s.recordSend(platform, subType, "circuit_open", time.Since(start))
		return fmt.Errorf("%s circuit breaker open", platform)
	}

	var err error
	switch job.Type {
	case "webhook":
		err = s.sendDiscordWebhook(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "WEBHOOK (%d ms)", time.Since(start).Milliseconds())
		}
	case "discord:channel":
		err = s.sendDiscordChannel(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "CHANNEL (%d ms)", time.Since(start).Milliseconds())
		}
	case "discord:user":
		err = s.sendDiscordUser(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "USER (%d ms)", time.Since(start).Milliseconds())
		}
	case "telegram:user", "telegram:channel", "telegram:group":
		err = s.sendTelegram(job.Target, job)
		if err == nil {
			s.logJobf(logging.TimingLevel(s.cfg), job, "TELEGRAM (%d ms)", time.Since(start).Milliseconds())
		}
	default:
		return fmt.Errorf("unsupported dispatch type %s", job.Type)
	}

	elapsed := time.Since(start)
	if err != nil {
		if b := s.breakers[platform]; b != nil {
			b.RecordFailure()
		}
		s.recordSend(platform, subType, "error", elapsed)
	} else {
		if b := s.breakers[platform]; b != nil {
			b.RecordSuccess()
		}
		s.recordSend(platform, subType, "success", elapsed)
	}
	return err
}

func (s *Sender) recordSend(platform, subType, status string, elapsed time.Duration) {
	m := metrics.Get()
	if m == nil {
		return
	}
	m.DispatchSendTotal.WithLabelValues(platform, subType, status).Inc()
	m.DispatchSendDuration.WithLabelValues(platform, subType).Observe(elapsed.Seconds())
}

func platformInfo(jobType string) (platform, subType string) {
	switch {
	case strings.HasPrefix(jobType, "telegram"):
		parts := strings.SplitN(jobType, ":", 2)
		if len(parts) == 2 {
			return "telegram", parts[1]
		}
		return "telegram", jobType
	case jobType == "webhook":
		return "discord", "webhook"
	case strings.HasPrefix(jobType, "discord:"):
		parts := strings.SplitN(jobType, ":", 2)
		return "discord", parts[1]
	default:
		return jobType, jobType
	}
}
