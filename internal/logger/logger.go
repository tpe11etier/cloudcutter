package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level = slog.Level

const (
	DEBUG = slog.LevelDebug // 4
	INFO  = slog.LevelInfo  // 0
	WARN  = slog.LevelWarn  // -4
	ERROR = slog.LevelError // -8
)

type Logger struct {
	logDir     string
	prefix     string
	mu         sync.Mutex
	currentDay string
	file       *os.File
	level      Level
}

type Config struct {
	LogDir string
	Prefix string
	Level  Level
}

type logEntry struct {
	Time      string `json:"time"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	PrettyLog string `json:"pretty_log,omitempty"`
	Args      []any  `json:"args,omitempty"`
}

func createLogMessage(level Level, message string, args map[string]any) string {
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("=== [%s] %s ===\n", level.String(), time.Now().Format("2006-01-02 15:04:05.000")))
	msg.WriteString(message)
	if len(args) > 0 {
		jsonBytes, _ := json.MarshalIndent(args, "", "    ")
		msg.WriteString("\nResults:\n")
		msg.WriteString(string(jsonBytes))
	}
	msg.WriteString("\n====================\n")
	return msg.String()
}

func New(cfg Config) (*Logger, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Set default level
	level := cfg.Level
	if level == 0 {
		level = INFO
	}

	today := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s.log", cfg.Prefix, today)
	fpath := filepath.Join(cfg.LogDir, filename)

	file, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	l := &Logger{
		logDir:     cfg.LogDir,
		prefix:     cfg.Prefix,
		level:      level,
		file:       file,
		currentDay: today,
	}

	initMsg := createLogMessage(INFO, "Logger initialized", map[string]any{
		"level":  level.String(),
		"dir":    cfg.LogDir,
		"prefix": cfg.Prefix,
	})

	if _, err := file.WriteString(initMsg); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write initial log message: %v", err)
	}

	return l, nil
}

func (l *Logger) rotateLogFile() error {
	currentDay := time.Now().Format("2006-01-02")

	// If the file for today is already open, do nothing
	if l.currentDay == currentDay {
		return nil
	}

	// Close existing file if open
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			fmt.Printf("Warning: Failed to close previous log file: %v\n", err)
		}
	}

	filename := fmt.Sprintf("%s_%s.log", l.prefix, currentDay)
	fpath := filepath.Join(l.logDir, filename)

	file, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	l.file = file
	l.currentDay = currentDay
	return nil
}

func formatJSON(v any) string {
	if v == nil {
		return ""
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(v); err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return strings.TrimSpace(buf.String())
}

func (l *Logger) Log(level Level, message string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotateLogFile(); err != nil {
		fmt.Printf("ERROR: Failed to rotate log file: %v\n", err)
		return
	}

	sArgs := make(map[string]any)
	prettyLog := ""

	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			break
		}
		if key, ok := args[i].(string); ok {
			if key == "pretty_log" {
				prettyLog = fmt.Sprintf("%v", args[i+1])
			} else {
				sArgs[key] = args[i+1]
			}
		}
	}

	// Build log message
	var logMsg strings.Builder
	logMsg.WriteString(fmt.Sprintf("=== [%s] %s ===\n", level.String(), time.Now().Format("2006-01-02 15:04:05.000")))

	if prettyLog != "" {
		logMsg.WriteString(prettyLog)
	} else {
		logMsg.WriteString(message)
	}

	if len(sArgs) > 0 {
		prettyJSON := formatJSON(sArgs)
		logMsg.WriteString("\nResults:\n")
		logMsg.WriteString(prettyJSON)
	}

	logMsg.WriteString("\n====================\n")

	if _, err := l.file.WriteString(logMsg.String()); err != nil {
		fmt.Printf("ERROR: Failed to write log: %v\n", err)
	}
}

func (l *Logger) Debug(message string, args ...any) {
	l.Log(DEBUG, message, args...)
}

func (l *Logger) Info(message string, args ...any) {
	l.Log(INFO, message, args...)
}

func (l *Logger) Warn(message string, args ...any) {
	l.Log(WARN, message, args...)
}

func (l *Logger) Error(message string, args ...any) {
	l.Log(ERROR, message, args...)
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %v", err)
		}
		l.file = nil
	}
	return nil
}

func ParseLevel(levelStr string) (Level, error) {
	levelStr = strings.ToLower(strings.TrimSpace(levelStr))

	switch levelStr {
	case "debug":
		return DEBUG, nil
	case "info", "":
		return INFO, nil
	case "warn", "warning":
		return WARN, nil
	case "error":
		return ERROR, nil
	default:
		return 0, fmt.Errorf("invalid log level: %s (valid options: debug, info, warn, error)", levelStr)
	}
}

func (l *Logger) SetCurrentDay(day string) {
	l.currentDay = day
}
