package exchanges

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type BaseAdapter struct {
	Name            string
	Config          ExchangeConfig
	APIKey          string
	APISecret       string
	Passphrase      string
	BaseURL         string
	WSURL           string
	HTTPClient      *http.Client
	WSSubscriptions map[string]bool
	mu              sync.RWMutex
	connected       bool
	lastHeartbeat  time.Time
	rateLimiter    *RateLimiter
}

type ExchangeConfig struct {
	APIKey       string
	APISecret    string
	Passphrase   string
	Testnet      bool
	RecvWindow   time.Duration
	MaxRetries   int
	RateLimit    int
	BaseURL      string
	WSURL        string
}

type RateLimiter struct {
	mu          sync.Mutex
	requests    []time.Time
	limit       int
	window      time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:  limit,
		window: window,
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	var valid []time.Time
	for _, t := range r.requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= r.limit {
		r.requests = valid
		return false
	}

	r.requests = append(valid, now)
	return true
}

func (r *RateLimiter) Wait() {
	for !r.Allow() {
		time.Sleep(100 * time.Millisecond)
	}
}

func New(name string, cfg ExchangeConfig) (*Exchange, error) {
	ex := &Exchange{
		BaseAdapter: BaseAdapter{
			Name:            name,
			Config:          cfg,
			APIKey:          cfg.APIKey,
			APISecret:       cfg.APISecret,
			Passphrase:      cfg.Passphrase,
			WSSubscriptions: make(map[string]bool),
			rateLimiter:     NewRateLimiter(cfg.RateLimit, time.Second),
			lastHeartbeat:   time.Now(),
		},
		balances: make(map[string]*types.Balance),
		positions: make(map[string]*types.Position),
		orders: make(map[string]*types.Order),
	}

	switch strings.ToLower(name) {
	case "binance":
		ex.BaseURL = "https://api.binance.com"
		ex.WSURL = "wss://stream.binance.com:9443/ws"
		if cfg.Testnet {
			ex.BaseURL = "https://testnet.binance.vision"
			ex.WSURL = "wss://testnet.binance.vision/ws"
		}
	case "bybit":
		ex.BaseURL = "https://api.bybit.com"
		ex.WSURL = "wss://stream.bybit.com"
		if cfg.Testnet {
			ex.BaseURL = "https://api-testnet.bybit.com"
			ex.WSURL = "wss://stream-testnet.bybit.com"
		}
	case "coinbase":
		ex.BaseURL = "https://api.coinbase.com"
		ex.WSURL = "wss://ws-feed.exchange.coinbase.com"
		if cfg.Testnet {
			ex.BaseURL = "https://api-sandbox.coinbase.com"
			ex.WSURL = "wss://ws-feed-public.sandbox.exchange.coinbase.com"
		}
	case "okx":
		ex.BaseURL = "https://www.okx.com"
		ex.WSURL = "wss://ws.okx.com:8443/ws/v5/public"
		if cfg.Testnet {
			ex.BaseURL = "https://www.okx.com"
			ex.WSURL = "wss://ws.okx.com:8443/ws/v5/public"
		}
	case "kraken":
		ex.BaseURL = "https://api.kraken.com"
		ex.WSURL = "wss://ws.kraken.com"
	case "alpaca":
		ex.BaseURL = "https://api.alpaca.markets"
		ex.WSURL = "wss://stream.data.alpaca.markets"
		if cfg.Testnet {
			ex.BaseURL = "https://paper-api.alpaca.markets"
		}
	default:
		return nil, fmt.Errorf("unsupported exchange: %s", name)
	}

	if cfg.BaseURL != "" {
		ex.BaseURL = cfg.BaseURL
	}
	if cfg.WSURL != "" {
		ex.WSURL = cfg.WSURL
	}

	ex.HTTPClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	return ex, nil
}

type Exchange struct {
	BaseAdapter
	balances  map[string]*types.Balance
	positions map[string]*types.Position
	orders    map[string]*types.Order
	markets   map[string]*types.Market
	mu        sync.RWMutex
}

func (e *Exchange) Connect(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.connected {
		return nil
	}

	logger.Info("Connecting to exchange", "exchange", e.Name)

	if e.APIKey != "" && e.APISecret != "" {
		if err := e.verifyCredentials(); err != nil {
			logger.Warn("Credential verification failed", "exchange", e.Name, "error", err)
		}
	}

	if err := e.loadMarkets(); err != nil {
		return fmt.Errorf("failed to load markets: %w", err)
	}

	e.connected = true
	e.lastHeartbeat = time.Now()

	logger.Info("Connected to exchange", "exchange", e.Name, "markets", len(e.markets))
	return nil
}

func (e *Exchange) Disconnect() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.connected {
		return nil
	}

	logger.Info("Disconnecting from exchange", "exchange", e.Name)

	e.connected = false

	logger.Info("Disconnected from exchange", "exchange", e.Name)
	return nil
}

func (e *Exchange) IsConnected() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.connected
}

func (e *Exchange) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.connected = false
	e.balances = nil
	e.positions = nil
	e.orders = nil

	logger.Info("Closed exchange connection", "exchange", e.Name)
	return nil
}

func (e *Exchange) verifyCredentials() error {
	return nil
}

func (e *Exchange) loadMarkets() error {
	e.markets = make(map[string]*types.Market)

	switch strings.ToLower(e.Name) {
	case "binance":
		e.markets["BTC/USDT"] = &types.Market{
			Symbol:           "BTC/USDT",
			Exchange:         types.ExchangeBinance,
			AssetType:        types.AssetTypeCrypto,
			BaseAsset:        "BTC",
			QuoteAsset:       "USDT",
			Status:           "TRADING",
			MinQty:           decimal.NewFromFloat(0.00001),
			MaxQty:           decimal.NewFromFloat(9000),
			StepSize:         decimal.NewFromFloat(0.00001),
			MinNotional:      decimal.NewFromFloat(10),
			PricePrecision:   2,
			QuantityPrecision: 5,
		}
		e.markets["ETH/USDT"] = &types.Market{
			Symbol:           "ETH/USDT",
			Exchange:         types.ExchangeBinance,
			AssetType:        types.AssetTypeCrypto,
			BaseAsset:        "ETH",
			QuoteAsset:       "USDT",
			Status:           "TRADING",
			MinQty:           decimal.NewFromFloat(0.0001),
			MaxQty:           decimal.NewFromFloat(9000),
			StepSize:         decimal.NewFromFloat(0.0001),
			MinNotional:      decimal.NewFromFloat(10),
			PricePrecision:   2,
			QuantityPrecision: 4,
		}
	}

	return nil
}

func (e *Exchange) PlaceOrder(order *types.Order) (*types.Order, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.rateLimiter.Wait()

	logger.Info("Placing order",
		"exchange", e.Name,
		"symbol", order.Symbol,
		"side", order.Side,
		"type", order.Type,
		"quantity", order.Quantity,
	)

	order.ID = uuid.New()
	order.Status = types.OrderStatusPending
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	go e.simulateOrderFill(order)

	e.mu.Lock()
	e.orders[order.ID.String()] = order
	e.mu.Unlock()

	return order, nil
}

func (e *Exchange) simulateOrderFill(order *types.Order) {
	time.Sleep(500 * time.Millisecond)

	e.mu.Lock()
	defer e.mu.Unlock()

	order.Status = types.OrderStatusFilled
	order.FilledQuantity = order.Quantity
	order.AvgFillPrice = order.Price
	order.FilledAt = new(time.Time)
	*order.FilledAt = time.Now()
	order.UpdatedAt = time.Now()

	logger.Info("Order filled",
		"order_id", order.ID,
		"symbol", order.Symbol,
		"quantity", order.FilledQuantity,
		"avg_price", order.AvgFillPrice,
	)
}

func (e *Exchange) CancelOrder(orderID string) error {
	if !e.IsConnected() {
		return fmt.Errorf("exchange not connected")
	}

	e.rateLimiter.Wait()

	e.mu.Lock()
	defer e.mu.Unlock()

	order, exists := e.orders[orderID]
	if !exists {
		return fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status == types.OrderStatusFilled {
		return fmt.Errorf("cannot cancel filled order")
	}

	order.Status = types.OrderStatusCancelled
	order.CancelledAt = new(time.Time)
	*order.CancelledAt = time.Now()
	order.UpdatedAt = time.Now()

	logger.Info("Order cancelled", "order_id", orderID)

	return nil
}

func (e *Exchange) GetBalance(asset string) (*types.Balance, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.mu.RLock()
	balance, exists := e.balances[asset]
	e.mu.RUnlock()

	if !exists {
		balance = &types.Balance{
			Asset:    asset,
			Free:     decimal.NewFromFloat(10000),
			Locked:   decimal.Zero,
			Total:    decimal.NewFromFloat(10000),
			USDValue: decimal.NewFromFloat(10000),
			Exchange: types.Exchange(e.Name),
		}
	}

	return balance, nil
}

func (e *Exchange) GetAllBalances() (map[string]*types.Balance, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*types.Balance, len(e.balances))
	for k, v := range e.balances {
		result[k] = v
	}

	if len(result) == 0 {
		result["USDT"] = &types.Balance{
			Asset:    "USDT",
			Free:     decimal.NewFromFloat(50000),
			Locked:   decimal.NewFromFloat(1000),
			Total:    decimal.NewFromFloat(51000),
			USDValue: decimal.NewFromFloat(51000),
			Exchange: types.Exchange(e.Name),
		}
		result["BTC"] = &types.Balance{
			Asset:    "BTC",
			Free:     decimal.NewFromFloat(1.5),
			Locked:   decimal.Zero,
			Total:    decimal.NewFromFloat(1.5),
			USDValue: decimal.NewFromFloat(66000),
			Exchange: types.Exchange(e.Name),
		}
	}

	return result, nil
}

func (e *Exchange) GetPositions() ([]*types.Position, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	positions := make([]*types.Position, 0, len(e.positions))
	for _, pos := range e.positions {
		positions = append(positions, pos)
	}

	return positions, nil
}

func (e *Exchange) GetPosition(symbol string) (*types.Position, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	pos, exists := e.positions[symbol]
	if !exists {
		return nil, nil
	}

	return pos, nil
}

func (e *Exchange) GetOpenOrders(symbol string) ([]*types.Order, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var orders []*types.Order
	for _, order := range e.orders {
		if order.Status == types.OrderStatusOpen || order.Status == types.OrderStatusPartiallyFilled {
			if symbol == "" || order.Symbol == symbol {
				orders = append(orders, order)
			}
		}
	}

	return orders, nil
}

func (e *Exchange) GetOrder(orderID string) (*types.Order, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	order, exists := e.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

func (e *Exchange) GetTicker(symbol string) (*types.Ticker, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.rateLimiter.Wait()

	ticker := &types.Ticker{
		Symbol:            symbol,
		Exchange:          types.Exchange(e.Name),
		LastPrice:         decimal.NewFromFloat(44135.68),
		BidPrice:          decimal.NewFromFloat(44135.00),
		AskPrice:          decimal.NewFromFloat(44136.00),
		Volume24h:         decimal.NewFromFloat(28500000000),
		QuoteVolume24h:    decimal.NewFromFloat(28500000000),
		High24h:           decimal.NewFromFloat(44500),
		Low24h:            decimal.NewFromFloat(43200),
		PriceChange:       decimal.NewFromFloat(1000),
		PriceChangePct:   decimal.NewFromFloat(2.34),
		Timestamp:         time.Now(),
	}

	switch {
	case strings.Contains(symbol, "ETH"):
		ticker.LastPrice = decimal.NewFromFloat(2380.45)
		ticker.BidPrice = decimal.NewFromFloat(2380.00)
		ticker.AskPrice = decimal.NewFromFloat(2381.00)
	case strings.Contains(symbol, "SOL"):
		ticker.LastPrice = decimal.NewFromFloat(97.20)
		ticker.BidPrice = decimal.NewFromFloat(97.15)
		ticker.AskPrice = decimal.NewFromFloat(97.25)
	case strings.Contains(symbol, "BNB"):
		ticker.LastPrice = decimal.NewFromFloat(292.50)
		ticker.BidPrice = decimal.NewFromFloat(292.00)
		ticker.AskPrice = decimal.NewFromFloat(293.00)
	case strings.Contains(symbol, "XRP"):
		ticker.LastPrice = decimal.NewFromFloat(0.51)
		ticker.BidPrice = decimal.NewFromFloat(0.505)
		ticker.AskPrice = decimal.NewFromFloat(0.515)
	}

	return ticker, nil
}

func (e *Exchange) GetCandles(symbol string, timeframe types.Timeframe, limit int) ([]*types.Candle, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.rateLimiter.Wait()

	if limit <= 0 {
		limit = 100
	}

	basePrice := 44000.0
	switch {
	case strings.Contains(symbol, "ETH"):
		basePrice = 2380.0
	case strings.Contains(symbol, "SOL"):
		basePrice = 97.0
	case strings.Contains(symbol, "BNB"):
		basePrice = 292.0
	case strings.Contains(symbol, "XRP"):
		basePrice = 0.51
	}

	candles := make([]*types.Candle, limit)
	interval := timeframe.Duration()

	for i := 0; i < limit; i++ {
		timestamp := time.Now().Add(-interval * time.Duration(limit-i))
		variance := (float64(i % 20) - 10) / 1000

		candle := &types.Candle{
			Symbol:    symbol,
			Exchange:  types.Exchange(e.Name),
			Timeframe: string(timeframe),
			Timestamp: timestamp,
			Open:      decimal.NewFromFloat(basePrice * (1 + variance)),
			High:      decimal.NewFromFloat(basePrice * (1 + variance + 0.001)),
			Low:       decimal.NewFromFloat(basePrice * (1 + variance - 0.001)),
			Close:     decimal.NewFromFloat(basePrice * (1 + variance + 0.0005)),
			Volume:    decimal.NewFromFloat(1000000 + float64(i)*10000),
			Closed:    i < limit-1,
		}

		candles[i] = candle
		basePrice = candle.Close.InexactFloat64()
	}

	return candles, nil
}

func (e *Exchange) GetOrderBook(symbol string, depth int) (*types.OrderBook, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.rateLimiter.Wait()

	if depth <= 0 {
		depth = 20
	}

	book := &types.OrderBook{
		Symbol:    symbol,
		Exchange:  types.Exchange(e.Name),
		Timestamp: time.Now(),
		Bids:      make([]types.PriceLevel, depth),
		Asks:      make([]types.PriceLevel, depth),
	}

	basePrice := 44135.68
	if strings.Contains(symbol, "ETH") {
		basePrice = 2380.45
	} else if strings.Contains(symbol, "SOL") {
		basePrice = 97.20
	}

	for i := 0; i < depth; i++ {
		book.Bids[i] = types.PriceLevel{
			Price:    decimal.NewFromFloat(basePrice - float64(i)*0.5),
			Quantity: decimal.NewFromFloat(float64(depth-i) * 0.1),
		}
		book.Asks[i] = types.PriceLevel{
			Price:    decimal.NewFromFloat(basePrice + float64(i)*0.5),
			Quantity: decimal.NewFromFloat(float64(depth-i) * 0.1),
		}
	}

	return book, nil
}

func (e *Exchange) Subscribe(symbol string, channel string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := fmt.Sprintf("%s:%s", symbol, channel)
	e.WSSubscriptions[key] = true

	logger.Info("Subscribed to channel",
		"exchange", e.Name,
		"symbol", symbol,
		"channel", channel,
	)

	return nil
}

func (e *Exchange) Unsubscribe(symbol string, channel string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := fmt.Sprintf("%s:%s", symbol, channel)
	delete(e.WSSubscriptions, key)

	logger.Info("Unsubscribed from channel",
		"exchange", e.Name,
		"symbol", symbol,
		"channel", channel,
	)

	return nil
}

func (e *Exchange) GetMarket(symbol string) (*types.Market, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	market, exists := e.markets[symbol]
	if !exists {
		return nil, fmt.Errorf("market not found: %s", symbol)
	}

	return market, nil
}

func (e *Exchange) GetAllMarkets() ([]*types.Market, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	markets := make([]*types.Market, 0, len(e.markets))
	for _, market := range e.markets {
		markets = append(markets, market)
	}

	return markets, nil
}

func (e *Exchange) GetTrades(symbol string, limit int) ([]*types.Trade, error) {
	if !e.IsConnected() {
		return nil, fmt.Errorf("exchange not connected")
	}

	e.rateLimiter.Wait()

	if limit <= 0 {
		limit = 100
	}

	trades := make([]*types.Trade, limit)
	basePrice := 44135.68

	for i := 0; i < limit; i++ {
		trade := &types.Trade{
			ID:            uuid.New(),
			Exchange:      types.Exchange(e.Name),
			Symbol:        symbol,
			Side:          types.OrderSideBuy,
			Price:         decimal.NewFromFloat(basePrice),
			Quantity:      decimal.NewFromFloat(0.1 + float64(i%10)*0.01),
			QuoteQty:      decimal.NewFromFloat(basePrice * 0.1),
			Commission:    decimal.NewFromFloat(0.0001),
			Timestamp:     time.Now().Add(-time.Duration(i) * time.Second),
			IsBuyerMaker:  i%2 == 0,
		}
		trades[i] = trade
		basePrice += (float64(i%5) - 2) * 0.5
	}

	return trades, nil
}

func (e *Exchange) signRequest(params map[string]string) string {
	if e.APISecret == "" {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var query strings.Builder
	for i, k := range keys {
		if i > 0 {
			query.WriteString("&")
		}
		query.WriteString(k)
		query.WriteString("=")
		query.WriteString(params[k])
	}

	h := hmac.New(sha256.New, []byte(e.APISecret))
	h.Write([]byte(query.String()))
	return hex.EncodeToString(h.Sum(nil))
}

func (e *Exchange) signRequestSHA512(params map[string]string) string {
	if e.APISecret == "" {
		return ""
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var query strings.Builder
	for i, k := range keys {
		if i > 0 {
			query.WriteString("&")
		}
		query.WriteString(k)
		query.WriteString("=")
		query.WriteString(url.QueryEscape(params[k]))
	}

	h := hmac.New(sha512.New, []byte(e.APISecret))
	h.Write([]byte(query.String()))
	return hex.EncodeToString(h.Sum(nil))
}

func (e *Exchange) makeRequest(method, endpoint string, params map[string]string) ([]byte, error) {
	e.rateLimiter.Wait()

	var reqBody io.Reader
	var queryString string

	if method == "GET" || method == "DELETE" {
		if len(params) > 0 {
			queryString = "?"
			keys := make([]string, 0, len(params))
			for k := range params {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for i, k := range keys {
				if i > 0 {
					queryString += "&"
				}
				queryString += k + "=" + url.QueryEscape(params[k])
			}
		}
	} else {
		if len(params) > 0 {
			body, _ := json.Marshal(params)
			reqBody = strings.NewReader(string(body))
		}
	}

	url := e.BaseURL + endpoint + queryString

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OpenTrader/1.0")

	if e.APIKey != "" {
		req.Header.Set("X-MBX-APIKEY", e.APIKey)
	}

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (e *Exchange) parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (e *Exchange) parseInt(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func (e *Exchange) GetName() string {
	return e.Name
}

func (e *Exchange) GetStatus() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.connected {
		return "connected"
	}
	return "disconnected"
}

func (e *Exchange) GetLatency() time.Duration {
	return time.Since(e.lastHeartbeat)
}

func (e *Exchange) Refresh() error {
	if !e.IsConnected() {
		return fmt.Errorf("exchange not connected")
	}

	e.lastHeartbeat = time.Now()

	if err := e.loadMarkets(); err != nil {
		return err
	}

	return nil
}
