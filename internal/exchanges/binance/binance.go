package binance

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

type Client struct {
	apiKey        string
	apiSecret     string
	testnet       bool
	baseURL       string
	wsURL         string
	recvWindow    time.Duration
	httpClient    *http.Client
	wsClient     *websocketClient
	rateLimiter  *RateLimiter
	account      *Account
	exchangeInfo  *ExchangeInfo
	mu            sync.RWMutex
	subscriptions map[string]bool
}

type Config struct {
	APIKey      string
	APISecret   string
	Testnet     bool
	BaseURL     string
	WSURL       string
	RecvWindow  time.Duration
	RateLimit   int
}

type RateLimiter struct {
	mu       sync.Mutex
	requests []time.Time
	limit    int
	window   time.Duration
}

type Account struct {
	MakerCommission  int
	TakerCommission  int
	BuyerCommission int
	SellerCommission int
	CanTrade       bool
	CanWithdraw    bool
	CanDeposit     bool
	UpdateTime     time.Time
	AccountType    string
	Permissions    []string
	Balances       []Balance
}

type Balance struct {
	Asset         string `json:"asset"`
	Free         string `json:"free"`
	Locked       string `json:"locked"`
}

type ExchangeInfo struct {
	Timezone      string    `json:"timezone"`
	ServerTime    int64     `json:"serverTime"`
	Symbols       []Symbol  `json:"symbols"`
	RateLimits    []RateLimit `json:"rateLimits"`
}

type Symbol struct {
	Symbol           string   `json:"symbol"`
	Status          string   `json:"status"`
	BaseAsset       string   `json:"baseAsset"`
	BaseAssetType   string   `json:"baseAssetPrecision"`
	QuoteAsset      string   `json:"quoteAsset"`
	QuotePrecision  int      `json:"quotePrecision"`
	QuoteAssetPrecision int  `json:"quoteAssetPrecision"`
	OrderTypes      []string `json:"orderTypes"`
	IcebergAllowed  bool     `json:"icebergAllowed"`
	OcoAllowed      bool     `json:"ocoAllowed"`
	QuoteOrderQtyMarketAllowed bool `json:"quoteOrderQtyMarketAllowed"`
	IsSpotTradingAllowed bool   `json:"isSpotTradingAllowed"`
	IsMarginTradingAllowed bool `json:"isMarginTradingAllowed"`
	Filters        []Filter  `json:"filters"`
}

type Filter struct {
	FilterType      string `json:"filterType"`
	MinPrice       string `json:"minPrice"`
	MaxPrice       string `json:"maxPrice"`
	TickSize       string `json:"tickSize"`
	MinQty         string `json:"minQty"`
	MaxQty         string `json:"maxQty"`
	StepSize       string `json:"stepSize"`
	MinNotional    string `json:"minNotional"`
	MinTrailingCashFlow string `json:"minTrailingCashFlow"`
	TrailingDelta  string `json:"trailingDelta"`
	MarketLotSize  string `json:"marketLotSize"`
	MaxNumOrders   string `json:"maxNumOrders"`
	MaxNumAlgoOrders string `json:"maxNumAlgoOrders"`
}

type RateLimit struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
}

type Order struct {
	Symbol           string `json:"symbol"`
	OrderID         int64  `json:"orderId"`
	OrderListID     int64  `json:"orderListId"`
	ClientOrderID   string `json:"clientOrderId"`
	Price           string `json:"price"`
	OrigQty         string `json:"origQty"`
	ExecutedQty     string `json:"executedQty"`
	CummulativeQuoteQty string `json:"cummulativeQuoteQty"`
	Status          string `json:"status"`
	TimeInForce     string `json:"timeInForce"`
	Type            string `json:"type"`
	Side            string `json:"side"`
	StopPrice       string `json:"stopPrice"`
	IcebergQty      string `json:"icebergQty"`
	Time            int64  `json:"time"`
	UpdateTime      int64  `json:"updateTime"`
	IsWorking       bool   `json:"isWorking"`
	OrigQuoteOrderQty string `json:"origQuoteOrderQty"`
}

type Trade struct {
	Symbol           string `json:"symbol"`
	ID              int64  `json:"id"`
	OrderID         int64  `json:"orderId"`
	OrderListID     int64  `json:"orderListId"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Commission       string `json:"commission"`
	CommissionAsset  string `json:"commissionAsset"`
	Time            int64  `json:"time"`
	IsBuyer         bool   `json:"isBuyer"`
	IsMaker         bool   `json:"isMaker"`
	IsBestMatch     bool   `json:"isBestMatch"`
}

type TickerPrice struct {
	Symbol      string `json:"symbol"`
	Price       string `json:"price"`
	Time        int64  `json:"time"`
}

type BookTicker struct {
	Symbol       string `json:"symbol"`
	BidPrice    string `json:"bidPrice"`
	BidQty      string `json:"bidQty"`
	AskPrice    string `json:"askPrice"`
	AskQty      string `json:"askQty"`
}

type Candlestick struct {
	OpenTime         int64   `json:"openTime"`
	Open            string  `json:"open"`
	High            string  `json:"high"`
	Low             string  `json:"low"`
	Close           string  `json:"close"`
	Volume          string  `json:"volume"`
	CloseTime       int64   `json:"closeTime"`
	QuoteVolume     string  `json:"quoteVolume"`
	NumTrades       int     `json:"numTrades"`
	TakerBuyBaseVol string  `json:"takerBuyBaseAssetVolume"`
	TakerBuyQuoteVol string `json:"takerBuyQuoteAssetVolume"`
	Ignore          string  `json:"ignore"`
}

type Depth struct {
	LastUpdateID int64        `json:"lastUpdateId"`
	Bids         [][]string   `json:"bids"`
	Asks         [][]string   `json:"asks"`
}

type websocketClient struct {
	conn     interface{}
	mu       sync.RWMutex
	handlers map[string][]func([]byte)
	wsURL    string
}

type WSTickerHandler func(*types.Ticker)
type WSOrderBookHandler func(*types.OrderBook)
type WSTradeHandler func(*types.Trade)
type WSCandleHandler func(*types.Candle)

func NewClient(cfg Config) (*Client, error) {
	if cfg.RecvWindow == 0 {
		cfg.RecvWindow = 5 * time.Second
	}
	if cfg.RateLimit == 0 {
		cfg.RateLimit = 1200
	}

	baseURL := "https://api.binance.com"
	wsURL := "wss://stream.binance.com:9443/ws"

	if cfg.Testnet {
		baseURL = "https://testnet.binance.vision"
		wsURL = "wss://testnet.binance.vision/ws"
	}

	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	if cfg.WSURL != "" {
		wsURL = cfg.WSURL
	}

	client := &Client{
		apiKey:        cfg.APIKey,
		apiSecret:     cfg.APISecret,
		testnet:       cfg.Testnet,
		baseURL:       baseURL,
		wsURL:         wsURL,
		recvWindow:    cfg.RecvWindow,
		rateLimiter:   newRateLimiter(cfg.RateLimit, time.Second),
		subscriptions: make(map[string]bool),
	}

	client.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	client.wsClient = &websocketClient{
		handlers: make(map[string][]func([]byte)),
		wsURL:    wsURL,
	}

	return client, nil
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

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.request(ctx, "GET", "/api/v3/ping", nil, false)
	return err
}

func (c *Client) GetServerTime(ctx context.Context) (int64, error) {
	resp, err := c.request(ctx, "GET", "/api/v3/time", nil, false)
	if err != nil {
		return 0, err
	}

	var result struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}

	return result.ServerTime, nil
}

func (c *Client) GetExchangeInfo(ctx context.Context) (*ExchangeInfo, error) {
	resp, err := c.request(ctx, "GET", "/api/v3/exchangeInfo", nil, false)
	if err != nil {
		return nil, err
	}

	var info ExchangeInfo
	if err := json.Unmarshal(resp, &info); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.exchangeInfo = &info
	c.mu.Unlock()

	return &info, nil
}

func (c *Client) GetAccount(ctx context.Context) (*Account, error) {
	resp, err := c.request(ctx, "GET", "/api/v3/account", nil, true)
	if err != nil {
		return nil, err
	}

	var account Account
	if err := json.Unmarshal(resp, &account); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.account = &account
	c.mu.Unlock()

	return &account, nil
}

func (c *Client) GetBalance(ctx context.Context) (map[string]*types.Balance, error) {
	account, err := c.GetAccount(ctx)
	if err != nil {
		return nil, err
	}

	balances := make(map[string]*types.Balance)
	for _, b := range account.Balances {
		free, _ := decimal.NewFromString(b.Free)
		locked, _ := decimal.NewFromString(b.Locked)
		total := free.Add(locked)

		if total.GreaterThan(decimal.Zero) {
			balances[b.Asset] = &types.Balance{
				Asset:    b.Asset,
				Free:     free,
				Locked:   locked,
				Total:    total,
				Exchange: types.ExchangeBinance,
			}
		}
	}

	return balances, nil
}

func (c *Client) GetAccountTradeList(ctx context.Context, symbol string, limit int) ([]*Trade, error) {
	params := map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	}

	resp, err := c.request(ctx, "GET", "/api/v3/myTrades", params, true)
	if err != nil {
		return nil, err
	}

	var trades []Trade
	if err := json.Unmarshal(resp, &trades); err != nil {
		return nil, err
	}

	result := make([]*Trade, len(trades))
	for i := range trades {
		result[i] = &trades[i]
	}

	return result, nil
}

func (c *Client) CreateOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	params := map[string]string{
		"symbol":            order.Symbol,
		"side":              string(order.Side),
		"type":              string(order.Type),
		"quantity":          order.Quantity.String(),
		"newClientOrderId":  order.ID.String(),
	}

	if !order.Price.IsZero() {
		params["price"] = order.Price.String()
	}

	if !order.StopPrice.IsZero() {
		params["stopPrice"] = order.StopPrice.String()
	}

	if order.TimeInForce != "" {
		params["timeInForce"] = string(order.TimeInForce)
	}

	if order.Type == types.OrderTypeIceberg {
		params["icebergQty"] = order.Quantity.String()
	}

	resp, err := c.request(ctx, "POST", "/api/v3/order", params, true)
	if err != nil {
		return nil, err
	}

	var binanceOrder Order
	if err := json.Unmarshal(resp, &binanceOrder); err != nil {
		return nil, err
	}

	return c.convertToOrder(&binanceOrder, order), nil
}

func (c *Client) GetOrder(ctx context.Context, symbol string, orderID int64) (*types.Order, error) {
	params := map[string]string{
		"symbol":  symbol,
		"orderId": strconv.FormatInt(orderID, 10),
	}

	resp, err := c.request(ctx, "GET", "/api/v3/order", params, true)
	if err != nil {
		return nil, err
	}

	var binanceOrder Order
	if err := json.Unmarshal(resp, &binanceOrder); err != nil {
		return nil, err
	}

	return c.orderToType(&binanceOrder), nil
}

func (c *Client) GetOpenOrders(ctx context.Context, symbol string) ([]*types.Order, error) {
	params := map[string]string{}
	if symbol != "" {
		params["symbol"] = symbol
	}

	resp, err := c.request(ctx, "GET", "/api/v3/openOrders", params, true)
	if err != nil {
		return nil, err
	}

	var binanceOrders []Order
	if err := json.Unmarshal(resp, &binanceOrders); err != nil {
		return nil, err
	}

	orders := make([]*types.Order, len(binanceOrders))
	for i := range binanceOrders {
		orders[i] = c.orderToType(&binanceOrders[i])
	}

	return orders, nil
}

func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	params := map[string]string{
		"symbol":  symbol,
		"orderId": strconv.FormatInt(orderID, 10),
	}

	_, err := c.request(ctx, "DELETE", "/api/v3/order", params, true)
	return err
}

func (c *Client) CancelAllOpenOrders(ctx context.Context, symbol string) error {
	orders, err := c.GetOpenOrders(ctx, symbol)
	if err != nil {
		return err
	}

	for _, order := range orders {
		if err := c.CancelOrder(ctx, symbol, order.ID.String()); err != nil {
			logger.Warn("Failed to cancel order", "order_id", order.ID, "error", err)
		}
	}

	return nil
}

func (c *Client) GetAllOrders(ctx context.Context, symbol string, limit int) ([]*types.Order, error) {
	params := map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	}

	resp, err := c.request(ctx, "GET", "/api/v3/allOrders", params, true)
	if err != nil {
		return nil, err
	}

	var binanceOrders []Order
	if err := json.Unmarshal(resp, &binanceOrders); err != nil {
		return nil, err
	}

	orders := make([]*types.Order, len(binanceOrders))
	for i := range binanceOrders {
		orders[i] = c.orderToType(&binanceOrders[i])
	}

	return orders, nil
}

func (c *Client) GetTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	params := map[string]string{
		"symbol": symbol,
	}

	resp, err := c.request(ctx, "GET", "/api/v3/ticker/24hr", params, false)
	if err != nil {
		return nil, err
	}

	var ticker struct {
		Symbol             string `json:"symbol"`
		PriceChange        string `json:"priceChange"`
		PriceChangePercent string `json:"priceChangePercent"`
		LastPrice         string `json:"lastPrice"`
		BidPrice          string `json:"bidPrice"`
		BidQty            string `json:"bidQty"`
		AskPrice          string `json:"askPrice"`
		AskQty            string `json:"askQty"`
		Volume            string `json:"volume"`
		QuoteVolume       string `json:"quoteVolume"`
		HighPrice         string `json:"highPrice"`
		LowPrice          string `json:"lowPrice"`
	}

	if err := json.Unmarshal(resp, &ticker); err != nil {
		return nil, err
	}

	return &types.Ticker{
		Symbol:            ticker.Symbol,
		Exchange:          types.ExchangeBinance,
		LastPrice:         parseDecimal(ticker.LastPrice),
		BidPrice:         parseDecimal(ticker.BidPrice),
		BidQty:           parseDecimal(ticker.BidQty),
		AskPrice:         parseDecimal(ticker.AskPrice),
		AskQty:           parseDecimal(ticker.AskQty),
		Volume24h:        parseDecimal(ticker.Volume),
		QuoteVolume24h:   parseDecimal(ticker.QuoteVolume),
		High24h:          parseDecimal(ticker.HighPrice),
		Low24h:           parseDecimal(ticker.LowPrice),
		PriceChange24h:   parseDecimal(ticker.PriceChange),
		PriceChangePct24h: parseDecimal(ticker.PriceChangePercent),
		Timestamp:        time.Now(),
	}, nil
}

func (c *Client) GetTickers(ctx context.Context) ([]*types.Ticker, error) {
	resp, err := c.request(ctx, "GET", "/api/v3/ticker/24hr", nil, false)
	if err != nil {
		return nil, err
	}

	var tickers []struct {
		Symbol             string `json:"symbol"`
		PriceChange        string `json:"priceChange"`
		PriceChangePercent string `json:"priceChangePercent"`
		LastPrice         string `json:"lastPrice"`
		BidPrice          string `json:"bidPrice"`
		AskPrice          string `json:"askPrice"`
		Volume            string `json:"volume"`
		QuoteVolume       string `json:"quoteVolume"`
		HighPrice         string `json:"highPrice"`
		LowPrice          string `json:"lowPrice"`
	}

	if err := json.Unmarshal(resp, &tickers); err != nil {
		return nil, err
	}

	result := make([]*types.Ticker, len(tickers))
	for i, t := range tickers {
		result[i] = &types.Ticker{
			Symbol:            t.Symbol,
			Exchange:          types.ExchangeBinance,
			LastPrice:        parseDecimal(t.LastPrice),
			BidPrice:         parseDecimal(t.BidPrice),
			AskPrice:         parseDecimal(t.AskPrice),
			Volume24h:        parseDecimal(t.Volume),
			QuoteVolume24h:   parseDecimal(t.QuoteVolume),
			High24h:          parseDecimal(t.HighPrice),
			Low24h:           parseDecimal(t.LowPrice),
			PriceChange24h:   parseDecimal(t.PriceChange),
			PriceChangePct24h: parseDecimal(t.PriceChangePercent),
			Timestamp:        time.Now(),
		}
	}

	return result, nil
}

func (c *Client) GetOrderBook(ctx context.Context, symbol string, limit int) (*types.OrderBook, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	params := map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	}

	resp, err := c.request(ctx, "GET", "/api/v3/depth", params, false)
	if err != nil {
		return nil, err
	}

	var depth Depth
	if err := json.Unmarshal(resp, &depth); err != nil {
		return nil, err
	}

	book := &types.OrderBook{
		Symbol:    symbol,
		Exchange:  types.ExchangeBinance,
		Timestamp: time.Now(),
		Bids:      make([]types.OrderBookLevel, len(depth.Bids)),
		Asks:      make([]types.OrderBookLevel, len(depth.Asks)),
	}

	for i, bid := range depth.Bids {
		book.Bids[i] = types.OrderBookLevel{
			Price:    parseDecimal(bid[0]),
			Quantity: parseDecimal(bid[1]),
		}
	}

	for i, ask := range depth.Asks {
		book.Asks[i] = types.OrderBookLevel{
			Price:    parseDecimal(ask[0]),
			Quantity: parseDecimal(ask[1]),
		}
	}

	return book, nil
}

func (c *Client) GetCandles(ctx context.Context, symbol string, interval types.Timeframe, limit int) ([]*types.Candle, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	params := map[string]string{
		"symbol":   symbol,
		"interval": string(interval),
		"limit":    strconv.Itoa(limit),
	}

	resp, err := c.request(ctx, "GET", "/api/v3/klines", params, false)
	if err != nil {
		return nil, err
	}

	var rawCandles [][]interface{}
	if err := json.Unmarshal(resp, &rawCandles); err != nil {
		return nil, err
	}

	candles := make([]*types.Candle, len(rawCandles))
	for i, raw := range rawCandles {
		candles[i] = &types.Candle{
			Symbol:    symbol,
			Exchange:  types.ExchangeBinance,
			Timeframe: string(interval),
			Timestamp: time.Unix(int64(raw[0].(float64))/1000, 0),
			Open:      parseDecimal(raw[1].(string)),
			High:      parseDecimal(raw[2].(string)),
			Low:       parseDecimal(raw[3].(string)),
			Close:     parseDecimal(raw[4].(string)),
			Volume:    parseDecimal(raw[5].(string)),
			Closed:    true,
		}
	}

	return candles, nil
}

func (c *Client) GetRecentTrades(ctx context.Context, symbol string, limit int) ([]*types.Trade, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	params := map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	}

	resp, err := c.request(ctx, "GET", "/api/v3/trades", params, false)
	if err != nil {
		return nil, err
	}

	var rawTrades []struct {
		ID              int64  `json:"id"`
		Price          string `json:"price"`
		Qty            string `json:"qty"`
		Time           int64  `json:"time"`
		IsBuyerMaker   bool   `json:"isBuyerMaker"`
	}

	if err := json.Unmarshal(resp, &rawTrades); err != nil {
		return nil, err
	}

	trades := make([]*types.Trade, len(rawTrades))
	for i, t := range rawTrades {
		side := types.OrderSideSell
		if t.IsBuyerMaker {
			side = types.OrderSideBuy
		}

		trades[i] = &types.Trade{
			ID:            uuid.New(),
			Exchange:      types.ExchangeBinance,
			Symbol:        symbol,
			Side:          side,
			Price:         parseDecimal(t.Price),
			Quantity:      parseDecimal(t.Qty),
			IsBuyerMaker: t.IsBuyerMaker,
			Timestamp:     time.Unix(t.Time/1000, 0),
		}
	}

	return trades, nil
}

func (c *Client) GetAggTrades(ctx context.Context, symbol string, limit int) ([]*types.Trade, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	params := map[string]string{
		"symbol": symbol,
		"limit":  strconv.Itoa(limit),
	}

	resp, err := c.request(ctx, "GET", "/api/v3/aggTrades", params, false)
	if err != nil {
		return nil, err
	}

	var rawTrades []struct {
		AggTradeID   int64  `json:"a"`
		Price       string `json:"p"`
		Quantity     string `json:"q"`
		FirstTradeID int64  `json:"f"`
		Time         int64  `json:"T"`
		IsBuyerMaker bool   `json:"m"`
	}

	if err := json.Unmarshal(resp, &rawTrades); err != nil {
		return nil, err
	}

	trades := make([]*types.Trade, len(rawTrades))
	for i, t := range rawTrades {
		side := types.OrderSideSell
		if t.IsBuyerMaker {
			side = types.OrderSideBuy
		}

		trades[i] = &types.Trade{
			ID:            uuid.New(),
			Exchange:      types.ExchangeBinance,
			Symbol:        symbol,
			Side:          side,
			Price:         parseDecimal(t.Price),
			Quantity:      parseDecimal(t.Quantity),
			IsBuyerMaker: t.IsBuyerMaker,
			Timestamp:     time.Unix(t.Time/1000, 0),
		}
	}

	return trades, nil
}

func (c *Client) GetAvgPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	params := map[string]string{
		"symbol": symbol,
	}

	resp, err := c.request(ctx, "GET", "/api/v3/avgPrice", params, false)
	if err != nil {
		return decimal.Zero, err
	}

	var result struct {
		Mins    int    `json:"mins"`
		Price   string `json:"price"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return decimal.Zero, err
	}

	return parseDecimal(result.Price), nil
}

func (c *Client) GetPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	params := map[string]string{
		"symbol": symbol,
	}

	resp, err := c.request(ctx, "GET", "/api/v3/ticker/price", params, false)
	if err != nil {
		return decimal.Zero, err
	}

	var result struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return decimal.Zero, err
	}

	return parseDecimal(result.Price), nil
}

func (c *Client) GetBookTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	params := map[string]string{
		"symbol": symbol,
	}

	resp, err := c.request(ctx, "GET", "/api/v3/ticker/bookTicker", params, false)
	if err != nil {
		return nil, err
	}

	var ticker BookTicker
	if err := json.Unmarshal(resp, &ticker); err != nil {
		return nil, err
	}

	return &types.Ticker{
		Symbol:   ticker.Symbol,
		Exchange: types.ExchangeBinance,
		BidPrice: parseDecimal(ticker.BidPrice),
		BidQty:   parseDecimal(ticker.BidQty),
		AskPrice: parseDecimal(ticker.AskPrice),
		AskQty:   parseDecimal(ticker.AskQty),
	}, nil
}

func (c *Client) GetPositions(ctx context.Context) ([]*types.Position, error) {
	return nil, fmt.Errorf("spot trading does not support positions, use balances instead")
}

func (c *Client) GetPosition(ctx context.Context, symbol string) (*types.Position, error) {
	return nil, fmt.Errorf("spot trading does not support positions, use balances instead")
}

func (c *Client) CreateOCOOrder(ctx context.Context, order *types.Order, stopPrice, stopLimitPrice decimal.Decimal) (*types.Order, error) {
	params := map[string]string{
		"symbol":        order.Symbol,
		"side":          string(order.Side),
		"quantity":      order.Quantity.String(),
		"price":         order.Price.String(),
		"stopPrice":     stopPrice.String(),
		"stopLimitPrice": stopLimitPrice.String(),
	}

	resp, err := c.request(ctx, "POST", "/api/v3/order/oco", params, true)
	if err != nil {
		return nil, err
	}

	logger.Info("OCO order created", "response", string(resp))
	return order, nil
}

func (c *Client) CancelOCOOrder(ctx context.Context, symbol string, orderListID int64) error {
	params := map[string]string{
		"symbol":      symbol,
		"orderListId": strconv.FormatInt(orderListID, 10),
	}

	_, err := c.request(ctx, "DELETE", "/api/v3/orderList", params, true)
	return err
}

func (c *Client) GetOCOOrders(ctx context.Context) ([]interface{}, error) {
	resp, err := c.request(ctx, "GET", "/api/v3/orderList/oco", nil, true)
	if err != nil {
		return nil, err
	}

	var result struct {
		OrderList []interface{} `json:"orderReports"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.OrderList, nil
}

func (c *Client) request(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
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

	var body io.Reader
	url := c.baseURL + endpoint

	if method == "GET" || method == "DELETE" {
		if queryString != "" {
			url += "?" + queryString
		}
	} else {
		body = strings.NewReader(queryString)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "OpenTrader/1.0")

	if signed {
		timestamp := time.Now().UnixMilli()
		recvWindow := c.recvWindow.Milliseconds()

		var signedParams string
		if queryString != "" {
			signedParams = queryString + "&"
		} else {
			signedParams = ""
		}
		signedParams += fmt.Sprintf("timestamp=%d&recvWindow=%d", timestamp, recvWindow)

		signature := c.sign(signedParams)
		url += "?" + signedParams + "&signature=" + signature

		req.URL, _ = url.Parse(url)
		req.Header.Set("X-MBX-APIKEY", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var errResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(respBody, &errResp); err == nil {
		if errResp.Code != 0 {
			return nil, fmt.Errorf("binance API error: %s", errResp.Msg)
		}
	}

	return respBody, nil
}

func (c *Client) sign(message string) string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *Client) signSHA512(message string) string {
	mac := hmac.New(sha512.New, []byte(c.apiSecret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *Client) convertToOrder(binanceOrder *Order, original *types.Order) *types.Order {
	order := &types.Order{
		ID:              uuid.New(),
		ClientOrderID:   binanceOrder.ClientOrderID,
		Exchange:        types.ExchangeBinance,
		Symbol:          binanceOrder.Symbol,
		Side:            types.OrderSide(binanceOrder.Side),
		Type:            types.OrderType(binanceOrder.Type),
		Status:          c.statusToType(binanceOrder.Status),
		Price:           parseDecimal(binanceOrder.Price),
		StopPrice:       parseDecimal(binanceOrder.StopPrice),
		Quantity:        parseDecimal(binanceOrder.OrigQty),
		FilledQuantity:  parseDecimal(binanceOrder.ExecutedQty),
		AvgFillPrice:    parseDecimal(binanceOrder.CummulativeQuoteQty).Div(parseDecimal(binanceOrder.ExecutedQty)),
		TimeInForce:     types.TimeInForce(binanceOrder.TimeInForce),
		CreatedAt:       time.Unix(binanceOrder.Time/1000, 0),
		UpdatedAt:       time.Unix(binanceOrder.UpdateTime/1000, 0),
		StrategyID:      original.StrategyID,
	}

	if binanceOrder.Status == "FILLED" {
		now := time.Now()
		order.FilledAt = &now
	}

	return order
}

func (c *Client) orderToType(binanceOrder *Order) *types.Order {
	order := &types.Order{
		ID:              uuid.New(),
		ClientOrderID:   binanceOrder.ClientOrderID,
		Exchange:        types.ExchangeBinance,
		Symbol:          binanceOrder.Symbol,
		Side:            types.OrderSide(binanceOrder.Side),
		Type:            types.OrderType(binanceOrder.Type),
		Status:          c.statusToType(binanceOrder.Status),
		Price:           parseDecimal(binanceOrder.Price),
		StopPrice:       parseDecimal(binanceOrder.StopPrice),
		Quantity:        parseDecimal(binanceOrder.OrigQty),
		FilledQuantity:  parseDecimal(binanceOrder.ExecutedQty),
		TimeInForce:     types.TimeInForce(binanceOrder.TimeInForce),
		CreatedAt:       time.Unix(binanceOrder.Time/1000, 0),
		UpdatedAt:       time.Unix(binanceOrder.UpdateTime/1000, 0),
	}

	if binanceOrder.Status == "FILLED" {
		now := time.Now()
		order.FilledAt = &now
	}

	return order
}

func (c *Client) statusToType(status string) types.OrderStatus {
	switch status {
	case "NEW":
		return types.OrderStatusOpen
	case "PARTIALLY_FILLED":
		return types.OrderStatusPartiallyFilled
	case "FILLED":
		return types.OrderStatusFilled
	case "CANCELED":
		return types.OrderStatusCancelled
	case "REJECTED":
		return types.OrderStatusRejected
	case "EXPIRED":
		return types.OrderStatusExpired
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

func (c *Client) GetSymbolInfo(symbol string) *Symbol {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.exchangeInfo == nil {
		return nil
	}

	for _, s := range c.exchangeInfo.Symbols {
		if s.Symbol == symbol {
			return &s
		}
	}

	return nil
}

func (c *Client) ConvertSymbol(symbol string) (string, string) {
	parts := strings.Split(symbol, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return symbol, "USDT"
}

func (c *Client) GetName() string {
	return "binance"
}

func (c *Client) IsConnected() bool {
	return c.account != nil
}

func (c *Client) Connect(ctx context.Context) error {
	if err := c.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	if c.apiKey != "" && c.apiSecret != "" {
		if _, err := c.GetAccount(ctx); err != nil {
			return fmt.Errorf("failed to get account: %w", err)
		}
	}

	if _, err := c.GetExchangeInfo(ctx); err != nil {
		return fmt.Errorf("failed to get exchange info: %w", err)
	}

	logger.Info("Binance client connected", "testnet", c.testnet)
	return nil
}

func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.account = nil
	logger.Info("Binance client disconnected")
	return nil
}
