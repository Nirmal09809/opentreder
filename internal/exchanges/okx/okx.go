package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	passphrase    string
	testnet       bool
	baseURL       string
	wsURL         string
	httpClient    *http.Client
	wsClient      *websocket.Dialer
	subscriptions map[string]chan<- types.Ticker
	mu            sync.RWMutex
	account       *types.Account
	positions     map[string]*types.Position
	orders        map[string]*types.Order
	rateLimiter   *RateLimiter
}

type Config struct {
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Passphrase string `json:"passphrase"`
	Testnet    bool   `json:"testnet"`
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
	baseURL := "https://www.okx.com"
	wsURL := "wss://ws.okx.com:8443/ws/v5/public"

	if cfg.Testnet {
		baseURL = "https://www.okx.com"
		wsURL = "wss://ws.okx.com:8443/ws/v5/public"
	}

	return &Client{
		apiKey:        cfg.APIKey,
		apiSecret:     cfg.APISecret,
		passphrase:    cfg.Passphrase,
		testnet:       cfg.Testnet,
		baseURL:       baseURL,
		wsURL:         wsURL,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		wsClient:      &websocket.Dialer{},
		subscriptions: make(map[string]chan<- types.Ticker),
		positions:     make(map[string]*types.Position),
		orders:        make(map[string]*types.Order),
		rateLimiter:   NewRateLimiter(600),
	}
}

func (c *Client) GetInstruments(ctx context.Context, instType string) ([]*Instrument, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v5/market/instruments?instType=%s", instType), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []*Instrument `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *Client) GetTicker(ctx context.Context, instId string) (*TickerData, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v5/market/ticker?instId=%s", instId), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []TickerData `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 {
		return &result.Data[0], nil
	}
	return nil, fmt.Errorf("no ticker data found")
}

func (c *Client) GetOrderBook(ctx context.Context, instId string, depth int) (*types.OrderBook, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/api/v5/market/books?instId=%s&sz=%d", instId, depth), nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []OrderBookData `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 {
		return c.parseOrderBook(&result.Data[0]), nil
	}
	return nil, fmt.Errorf("no order book data found")
}

func (c *Client) GetCandles(ctx context.Context, instId, after, before, bar string, limit int) ([]*types.Candle, error) {
	path := fmt.Sprintf("/api/v5/market/candles?instId=%s&bar=%s&limit=%d", instId, bar, limit)
	if after != "" {
		path += "&after=" + after
	}
	if before != "" {
		path += "&before=" + before
	}

	resp, err := c.doRequest(ctx, "GET", path, nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data [][]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	candles := make([]*types.Candle, len(result.Data))
	for i, d := range result.Data {
		ts, _ := strconv.ParseInt(d[0], 10, 64)
		open, _ := decimal.NewFromString(d[1])
		high, _ := decimal.NewFromString(d[2])
		low, _ := decimal.NewFromString(d[3])
		close, _ := decimal.NewFromString(d[4])
		vol, _ := decimal.NewFromString(d[5])

		candles[i] = &types.Candle{
			Symbol:    instId,
			Exchange:  types.ExchangeOKX,
			Timeframe: bar,
			Timestamp: time.UnixMilli(ts),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    vol,
		}
	}

	return candles, nil
}

func (c *Client) PlaceOrder(ctx context.Context, params *PlaceOrderParams) (*Order, error) {
	body, _ := json.Marshal(params)

	resp, err := c.signedRequest(ctx, "POST", "/api/v5/trade/order", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []Order `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 {
		return &result.Data[0], nil
	}
	return nil, fmt.Errorf("order placement failed")
}

func (c *Client) CancelOrder(ctx context.Context, instId, ordId string) error {
	params := map[string]string{
		"instId": instId,
		"ordId":  ordId,
	}
	body, _ := json.Marshal(params)

	_, err := c.signedRequest(ctx, "POST", "/api/v5/trade/cancel-order", body)
	return err
}

func (c *Client) GetOrder(ctx context.Context, instId, ordId string) (*Order, error) {
	path := fmt.Sprintf("/api/v5/trade/order?instId=%s&ordId=%s", instId, ordId)

	resp, err := c.signedRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []Order `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 {
		return &result.Data[0], nil
	}
	return nil, fmt.Errorf("order not found")
}

func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	resp, err := c.signedRequest(ctx, "GET", "/api/v5/account/balance", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []Account `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 {
		return &result.Data[0], nil
	}
	return nil, fmt.Errorf("account not found")
}

func (c *Client) GetPositions(ctx context.Context) ([]*Position, error) {
	resp, err := c.signedRequest(ctx, "GET", "/api/v5/account/positions", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []*Position `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *Client) SubscribeTicker(instId string, handler func(*types.Ticker)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	channel := fmt.Sprintf("tickers.%s", instId)
	c.subscriptions[channel] = make(chan types.Ticker, 100)

	return nil
}

func (c *Client) StartWebSocket(ctx context.Context) error {
	conn, _, err := c.wsClient.Dial(c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}
	defer conn.Close()

	subs := make([]map[string]interface{}, 0)
	for channel := range c.subscriptions {
		parts := strings.Split(channel, ".")
		if len(parts) >= 2 {
			subs = append(subs, map[string]interface{}{
				"channel": parts[0],
				"instId":  parts[1],
			})
		}
	}

	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": subs,
	}
	if err := conn.WriteJSON(subMsg); err != nil {
		return err
	}

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
	var data struct {
		Data []TickerData `json:"data"`
		Arg  struct {
			Channel string `json:"channel"`
		} `json:"arg"`
	}
	if err := json.Unmarshal(msg, &data); err != nil {
		return
	}

	if len(data.Data) > 0 {
		ticker := c.parseTicker(&data.Data[0])
		channel := fmt.Sprintf("%s.%s", data.Arg.Channel, data.Data[0].InstId)
		if ch, ok := c.subscriptions[channel]; ok {
			ch <- *ticker
		}
	}
}

func (c *Client) parseTicker(t *TickerData) *types.Ticker {
	lastPrice, _ := decimal.NewFromString(t.Last)
	bidPrice, _ := decimal.NewFromString(t.BidPx)
	askPrice, _ := decimal.NewFromString(t.AskPx)
	vol24h, _ := decimal.NewFromString(t.Vol24h)
	high24h, _ := decimal.NewFromString(t.High24h)
	low24h, _ := decimal.NewFromString(t.Low24h)
	open24h, _ := decimal.NewFromString(t.Open24h)
	
	change := lastPrice.Sub(open24h)
	changePct := change.Div(open24h).Mul(decimal.NewFromInt(100))

	return &types.Ticker{
		Symbol:          t.InstId,
		Exchange:       types.ExchangeOKX,
		LastPrice:       lastPrice,
		BidPrice:        bidPrice,
		AskPrice:        askPrice,
		Volume24h:       vol24h,
		High24h:         high24h,
		Low24h:          low24h,
		PriceChange:     change,
		PriceChangePct:  changePct,
		Timestamp:       time.Now(),
	}
}

func (c *Client) parseOrderBook(data *OrderBookData) *types.OrderBook {
	bids := make([]types.PriceLevel, len(data.Bids))
	for i, b := range data.Bids {
		price, _ := decimal.NewFromString(b[0])
		qty, _ := decimal.NewFromString(b[1])
		bids[i] = types.PriceLevel{Price: price, Quantity: qty}
	}

	asks := make([]types.PriceLevel, len(data.Asks))
	for i, a := range data.Asks {
		price, _ := decimal.NewFromString(a[0])
		qty, _ := decimal.NewFromString(a[1])
		asks[i] = types.PriceLevel{Price: price, Quantity: qty}
	}

	return &types.OrderBook{
		Symbol:    data.InstId,
		Exchange:  types.ExchangeOKX,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}
}

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

	return c.httpClient.Do(req)
}

func (c *Client) signedRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	c.rateLimiter.Wait()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	message := timestamp + method + path

	var bodyStr string
	if body != nil {
		bodyStr = string(body)
		message += bodyStr
	}

	signature := c.sign(message)

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", c.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.passphrase)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) sign(message string) string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ch := range c.subscriptions {
		close(ch)
	}
	c.subscriptions = nil
}

type Instrument struct {
	InstId    string `json:"instId"`
	InstType  string `json:"instType"`
	BaseCcy   string `json:"baseCcy"`
	QuoteCcy  string `json:"quoteCcy"`
	MinSz     string `json:"minSz"`
	LotSz     string `json:"lotSz"`
	PriceStep string `json:"priceStep"`
}

type TickerData struct {
	InstId   string `json:"instId"`
	Last     string `json:"last"`
	LastSz   string `json:"lastSz"`
	AskPx    string `json:"askPx"`
	AskSz    string `json:"askSz"`
	BidPx    string `json:"bidPx"`
	BidSz    string `json:"bidSz"`
	Open24h  string `json:"open24h"`
	High24h  string `json:"high24h"`
	Low24h   string `json:"low24h"`
	Vol24h   string `json:"vol24h"`
	Ts       string `json:"ts"`
}

type OrderBookData struct {
	InstId string   `json:"instId"`
	Bids   [][]string `json:"bids"`
	Asks   [][]string `json:"asks"`
	Ts     string `json:"ts"`
}

type OrderBook struct {
	Symbol    string
	Exchange  types.Exchange
	Bids      []types.PriceLevel
	Asks      []types.PriceLevel
	Timestamp time.Time
}

type Order struct {
	OrdId     string `json:"ordId"`
	ClOrdId   string `json:"clOrdId"`
	InstId    string `json:"instId"`
	Side      string `json:"side"`
	OrdType   string `json:"ordType"`
	Sz        string `json:"sz"`
	Px        string `json:"px"`
	State     string `json:"state"`
	AvgPx     string `json:"avgPx"`
	FilledSz  string `json:"filledSz"`
	Fee       string `json:"fee"`
	Ts        string `json:"ts"`
}

type PlaceOrderParams struct {
	InstId  string `json:"instId"`
	TdMode  string `json:"tdMode"`
	Side    string `json:"side"`
	OrdType string `json:"ordType"`
	Sz      string `json:"sz"`
	Px      string `json:"px,omitempty"`
	SlOrdPx string `json:"slOrdPx,omitempty"`
	SlTriggerPx string `json:"slTriggerPx,omitempty"`
	TpTriggerPx  string `json:"tpTriggerPx,omitempty"`
	TpOrdPx      string `json:"tpOrdPx,omitempty"`
}

type Account struct {
	UTime    string      `json:"uTime"`
	TotalEq  string      `json:"totalEq"`
	AdjEq    string      `json:"adjEq"`
	IsoEq    string      `json:"isoEq"`
	OrdFroz  string      `json:"ordFroz"`
	MgnRatio string      `json:"mgnRatio"`
	MgnMode  string      `json:"mgnMode"`
	NotionalLeave string `json:"notionalLeave"`
	Greeks   string      `json:"greeks"`
	Balances []struct {
		Ccy  string `json:"ccy"`
		AvailBal string `json:"availBal"`
		CashBal string `json:"cashBal"`
		FrozenBal string `json:"frozenBal"`
		OrdFrozen string `json:"ordFrozen"`
	} `json:"balances"`
}

type Position struct {
	InstId     string `json:"instId"`
	MgnMode    string `json:"mgnMode"`
	PosSide    string `json:"posSide"`
	Pos        string `json:"pos"`
	AvailPos   string `json:"availPos"`
	NotionalUsd string `json:"notionalUsd"`
	Upl         string `json:"upl"`
	UplRatio    string `json:"uplRatio"`
	AvgPx       string `json:"avgPx"`
		Lever       string `json:"leverage"`
	LiqPx       string `json:"liqPx"`
	Imr         string `json:"imr"`
	Mmr         string `json:"mmr"`
	Fee         string `json:"fee"`
	FundFee     string `json:"fundFee"`
	SpotIn PosBase `json:"spotInPosBase"`
	SpotPx      string `json:"spotPx"`
	BaseBoggy   string `json:"baseBoggy"`
	QuoteBoggy  string `json:"quoteBoggy"`
}

type PosBase struct {
	Pos   string `json:"pos"`
	Ccy   string `json:"ccy"`
	Cost  string `json:"cost"`
	CostBoggy string `json:"costBoggy"`
}
