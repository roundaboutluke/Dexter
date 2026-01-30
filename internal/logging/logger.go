package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"poraclego/internal/config"
)

// Level describes log severity.
type Level int

const (
	LevelSilly Level = iota
	LevelDebug
	LevelVerbose
	LevelInfo
	LevelWarn
	LevelError
)

// Logger writes structured log lines to console and files.
type Logger struct {
	name         string
	consoleLevel Level
	fileLevel    Level
	writer       *rotatingWriter
	console      io.Writer
}

// Loggers holds category loggers.
type Loggers struct {
	General    *Logger
	Webhooks   *Logger
	Discord    *Logger
	Telegram   *Logger
	Command    *Logger
	Controller *Logger
}

var global Loggers

// Init configures loggers and file rotation.
func Init(cfg *config.Config, root string) error {
	logDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}
	consoleLevel := parseLevel(getString(cfg, "logger.consoleLogLevel", "verbose"))
	fileLevel := parseLevel(getString(cfg, "logger.logLevel", "verbose"))
	dailyLimit := getInt(cfg, "logger.dailyLogLimit", 7)
	webhookLimit := getInt(cfg, "logger.webhookLogLimit", 12)

	global = Loggers{
		General:    newLogger("general", consoleLevel, fileLevel, logDir, "daily", dailyLimit),
		Webhooks:   newLogger("webhooks", consoleLevel, fileLevel, logDir, "hourly", webhookLimit),
		Discord:    newLogger("discord", consoleLevel, fileLevel, logDir, "daily", dailyLimit),
		Telegram:   newLogger("telegram", consoleLevel, fileLevel, logDir, "daily", dailyLimit),
		Command:    newLogger("commands", consoleLevel, fileLevel, logDir, "daily", dailyLimit),
		Controller: newLogger("controller", consoleLevel, fileLevel, logDir, "daily", dailyLimit),
	}

	return nil
}

// Get returns the configured loggers.
func Get() Loggers {
	return global
}

func newLogger(name string, consoleLevel, fileLevel Level, logDir, period string, keep int) *Logger {
	writer := newRotatingWriter(logDir, name, period, keep)
	return &Logger{
		name:         name,
		consoleLevel: consoleLevel,
		fileLevel:    fileLevel,
		writer:       writer,
		console:      os.Stdout,
	}
}

func (l *Logger) Debugf(format string, args ...any) {
	l.log(LevelDebug, format, args...)
}

func (l *Logger) Verbosef(format string, args ...any) {
	l.log(LevelVerbose, format, args...)
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(LevelInfo, format, args...)
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log(LevelWarn, format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(LevelError, format, args...)
}

func (l *Logger) log(level Level, format string, args ...any) {
	if l == nil {
		return
	}
	message := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), levelLabel(level), message)
	if level >= l.consoleLevel {
		_, _ = io.WriteString(l.console, line)
	}
	if level >= l.fileLevel && l.writer != nil {
		_, _ = l.writer.Write([]byte(line))
	}
}

type rotatingWriter struct {
	dir      string
	prefix   string
	period   string
	keep     int
	mu       sync.Mutex
	filename string
	file     *os.File
}

func newRotatingWriter(dir, prefix, period string, keep int) *rotatingWriter {
	return &rotatingWriter{
		dir:    dir,
		prefix: prefix,
		period: period,
		keep:   keep,
	}
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	filename := w.currentFilename()
	if w.file == nil || filename != w.filename {
		if w.file != nil {
			_ = w.file.Close()
		}
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return 0, err
		}
		w.file = file
		w.filename = filename
		cleanupLogs(w.dir, w.prefix+"-", w.keep)
	}
	return w.file.Write(p)
}

func (w *rotatingWriter) currentFilename() string {
	period := time.Now().Format("2006-01-02")
	if w.period == "hourly" {
		period = time.Now().Format("2006-01-02-15")
	}
	return filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, period))
}

func cleanupLogs(logDir, prefix string, keep int) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}
	matches := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".log") {
			matches = append(matches, filepath.Join(logDir, name))
		}
	}
	sort.Strings(matches)
	if len(matches) <= keep {
		return
	}
	for _, path := range matches[:len(matches)-keep] {
		_ = os.Remove(path)
	}
}

func parseLevel(raw string) Level {
	switch strings.ToLower(raw) {
	case "silly":
		return LevelSilly
	case "debug":
		return LevelDebug
	case "verbose":
		return LevelVerbose
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func levelLabel(level Level) string {
	switch level {
	case LevelSilly:
		return "silly"
	case LevelDebug:
		return "debug"
	case LevelVerbose:
		return "verbose"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

func getString(cfg *config.Config, path, fallback string) string {
	if cfg == nil {
		return fallback
	}
	value, ok := cfg.GetString(path)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func getInt(cfg *config.Config, path string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	value, ok := cfg.GetInt(path)
	if !ok || value <= 0 {
		return fallback
	}
	return value
}
