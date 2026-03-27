package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	testnet       bool
	baseURL       string
	wsURL         string
	httpClient    *http.Client
	wsClient      *websocket.Dialer
	subscriptions map[string]chan<- types.Ticker
	handlers      map[string][]interface{}
	mu            sync.RWMutex
	account       *types.Account
	positions     map[string]*types.Position
	orders       map[string]*types.Order
	rateLimiter  *RateLimiter
}

type Config struct {
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Testnet    bool   `json:"testnet"`
	BaseURL    string `json:"base_url"`
	TestnetURL string `json:"testnet_url"`
}

type RateLimiter struct {
	mu             sync.Mutex
	lastRequest    time.Time
	minInterval    time.Duration
	requestCounts  map[string]int
	windowDuration time.Duration
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		minInterval:    time.Minute / time.Duration(requestsPerMinute),
		requestCounts:  make(map[string]int),
		windowDuration: time.Minute,
	}
}

func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.lastRequest) < r.minInterval {
		time.Sleep(r.minInterval - now.Sub(r.lastRequest))
	}
	r.lastRequest = time.Now()
}

func NewClient(cfg Config) *Client {
	baseURL := "https://api.binance.com"
	wsURL := "wss://stream.binance.com:9443/ws"

	if cfg.Testnet {
		baseURL = "https://testnet.binance.vision"
		wsURL = "wss://testnet.binance.vision/ws"
	} else if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}

	return &Client{
		apiKey:        cfg.APIKey,
		apiSecret:     cfg.APISecret,
		testnet:       cfg.Testnet,
		baseURL:       baseURL,
		wsURL:         wsURL,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		wsClient:      &websocket.Dialer{},
		subscriptions: make(map[string]chan<- types.Ticker),
		handlers:      make(map[string][]interface{}),
		positions:     make(map[string]*types.Position),
		orders:        make(map[string]*types.Order),
		rateLimiter:   NewRateLimiter(1200),
	}
}

// ---------------------------------------------------------------------------
// REST API Methods
// ---------------------------------------------------------------------------

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/api/v3/ping", nil, false)
	return err
}

func (c *Client) GetServerTime(ctx context.Context) (time.Time, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v3/time", nil, false)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	var result struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return time.Time{}, err
	}

	return time.UnixMilli(result.ServerTime), nil
}

func (c *Client) GetExchangeInfo(ctx context.Context) (*ExchangeInfo, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v3/exchangeInfo", nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info ExchangeInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

func (c *Client) GetAccount(ctx context.Context) (*types.Account, error) {
	data, err := c.signedRequest(ctx, "/api/v3/account", nil)
	if err != nil {
		return nil, err
	}

	var resp AccountResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	account := &types.Account{
		AccountID:      resp.AccountType,
		Exchange:       types.ExchangeBinance,
		TradingEnabled: true,
		Enabled:        true,
	}

	for _, balance := range resp.Balances {
		free, _ := decimal.NewFromString(balance.Free)
		locked, _ := decimal.NewFromString(balance.Locked)
		if free.Add(locked).IsPositive() {
			account.Balances[balance.Asset] = &types.Balance{
				Asset:  balance.Asset,
				Free:   free,
				Locked: locked,
				Total:  free.Add(locked),
			}
		}
	}

	c.mu.Lock()
	c.account = account
	c.mu.Unlock()

	return account, nil
}

func (c *Client) GetPositions(ctx context.Context) ([]*types.Position, error) {
	data, err := c.signedRequest(ctx, "/api/v3/account", nil)
	if err != nil {
		return nil, err
	}

	var resp AccountResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var positions []*types.Position
	for _, balance := range resp.Balances {
		free, _ := decimal.NewFromString(balance.Free)
		locked, _ := decimal.NewFromString(balance.Locked)
		quantity := free.Add(locked)

		if quantity.IsPositive() {
			pos := &types.Position{
				Symbol:      balance.Asset + "USDT",
				Exchange:    types.ExchangeBinance,
				Quantity:    quantity,
				Side:       types.PositionSideLong,
				OpenedAt:   time.Now(),
			}

			positions = append(positions, pos)
			c.mu.Lock()
			c.positions[pos.Symbol] = pos
			c.mu.Unlock()
		}
	}

	return positions, nil
}

// ---------------------------------------------------------------------------
// Order Management
// ---------------------------------------------------------------------------

func (c *Client) PlaceOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	params := map[string]string{
		"symbol":        order.Symbol,
		"side":          string(order.Side),
		"type":          string(order.Type),
		"quantity":      order.Quantity.String(),
		"timeInForce":   string(order.TimeInForce),
	}

	if !order.Price.IsZero() {
		params["price"] = order.Price.String()
	}
	if !order.StopPrice.IsZero() {
		params["stopPrice"] = order.StopPrice.String()
	}

	data, err := c.signedRequest(ctx, "/api/v3/order", params)
	if err != nil {
		return nil, err
	}

	var resp OrderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return c.parseOrder(&resp), nil
}

func (c *Client) CancelOrder(ctx context.Context, symbol, orderID string) error {
	params := map[string]string{
		"symbol":   symbol,
		"orderId": orderID,
	}

	_, err := c.signedRequest(ctx, "/api/v3/order", params)
	return err
}

func (c *Client) GetOrder(ctx context.Context, symbol, orderID string) (*types.Order, error) {
	params := map[string]string{
		"symbol": symbol,
		"orderId": orderID,
	}

	data, err := c.signedRequest(ctx, "/api/v3/order", params)
	if err != nil {
		return nil, err
	}

	var resp OrderResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	return c.parseOrder(&resp), nil
}

func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]*types.Order, error) {
	params := map[string]string{}
	if symbol != "" {
		params["symbol"] = symbol
	}

	data, err := c.signedRequest(ctx, "/api/v3/openOrders", params)
	if err != nil {
		return nil, err
	}

	var orders []OrderResponse
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}

	result := make([]*types.Order, len(orders))
	for i, o := range orders {
		result[i] = c.parseOrder(&o)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Market Data
// ---------------------------------------------------------------------------

func (c *Client) GetTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v3/ticker/24hr?symbol=%s", symbol), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ticker TickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&ticker); err != nil {
		return nil, err
	}

	return c.parseTicker(&ticker), nil
}

func (c *Client) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v3/ticker/bookTicker?symbol=%s", symbol), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var book BookTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return nil, err
	}

	bidPrice, _ := decimal.NewFromString(book.BidPrice)
	askPrice, _ := decimal.NewFromString(book.AskPrice)
	bidSize, _ := decimal.NewFromString(book.BidQty)
	askSize, _ := decimal.NewFromString(book.AskQty)

	return &types.Quote{
		Symbol:    symbol,
		Exchange:  types.ExchangeBinance,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		BidSize:   bidSize,
		AskSize:   askSize,
		Timestamp: time.Now(),
	}, nil
}

func (c *Client) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]*types.Candle, error) {
	resp, err := c.doRequest(ctx, "GET", 
		fmt.Sprintf("/api/v3/klines?symbol=%s&interval=%s&limit=%d", symbol, interval, limit), 
		nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rawKlines [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawKlines); err != nil {
		return nil, err
	}

	candles := make([]*types.Candle, len(rawKlines))
	for i, k := range rawKlines {
		openTime := int64(k[0].(float64))
		closeTime := int64(k[6].(float64))

		open, _ := decimal.NewFromString(k[1].(string))
		high, _ := decimal.NewFromString(k[2].(string))
		low, _ := decimal.NewFromString(k[3].(string))
		close, _ := decimal.NewFromString(k[4].(string))
		volume, _ := decimal.NewFromString(k[5].(string))

		candles[i] = &types.Candle{
			Symbol:    symbol,
			Exchange:  types.ExchangeBinance,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Timeframe: interval,
			Timestamp: time.UnixMilli(openTime),
			EndTime:   time.UnixMilli(closeTime),
		}
	}

	return candles, nil
}

func (c *Client) GetTrades(ctx context.Context, symbol string, limit int) ([]*types.Trade, error) {
	resp, err := c.doRequest(ctx, "GET", 
		fmt.Sprintf("/api/v3/trades?symbol=%s&limit=%d", symbol, limit), 
		nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rawTrades []TradeResponse
	if err := json.NewDecoder(resp.Body).Decode(&rawTrades); err != nil {
		return nil, err
	}

	trades := make([]*types.Trade, len(rawTrades))
	for i, t := range rawTrades {
		trades[i] = c.parseTrade(&t)
	}

	return trades, nil
}

func (c *Client) GetDepth(ctx context.Context, symbol string, limit int) (*types.OrderBook, error) {
	resp, err := c.doRequest(ctx, "GET", 
		fmt.Sprintf("/api/v3/depth?symbol=%s&limit=%d", symbol, limit), 
		nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var depth DepthResponse
	if err := json.NewDecoder(resp.Body).Decode(&depth); err != nil {
		return nil, err
	}

	bids := make([]types.PriceLevel, len(depth.Bids))
	for i, b := range depth.Bids {
		price, _ := decimal.NewFromString(b[0])
		qty, _ := decimal.NewFromString(b[1])
		bids[i] = types.PriceLevel{Price: price, Quantity: qty}
	}

	asks := make([]types.PriceLevel, len(depth.Asks))
	for i, a := range depth.Asks {
		price, _ := decimal.NewFromString(a[0])
		qty, _ := decimal.NewFromString(a[1])
		asks[i] = types.PriceLevel{Price: price, Quantity: qty}
	}

	return &types.OrderBook{
		Symbol:    symbol,
		Exchange:  types.ExchangeBinance,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}, nil
}

// ---------------------------------------------------------------------------
// WebSocket Streams
// ---------------------------------------------------------------------------

type WsHandler struct {
	onTicker   func(*types.Ticker)
	onTrade    func(*types.Trade)
	onKline    func(*types.Candle)
	onDepth    func(*types.OrderBook)
	onOrderBook func(*types.OrderBook)
}

func (c *Client) SubscribeTicker(symbol string, handler func(*types.Ticker)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream := strings.ToLower(symbol) + "@ticker"
	c.handlers[stream] = append(c.handlers[stream], handler)

	logger.Info("Subscribed to ticker", "symbol", symbol, "stream", stream)
	return nil
}

func (c *Client) SubscribeTrades(symbol string, handler func(*types.Trade)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream := strings.ToLower(symbol) + "@trade"
	c.handlers[stream] = append(c.handlers[stream], handler)

	logger.Info("Subscribed to trades", "symbol", symbol)
	return nil
}

func (c *Client) SubscribeKlines(symbol, interval string, handler func(*types.Candle)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream := fmt.Sprintf("%s@kline_%s", strings.ToLower(symbol), interval)
	c.handlers[stream] = append(c.handlers[stream], handler)

	logger.Info("Subscribed to klines", "symbol", symbol, "interval", interval)
	return nil
}

func (c *Client) SubscribeDepth(symbol string, handler func(*types.OrderBook)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream := strings.ToLower(symbol) + "@depth@100ms"
	c.handlers[stream] = append(c.handlers[stream], handler)

	logger.Info("Subscribed to depth", "symbol", symbol)
	return nil
}

func (c *Client) StartWebSocket(ctx context.Context) error {
	if len(c.handlers) == 0 {
		return fmt.Errorf("no subscriptions configured")
	}

	streams := make([]string, 0, len(c.handlers))
	for stream := range c.handlers {
		streams = append(streams, stream)
	}

	wsURL := fmt.Sprintf("%s/%s", c.wsURL, strings.Join(streams, "/"))
	
	conn, _, err := c.wsClient.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}
	defer conn.Close()

	logger.Info("WebSocket connected", "streams", len(streams))

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, msg, err := conn.ReadMessage()
				if err != nil {
					logger.Error("WebSocket read error", "error", err)
					return
				}
				c.handleWSMessage(msg)
			}
		}
	}()

	return nil
}

func (c *Client) handleWSMessage(msg []byte) {
	var base struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(msg, &base); err != nil {
		return
	}

	// Handle different stream types
	switch {
	case strings.Contains(base.Stream, "@ticker"):
		var ticker TickerStream
		if err := json.Unmarshal(base.Data, &ticker); err == nil {
			for _, handler := range c.handlers[base.Stream] {
				if h, ok := handler.(func(*types.Ticker)); ok {
					h(c.parseTickerStream(&ticker))
				}
			}
		}

	case strings.Contains(base.Stream, "@trade"):
		var trade TradeStream
		if err := json.Unmarshal(base.Data, &trade); err == nil {
			for _, handler := range c.handlers[base.Stream] {
				if h, ok := handler.(func(*types.Trade)); ok {
					h(c.parseTradeStream(&trade))
				}
			}
		}

	case strings.Contains(base.Stream, "@kline"):
		var kline KlineStream
		if err := json.Unmarshal(base.Data, &kline); err == nil {
			for _, handler := range c.handlers[base.Stream] {
				if h, ok := handler.(func(*types.Candle)); ok {
					h(c.parseKlineStream(&kline))
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helper Methods
// ---------------------------------------------------------------------------

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, signed bool) (*http.Response, error) {
	c.rateLimiter.Wait()

	var reqBody *strings.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OpenTrader/1.0")

	return c.httpClient.Do(req)
}

func (c *Client) signedRequest(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	c.rateLimiter.Wait()

	queryString := ""
	if params != nil {
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}
		queryString = values.Encode()
	}

	if queryString != "" {
		endpoint += "?" + queryString
	}

	signature := c.sign(queryString)

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint+"&signature="+signature, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) sign(queryString string) string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(queryString))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func (c *Client) parseOrder(o *OrderResponse) *types.Order {
	side := types.OrderSide(strings.ToUpper(o.Side))
	orderType := types.OrderType(strings.ToUpper(o.Type))
	status := c.parseStatus(o.Status)

	orderID := fmt.Sprintf("%d", o.OrderID)
	order := &types.Order{
		ID:              types.UUIDFromString(orderID),
		Symbol:          o.Symbol,
		Exchange:        types.ExchangeBinance,
		Side:           side,
		Type:           orderType,
		Quantity:       decimal.RequireFromString(o.OrigQty),
		FilledQuantity: decimal.RequireFromString(o.ExecutedQty),
		Price:          decimal.RequireFromString(o.Price),
		StopPrice:      decimal.RequireFromString(o.StopPrice),
		TimeInForce:    types.TimeInForce(o.TimeInForce),
		Status:         status,
		CreatedAt:      time.UnixMilli(o.Time),
		UpdatedAt:      time.UnixMilli(o.UpdateTime),
	}

	c.mu.Lock()
	c.orders[orderID] = order
	c.mu.Unlock()

	return order
}

func (c *Client) parseStatus(status string) types.OrderStatus {
	switch strings.ToUpper(status) {
	case "NEW":
		return types.OrderStatusOpen
	case "PARTIALLY_FILLED":
		return types.OrderStatusPartiallyFilled
	case "FILLED":
		return types.OrderStatusFilled
	case "CANCELED", "CANCELLED":
		return types.OrderStatusCancelled
	case "REJECTED":
		return types.OrderStatusRejected
	case "EXPIRED":
		return types.OrderStatusExpired
	default:
		return types.OrderStatus(status)
	}
}

func (c *Client) parseTicker(t *TickerResponse) *types.Ticker {
	lastPrice, _ := decimal.NewFromString(t.LastPrice)
	bidPrice, _ := decimal.NewFromString(t.BidPrice)
	askPrice, _ := decimal.NewFromString(t.AskPrice)
	volume, _ := decimal.NewFromString(t.Volume)
	quoteVolume, _ := decimal.NewFromString(t.QuoteVolume)
	high, _ := decimal.NewFromString(t.HighPrice)
	low, _ := decimal.NewFromString(t.LowPrice)
	change, _ := decimal.NewFromString(t.PriceChange)
	changePct, _ := decimal.NewFromString(t.PriceChangePercent)

	return &types.Ticker{
		Symbol:         t.Symbol,
		Exchange:       types.ExchangeBinance,
		LastPrice:      lastPrice,
		BidPrice:       bidPrice,
		AskPrice:       askPrice,
		Volume24h:      volume,
		QuoteVolume24h: quoteVolume,
		High24h:        high,
		Low24h:         low,
		PriceChange:    change,
		PriceChangePct: changePct,
		Timestamp:      time.Now(),
	}
}

func (c *Client) parseTrade(t *TradeResponse) *types.Trade {
	price, _ := decimal.NewFromString(t.Price)
	qty, _ := decimal.NewFromString(t.Qty)

	side := types.OrderSideBuy
	if t.IsBuyerMaker {
		side = types.OrderSideSell
	}

	return &types.Trade{
		ID:        types.UUIDFromString(strconv.FormatInt(t.ID, 10)),
		Symbol:    t.Symbol,
		Exchange:  types.ExchangeBinance,
		Price:     price,
		Quantity:  qty,
		Side:      side,
		Timestamp: time.UnixMilli(t.Time),
	}
}

func (c *Client) parseTickerStream(t *TickerStream) *types.Ticker {
	lastPrice, _ := decimal.NewFromString(t.LastPrice)
	bestBidPrice, _ := decimal.NewFromString(t.BestBidPrice)
	bestAskPrice, _ := decimal.NewFromString(t.BestAskPrice)
	volume, _ := decimal.NewFromString(t.Volume)
	quoteVolume, _ := decimal.NewFromString(t.QuoteVolume)

	return &types.Ticker{
		Symbol:         t.Symbol,
		Exchange:       types.ExchangeBinance,
		LastPrice:      lastPrice,
		BidPrice:       bestBidPrice,
		AskPrice:       bestAskPrice,
		Volume24h:      volume,
		QuoteVolume24h: quoteVolume,
		Timestamp:       time.UnixMilli(t.CloseTime),
	}
}

func (c *Client) parseTradeStream(t *TradeStream) *types.Trade {
	price, _ := decimal.NewFromString(t.Price)
	qty, _ := decimal.NewFromString(t.Qty)

	side := types.OrderSideBuy
	if t.IsBuyerMaker {
		side = types.OrderSideSell
	}

	return &types.Trade{
		ID:        types.UUIDFromString(strconv.FormatInt(t.TradeID, 10)),
		Symbol:    t.Symbol,
		Exchange:  types.ExchangeBinance,
		Price:     price,
		Quantity:  qty,
		Side:      side,
		Timestamp: time.UnixMilli(t.Time),
	}
}

func (c *Client) parseKlineStream(k *KlineStream) *types.Candle {
	kline := k.Kline

	open, _ := decimal.NewFromString(kline.Open)
	high, _ := decimal.NewFromString(kline.High)
	low, _ := decimal.NewFromString(kline.Low)
	close, _ := decimal.NewFromString(kline.Close)
	volume, _ := decimal.NewFromString(kline.Volume)

	return &types.Candle{
		Symbol:    k.Symbol,
		Exchange:  types.ExchangeBinance,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    volume,
		Timeframe: kline.Interval,
		Timestamp: time.UnixMilli(kline.StartTime),
		EndTime:   time.UnixMilli(kline.EndTime),
	}
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ch := range c.subscriptions {
		close(ch)
	}
	c.subscriptions = nil
	c.handlers = nil
}
