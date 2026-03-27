package dydx

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
)

type Client struct {
	apiKey        string
	apiSecret     []byte
	apiPassphrase string
	apiStarkKey   string
	baseURL       string
	networkID     string
	httpClient    *http.Client
}

type Config struct {
	APIKey        string `json:"api_key"`
	APISecret     string `json:"api_secret"`
	APIPassphrase string `json:"api_passphrase"`
	APIStarkKey   string `json:"api_stark_key"`
	NetworkID     string `json:"network_id"`
}

type Request struct {
	Method      string            `json:"method,omitempty"`
	RequestType string            `json:"REQUEST_TYPE,omitempty"`
	MaxAge     int               `json:"MAX_AGE,omitempty"`
	Path       string            `json:"PATH,omitempty"`
	Body       map[string]any    `json:"BODY,omitempty"`
}

type Response struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type Market struct {
	Id                     string  `json:"id"`
	Pair                  string  `json:"pair"`
	BaseAsset             string  `json:"base_asset"`
	QuoteAsset            string  `json:"quote_asset"`
	MinOrderSize          float64 `json:"min_order_size"`
	StepSize              float64 `json:"step_size"`
	TickSize              float64 `json:"tick_size"`
	NativeStepSize        string  `json:"native_step_size"`
	NativeTickSize        string  `json:"native_tick_size"`
	交易所ID              string  `json:"交易所_id"`
	MarketType            string  `json:"market_type"`
	InitialMarginFraction string  `json:"initial_margin_fraction"`
	MaintenanceMarginFraction string  `json:"maintenance_margin_fraction"`
	NextFundingRate       float64 `json:"next_funding_rate"`
}

type MarketsResponse struct {
	Markets map[string]Market `json:"markets"`
}

type OrderBookLevel struct {
	Price  string `json:"price"`
	Size   string `json:"size"`
}

type OrderBookResponse struct {
	Asks []OrderBookLevel `json:"asks"`
	Bids []OrderBookLevel `json:"bids"`
}

type Candle struct {
	StartedAt    string  `json:"startedAt"`
	UpdatedAt    string  `json:"updatedAt"`
	Trades       int     `json:"trades"`
	Open         float64 `json:"open"`
	High         float64 `json:"high"`
	Low          float64 `json:"low"`
	Close        float64 `json:"close"`
	BaseTokenVolume float64 `json:"baseTokenVolume"`
	QuoteTokenVolume float64 `json:"quoteTokenVolume"`
}

type CandlesResponse struct {
	Candles []Candle `json:"candles"`
}

type Order struct {
	Id               string `json:"id"`
	ClientId         string `json:"client_id"`
	AccountId        string `json:"account_id"`
	Market            string `json:"market"`
	Side             string `json:"side"`
	Type             string `json:"type"`
	Size             string `json:"size"`
	Price            string `json:"price"`
	Status           string `json:"status"`
	RemainingSize    string `json:"remaining_size"`
	FilledSize       string `json:"filled_size"`
	AverageFilledPrice string `json:"average_filled_price"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type OrdersResponse struct {
	Orders []Order `json:"orders"`
}

type Position struct {
	Id                string  `json:"id"`
	AccountId        string  `json:"account_id"`
	Market            string  `json:"market"`
	Side              string  `json:"side"`
	Size              string  `json:"size"`
	MaxSize          string  `json:"max_size"`
	EntryPrice       string  `json:"entry_price"`
	ExitPrice        string  `json:"exit_price"`
	OpenValue        string  `json:"open_value"`
	RealizedPnl      string  `json:"realized_pnl"`
	UnrealizedPnl    string  `json:"unrealized_pnl"`
	Leverage         string  `json:"leverage"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type PositionsResponse struct {
	Positions []Position `json:"positions"`
}

type Account struct {
	Id                    string `json:"id"`
	AccountNumber         int    `json:"account_number"`
	Market                string `json:"market"`
	Collateral            string `json:" Collateral"`
	FreeCollateral        string `json:"free_collateral"`
	PlacedOrders          int    `json:"placed_orders"`
	TotalTradeSize       string `json:"total_trade_size"`
	OpenPositionValue     string `json:"open_position_value"`
	InitialMarginFraction string `json:"initial_margin_fraction"`
	MaintenanceMarginFraction string `json:"maintenance_margin_fraction"`
	NetQuoteValue         string `json:"net_quote_value"`
}

type AccountResponse struct {
	Account Account `json:"account"`
}

type Trade struct {
	Id             string `json:"id"`
	Side           string `json:"side"`
	OrderId        string `json:"order_id"`
	Market         string `json:"market"`
	Price          string `json:"price"`
	Size           string `json:"size"`
	Fee            string `json:"fee"`
	CreatedAt      string `json:"created_at"`
	PositionId     string `json:"position_id"`
	TransactionId  string `json:"transaction_id"`
}

type TradesResponse struct {
	Trades []Trade `json:"trades"`
}

type FillsResponse struct {
	Fills []struct {
		Id              string `json:"id"`
		Market          string `json:"market"`
		OrderId         string `json:"order_id"`
		Side            string `json:"side"`
		Type            string `json:"type"`
		Price           string `json:"price"`
		Size            string `json:"size"`
		Fee             string `json:"fee"`
		CreatedAt       string `json:"created_at"`
		TradeId         string `json:"trade_id"`
		TransactionId    string `json:"transaction_id"`
	} `json:"fills"`
}

func NewClient(cfg Config) *Client {
	baseURL := "https://api.dydx.exchange"

	if cfg.NetworkID == "mainnet" {
		baseURL = "https://api.dydx.trade"
	}

	return &Client{
		apiKey:        cfg.APIKey,
		apiSecret:     []byte(cfg.APISecret),
		apiPassphrase: cfg.APIPassphrase,
		apiStarkKey:   cfg.APIStarkKey,
		baseURL:       baseURL,
		networkID:     cfg.NetworkID,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) GetMarkets(ctx context.Context) (*MarketsResponse, error) {
	path := "/v4/markets"
	
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result MarketsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetOrderBook(ctx context.Context, market string, limit int) (*OrderBookResponse, error) {
	path := fmt.Sprintf("/v4/orderbook/%s?limit=%d", market, limit)
	
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result OrderBookResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetCandles(ctx context.Context, market, resolution string, fromISO, toISO string, limit int) (*CandlesResponse, error) {
	path := fmt.Sprintf("/v4/candles/%s?resolution=%s", market, resolution)
	if limit > 0 {
		path += fmt.Sprintf("&limit=%d", limit)
	}
	if fromISO != "" {
		path += "&fromISO=" + fromISO
	}
	if toISO != "" {
		path += "&toISO=" + toISO
	}
	
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result CandlesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetTrades(ctx context.Context, market string, limit int) (*TradesResponse, error) {
	path := fmt.Sprintf("/v4/trades?market=%s", market)
	if limit > 0 {
		path += fmt.Sprintf("&limit=%d", limit)
	}
	
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result TradesResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetOrders(ctx context.Context, market, side, status string, limit int) (*OrdersResponse, error) {
	path := "/v4/orders"
	queryParams := []string{}
	
	if market != "" {
		queryParams = append(queryParams, "market="+market)
	}
	if side != "" {
		queryParams = append(queryParams, "side="+side)
	}
	if status != "" {
		queryParams = append(queryParams, "status="+status)
	}
	if limit > 0 {
		queryParams = append(queryParams, fmt.Sprintf("limit=%d", limit))
	}
	
	if len(queryParams) > 0 {
		path += "?" + strings.Join(queryParams, "&")
	}
	
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result OrdersResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) CreateOrder(ctx context.Context, order *types.Order) (*Order, error) {
	body := map[string]any{
		"market": order.Symbol,
		"side":   strings.ToUpper(string(order.Side)),
		"type":   strings.ToUpper(string(order.Type)),
		"size":   order.Quantity.String(),
		"price":  order.Price.String(),
	}

	if order.TimeInForce == types.TimeInForceIOC {
		body["timeInForce"] = "IOC"
	}

	resp, err := c.doSignedRequest(ctx, "POST", "/v4/orders", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Order Order `json:"order"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result.Order, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	path := fmt.Sprintf("/v4/orders/%s", orderID)
	_, err := c.doSignedRequest(ctx, "DELETE", path, nil)
	return err
}

func (c *Client) GetAccount(ctx context.Context) (*AccountResponse, error) {
	resp, err := c.doSignedRequest(ctx, "GET", "/v4/accounts", nil)
	if err != nil {
		return nil, err
	}

	var result AccountResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetPositions(ctx context.Context) (*PositionsResponse, error) {
	resp, err := c.doSignedRequest(ctx, "GET", "/v4/positions", nil)
	if err != nil {
		return nil, err
	}

	var result PositionsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetFills(ctx context.Context, market string, limit int) (*FillsResponse, error) {
	path := "/v4/fills"
	queryParams := []string{}
	
	if market != "" {
		queryParams = append(queryParams, "market="+market)
	}
	if limit > 0 {
		queryParams = append(queryParams, fmt.Sprintf("limit=%d", limit))
	}
	
	if len(queryParams) > 0 {
		path += "?" + strings.Join(queryParams, "&")
	}
	
	resp, err := c.doSignedRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result FillsResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetHistoricalFundingRates(ctx context.Context, market string) ([]struct {
	FundingRate string `json:"rate"`
	Period      string `json:"period"`
}, error) {
	path := fmt.Sprintf("/v4/funding?market=%s", market)
	
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []struct {
		FundingRate string `json:"rate"`
		Period      string `json:"period"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body map[string]any) ([]byte, error) {
	url := c.baseURL + path
	
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
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
	timestamp := time.Now().UTC().Format(time.RFC3339)
	
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DYDX-SIGNATURE", c.generateSignature(method, path, string(timestamp), body))
	req.Header.Set("DYDX-TIMESTAMP", timestamp)
	req.Header.Set("DYDX-API-KEY", c.apiKey)
	req.Header.Set("DYDX-PASSPHRASE", c.apiPassphrase)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *Client) generateSignature(method, path, timestamp string, body map[string]any) string {
	signatureBody := method + path + timestamp
	
	if body != nil {
		bodyJSON, _ := json.Marshal(body)
		signatureBody += string(bodyJSON)
	}

	h := ed25519.Sign(ed25519.PrivateKey(c.apiSecret), []byte(signatureBody))
	return base64.StdEncoding.EncodeToString(h)
}

func GenerateNonce() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func init() {
	logger.Debug("dYdX exchange adapter initialized")
}
