package tradier

import (
	"bytes"
	"context"
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
	apiKey     string
	accountID  string
	baseURL    string
	env        string
	httpClient *http.Client
	wsConn     *WebSocketClient
	account    *types.Account
	accountMu  sync.RWMutex
}

type Config struct {
	APIKey    string
	AccountID string
	Env       string
}

type AccountResponse struct {
	Balances Balances `json:"balances"`
}

type Balances struct {
	TotalCash            decimal.Decimal `json:"total_cash"`
	Cash                 decimal.Decimal `json:"cash"`
	PortfolioValue       decimal.Decimal `json:"portfolio_value"`
	Equity               decimal.Decimal `json:"equity"`
	LongMarketValue      decimal.Decimal `json:"long_market_value"`
	ShortMarketValue     decimal.Decimal `json:"short_market_value"`
	DayTradeBuyingPower  decimal.Decimal `json:"day_trade_buying_power"`
	PendingCash          decimal.Decimal `json:"pending_cash"`
	PortfolioMargin     decimal.Decimal `json:"portfolio_margin"`
	RegTMargin          decimal.Decimal `json:"reg_t_margin"`
	IRAMargin           decimal.Decimal `json:"ira_margin"`
}

type PositionResponse struct {
	Position []Position `json:"position"`
}

type Position struct {
	ID              string          `json:"id"`
	AccountID       string          `json:"account_id"`
	Symbol          string          `json:"symbol"`
	Quantity        decimal.Decimal `json:"quantity"`
	CostBasis       decimal.Decimal `json:"cost_basis"`
	Sharing         string          `json:"sharing"`
	SymbolType      string          `json:"symbol_type"`
	Type            string          `json:"type"`
	SubType         string          `json:"subtype"`
	OpenDate        string          `json:"open_date"`
	Last            decimal.Decimal `json:"last"`
	CurrentValue    decimal.Decimal `json:"current_value"`
	CostBasisPerShare decimal.Decimal `json:"cost_basis_per_share"`
	PercentGain     decimal.Decimal `json:"percent_gain"`
	Pln             decimal.Decimal `json:"pln"`
	Plp             decimal.Decimal `json:"plp"`
	TodayPln        decimal.Decimal `json:"today_pln"`
	TodayPlp        decimal.Decimal `json:"today_plp"`
}

type OrderResponse struct {
	Orders OrderList `json:"orders"`
}

type OrderList struct {
	Order []Order `json:"order"`
}

type Order struct {
	ID               string          `json:"id"`
	Status           string          `json:"status"`
	Origin           string          `json:"origin"`
	OrderType        string          `json:"type"`
	Symbol           string          `json:"symbol"`
	Side             string          `json:"side"`
	Quantity         decimal.Decimal `json:"quantity"`
	PositionQuantity decimal.Decimal `json:"position_quantity"`
	Price            decimal.Decimal `json:"price"`
	Stop             decimal.Decimal `json:"stop"`
	Class            string          `json:"class"`
	Duration         string          `json:"duration"`
	Legs             []OrderLeg     `json:"leg"`
	CreateDate       string          `json:"create_date"`
	TransactionDate  string          `json:"transaction_date"`
	ExpiresDate      string          `json:"expires_date"`
	DelayDays        string          `json:"delay_days"`
	FillQty          decimal.Decimal `json:"fill_qty"`
	AvgFillPrice     decimal.Decimal `json:"avg_fill_price"`
}

type OrderLeg struct {
	Instrument Instrument `json:"instrument"`
	PositionQuantity decimal.Decimal `json:"position_quantity"`
	Quantity    decimal.Decimal `json:"quantity"`
	Price       decimal.Decimal `json:"price"`
}

type Instrument struct {
	Symbol string `json:"symbol"`
	Type   string `json:"type"`
}

type QuoteResponse struct {
	Quotes Quotes `json:"quotes"`
}

type Quotes struct {
	Quote Quote `json:"quote"`
}

type Quote struct {
	Symbol            string  `json:"symbol"`
	Exchange          string  `json:"exchange"`
	SymbolType        string  `json:"symbol_type"`
	Last              float64 `json:"last"`
	Bid               float64 `json:"bid"`
	Ask               float64 `json:"ask"`
	BidSize           int     `json:"bid_size"`
	AskSize           int     `json:"ask_size"`
	BidDate           string  `json:"bid_date"`
	AskDate           string  `json:"ask_date"`
	Open              float64 `json:"open"`
	High              float64 `json:"high"`
	Low               float64 `json:"low"`
	Close             float64 `json:"close"`
	Volume            int     `json:"volume"`
	AvgVolume         int     `json:"avg_volume"`
	Week52High        float64 `json:"week_52_high"`
	Week52Low         float64 `json:"week_52_low"`
	Change            float64 `json:"change"`
	ChangePercentage  float64 `json:"change_percentage"`
}

type OptionQuoteResponse struct {
	Options OptionsQuotes `json:"options"`
}

type OptionsQuotes struct {
	Option []OptionQuote `json:"option"`
}

type OptionQuote struct {
	Symbol           string  `json:"symbol"`
	Strike           float64 `json:"strike"`
	SymbolType       string  `json:"symbol_type"`
	Last             float64 `json:"last"`
	Bid              float64 `json:"bid"`
	Ask              float64 `json:"ask"`
	BidSize          int     `json:"bid_size"`
	AskSize          int     `json:"ask_size"`
	Volume           int     `json:"Volume"`
	OpenInterest     int     `json:"open_interest"`
	Delta            float64 `json:"delta"`
	Gamma            float64 `json:"gamma"`
	Theta            float64 `json:"theta"`
	Vega             float64 `json:"vega"`
	 Rho             float64 `json:"rho"`
	ITM             bool    `json:"itm"`
	ChangePercentage float64 `json:"change_percentage"`
}

type ChainResponse struct {
	Options OptionsChain `json:"options"`
}

type OptionsChain struct {
	Date      string             `json:"date"`
	Underlying string            `json:"underlying"`
	StrikeCount int              `json:"strike_count"`
	Expiration []ExpirationDate `json:"expiration"`
}

type ExpirationDate struct {
	Date         string        `json:"date"`
	StrikeCount  int           `json:"strike_count"`
	Option      []OptionContract `json:"option"`
}

type OptionContract struct {
	Symbol        string  `json:"symbol"`
	Strike        float64 `json:"strike"`
	Type          string  `json:"type"`
	ExpirationDate string `json:"expiration_date"`
	SymbolType    string  `json:"symbol_type"`
}

type HistoryResponse struct {
	Transactions TransactionList `json:"transactions"`
}

type TransactionList struct {
	Transaction []Transaction `json:"transaction"`
}

type Transaction struct {
	ID            string `json:"id"`
	AccountID     string `json:"account_id"`
	Date          string `json:"date"`
	Type          string `json:"type"`
	Symbol        string `json:"symbol"`
	Description   string `json:"Description"`
	Amount        string `json:"amount"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
}

type OrderRequest struct {
	AccountID  string       `json:"account_id"`
	Symbol     string       `json:"symbol"`
	Side       string       `json:"side"`
	Quantity   string       `json:"quantity"`
	OrderType  string       `json:"type"`
	Duration   string       `json:"duration"`
	Price      string       `json:"price,omitempty"`
	Stop       string       `json:"stop,omitempty"`
	OptionSymbol string    `json:"option_symbol,omitempty"`
	Legs       []OrderLegRequest `json:"legs,omitempty"`
}

type OrderLegRequest struct {
	Symbol       string `json:"symbol"`
	Quantity     string `json:"quantity"`
	Side         string `json:"side"`
	PositionEffect string `json:"position_effect,omitempty"`
}

func NewClient(cfg *Config) (*Client, error) {
	baseURL := "https://api.tradier.com/v1"

	if cfg.Env == "paper" {
		baseURL = "https://api.tradier.com/v1"
	}

	client := &Client{
		apiKey:     cfg.APIKey,
		accountID:  cfg.AccountID,
		baseURL:    baseURL,
		env:        cfg.Env,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	return client, nil
}

func (c *Client) GetAccount(ctx context.Context) (*types.Account, error) {
	resp, err := c.doRequest(ctx, "GET", "/accounts/"+c.accountID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var accountResp AccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&accountResp); err != nil {
		return nil, err
	}

	account := &types.Account{
		AccountID:       c.accountID,
		Exchange:        "tradier",
		Currency:        "USD",
		BuyingPower:     accountResp.Balances.DayTradeBuyingPower,
		Cash:            accountResp.Balances.TotalCash,
		PortfolioValue:  accountResp.Balances.PortfolioValue,
		TradingEnabled:  true,
	}

	c.accountMu.Lock()
	c.account = account
	c.accountMu.Unlock()

	return account, nil
}

func (c *Client) PlaceOrder(ctx context.Context, req *types.Order) (*types.Order, error) {
	orderReq := OrderRequest{
		AccountID: c.accountID,
		Symbol:    req.Symbol,
		Side:     strings.ToLower(string(req.Side)),
		Quantity: req.Quantity.String(),
		OrderType: c.mapOrderType(req.Type),
		Duration: c.mapDuration(req.TimeInForce),
	}

	if !req.Price.IsZero() {
		orderReq.Price = req.Price.String()
	}
	if !req.StopPrice.IsZero() {
		orderReq.Stop = req.StopPrice.String()
	}

	body, _ := json.Marshal(orderReq)
	resp, err := c.doRequest(ctx, "POST", "/accounts/"+c.accountID+"/orders", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	orderID, _ := result["id"].(string)
	req.ID = orderID
	return req, nil
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) error {
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/accounts/%s/orders/%s", c.accountID, orderID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cancel order: %s", string(body))
	}

	return nil
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (*types.Order, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/accounts/%s/orders/%s", c.accountID, orderID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var orderResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, err
	}

	if len(orderResp.Orders.Order) == 0 {
		return nil, fmt.Errorf("order not found")
	}

	return c.parseOrder(&orderResp.Orders.Order[0]), nil
}

func (c *Client) ListOrders(ctx context.Context, status string, limit int) ([]*types.Order, error) {
	params := url.Values{}
	if status != "" {
		params.Set("status", status)
	}

	path := "/accounts/" + c.accountID + "/orders"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var orderResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, err
	}

	orders := make([]*types.Order, len(orderResp.Orders.Order))
	for i, o := range orderResp.Orders.Order {
		orders[i] = c.parseOrder(&o)
	}

	return orders, nil
}

func (c *Client) GetPosition(ctx context.Context, symbol string) (*types.Position, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/accounts/%s/positions", c.accountID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var posResp PositionResponse
	if err := json.NewDecoder(resp.Body).Decode(&posResp); err != nil {
		return nil, err
	}

	for _, p := range posResp.Position {
		if p.Symbol == symbol {
			return c.parsePosition(&p), nil
		}
	}

	return nil, fmt.Errorf("position not found for %s", symbol)
}

func (c *Client) GetAllPositions(ctx context.Context) ([]*types.Position, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/accounts/%s/positions", c.accountID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var posResp PositionResponse
	if err := json.NewDecoder(resp.Body).Decode(&posResp); err != nil {
		return nil, err
	}

	positions := make([]*types.Position, len(posResp.Position))
	for i, p := range posResp.Position {
		positions[i] = c.parsePosition(&p)
	}

	return positions, nil
}

func (c *Client) ClosePosition(ctx context.Context, symbol string) error {
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/accounts/%s/positions/%s", c.accountID, symbol), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to close position: %s", string(body))
	}

	return nil
}

func (c *Client) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	params := url.Values{}
	params.Set("symbols", symbol)

	resp, err := c.doRequest(ctx, "GET", "/markets/quotes?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var quoteResp QuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&quoteResp); err != nil {
		return nil, err
	}

	return &types.Quote{
		Symbol:    quoteResp.Quotes.Quote.Symbol,
		Exchange:  "tradier",
		Bid:       decimal.NewFromFloat(quoteResp.Quotes.Quote.Bid),
		Ask:       decimal.NewFromFloat(quoteResp.Quotes.Quote.Ask),
		BidSize:   decimal.NewFromInt(int64(quoteResp.Quotes.Quote.BidSize)),
		AskSize:   decimal.NewFromInt(int64(quoteResp.Quotes.Quote.AskSize)),
		Timestamp: time.Now(),
	}, nil
}

func (c *Client) GetOptionChain(ctx context.Context, symbol string, expiration string) ([]*types.OptionContract, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	if expiration != "" {
		params.Set("expiration", expiration)
	}

	resp, err := c.doRequest(ctx, "GET", "/markets/options/chains?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var chainResp ChainResponse
	if err := json.NewDecoder(resp.Body).Decode(&chainResp); err != nil {
		return nil, err
	}

	contracts := make([]*types.OptionContract, 0)
	for _, exp := range chainResp.Options.Expiration {
		for _, opt := range exp.Option {
			contracts = append(contracts, &types.OptionContract{
				Symbol:     opt.Symbol,
				Strike:     decimal.NewFromFloat(opt.Strike),
				Type:       opt.Type,
				Expiration: opt.ExpirationDate,
			})
		}
	}

	return contracts, nil
}

func (c *Client) GetOptionQuote(ctx context.Context, symbol string) (*types.OptionQuote, error) {
	params := url.Values{}
	params.Set("symbols", symbol)

	resp, err := c.doRequest(ctx, "GET", "/markets/options/quotes?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var optionResp OptionQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&optionResp); err != nil {
		return nil, err
	}

	if len(optionResp.Options.Option) == 0 {
		return nil, fmt.Errorf("option quote not found")
	}

	opt := optionResp.Options.Option[0]
	return &types.OptionQuote{
		Symbol:        opt.Symbol,
		Bid:           decimal.NewFromFloat(opt.Bid),
		Ask:           decimal.NewFromFloat(opt.Ask),
		Strike:        decimal.NewFromFloat(opt.Strike),
		Delta:         decimal.NewFromFloat(opt.Delta),
		Gamma:         decimal.NewFromFloat(opt.Gamma),
		Theta:         decimal.NewFromFloat(opt.Theta),
		Vega:          decimal.NewFromFloat(opt.Vega),
		ImpliedVol:    decimal.NewFromFloat(opt.Vega),
		Underlying:    symbol,
	}, nil
}

func (c *Client) GetHistory(ctx context.Context, start, end time.Time) ([]*types.Transaction, error) {
	params := url.Values{}
	params.Set("start", start.Format("2006-01-02"))
	params.Set("end", end.Format("2006-01-02"))

	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/accounts/%s/orders", c.accountID)+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var historyResp HistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&historyResp); err != nil {
		return nil, err
	}

	transactions := make([]*types.Transaction, len(historyResp.Transactions.Transaction))
	for i, t := range historyResp.Transactions.Transaction {
		amount, _ := strconv.ParseFloat(t.Amount, 64)
		quantity, _ := strconv.ParseFloat(t.Quantity, 64)
		price, _ := strconv.ParseFloat(t.Price, 64)

		transactions[i] = &types.Transaction{
			ID:          t.ID,
			AccountID:   t.AccountID,
			Date:        t.Date,
			Type:        t.Type,
			Symbol:      t.Symbol,
			Description: t.Description,
			Amount:      decimal.NewFromFloat(amount),
			Quantity:    decimal.NewFromFloat(quantity),
			Price:       decimal.NewFromFloat(price),
		}
	}

	return transactions, nil
}

func (c *Client) ConnectWebSocket(ctx context.Context) error {
	c.wsConn = NewWebSocketClient(c.accountID, c.env == "paper")
	return c.wsConn.Connect(ctx, c.apiKey)
}

func (c *Client) SubscribeQuotes(symbols []string, handler func(*types.Quote)) error {
	if c.wsConn == nil {
		return fmt.Errorf("WebSocket not connected")
	}
	return c.wsConn.SubscribeQuotes(symbols, handler)
}

func (c *Client) SubscribeOptions(symbols []string, handler func(*types.OptionQuote)) error {
	if c.wsConn == nil {
		return fmt.Errorf("WebSocket not connected")
	}
	return c.wsConn.SubscribeOptions(symbols, handler)
}

func (c *Client) UnsubscribeQuotes(symbols []string) error {
	if c.wsConn == nil {
		return nil
	}
	return c.wsConn.Unsubscribe(symbols)
}

func (c *Client) Close() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func (c *Client) mapOrderType(orderType types.OrderType) string {
	switch orderType {
	case types.OrderTypeMarket:
		return "market"
	case types.OrderTypeLimit:
		return "limit"
	case types.OrderTypeStop:
		return "stop"
	case types.OrderTypeStopLimit:
		return "stop_limit"
	default:
		return "market"
	}
}

func (c *Client) mapDuration(tif types.TimeInForce) string {
	switch tif {
	case types.TimeInForceDay:
		return "day"
	case types.TimeInForceGTC:
		return "gtc"
	case types.TimeInForceIOC:
		return "ioc"
	case types.TimeInForceFOK:
		return "fok"
	default:
		return "gtc"
	}
}

func (c *Client) parseOrder(order *Order) *types.Order {
	side := types.SideBuy
	if strings.ToLower(order.Side) == "sell" {
		side = types.SideSell
	}

	orderType := types.OrderTypeMarket
	switch strings.ToLower(order.OrderType) {
	case "limit":
		orderType = types.OrderTypeLimit
	case "stop":
		orderType = types.OrderTypeStop
	case "stop_limit":
		orderType = types.OrderTypeStopLimit
	}

	duration := types.TimeInForceGTC
	switch strings.ToLower(order.Duration) {
	case "day":
		duration = types.TimeInForceDay
	case "gtc":
		duration = types.TimeInForceGTC
	}

	createDate, _ := time.Parse("2006-01-02T15:04:05Z", order.CreateDate)

	return &types.Order{
		ID:              order.ID,
		Symbol:          order.Symbol,
		Exchange:        "tradier",
		Side:           side,
		Type:           orderType,
		Quantity:       order.Quantity,
		FilledQuantity: order.FillQty,
		Price:          order.Price,
		StopPrice:      order.Stop,
		AvgFillPrice:   order.AvgFillPrice,
		TimeInForce:    duration,
		Status:         c.parseOrderStatus(order.Status),
		CreatedAt:      createDate,
	}
}

func (c *Client) parseOrderStatus(status string) types.OrderStatus {
	switch strings.ToLower(status) {
	case "pending":
		return types.OrderStatusPending
	case "open", "active":
		return types.OrderStatusOpen
	case "partially_filled":
		return types.OrderStatusPartiallyFilled
	case "filled":
		return types.OrderStatusFilled
	case "cancelled", "canceled":
		return types.OrderStatusCanceled
	case "expired":
		return types.OrderStatusExpired
	case "rejected":
		return types.OrderStatusRejected
	default:
		return types.OrderStatusUnknown
	}
}

func (c *Client) parsePosition(pos *Position) *types.Position {
	return &types.Position{
		Symbol:       pos.Symbol,
		Exchange:     "tradier",
		Quantity:     pos.Quantity,
		AvgEntryPrice: pos.CostBasisPerShare,
		UnrealizedPnL: pos.Pln,
		RealizedPnL:  decimal.Zero,
		OpenedAt:     time.Now(),
	}
}

type WebSocketClient struct {
	accountID string
	live      bool
	conn      *Conn
	mu        sync.RWMutex
}

func NewWebSocketClient(accountID string, live bool) *WebSocketClient {
	return &WebSocketClient{
		accountID: accountID,
		live:      live,
	}
}

func (w *WebSocketClient) Connect(ctx context.Context, token string) error {
	url := "wss://api.tradier.com/v1/events"
	if !w.live {
		url = "wss://api.tradier.com/v1/events"
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return err
	}
	w.conn = &Conn{conn: conn}

	authMsg := map[string]interface{}{
		"type": "authenticate",
		"token": token,
	}
	w.conn.WriteJSON(authMsg)

	go w.readLoop()
	return nil
}

func (w *WebSocketClient) SubscribeQuotes(symbols []string, handler func(*types.Quote)) error {
	msg := map[string]interface{}{
		"type":    "subscribe",
		"symbols": symbols,
		"channel": "quotes",
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) SubscribeOptions(symbols []string, handler func(*types.OptionQuote)) error {
	msg := map[string]interface{}{
		"type":    "subscribe",
		"symbols": symbols,
		"channel": "options",
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) Unsubscribe(symbols []string) error {
	msg := map[string]interface{}{
		"type":    "unsubscribe",
		"symbols": symbols,
	}
	return w.conn.WriteJSON(msg)
}

func (w *WebSocketClient) readLoop() {
	for {
		var msg map[string]interface{}
		if err := w.conn.ReadJSON(&msg); err != nil {
			logger.Error("WebSocket read error: %v", err)
			break
		}
	}
}

func (w *WebSocketClient) Close() error {
	if w.conn != nil {
		return w.conn.Close()
	}
	return nil
}

type Conn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *Conn) WriteJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}

func (c *Conn) ReadJSON(v interface{}) error {
	return c.conn.ReadJSON(v)
}

func (c *Conn) Close() error {
	return c.conn.Close()
}
