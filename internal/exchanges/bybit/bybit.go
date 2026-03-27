package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	recvWindow    time.Duration
	rateLimiter  *RateLimiter
	mu            sync.RWMutex
}

type Config struct {
	APIKey      string
	APISecret   string
	Testnet     bool
	BaseURL     string
	WSURL       string
	RecvWindow  time.Duration
	Category    string
}

type RateLimiter struct {
	mu       sync.Mutex
	requests []time.Time
	limit    int
	window   time.Duration
}

type APIResponse struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Body    json.RawMessage `json:"result"`
}

type Category string

const (
	CategoryLinear   Category = "linear"
	CategoryInverse  Category = "inverse"
	CategorySpot     Category = "spot"
)

func NewClient(cfg *Config) (*Client, error) {
	if cfg.RecvWindow == 0 {
		cfg.RecvWindow = 5 * time.Second
	}

	baseURL := "https://api.bybit.com"
	wsURL := "wss://stream.bybit.com"

	if cfg.Testnet {
		baseURL = "https://api-testnet.bybit.com"
		wsURL = "wss://stream-testnet.bybit.com"
	}

	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	if cfg.WSURL != "" {
		wsURL = cfg.WSURL
	}

	return &Client{
		apiKey:       cfg.APIKey,
		apiSecret:    cfg.APISecret,
		testnet:      cfg.Testnet,
		baseURL:       baseURL,
		wsURL:         wsURL,
		recvWindow:    cfg.RecvWindow,
		rateLimiter:   newRateLimiter(100, time.Second),
	}, nil
}

func newRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit: limit,
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

func (c *Client) GetServerTime(ctx context.Context) (string, error) {
	params := map[string]string{}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/market/time", params, false, CategorySpot, &resp); err != nil {
		return "", err
	}

	var result struct {
		TimeSecond string `json:"timeSecond"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return "", err
	}

	return result.TimeSecond, nil
}

func (c *Client) GetInstrumentsInfo(ctx context.Context, category Category, symbol string) ([]*types.Market, error) {
	params := map[string]string{
		"category": string(category),
	}
	if symbol != "" {
		params["symbol"] = symbol
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/market/instruments-info", params, false, category, &resp); err != nil {
		return nil, err
	}

	var result struct {
		List []struct {
			Symbol         string `json:"symbol"`
			ContractType   string `json:"contractType"`
			BaseCoin       string `json:"baseCoin"`
			QuoteCoin      string `json:"quoteCoin"`
			Status         string `json:"status"`
			LotSizeFilter  struct {
				MinOrderQty string `json:"minOrderQty"`
				MaxOrderQty string `json:"maxOrderQty"`
				QtyStep      string `json:"qtyStep"`
			} `json:"lotSizeFilter"`
			PriceFilter struct {
				MinPrice    string `json:"minPrice"`
				MaxPrice    string `json:"maxPrice"`
				TickSize    string `json:"tickSize"`
			} `json:"priceFilter"`
		} `json:"list"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	markets := make([]*types.Market, len(result.List))
	for i, item := range result.List {
		markets[i] = &types.Market{
			Symbol:   item.Symbol,
			Exchange: types.ExchangeBybit,
			BaseAsset: item.BaseCoin,
			QuoteAsset: item.QuoteCoin,
			Status:   item.Status,
			MinQty:   parseDecimal(item.LotSizeFilter.MinOrderQty),
			MaxQty:   parseDecimal(item.LotSizeFilter.MaxOrderQty),
			StepSize: parseDecimal(item.LotSizeFilter.QtyStep),
		}
	}

	return markets, nil
}

func (c *Client) GetTicker(ctx context.Context, category Category, symbol string) (*types.Ticker, error) {
	params := map[string]string{
		"category": string(category),
		"symbol":   symbol,
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/market/tickers", params, false, category, &resp); err != nil {
		return nil, err
	}

	var result struct {
		List []struct {
			Symbol            string `json:"symbol"`
			Bid1Price       string `json:"bid1Price"`
			Bid1Size         string `json:"bid1Size"`
			Ask1Price       string `json:"ask1Price"`
			Ask1Size         string `json:"ask1Size"`
			LastPrice       string `json:"lastPrice"`
			HighPrice24h    string `json:"highPrice24h"`
			LowPrice24h     string `json:"lowPrice24h"`
			Volume24h       string `json:"volume24h"`
			Turnover24h     string `json:"turnover24h"`
			Price24hPcnt    string `json:"price24hPcnt"`
		} `json:"list"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	if len(result.List) == 0 {
		return nil, fmt.Errorf("no ticker data for %s", symbol)
	}

	item := result.List[0]

	return &types.Ticker{
		Symbol:           symbol,
		Exchange:         types.ExchangeBybit,
		LastPrice:        parseDecimal(item.LastPrice),
		BidPrice:         parseDecimal(item.Bid1Price),
		AskPrice:         parseDecimal(item.Ask1Price),
		High24h:          parseDecimal(item.HighPrice24h),
		Low24h:           parseDecimal(item.LowPrice24h),
		Volume24h:        parseDecimal(item.Volume24h),
		QuoteVolume24h:   parseDecimal(item.Turnover24h),
		PriceChangePct:   parseDecimal(item.Price24hPcnt).Mul(decimal.NewFromInt(100)),
		Timestamp:        time.Now(),
	}, nil
}

func (c *Client) GetOrderBook(ctx context.Context, category Category, symbol string, limit int) (*types.OrderBook, error) {
	if limit <= 0 {
		limit = 50
	}

	params := map[string]string{
		"category": string(category),
		"symbol":   symbol,
		"limit":    fmt.Sprintf("%d", limit),
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/market/orderbook", params, false, category, &resp); err != nil {
		return nil, err
	}

	var result struct {
		B [][]string `json:"b"`
		A [][]string `json:"a"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	book := &types.OrderBook{
		Symbol:    symbol,
		Exchange:  types.ExchangeBybit,
		Timestamp: time.Now(),
		Bids:      make([]types.PriceLevel, len(result.B)),
		Asks:      make([]types.PriceLevel, len(result.A)),
	}

	for i, bid := range result.B {
		book.Bids[i] = types.PriceLevel{
			Price:    parseDecimal(bid[0]),
			Quantity: parseDecimal(bid[1]),
		}
	}

	for i, ask := range result.A {
		book.Asks[i] = types.PriceLevel{
			Price:    parseDecimal(ask[0]),
			Quantity: parseDecimal(ask[1]),
		}
	}

	return book, nil
}

func (c *Client) GetCandles(ctx context.Context, category Category, symbol string, interval string, limit int) ([]*types.Candle, error) {
	if limit <= 0 {
		limit = 200
	}

	params := map[string]string{
		"category": string(category),
		"symbol":   symbol,
		"interval": interval,
		"limit":    fmt.Sprintf("%d", limit),
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/market/kline", params, false, category, &resp); err != nil {
		return nil, err
	}

	var result struct {
		List [][]string `json:"list"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	candles := make([]*types.Candle, len(result.List))
	for i, item := range result.List {
		timestamp, _ := time.Parse("2006-01-02T15:04:05.000Z", item[0])

		candles[i] = &types.Candle{
			Symbol:    symbol,
			Exchange:  types.ExchangeBybit,
			Timeframe: interval,
			Timestamp: timestamp,
			Open:      parseDecimal(item[1]),
			High:      parseDecimal(item[2]),
			Low:       parseDecimal(item[3]),
			Close:     parseDecimal(item[4]),
			Volume:    parseDecimal(item[5]),
			Closed:    true,
		}
	}

	return candles, nil
}

func (c *Client) GetWalletBalance(ctx context.Context, accountType string) (map[string]*types.Balance, error) {
	params := map[string]string{
		"accountType": accountType,
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/account/wallet-balance", params, true, "", &resp); err != nil {
		return nil, err
	}

	var result struct {
		List []struct {
			AccountType string `json:"accountType"`
			Coins      []struct {
				Coin         string `json:"coin"`
				WalletBalance string `json:"walletBalance"`
				AvailableToWithdraw string `json:"availableToWithdraw"`
			} `json:"coin"`
		} `json:"list"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	balances := make(map[string]*types.Balance)

	for _, account := range result.List {
		for _, coin := range account.Coins {
			balance := &types.Balance{
				Asset:    coin.Coin,
				Free:     parseDecimal(coin.AvailableToWithdraw),
				Total:    parseDecimal(coin.WalletBalance),
				Exchange: types.ExchangeBybit,
			}
			balance.Locked = balance.Total.Sub(balance.Free)
			balances[coin.Coin] = balance
		}
	}

	return balances, nil
}

func (c *Client) PlaceOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	params := map[string]string{
		"category": string(CategoryLinear),
		"symbol":   order.Symbol,
		"side":     string(order.Side),
		"orderType": string(order.Type),
		"qty":       order.Quantity.String(),
	}

	if !order.Price.IsZero() {
		params["price"] = order.Price.String()
	}

	if order.Type == types.OrderTypeStop || order.Type == types.OrderTypeStopLimit {
		params["triggerPrice"] = order.StopPrice.String()
	}

	if order.TimeInForce != "" {
		params["timeInForce"] = string(order.TimeInForce)
	}

	var resp APIResponse
	if err := c.request(ctx, "POST", "/v5/order/create", params, true, CategoryLinear, &resp); err != nil {
		return nil, err
	}

	var result struct {
		OrderID string `json:"orderId"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	order.ID, _ = uuid.Parse(result.OrderID)
	order.Status = types.OrderStatusPending
	order.CreatedAt = time.Now()

	return order, nil
}

func (c *Client) GetOpenOrders(ctx context.Context, category Category, symbol string) ([]*types.Order, error) {
	params := map[string]string{
		"category": string(category),
	}
	if symbol != "" {
		params["symbol"] = symbol
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/order/realtime", params, true, category, &resp); err != nil {
		return nil, err
	}

	var result struct {
		List []struct {
			OrderID       string `json:"orderId"`
			Symbol        string `json:"symbol"`
			Side         string `json:"side"`
			OrderType    string `json:"orderType"`
			Qty          string `json:"qty"`
			Price        string `json:"price"`
			OrderStatus  string `json:"orderStatus"`
			CreateTime   string `json:"createTime"`
		} `json:"list"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	orders := make([]*types.Order, len(result.List))
	for i, item := range result.List {
		createdAt, _ := time.Parse("2006-01-02T15:04:05.000Z", item.CreateTime)

		orders[i] = &types.Order{
			ID:       uuid.MustParse(item.OrderID),
			Symbol:   item.Symbol,
			Side:     types.OrderSide(item.Side),
			Type:     types.OrderType(item.OrderType),
			Quantity: parseDecimal(item.Qty),
			Price:    parseDecimal(item.Price),
			Status:   c.parseStatus(item.OrderStatus),
			CreatedAt: createdAt,
		}
	}

	return orders, nil
}

func (c *Client) CancelOrder(ctx context.Context, category Category, symbol, orderID string) error {
	params := map[string]string{
		"category": string(category),
		"symbol":   symbol,
		"orderId":  orderID,
	}

	var resp APIResponse
	return c.request(ctx, "POST", "/v5/order/cancel", params, true, category, &resp)
}

func (c *Client) GetPositions(ctx context.Context, category Category, symbol string) ([]*types.Position, error) {
	params := map[string]string{
		"category": string(category),
	}
	if symbol != "" {
		params["symbol"] = symbol
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", "/v5/position/list", params, true, category, &resp); err != nil {
		return nil, err
	}

	var result struct {
		List []struct {
			Symbol       string `json:"symbol"`
			Side        string `json:"side"`
			Size        string `json:"size"`
			EntryPrice  string `json:"entryPrice"`
			MarkPrice   string `json:"markPrice"`
			Leverage    string `json:"leverage"`
			UnrealizedPnl string `json:"unrealizedPnl"`
			PositionValue string `json:"positionValue"`
		} `json:"list"`
	}

	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, err
	}

	positions := make([]*types.Position, 0)
	for _, item := range result.List {
		size := parseDecimal(item.Size)
		if size.IsZero() {
			continue
		}

		side := types.PositionSideLong
		if item.Side == "Sell" {
			side = types.PositionSideShort
		}

		positions = append(positions, &types.Position{
			ID:              uuid.New(),
			Symbol:          item.Symbol,
			Exchange:        types.ExchangeBybit,
			Side:            side,
			Quantity:        size,
			AvgEntryPrice:   parseDecimal(item.EntryPrice),
			CurrentPrice:    parseDecimal(item.MarkPrice),
			UnrealizedPnL:   parseDecimal(item.UnrealizedPnl),
			Leverage:        parseDecimal(item.Leverage),
		})
	}

	return positions, nil
}

func (c *Client) SetLeverage(ctx context.Context, category Category, symbol string, buyLeverage, sellLeverage int) error {
	params := map[string]string{
		"category":      string(category),
		"symbol":        symbol,
		"buyLeverage":   fmt.Sprintf("%d", buyLeverage),
		"sellLeverage":  fmt.Sprintf("%d", sellLeverage),
	}

	var resp APIResponse
	return c.request(ctx, "POST", "/v5/position/set-leverage", params, true, category, &resp)
}

func (c *Client) request(ctx context.Context, method, endpoint string, params map[string]string, signed bool, category Category, result *APIResponse) error {
	c.rateLimiter.Wait()

	var queryString string
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, url.QueryEscape(params[k])))
		}
		queryString = strings.Join(parts, "&")
	}

	url := c.baseURL + endpoint

	var body io.Reader
	if method == "POST" {
		body = strings.NewReader(queryString)
	} else if queryString != "" {
		url += "?" + queryString
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "OpenTrader/1.0")

	if signed {
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		recvWindow := fmt.Sprintf("%d", c.recvWindow.Milliseconds())

		paramStr := queryString
		signStr := timestamp + c.apiKey + recvWindow + paramStr

		if category != "" {
			signStr = string(category) + signStr
		}

		signature := c.sign(signStr)

		req.Header.Set("X-BAPI-API-KEY", c.apiKey)
		req.Header.Set("X-BAPI-SIGN", signature)
		req.Header.Set("X-BAPI-SIGN-TYPE", "2")
		req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
		req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.RetCode != 0 {
		return fmt.Errorf("bybit API error: %s (code %d)", result.RetMsg, result.RetCode)
	}

	return nil
}

func (c *Client) sign(message string) string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *Client) parseStatus(status string) types.OrderStatus {
	switch status {
	case "Created", "New":
		return types.OrderStatusOpen
	case "PartiallyFilled":
		return types.OrderStatusPartiallyFilled
	case "Filled":
		return types.OrderStatusFilled
	case "Cancelled":
		return types.OrderStatusCancelled
	case "Rejected":
		return types.OrderStatusRejected
	default:
		return types.OrderStatusPending
	}
}

func parseDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func (c *Client) GetName() string {
	return "bybit"
}

func (c *Client) IsConnected() bool {
	return c.apiKey != ""
}

func (c *Client) Connect(ctx context.Context) error {
	logger.Info("Bybit client connected", "testnet", c.testnet)
	return nil
}

func (c *Client) Disconnect() error {
	logger.Info("Bybit client disconnected")
	return nil
}
