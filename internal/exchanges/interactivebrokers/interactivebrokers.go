package interactivebrokers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Client struct {
	host           string
	port           int
	clientID       int
	conn           net.Conn
	requestID      int
	mu             sync.RWMutex
	subscriptions  map[string]chan interface{}
	accountData    *AccountData
	positions      map[string]*Position
	orders         map[int]*Order
	nextOrderID    int
	connected      bool
}

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	ClientID int    `json:"client_id"`
}

type AccountData struct {
	NetLiquidation    decimal.Decimal
	Cash             decimal.Decimal
	AvailableFunds   decimal.Decimal
	Equity           decimal.Decimal
	MarginUsed       decimal.Decimal
	BuyingPower      decimal.Decimal
	InitMargin       decimal.Decimal
	MaintMargin      decimal.Decimal
}

type Position struct {
	Symbol        string
	Quantity      decimal.Decimal
	AvgCost       decimal.Decimal
	MarketValue   decimal.Decimal
	UnrealizedPNL decimal.Decimal
	Account       string
}

type Order struct {
	ID           int
	Symbol       string
	Side         string
	Quantity     decimal.Decimal
	OrderType    string
	LimitPrice   decimal.Decimal
	Status       string
	FilledQty    decimal.Decimal
	AvgFillPrice decimal.Decimal
	CreatedAt    time.Time
}

type Contract struct {
	Symbol           string
	SecurityType     string
	Expiry           string
	Strike           float64
	Right            string
	Multiplier       string
	Exchange         string
	Currency         string
	PrimaryExchange  string
}

type OrderParams struct {
	Symbol      string
	Side        string
	Quantity    decimal.Decimal
	OrderType   string
	LimitPrice  decimal.Decimal
	StopPrice   decimal.Decimal
	TIF         string
}

type TickData struct {
	Symbol    string
	Time      time.Time
	Bid       decimal.Decimal
	Ask       decimal.Decimal
	Last      decimal.Decimal
	Volume    int64
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
}

type DepthData struct {
	Symbol   string
	Time     time.Time
	Bids     []types.PriceLevel
	Asks     []types.PriceLevel
}

func NewClient(config Config) *Client {
	if config.Host == "" {
		config.Host = "127.0.0.1"
	}
	if config.Port == 0 {
		config.Port = 4002
	}
	if config.ClientID == 0 {
		config.ClientID = 1
	}

	return &Client{
		host:          config.Host,
		port:          config.Port,
		clientID:      config.ClientID,
		subscriptions: make(map[string]chan interface{}),
		positions:     make(map[string]*Position),
		orders:       make(map[int]*Order),
		nextOrderID:   1,
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to IB: %w", err)
	}

	c.conn = conn
	c.connected = true

	go c.readLoop()

	if err := c.sendAPIStart(); err != nil {
		return err
	}

	logger.Info("Connected to Interactive Brokers")
	return nil
}

func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	logger.Info("Disconnected from Interactive Brokers")
	return nil
}

func (c *Client) readLoop() {
	buffer := make([]byte, 4096)
	for {
		if c.conn == nil {
			return
		}

		c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := c.conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		if n > 0 {
			c.handleMessage(buffer[:n])
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "tickPrice":
		c.handleTickPrice(msg)
	case "tickSize":
		c.handleTickSize(msg)
	case "tickString":
		c.handleTickString(msg)
	case "accountSummary":
		c.handleAccountSummary(msg)
	case "position":
		c.handlePosition(msg)
	case "orderStatus":
		c.handleOrderStatus(msg)
	case "error":
		logger.Error("IB Error: %v", msg)
	}
}

func (c *Client) handleTickPrice(msg map[string]interface{}) {
	symbol, _ := msg["symbol"].(string)
	field, _ := msg["field"].(float64)
	price, _ := msg["price"].(float64)

	tick := TickData{
		Symbol: symbol,
		Time:   time.Now(),
	}

	switch int(field) {
	case 1:
		tick.Bid = decimal.NewFromFloat(price)
	case 2:
		tick.Ask = decimal.NewFromFloat(price)
	case 4:
		tick.Last = decimal.NewFromFloat(price)
	case 6:
		tick.High = decimal.NewFromFloat(price)
	case 7:
		tick.Low = decimal.NewFromFloat(price)
	case 9:
		tick.Close = decimal.NewFromFloat(price)
	}

	if ch, ok := c.subscriptions["ticker:"+symbol]; ok {
		ch <- tick
	}
}

func (c *Client) handleTickSize(msg map[string]interface{}) {
	symbol, _ := msg["symbol"].(string)
	size, _ := msg["size"].(float64)

	tick := TickData{
		Symbol: symbol,
		Time:   time.Now(),
		Volume: int64(size),
	}

	if ch, ok := c.subscriptions["ticker:"+symbol]; ok {
		ch <- tick
	}
}

func (c *Client) handleTickString(msg map[string]interface{}) {}

func (c *Client) handleAccountSummary(msg map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	account := msg["account"].(string)
	value := msg["value"].(string)
	tag := msg["tag"].(string)

	if c.accountData == nil {
		c.accountData = &AccountData{}
	}

	val, _ := decimal.NewFromString(value)
	switch tag {
	case "NetLiquidation":
		c.accountData.NetLiquidation = val
	case "Cash":
		c.accountData.Cash = val
	case "AvailableFunds":
		c.accountData.AvailableFunds = val
	case "EquityWithLoanValue":
		c.accountData.Equity = val
	case "MaintMarginReq":
		c.accountData.MarginUsed = val
	case "BuyingPower":
		c.accountData.BuyingPower = val
	}

	logger.Debug("Account %s: %s = %s", account, tag, value)
}

func (c *Client) handlePosition(msg map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	symbol, _ := msg["symbol"].(string)
	position := &Position{
		Symbol: symbol,
	}

	if qty, ok := msg["position"].(float64); ok {
		position.Quantity = decimal.NewFromFloat(qty)
	}
	if cost, ok := msg["avgCost"].(float64); ok {
		position.AvgCost = decimal.NewFromFloat(cost)
	}
	if account, ok := msg["account"].(string); ok {
		position.Account = account
	}

	c.positions[symbol] = position
}

func (c *Client) handleOrderStatus(msg map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	orderID, _ := msg["orderId"].(int)
	status, _ := msg["status"].(string)

	if order, ok := c.orders[orderID]; ok {
		order.Status = status
		if filled, ok := msg["filled"].(float64); ok {
			order.FilledQty = decimal.NewFromFloat(filled)
		}
		if avgPrice, ok := msg["avgFillPrice"].(float64); ok {
			order.AvgFillPrice = decimal.NewFromFloat(avgPrice)
		}
	}
}

func (c *Client) sendAPIStart() error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	msg := map[string]interface{}{
		"type":      "api",
		"version":   2,
		"clientId":  c.clientID,
	}
	return c.sendMessage(conn, msg)
}

func (c *Client) sendMessage(conn net.Conn, msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}

func (c *Client) nextReqID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestID++
	return c.requestID
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) GetAccountSummary(ctx context.Context) (*AccountData, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	msg := map[string]interface{}{
		"type":           "reqAccountSummary",
		"requestId":      reqID,
		"groupName":      "ALL",
		"tags":          "NetLiquidation,Cash,AvailableFunds,EquityWithLoanValue,MaintMarginReq,BuyingPower",
	}

	if err := c.sendMessage(conn, msg); err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accountData, nil
}

func (c *Client) GetPositions(ctx context.Context) ([]*Position, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	msg := map[string]interface{}{
		"type":        "reqPositions",
		"requestId":   reqID,
	}

	if err := c.sendMessage(conn, msg); err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)

	c.mu.RLock()
	defer c.mu.RUnlock()
	positions := make([]*Position, 0, len(c.positions))
	for _, p := range c.positions {
		positions = append(positions, p)
	}
	return positions, nil
}

func (c *Client) SubscribeTicker(symbol string, handler func(*types.Ticker)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch := make(chan interface{}, 100)
	c.subscriptions["ticker:"+symbol] = ch

	go func() {
		for msg := range ch {
			if tick, ok := msg.(TickData); ok {
				ticker := &types.Ticker{
					Symbol:     tick.Symbol,
					BidPrice:   tick.Bid,
					AskPrice:   tick.Ask,
					LastPrice:  tick.Last,
					Volume24h:  decimal.NewFromInt(tick.Volume),
					Timestamp:  tick.Time,
				}
				handler(ticker)
			}
		}
	}()

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":           "subscribe",
		"requestId":      reqID,
		"symbol":         symbol,
		"subscribe":      true,
		"contract":       contract,
	}

	data, _ := json.Marshal(msg)
	c.conn.Write(data)
	return nil
}

func (c *Client) createContract(symbol string) map[string]interface{} {
	secType := "STK"
	exchange := "SMART"
	currency := "USD"

	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USD" {
		symbol = symbol[:len(symbol)-4]
	}

	return map[string]interface{}{
		"symbol":          symbol,
		"securityType":    secType,
		"exchange":        exchange,
		"currency":        currency,
	}
}

func (c *Client) GetContractDetails(ctx context.Context, symbol string) ([]Contract, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":        "reqContractDetails",
		"requestId":   reqID,
		"contract":    contract,
	}

	if err := c.sendMessage(conn, msg); err != nil {
		return nil, err
	}

	time.Sleep(500 * time.Millisecond)

	return []Contract{{Symbol: symbol}}, nil
}

func (c *Client) PlaceOrder(ctx context.Context, params OrderParams) (*Order, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	orderID := c.nextOrderID
	c.nextOrderID++

	order := &Order{
		ID:        orderID,
		Symbol:    params.Symbol,
		Side:      params.Side,
		Quantity:  params.Quantity,
		OrderType: params.OrderType,
		Status:    "Pending",
		CreatedAt: time.Now(),
	}

	if params.LimitPrice.GreaterThan(decimal.Zero) {
		order.LimitPrice = params.LimitPrice
	}

	c.orders[orderID] = order

	contract := c.createContract(params.Symbol)

	orderMsg := map[string]interface{}{
		"type":         "placeOrder",
		"orderId":      orderID,
		"contract":     contract,
		"orderType":    params.OrderType,
		"action":       c.actionFromSide(params.Side),
		"totalQuantity": params.Quantity.String(),
		"tif":          params.TIF,
	}

	if params.LimitPrice.GreaterThan(decimal.Zero) {
		orderMsg["lmtPrice"] = params.LimitPrice.String()
	}

	if params.StopPrice.GreaterThan(decimal.Zero) {
		orderMsg["auxPrice"] = params.StopPrice.String()
	}

	if err := c.sendMessage(c.conn, orderMsg); err != nil {
		return nil, err
	}

	logger.Info("Placed order %d: %s %s %s @ %s",
		orderID, params.Side, params.Quantity.String(), params.Symbol, order.LimitPrice.String())

	return order, nil
}

func (c *Client) actionFromSide(side string) string {
	switch side {
	case "buy", "BUY":
		return "BUY"
	case "sell", "SELL":
		return "SELL"
	default:
		return side
	}
}

func (c *Client) CancelOrder(ctx context.Context, orderID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"type":     "cancelOrder",
		"orderId":  orderID,
	}

	if err := c.sendMessage(c.conn, msg); err != nil {
		return err
	}

	logger.Info("Cancelled order %d", orderID)
	return nil
}

func (c *Client) GetOpenOrders(ctx context.Context) ([]*Order, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	orders := make([]*Order, 0, len(c.orders))
	for _, o := range c.orders {
		if o.Status == "Pending" || o.Status == "Submitted" || o.Status == "Working" {
			orders = append(orders, o)
		}
	}
	return orders, nil
}

func (c *Client) RequestMarketDepth(ctx context.Context, symbol string, numRows int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":       "reqMktDepth",
		"requestId":  reqID,
		"symbol":     symbol,
		"contract":   contract,
		"numRows":    numRows,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) RequestMarketData(ctx context.Context, symbol string, snapshot bool) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":      "reqMktData",
		"requestId": reqID,
		"symbol":    symbol,
		"contract":  contract,
		"snapshot": snapshot,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) CancelMarketData(ctx context.Context, reqID int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"type":      "cancelMktData",
		"requestId": reqID,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) SetServerLogLevel(level int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"type":   "setServerLogLevel",
		"level":  level,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) GetCurrentTime(ctx context.Context) (time.Time, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return time.Time{}, fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	msg := map[string]interface{}{
		"type":       "reqCurrentTime",
		"requestId":  reqID,
	}

	if err := c.sendMessage(conn, msg); err != nil {
		return time.Time{}, err
	}

	time.Sleep(100 * time.Millisecond)
	return time.Now(), nil
}

func (c *Client) RequestHistoricalData(ctx context.Context, symbol string, duration string, barSize string) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":         "reqHistoricalData",
		"requestId":    reqID,
		"symbol":       symbol,
		"contract":     contract,
		"duration":     duration,
		"barSize":      barSize,
		"whatToShow":   "TRADES",
		"useRTH":       0,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) CalculateImpliedVolatility(ctx context.Context, symbol string, optionPrice decimal.Decimal, underPrice decimal.Decimal) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":           "calculateImpliedVolatility",
		"requestId":      reqID,
		"contract":       contract,
		"optionPrice":    optionPrice.String(),
		"underPrice":     underPrice.String(),
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) CancelCalculateImpliedVolatility(ctx context.Context, reqID int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"type":       "cancelCalculateImpliedVolatility",
		"requestId":  reqID,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) ExerciseOptions(ctx context.Context, symbol string, exerciseAction int, exerciseQuantity int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":               "exerciseOptions",
		"requestId":         reqID,
		"contract":          contract,
		"exerciseAction":    exerciseAction,
		"exerciseQuantity":  exerciseQuantity,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) RequestScannerParameters(ctx context.Context) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	msg := map[string]interface{}{
		"type":      "reqScannerParameters",
		"requestId": reqID,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) RequestScannerSubscription(ctx context.Context, subscription string) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	msg := map[string]interface{}{
		"type":          "reqScannerSubscription",
		"requestId":     reqID,
		"subscription":  subscription,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) CancelScannerSubscription(ctx context.Context, reqID int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"type":      "cancelScannerSubscription",
		"requestId": reqID,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) RequestRealTimeBars(ctx context.Context, symbol string, barSize int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":       "reqRealTimeBars",
		"requestId":  reqID,
		"symbol":     symbol,
		"contract":   contract,
		"barSize":    barSize,
		"whatToShow": "TRADES",
		"useRTH":    0,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) CancelRealTimeBars(ctx context.Context, reqID int) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"type":      "cancelRealTimeBars",
		"requestId": reqID,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) RequestSecDefOptParams(ctx context.Context, symbol string, exchange string) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	contract := c.createContract(symbol)

	msg := map[string]interface{}{
		"type":      "reqSecDefOptParams",
		"requestId": reqID,
		"symbol":    symbol,
		"exchange":  exchange,
		"contract":   contract,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) RequestSoftDollarTiers(ctx context.Context) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	msg := map[string]interface{}{
		"type":      "reqSoftDollarTiers",
		"requestId": reqID,
	}

	return c.sendMessage(conn, msg)
}

func (c *Client) RequestFamilyTiers(ctx context.Context, familyCode string) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	reqID := c.nextReqID()
	msg := map[string]interface{}{
		"type":        "reqFamilyTiers",
		"requestId":   reqID,
		"familyCode":  familyCode,
	}

	return c.sendMessage(conn, msg)
}

func ParseContract(symbol string, expiry string, strike float64, right string) Contract {
	return Contract{
		Symbol:       symbol,
		SecurityType: "OPT",
		Expiry:       expiry,
		Strike:       strike,
		Right:        right,
		Exchange:     "SMART",
		Currency:     "USD",
	}
}

func ParseFuturesContract(symbol string, expiry string) Contract {
	return Contract{
		Symbol:       symbol,
		SecurityType: "FUT",
		Expiry:       expiry,
		Exchange:     "GLOBEX",
		Currency:     "USD",
	}
}

func ParseForexContract(symbol string) Contract {
	parts := symbol
	currency := "USD"
	if len(symbol) == 6 {
		currency = symbol[3:]
	}

	return Contract{
		Symbol:       parts,
		SecurityType: "CASH",
		Exchange:     "IDEALPRO",
		Currency:     currency,
	}
}

func FormatContractID(contract Contract) string {
	return fmt.Sprintf("%s.%s.%s.%f.%s.%s",
		contract.Symbol,
		contract.SecurityType,
		contract.Expiry,
		contract.Strike,
		contract.Right,
		contract.Exchange)
}

func ParseContractID(contractID string) (Contract, error) {
	var c Contract
	_, err := fmt.Sscanf(contractID, "%s.%s.%s.%f.%s.%s",
		&c.Symbol, &c.SecurityType, &c.Expiry, &c.Strike, &c.Right, &c.Exchange)
	return c, err
}

func BarSizeToDuration(barSize string) string {
	sizes := map[string]string{
		"1 secs":  "20 S",
		"5 secs":  "1 min",
		"10 secs": "1 min",
		"15 secs": "5 min",
		"30 secs": "5 min",
		"1 min":   "1 D",
		"2 mins":  "1 D",
		"3 mins":  "1 D",
		"5 mins":  "1 D",
		"10 mins": "1 D",
		"15 mins": "1 D",
		"20 mins": "1 D",
		"30 mins": "1 D",
		"1 hour":  "1 D",
		"2 hours": "1 D",
		"3 hours": "1 D",
		"4 hours": "1 D",
		"8 hours": "1 D",
		"1 day":   "1 M",
		"1 week":  "1 Y",
	}
	return sizes[barSize]
}

func ParseBarSize(sizeStr string) int {
	sizes := map[string]int{
		"1 secs":  1,
		"5 secs":  5,
		"10 secs": 10,
		"15 secs": 15,
		"30 secs": 30,
		"1 min":   60,
		"2 mins":  120,
		"3 mins":  180,
		"5 mins":  300,
		"10 mins": 600,
		"15 mins": 900,
		"20 mins": 1200,
		"30 mins": 1800,
		"1 hour":  3600,
		"2 hours": 7200,
		"3 hours": 10800,
		"4 hours": 14400,
		"8 hours": 28800,
		"1 day":   86400,
		"1 week":  604800,
	}
	return sizes[sizeStr]
}

func FormatPrice(price float64, decimals int) string {
	format := "%." + strconv.Itoa(decimals) + "f"
	return fmt.Sprintf(format, price)
}

func init() {
	logger.Debug("Interactive Brokers exchange adapter initialized")
}
