package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/config"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Notifier struct {
	config   *config.NotificationsConfig
	channels map[string]Channel
	mu       sync.RWMutex
	enabled  bool
}

type Channel interface {
	Send(msg *Message) error
	Format(msg *Message) string
	IsEnabled() bool
}

type Message struct {
	Type      MessageType
	Title     string
	Body      string
	Data      interface{}
	Priority  Priority
	Timestamp time.Time
	Metadata  map[string]string
}

type MessageType string

const (
	MessageTypeTrade      MessageType = "trade"
	MessageTypeOrder      MessageType = "order"
	MessageTypePosition   MessageType = "position"
	MessageTypeSignal     MessageType = "signal"
	MessageTypeRisk       MessageType = "risk"
	MessageTypeSystem     MessageType = "system"
	MessageTypeAlert      MessageType = "alert"
	MessageTypeDaily      MessageType = "daily"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

type TradeMessage struct {
	Symbol      string
	Side        string
	Quantity    string
	Price       string
	PnL         string
	Commission  string
	Exchange    string
}

type OrderMessage struct {
	OrderID    string
	Symbol     string
	Side       string
	Type       string
	Quantity   string
	Price      string
	Status     string
	Exchange   string
}

type PositionMessage struct {
	Symbol     string
	Side       string
	Quantity   string
	EntryPrice string
	CurrentPrice string
	PnL        string
	ROI        string
	Exchange   string
}

type SignalMessage struct {
	Symbol     string
	Strategy   string
	Action     string
	Strength   string
	Confidence string
	Reason     string
	Exchange   string
}

type RiskMessage struct {
	LimitType  string
	Current    string
	Limit      string
	Action     string
}

type DailyReportMessage struct {
	TotalValue   string
	DayPnL       string
	DayPnLPct    string
	TotalTrades  int
	WinRate      string
	OpenPositions int
}

func NewNotifier(cfg *config.NotificationsConfig) *Notifier {
	n := &Notifier{
		config:   cfg,
		channels: make(map[string]Channel),
		enabled:  cfg.Enabled,
	}

	if cfg.Telegram.Enabled {
		n.channels["telegram"] = NewTelegramChannel(&cfg.Telegram)
	}

	if cfg.Slack.Enabled {
		n.channels["slack"] = NewSlackChannel(&cfg.Slack)
	}

	if cfg.Discord.Enabled {
		n.channels["discord"] = NewDiscordChannel(&cfg.Discord)
	}

	if cfg.Email.Enabled {
		n.channels["email"] = NewEmailChannel(&cfg.Email)
	}

	if cfg.Webhook.Enabled {
		n.channels["webhook"] = NewWebhookChannel(&cfg.Webhook)
	}

	return n
}

func (n *Notifier) Send(msg *Message) error {
	if !n.enabled {
		return nil
	}

	n.mu.RLock()
	defer n.mu.RUnlock()

	for name, channel := range n.channels {
		if !channel.IsEnabled() {
			continue
		}

		go func(ch Channel, chName string) {
			if err := ch.Send(msg); err != nil {
				logger.Error("Failed to send notification",
					"channel", chName,
					"error", err,
				)
			}
		}(channel, name)
	}

	return nil
}

func (n *Notifier) NotifyTrade(trade *TradeMessage) error {
	msg := &Message{
		Type:      MessageTypeTrade,
		Title:     fmt.Sprintf("Trade Executed: %s %s", trade.Side, trade.Symbol),
		Body:      fmt.Sprintf("%s %s @ %s\nPnL: %s\nCommission: %s",
			trade.Side, trade.Quantity, trade.Price, trade.PnL, trade.Commission),
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"symbol":    trade.Symbol,
			"side":      trade.Side,
			"exchange":  trade.Exchange,
		},
	}

	return n.Send(msg)
}

func (n *Notifier) NotifyOrder(order *OrderMessage) error {
	emoji := "📝"
	switch order.Status {
	case "filled":
		emoji = "✅"
	case "cancelled":
		emoji = "❌"
	case "rejected":
		emoji = "🚫"
	}

	msg := &Message{
		Type:      MessageTypeOrder,
		Title:     fmt.Sprintf("%s Order %s: %s %s", emoji, order.Status, order.Side, order.Symbol),
		Body:      fmt.Sprintf("%s %s @ %s\nOrder ID: %s", order.Side, order.Quantity, order.Price, order.OrderID),
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"order_id": order.OrderID,
			"symbol":   order.Symbol,
			"status":   order.Status,
		},
	}

	return n.Send(msg)
}

func (n *Notifier) NotifyPosition(pos *PositionMessage) error {
	msg := &Message{
		Type:      MessageTypePosition,
		Title:     fmt.Sprintf("Position Update: %s %s", pos.Side, pos.Symbol),
		Body:      fmt.Sprintf("Quantity: %s\nEntry: %s\nCurrent: %s\nPnL: %s (ROI: %s)",
			pos.Quantity, pos.EntryPrice, pos.CurrentPrice, pos.PnL, pos.ROI),
		Priority:  PriorityLow,
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"symbol": pos.Symbol,
			"side":   pos.Side,
		},
	}

	return n.Send(msg)
}

func (n *Notifier) NotifySignal(signal *SignalMessage) error {
	emoji := "🔔"
	switch signal.Action {
	case "buy":
		emoji = "🟢"
	case "sell":
		emoji = "🔴"
	case "close":
		emoji = "⚠️"
	}

	msg := &Message{
		Type:      MessageTypeSignal,
		Title:     fmt.Sprintf("%s Signal: %s", emoji, signal.Action),
		Body:      fmt.Sprintf("%s\nStrategy: %s\nConfidence: %s\n%s",
			signal.Symbol, signal.Strategy, signal.Confidence, signal.Reason),
		Priority:  PriorityHigh,
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"symbol":     signal.Symbol,
			"strategy":   signal.Strategy,
			"action":     signal.Action,
			"confidence": signal.Confidence,
		},
	}

	return n.Send(msg)
}

func (n *Notifier) NotifyRisk(limit *RiskMessage) error {
	emoji := "⚠️"

	msg := &Message{
		Type:      MessageTypeRisk,
		Title:     fmt.Sprintf("%s Risk Alert: %s", emoji, limit.LimitType),
		Body:      fmt.Sprintf("Current: %s\nLimit: %s\nAction: %s",
			limit.Current, limit.Limit, limit.Action),
		Priority:  PriorityUrgent,
		Timestamp: time.Now(),
		Metadata: map[string]string{
			"limit_type": limit.LimitType,
		},
	}

	return n.Send(msg)
}

func (n *Notifier) NotifyDailyReport(report *DailyReportMessage) error {
	msg := &Message{
		Type:      MessageTypeDaily,
		Title:     "📊 Daily Trading Report",
		Body: fmt.Sprintf("Total Value: %s\nDay P&L: %s (%s)\nTrades: %d\nWin Rate: %s\nOpen Positions: %d",
			report.TotalValue, report.DayPnL, report.DayPnLPct,
			report.TotalTrades, report.WinRate, report.OpenPositions),
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
	}

	return n.Send(msg)
}

func (n *Notifier) NotifyAlert(title, body string, priority Priority) error {
	msg := &Message{
		Type:      MessageTypeAlert,
		Title:     title,
		Body:      body,
		Priority:  priority,
		Timestamp: time.Now(),
	}

	return n.Send(msg)
}

func (n *Notifier) NotifySystem(status, details string) error {
	msg := &Message{
		Type:      MessageTypeSystem,
		Title:     fmt.Sprintf("System: %s", status),
		Body:      details,
		Priority:  PriorityLow,
		Timestamp: time.Now(),
	}

	return n.Send(msg)
}

type TelegramChannel struct {
	botToken string
	chatIDs  []string
	enabled  bool
	client   *http.Client
}

func NewTelegramChannel(cfg *config.TelegramConfig) *TelegramChannel {
	return &TelegramChannel{
		botToken: cfg.BotToken,
		chatIDs:  cfg.ChatIDs,
		enabled:  cfg.Enabled,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TelegramChannel) IsEnabled() bool {
	return t.enabled && t.botToken != "" && len(t.chatIDs) > 0
}

func (t *TelegramChannel) Send(msg *Message) error {
	if !t.IsEnabled() {
		return nil
	}

	text := fmt.Sprintf("📊 *%s*\n\n%s", msg.Title, msg.Body)

	for _, chatID := range t.chatIDs {
		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

		payload := map[string]interface{}{
			"chat_id":    chatID,
			"text":       text,
			"parse_mode": "Markdown",
		}

		if msg.Priority == PriorityUrgent {
			payload["disable_notification"] = false
		} else {
			payload["disable_notification"] = true
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := t.client.Do(req)
		if err != nil {
			return fmt.Errorf("telegram send failed: %w", err)
		}
		defer resp.Body.Close()
	}

	return nil
}

func (t *TelegramChannel) Format(msg *Message) string {
	return fmt.Sprintf("📊 *%s*\n\n%s", msg.Title, msg.Body)
}

type SlackChannel struct {
	webhookURL string
	channel    string
	username   string
	iconEmoji  string
	enabled    bool
}

func NewSlackChannel(cfg *config.SlackConfig) *SlackChannel {
	return &SlackChannel{
		webhookURL: cfg.WebhookURL,
		channel:    cfg.Channel,
		username:   cfg.Username,
		iconEmoji:  cfg.IconEmoji,
		enabled:    cfg.Enabled,
	}
}

func (s *SlackChannel) IsEnabled() bool {
	return s.enabled && s.webhookURL != ""
}

func (s *SlackChannel) Send(msg *Message) error {
	if !s.IsEnabled() {
		return nil
	}

	color := "#36a64f"
	switch msg.Priority {
	case PriorityHigh:
		color = "#ff9800"
	case PriorityUrgent:
		color = "#f44336"
	}

	payload := map[string]interface{}{
		"channel":   s.channel,
		"username":  s.username,
		"icon_emoji": s.iconEmoji,
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": msg.Title,
				"text":  msg.Body,
				"ts":    msg.Timestamp.Unix(),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("slack send failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (s *SlackChannel) Format(msg *Message) string {
	return fmt.Sprintf("*%s*\n%s", msg.Title, msg.Body)
}

type DiscordChannel struct {
	webhookURL string
	username   string
	avatarURL  string
	enabled    bool
}

func NewDiscordChannel(cfg *config.DiscordConfig) *DiscordChannel {
	return &DiscordChannel{
		webhookURL: cfg.WebhookURL,
		username:   cfg.Username,
		avatarURL:  cfg.AvatarURL,
		enabled:    cfg.Enabled,
	}
}

func (d *DiscordChannel) IsEnabled() bool {
	return d.enabled && d.webhookURL != ""
}

func (d *DiscordChannel) Send(msg *Message) error {
	if !d.IsEnabled() {
		return nil
	}

	color := 3066993
	switch msg.Priority {
	case PriorityHigh:
		color = 15105570
	case PriorityUrgent:
		color = 15158332
	}

	fields := []map[string]interface{}{}
	for k, v := range msg.Metadata {
		fields = append(fields, map[string]interface{}{
			"name":   k,
			"value":  fmt.Sprintf("%v", v),
			"inline": true,
		})
	}

	payload := map[string]interface{}{
		"username": d.username,
		"avatar_url": d.avatarURL,
		"embeds": []map[string]interface{}{
			{
				"title":       msg.Title,
				"description": msg.Body,
				"color":       color,
				"timestamp":   msg.Timestamp.Format(time.RFC3339),
				"fields":      fields,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", d.webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("discord send failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (d *DiscordChannel) Format(msg *Message) string {
	return fmt.Sprintf("**%s**\n%s", msg.Title, msg.Body)
}

type EmailChannel struct {
	smtpHost  string
	smtpPort  int
	username  string
	password  string
	from      string
	to        []string
	useTLS    bool
	enabled   bool
}

func NewEmailChannel(cfg *config.EmailConfig) *EmailChannel {
	return &EmailChannel{
		smtpHost: cfg.SMTPHost,
		smtpPort: cfg.SMTPPort,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
		to:       cfg.To,
		useTLS:   cfg.UseTLS,
		enabled:  cfg.Enabled,
	}
}

func (e *EmailChannel) IsEnabled() bool {
	return e.enabled && e.smtpHost != "" && len(e.to) > 0
}

func (e *EmailChannel) Send(msg *Message) error {
	if !e.IsEnabled() {
		return nil
	}

	logger.Info("Email notification",
		"to", strings.Join(e.to, ", "),
		"subject", msg.Title,
	)

	return nil
}

func (e *EmailChannel) Format(msg *Message) string {
	return fmt.Sprintf("Subject: %s\n\n%s", msg.Title, msg.Body)
}

type WebhookChannel struct {
	urls    map[string]string
	enabled bool
}

func NewWebhookChannel(cfg *config.WebhookConfig) *WebhookChannel {
	return &WebhookChannel{
		urls:    cfg.URLs,
		enabled: cfg.Enabled,
	}
}

func (w *WebhookChannel) IsEnabled() bool {
	return w.enabled && len(w.urls) > 0
}

func (w *WebhookChannel) Send(msg *Message) error {
	if !w.IsEnabled() {
		return nil
	}

	msgType := string(msg.Type)
	webhookURL, exists := w.urls[msgType]
	if !exists {
		webhookURL, exists = w.urls["default"]
		if !exists {
			return nil
		}
	}

	payload := map[string]interface{}{
		"type":      msg.Type,
		"title":     msg.Title,
		"body":      msg.Body,
		"priority":  msg.Priority,
		"timestamp": msg.Timestamp,
		"metadata":  msg.Metadata,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook send failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (w *WebhookChannel) Format(msg *Message) string {
	return fmt.Sprintf("%s: %s", msg.Title, msg.Body)
}

type TradeFormatter struct{}

func (f *TradeFormatter) Format(trade *types.Trade) string {
	pnl := decimal.Zero
	direction := "📈"
	if trade.Side == types.OrderSideSell {
		direction = "📉"
	}

	return fmt.Sprintf("%s %s %s @ %s\nPnL: %s\nTime: %s",
		direction,
		trade.Side,
		trade.Quantity,
		trade.Price,
		pnl,
		trade.Timestamp.Format("15:04:05"),
	)
}

type OrderFormatter struct{}

func (f *OrderFormatter) Format(order *types.Order) string {
	status := "⚠️"
	switch order.Status {
	case types.OrderStatusFilled:
		status = "✅"
	case types.OrderStatusCancelled:
		status = "❌"
	case types.OrderStatusRejected:
		status = "🚫"
	}

	return fmt.Sprintf("%s Order %s: %s %s\nQuantity: %s / %s\nPrice: %s",
		status,
		order.Status,
		order.Side,
		order.Symbol,
		order.FilledQuantity,
		order.Quantity,
		order.Price,
	)
}

type PositionFormatter struct{}

func (f *PositionFormatter) Format(pos *types.Position) string {
	direction := "🟢 Long"
	if pos.Side == types.PositionSideShort {
		direction = "🔴 Short"
	}

	return fmt.Sprintf("%s %s\nQuantity: %s\nEntry: %s\nCurrent: %s\nPnL: %s (%s)",
		direction,
		pos.Symbol,
		pos.Quantity,
		pos.AvgEntryPrice,
		pos.CurrentPrice,
		pos.UnrealizedPnL,
		pos.ROI,
	)
}
