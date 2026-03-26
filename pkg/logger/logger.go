package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	*logrus.Logger
	fields  logrus.Fields
	mu      sync.RWMutex
	outputs []io.Writer
}

type LoggerConfig struct {
	Level            string `mapstructure:"level"`
	Format           string `mapstructure:"format"`
	Output           string `mapstructure:"output"`
	ReportCaller     bool   `mapstructure:"report_caller"`
	TimestampFormat  string `mapstructure:"timestamp_format"`
	MaxSize          int    `mapstructure:"max_size"`
	MaxBackups       int    `mapstructure:"max_backups"`
	MaxAge           int    `mapstructure:"max_age"`
	Compress         bool   `mapstructure:"compress"`
	EnableFileLog    bool   `mapstructure:"enable_file_log"`
	EnableConsoleLog bool   `mapstructure:"enable_console_log"`
}

type LogLevel string

const (
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarn    LogLevel = "warn"
	LogLevelError   LogLevel = "error"
	LogLevelFatal   LogLevel = "fatal"
	LogLevelPanic   LogLevel = "panic"
)

var (
	defaultLogger *Logger
	once          sync.Once
)

func New(config *LoggerConfig) *Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)

	if config == nil {
		config = &LoggerConfig{
			Level:            "info",
			Format:           "json",
			EnableConsoleLog: true,
			EnableFileLog:    false,
			TimestampFormat:  time.RFC3339,
		}
	}

	switch strings.ToLower(config.Format) {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: config.TimestampFormat,
			CallerPrettyfier: func(r *runtime.Frame) (string, string) {
				return "", fmt.Sprintf("%s:%d", filepath.Base(r.File), r.Line)
			},
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
				logrus.FieldKeyFunc:  "function",
				logrus.FieldKeyFile:  "file",
			},
		})
	case "text", "pretty":
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: config.TimestampFormat,
			FullTimestamp:   true,
			CallerPrettyfier: func(r *runtime.Frame) (string, string) {
				return "", fmt.Sprintf("%s:%d", filepath.Base(r.File), r.Line)
			},
			DisableColors: false,
		})
	default:
		logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: config.TimestampFormat,
			FullTimestamp:   true,
		})
	}

	level, err := logrus.ParseLevel(strings.ToLower(config.Level))
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	var outputs []io.Writer

	if config.EnableConsoleLog {
		outputs = append(outputs, os.Stdout)
	}

	if config.EnableFileLog {
		if err := os.MkdirAll(filepath.Dir(config.Output), 0755); err != nil {
			logger.Warnf("Failed to create log directory: %v", err)
		} else {
			fileLogger := &lumberjack.Logger{
				Filename:   config.Output,
				MaxSize:    config.MaxSize,
				MaxBackups: config.MaxBackups,
				MaxAge:     config.MaxAge,
				Compress:   config.Compress,
			}
			outputs = append(outputs, fileLogger)
		}
	}

	if len(outputs) > 0 {
		logger.SetOutput(io.MultiWriter(outputs...))
	}

	return &Logger{
		Logger: logger,
		fields: make(logrus.Fields),
		outputs: outputs,
	}
}

func Default() *Logger {
	once.Do(func() {
		defaultLogger = New(&LoggerConfig{
			Level:            "info",
			Format:           "json",
			EnableConsoleLog: true,
			EnableFileLog:    false,
			TimestampFormat:  time.RFC3339,
		})
	})
	return defaultLogger
}

func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newFields := make(logrus.Fields, len(l.fields)+1)
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &Logger{
		Logger: l.Logger,
		fields: newFields,
		outputs: l.outputs,
	}
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newFields := make(logrus.Fields, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		Logger: l.Logger,
		fields: newFields,
		outputs: l.outputs,
	}
}

func (l *Logger) WithError(err error) *Logger {
	return l.WithField("error", err.Error())
}

func (l *Logger) WithCaller() *Logger {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return l
	}
	return l.WithFields(map[string]interface{}{
		"file": file,
		"line": line,
	})
}

func (l *Logger) WithContext(ctx map[string]interface{}) *Logger {
	return l.WithFields(ctx)
}

func (l *Logger) WithStrategy(strategyID string) *Logger {
	return l.WithField("strategy_id", strategyID)
}

func (l *Logger) WithExchange(exchange types.Exchange) *Logger {
	return l.WithField("exchange", exchange)
}

func (l *Logger) WithSymbol(symbol string) *Logger {
	return l.WithField("symbol", symbol)
}

func (l *Logger) WithOrder(orderID string) *Logger {
	return l.WithField("order_id", orderID)
}

func (l *Logger) WithTrade(tradeID string) *Logger {
	return l.WithField("trade_id", tradeID)
}

func (l *Logger) WithPosition(positionID string) *Logger {
	return l.WithField("position_id", positionID)
}

func (l *Logger) WithTransaction(txID string) *Logger {
	return l.WithField("tx_id", txID)
}

func (l *Logger) WithExecutionTime(start time.Time) *Logger {
	return l.WithField("execution_time", time.Since(start).String())
}

func (l *Logger) WithDuration(d time.Duration) *Logger {
	return l.WithField("duration", d.String())
}

func (l *Logger) WithPnL(pnl string) *Logger {
	return l.WithField("pnl", pnl)
}

func (l *Logger) WithBalance(asset string, amount string) *Logger {
	return l.WithFields(map[string]interface{}{
		"asset":  asset,
		"amount": amount,
	})
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Logger.WithFields(l.fields).Debugf(format, args...)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.Logger.WithFields(l.fields).Infof(format, args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Logger.WithFields(l.fields).Warnf(format, args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Logger.WithFields(l.fields).Errorf(format, args...)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Logger.WithFields(l.fields).Fatalf(format, args...)
}

func (l *Logger) Panicf(format string, args ...interface{}) {
	l.Logger.WithFields(l.fields).Panicf(format, args...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.Logger.WithFields(l.fields).Debug(args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.Logger.WithFields(l.fields).Info(args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.Logger.WithFields(l.fields).Warn(args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.Logger.WithFields(l.fields).Error(args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.Logger.WithFields(l.fields).Fatal(args...)
}

func (l *Logger) Panic(args ...interface{}) {
	l.Logger.WithFields(l.fields).Panic(args...)
}

func (l *Logger) Debugw(msg string, keysAndValues ...interface{}) {
	l.Logger.WithFields(l.fields).WithFields(logrus.Fields(keysAndValuesToMap(keysAndValues))).Debug(msg)
}

func (l *Logger) Infow(msg string, keysAndValues ...interface{}) {
	l.Logger.WithFields(l.fields).WithFields(logrus.Fields(keysAndValuesToMap(keysAndValues))).Info(msg)
}

func (l *Logger) Warnw(msg string, keysAndValues ...interface{}) {
	l.Logger.WithFields(l.fields).WithFields(logrus.Fields(keysAndValuesToMap(keysAndValues))).Warn(msg)
}

func (l *Logger) Errorw(msg string, keysAndValues ...interface{}) {
	l.Logger.WithFields(l.fields).WithFields(logrus.Fields(keysAndValuesToMap(keysAndValues))).Error(msg)
}

func (l *Logger) Fatalw(msg string, keysAndValues ...interface{}) {
	l.Logger.WithFields(l.fields).WithFields(logrus.Fields(keysAndValuesToMap(keysAndValues))).Fatal(msg)
}

func keysAndValuesToMap(keysAndValues []interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			key = fmt.Sprintf("key%d", i)
		}
		result[key] = keysAndValues[i+1]
	}
	return result
}

func (l *Logger) AddHook(hook logrus.Hook) {
	l.Logger.AddHook(hook)
}

func (l *Logger) SetLevel(level LogLevel) {
	lvl, err := logrus.ParseLevel(string(level))
	if err != nil {
		return
	}
	l.Logger.SetLevel(lvl)
}

func (l *Logger) SetFormat(format string) {
	switch strings.ToLower(format) {
	case "json":
		l.Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	case "text", "pretty":
		l.Logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: time.RFC3339,
			FullTimestamp:   true,
		})
	}
}

func (l *Logger) AddOutput(writer io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.outputs = append(l.outputs, writer)
	l.Logger.SetOutput(io.MultiWriter(l.outputs...))
}

func (l *Logger) Trade(exchange types.Exchange, symbol, side, status string, price, quantity, pnl string) {
	l.WithFields(map[string]interface{}{
		"exchange": exchange,
		"symbol":   symbol,
		"side":     side,
		"status":   status,
		"price":    price,
		"qty":      quantity,
		"pnl":      pnl,
		"type":     "trade",
	}).Info("Trade executed")
}

func (l *Logger) Order(exchange types.Exchange, symbol, orderID, side, orderType, status string) {
	l.WithFields(map[string]interface{}{
		"exchange":   exchange,
		"symbol":     symbol,
		"order_id":   orderID,
		"side":       side,
		"order_type": orderType,
		"status":     status,
		"type":       "order",
	}).Info("Order update")
}

func (l *Logger) Position(exchange types.Exchange, symbol, side string, entryPrice, currentPrice, unrealizedPnL, roi string) {
	l.WithFields(map[string]interface{}{
		"exchange":      exchange,
		"symbol":        symbol,
		"side":          side,
		"entry_price":   entryPrice,
		"current_price": currentPrice,
		"unrealized_pnl": unrealizedPnL,
		"roi":           roi,
		"type":          "position",
	}).Info("Position update")
}

func (l *Logger) Signal(strategyID, symbol, action, reason string, strength, confidence string) {
	l.WithFields(map[string]interface{}{
		"strategy_id": strategyID,
		"symbol":      symbol,
		"action":      action,
		"reason":      reason,
		"strength":    strength,
		"confidence":  confidence,
		"type":        "signal",
	}).Info("Trading signal")
}

func (l *Logger) Risk(limitType, exchange, symbol string, limit, current string, exceeded bool) {
	level := logrus.InfoLevel
	if exceeded {
		level = logrus.WarnLevel
	}
	l.WithFields(map[string]interface{}{
		"limit_type": limitType,
		"exchange":   exchange,
		"symbol":      symbol,
		"limit":       limit,
		"current":     current,
		"exceeded":    exceeded,
		"type":        "risk",
	}).Log(level, "Risk limit check")
}

func (l *Logger) Backtest(progress int, total int, pnl, sharpe, drawdown string) {
	l.WithFields(map[string]interface{}{
		"progress":  strconv.Itoa(progress) + "/" + strconv.Itoa(total),
		"pnl":       pnl,
		"sharpe":    sharpe,
		"drawdown":  drawdown,
		"type":      "backtest",
	}).Info("Backtest progress")
}

func (l *Logger) Health(component, status string, metrics map[string]interface{}) {
	l.WithFields(map[string]interface{}{
		"component": component,
		"status":   status,
		"metrics":  metrics,
		"type":     "health",
	}).Info("Health check")
}

func (l *Logger) Metrics(name string, value float64, tags map[string]string) {
	l.WithFields(map[string]interface{}{
		"name":   name,
		"value":  value,
		"tags":   tags,
		"type":   "metrics",
	}).Debug("Metric recorded")
}

func (l *Logger) API(method, endpoint string, statusCode int, latency time.Duration, err error) {
	fields := map[string]interface{}{
		"method":    method,
		"endpoint":  endpoint,
		"status":    statusCode,
		"latency":   latency.String(),
		"type":      "api",
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	l.WithFields(fields).Debug("API request")
}

func (l *Logger) WebSocket(exchange, event string, data map[string]interface{}) {
	l.WithFields(map[string]interface{}{
		"exchange": exchange,
		"event":    event,
		"data":     data,
		"type":     "websocket",
	}).Debug("WebSocket event")
}

func (l *Logger) Database(operation, table string, rows int, latency time.Duration, err error) {
	fields := map[string]interface{}{
		"operation": operation,
		"table":     table,
		"rows":      rows,
		"latency":   latency.String(),
		"type":      "database",
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	l.WithFields(fields).Debug("Database operation")
}

func (l *Logger) Cache(operation, key string, hit bool, latency time.Duration) {
	l.WithFields(map[string]interface{}{
		"operation": operation,
		"key":       key,
		"hit":       hit,
		"latency":   latency.String(),
		"type":      "cache",
	}).Debug("Cache operation")
}

func (l *Logger) AI(operation, model string, tokens int, latency time.Duration, input, output string) {
	l.WithFields(map[string]interface{}{
		"operation": operation,
		"model":     model,
		"tokens":    tokens,
		"latency":   latency.String(),
		"input":     input,
		"output":    output,
		"type":      "ai",
	}).Debug("AI operation")
}

func (l *Logger) StrategyUpdate(strategyID, name, status string, metrics map[string]interface{}) {
	l.WithFields(map[string]interface{}{
		"strategy_id": strategyID,
		"name":        name,
		"status":      status,
		"metrics":     metrics,
		"type":        "strategy",
	}).Info("Strategy update")
}

func (l *Logger) Shutdown() {
	l.Info("Logger shutdown initiated")
	l.Logger.WithFields(l.fields).Info("Logger shutdown complete")
}

func Debug(args ...interface{})                    { Default().Debug(args...) }
func Info(args ...interface{})                    { Default().Info(args...) }
func Warn(args ...interface{})                    { Default().Warn(args...) }
func Error(args ...interface{})                   { Default().Error(args...) }
func Fatal(args ...interface{})                   { Default().Fatal(args...) }
func Panic(args ...interface{})                   { Default().Panic(args...) }
func Debugf(format string, args ...interface{})    { Default().Debugf(format, args...) }
func Infof(format string, args ...interface{})    { Default().Infof(format, args...) }
func Warnf(format string, args ...interface{})    { Default().Warnf(format, args...) }
func Errorf(format string, args ...interface{})   { Default().Errorf(format, args...) }
func Fatalf(format string, args ...interface{})   { Default().Fatalf(format, args...) }
func Panicf(format string, args ...interface{})   { Default().Panicf(format, args...) }
func WithField(key string, val interface{}) *Logger { return Default().WithField(key, val) }
func WithFields(fields map[string]interface{}) *Logger { return Default().WithFields(fields) }
func WithError(err error) *Logger                 { return Default().WithError(err) }
