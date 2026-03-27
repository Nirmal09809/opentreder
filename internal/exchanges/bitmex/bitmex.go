package bitmex

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	baseURL       string
	wsURL         string
	httpClient    *http.Client
	wsClient      *websocket.Dialer
	wsConn        *websocket.Conn
	testnet       bool
	mu            sync.RWMutex
}

type Config struct {
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Testnet    bool   `json:"testnet"`
}

type Request struct {
	Op     string      `json:"op,omitempty"`
	Args   []string    `json:"args,omitempty"`
	Filter interface{} `json:"filter,omitempty"`
}

type Response struct {
	Success   bool `json:"success,omitempty"`
	Subscribe string `json:"subscribe,omitempty"`
	Table     string `json:"table,omitempty"`
	Action    string `json:"action,omitempty"`
	Data      []any `json:"data,omitempty"`
}

type Ticker struct {
	Symbol     string `json:"symbol"`
	BidPrice   string `json:"bidPrice"`
	AskPrice   string `json:"askPrice"`
	LastPrice  string `json:"lastPrice"`
	MarkPrice  string `json:"markPrice"`
	HighPrice  string `json:"highPrice"`
	LowPrice   string `json:"lowPrice"`
	Volume     string `json:"volume"`
	Timestamp  string `json:"timestamp"`
}

type OrderBook struct {
	Symbol   string   `json:"symbol"`
	Bids     [][]any `json:"bids"`
	Asks     [][]any `json:"asks"`
	Timestamp string `json:"timestamp"`
}

type Trade struct {
	Symbol     string `json:"symbol"`
	Side       string `json:"side"`
	Price      string `json:"price"`
	Size       string `json:"size"`
	ID         int64  `json:"id"`
	Timestamp  string `json:"timestamp"`
}

type Candle struct {
	Symbol     string `json:"symbol"`
	Timestamp  int64  `json:"timestamp"`
	Open       string `json:"open"`
	High       string `json:"high"`
	Low        string `json:"low"`
	Close      string `json:"close"`
	Volume     string `json:"volume"`
	Trades     int    `json:"trades"`
}

type Order struct {
	OrderID    int64   `json:"orderID"`
	Symbol     string  `json:"symbol"`
	Side       string  `json:"side"`
	Price      string  `json:"price"`
	Size       string  `json:"size"`
	OrderType  string  `json:"ordType"`
	Status     string  `json:"ordStatus"`
	FilledQty  string  `json:"filledQty"`
	AvgFillPx  string  `json:"avgFillPx"`
	CreatedAt  string  `json:"createdAt"`
}

type Position struct {
	Symbol        string `json:"symbol"`
	CurrentQty    int    `json:"currentQty"`
	AvgCostPrice  string `json:"avgCostPrice"`
	MarkPrice     string `json:"markPrice"`
	LiquidationPx string `json:"liquidationPrice"`
	UnrealizedPnL string `json:"unrealisedPnl"`
	Leverage      string `json:"leverage"`
}

type Balance struct {
	Asset     string `json:"asset"`
	WalletBal string `json:"walletBalance"`
	MarginBal string `json:"marginBalance"`
	AvailBal  string `json:"availableMargin"`
}

type Funding struct {
	Symbol       string `json:"symbol"`
	FundingRate  string `json:"fundingRate"`
	Timestamp    int64  `json:"timestamp"`
	FundingInterval int `json:"fundingInterval"`
}

func NewClient(cfg Config) *Client {
	baseURL := "https://www.bitmex.com/api/v1"
	wsURL := "wss://www.bitmex.com/realtime"

	if cfg.Testnet {
		baseURL = "https://testnet.bitmex.com/api/v1"
		wsURL = "wss://testnet.bitmex.com/realtime"
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

func (c *Client) GetInstruments(ctx context.Context, symbol string) ([]map[string]any, error) {
	path := "/instrument"
	if symbol != "" {
		path += "?symbol=" + symbol
	}

	resp, err := c.doRequest(ctx, "GET", path, nil, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	path := fmt.Sprintf("/instrument/%s", symbol)

	resp, err := c.doRequest(ctx, "GET", path, nil, false)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &types.Ticker{
		Symbol:    symbol,
		Exchange: types.ExchangeBitmex,
		LastPrice: c.parseDecimal(result["lastPrice"]),
		BidPrice:  c.parseDecimal(result["bidPrice"]),
		AskPrice:  c.parseDecimal(result["askPrice"]),
		High24h:   c.parseDecimal(result["highPrice"]),
		Low24h:    c.parseDecimal(result["lowPrice"]),
		Volume24h: c.parseDecimal(result["volume"]),
	}, nil
}

func (c *Client) GetOrderBook(ctx context.Context, symbol string, depth int) (*types.OrderBook, error) {
	path := fmt.Sprintf("/orderBook/L2?symbol=%s&depth=%d", symbol, depth)

	resp, err := c.doRequest(ctx, "GET", path, nil, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	bids := make([]types.PriceLevel, 0)
	asks := make([]types.PriceLevel, 0)

	for _, item := range result {
		side, _ := item["side"].(string)
		price := c.parseDecimal(item["price"])
		size := c.parseDecimal(item["size"])

		if side == "Buy" {
			bids = append(bids, types.PriceLevel{Price: price, Quantity: size})
		} else {
			asks = append(asks, types.PriceLevel{Price: price, Quantity: size})
		}
	}

	return &types.OrderBook{
		Symbol:    symbol,
		Exchange:  types.ExchangeBitmex,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}, nil
}

func (c *Client) GetTrades(ctx context.Context, symbol string, count int) ([]*types.Trade, error) {
	path := fmt.Sprintf("/trade?symbol=%s&count=%d", symbol, count)

	resp, err := c.doRequest(ctx, "GET", path, nil, false)
	if err != nil {
		return nil, err
	}

	var result []Trade
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	trades := make([]*types.Trade, len(result))
	for i, t := range result {
		side := types.OrderSideBuy
		if t.Side == "Sell" {
			side = types.OrderSideSell
		}

		trades[i] = &types.Trade{
			Symbol:    symbol,
			Exchange:  types.ExchangeBitmex,
			Side:      side,
			Price:     c.parseDecimalStr(t.Price),
			Quantity:  c.parseDecimalStr(t.Size),
			Timestamp: c.parseTime(t.Timestamp),
		}
	}

	return trades, nil
}

func (c *Client) GetCandles(ctx context.Context, symbol, timeframe string, count int) ([]*types.Candle, error) {
	path := fmt.Sprintf("/trade/bucketed?symbol=%s&binSize=%s&count=%d&partial=false", symbol, timeframe, count)

	resp, err := c.doRequest(ctx, "GET", path, nil, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	candles := make([]*types.Candle, len(result))
	for i, item := range result {
		timestamp, _ := item["timestamp"].(string)
		
		candles[i] = &types.Candle{
			Symbol:    symbol,
			Exchange:  types.ExchangeBitmex,
			Timeframe: timeframe,
			Timestamp: c.parseTime(timestamp),
			Open:      c.parseDecimal(item["open"]),
			High:      c.parseDecimal(item["high"]),
			Low:       c.parseDecimal(item["low"]),
			Close:     c.parseDecimal(item["close"]),
			Volume:    c.parseDecimal(item["volume"]),
		}
	}

	return candles, nil
}

func (c *Client) GetPositions(ctx context.Context, symbol string) ([]*types.Position, error) {
	path := "/position"
	if symbol != "" {
		path += "?symbol=" + symbol
	}

	resp, err := c.doSignedRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Position
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	positions := make([]*types.Position, 0, len(result))
	for _, p := range result {
		if p.Symbol == "" {
			continue
		}

		side := types.PositionSideLong
		if p.CurrentQty < 0 {
			side = types.PositionSideShort
		}

		positions = append(positions, &types.Position{
			Symbol:           p.Symbol,
			Exchange:         types.ExchangeBitmex,
			Side:            side,
			Quantity:        decimal.NewFromInt(int64(p.CurrentQty)),
			AvgEntryPrice:   c.parseDecimalStr(p.AvgCostPrice),
			CurrentPrice:    c.parseDecimalStr(p.MarkPrice),
			UnrealizedPnL:  c.parseDecimalStr(p.UnrealizedPnL),
			Leverage:        c.parseDecimalStr(p.Leverage),
		})
	}

	return positions, nil
}

func (c *Client) GetBalance(ctx context.Context) (map[string]*types.Balance, error) {
	resp, err := c.doSignedRequest(ctx, "GET", "/user/margin", nil)
	if err != nil {
		return nil, err
	}

	var result Balance
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return map[string]*types.Balance{
		result.Asset: {
			Asset:    result.Asset,
			Total:   c.parseDecimalStr(result.WalletBal),
			Free:    c.parseDecimalStr(result.AvailBal),
			Exchange: types.ExchangeBitmex,
		},
	}, nil
}

func (c *Client) PlaceOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	side := "Buy"
	if order.Side == types.OrderSideSell {
		side = "Sell"
	}

	orderType := "Limit"
	if order.Type == types.OrderTypeMarket {
		orderType = "Market"
	}

	body := map[string]any{
		"symbol": order.Symbol,
		"side":   side,
		"orderQty": order.Quantity.String(),
		"ordType": orderType,
	}

	if !order.Price.IsZero() {
		body["price"] = order.Price.String()
	}

	resp, err := c.doSignedRequest(ctx, "POST", "/order", body)
	if err != nil {
		return nil, err
	}

	var result Order
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	order.ID = types.UUIDFromString(fmt.Sprintf("%d", result.OrderID))
	order.Status = c.parseOrderStatus(result.Status)

	return order, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	path := fmt.Sprintf("/order?orderID=%s", orderID)
	_, err := c.doSignedRequest(ctx, "DELETE", path, nil)
	return err
}

func (c *Client) GetOrders(ctx context.Context, symbol string) ([]*types.Order, error) {
	path := "/order"
	if symbol != "" {
		path += "?symbol=" + symbol
	}

	resp, err := c.doSignedRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Order
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	orders := make([]*types.Order, len(result))
	for i, o := range result {
		side := types.OrderSideBuy
		if o.Side == "Sell" {
			side = types.OrderSideSell
		}

		orders[i] = &types.Order{
			ID:           types.UUIDFromString(fmt.Sprintf("%d", o.OrderID)),
			Symbol:       o.Symbol,
			Exchange:     types.ExchangeBitmex,
			Side:        side,
			Price:       c.parseDecimalStr(o.Price),
			Quantity:     c.parseDecimalStr(o.Size),
			Status:      c.parseOrderStatus(o.Status),
		}
	}

	return orders, nil
}

func (c *Client) GetFunding(ctx context.Context, symbol string) ([]*Funding, error) {
	path := fmt.Sprintf("/funding?symbol=%s", symbol)

	resp, err := c.doRequest(ctx, "GET", path, nil, false)
	if err != nil {
		return nil, err
	}

	var result []Funding
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	fundings := make([]*Funding, len(result))
	for i, f := range result {
		fundings[i] = &f
	}

	return fundings, nil
}

func (c *Client) Subscribe(channel, symbol string) error {
	req := Request{
		Op:   "subscribe",
		Args: []string{fmt.Sprintf("%s:%s", channel, symbol)},
	}

	data, _ := json.Marshal(req)
	return c.wsConn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) Connect() error {
	conn, _, err := c.wsClient.Dial(c.wsURL, nil)
	if err != nil {
		return err
	}
	c.wsConn = conn
	return nil
}

func (c *Client) Close() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body map[string]any, signed bool) ([]byte, error) {
	url := c.baseURL + path
	
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
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

func (c *Client) doSignedRequest(ctx context.Context, method, path string, body map[string]any) ([]byte, error) {
	url := c.baseURL + path
	expires := time.Now().Unix() + 60

	var postBody string
	if body != nil {
		data, _ := json.Marshal(body)
		postBody = string(data)
	}

	signature := c.sign(method, path+postBody, expires)

	var reqBody io.Reader
	if postBody != "" {
		reqBody = strings.NewReader(postBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-signature", signature)
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("api-expires", strconv.FormatInt(expires, 10))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) sign(message string, path string, expires int64) string {
	signatureMessage := fmt.Sprintf("%s%s%d", message, path, expires)
	h := hmac.New(sha256.New, []byte(c.apiSecret))
	h.Write([]byte(signatureMessage))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) parseDecimal(val any) decimal.Decimal {
	if val == nil {
		return decimal.Zero
	}
	switch v := val.(type) {
	case float64:
		return decimal.NewFromFloat(v)
	case string:
		d, _ := decimal.NewFromString(v)
		return d
	default:
		return decimal.Zero
	}
}

func (c *Client) parseDecimalStr(val string) decimal.Decimal {
	if val == "" {
		return decimal.Zero
	}
	d, _ := decimal.NewFromString(val)
	return d
}

func (c *Client) parseTime(val string) time.Time {
	if val == "" {
		return time.Now()
	}
	t, _ := time.Parse(time.RFC3339, val)
	return t
}

func (c *Client) parseOrderStatus(status string) types.OrderStatus {
	switch status {
	case "New":
		return types.OrderStatusOpen
	case "Filled":
		return types.OrderStatusFilled
	case "PartiallyFilled":
		return types.OrderStatusPartiallyFilled
	case "Canceled":
		return types.OrderStatusCancelled
	case "Rejected":
		return types.OrderStatusRejected
	default:
		return types.OrderStatusPending
	}
}

func init() {
	logger.Debug("BitMEX exchange adapter initialized")
}
