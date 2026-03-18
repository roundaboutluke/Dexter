package webhook

import (
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/dispatch"
)

func (p *Processor) applyRateLimit(job dispatch.MessageJob) ([]dispatch.MessageJob, bool) {
	if p.rateChecker == nil || p.cfg == nil {
		return nil, true
	}
	destinationID := job.Target
	if job.Type == "webhook" && job.Name != "" {
		destinationID = job.Name
	}
	rate := p.rateChecker.ValidateMessage(destinationID, job.Type)
	if rate.PassMessage {
		return nil, true
	}
	if !rate.JustBreached {
		if logger := p.generalLogger(); logger != nil {
			logger.Infof("%s: Intercepted and stopped message for user (Rate limit) for %s %s %s Time to release: %d", job.LogReference, job.Type, job.Target, job.Name, rate.ResetTime)
		}
		return nil, false
	}
	if logger := p.generalLogger(); logger != nil {
		logger.Infof("%s: Stopping alerts (Rate limit) for %s %s %s Time to release: %d", job.LogReference, job.Type, job.Target, job.Name, rate.ResetTime)
	}

	tr := p.i18n.Translator(job.Language)
	warning := tr.TranslateFormat("You have reached the limit of {0} messages over {1} seconds", rate.MessageLimit, rate.MessageTTL)
	prefix := "/"
	if strings.HasPrefix(job.Type, "discord") || job.Type == "webhook" {
		prefix, _ = p.cfg.GetString("discord.prefix")
		if prefix == "" {
			prefix = "!"
		}
	}

	logMessage := ""
	shameMessage := ""
	disableOnStop, _ := p.cfg.GetBool("alertLimits.disableOnStop")
	maxLimits, _ := p.cfg.GetInt("alertLimits.maxLimitsBeforeStop")
	if maxLimits > 0 {
		stopRes := p.rateChecker.UserIsBanned(destinationID)
		if !stopRes.PassMessage {
			stopTemplate := "You have breached the rate limit too many times in the last 24 hours. Your messages are now stopped, use {0}start to resume"
			if disableOnStop {
				stopTemplate = "You have breached the rate limit too many times in the last 24 hours. Your messages are now stopped, contact an administrator to resume"
			}
			warning = tr.TranslateFormat(stopTemplate, prefix)

			if shouldDisable(job.Type) && p.query != nil {
				changed := false
				if disableOnStop {
					if updated, err := p.query.UpdateQuery("humans", map[string]any{
						"admin_disable": 1,
						"disabled_date": nil,
					}, map[string]any{"id": job.Target}); err == nil && updated > 0 {
						changed = true
					}
				} else {
					if updated, err := p.query.UpdateQuery("humans", map[string]any{
						"enabled": 0,
					}, map[string]any{"id": job.Target}); err == nil && updated > 0 {
						changed = true
					}
				}
				if changed {
					p.RefreshAlertCacheAsync()
				}
			}

			logMessage = fmt.Sprintf("Stopped alerts (rate-limit exceeded too many times) for target %s %s %s", job.Type, destinationID, job.Name)
			if logger := p.generalLogger(); logger != nil {
				logger.Infof("%s: Stopping alerts [until restart] (Rate limit) for %s %s %s", job.LogReference, job.Type, job.Target, job.Name)
			}
			if job.Type == "discord:user" {
				shameMessage = tr.TranslateFormat("<@{0}> has had their Poracle tracking disabled for exceeding the rate limit too many times!", destinationID)
			}
		}
	}

	warningJob := job
	warningJob.Message = warning
	warningJob.Payload = map[string]any{"content": warning}
	warningJob.AlwaysSend = true

	jobs := []dispatch.MessageJob{warningJob}
	if logMessage != "" {
		if logChannel, ok := p.cfg.GetString("discord.dmLogChannelID"); ok && logChannel != "" {
			jobs = append(jobs, dispatch.MessageJob{
				Message:      logMessage,
				Payload:      map[string]any{"content": logMessage},
				Target:       logChannel,
				Type:         "discord:channel",
				Name:         "Log channel",
				TTH:          dispatch.TimeToHide{Hours: 0, Minutes: getIntConfig(p.cfg, "discord.dmLogChannelDeletionTime", 0), Seconds: 0},
				Clean:        getIntConfig(p.cfg, "discord.dmLogChannelDeletionTime", 0) > 0,
				LogReference: job.LogReference,
				Language:     job.Language,
				AlwaysSend:   true,
			})
		}
	}
	if shameMessage != "" {
		if shameChannel, ok := p.cfg.GetString("alertLimits.shameChannel"); ok && shameChannel != "" {
			jobs = append(jobs, dispatch.MessageJob{
				Message:      shameMessage,
				Payload:      map[string]any{"content": shameMessage},
				Target:       shameChannel,
				Type:         "discord:channel",
				Name:         "Shame channel",
				TTH:          dispatch.TimeToHide{Hours: 0, Minutes: 0, Seconds: 0},
				Clean:        false,
				LogReference: job.LogReference,
				Language:     job.Language,
				AlwaysSend:   true,
			})
		}
	}
	return jobs, false
}

func shouldDisable(targetType string) bool {
	return strings.Contains(targetType, "user") || strings.Contains(targetType, "channel")
}

func getIntConfig(cfg *config.Config, path string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	value, ok := cfg.GetInt(path)
	if !ok {
		return fallback
	}
	return value
}

func (p *Processor) disabled(path string) bool {
	if p.cfg == nil {
		return false
	}
	value, _ := p.cfg.GetBool(path)
	return value
}
