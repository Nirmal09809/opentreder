package alpaca

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Client struct {
	apiKey        string
	apiSecret     string
	baseURL       string
	dataURL       string
	httpClient    *http.Client
	wsConn        *WebSocketClient
	rateLimiter   *RateLimiter
	account       *types.Account
	accountMu     sync.RWMutex
}

type Config struct {
	APIKey    string
	APISecret string
	Paper     bool
}

type RateLimiter struct {
	mu              sync.Mutex
	lastRequestTime time.Time
	minInterval     time.Duration
}

type APIResponse struct {
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

type AccountResponse struct {
	ID                 string `json:"id"`
	AccountNumber      string `json:"account_number"`
	Status             string `json:"status"`
	Currency           string `json:"currency"`
	BuyingPower        string `json:"buying_power"`
	Cash                string `json:"cash"`
	PortfolioValue     string `json:"portfolio_value"`
	Equity             string `json:"equity"`
	LastEquity         string `json:"last_equity"`
	PatternDayTrader   bool   `json:"pattern_day_trader"`
	DayTradeCount      int    `json:"day_trade_count"`
	TradingBlocked     bool   `json:"trading_blocked"`
	TransfersBlocked   bool   `json:"transfers_blocked"`
	AccountBlocked     bool   `json:"account_blocked"`
}

type OrderRequest struct {
	Symbol           string `json:"symbol"`
	Side             string `json:"side"`
	Type             string `json:"type"`
	Qty              string `json:"qty"`
	Notional         string `json:"notional,omitempty"`
	TimeInForce      string `json:"time_in_force"`
	LimitPrice       string `json:"limit_price,omitempty"`
	StopPrice        string `json:"stop_price,omitempty"`
	Trail            string `json:"trail_price,omitempty"`
	ExtendedHours    bool   `json:"extended_hours,omitempty"`
}

type OrderResponse struct {
	ID                string    `json:"id"`
	ClientOrderID     string    `json:"client_order_id"`
	Status            string    `json:"status"`
	Symbol            string    `json:"symbol"`
	Exchange          string    `json:"exchange"`
	AssetClass       string    `json:"asset_class"`
	AssetID          string    `json:"asset_id"`
	Qty               string    `json:"qty"`
	FilledQty         string    `json:"filled_qty"`
	Side              string    `json:"side"`
	Type              string    `json:"type"`
	LimitPrice       string    `json:"limit_price"`
	StopPrice        string    `json:"stop_price"`
	FilledAvgPrice   string    `json:"filled_avg_price"`
	Trail            string    `json:"trail"`
	TimeInForce       string    `json:"time_in_force"`
	ExtendedHours    bool      `json:"extended_hours"`
	OrderClass       string    `json:"order_class"`
	Replaceable      bool      `json:"replaceable"`
	ReplacedBy       string    `json:"replaced_by"`
	ReplacedByOrder  string    `json:"replaced_by_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	SubmittedAt      time.Time `json:"submitted_at"`
	FilledAt         time.Time `json:"filled_at"`
	ExpiredAt        time.Time `json:"expired_at"`
	CanceledAt       time.Time `json:"canceled_at"`
}

type PositionResponse struct {
	Symbol            string `json:"symbol"`
	Exchange          string `json:"exchange"`
	AssetClass       string `json:"asset_class"`
	AssetID          string `json:"asset_id"`
	Qty               string `json:"qty"`
	AvgEntryPrice    string `json:"avg_entry_price"`
	Side              string `json:"side"`
	MarketValue      string `json:"market_value"`
	CostBasis        string `json:"cost_basis"`
	UnrealizedPL     string `json:"unrealized_pl"`
	UnrealizedPLPC  string `json:"unrealized_plpc"`
	CurrentPrice     string `json:"current_price"`
	LastTradePrice   string `json:"lasttrade_price"`
	ChangeToday      string `json:"change_today"`
}

type AssetResponse struct {
	ID              string `json:"id"`
	Symbol          string `json:"symbol"`
	Exchange        string `json:"exchange"`
	AssetClass      string `json:"asset_class"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	Tradable        bool   `json:"tradable"`
	Marginable      bool   `json:"marginable"`
	Shortable       bool   `json:"shortable"`
	EasyToBorrow    bool   `json:"easy_to_borrow"`
	Fractionable    bool   `json:"fractionable"`
	MaintenanceMargin string `json:"maintenance_margin_requirement"`
}

type BarResponse struct {
	Symbol      string    `json:"symbol"`
	Exchange    string    `json:"exchange"`
	Open        float64   `json:"o"`
	High        float64   `json:"h"`
	Low         float64   `json:"l"`
	Close       float64   `json:"c"`
	Volume      int64     `json:"v"`
	Timestamp   time.Time `json:"t"`
	TradeCount  int       `json:"n"`
	VWAP        float64   `json:"vwap"`
}

type QuoteResponse struct {
	Symbol      string    `json:"S"`
	BidExchange string    `json:"bx"`
	BidPrice    float64   `json:"bp"`
	BidSize     float64   `json:"bs"`
	AskExchange string    `json:"ax"`
	AskPrice    float64   `json:"ap"`
	AskSize     float64   `json:"as"`
	Timestamp   time.Time `json:"t"`
}

type TradeResponse struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Exchange    string    `json:"exchange"`
	Price       float64   `json:"p"`
	Size        int       `json:"s"`
	Timestamp   time.Time `json:"t"`
	Conditions  []string  `json:"c"`
}

type ClockResponse struct {
	Timestamp     time.Time `json:"timestamp"`
	IsOpen        bool      `json:"is_open"`
	NextOpen      time.Time `json:"next_open"`
	NextClose     time.Time `json:"next_close"`
}

type CalendarResponse struct {
	Date      string `json:"date"`
	Open      string `json:"open"`
	Close     string `json:"close"`
}

type WatchlistResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Symbols   []string `json:"symbols"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewClient(cfg *Config) (*Client, error) {
	baseURL := "https://api.alpaca.markets"
	dataURL := "https://data.alpaca.markets"

	if cfg.Paper {
		baseURL = "https://paper-api.alpaca.markets"
		dataURL = "https://paper-data.alpaca.markets"
	}

	client := &Client{
		apiKey:      cfg.APIKey,
		apiSecret:   cfg.APISecret,
		baseURL:     baseURL,
		dataURL:     dataURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		rateLimiter: &RateLimiter{minInterval: 100 * time.Millisecond},
	}

	return client, nil
}

func (c *Client) GetAccount(ctx context.Context) (*types.Account, error) {
	resp, err := c.doRequest(ctx, "GET", "/v2/account", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var accountResp AccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&accountResp); err != nil {
		return nil, err
	}

	account := &types.Account{
		AccountID:       accountResp.ID,
		Exchange:        "alpaca",
		Currency:        accountResp.Currency,
		BuyingPower:     decimal.RequireFromString(accountResp.BuyingPower),
		Cash:            decimal.RequireFromString(accountResp.Cash),
		PortfolioValue:  decimal.RequireFromString(accountResp.PortfolioValue),
		PatternDayTrader: accountResp.PatternDayTrader,
		TradingEnabled:  !accountResp.TradingBlocked && !accountResp.AccountBlocked,
	}

	c.accountMu.Lock()
	c.account = account
	c.accountMu.Unlock()

	return account, nil
}

func (c *Client) PlaceOrder(ctx context.Context, req *types.Order) (*types.Order, error) {
	orderReq := OrderRequest{
		Symbol:        req.Symbol,
		Side:         strings.ToLower(string(req.Side)),
		Type:         strings.ToLower(string(req.Type)),
		Qty:          req.Quantity.String(),
		TimeInForce:  string(req.TimeInForce),
	}

	if !req.Price.IsZero() {
		orderReq.LimitPrice = req.Price.String()
	}
	if !req.StopPrice.IsZero() {
		orderReq.StopPrice = req.StopPrice.String()
	}

	body, err := json.Marshal(orderReq)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, "POST", "/v2/orders", bytes.NewReader(body), true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var orderResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, err
	}

	return c.parseOrder(&orderResp), nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/v2/orders/%s", orderID), nil, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cancel order: %s", string(body))
	}

	return nil
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (*types.Order, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/v2/orders/%s", orderID), nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var orderResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, err
	}

	return c.parseOrder(&orderResp), nil
}

func (c *Client) ListOrders(ctx context.Context, status string, limit int) ([]*types.Order, error) {
	params := url.Values{}
	params.Set("status", status)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	resp, err := c.doRequest(ctx, "GET", "/v2/orders?"+params.Encode(), nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ordersResp []OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&ordersResp); err != nil {
		return nil, err
	}

	orders := make([]*types.Order, len(ordersResp))
	for i, o := range ordersResp {
		orders[i] = c.parseOrder(&o)
	}

	return orders, nil
}

func (c *Client) GetPosition(ctx context.Context, symbol string) (*types.Position, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/v2/positions/%s", symbol), nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var posResp PositionResponse
	if err := json.NewDecoder(resp.Body).Decode(&posResp); err != nil {
		return nil, err
	}

	return c.parsePosition(&posResp), nil
}

func (c *Client) GetAllPositions(ctx context.Context) ([]*types.Position, error) {
	resp, err := c.doRequest(ctx, "GET", "/v2/positions", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var positionsResp []PositionResponse
	if err := json.NewDecoder(resp.Body).Decode(&positionsResp); err != nil {
		return nil, err
	}

	positions := make([]*types.Position, len(positionsResp))
	for i, p := range positionsResp {
		positions[i] = c.parsePosition(&p)
	}

	return positions, nil
}

func (c *Client) ClosePosition(ctx context.Context, symbol string) error {
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/v2/positions/%s", symbol), nil, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to close position: %s", string(body))
	}

	return nil
}

func (c *Client) CloseAllPositions(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "DELETE", "/v2/positions", nil, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to close all positions: %s", string(body))
	}

	return nil
}

func (c *Client) GetAsset(ctx context.Context, symbol string) (*types.Asset, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/v2/assets/%s", symbol), nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var assetResp AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&assetResp); err != nil {
		return nil, err
	}

	return &types.Asset{
		ID:           assetResp.ID,
		Symbol:       assetResp.Symbol,
		Exchange:     assetResp.Exchange,
		AssetClass:   assetResp.AssetClass,
		Name:         assetResp.Name,
		Status:       assetResp.Status,
		Tradable:     assetResp.Tradable,
		Marginable:   assetResp.Marginable,
		Shortable:    assetResp.Shortable,
		EasyToBorrow: assetResp.EasyToBorrow,
	}, nil
}

func (c *Client) GetBars(ctx context.Context, symbol string, timeframe string, start, end time.Time, limit int) ([]*types.Bar, error) {
	params := url.Values{}
	params.Set("symbols", symbol)
	params.Set("timeframe", timeframe)
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", end.Format(time.RFC3339))
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	resp, err := c.doRequestData(ctx, "GET", "/v2/stocks/bars?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string][]BarResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var bars []*types.Bar
	for _, barResp := range result[symbol] {
		bars = append(bars, &types.Bar{
			Symbol:    symbol,
			Exchange:  "alpaca",
			Open:      decimal.NewFromFloat(barResp.Open),
			High:      decimal.NewFromFloat(barResp.High),
			Low:       decimal.NewFromFloat(barResp.Low),
			Close:     decimal.NewFromFloat(barResp.Close),
			Volume:    decimal.NewFromInt(barResp.Volume),
			TradeCount: barResp.TradeCount,
			VWAP:      decimal.NewFromFloat(barResp.VWAP),
			Timestamp: barResp.Timestamp,
		})
	}

	return bars, nil
}

func (c *Client) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	params := url.Values{}
	params.Set("symbols", symbol)

	resp, err := c.doRequestData(ctx, "GET", "/v2/stocks/quotes?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string][]QuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	quotes, ok := result[symbol]
	if !ok || len(quotes) == 0 {
		return nil, fmt.Errorf("no quote found for %s", symbol)
	}

	q := quotes[0]
	return &types.Quote{
		Symbol:    symbol,
		Exchange:  "alpaca",
		Bid:       decimal.NewFromFloat(q.BidPrice),
		Ask:       decimal.NewFromFloat(q.AskPrice),
		BidSize:   decimal.NewFromFloat(q.BidSize),
		AskSize:   decimal.NewFromFloat(q.AskSize),
		Timestamp: q.Timestamp,
	}, nil
}

func (c *Client) GetTrade(ctx context.Context, symbol string) (*types.Trade, error) {
	params := url.Values{}
	params.Set("symbols", symbol)

	resp, err := c.doRequestData(ctx, "GET", "/v2/stocks/trades?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string][]TradeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	trades, ok := result[symbol]
	if !ok || len(trades) == 0 {
		return nil, fmt.Errorf("no trade found for %s", symbol)
	}

	t := trades[0]
	return &types.Trade{
		ID:        t.ID,
		Symbol:    symbol,
		Exchange:  "alpaca",
		Price:     decimal.NewFromFloat(t.Price),
		Size:      decimal.NewFromInt(int64(t.Size)),
		Side:      types.SideBuy,
		Timestamp: t.Timestamp,
	}, nil
}

func (c *Client) GetClock(ctx context.Context) (*types.MarketClock, error) {
	resp, err := c.doRequest(ctx, "GET", "/v2/clock", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var clockResp ClockResponse
	if err := json.NewDecoder(resp.Body).Decode(&clockResp); err != nil {
		return nil, err
	}

	return &types.MarketClock{
		Timestamp: clockResp.Timestamp,
		IsOpen:    clockResp.IsOpen,
		NextOpen:  clockResp.NextOpen,
		NextClose: clockResp.NextClose,
	}, nil
}

func (c *Client) IsMarketOpen(ctx context.Context) (bool, error) {
	clock, err := c.GetClock(ctx)
	if err != nil {
		return false, err
	}
	return clock.IsOpen, nil
}

func (c *Client) GetCalendar(ctx context.Context, start, end time.Time) ([]*types.CalendarDay, error) {
	params := url.Values{}
	params.Set("start", start.Format("2006-01-02"))
	params.Set("end", end.Format("2006-01-02"))

	resp, err := c.doRequest(ctx, "GET", "/v2/calendar?"+params.Encode(), nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var calendarResp []CalendarResponse
	if err := json.NewDecoder(resp.Body).Decode(&calendarResp); err != nil {
		return nil, err
	}

	days := make([]*types.CalendarDay, len(calendarResp))
	for i, cal := range calendarResp {
		openTime, _ := time.Parse("15:04", cal.Open)
		closeTime, _ := time.Parse("15:04", cal.Close)

		days[i] = &types.CalendarDay{
			Date:     cal.Date,
			OpenTime: openTime,
			CloseTime: closeTime,
		}
	}

	return days, nil
}

func (c *Client) CreateWebSocket() error {
	c.wsConn = NewWebSocketClient(c.apiKey, c.apiKey, !strings.Contains(c.baseURL, "paper"))
	return c.wsConn.Connect()
}

func (c *Client) SubscribeQuotes(symbols []string, handler func(*types.Quote)) error {
	if c.wsConn == nil {
		if err := c.CreateWebSocket(); err != nil {
			return err
		}
	}
	return c.wsConn.SubscribeQuotes(symbols, handler)
}

func (c *Client) SubscribeTrades(symbols []string, handler func(*types.Trade)) error {
	if c.wsConn == nil {
		if err := c.CreateWebSocket(); err != nil {
			return err
		}
	}
	return c.wsConn.SubscribeTrades(symbols, handler)
}

func (c *Client) SubscribeBars(symbols []string, handler func(*types.Bar)) error {
	if c.wsConn == nil {
		if err := c.CreateWebSocket(); err != nil {
			return err
		}
	}
	return c.wsConn.SubscribeBars(symbols, handler)
}

func (c *Client) UnsubscribeQuotes(symbols []string) error {
	if c.wsConn == nil {
		return nil
	}
	return c.wsConn.UnsubscribeQuotes(symbols)
}

func (c *Client) UnsubscribeTrades(symbols []string) error {
	if c.wsConn == nil {
		return nil
	}
	return c.wsConn.UnsubscribeTrades(symbols)
}

func (c *Client) UnsubscribeBars(symbols []string) error {
	if c.wsConn == nil {
		return nil
	}
	return c.wsConn.UnsubscribeBars(symbols)
}

func (c *Client) Close() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, auth bool) (*http.Response, error) {
	c.rateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if auth {
		timestamp := time.Now().UnixNano()
		signature := c.generateSignature(method, path, timestamp, body)
		req.Header.Set("APCA-API-KEY-ID", c.apiKey)
		req.Header.Set("APCA-API-SECRET-KEY", c.apiSecret)
		req.Header.Set("APCA-API-TESTAMP", strconv.FormatInt(timestamp, 10))
		req.Header.Set("APCA-API-SIGNATURE", signature)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return resp, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func (c *Client) doRequestData(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	c.rateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, method, c.dataURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.apiSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return resp, fmt.Errorf("Data API error: %d - %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

func (c *Client) generateSignature(method, path string, timestamp int64, body io.Reader) string {
	var bodyStr string
	if body != nil {
		data, _ := io.ReadAll(body)
		bodyStr = string(data)
	}

	signString := fmt.Sprintf("%d%s%s%s", timestamp, method, path, bodyStr)
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(signString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) parseOrder(resp *OrderResponse) *types.Order {
	order := &types.Order{
		ID:            resp.ID,
		ClientOrderID: resp.ClientOrderID,
		Symbol:        resp.Symbol,
		Exchange:      "alpaca",
		Side:          types.Side(strings.ToUpper(resp.Side)),
		Type:          types.OrderType(strings.ToLower(resp.Type)),
		Quantity:      decimal.RequireFromString(resp.Qty),
		FilledQuantity: decimal.RequireFromString(resp.FilledQty),
		Price:         decimal.RequireFromString(resp.LimitPrice),
		StopPrice:     decimal.RequireFromString(resp.StopPrice),
		AvgFillPrice:  decimal.RequireFromString(resp.FilledAvgPrice),
		TimeInForce:   types.TimeInForce(resp.TimeInForce),
		Status:        c.parseOrderStatus(resp.Status),
		CreatedAt:     resp.CreatedAt,
		UpdatedAt:     resp.UpdatedAt,
	}

	if !resp.FilledAt.IsZero() {
		order.FilledAt = resp.FilledAt
	}
	if !resp.ExpiredAt.IsZero() {
		order.ExpiredAt = resp.ExpiredAt
	}
	if !resp.CanceledAt.IsZero() {
		order.CanceledAt = resp.CanceledAt
	}

	return order
}

func (c *Client) parseOrderStatus(status string) types.OrderStatus {
	switch strings.ToLower(status) {
	case "pending_new":
		return types.OrderStatusPending
	case "new":
		return types.OrderStatusOpen
	case "partially_filled":
		return types.OrderStatusPartiallyFilled
	case "filled":
		return types.OrderStatusFilled
	case "done_for_day":
		return types.OrderStatusDoneForDay
	case "canceled":
		return types.OrderStatusCanceled
	case "expired":
		return types.OrderStatusExpired
	case "replaced":
		return types.OrderStatusReplaced
	case "pending_cancel":
		return types.OrderStatusPendingCancel
	case "pending_replace":
		return types.OrderStatusPendingReplace
	case "accepted":
		return types.OrderStatusAccepted
	case "pending_new":
		return types.OrderStatusPending
	default:
		return types.OrderStatusUnknown
	}
}

func (c *Client) parsePosition(resp *PositionResponse) *types.Position {
	return &types.Position{
		Symbol:         resp.Symbol,
		Exchange:       "alpaca",
		AssetClass:     resp.AssetClass,
		Quantity:       decimal.RequireFromString(resp.Qty),
		AvgEntryPrice:  decimal.RequireFromString(resp.AvgEntryPrice),
		CurrentPrice:   decimal.RequireFromString(resp.CurrentPrice),
		MarketValue:    decimal.RequireFromString(resp.MarketValue),
		CostBasis:      decimal.RequireFromString(resp.CostBasis),
		UnrealizedPnL:  decimal.RequireFromString(resp.UnrealizedPL),
		RealizedPnL:    decimal.Zero,
		Side:           types.Side(strings.ToLower(resp.Side)),
		AssetID:        resp.AssetID,
	}
}

func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRequestTime)
	if elapsed < r.minInterval {
		time.Sleep(r.minInterval - elapsed)
	}
	r.lastRequestTime = time.Now()
}

type WebSocketClient struct {
	apiKey    string
	apiSecret string
	baseURL   string
	conn      *Conn
	mu        sync.RWMutex
}

func NewWebSocketClient(apiKey, apiSecret string, live bool) *WebSocketClient {
	url := "wss://paper-api.alpaca.markets/stream"
	if live {
		url = "wss://api.alpaca.markets/stream"
	}
	return &WebSocketClient{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   url,
	}
}

func (w *WebSocketClient) Connect() error {
	conn, err := Dial(w.baseURL)
	if err != nil {
		return err
	}
	w.conn = conn

	if err := w.authenticate(); err != nil {
		return err
	}

	go w.readLoop()
	return nil
}

func (w *WebSocketClient) authenticate() error {
	msg := map[string]interface{}{
		"action": "auth",
		"key":    w.apiKey,
		"secret": w.apiSecret,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) SubscribeQuotes(symbols []string, handler func(*types.Quote)) error {
	msg := map[string]interface{}{
		"action":  "subscribe",
		"quotes": symbols,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) SubscribeTrades(symbols []string, handler func(*types.Trade)) error {
	msg := map[string]interface{}{
		"action":  "subscribe",
		"trades": symbols,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) SubscribeBars(symbols []string, handler func(*types.Bar)) error {
	msg := map[string]interface{}{
		"action": "subscribe",
		"bars":  symbols,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) UnsubscribeQuotes(symbols []string) error {
	msg := map[string]interface{}{
		"action":  "unsubscribe",
		"quotes": symbols,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) UnsubscribeTrades(symbols []string) error {
	msg := map[string]interface{}{
		"action":  "unsubscribe",
		"trades": symbols,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) UnsubscribeBars(symbols []string) error {
	msg := map[string]interface{}{
		"action": "unsubscribe",
		"bars":  symbols,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) readLoop() {
	for {
		var msg map[string]interface{}
		if err := w.conn.ReadJSON(&msg); err != nil {
			logger.Error("WebSocket read error: %v", err)
			break
		}
	}
}

func (w *WebSocketClient) Close() error {
	if w.conn != nil {
		return w.conn.Close()
	}
	return nil
}

type Conn struct {
	conn   *websocket.Conn
	mu     sync.Mutex
}

func Dial(url string) (*Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &Conn{conn: conn}, nil
}

func (c *Conn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *Conn) ReadJSON(v interface{}) error {
	return c.conn.ReadJSON(v)
}

func (c *Conn) Close() error {
	return c.conn.Close()
}
