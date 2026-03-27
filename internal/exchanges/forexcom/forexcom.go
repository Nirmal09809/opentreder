package forexcom

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
	apiKey       string
	apiSecret    string
	accountID    string
	baseURL      string
	streamURL    string
	httpClient   *http.Client
	wsConn       *WebSocketClient
	authToken    string
	tokenExpiry  time.Time
	account      *types.Account
	accountMu    sync.RWMutex
	rateLimiter  *RateLimiter
}

type Config struct {
	APIKey     string
	APISecret  string
	AccountID  string
	Domain     string
}

type RateLimiter struct {
	mu              sync.Mutex
	lastRequestTime time.Time
	minInterval     time.Duration
}

type AuthResponse struct {
	AuthToken     string `json:"authToken"`
	TokenValidity string `json:"tokenValidity"`
}

type AccountResponse struct {
	AccountID            string `json:"accountId"`
	AccountName         string `json:"accountName"`
	AccountType         string `json:"accountType"`
	Balance             BalanceInfo `json:"balance"`
	MarginAvailable     decimal.Decimal `json:"marginAvailable"`
	MarginUsed          decimal.Decimal `json:"marginUsed"`
	UnrealizedPL        decimal.Decimal `json:"unrealizedPl"`
	RealizedPL          decimal.Decimal `json:"realizedPl"`
	Currency            string `json:"currency"`
	BalanceAvailable    decimal.Decimal `json:"balanceAvailable"`
}

type BalanceInfo struct {
	Equity         decimal.Decimal `json:"equity"`
	UsedMargin     decimal.Decimal `json:"usedMargin"`
	UsableMargin   decimal.Decimal `json:"usableMargin"`
	UsableMargin4  decimal.Decimal `json:"usableMargin4"`
}

type TradeResponse struct {
	TradeID       string `json:"tradeId"`
	AccountID     string `json:"accountId"`
	ExternalAccountID string `json:"externalAccountId"`
	Broker        string `json:"broker"`
	Currency      string `json:"currency"`
	MarketName    string `json:"marketName"`
	Direction     string `json:"direction"`
	Quantity      decimal.Decimal `json:"quantity"`
	OriginQuantity decimal.Decimal `json:"originQuantity"`
	OpenPrice     decimal.Decimal `json:"openPrice"`
	OpenTimestamp time.Time `json:"openTimestamp"`
	PlN           decimal.Decimal `json:"plN"`
	PlP           decimal.Decimal `json:"plP"`
	CostPrice     decimal.Decimal `json:"costPrice"`
	UsedMargin    decimal.Decimal `json:"usedMargin"`
	OpenTradeID   string `json:"openTrades"`
}

type OrderResponse struct {
	OrderId       string `json:"orderId"`
	AccountID     string `json:"accountId"`
	ExternalAccountID string `json:"externalAccountId"`
	MarketName    string `json:"marketName"`
	Direction     string `json:"direction"`
	Quantity      decimal.Decimal `json:"quantity"`
	OrderType     string `json:"type"`
	OrderFillType string `json:"orderFillType"`
	Status        string `json:"status"`
	MarketBid     decimal.Decimal `json:"marketBid"`
	MarketAsk     decimal.Decimal `json:"marketAsk"`
	PriceDistance decimal.Decimal `json:"priceDistance"`
	TimeInForce   string `json:"timeInForce"`
	ExpTime       time.Time `json:"expTime"`
	Created       time.Time `json:"created"`
	Modified      time.Time `json:"modified"`
	FilledQty     decimal.Decimal `json:"filledQty"`
	AvgFillPrice  decimal.Decimal `json:"avgFillPrice"`
	ClosedQty     decimal.Decimal `json:"closedQty"`
}

type PriceResponse struct {
	MarketStatus      string `json:"marketStatus"`
	MarketName        string `json:"marketName"`
	Offer             decimal.Decimal `json:"offer"`
	Bid               decimal.Decimal `json:"bid"`
	PriceChange       decimal.Decimal `json:"priceChange"`
	PriceChangePct    decimal.Decimal `json:"priceChangePct"`
	High              decimal.Decimal `json:"high"`
	Low               decimal.Decimal `json:"low"`
	TradePrice       decimal.Decimal `json:"tradePrice"`
	Volume            decimal.Decimal `json:"volume"`
	Time             time.Time `json:"time"`
}

type CandleResponse struct {
	MarketName    string  `json:"marketName"`
	Timestamp     time.Time `json:"timestamp"`
	Open          decimal.Decimal `json:"open"`
	High          decimal.Decimal `json:"high"`
	Low           decimal.Decimal `json:"low"`
	Close         decimal.Decimal `json:"close"`
	Volume        decimal.Decimal `json:"volume"`
	Complete      bool   `json:"complete"`
}

type TradeRequest struct {
	Market      string `json:"market"`
	Direction   string `json:"direction"`
	Quantity    string `json:"quantity"`
	OrderType   string `json:"orderType"`
	TimeInForce string `json:"timeInForce"`
	Price       string `json:"price,omitempty"`
	StopLoss    string `json:"stopLoss,omitempty"`
	TakeProfit  string `json:"takeProfit,omitempty"`
}

type OrderRequest struct {
	Market      string `json:"market"`
	Direction   string `json:"direction"`
	Quantity    string `json:"quantity"`
	OrderType   string `json:"type"`
	TimeInForce string `json:"timeInForce"`
	Price       string `json:"price,omitempty"`
	StopPrice   string `json:"stopPrice,omitempty"`
	ExpTime     string `json:"expTime,omitempty"`
}

func NewClient(cfg *Config) (*Client, error) {
	baseURL := "https://api.forex.com/api/v1"
	streamURL := "wss://streaming.forex.com"

	if cfg.Domain == "demo" {
		baseURL = "https://demo.forex.com/api/v1"
		streamURL = "wss://demo-streaming.forex.com"
	}

	client := &Client{
		apiKey:       cfg.APIKey,
		apiSecret:    cfg.APISecret,
		accountID:    cfg.AccountID,
		baseURL:      baseURL,
		streamURL:    streamURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		rateLimiter:  &RateLimiter{minInterval: 100 * time.Millisecond},
	}

	return client, nil
}

func (c *Client) Authenticate(ctx context.Context) error {
	signature := c.generateSignature()

	body, _ := json.Marshal(map[string]string{
		"apiKey":    c.apiKey,
		"signature": signature,
	})

	resp, err := c.doRequest(ctx, "POST", "/authentications", bytes.NewReader(body), false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return err
	}

	c.authToken = authResp.AuthToken

	validity, _ := time.ParseDuration("1h")
	c.tokenExpiry = time.Now().Add(validity)

	logger.Info("Forex.com authentication successful")
	return nil
}

func (c *Client) generateSignature() string {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	message := timestamp + c.apiKey

	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) GetAccount(ctx context.Context) (*types.Account, error) {
	if c.authToken == "" || time.Now().After(c.tokenExpiry) {
		if err := c.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/accounts/%s", c.accountID), nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var accountResp AccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&accountResp); err != nil {
		return nil, err
	}

	account := &types.Account{
		AccountID:       accountResp.AccountID,
		Exchange:        "forexcom",
		Currency:        accountResp.Currency,
		BuyingPower:     accountResp.MarginAvailable,
		Cash:            accountResp.Balance.Equity,
		PortfolioValue:  accountResp.Balance.Equity,
		TradingEnabled:  true,
	}

	c.accountMu.Lock()
	c.account = account
	c.accountMu.Unlock()

	return account, nil
}

func (c *Client) PlaceOrder(ctx context.Context, req *types.Order) (*types.Order, error) {
	if c.authToken == "" || time.Now().After(c.tokenExpiry) {
		if err := c.Authenticate(ctx); err != nil {
			return nil, err
		}
	}

	direction := "BUY"
	if req.Side == types.OrderSideSell {
		direction = "SELL"
	}

	orderType := c.mapOrderType(req.Type)
	timeInForce := c.mapTimeInForce(req.TimeInForce)

	orderReq := OrderRequest{
		Market:      req.Symbol,
		Direction:   direction,
		Quantity:    req.Quantity.String(),
		OrderType:   orderType,
		TimeInForce: timeInForce,
	}

	if !req.Price.IsZero() {
		orderReq.Price = req.Price.String()
	}
	if !req.StopPrice.IsZero() {
		orderReq.StopPrice = req.StopPrice.String()
	}

	body, _ := json.Marshal(orderReq)
	resp, err := c.doRequest(ctx, "POST", "/orders", bytes.NewReader(body), true)
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
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/orders/%s", orderID), nil, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cancel order: %s", string(body))
	}

	return nil
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (*types.Order, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/orders/%s", orderID), nil, true)
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
	path := "/orders"
	if status != "" {
		path += "?status=" + status
	}

	resp, err := c.doRequest(ctx, "GET", path, nil, true)
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
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/openpositions/%s", symbol), nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var posResp TradeResponse
	if err := json.NewDecoder(resp.Body).Decode(&posResp); err != nil {
		return nil, err
	}

	return c.parsePosition(&posResp), nil
}

func (c *Client) GetAllPositions(ctx context.Context) ([]*types.Position, error) {
	resp, err := c.doRequest(ctx, "GET", "/openpositions", nil, true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var positionsResp []TradeResponse
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
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/openpositions/%s", symbol), nil, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to close position: %s", string(body))
	}

	return nil
}

func (c *Client) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/prices/%s", symbol), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var priceResp PriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return nil, err
	}

	return &types.Quote{
		Symbol:    symbol,
		Exchange:  "forexcom",
		BidPrice:  priceResp.Bid,
		AskPrice:  priceResp.Offer,
		Timestamp: priceResp.Time,
	}, nil
}

func (c *Client) GetBars(ctx context.Context, symbol string, timeframe string, start, end time.Time, limit int) ([]*types.Bar, error) {
	params := url.Values{}
	params.Set(" granularity", timeframe)
	params.Set("startTime", start.Format(time.RFC3339))
	params.Set("endTime", end.Format(time.RFC3339))
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/candles/%s?%s", symbol, params.Encode()), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var candlesResp []CandleResponse
	if err := json.NewDecoder(resp.Body).Decode(&candlesResp); err != nil {
		return nil, err
	}

	bars := make([]*types.Bar, len(candlesResp))
	for i, c := range candlesResp {
		bars[i] = &types.Bar{
			Symbol:    symbol,
			Exchange:  "forexcom",
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			StartTime: c.Timestamp,
			EndTime:   c.Timestamp.Add(time.Minute),
		}
	}

	return bars, nil
}

func (c *Client) GetCandles(ctx context.Context, symbol string, timeframe types.Timeframe, limit int) ([]*types.Candle, error) {
	interval := c.mapTimeframe(timeframe)

	start := time.Now().Add(-time.Duration(limit*int(timeframe.Duration())) * time.Second)
	end := time.Now()

	bars, err := c.GetBars(ctx, symbol, interval, start, end, limit)
	if err != nil {
		return nil, err
	}

	candles := make([]*types.Candle, len(bars))
	for i, bar := range bars {
		candles[i] = &types.Candle{
			Symbol:    bar.Symbol,
			Exchange:  bar.Exchange,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			Timeframe: string(timeframe),
			Timestamp: bar.StartTime,
		}
	}

	return candles, nil
}

func (c *Client) ConnectWebSocket(ctx context.Context) error {
	c.wsConn = NewWebSocketClient(c.authToken, c.baseURL)
	return c.wsConn.Connect(ctx)
}

func (c *Client) SubscribePrices(symbols []string, handler func(*types.Quote)) error {
	if c.wsConn == nil {
		return fmt.Errorf("WebSocket not connected")
	}
	return c.wsConn.SubscribePrices(symbols, handler)
}

func (c *Client) SubscribeCandles(symbol string, timeframe string, handler func(*types.Candle)) error {
	if c.wsConn == nil {
		return fmt.Errorf("WebSocket not connected")
	}
	return c.wsConn.SubscribeCandles(symbol, timeframe, handler)
}

func (c *Client) UnsubscribePrices(symbols []string) error {
	if c.wsConn == nil {
		return nil
	}
	return c.wsConn.UnsubscribePrices(symbols)
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

	if auth && c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	return resp, nil
}

func (c *Client) mapOrderType(orderType types.OrderType) string {
	switch orderType {
	case types.OrderTypeMarket:
		return "MARKET"
	case types.OrderTypeLimit:
		return "LIMIT"
	case types.OrderTypeStop:
		return "STOP"
	case types.OrderTypeStopLimit:
		return "STOP_LIMIT"
	default:
		return "MARKET"
	}
}

func (c *Client) mapTimeInForce(tif types.TimeInForce) string {
	switch tif {
	case types.TimeInForceGTC:
		return "GTC"
	case types.TimeInForceIOC:
		return "IOC"
	case types.TimeInForceFOK:
		return "FOK"
	case types.TimeInForceGTX:
		return "GTX"
	case types.TimeInForceGTT:
		return "GTT"
	default:
		return "GTC"
	}
}

func (c *Client) mapTimeframe(tf types.Timeframe) string {
	switch tf {
	case types.Timeframe1m:
		return "M1"
	case types.Timeframe5m:
		return "M5"
	case types.Timeframe15m:
		return "M15"
	case types.Timeframe1h:
		return "H1"
	case types.Timeframe4h:
		return "H4"
	case types.Timeframe1d:
		return "D"
	default:
		return "H1"
	}
}

func (c *Client) parseOrder(resp *OrderResponse) *types.Order {
	side := types.OrderSideBuy
	if strings.ToUpper(resp.Direction) == "SELL" {
		side = types.OrderSideSell
	}

	orderType := types.OrderTypeMarket
	switch strings.ToUpper(resp.OrderType) {
	case "LIMIT":
		orderType = types.OrderTypeLimit
	case "STOP":
		orderType = types.OrderTypeStop
	case "STOP_LIMIT":
		orderType = types.OrderTypeStopLimit
	}

	orderID, _ := uuid.Parse(resp.OrderId)

	return &types.Order{
		ID:             orderID,
		Symbol:         resp.MarketName,
		Exchange:       "forexcom",
		Side:           side,
		Type:           orderType,
		Quantity:       resp.Quantity,
		FilledQuantity: resp.FilledQty,
		AvgFillPrice:   resp.AvgFillPrice,
		TimeInForce:    types.TimeInForce(resp.TimeInForce),
		Status:         c.parseOrderStatus(resp.Status),
		CreatedAt:      resp.Created,
		UpdatedAt:      resp.Modified,
	}
}

func (c *Client) parseOrderStatus(status string) types.OrderStatus {
	switch strings.ToUpper(status) {
	case "PENDING":
		return types.OrderStatusPending
	case "OPEN", "WORKING":
		return types.OrderStatusOpen
	case "FILLED":
		return types.OrderStatusFilled
	case "PARTIALLY_FILLED":
		return types.OrderStatusPartiallyFilled
	case "CANCELLED", "CANCELED":
		return types.OrderStatusCancelled
	case "EXPIRED":
		return types.OrderStatusExpired
	case "REJECTED":
		return types.OrderStatusRejected
	default:
		return types.OrderStatus(status)
	}
}

func (c *Client) parsePosition(resp *TradeResponse) *types.Position {
	side := types.PositionSideLong
	if strings.ToUpper(resp.Direction) == "SELL" {
		side = types.PositionSideShort
	}

	return &types.Position{
		Symbol:        resp.MarketName,
		Exchange:      "forexcom",
		Quantity:      resp.Quantity,
		AvgEntryPrice: resp.OpenPrice,
		UnrealizedPnL: resp.PlN,
		RealizedPnL:   decimal.Zero,
		Side:          side,
		OpenedAt:      resp.OpenTimestamp,
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
	authToken string
	baseURL  string
	conn     *Conn
	mu       sync.RWMutex
}

func NewWebSocketClient(authToken, baseURL string) *WebSocketClient {
	return &WebSocketClient{
		authToken: authToken,
		baseURL:  baseURL,
	}
}

func (w *WebSocketClient) Connect(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, w.baseURL, nil)
	if err != nil {
		return err
	}
	w.conn = &Conn{conn: conn}

	go w.readLoop()
	return nil
}

func (w *WebSocketClient) SubscribePrices(symbols []string, handler func(*types.Quote)) error {
	msg := map[string]interface{}{
		"type":    "subscribe",
		"data":    map[string]interface{}{"symbols": symbols},
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) SubscribeCandles(symbol string, timeframe string, handler func(*types.Candle)) error {
	msg := map[string]interface{}{
		"type": "subscribe",
		"data": map[string]interface{}{
			"symbols":    []string{symbol},
			"granularity": timeframe,
		},
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) UnsubscribePrices(symbols []string) error {
	msg := map[string]interface{}{
		"type":    "unsubscribe",
		"data":    map[string]interface{}{"symbols": symbols},
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
	conn *websocket.Conn
	mu   sync.Mutex
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
