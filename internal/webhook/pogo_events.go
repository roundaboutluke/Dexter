package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"poraclego/internal/logging"
	"poraclego/internal/tz"
)

const defaultPogoEventsURL = "https://raw.githubusercontent.com/bigfoott/ScrapedDuck/data/events.json"

type PogoEvent struct {
	EventType string `json:"eventType"`
	Name      string `json:"name"`
	Start     string `json:"start"`
	End       string `json:"end"`
}

type EventChange struct {
	Reason string
	Name   string
	Time   string
}

type PogoEventParser struct {
	mu          sync.RWMutex
	url         string
	spawnEvents []PogoEvent
	questEvents []PogoEvent
}

func NewPogoEventParser(url string) *PogoEventParser {
	if url == "" {
		url = defaultPogoEventsURL
	}
	return &PogoEventParser{url: url}
}

func (p *PogoEventParser) Start(ctx context.Context, cachePath string) {
	if p == nil {
		return
	}
	if cachePath != "" {
		_ = p.loadCache(cachePath)
	}
	go func() {
		for {
			if err := p.refresh(cachePath); err != nil {
				if logger := logging.Get().General; logger != nil {
					logger.Warnf("pogo events refresh failed: %v", err)
				}
				select {
				case <-time.After(15 * time.Minute):
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case <-time.After(6 * time.Hour):
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (p *PogoEventParser) EventChangesSpawn(startTime, disappearTime int64, lat, lon float64, locator *tz.Locator) *EventChange {
	events := p.spawnEventList()
	return eventChange(events, startTime, disappearTime, lat, lon, locator)
}

func (p *PogoEventParser) EventChangesQuest(startTime, disappearTime int64, lat, lon float64, locator *tz.Locator) *EventChange {
	events := p.questEventList()
	return eventChange(events, startTime, disappearTime, lat, lon, locator)
}

func eventChange(events []PogoEvent, startTime, disappearTime int64, lat, lon float64, locator *tz.Locator) *EventChange {
	if len(events) == 0 || startTime == 0 || disappearTime == 0 {
		return nil
	}
	loc := time.Local
	if locator != nil {
		if l, ok := locator.Location(lat, lon); ok && l != nil {
			loc = l
		}
	}
	for _, event := range events {
		eventStart, ok := parseEventTime(event.Start, loc)
		if ok {
			start := eventStart.Unix()
			if startTime < start && start < disappearTime {
				return &EventChange{
					Reason: "start",
					Name:   event.Name,
					Time:   eventStart.In(loc).Format("15:04"),
				}
			}
		}
		eventEnd, ok := parseEventTime(event.End, loc)
		if ok {
			end := eventEnd.Unix()
			if startTime < end && end < disappearTime {
				return &EventChange{
					Reason: "end",
					Name:   event.Name,
					Time:   eventEnd.In(loc).Format("15:04"),
				}
			}
		}
	}
	return nil
}

func parseEventTime(value string, loc *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
	}
	for _, layout := range layouts {
		if strings.Contains(layout, "Z07") || strings.Contains(layout, "MST") {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed, true
			}
			continue
		}
		if loc == nil {
			loc = time.Local
		}
		if parsed, err := time.ParseInLocation(layout, value, loc); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (p *PogoEventParser) spawnEventList() []PogoEvent {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]PogoEvent, len(p.spawnEvents))
	copy(out, p.spawnEvents)
	return out
}

func (p *PogoEventParser) questEventList() []PogoEvent {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]PogoEvent, len(p.questEvents))
	copy(out, p.questEvents)
	return out
}

func (p *PogoEventParser) refresh(cachePath string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(p.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("bad status %d", resp.StatusCode)
	}
	events, raw, err := decodeEvents(resp.Body)
	if err != nil {
		return err
	}
	p.loadEvents(events)
	if cachePath != "" {
		_ = os.WriteFile(cachePath, raw, 0o644)
	}
	return nil
}

func decodeEvents(r io.Reader) ([]PogoEvent, []byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	var events []PogoEvent
	if err := json.Unmarshal(raw, &events); err == nil {
		return events, raw, nil
	}
	var wrapper struct {
		Events []PogoEvent `json:"events"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, nil, err
	}
	return wrapper.Events, raw, nil
}

func (p *PogoEventParser) loadEvents(events []PogoEvent) {
	spawn := make([]PogoEvent, 0, len(events))
	quest := make([]PogoEvent, 0, len(events))
	for _, event := range events {
		switch event.EventType {
		case "community-day", "pokemon-spotlight-hour":
			spawn = append(spawn, event)
			if event.EventType == "community-day" {
				quest = append(quest, event)
			}
		}
	}
	p.mu.Lock()
	p.spawnEvents = spawn
	p.questEvents = quest
	p.mu.Unlock()
}

func (p *PogoEventParser) loadCache(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var events []PogoEvent
	if err := json.Unmarshal(raw, &events); err != nil {
		return err
	}
	p.loadEvents(events)
	return nil
}
