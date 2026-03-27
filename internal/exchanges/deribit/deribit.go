package deribit

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Client struct {
	apiKey        string
	apiSecret     string
	baseURL       string
	wsURL         string
	httpClient    *http.Client
	wsClient      *websocket.Dialer
	accessToken   string
	refreshToken  string
	expiresAt     time.Time
	mu            sync.RWMutex
	testnet       bool
}

type Config struct {
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Testnet    bool   `json:"testnet"`
}

type Request struct {
	JsonRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

type Response struct {
	JsonRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorResponse  `json:"error,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type TickerResult struct {
	InstrumentName string `json:"instrument_name"`
	MarkPrice      float64 `json:"mark_price"`
	BidPrice       float64 `json:"best_bid_price"`
	AskPrice       float64 `json:"best_ask_price"`
	LastPrice      float64 `json:"last_price"`
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	Volume         float64 `json:"volume"`
	OpenInterest   float64 `json:"open_interest"`
	Timestamp      int64  `json:"timestamp"`
}

type OrderBookResult struct {
	InstrumentName string   `json:"instrument_name"`
	Bids           [][]any `json:"bids"`
	Asks           [][]any `json:"asks"`
	Timestamp      int64   `json:"timestamp"`
}

type CandleResult struct {
	Ticker       string    `json:"ticker"`
	Timestamp    int64    `json:"t"`
	Open         float64   `json:"o"`
	High         float64   `json:"h"`
	Low          float64   `json:"l"`
	Close        float64   `json:"c"`
	Volume       float64   `json:"v"`
}

type TradeResult struct {
	ID             int64   `json:"trade_seq"`
	InstrumentName string  `json:"instrument_name"`
	Direction      string  `json:"direction"`
	Price          float64 `json:"price"`
	Amount         float64 `json:"amount"`
	Timestamp      int64   `json:"timestamp"`
	TradeID        string  `json:"trade_id"`
}

type PositionResult struct {
	InstrumentName    string  `json:"instrument_name"`
	Direction         string  `json:"direction"`
	Size             float64 `json:"size"`
	AveragePrice     float64 `json:"average_price"`
	MarkPrice        float64 `json:"mark_price"`
	TotalProfitLoss  float64 `json:"total_profit_loss"`
	Leverage         float64 `json:"leverage"`
	Margin           float64 `json:"margin"`
}

type BalanceResult struct {
	Username         string  `json:"username"`
	Equity           float64 `json:"equity"`
	Balance          float64 `json:"balance"`
	MarginBalance    float64 `json:"margin_balance"`
	AvailableBalance float64 `json:"available_without_margin"`
	InitialMargin    float64 `json:"initial_margin"`
	MaintMargin      float64 `json:"maintenance_margin"`
}

type OrderResult struct {
	OrderID       int64   `json:"order_id"`
	InstrumentName string  `json:"instrument_name"`
	Direction     string  `json:"direction"`
	Price         float64 `json:"price"`
	Amount        float64 `json:"amount"`
	Label         string  `json:"label"`
	OrderType     string  `json:"order_type"`
	State         string  `json:"state"`
	FilledAmount  float64 `json:"filled_amount"`
	AveragePrice  float64 `json:"average_price"`
	CreationTS    int64   `json:"creation_timestamp"`
	ExpirationTS  int64   `json:"expiration_timestamp"`
}

func NewClient(cfg Config) *Client {
	baseURL := "https://www.deribit.com"
	wsURL := "wss://www.deribit.com/ws/api/v2"

	if cfg.Testnet {
		baseURL = "https://test.deribit.com"
		wsURL = "wss://test.deribit.com/ws/api/v2"
	}

	return &Client{
		apiKey:     cfg.APIKey,
		apiSecret:  cfg.APISecret,
		baseURL:    baseURL,
		wsURL:      wsURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		wsClient:   &websocket.Dialer{},
		testnet:    cfg.Testnet,
	}
}

func (c *Client) Authenticate(ctx context.Context) error {
	if c.apiKey == "" || c.apiSecret == "" {
		return nil
	}

	timestamp := time.Now().UnixMilli()
	nonce := generateNonce()
	signature := c.generateSignature("GET", "/api/v2/public/get_time", "", timestamp, nonce)

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "public/auth",
		"params": map[string]interface{}{
			"grant_type": "client_signature",
			"client_id": c.apiKey,
			"timestamp": timestamp,
			"nonce":     nonce,
			"signature": signature,
		},
		"id": 1,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if data, ok := result["result"].(map[string]interface{}); ok {
		if token, ok := data["access_token"].(string); ok {
			c.accessToken = token
		}
		if refresh, ok := data["refresh_token"].(string); ok {
			c.refreshToken = refresh
		}
	}

	return nil
}

func (c *Client) GetTicker(ctx context.Context, instrument string) (*types.Ticker, error) {
	c.mu.RLock()
	hasAuth := c.accessToken != ""
	c.mu.RUnlock()

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "public/get_ticker",
		"params": map[string]interface{}{
			"instrument_name": instrument,
		},
		"id": 1,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var ticker TickerResult
	if err := json.Unmarshal(result.Result, &ticker); err != nil {
		return nil, err
	}

	_ = hasAuth

	return &types.Ticker{
		Symbol:     ticker.InstrumentName,
		Exchange:   types.Exchange("deribit"),
		LastPrice: decimal.NewFromFloat(ticker.LastPrice),
		BidPrice:  decimal.NewFromFloat(ticker.BidPrice),
		AskPrice:  decimal.NewFromFloat(ticker.AskPrice),
		High24h:   decimal.NewFromFloat(ticker.High),
		Low24h:    decimal.NewFromFloat(ticker.Low),
		Volume24h: decimal.NewFromFloat(ticker.Volume),
		Timestamp: time.UnixMilli(ticker.Timestamp),
	}, nil
}

func (c *Client) GetOrderBook(ctx context.Context, instrument string, depth int) (*types.OrderBook, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "public/get_order_book",
		"params": map[string]interface{}{
			"instrument_name": instrument,
			"depth":          depth,
		},
		"id": 1,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var ob OrderBookResult
	if err := json.Unmarshal(result.Result, &ob); err != nil {
		return nil, err
	}

	bids := make([]types.PriceLevel, len(ob.Bids))
	for i, bid := range ob.Bids {
		if len(bid) >= 2 {
			price, _ := decimal.NewFromString(fmt.Sprintf("%v", bid[0]))
			qty, _ := decimal.NewFromString(fmt.Sprintf("%v", bid[1]))
			bids[i] = types.PriceLevel{Price: price, Quantity: qty}
		}
	}

	asks := make([]types.PriceLevel, len(ob.Asks))
	for i, ask := range ob.Asks {
		if len(ask) >= 2 {
			price, _ := decimal.NewFromString(fmt.Sprintf("%v", ask[0]))
			qty, _ := decimal.NewFromString(fmt.Sprintf("%v", ask[1]))
			asks[i] = types.PriceLevel{Price: price, Quantity: qty}
		}
	}

	return &types.OrderBook{
		Symbol:    ob.InstrumentName,
		Exchange:  types.Exchange("deribit"),
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.UnixMilli(ob.Timestamp),
	}, nil
}

func (c *Client) GetCandles(ctx context.Context, instrument, timeframe string, from, to int64, count int) ([]*types.Candle, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "public/get_tradingview_chart_data",
		"params": map[string]interface{}{
			"instrument_name": instrument,
			"start_timestamp": from,
			"end_timestamp":   to,
			"resolution":      timeframe,
		},
		"id": 1,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var candles []CandleResult
	if err := json.Unmarshal(result.Result, &candles); err != nil {
		return nil, err
	}

	typesCandles := make([]*types.Candle, len(candles))
	for i, c := range candles {
		typesCandles[i] = &types.Candle{
			Symbol:    instrument,
			Exchange:  types.Exchange("deribit"),
			Timeframe: timeframe,
			Timestamp: time.UnixMilli(c.Timestamp),
			Open:      decimal.NewFromFloat(c.Open),
			High:      decimal.NewFromFloat(c.High),
			Low:       decimal.NewFromFloat(c.Low),
			Close:     decimal.NewFromFloat(c.Close),
			Volume:    decimal.NewFromFloat(c.Volume),
		}
	}

	return typesCandles, nil
}

func (c *Client) GetTrades(ctx context.Context, instrument string, count int) ([]*types.Trade, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "public/get_last_trades_by_instrument",
		"params": map[string]interface{}{
			"instrument_name": instrument,
			"count":           count,
		},
		"id": 1,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var trades []TradeResult
	if err := json.Unmarshal(result.Result, &trades); err != nil {
		return nil, err
	}

	typesTrades := make([]*types.Trade, len(trades))
	for i, t := range trades {
		side := types.OrderSideBuy
		if t.Direction == "sell" {
			side = types.OrderSideSell
		}

		typesTrades[i] = &types.Trade{
			Symbol:    t.InstrumentName,
			Exchange:  types.Exchange("deribit"),
			Side:      side,
			Price:     decimal.NewFromFloat(t.Price),
			Quantity:  decimal.NewFromFloat(t.Amount),
			Timestamp: time.UnixMilli(t.Timestamp),
		}
	}

	return typesTrades, nil
}

func (c *Client) GetBalance(ctx context.Context) (*BalanceResult, error) {
	c.mu.RLock()
	hasAuth := c.accessToken != ""
	c.mu.RUnlock()

	if !hasAuth {
		if err := c.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "private/get_account_summary",
		"params": map[string]interface{}{
			"currency": "BTC",
		},
		"id": 1,
	}

	resp, err := c.doAuthenticatedRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var balance BalanceResult
	if err := json.Unmarshal(result.Result, &balance); err != nil {
		return nil, err
	}

	return &balance, nil
}

func (c *Client) GetPositions(ctx context.Context, instrument string) ([]*types.Position, error) {
	c.mu.RLock()
	hasAuth := c.accessToken != ""
	c.mu.RUnlock()

	if !hasAuth {
		if err := c.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "private/get_positions",
		"params": map[string]interface{}{
			"currency": "BTC",
		},
		"id": 1,
	}

	resp, err := c.doAuthenticatedRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var positions []PositionResult
	if err := json.Unmarshal(result.Result, &positions); err != nil {
		return nil, err
	}

	typesPositions := make([]*types.Position, len(positions))
	for i, p := range positions {
		side := types.PositionSideLong
		if p.Direction == "sell" {
			side = types.PositionSideShort
		}

		typesPositions[i] = &types.Position{
			Symbol:         p.InstrumentName,
			Exchange:       types.Exchange("deribit"),
			Side:           side,
			Quantity:       decimal.NewFromFloat(p.Size),
			AvgEntryPrice:  decimal.NewFromFloat(p.AveragePrice),
			CurrentPrice:   decimal.NewFromFloat(p.MarkPrice),
			UnrealizedPnL:  decimal.NewFromFloat(p.TotalProfitLoss),
			Leverage:       decimal.NewFromFloat(p.Leverage),
		}
	}

	return typesPositions, nil
}

func (c *Client) PlaceOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	c.mu.RLock()
	hasAuth := c.accessToken != ""
	c.mu.RUnlock()

	if !hasAuth {
		if err := c.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	direction := "buy"
	if order.Side == types.OrderSideSell {
		direction = "sell"
	}

	orderType := "limit"
	if order.Type == types.OrderTypeMarket {
		orderType = "market"
	}

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "private/" + direction,
		"params": map[string]interface{}{
			"instrument_name": order.Symbol,
			"amount":          order.Quantity.String(),
			"type":            orderType,
			"price":           order.Price.String(),
		},
		"id": 1,
	}

	resp, err := c.doAuthenticatedRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var orderResult OrderResult
	if err := json.Unmarshal(result.Result, &orderResult); err != nil {
		return nil, err
	}

	order.ID = types.UUIDFromString(fmt.Sprintf("%d", orderResult.OrderID))
	order.Status = c.parseOrderState(orderResult.State)

	return order, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	c.mu.RLock()
	hasAuth := c.accessToken != ""
	c.mu.RUnlock()

	if !hasAuth {
		if err := c.Authenticate(ctx); err != nil {
			return err
		}
	}

	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "private/cancel",
		"params": map[string]interface{}{
			"order_id": orderID,
		},
		"id": 1,
	}

	_, err := c.doAuthenticatedRequest(ctx, body)
	return err
}

func (c *Client) GetInstruments(ctx context.Context, currency string, expired bool) ([]string, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "public/get_instruments",
		"params": map[string]interface{}{
			"currency":  currency,
			"expired": expired,
		},
		"id": 1,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result []struct {
			InstrumentName string `json:"instrument_name"`
		} `json:"result"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	instruments := make([]string, len(result.Result))
	for i, inst := range result.Result {
		instruments[i] = inst.InstrumentName
	}

	return instruments, nil
}

func (c *Client) GetCurrencies(ctx context.Context) ([]string, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "public/get_currencies",
		"id":      1,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result []struct {
			Currency string `json:"currency"`
		} `json:"result"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	currencies := make([]string, len(result.Result))
	for i, curr := range result.Result {
		currencies[i] = curr.Currency
	}

	return currencies, nil
}

func (c *Client) doRequest(ctx context.Context, body map[string]interface{}) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v2", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) doAuthenticatedRequest(ctx context.Context, body map[string]interface{}) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v2", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) generateSignature(method, endpoint, body string, timestamp int64, nonce string) string {
	data := fmt.Sprintf("%d%s%s%s", timestamp, method, endpoint, body)
	h := hmac.New(sha256.New, []byte(c.apiSecret))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (c *Client) parseOrderState(state string) types.OrderStatus {
	switch state {
	case "open":
		return types.OrderStatusOpen
	case "filled":
		return types.OrderStatusFilled
	case "cancelled":
		return types.OrderStatusCancelled
	case "rejected":
		return types.OrderStatusRejected
	default:
		return types.OrderStatusPending
	}
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func generatePrivateKey() (ed25519.PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	return priv, err
}

func init() {
	logger.Debug("Deribit exchange adapter initialized")
}
