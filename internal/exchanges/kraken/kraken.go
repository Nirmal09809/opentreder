package kraken

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
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
	baseURL       string
	wsURL         string
	httpClient    *http.Client
	wsClient      *websocket.Dialer
	subscriptions map[string]chan interface{}
	handlers      map[string][]interface{}
	mu            sync.RWMutex
	nonce        int64
}

type Config struct {
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Sandbox    bool   `json:"sandbox"`
}

type TickerResponse struct {
	PairName string `json:"pair"`
	Close    []interface{} `json:"c"`
	Open     []interface{} `json:"o"`
	High     []interface{} `json:"h"`
	Low      []interface{} `json:"l"`
	Volume   []interface{} `json:"v"`
	VWAP     []interface{} `json:"p"`
	Trades   []interface{} `json:"t"`
	Spread   []interface{} `json:"b"`
	TSpread  []interface{} `json:"bs"`
}

type OHLCResponse struct {
	PairName string     `json:"pair"`
	OHLC     [][]string `json:"ohlc"`
}

type OrderBookResponse struct {
	PairName string `json:"pair"`
	Bids     [][]interface{} `json:"bs"`
	Asks     [][]interface{} `json:"as"`
}

type TradesResponse struct {
	PairName string      `json:"pair"`
	Trades   [][]interface{} `json:"trades"`
}

type BalanceResponse struct {
	Result map[string]string `json:"result"`
}

type OpenOrdersResponse struct {
	Result struct {
		Open map[string]OpenOrder `json:"open"`
	} `json:"result"`
}

type OpenOrder struct {
	Descr    OrderDescription `json:"descr"`
	Vol      string `json:"vol"`
	VolExec  string `json:"vol_exec"`
	Status   string `json:"status"`
	Opentm   float64 `json:"opentm"`
	Expiretm float64 `json:"expiretm"`
}

type OrderDescription struct {
	Pair   string `json:"pair"`
	Type   string `json:"type"`
	Side   string `json:"side"`
	Price  string `json:"price"`
	Price2 string `json:"price2"`
	Leverage string `json:"leverage"`
	Order  string `json:"order"`
}

type AddOrderResponse struct {
	Result struct {
		TxID []string `json:"txid"`
	} `json:"result"`
}

type CancelOrderResponse struct {
	Result struct {
		Count int64 `json:"count"`
	} `json:"result"`
}

type WSMessage struct {
	Event     string          `json:"event"`
	ChannelID int            `json:"channelID,omitempty"`
	Channel   string         `json:"channel,omitempty"`
	Pair      string         `json:"pair,omitempty"`
	Status    string         `json:"status,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type WSTickerData struct {
	ChannelID int      `json:"channelID"`
	Pair      string   `json:"pair"`
	Bid       float64  `json:"b"`
	BAsk      float64  `json:"a"`
	Close     []string `json:"c"`
	Volume    []string `json:"v"`
	VWAP      []string `json:"p"`
	Trades    []int    `json:"t"`
	Open      []string `json:"o"`
	High      []string `json:"h"`
	Low       []string `json:"l"`
	Today     string   `json:"today"`
}

type WSOrderBookData struct {
	ChannelID int         `json:"channelID"`
	Pair      string      `json:"pair"`
	Bs        [][]string  `json:"bs,omitempty"`
	As        [][]string  `json:"as,omitempty"`
	B         [][]string  `json:"b,omitempty"`
	A         [][]string  `json:"a,omitempty"`
}

func NewClient(cfg Config) *Client {
	baseURL := "https://api.kraken.com"
	wsURL := "wss://ws.kraken.com"

	if cfg.Sandbox {
		baseURL = "https://demo-futures.kraken.com"
		wsURL = "wss://demo-futures.kraken.com/ws/v1/public/signal"
	}

	return &Client{
		apiKey:        cfg.APIKey,
		apiSecret:     cfg.APISecret,
		baseURL:       baseURL,
		wsURL:         wsURL,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		wsClient:      &websocket.Dialer{},
		subscriptions: make(map[string]chan interface{}),
		handlers:      make(map[string][]interface{}),
	}
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.doRequest(ctx, "POST", "/0/public/Ping", nil, false)
	return err
}

func (c *Client) GetServerTime(ctx context.Context) (time.Time, error) {
	resp, err := c.doRequest(ctx, "POST", "/0/public/Time", nil, false)
	if err != nil {
		return time.Time{}, err
	}

	var result struct {
		Result struct {
			Unixtime int64 `json:"unixtime"`
			Rfc1123  string `json:"rfc1123"`
		} `json:"result"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return time.Time{}, err
	}

	return time.Unix(result.Result.Unixtime, 0), nil
}

func (c *Client) GetTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	pair := c.formatSymbol(symbol)
	resp, err := c.doRequest(ctx, "POST", "/0/public/Ticker", map[string]string{"pair": pair}, false)
	if err != nil {
		return nil, err
	}

	var result map[string]TickerResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	for _, ticker := range result {
		closePrice, _ := decimal.NewFromString(ticker.Close[0].(string))
		openPrice, _ := decimal.NewFromString(ticker.Open[0].(string))
		highPrice, _ := decimal.NewFromString(ticker.High[1].(string))
		lowPrice, _ := decimal.NewFromString(ticker.Low[1].(string))
		volume, _ := decimal.NewFromString(ticker.Volume[1].(string))
		bidPrice, _ := decimal.NewFromString(ticker.Spread[0].(string))
		askPrice, _ := decimal.NewFromString(ticker.Spread[1].(string))

		priceChange := closePrice.Sub(openPrice)
		priceChangePct := decimal.Zero
		if !openPrice.IsZero() {
			priceChangePct = priceChange.Div(openPrice).Mul(decimal.NewFromInt(100))
		}

		return &types.Ticker{
			Symbol:         symbol,
			Exchange:       types.ExchangeKraken,
			LastPrice:      closePrice,
			BidPrice:       bidPrice,
			AskPrice:       askPrice,
			Volume24h:      volume,
			High24h:        highPrice,
			Low24h:         lowPrice,
			PriceChange:     priceChange,
			PriceChangePct: priceChangePct,
			Timestamp:      time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("no ticker data found for %s", symbol)
}

func (c *Client) GetKlines(ctx context.Context, symbol string, interval string, since int64) ([]*types.Candle, error) {
	pair := c.formatSymbol(symbol)
	reqParams := map[string]string{
		"pair":    pair,
		"interval": interval,
	}
	if since > 0 {
		reqParams["since"] = strconv.FormatInt(since, 10)
	}

	resp, err := c.doRequest(ctx, "POST", "/0/public/OHLC", reqParams, false)
	if err != nil {
		return nil, err
	}

	var result map[string]OHLCResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var candles []*types.Candle
	for _, data := range result {
		for _, ohlc := range data.OHLC {
			if len(ohlc) < 8 {
				continue
			}

			timeUnix, _ := strconv.ParseInt(ohlc[0], 10, 64)
			open, _ := decimal.NewFromString(ohlc[1])
			high, _ := decimal.NewFromString(ohlc[2])
			low, _ := decimal.NewFromString(ohlc[3])
			close, _ := decimal.NewFromString(ohlc[4])
			volume, _ := decimal.NewFromString(ohlc[6])

			candles = append(candles, &types.Candle{
				Symbol:    symbol,
				Exchange:  types.ExchangeKraken,
				Timeframe: interval,
				Timestamp: time.Unix(timeUnix, 0),
				Open:      open,
				High:      high,
				Low:       low,
				Close:     close,
				Volume:    volume,
			})
		}
	}

	return candles, nil
}

func (c *Client) GetOrderBook(ctx context.Context, symbol string, depth int) (*types.OrderBook, error) {
	pair := c.formatSymbol(symbol)
	reqParams := map[string]string{
		"pair": pair,
		"count": strconv.Itoa(depth),
	}

	resp, err := c.doRequest(ctx, "POST", "/0/public/Depth", reqParams, false)
	if err != nil {
		return nil, err
	}

	var result map[string]OrderBookResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	for _, ob := range result {
		bids := make([]types.PriceLevel, len(ob.Bids))
		for i, bid := range ob.Bids {
			price, _ := decimal.NewFromString(bid[0].(string))
			qty, _ := decimal.NewFromString(bid[1].(string))
			bids[i] = types.PriceLevel{Price: price, Quantity: qty}
		}

		asks := make([]types.PriceLevel, len(ob.Asks))
		for i, ask := range ob.Asks {
			price, _ := decimal.NewFromString(ask[0].(string))
			qty, _ := decimal.NewFromString(ask[1].(string))
			asks[i] = types.PriceLevel{Price: price, Quantity: qty}
		}

		return &types.OrderBook{
			Symbol:    symbol,
			Exchange:  types.ExchangeKraken,
			Bids:      bids,
			Asks:      asks,
			Timestamp: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("no order book data found")
}

func (c *Client) GetTrades(ctx context.Context, symbol string, since int64) ([]*types.Trade, error) {
	pair := c.formatSymbol(symbol)
	reqParams := map[string]string{
		"pair": pair,
	}
	if since > 0 {
		reqParams["since"] = strconv.FormatInt(since, 10)
	}

	resp, err := c.doRequest(ctx, "POST", "/0/public/Trades", reqParams, false)
	if err != nil {
		return nil, err
	}

	var result map[string]TradesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var trades []*types.Trade
	for _, data := range result {
		for _, t := range data.Trades {
			if len(t) < 4 {
				continue
			}

			price, _ := decimal.NewFromString(t[0].(string))
			volume, _ := decimal.NewFromString(t[1].(string))
			timeUnix, _ := strconv.ParseFloat(t[2].(string), 64)
			side := t[3].(string)

			tradeSide := types.OrderSideBuy
			if side == "s" {
				tradeSide = types.OrderSideSell
			}

			trades = append(trades, &types.Trade{
				Symbol:    symbol,
				Exchange:  types.ExchangeKraken,
				Side:      tradeSide,
				Price:     price,
				Quantity:  volume,
				Timestamp: time.Unix(int64(timeUnix), 0),
			})
		}
	}

	return trades, nil
}

func (c *Client) GetBalance(ctx context.Context) (map[string]*types.Balance, error) {
	resp, err := c.signedRequest(ctx, "/0/private/Balance", nil)
	if err != nil {
		return nil, err
	}

	var result BalanceResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	balances := make(map[string]*types.Balance)
	for asset, balance := range result.Result {
		balanceDec, _ := decimal.NewFromString(balance)
		if balanceDec.IsPositive() {
			balances[asset] = &types.Balance{
				Asset:    asset,
				Free:     balanceDec,
				Locked:   decimal.Zero,
				Total:    balanceDec,
				Exchange: types.ExchangeKraken,
			}
		}
	}

	return balances, nil
}

func (c *Client) GetOpenOrders(ctx context.Context) ([]*types.Order, error) {
	resp, err := c.signedRequest(ctx, "/0/private/OpenOrders", nil)
	if err != nil {
		return nil, err
	}

	var result OpenOrdersResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	var orders []*types.Order
	for _, order := range result.Result.Open {
		symbol := c.parsePair(order.Descr.Pair)
		price, _ := decimal.NewFromString(order.Descr.Price)

		side := types.OrderSideBuy
		if order.Descr.Side == "sell" {
			side = types.OrderSideSell
		}

		orderType := types.OrderTypeLimit
		if strings.Contains(order.Descr.Order, "market") {
			orderType = types.OrderTypeMarket
		}

		orders = append(orders, &types.Order{
			Symbol:      symbol,
			Exchange:    types.ExchangeKraken,
			Side:        side,
			Type:        orderType,
			Price:       price,
			Quantity:    decimal.RequireFromString(order.Vol),
			CreatedAt:   time.Unix(int64(order.Opentm), 0),
			Status:      types.OrderStatusOpen,
		})
	}

	return orders, nil
}

func (c *Client) PlaceOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	params := map[string]string{
		"pair":      c.formatSymbol(order.Symbol),
		"type":      string(order.Side),
		"ordertype": string(order.Type),
		"volume":    order.Quantity.String(),
	}

	if !order.Price.IsZero() {
		params["price"] = order.Price.String()
	}

	if order.TimeInForce != "" {
		params["timeinforce"] = string(order.TimeInForce)
	}

	resp, err := c.signedRequest(ctx, "/0/private/AddOrder", params)
	if err != nil {
		return nil, err
	}

	var result AddOrderResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Result.TxID) > 0 {
		order.ID = types.UUIDFromString(result.Result.TxID[0])
		order.Status = types.OrderStatusPending
	}

	return order, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	params := map[string]string{
		"txid": orderID,
	}

	_, err := c.signedRequest(ctx, "/0/private/CancelOrder", params)
	return err
}

func (c *Client) SubscribeTicker(symbol string, handler func(*types.Ticker)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	pair := c.formatSymbol(symbol)
	channel := make(chan interface{}, 100)
	c.subscriptions["ticker:"+pair] = channel

	go c.handleTickerStream(pair, channel, handler)

	return c.wsSubscribe("ticker", pair)
}

func (c *Client) SubscribeOrderBook(symbol string, handler func(*types.OrderBook)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	pair := c.formatSymbol(symbol)
	channel := make(chan interface{}, 100)
	c.subscriptions["orderbook:"+pair] = channel

	go c.handleOrderBookStream(pair, channel, handler)

	return c.wsSubscribe("book", pair)
}

func (c *Client) wsSubscribe(channel, pair string) error {
	msg := map[string]interface{}{
		"event":   "subscribe",
		"pair":    []string{pair},
		"channel": []string{channel},
	}

	data, _ := json.Marshal(msg)
	return wsConn.WriteMessage(websocket.TextMessage, data)
}

type wsConnection struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsConnection) WriteMessage(messageType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}
	return w.conn.WriteMessage(messageType, data)
}

var wsConn wsConnection

func (c *Client) wsConnect() error {
	conn, _, err := c.wsClient.Dial(c.wsURL, nil)
	if err != nil {
		return err
	}
	wsConn.conn = conn
	return nil
}

func (c *Client) handleTickerStream(pair string, ch chan interface{}, handler func(*types.Ticker)) {
	for {
		select {
		case msg := <-ch:
			if ticker, ok := msg.(*types.Ticker); ok {
				handler(ticker)
			}
		}
	}
}

func (c *Client) handleOrderBookStream(pair string, ch chan interface{}, handler func(*types.OrderBook)) {
	for {
		select {
		case msg := <-ch:
			if ob, ok := msg.(*types.OrderBook); ok {
				handler(ob)
			}
		}
	}
}

func (c *Client) formatSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "/", "")
	
	replacePairs := map[string]string{
		"BTCUSD": "XBT/USD",
		"ETHUSD": "ETH/USD",
		"SOLUSD": "SOL/USD",
		"XRPUSD": "XRP/USD",
	}
	
	if replacement, ok := replacePairs[symbol]; ok {
		return replacement
	}
	
	return symbol
}

func (c *Client) parsePair(pair string) string {
	pair = strings.ReplaceAll(pair, "/", "")
	
	replacePairs := map[string]string{
		"XBT": "BTC",
	}
	
	for old, new := range replacePairs {
		pair = strings.ReplaceAll(pair, old, new)
	}
	
	return pair + "USD"
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
	var postData string
	if params != nil {
		for k, v := range params {
			postData += k + "=" + v + "&"
		}
		postData = strings.TrimSuffix(postData, "&")
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, strings.NewReader(postData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) signedRequest(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	if params == nil {
		params = make(map[string]string)
	}

	c.nonce = time.Now().UnixNano()
	params["nonce"] = strconv.FormatInt(c.nonce, 10)

	var postData string
	for k, v := range params {
		postData += k + "=" + v + "&"
	}
	postData = strings.TrimSuffix(postData, "&")

	path := endpoint
	encoded := base64.StdEncoding.EncodeToString([]byte(postData))

	h := hmac.New(sha512.New, []byte(c.apiSecret))
	h.Write([]byte(path))
	h.Write([]byte(encoded))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, strings.NewReader(postData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("API-Key", c.apiKey)
	req.Header.Set("API-Sign", signature)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func GetAssetPairs(ctx context.Context) (map[string]string, error) {
	client := NewClient(Config{})
	resp, err := client.doRequest(ctx, "POST", "/0/public/AssetPairs", nil, false)
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct {
		Altname string `json:"altname"`
	})

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	pairs := make(map[string]string)
	for k, v := range result {
		pairs[k] = v.Altname
	}

	return pairs, nil
}

func GetServerTime(ctx context.Context) (time.Time, error) {
	client := NewClient(Config{})
	return client.GetServerTime(ctx)
}

func init() {
	logger.Debug("Kraken exchange adapter initialized")
}
