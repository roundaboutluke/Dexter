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

// Logger writes log lines to console and files.
type Logger struct {
	name         string
	consoleLevel Level
	fileLevel    Level
	writer       *rotatingWriter
	errorWriter  *rotatingWriter
	console      io.Writer
}

// Loggers holds the configured category loggers.
type Loggers struct {
	General    *Logger
	Errors     *Logger
	Webhooks   *Logger
	Discord    *Logger
	Telegram   *Logger
	Commands   *Logger
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

	errorWriter := newRotatingWriter(logDir, "errors", "daily", dailyLimit)
	errorsLogger := &Logger{
		name:         "errors",
		consoleLevel: LevelError,
		fileLevel:    LevelWarn,
		writer:       errorWriter,
		console:      io.Discard,
	}
	commandsLogger := newLogger("commands", consoleLevel, fileLevel, errorWriter, logDir, "daily", dailyLimit)

	global = Loggers{
		General:    newLogger("general", consoleLevel, fileLevel, errorWriter, logDir, "daily", dailyLimit),
		Errors:     errorsLogger,
		Webhooks:   newLogger("webhooks", consoleLevel, categoryFileLevel(cfg, "logger.enableLogs.webhooks", fileLevel), errorWriter, logDir, "hourly", webhookLimit),
		Discord:    newLogger("discord", consoleLevel, categoryFileLevel(cfg, "logger.enableLogs.discord", fileLevel), errorWriter, logDir, "daily", dailyLimit),
		Telegram:   newLogger("telegram", consoleLevel, categoryFileLevel(cfg, "logger.enableLogs.telegram", fileLevel), errorWriter, logDir, "daily", dailyLimit),
		Commands:   commandsLogger,
		Command:    commandsLogger,
		Controller: newLogger("controller", consoleLevel, fileLevel, errorWriter, logDir, "daily", dailyLimit),
	}
	return nil
}

// Close closes the configured log writers.
func Close() error {
	return global.Close()
}

// Get returns the configured loggers.
func Get() Loggers {
	return global
}

// Close closes all unique log writers.
func (l Loggers) Close() error {
	seen := map[*rotatingWriter]struct{}{}
	var firstErr error
	for _, logger := range []*Logger{l.General, l.Errors, l.Webhooks, l.Discord, l.Telegram, l.Commands, l.Controller} {
		if logger == nil || logger.writer == nil {
			continue
		}
		if _, ok := seen[logger.writer]; ok {
			continue
		}
		seen[logger.writer] = struct{}{}
		if err := logger.writer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func newLogger(name string, consoleLevel, fileLevel Level, errorWriter *rotatingWriter, logDir, period string, keep int) *Logger {
	return &Logger{
		name:         name,
		consoleLevel: consoleLevel,
		fileLevel:    fileLevel,
		writer:       newRotatingWriter(logDir, name, period, keep),
		errorWriter:  errorWriter,
		console:      os.Stdout,
	}
}

// Enabled reports whether the logger would emit the given level anywhere.
func (l *Logger) Enabled(level Level) bool {
	if l == nil {
		return false
	}
	if level >= l.consoleLevel {
		return true
	}
	if level >= l.fileLevel && l.writer != nil {
		return true
	}
	return level >= LevelWarn && l.errorWriter != nil
}

// Close closes the logger's primary file writer.
func (l *Logger) Close() error {
	if l == nil || l.writer == nil {
		return nil
	}
	return l.writer.Close()
}

// Logf writes a message at the given level.
func (l *Logger) Logf(level Level, format string, args ...any) {
	l.log(level, format, args...)
}

func (l *Logger) Sillyf(format string, args ...any) {
	l.log(LevelSilly, format, args...)
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
	line := fmt.Sprintf("%s %s: %s\n", time.Now().Format("2006-01-02 15:04:05"), level.String(), message)
	if level >= l.consoleLevel && l.console != nil {
		_, _ = io.WriteString(l.console, line)
	}
	if level >= l.fileLevel && l.writer != nil {
		_, _ = l.writer.Write([]byte(line))
	}
	if level >= LevelWarn && l.errorWriter != nil && l.errorWriter != l.writer {
		_, _ = l.errorWriter.Write([]byte(line))
	}
}

// TimingLevel returns the preferred log level for timing measurements.
func TimingLevel(cfg *config.Config) Level {
	if cfg != nil {
		if enabled, ok := cfg.GetBool("logger.timingStats"); ok && enabled {
			return LevelVerbose
		}
	}
	return LevelDebug
}

// PvpEnabled reports whether verbose PvP logging is enabled.
func PvpEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	enabled, _ := cfg.GetBool("logger.enableLogs.pvp")
	return enabled
}

func categoryFileLevel(cfg *config.Config, path string, defaultLevel Level) Level {
	if cfg == nil {
		return defaultLevel
	}
	enabled, ok := cfg.GetBool(path)
	if !ok || enabled {
		return defaultLevel
	}
	return LevelWarn
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

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.filename = ""
	return err
}

func (w *rotatingWriter) currentFilename() string {
	period := time.Now().Format("2006-01-02")
	if w.period == "hourly" {
		period = time.Now().Format("2006-01-02-15")
	}
	return filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, period))
}

func cleanupLogs(logDir, prefix string, keep int) {
	if keep <= 0 {
		return
	}
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
	switch strings.ToLower(strings.TrimSpace(raw)) {
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

func (l Level) String() string {
	switch l {
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
