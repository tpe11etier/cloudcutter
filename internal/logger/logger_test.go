package logger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tpelletiersophos/cloudcutter/internal/logger"
)

func TestLoggerTableDriven(t *testing.T) {
	tests := []struct {
		name       string
		config     logger.Config
		logLevel   logger.Level
		message    string
		args       []any
		expectsErr bool
		verifyFunc func(t *testing.T, logDir string)
	}{
		{
			name: "Basic info log",
			config: logger.Config{
				LogDir: t.TempDir(),
				Prefix: "test",
				Level:  logger.INFO,
			},
			logLevel: logger.INFO,
			message:  "Test info log",
			args:     []any{"key1", "value1"},
			verifyFunc: func(t *testing.T, logDir string) {
				logFile := filepath.Join(logDir, "test_"+time.Now().Format("2006-01-02")+".log")
				content, err := os.ReadFile(logFile)
				if err != nil {
					t.Fatalf("Failed to read log file: %v", err)
				}
				if !contains(content, "Test info log") || !contains(content, "key1") || !contains(content, "value1") {
					t.Fatal("Expected log message or arguments not found in log file")
				}
			},
		},
		{
			name: "Log rotation",
			config: logger.Config{
				LogDir: t.TempDir(),
				Prefix: "test",
				Level:  logger.INFO,
			},
			logLevel: logger.INFO,
			message:  "Rotated log",
			args:     nil,
			verifyFunc: func(t *testing.T, logDir string) {
				logFile := filepath.Join(logDir, "test_"+time.Now().Format("2006-01-02")+".log")
				if _, err := os.Stat(logFile); os.IsNotExist(err) {
					t.Fatalf("Expected log file does not exist after rotation: %v", logFile)
				}
			},
		},
		{
			name: "Log level filtering",
			config: logger.Config{
				LogDir: t.TempDir(),
				Prefix: "test",
				Level:  logger.WARN,
			},
			logLevel: logger.INFO,
			message:  "Filtered log",
			args:     nil,
			verifyFunc: func(t *testing.T, logDir string) {
				logFile := filepath.Join(logDir, "test_"+time.Now().Format("2006-01-02")+".log")
				content, err := os.ReadFile(logFile)
				if err != nil {
					t.Fatalf("Failed to read log file: %v", err)
				}
				if contains(content, "Filtered log") {
					t.Fatal("Log message should have been filtered based on log level")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logDir := test.config.LogDir
			loggerInstance, err := logger.New(test.config)
			if (err != nil) != test.expectsErr {
				t.Fatalf("Unexpected error state: %v", err)
			}
			defer loggerInstance.Close()

			if !test.expectsErr {
				loggerInstance.Log(test.logLevel, test.message, test.args...)
				test.verifyFunc(t, logDir)
			}
		})
	}
}

func contains(content []byte, substr string) bool {
	return strings.Contains(string(content), substr)
}
