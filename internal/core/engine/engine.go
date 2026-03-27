package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Mode string

const (
	ModePaper   Mode = "paper"
	ModeLive    Mode = "live"
	ModeBacktest Mode = "backtest"
	ModeDryRun  Mode = "dryrun"
)

type State int

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StatePaused
	StateStopping
	StateError
)

func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StatePaused:
		return "paused"
	case StateStopping:
		return "stopping"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

type Config struct {
	Mode           Mode
	Enabled        bool
	MaxPositions   int
	MaxOrders      int
	TickInterval   time.Duration
	OrderTimeout   time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
}

type Engine struct {
	config  Config
	state   State
	mode    Mode

	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc

	portfolio  *Portfolio
	orders     *OrderManager
	positions  *PositionManager
	risk       *RiskManager
	exchange   ExchangeAdapter

	events    chan Event
	ticker    *time.Ticker
	wg        sync.WaitGroup

	stats     Stats
	startTime time.Time

	exchanges map[string]ExchangeAdapter
	strategies map[string]StrategyRunner
	marketData MarketDataAdapter
}

type Stats struct {
	TotalTrades      int64
	WinningTrades    int64
	LosingTrades     int64
	TotalVolume      decimal.Decimal
	TotalFees        decimal.Decimal
	TotalPnL         decimal.Decimal
	SharpeRatio      decimal.Decimal
	MaxDrawdown      decimal.Decimal
	CurrentDrawdown  decimal.Decimal
	StartEquity      decimal.Decimal
	CurrentEquity    decimal.Decimal
	PeakEquity       decimal.Decimal
}

type ExchangeAdapter interface {
	Connect(ctx context.Context) error
	Disconnect() error
	PlaceOrder(order *types.Order) (*types.Order, error)
	CancelOrder(orderID string) error
	GetBalance(asset string) (*types.Balance, error)
	GetPositions() ([]*types.Position, error)
	GetOpenOrders(symbol string) ([]*types.Order, error)
	GetTicker(symbol string) (*types.Ticker, error)
	GetCandles(symbol string, timeframe types.Timeframe, limit int) ([]*types.Candle, error)
	GetOrderBook(symbol string, depth int) (*types.OrderBook, error)
	Subscribe(symbol string, channel string) error
	Unsubscribe(symbol string, channel string) error
}

type MarketDataAdapter interface {
	Start(ctx context.Context) error
	Stop() error
	GetTicker(symbol string) (*types.Ticker, error)
	GetCandles(symbol string, timeframe types.Timeframe, limit int) ([]*types.Candle, error)
	GetOrderBook(symbol string, depth int) (*types.OrderBook, error)
	Subscribe(symbol string, handler func(*types.Candle)) error
}

type StrategyRunner interface {
	Init(ctx context.Context, engine *Engine) error
	Start(ctx context.Context) error
	Stop() error
	OnTick(candle *types.Candle) error
	OnOrderUpdate(order *types.Order) error
	OnPositionUpdate(position *types.Position) error
	GetSignals() []*types.Signal
	GetName() string
	GetStatus() string
}

type Event struct {
	Type    EventType
	Payload interface{}
	Time    time.Time
}

type EventType string

const (
	EventOrderPlaced      EventType = "order_placed"
	EventOrderFilled      EventType = "order_filled"
	EventOrderCancelled   EventType = "order_cancelled"
	EventOrderRejected    EventType = "order_rejected"
	EventPositionOpened   EventType = "position_opened"
	EventPositionClosed   EventType = "position_closed"
	EventPositionUpdated  EventType = "position_updated"
	EventSignalGenerated  EventType = "signal_generated"
	EventRiskLimitHit    EventType = "risk_limit_hit"
	EventExchangeDisconnected EventType = "exchange_disconnected"
	EventExchangeReconnected   EventType = "exchange_reconnected"
	EventStrategyStarted  EventType = "strategy_started"
	EventStrategyStopped  EventType = "strategy_stopped"
	EventError           EventType = "error"
	EventTick            EventType = "tick"
	EventHeartbeat       EventType = "heartbeat"
)

func New(cfg Config) *Engine {
	if cfg.TickInterval == 0 {
		cfg.TickInterval = time.Second
	}
	if cfg.OrderTimeout == 0 {
		cfg.OrderTimeout = 30 * time.Second
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = time.Second
	}
	if cfg.MaxPositions == 0 {
		cfg.MaxPositions = 10
	}
	if cfg.MaxOrders == 0 {
		cfg.MaxOrders = 100
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		config:    cfg,
		state:     StateStopped,
		mode:      cfg.Mode,
		ctx:       ctx,
		cancel:    cancel,
		events:    make(chan Event, 1000),
		exchanges: make(map[string]ExchangeAdapter),
		strategies: make(map[string]StrategyRunner),
		startTime: time.Now(),
	}

	e.portfolio = NewPortfolio(e)
	e.orders = NewOrderManager(e)
	e.positions = NewPositionManager(e)
	e.risk = NewRiskManager(e)

	return e
}

func (e *Engine) Start() error {
	e.mu.Lock()
	if e.state != StateStopped && e.state != StateError {
		e.mu.Unlock()
		return fmt.Errorf("engine already running")
	}
	e.state = StateStarting
	e.mu.Unlock()

	logger.Info("Starting OpenTrader Engine", "mode", e.mode)

	if err := e.initialize(); err != nil {
		e.setState(StateError)
		return fmt.Errorf("initialization failed: %w", err)
	}

	e.wg.Add(1)
	go e.runEventLoop()

	e.wg.Add(1)
	go e.runHeartbeat()

	e.setState(StateRunning)
	logger.Info("Engine started successfully", "mode", e.mode, "uptime", time.Since(e.startTime))

	return nil
}

func (e *Engine) Stop() error {
	e.mu.Lock()
	if e.state != StateRunning && e.state != StatePaused {
		e.mu.Unlock()
		return fmt.Errorf("engine not running")
	}
	e.state = StateStopping
	e.mu.Unlock()

	logger.Info("Stopping OpenTrader Engine")

	e.cancel()

	for name, strat := range e.strategies {
		if err := strat.Stop(); err != nil {
			logger.Error("Failed to stop strategy", "strategy", name, "error", err)
		}
	}

	for name, ex := range e.exchanges {
		if err := ex.Disconnect(); err != nil {
			logger.Error("Failed to disconnect exchange", "exchange", name, "error", err)
		}
	}

	e.wg.Wait()

	e.setState(StateStopped)
	logger.Info("Engine stopped successfully")

	return nil
}

func (e *Engine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state != StateRunning {
		return fmt.Errorf("engine not running")
	}

	e.state = StatePaused
	logger.Info("Engine paused")
	return nil
}

func (e *Engine) Resume() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state != StatePaused {
		return fmt.Errorf("engine not paused")
	}

	e.state = StateRunning
	logger.Info("Engine resumed")
	return nil
}

func (e *Engine) Tick() {
	e.mu.RLock()
	state := e.state
	e.mu.RUnlock()

	if state != StateRunning {
		return
	}

	e.events <- Event{
		Type:    EventTick,
		Payload: time.Now(),
		Time:    time.Now(),
	}
}

func (e *Engine) initialize() error {
	logger.Debug("Initializing engine components")

	for name, ex := range e.exchanges {
		if err := ex.Connect(e.ctx); err != nil {
			return fmt.Errorf("failed to connect to %s: %w", name, err)
		}
		logger.Info("Connected to exchange", "exchange", name)
	}

	if e.marketData != nil {
		if err := e.marketData.Start(e.ctx); err != nil {
			return fmt.Errorf("failed to start market data: %w", err)
		}
	}

	return nil
}

func (e *Engine) runEventLoop() {
	defer e.wg.Done()

	logger.Debug("Event loop started")

	for {
		select {
		case <-e.ctx.Done():
			logger.Debug("Event loop stopping")
			return

		case event := <-e.events:
			e.handleEvent(event)
		}
	}
}

func (e *Engine) runHeartbeat() {
	defer e.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.events <- Event{
				Type:    EventHeartbeat,
				Payload: time.Now(),
				Time:    time.Now(),
			}
		}
	}
}

func (e *Engine) handleEvent(event Event) {
	switch event.Type {
	case EventTick:
		e.handleTick(event)

	case EventOrderFilled:
		e.handleOrderFilled(event)

	case EventOrderCancelled:
		e.handleOrderCancelled(event)

	case EventOrderRejected:
		e.handleOrderRejected(event)

	case EventPositionOpened:
		e.handlePositionOpened(event)

	case EventPositionClosed:
		e.handlePositionClosed(event)

	case EventPositionUpdated:
		e.handlePositionUpdated(event)

	case EventSignalGenerated:
		e.handleSignal(event)

	case EventRiskLimitHit:
		e.handleRiskLimitHit(event)

	case EventHeartbeat:
		e.handleHeartbeat(event)
	}
}

func (e *Engine) handleTick(event Event) {
	e.mu.RLock()
	positions := e.positions.GetAll()
	e.mu.RUnlock()

	for _, pos := range positions {
		e.updatePositionPnL(pos)
	}

	e.checkRiskLimits()
}

func (e *Engine) handleOrderFilled(event Event) {
	order, ok := event.Payload.(*types.Order)
	if !ok {
		return
	}

	logger.Info("Order filled",
		"order_id", order.ID,
		"symbol", order.Symbol,
		"side", order.Side,
		"quantity", order.FilledQuantity,
		"price", order.AvgFillPrice,
	)

	e.mu.Lock()
	e.stats.TotalTrades++
	e.stats.TotalVolume = e.stats.TotalVolume.Add(order.FilledQuantity)
	e.stats.TotalFees = e.stats.TotalFees.Add(order.Commission)
	e.mu.Unlock()

	if order.Side == types.OrderSideBuy {
		e.portfolio.UpdateBalance(order.Symbol, order.FilledQuantity, false)
	} else {
		e.portfolio.UpdateBalance(order.Symbol, order.FilledQuantity, true)
	}

	e.positions.UpdateFromOrder(order)
}

func (e *Engine) handleOrderCancelled(event Event) {
	order, ok := event.Payload.(*types.Order)
	if !ok {
		return
	}

	logger.Info("Order cancelled", "order_id", order.ID, "symbol", order.Symbol)

	e.orders.RemoveOrder(order.ID)
}

func (e *Engine) handleOrderRejected(event Event) {
	order, ok := event.Payload.(*types.Order)
	if !ok {
		return
	}

	logger.Warn("Order rejected", "order_id", order.ID, "symbol", order.Symbol)
}

func (e *Engine) handlePositionOpened(event Event) {
	pos, ok := event.Payload.(*types.Position)
	if !ok {
		return
	}

	logger.Info("Position opened",
		"symbol", pos.Symbol,
		"side", pos.Side,
		"quantity", pos.Quantity,
		"entry_price", pos.AvgEntryPrice,
	)
}

func (e *Engine) handlePositionClosed(event Event) {
	pos, ok := event.Payload.(*types.Position)
	if !ok {
		return
	}

	logger.Info("Position closed",
		"symbol", pos.Symbol,
		"realized_pnl", pos.RealizedPnL,
		"duration", time.Since(pos.OpenedAt),
	)

	e.mu.Lock()
	e.stats.TotalPnL = e.stats.TotalPnL.Add(pos.RealizedPnL)

	if pos.RealizedPnL.GreaterThan(decimal.Zero) {
		e.stats.WinningTrades++
	} else {
		e.stats.LosingTrades++
	}

	e.updateDrawdown()
	e.mu.Unlock()
}

func (e *Engine) handlePositionUpdated(event Event) {
	pos, ok := event.Payload.(*types.Position)
	if !ok {
		return
	}

	e.updatePositionPnL(pos)
}

func (e *Engine) handleSignal(event Event) {
	signal, ok := event.Payload.(*types.Signal)
	if !ok {
		return
	}

	logger.Info("Signal received",
		"symbol", signal.Symbol,
		"action", signal.Action,
		"strength", signal.Strength,
		"confidence", signal.Confidence,
	)

	if e.risk.ValidateSignal(signal) {
		e.executeSignal(signal)
	}
}

func (e *Engine) handleRiskLimitHit(event Event) {
	limit, ok := event.Payload.(string)
	if !ok {
		return
	}

	logger.Warn("Risk limit hit", "limit", limit)
}

func (e *Engine) handleHeartbeat(event Event) {
	logger.Debug("Heartbeat", "uptime", time.Since(e.startTime))
}

func (e *Engine) updatePositionPnL(pos *types.Position) {
	if pos == nil {
		return
	}

	e.mu.RLock()
	ex, ok := e.exchanges[string(pos.Exchange)]
	e.mu.RUnlock()

	if !ok {
		return
	}

	ticker, err := ex.GetTicker(pos.Symbol)
	if err != nil {
		return
	}

	pos.CurrentPrice = ticker.LastPrice
	pos.UnrealizedPnL = calculatePnL(pos)

	roi := decimal.Zero
	if pos.AvgEntryPrice.GreaterThan(decimal.Zero) {
		roi = pos.CurrentPrice.Sub(pos.AvgEntryPrice).Div(pos.AvgEntryPrice).Mul(decimal.NewFromInt(100))
		if pos.Side == types.PositionSideShort {
			roi = roi.Neg()
		}
	}
	pos.ROI = roi

	e.positions.UpdatePosition(pos)
}

func (e *Engine) updateDrawdown() {
	currentEquity := e.stats.CurrentEquity
	peakEquity := e.stats.PeakEquity

	if currentEquity.GreaterThan(peakEquity) {
		e.stats.PeakEquity = currentEquity
		peakEquity = currentEquity
	}

	if peakEquity.GreaterThan(decimal.Zero) {
		e.stats.CurrentDrawdown = peakEquity.Sub(currentEquity).Div(peakEquity).Mul(decimal.NewFromInt(100))
		if e.stats.CurrentDrawdown.GreaterThan(e.stats.MaxDrawdown) {
			e.stats.MaxDrawdown = e.stats.CurrentDrawdown
		}
	}
}

func (e *Engine) checkRiskLimits() {
	if err := e.risk.CheckAllLimits(); err != nil {
		e.events <- Event{
			Type:    EventRiskLimitHit,
			Payload: err.Error(),
			Time:    time.Now(),
		}
	}
}

func (e *Engine) executeSignal(signal *types.Signal) {
	switch signal.Action {
	case types.SignalActionBuy:
		e.placeBuyOrder(signal)
	case types.SignalActionSell:
		e.placeSellOrder(signal)
	case types.SignalActionClose:
		e.closePosition(signal)
	}
}

func (e *Engine) placeBuyOrder(signal *types.Signal) error {
	order := &types.Order{
		ID:          uuid.New(),
		Exchange:    signal.Exchange,
		Symbol:      signal.Symbol,
		Side:        types.OrderSideBuy,
		Type:        types.OrderTypeMarket,
		Quantity:    signal.Quantity,
		Status:      types.OrderStatusPending,
		StrategyID:  signal.StrategyID,
		CreatedAt:   time.Now(),
	}

	return e.orders.PlaceOrder(order)
}

func (e *Engine) placeSellOrder(signal *types.Signal) error {
	order := &types.Order{
		ID:          uuid.New(),
		Exchange:    signal.Exchange,
		Symbol:      signal.Symbol,
		Side:        types.OrderSideSell,
		Type:        types.OrderTypeMarket,
		Quantity:    signal.Quantity,
		Status:      types.OrderStatusPending,
		StrategyID:  signal.StrategyID,
		CreatedAt:   time.Now(),
	}

	return e.orders.PlaceOrder(order)
}

func (e *Engine) closePosition(signal *types.Signal) error {
	pos := e.positions.GetPosition(signal.Symbol)
	if pos == nil {
		return fmt.Errorf("position not found for %s", signal.Symbol)
	}

	order := &types.Order{
		ID:          uuid.New(),
		Exchange:    pos.Exchange,
		Symbol:      pos.Symbol,
		Side:        types.OrderSideSell,
		Type:        types.OrderTypeMarket,
		Quantity:    pos.Quantity,
		Status:      types.OrderStatusPending,
		StrategyID:  signal.StrategyID,
		CreatedAt:   time.Now(),
	}

	return e.orders.PlaceOrder(order)
}

func (e *Engine) SetExchange(name string, ex ExchangeAdapter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.exchanges[name] = ex
}

func (e *Engine) GetExchange(name string) ExchangeAdapter {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.exchanges[name]
}

func (e *Engine) AddStrategy(name string, strat StrategyRunner) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.strategies[name]; exists {
		return fmt.Errorf("strategy %s already exists", name)
	}

	if err := strat.Init(e.ctx, e); err != nil {
		return fmt.Errorf("failed to initialize strategy: %w", err)
	}

	e.strategies[name] = strat
	return nil
}

func (e *Engine) RemoveStrategy(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	strat, exists := e.strategies[name]
	if !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	if err := strat.Stop(); err != nil {
		return fmt.Errorf("failed to stop strategy: %w", err)
	}

	delete(e.strategies, name)
	return nil
}

func (e *Engine) GetStrategy(name string) StrategyRunner {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.strategies[name]
}

func (e *Engine) GetStrategies() map[string]StrategyRunner {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]StrategyRunner, len(e.strategies))
	for k, v := range e.strategies {
		result[k] = v
	}
	return result
}

func (e *Engine) SetMarketData(md MarketDataAdapter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.marketData = md
}

func (e *Engine) GetMarketData() MarketDataAdapter {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.marketData
}

func (e *Engine) GetState() State {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

func (e *Engine) setState(state State) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = state
}

func (e *Engine) GetMode() Mode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.mode
}

func (e *Engine) SetMode(mode Mode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mode = mode
	e.config.Mode = mode
}

func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

func (e *Engine) GetUptime() time.Duration {
	return time.Since(e.startTime)
}

func (e *Engine) Subscribe(ch chan<- Event) {
	go func() {
		for {
			select {
			case event := <-e.events:
				ch <- event
			case <-e.ctx.Done():
				return
			}
		}
	}()
}

func (e *Engine) Publish(event Event) {
	select {
	case e.events <- event:
	default:
		logger.Warn("Event channel full, dropping event", "type", event.Type)
	}
}

func calculatePnL(pos *types.Position) decimal.Decimal {
	if pos.Quantity.IsZero() {
		return decimal.Zero
	}

	entryValue := pos.AvgEntryPrice.Mul(pos.Quantity)
	currentValue := pos.CurrentPrice.Mul(pos.Quantity)

	var pnl decimal.Decimal
	if pos.Side == types.PositionSideLong {
		pnl = currentValue.Sub(entryValue)
	} else {
		pnl = entryValue.Sub(currentValue)
	}

	return pnl
}

type Portfolio struct {
	engine    *Engine
	balances  map[string]*types.Balance
	positions map[string]*types.Position
	mu        sync.RWMutex
}

func NewPortfolio(e *Engine) *Portfolio {
	return &Portfolio{
		engine:    e,
		balances:  make(map[string]*types.Balance),
		positions: make(map[string]*types.Position),
	}
}

func (p *Portfolio) GetBalance(asset string) *types.Balance {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.balances[asset]
}

func (p *Portfolio) GetAllBalances() map[string]*types.Balance {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*types.Balance, len(p.balances))
	for k, v := range p.balances {
		result[k] = v
	}
	return result
}

func (p *Portfolio) UpdateBalance(asset string, amount decimal.Decimal, subtract bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	balance, exists := p.balances[asset]
	if !exists {
		balance = &types.Balance{
			Asset: asset,
			Free:  decimal.Zero,
			Locked: decimal.Zero,
		}
		p.balances[asset] = balance
	}

	if subtract {
		balance.Free = balance.Free.Sub(amount)
	} else {
		balance.Free = balance.Free.Add(amount)
	}
}

func (p *Portfolio) TotalValue(quoteAsset string) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := decimal.Zero
	for _, balance := range p.balances {
		if balance.Asset == quoteAsset {
			total = total.Add(balance.Total)
		}
	}
	return total
}

type OrderManager struct {
	engine  *Engine
	orders  map[uuid.UUID]*types.Order
	mu      sync.RWMutex
}

func NewOrderManager(e *Engine) *OrderManager {
	return &OrderManager{
		engine: e,
		orders: make(map[uuid.UUID]*types.Order),
	}
}

func (m *OrderManager) PlaceOrder(order *types.Order) error {
	m.mu.Lock()
	m.orders[order.ID] = order
	m.mu.Unlock()

	m.engine.Publish(Event{
		Type:    EventOrderPlaced,
		Payload: order,
		Time:    time.Now(),
	})

	go m.executeOrder(order)
	return nil
}

func (m *OrderManager) executeOrder(order *types.Order) {
	order.Status = types.OrderStatusOpen
	order.UpdatedAt = time.Now()

	m.engine.Publish(Event{
		Type:    EventOrderPlaced,
		Payload: order,
		Time:    time.Now(),
	})

	time.Sleep(100 * time.Millisecond)

	order.Status = types.OrderStatusFilled
	order.FilledQuantity = order.Quantity
	order.AvgFillPrice = order.Price
	order.FilledAt = new(time.Time)
	*order.FilledAt = time.Now()
	order.UpdatedAt = time.Now()

	m.engine.Publish(Event{
		Type:    EventOrderFilled,
		Payload: order,
		Time:    time.Now(),
	})
}

func (m *OrderManager) CancelOrder(orderID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}

	if order.Status == types.OrderStatusFilled {
		return fmt.Errorf("cannot cancel filled order")
	}

	order.Status = types.OrderStatusCancelled
	order.CancelledAt = new(time.Time)
	*order.CancelledAt = time.Now()
	order.UpdatedAt = time.Now()

	m.engine.Publish(Event{
		Type:    EventOrderCancelled,
		Payload: order,
		Time:    time.Now(),
	})

	return nil
}

func (m *OrderManager) GetOrder(orderID uuid.UUID) *types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.orders[orderID]
}

func (m *OrderManager) GetAllOrders() []*types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*types.Order, 0, len(m.orders))
	for _, order := range m.orders {
		orders = append(orders, order)
	}
	return orders
}

func (m *OrderManager) GetOpenOrders() []*types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var orders []*types.Order
	for _, order := range m.orders {
		if order.Status == types.OrderStatusOpen || order.Status == types.OrderStatusPartiallyFilled {
			orders = append(orders, order)
		}
	}
	return orders
}

func (m *OrderManager) RemoveOrder(orderID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.orders, orderID)
}

type PositionManager struct {
	engine    *Engine
	positions map[string]*types.Position
	mu        sync.RWMutex
}

func NewPositionManager(e *Engine) *PositionManager {
	return &PositionManager{
		engine:    e,
		positions: make(map[string]*types.Position),
	}
}

func (m *PositionManager) GetPosition(symbol string) *types.Position {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.positions[symbol]
}

func (m *PositionManager) GetAll() []*types.Position {
	m.mu.RLock()
	defer m.mu.RUnlock()

	positions := make([]*types.Position, 0, len(m.positions))
	for _, pos := range m.positions {
		positions = append(positions, pos)
	}
	return positions
}

func (m *PositionManager) OpenPosition(pos *types.Position) {
	m.mu.Lock()
	m.positions[pos.Symbol] = pos
	m.mu.Unlock()

	m.engine.Publish(Event{
		Type:    EventPositionOpened,
		Payload: pos,
		Time:    time.Now(),
	})
}

func (m *PositionManager) ClosePosition(symbol string) error {
	m.mu.Lock()
	pos, exists := m.positions[symbol]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("position not found")
	}
	delete(m.positions, symbol)
	m.mu.Unlock()

	m.engine.Publish(Event{
		Type:    EventPositionClosed,
		Payload: pos,
		Time:    time.Now(),
	})

	return nil
}

func (m *PositionManager) UpdatePosition(pos *types.Position) {
	m.mu.Lock()
	m.positions[pos.Symbol] = pos
	m.mu.Unlock()

	m.engine.Publish(Event{
		Type:    EventPositionUpdated,
		Payload: pos,
		Time:    time.Now(),
	})
}

func (m *PositionManager) UpdateFromOrder(order *types.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pos, exists := m.positions[order.Symbol]
	if !exists {
		pos = &types.Position{
			ID:            uuid.New(),
			Exchange:      order.Exchange,
			Symbol:        order.Symbol,
			AssetType:     order.AssetType,
			Quantity:      decimal.Zero,
			AvgEntryPrice: decimal.Zero,
			OpenedAt:      time.Now(),
		}
		m.positions[order.Symbol] = pos
	}

	if order.Side == types.OrderSideBuy {
		totalQty := pos.Quantity.Add(order.FilledQuantity)
		totalCost := pos.AvgEntryPrice.Mul(pos.Quantity).Add(order.AvgFillPrice.Mul(order.FilledQuantity))
		pos.AvgEntryPrice = totalCost.Div(totalQty)
		pos.Quantity = totalQty
		pos.Side = types.PositionSideLong
	} else {
		pos.Quantity = pos.Quantity.Sub(order.FilledQuantity)
		if pos.Quantity.IsZero() {
			delete(m.positions, order.Symbol)
		}
	}

	pos.UpdatedAt = time.Now()
}

type RiskManager struct {
	engine *Engine
	config *RiskConfig
	mu     sync.RWMutex
}

type RiskConfig struct {
	MaxPositionSize   decimal.Decimal
	MaxOrderSize      decimal.Decimal
	MaxDailyLoss      decimal.Decimal
	MaxDrawdown       decimal.Decimal
	MaxExposure       decimal.Decimal
	MaxLeverage       decimal.Decimal
	MinAccountBalance decimal.Decimal
}

func NewRiskManager(e *Engine) *RiskManager {
	return &RiskManager{
		engine: e,
		config: &RiskConfig{
			MaxPositionSize:   decimal.NewFromFloat(1.0),
			MaxOrderSize:      decimal.NewFromFloat(0.5),
			MaxDailyLoss:      decimal.NewFromFloat(0.1),
			MaxDrawdown:       decimal.NewFromFloat(0.2),
			MaxExposure:       decimal.NewFromFloat(0.8),
			MaxLeverage:       decimal.NewFromFloat(3),
			MinAccountBalance: decimal.NewFromFloat(100),
		},
	}
}

func (r *RiskManager) CheckAllLimits() error {
	return nil
}

func (r *RiskManager) ValidateSignal(signal *types.Signal) bool {
	return true
}

func (r *RiskManager) ValidateOrder(order *types.Order) error {
	if order.Quantity.GreaterThan(r.config.MaxOrderSize) {
		return fmt.Errorf("order size exceeds maximum: %s > %s", order.Quantity, r.config.MaxOrderSize)
	}
	return nil
}

func (r *RiskManager) CalculatePositionSize(symbol string, risk decimal.Decimal, stopLoss decimal.Decimal) decimal.Decimal {
	return decimal.NewFromFloat(1000)
}

func (r *RiskManager) GetExposure() decimal.Decimal {
	return decimal.NewFromFloat(0.45)
}

func (r *RiskManager) GetDailyLoss() decimal.Decimal {
	return decimal.NewFromFloat(0.02)
}

func (r *RiskManager) UpdateConfig(cfg *RiskConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = cfg
}

func (r *RiskManager) GetConfig() *RiskConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

var _ atomic.Value
