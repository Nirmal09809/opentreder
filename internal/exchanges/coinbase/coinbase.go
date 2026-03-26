package coinbase

import (
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
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Client struct {
	apiKey        string
	apiSecret     string
	passphrase    string
	baseURL       string
	sandbox       bool
	httpClient    *http.Client
	rateLimiter   *RateLimiter
}

type Config struct {
	APIKey      string
	APISecret   string
	Passphrase  string
	Sandbox     bool
}

type RateLimiter struct {
	mu       sync.Mutex
	requests []time.Time
	limit    int
	window   time.Duration
}

type APIResponse struct {
	Data    interface{} `json:"data"`
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
}

type Product struct {
	ID               string `json:"id"`
	BaseCurrency    string `json:"base_currency"`
	QuoteCurrency   string `json:"quote_currency"`
	BaseMinSize     string `json:"base_min_size"`
	BaseMaxSize     string `json:"base_max_size"`
	QuoteIncrement  string `json:"quote_increment"`
	BaseIncrement   string `json:"base_increment"`
	DisplayName     string `json:"display_name"`
	Status          string `json:"status"`
	MarketMode      string `json:"market_mode"`
	TradingDisabled bool   `json:"trading_disabled"`
}

type Account struct {
	ID        string `json:"id"`
	Currency  string `json:"currency"`
	Balance   string `json:"balance"`
	Available string `json:"available"`
	Hold      string `json:"hold"`
}

type Order struct {
	ID            string `json:"id"`
	ProductID    string `json:"product_id"`
	Side         string `json:"side"`
	Type         string `json:"type"`
	Price        string `json:"price,omitempty"`
	Size         string `json:"size,omitempty"`
	Funds        string `json:"funds,omitempty"`
	SpecifiedFunds string `json:"specified_funds,omitempty"`
	Status       string `json:"status"`
	FillFees     string `json:"fill_fees"`
	FilledSize   string `json:"filled_size"`
	ExecutedValue string `json:"executed_value"`
	Time         string `json:"time"`
	TimeInForce  string `json:"time_in_force,omitempty"`
	PostOnly     bool   `json:"post_only,omitempty"`
}

type Trade struct {
	ID        string `json:"id"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Side      string `json:"side"`
	Time      string `json:"time"`
}

type Candle struct {
	Time   float64 `json:"time"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

func NewClient(cfg *Config) (*Client, error) {
	baseURL := "https://api.exchange.coinbase.com"
	if cfg.Sandbox {
		baseURL = "https://api-public.sandbox.exchange.coinbase.com"
	}

	return &Client{
		apiKey:     cfg.APIKey,
		apiSecret:  cfg.APISecret,
		passphrase: cfg.Passphrase,
		baseURL:    baseURL,
		sandbox:    cfg.Sandbox,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		rateLimiter: newRateLimiter(10, time.Second),
	}, nil
}

func newRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window}
}

func (r *RateLimiter) Wait() {
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
		time.Sleep(100 * time.Millisecond)
		r.Wait()
		return
	}

	r.requests = append(valid, now)
}

func (c *Client) GetProducts(ctx context.Context) ([]*types.Market, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", "/products", nil, false, &resp); err != nil {
		return nil, err
	}

	var products []Product
	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &products)

	markets := make([]*types.Market, len(products))
	for i, p := range products {
		markets[i] = &types.Market{
			Symbol:       p.ID,
			Exchange:     types.ExchangeCoinbase,
			AssetType:    types.AssetTypeCrypto,
			BaseAsset:    p.BaseCurrency,
			QuoteAsset:   p.QuoteCurrency,
			Status:       p.Status,
			MinQty:       parseDecimal(p.BaseMinSize),
			MaxQty:       parseDecimal(p.BaseMaxSize),
			StepSize:     parseDecimal(p.BaseIncrement),
		}
	}

	return markets, nil
}

func (c *Client) GetProduct(ctx context.Context, productID string) (*types.Market, error) {
	products, err := c.GetProducts(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range products {
		if p.Symbol == productID {
			return p, nil
		}
	}

	return nil, fmt.Errorf("product not found: %s", productID)
}

func (c *Client) GetAccounts(ctx context.Context) ([]*types.Balance, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", "/accounts", nil, true, &resp); err != nil {
		return nil, err
	}

	var accounts []Account
	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &accounts)

	balances := make([]*types.Balance, 0)
	for _, acc := range accounts {
		balance := parseDecimal(acc.Balance)
		if balance.GreaterThan(decimal.Zero) {
			balances = append(balances, &types.Balance{
				Asset:    acc.Currency,
				Free:     parseDecimal(acc.Available),
				Locked:   parseDecimal(acc.Hold),
				Total:    balance,
				Exchange: types.ExchangeCoinbase,
			})
		}
	}

	return balances, nil
}

func (c *Client) GetAccount(ctx context.Context, accountID string) (*types.Balance, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", fmt.Sprintf("/accounts/%s", accountID), nil, true, &resp); err != nil {
		return nil, err
	}

	var account Account
	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &account)

	return &types.Balance{
		Asset:    account.Currency,
		Free:     parseDecimal(account.Available),
		Locked:   parseDecimal(account.Hold),
		Total:    parseDecimal(account.Balance),
		Exchange: types.ExchangeCoinbase,
	}, nil
}

func (c *Client) PlaceOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	params := map[string]string{
		"product_id": order.Symbol,
		"side":      string(order.Side),
		"type":      string(order.Type),
	}

	if !order.Price.IsZero() {
		params["price"] = order.Price.String()
	}

	if !order.Quantity.IsZero() {
		params["size"] = order.Quantity.String()
	}

	if order.TimeInForce != "" {
		params["time_in_force"] = string(order.TimeInForce)
	}

	var resp APIResponse
	if err := c.request(ctx, "POST", "/orders", params, true, &resp); err != nil {
		return nil, err
	}

	var coinbaseOrder Order
	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &coinbaseOrder)

	order.ID, _ = uuid.Parse(coinbaseOrder.ID)
	order.Status = parseOrderStatus(coinbaseOrder.Status)
	order.FilledQuantity = parseDecimal(coinbaseOrder.FilledSize)
	order.Commission = parseDecimal(coinbaseOrder.FillFees)

	return order, nil
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (*types.Order, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", fmt.Sprintf("/orders/%s", orderID), nil, true, &resp); err != nil {
		return nil, err
	}

	var order Order
	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &order)

	return &types.Order{
		ID:             uuid.MustParse(order.ID).String(),
		Symbol:         order.ProductID,
		Side:           types.OrderSide(order.Side),
		Type:           types.OrderType(order.Type),
		Status:         parseOrderStatus(order.Status),
		Quantity:       parseDecimal(order.Size),
		FilledQuantity: parseDecimal(order.FilledSize),
		Commission:    parseDecimal(order.FillFees),
		Exchange:      types.ExchangeCoinbase,
	}, nil
}

func (c *Client) GetOpenOrders(ctx context.Context) ([]*types.Order, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", "/orders?status=open", nil, true, &resp); err != nil {
		return nil, err
	}

	var orders []Order
	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &orders)

	result := make([]*types.Order, len(orders))
	for i, o := range orders {
		result[i] = &types.Order{
			ID:             uuid.MustParse(o.ID).String(),
			Symbol:         o.ProductID,
			Side:           types.OrderSide(o.Side),
			Type:           types.OrderType(o.Type),
			Status:         parseOrderStatus(o.Status),
			Quantity:       parseDecimal(o.Size),
			FilledQuantity: parseDecimal(o.FilledSize),
			Commission:    parseDecimal(o.FillFees),
			Exchange:      types.ExchangeCoinbase,
		}
	}

	return result, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	return c.request(ctx, "DELETE", fmt.Sprintf("/orders/%s", orderID), nil, true, nil)
}

func (c *Client) CancelAllOrders(ctx context.Context) error {
	return c.request(ctx, "DELETE", "/orders", nil, true, nil)
}

func (c *Client) GetFills(ctx context.Context, orderID string) ([]*types.Trade, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", fmt.Sprintf("/fills?order_id=%s", orderID), nil, true, &resp); err != nil {
		return nil, err
	}

	var fills []struct {
		TradeID   string `json:"trade_id"`
		Price     string `json:"price"`
		Size      string `json:"size"`
		Side      string `json:"side"`
		Time      string `json:"time"`
		ProductID string `json:"product_id"`
	}

	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &fills)

	trades := make([]*types.Trade, len(fills))
	for i, f := range fills {
		trades[i] = &types.Trade{
			ID:        uuid.MustParse(f.TradeID).String(),
			Symbol:    f.ProductID,
			Side:      types.OrderSide(f.Side),
			Price:     parseDecimal(f.Price),
			Quantity:  parseDecimal(f.Size),
			Exchange:  types.ExchangeCoinbase,
		}
	}

	return trades, nil
}

func (c *Client) GetCandles(ctx context.Context, productID string, granularity string) ([]*types.Candle, error) {
	params := map[string]string{
		"granularity": granularity,
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", fmt.Sprintf("/products/%s/candles", productID), params, false, &resp); err != nil {
		return nil, err
	}

	var rawCandles [][]float64
	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &rawCandles)

	candles := make([]*types.Candle, len(rawCandles))
	for i, raw := range rawCandles {
		candles[i] = &types.Candle{
			Symbol:    productID,
			Exchange:  types.ExchangeCoinbase,
			Timeframe: granularity,
			Timestamp: time.Unix(int64(raw[0]), 0),
			Low:       decimal.NewFromFloat(raw[1]),
			High:      decimal.NewFromFloat(raw[2]),
			Open:      decimal.NewFromFloat(raw[3]),
			Close:     decimal.NewFromFloat(raw[4]),
			Volume:    decimal.NewFromFloat(raw[5]),
			Closed:    true,
		}
	}

	return candles, nil
}

func (c *Client) GetTicker(ctx context.Context, productID string) (*types.Ticker, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", fmt.Sprintf("/products/%s/ticker", productID), nil, false, &resp); err != nil {
		return nil, err
	}

	var ticker struct {
		Price     string `json:"price"`
		Bid       string `json:"bid"`
		Ask       string `json:"ask"`
		Volume    string `json:"volume"`
		Time      string `json:"time"`
	}

	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &ticker)

	return &types.Ticker{
		Symbol:     productID,
		Exchange:   types.ExchangeCoinbase,
		LastPrice:  parseDecimal(ticker.Price),
		BidPrice:  parseDecimal(ticker.Bid),
		AskPrice:  parseDecimal(ticker.Ask),
		Volume24h: parseDecimal(ticker.Volume),
	}, nil
}

func (c *Client) GetOrderBook(ctx context.Context, productID string, level int) (*types.OrderBook, error) {
	params := map[string]string{}
	if level > 0 {
		params["level"] = strconv.Itoa(level)
	}

	var resp APIResponse
	if err := c.request(ctx, "GET", fmt.Sprintf("/products/%s/book", productID), params, false, &resp); err != nil {
		return nil, err
	}

	var book struct {
		Bids [][]string `json:"bids"`
		Asks [][]string `json:"asks"`
	}

	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &book)

	orderBook := &types.OrderBook{
		Symbol:   productID,
		Exchange: types.ExchangeCoinbase,
		Bids:     make([]types.OrderBookLevel, len(book.Bids)),
		Asks:     make([]types.OrderBookLevel, len(book.Asks)),
	}

	for i, bid := range book.Bids {
		orderBook.Bids[i] = types.OrderBookLevel{
			Price:    parseDecimal(bid[0]),
			Quantity: parseDecimal(bid[1]),
		}
	}

	for i, ask := range book.Asks {
		orderBook.Asks[i] = types.OrderBookLevel{
			Price:    parseDecimal(ask[0]),
			Quantity: parseDecimal(ask[1]),
		}
	}

	return orderBook, nil
}

func (c *Client) GetTrades(ctx context.Context, productID string) ([]*types.Trade, error) {
	var resp APIResponse
	if err := c.request(ctx, "GET", fmt.Sprintf("/products/%s/trades", productID), nil, false, &resp); err != nil {
		return nil, err
	}

	var rawTrades []struct {
		ID        string `json:"id"`
		Price     string `json:"price"`
		Size      string `json:"size"`
		Side      string `json:"side"`
		Time      string `json:"time"`
	}

	data, _ := json.Marshal(resp.Data)
	json.Unmarshal(data, &rawTrades)

	trades := make([]*types.Trade, len(rawTrades))
	for i, t := range rawTrades {
		trades[i] = &types.Trade{
			ID:        t.ID,
			Symbol:    productID,
			Side:      types.OrderSide(t.Side),
			Price:     parseDecimal(t.Price),
			Quantity:  parseDecimal(t.Size),
			Exchange:  types.ExchangeCoinbase,
		}
	}

	return trades, nil
}

func (c *Client) request(ctx context.Context, method, endpoint string, params map[string]string, signed bool, result *APIResponse) error {
	c.rateLimiter.Wait()

	var body io.Reader
	url := c.baseURL + endpoint

	if len(params) > 0 {
		values := url.Values{}
		for k, v := range params {
			values.Set(k, v)
		}

		if method == "GET" {
			url += "?" + values.Encode()
		} else {
			body = strings.NewReader(values.Encode())
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if signed {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		signature := c.sign(method, endpoint, "", timestamp)

		req.Header.Set("CB-ACCESS-KEY", c.apiKey)
		req.Header.Set("CB-ACCESS-SIGN", signature)
		req.Header.Set("CB-ACCESS-TIMESTAMP", timestamp)
		req.Header.Set("CB-ACCESS-PASSPHRASE", c.passphrase)
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

	if resp.StatusCode >= 400 {
		return fmt.Errorf("coinbase API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		json.Unmarshal(respBody, result)
	}

	return nil
}

func (c *Client) sign(method, requestPath, body, timestamp string) string {
	message := timestamp + method + requestPath + body
	secret, _ := base64.StdEncoding.DecodeString(c.apiSecret)
	
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func parseDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func parseOrderStatus(status string) types.OrderStatus {
	switch status {
	case "pending":
		return types.OrderStatusPending
	case "open":
		return types.OrderStatusOpen
	case "active":
		return types.OrderStatusOpen
	case "done":
		return types.OrderStatusFilled
	case "cancelled":
		return types.OrderStatusCancelled
	default:
		return types.OrderStatusPending
	}
}

func (c *Client) GetName() string       { return "coinbase" }
func (c *Client) IsConnected() bool     { return c.apiKey != "" }
func (c *Client) Connect(ctx context.Context) error {
	logger.Info("Coinbase client connected", "sandbox", c.sandbox)
	return nil
}
func (c *Client) Disconnect() error {
	logger.Info("Coinbase client disconnected")
	return nil
}
