package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/logger"
)

type AuditLevel string

const (
	AuditLevelInfo  AuditLevel = "info"
	AuditLevelWarn  AuditLevel = "warn"
	AuditLevelError AuditLevel = "error"
	AuditLevelCrit  AuditLevel = "critical"
)

type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Level       AuditLevel              `json:"level"`
	Action      string                 `json:"action"`
	Actor       string                 `json:"actor"`
	Resource    string                 `json:"resource"`
	ResourceID  string                 `json:"resource_id"`
	IPAddress   string                 `json:"ip_address"`
	UserAgent   string                 `json:"user_agent"`
	Success     bool                   `json:"success"`
	ErrorMsg    string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RiskScore   float64               `json:"risk_score,omitempty"`
}

type AuditLogger interface {
	Log(event *AuditEvent)
	LogLogin(userID, ip string, success bool)
	LogLogout(userID string)
	LogOrder(orderID string, action string, success bool)
	LogTrade(tradeID string, symbol string, side string, amount decimal.Decimal)
	LogRiskViolation(violation string, details map[string]interface{})
	LogAPIKeyCreated(keyID string)
	LogAPIKeyRevoked(keyID string)
	LogConfigChange(key string, oldValue, newValue interface{})
}

type auditLogger struct {
	mu         sync.RWMutex
	events     []*AuditEvent
	maxEvents  int
	encrypt    bool
	encryptionKey []byte
	output     *os.File
	handlers   []AuditHandler
}

type AuditHandler func(event *AuditEvent)

func NewAuditLogger(opts ...AuditOption) AuditLogger {
	l := &auditLogger{
		events:    make([]*AuditEvent, 0),
		maxEvents: 10000,
	}

	for _, opt := range opts {
		opt(l)
	}

	if l.output == nil {
		l.output = os.Stdout
	}

	return l
}

type AuditOption func(*auditLogger)

func WithMaxEvents(max int) AuditOption {
	return func(l *auditLogger) {
		l.maxEvents = max
	}
}

func WithEncryption(key string) AuditOption {
	return func(l *auditLogger) {
		l.encrypt = true
		l.encryptionKey = sha256.Sum256([]byte(key))
	}
}

func WithFileOutput(path string) AuditOption {
	return func(l *auditLogger) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("Failed to open audit log file", "error", err)
			return
		}
		l.output = f
	}
}

func (l *auditLogger) Log(event *AuditEvent) {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	l.mu.Lock()
	l.events = append(l.events, event)
	if len(l.events) > l.maxEvents {
		l.events = l.events[len(l.events)-l.maxEvents:]
	}
	l.mu.Unlock()

	for _, handler := range l.handlers {
		go handler(event)
	}

	l.writeEvent(event)
}

func (l *auditLogger) LogLogin(userID, ip string, success bool) {
	level := AuditLevelInfo
	if !success {
		level = AuditLevelWarn
	}

	l.Log(&AuditEvent{
		Level:     level,
		Action:    "login",
		Actor:     userID,
		Resource:  "auth",
		IPAddress: ip,
		Success:   success,
		Metadata: map[string]interface{}{
			"user_id": userID,
		},
	})
}

func (l *auditLogger) LogLogout(userID string) {
	l.Log(&AuditEvent{
		Level:    AuditLevelInfo,
		Action:   "logout",
		Actor:    userID,
		Resource: "auth",
		Success:  true,
	})
}

func (l *auditLogger) LogOrder(orderID string, action string, success bool) {
	level := AuditLevelInfo
	if !success {
		level = AuditLevelWarn
	}

	l.Log(&AuditEvent{
		Level:      level,
		Action:     action,
		Actor:      "system",
		Resource:   "order",
		ResourceID: orderID,
		Success:    success,
	})
}

func (l *auditLogger) LogTrade(tradeID string, symbol string, side string, amount decimal.Decimal) {
	l.Log(&AuditEvent{
		Level:      AuditLevelInfo,
		Action:     "trade_executed",
		Resource:   "trade",
		ResourceID: tradeID,
		Success:    true,
		Metadata: map[string]interface{}{
			"symbol": symbol,
			"side":   side,
			"amount": amount.String(),
		},
	})
}

func (l *auditLogger) LogRiskViolation(violation string, details map[string]interface{}) {
	l.Log(&AuditEvent{
		Level:      AuditLevelCrit,
		Action:     "risk_violation",
		Resource:   "risk",
		Success:    false,
		ErrorMsg:   violation,
		RiskScore:  1.0,
		Metadata:   details,
	})
}

func (l *auditLogger) LogAPIKeyCreated(keyID string) {
	l.Log(&AuditEvent{
		Level:      AuditLevelWarn,
		Action:     "api_key_created",
		Resource:   "api_key",
		ResourceID: keyID,
		Success:    true,
	})
}

func (l *auditLogger) LogAPIKeyRevoked(keyID string) {
	l.Log(&AuditEvent{
		Level:      AuditLevelWarn,
		Action:     "api_key_revoked",
		Resource:   "api_key",
		ResourceID: keyID,
		Success:    true,
	})
}

func (l *auditLogger) LogConfigChange(key string, oldValue, newValue interface{}) {
	l.Log(&AuditEvent{
		Level:    AuditLevelWarn,
		Action:   "config_change",
		Resource: "config",
		Success:  true,
		Metadata: map[string]interface{}{
			"key":       key,
			"old_value": fmt.Sprintf("%v", oldValue),
			"new_value": fmt.Sprintf("%v", newValue),
		},
	})
}

func (l *auditLogger) writeEvent(event *AuditEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal audit event", "error", err)
		return
	}

	if l.encrypt {
		data = l.encryptData(data)
	}

	fmt.Fprintln(l.output, string(data))
}

func (l *auditLogger) encryptData(data []byte) []byte {
	block, err := aes.NewCipher(l.encryptionKey)
	if err != nil {
		return data
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return data
	}

	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return []byte(base64.StdEncoding.EncodeToString(ciphertext))
}

func (l *auditLogger) RegisterHandler(handler AuditHandler) {
	l.handlers = append(l.handlers, handler)
}

func (l *auditLogger) GetEvents(filter func(*AuditEvent) bool) []*AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*AuditEvent, 0)
	for _, e := range l.events {
		if filter(e) {
			result = append(result, e)
		}
	}
	return result
}

func RiskScoreLogin(failedAttempts int, lastLogin time.Time) float64 {
	score := 0.0

	if failedAttempts > 5 {
		score += 0.5
	} else if failedAttempts > 0 {
		score += float64(failedAttempts) * 0.1
	}

	if time.Since(lastLogin) < time.Minute {
		score += 0.2
	}

	return score
}

func RiskScoreTrade(amount decimal.Decimal, symbol string, dailyVolume decimal.Decimal) float64 {
	score := 0.0

	ratio := amount.Div(dailyVolume)
	if ratio.GreaterThan(decimal.NewFromFloat(0.5)) {
		score += 0.5
	} else if ratio.GreaterThan(decimal.NewFromFloat(0.2)) {
		score += 0.3
	}

	return score
}
