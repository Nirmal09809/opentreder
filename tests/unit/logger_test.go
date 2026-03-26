package opentreder_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerInitialization(t *testing.T) {
	t.Run("Default logger", func(t *testing.T) {
		logger := GetLogger()
		require.NotNil(t, logger)
		assert.Equal(t, LevelInfo, logger.level)
	})

	t.Run("Custom logger", func(t *testing.T) {
		logger := New(Config{
			Level:      LevelDebug,
			Format:     FormatJSON,
			Output:     os.Stdout,
			TimeFormat: time.RFC3339,
		})
		require.NotNil(t, logger)
		assert.Equal(t, LevelDebug, logger.level)
	})
}

func TestLogLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelDebug,
		Format: FormatText,
		Output: &buf,
	})

	testCases := []struct {
		level    Level
		testFunc func(string, ...interface{})
		prefix   string
	}{
		{LevelDebug, func(msg string, args ...interface{}) { logger.Debug(msg, args...) }, "[DEBUG]"},
		{LevelInfo, func(msg string, args ...interface{}) { logger.Info(msg, args...) }, "[INFO]"},
		{LevelWarn, func(msg string, args ...interface{}) { logger.Warn(msg, args...) }, "[WARN]"},
		{LevelError, func(msg string, args ...interface{}) { logger.Error(msg, args...) }, "[ERROR]"},
	}

	for _, tc := range testCases {
		t.Run(tc.prefix, func(t *testing.T) {
			buf.Reset()
			tc.testFunc("Test message")
			output := buf.String()

			assert.Contains(t, output, tc.prefix)
			assert.Contains(t, output, "Test message")
		})
	}
}

func TestLogFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelWarn,
		Format: FormatText,
		Output: &buf,
	})

	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message")

	output := buf.String()

	assert.NotContains(t, output, "Debug message")
	assert.NotContains(t, output, "Info message")
	assert.Contains(t, output, "Warning message")
	assert.Contains(t, output, "Error message")
}

func TestStructuredLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: &buf,
	})

	logger.Info("Order placed", "order_id", "12345", "symbol", "BTC/USDT", "quantity", 0.5)

	output := buf.String()
	require.NotEmpty(t, output)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "Order placed", logEntry["message"])
	assert.Equal(t, "12345", logEntry["order_id"])
	assert.Equal(t, "BTC/USDT", logEntry["symbol"])
}

func TestLogWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: &buf,
	})

	orderLogger := logger.WithFields(Fields{
		"order_id": "12345",
		"user_id":  "user_001",
	})

	orderLogger.Info("Order executed")

	output := buf.String()
	require.NotEmpty(t, output)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(output), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "12345", logEntry["order_id"])
	assert.Equal(t, "user_001", logEntry["user_id"])
	assert.Equal(t, "Order executed", logEntry["message"])
}

func TestLogRotation(t *testing.T) {
	tempDir := t.TempDir()

	logger := New(Config{
		Level:     LevelInfo,
		Format:    FormatText,
		Output:    nil,
		Rotate: RotateConfig{
			Enabled:   true,
			Directory:  tempDir,
			MaxSize:   1024, // 1KB
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
		},
	})

	require.NotNil(t, logger)

	for i := 0; i < 100; i++ {
		logger.Info(fmt.Sprintf("Log message number %d", i))
	}

	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 1)
}

func TestConcurrentLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelInfo,
		Format: FormatText,
		Output: &buf,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info(fmt.Sprintf("Concurrent message %d", id))
		}(i)
	}

	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, 100, len(lines))
}

func TestLogFormatOptions(t *testing.T) {
	t.Run("Text format", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(Config{
			Level:  LevelInfo,
			Format: FormatText,
			Output: &buf,
		})

		logger.Info("Test message")

		output := buf.String()
		assert.Contains(t, output, "[INFO]")
		assert.Contains(t, output, "Test message")
		assert.Contains(t, output, time.Now().Format("2006-01-02"))
	})

	t.Run("JSON format", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(Config{
			Level:  LevelInfo,
			Format: FormatJSON,
			Output: &buf,
		})

		logger.Info("Test message")

		output := buf.String()
		assert.True(t, strings.HasPrefix(output, "{"))
		assert.True(t, strings.HasSuffix(output, "}"))
	})

	t.Run("Custom format", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(Config{
			Level:  LevelInfo,
			Format: FormatCustom,
			Output: &buf,
		})

		logger.Info("Test message")

		output := buf.String()
		assert.NotEmpty(t, output)
	})
}

func TestLevelFromString(t *testing.T) {
	testCases := []struct {
		input    string
		expected Level
		valid    bool
	}{
		{"debug", LevelDebug, true},
		{"DEBUG", LevelDebug, true},
		{"info", LevelInfo, true},
		{"INFO", LevelInfo, true},
		{"warn", LevelWarn, true},
		{"WARN", LevelWarn, true},
		{"error", LevelError, true},
		{"ERROR", LevelError, true},
		{"invalid", LevelInfo, false},
		{"", LevelInfo, false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			level, err := LevelFromString(tc.input)
			if tc.valid {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, level)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestLoggerFields(t *testing.T) {
	fields := Fields{
		"string_field": "value",
		"int_field":   42,
		"float_field": 3.14,
		"bool_field":   true,
	}

	assert.Equal(t, "value", fields["string_field"])
	assert.Equal(t, 42, fields["int_field"])
	assert.Equal(t, 3.14, fields["float_field"])
	assert.Equal(t, true, fields["bool_field"])
}

func TestGlobalLoggerReplacement(t *testing.T) {
	originalLogger := globalLogger
	defer func() { globalLogger = originalLogger }()

	newLogger := New(Config{Level: LevelDebug})
	SetLogger(newLogger)

	assert.Equal(t, newLogger, globalLogger)
}

func BenchmarkLoggerInfo(b *testing.B) {
	logger := New(Config{
		Level:  LevelInfo,
		Format: FormatText,
		Output: os.Stdout,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message", "iteration", i)
	}
}

func BenchmarkLoggerJSON(b *testing.B) {
	logger := New(Config{
		Level:  LevelInfo,
		Format: FormatJSON,
		Output: os.Stdout,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message", "iteration", i, "data", map[string]int{"a": 1, "b": 2})
	}
}
