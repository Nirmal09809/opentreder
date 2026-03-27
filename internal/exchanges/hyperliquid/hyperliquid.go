package hyperliquid

import (
	"context"
	"crypto/ed25519"
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
	baseURL       string
	wsURL         string
	httpClient    *http.Client
	wsClient      *websocket.Dialer
	signer       *Signer
	address       string
	mu            sync.RWMutex
}

type Config struct {
	BaseURL  string `json:"base_url"`
	WSURL    string `json:"ws_url"`
	Address  string `json:"address"`
	SecretKey string `json:"secret_key"`
}

type Signer struct {
	privateKey ed25519.PrivateKey
}

type Request struct {
	Type  string `json:"type"`
	Stream string `json:"stream,omitempty"`
}

type WsMessage struct {
	Type string `json:"type"`
	Data any   `json:"data,omitempty"`
}

type WsSubscription struct {
	Type  string   `json:"type"`
	Channels []string `json:"channels"`
}

type TickerData struct {
	Symbol     string  `json:"symbol"`
	MarkPx    string  `json:"markPx"`
	LastPx    string  `json:"lastPx"`
	BidPx     string  `json:"bidPx"`
	AskPx     string  `json:"askPx"`
	High24h   string  `json:"high24h"`
	Low24h    string  `json:"low24h"`
	Volume24h string  `json:"volume24h"`
}

type OrderBookData struct {
	Symbol   string   `json:"symbol"`
	Bids     [][]string `json:"bids"`
	Asks     [][]string `json:"asks"`
}

type TradeData struct {
	Symbol    string `json:"symbol"`
	Side     string `json:"side"`
	Price    string `json:"px"`
	Quantity string `json:"sz"`
	Hash     string `json:"hash"`
	Time     int64  `json:"time"`
}

type CandleData struct {
	Symbol    string  `json:"symbol"`
	StartTime int64   `json:"t"`
	Open      string  `json:"o"`
	High      string  `json:"h"`
	Low       string  `json:"l"`
	Close     string  `json:"c"`
	Volume    string  `json:"v"`
}

type AllMidsResponse struct {
	Data struct {
		Mids map[string]string `json:"mids"`
	} `json:"data"`
}

type OrderbookResponse struct {
	Data struct {
		Levels []struct {
			Price string `json:"px"`
			Size  string `json:"sz"`
		} `json:"levels"`
	} `json:"data"`
}

type TradesResponse struct {
	Data []struct {
		Side     string `json:"side"`
		Price    string `json:"px"`
		Size     string `json:"sz"`
		Hash     string `json:"hash"`
		TimeNano int64  `json:"timeNano"`
	} `json:"data"`
}

type CandlesResponse struct {
	Data []struct {
		StartTime int64  `json:"startTime"`
		Open      string `json:"open"`
		High      string `json:"high"`
		Low       string `json:"low"`
		Close     string `json:"close"`
		Volume    string `json:"volume"`
	} `json:"data"`
}

type CandleSnapshotResponse struct {
	Data []struct {
		StartTime int64  `json:"t"`
		Open      string `json:"o"`
		High      string `json:"h"`
		Low       string `json:"l"`
		Close     string `json:"c"`
		Volume    string `json:"v"`
	} `json:"data"`
}

type UserFillsResponse struct {
	Data []struct {
		Side       string `json:"side"`
		Price      string `json:"px"`
		Size       string `json:"sz"`
		Fee        string `json:"fee"`
		Hash       string `json:"hash"`
		OrderID    string `json:"oid"`
		TimeNano   int64  `json:"timeNano"`
	} `json:"data"`
}

type UserFundingsResponse struct {
	Data []struct {
		TimeNano    int64  `json:"timeNano"`
		Symbol      string `json:"symbol"`
		FundingRate string `json:"fundingRate"`
		Position    string `json:"position"`
	} `json:"data"`
}

type AccountData struct {
	AccountAddress string `json:"accountAddress"`
	MarginSummary struct {
		TotalMarginUsed string `json:"totalMarginUsed"`
		TotalValue     string `json:"totalValue"`
	} `json:"marginSummary"`
	AssetPositions []struct {
		Asset int `json:"asset"`
		Position struct {
			Size string `json:"size"`
		} `json:"position"`
	} `json:"assetPositions"`
}

type OrderResult struct {
	Order struct {
		OrderID   int64  `json:"orderId"`
		Symbol    string `json:"symbol"`
		Side      string `json:"side"`
		Size      string `json:"sz"`
		Price     string `json:"px"`
		OrderType string `json:"orderType"`
	} `json:"order"`
	Status string `json:"status"`
}

type OrderStatusResult struct {
	Order struct {
		OrderID   int64  `json:"orderId"`
		Symbol    string `json:"symbol"`
		Side      string `json:"side"`
		Size      string `json:"sz"`
		Price     string `json:"px"`
		OrderType string `json:"orderType"`
		Status    string `json:"status"`
	} `json:"order"`
	Statuses []string `json:"statuses"`
}

func NewClient(cfg Config) *Client {
	baseURL := "https://api.hyperliquid.xyz"
	wsURL := "wss://api.hyperliquid.xyz"

	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}
	if cfg.WSURL != "" {
		wsURL = cfg.WSURL
	}

	var signer *Signer
	if cfg.SecretKey != "" {
		keyBytes, _ := base64.StdEncoding.DecodeString(cfg.SecretKey)
		if len(keyBytes) == ed25519.PrivateKeySize {
			signer = &Signer{privateKey: ed25519.PrivateKey(keyBytes)}
		}
	}

	return &Client{
		baseURL:    baseURL,
		wsURL:      wsURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		wsClient:   &websocket.Dialer{},
		signer:     signer,
		address:    cfg.Address,
	}
}

func (c *Client) Info(ctx context.Context) (map[string]any, error) {
	body := map[string]any{
		"type": "info",
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetAllMids(ctx context.Context) (map[string]string, error) {
	body := map[string]any{
		"type": "allMids",
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result AllMidsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Data.Mids, nil
}

func (c *Client) GetOrderBook(ctx context.Context, symbol string) (*types.OrderBook, error) {
	body := map[string]any{
		"type":   "orderbook",
		"symbol": symbol,
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	bids := make([]types.PriceLevel, len(result.Data.Bids))
	for i, bid := range result.Data.Bids {
		price, _ := decimal.NewFromString(bid[0])
		size, _ := decimal.NewFromString(bid[1])
		bids[i] = types.PriceLevel{Price: price, Quantity: size}
	}

	asks := make([]types.PriceLevel, len(result.Data.Asks))
	for i, ask := range result.Data.Asks {
		price, _ := decimal.NewFromString(ask[0])
		size, _ := decimal.NewFromString(ask[1])
		asks[i] = types.PriceLevel{Price: price, Quantity: size}
	}

	return &types.OrderBook{
		Symbol:    symbol,
		Exchange:  types.Exchange("hyperliquid"),
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}, nil
}

func (c *Client) GetTrades(ctx context.Context, symbol string, limit int) ([]*types.Trade, error) {
	body := map[string]any{
		"type":   "trades",
		"symbol": symbol,
	}
	if limit > 0 {
		body["maxCount"] = limit
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result TradesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	trades := make([]*types.Trade, len(result.Data))
	for i, t := range result.Data {
		price, _ := decimal.NewFromString(t.Price)
		size, _ := decimal.NewFromString(t.Size)

		side := types.OrderSideBuy
		if t.Side == "Ask" {
			side = types.OrderSideSell
		}

		trades[i] = &types.Trade{
			Symbol:    symbol,
			Exchange:  types.Exchange("hyperliquid"),
			Side:      side,
			Price:     price,
			Quantity:  size,
			Timestamp: time.Unix(0, t.TimeNano),
		}
	}

	return trades, nil
}

func (c *Client) GetCandles(ctx context.Context, symbol, interval string, limit int) ([]*types.Candle, error) {
	body := map[string]any{
		"type":      "candleSnapshot",
		"req": map[string]any{
			"type":      interval,
			"symbol":    symbol,
			"maxCount": limit,
		},
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			StartTime int64  `json:"t"`
			Open      string `json:"o"`
			High      string `json:"h"`
			Low       string `json:"l"`
			Close     string `json:"c"`
			Volume    string `json:"v"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	candles := make([]*types.Candle, len(result.Data))
	for i, c := range result.Data {
		open, _ := decimal.NewFromString(c.Open)
		high, _ := decimal.NewFromString(c.High)
		low, _ := decimal.NewFromString(c.Low)
		close, _ := decimal.NewFromString(c.Close)
		volume, _ := decimal.NewFromString(c.Volume)

		candles[i] = &types.Candle{
			Symbol:    symbol,
			Exchange:  types.Exchange("hyperliquid"),
			Timeframe: interval,
			Timestamp: time.UnixMilli(c.StartTime),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		}
	}

	return candles, nil
}

func (c *Client) PlaceOrder(ctx context.Context, order *types.Order) (*types.Order, error) {
	if c.signer == nil {
		return nil, fmt.Errorf("signer not configured")
	}

	orderRequest := map[string]any{
		"type":  "order",
		"symbol": order.Symbol,
		"side":   strings.ToUpper(string(order.Side)),
		"sz":     order.Quantity.String(),
		"px":     order.Price.String(),
		"orderType": map[string]any{
			"type": "Limit",
		},
	}

	if order.Type == types.OrderTypeMarket {
		orderRequest["orderType"] = map[string]any{"type": "Market"}
	}

	resp, err := c.doSignedRequest(ctx, orderRequest)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []OrderResult `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Data) > 0 {
		order.ID = types.UUIDFromString(fmt.Sprintf("%d", result.Data[0].Order.OrderID))
	}

	return order, nil
}

func (c *Client) CancelOrder(ctx context.Context, symbol string, orderID int64) error {
	cancelRequest := map[string]any{
		"type":   "cancel",
		"symbol": symbol,
		"oid":    orderID,
	}

	_, err := c.doSignedRequest(ctx, cancelRequest)
	return err
}

func (c *Client) GetAccount(ctx context.Context) (*AccountData, error) {
	body := map[string]any{
		"type": "queryAccount",
	}

	resp, err := c.doSignedRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data AccountData `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) GetUserFills(ctx context.Context, symbol string, limit int) ([]*types.Trade, error) {
	body := map[string]any{
		"type": "userFills",
	}
	if symbol != "" {
		body["symbol"] = symbol
	}
	if limit > 0 {
		body["maxCount"] = limit
	}

	resp, err := c.doSignedRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result UserFillsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	trades := make([]*types.Trade, len(result.Data))
	for i, f := range result.Data {
		price, _ := decimal.NewFromString(f.Price)
		size, _ := decimal.NewFromString(f.Size)
		fee, _ := decimal.NewFromString(f.Fee)

		side := types.OrderSideBuy
		if f.Side == "Ask" {
			side = types.OrderSideSell
		}

		trades[i] = &types.Trade{
			Symbol:    symbol,
			Exchange:  types.Exchange("hyperliquid"),
			Side:      side,
			Price:     price,
			Quantity:  size,
			Commission: fee,
			Timestamp: time.Unix(0, f.TimeNano),
		}
	}

	return trades, nil
}

func (c *Client) GetUserFundings(ctx context.Context) ([]struct {
	TimeNano    int64  `json:"timeNano"`
	FundingRate string `json:"fundingRate"`
	Position    string `json:"position"`
}, error) {
	body := map[string]any{
		"type": "userFundings",
	}

	resp, err := c.doSignedRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			TimeNano    int64  `json:"timeNano"`
			FundingRate string `json:"fundingRate"`
			Position    string `json:"position"`
		} `json:"data"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *Client) doRequest(ctx context.Context, body map[string]any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/info", strings.NewReader(string(data)))
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

func (c *Client) doSignedRequest(ctx context.Context, body map[string]any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/trade", strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if c.signer != nil {
		signature := c.signer.Sign(data)
		req.Header.Set("X-HL-Signature", base64.StdEncoding.EncodeToString(signature))
	}

	if c.address != "" {
		req.Header.Set("X-HL-Address", c.address)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (s *Signer) Sign(message []byte) []byte {
	return ed25519.Sign(s.privateKey, message)
}

func init() {
	logger.Debug("Hyperliquid exchange adapter initialized")
}
