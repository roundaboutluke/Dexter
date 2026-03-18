package ratelimit

import (
	"strings"
	"sync"
	"time"

	"dexter/internal/config"
)

// Result describes a rate limit check.
type Result struct {
	PassMessage  bool
	JustBreached bool
	MessageCount int
	ResetTime    int
	MessageLimit int
	MessageTTL   int
}

type counter struct {
	count   int
	expires time.Time
	bad     bool
}

// Checker enforces alert limits for targets.
type Checker struct {
	mu          sync.Mutex
	cfg         *config.Config
	limits      map[string]*counter
	limitCounts map[string]*counter
}

// NewChecker constructs a new rate limit checker.
func NewChecker(cfg *config.Config) *Checker {
	return &Checker{
		cfg:         cfg,
		limits:      map[string]*counter{},
		limitCounts: map[string]*counter{},
	}
}

// ValidateMessage checks if a message should pass rate limits.
func (c *Checker) ValidateMessage(id, targetType string) Result {
	timeout := c.messageTimeout()
	limit := c.messageLimit(id, targetType)

	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()

	entry := c.limits[id]
	if entry == nil || now.After(entry.expires) {
		entry = &counter{count: 1, expires: now.Add(time.Duration(timeout) * time.Second)}
		c.limits[id] = entry
	} else {
		entry.count++
	}

	resetTime := int(entry.expires.Sub(now).Seconds())
	if resetTime < 1 {
		resetTime = 1
	}
	if entry.count > limit {
		entry.bad = true
	}

	return Result{
		PassMessage:  entry.count <= limit,
		JustBreached: entry.count == limit+1,
		MessageCount: entry.count,
		ResetTime:    resetTime,
		MessageLimit: limit,
		MessageTTL:   timeout,
	}
}

// UserIsBanned tracks repeated breaches.
func (c *Checker) UserIsBanned(id string) Result {
	maxLimits := c.maxLimitsBeforeStop()
	timeout := 24 * 60 * 60

	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()

	entry := c.limitCounts[id]
	if entry == nil || now.After(entry.expires) {
		entry = &counter{count: 1, expires: now.Add(time.Duration(timeout) * time.Second)}
		c.limitCounts[id] = entry
	} else {
		entry.count++
	}

	resetTime := int(entry.expires.Sub(now).Seconds())
	if resetTime < 1 {
		resetTime = 1
	}

	return Result{
		PassMessage:  entry.count <= maxLimits,
		JustBreached: entry.count == maxLimits+1,
		MessageCount: entry.count,
		ResetTime:    resetTime,
		MessageLimit: maxLimits,
		MessageTTL:   timeout,
	}
}

// GetBadBoys returns current rate-limited entries.
func (c *Checker) GetBadBoys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	out := []string{}
	for key, entry := range c.limits {
		if entry.bad && now.Before(entry.expires) {
			out = append(out, key)
		}
	}
	return out
}

func (c *Checker) messageTimeout() int {
	if c.cfg == nil {
		return 0
	}
	timeout, _ := c.cfg.GetInt("alertLimits.timingPeriod")
	if timeout <= 0 {
		timeout = 1
	}
	return timeout
}

func (c *Checker) messageLimit(id, targetType string) int {
	if c.cfg == nil {
		return 0
	}
	if raw, ok := c.cfg.Get("alertLimits.limitOverride"); ok {
		if overrides, ok := raw.(map[string]any); ok {
			if value, ok := overrides[id]; ok {
				if limit, ok := value.(float64); ok {
					return int(limit)
				}
				if limit, ok := value.(int); ok {
					return limit
				}
			}
		}
	}
	if strings.Contains(targetType, "user") {
		limit, _ := c.cfg.GetInt("alertLimits.dmLimit")
		return limit
	}
	limit, _ := c.cfg.GetInt("alertLimits.channelLimit")
	return limit
}

func (c *Checker) maxLimitsBeforeStop() int {
	if c.cfg == nil {
		return 0
	}
	limit, _ := c.cfg.GetInt("alertLimits.maxLimitsBeforeStop")
	return limit
}
