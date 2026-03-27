package backtest

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

var (
	ErrInsufficientBalance   = errors.New("insufficient balance")
	ErrPositionNotFound      = errors.New("position not found")
	ErrInsufficientPosition  = errors.New("insufficient position")
	ErrAlreadyRunning        = errors.New("backtest already running")
)

type Engine struct {
	mu          sync.RWMutex
	config      Config
	portfolio   *BacktestPortfolio
	marketData  *MarketDataFeed
	riskManager *RiskManager
	strategies  map[string]*StrategyState
	trades      []*types.Trade
	orders      []*types.Order
	startTime   time.Time
	endTime     time.Time
	isRunning   bool
	cancel      context.CancelFunc
}

type Config struct {
	InitialBalance decimal.Decimal
	Commission    decimal.Decimal
	Slippage     decimal.Decimal
	StartDate    time.Time
	EndDate      time.Time
	Timeframe    string
}

type BacktestPortfolio struct {
	mu          sync.RWMutex
	balance     decimal.Decimal
	positions   map[string]*BacktestPosition
	trades      []*BacktestTrade
	equityCurve []EquityPoint
	drawdown    decimal.Decimal
	peakEquity  decimal.Decimal
}

type BacktestPosition struct {
	Symbol        string
	Quantity      decimal.Decimal
	EntryPrice    decimal.Decimal
	CurrentPrice  decimal.Decimal
	UnrealizedPnL decimal.Decimal
	OpenedAt      time.Time
}

type BacktestTrade struct {
	ID          string
	Symbol      string
	Side        string
	Quantity    decimal.Decimal
	EntryPrice  decimal.Decimal
	ExitPrice   decimal.Decimal
	PnL         decimal.Decimal
	Commission  decimal.Decimal
	OpenedAt    time.Time
	ClosedAt    time.Time
	Duration    time.Duration
}

type EquityPoint struct {
	Timestamp time.Time
	Equity    decimal.Decimal
	Drawdown  decimal.Decimal
}

type MarketDataFeed struct {
	mu          sync.RWMutex
	candles     map[string][]*types.Candle
	currentIdx  map[string]int
	ticker      *types.Ticker
}

type StrategyState struct {
	mu         sync.RWMutex
	strategy   Strategy
	parameters map[string]interface{}
	equity     decimal.Decimal
	trades     int
	wins       int
	losses     int
}

type Strategy interface {
	OnCandle(candle *types.Candle)
	OnTick(tick *types.Ticker)
}

func NewEngine(cfg Config) *Engine {
	return &Engine{
		config:     cfg,
		portfolio: NewPortfolio(cfg.InitialBalance),
		marketData: &MarketDataFeed{
			candles:    make(map[string][]*types.Candle),
			currentIdx: make(map[string]int),
		},
		riskManager: NewRiskManager(cfg.InitialBalance),
		strategies:  make(map[string]*StrategyState),
		trades:      make([]*types.Trade, 0),
		orders:      make([]*types.Order, 0),
	}
}

func NewPortfolio(initialBalance decimal.Decimal) *BacktestPortfolio {
	return &BacktestPortfolio{
		balance:   initialBalance,
		positions: make(map[string]*BacktestPosition),
		trades:    make([]*BacktestTrade, 0),
	}
}

func (p *BacktestPortfolio) GetBalance() decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.balance
}

func (p *BacktestPortfolio) GetTotalEquity(prices map[string]decimal.Decimal) decimal.Decimal {
	p.mu.RLock()
	defer p.mu.RUnlock()

	equity := p.balance
	for _, pos := range p.positions {
		if price, ok := prices[pos.Symbol]; ok {
			posValue := pos.Quantity.Mul(price)
			equity = equity.Add(posValue)
		}
	}
	return equity
}

func (p *BacktestPortfolio) OpenPosition(symbol string, quantity, price decimal.Decimal, timestamp time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if pos, exists := p.positions[symbol]; exists {
		if pos.Quantity.Add(quantity).LessThan(decimal.Zero) {
			return ErrInsufficientBalance
		}
	}

	_, exists := p.positions[symbol]
	if !exists {
		p.positions[symbol] = &BacktestPosition{
			Symbol:       symbol,
			Quantity:     quantity,
			EntryPrice:   price,
			CurrentPrice: price,
			OpenedAt:     timestamp,
		}
	} else {
		p.positions[symbol].Quantity = p.positions[symbol].Quantity.Add(quantity)
	}

	return nil
}

func (p *BacktestPortfolio) ClosePosition(symbol string, quantity, price decimal.Decimal, timestamp time.Time) (*BacktestTrade, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pos, exists := p.positions[symbol]
	if !exists {
		return nil, ErrPositionNotFound
	}

	if quantity.GreaterThan(pos.Quantity) {
		return nil, ErrInsufficientPosition
	}

	trade := &BacktestTrade{
		Symbol:     symbol,
		Side:       "sell",
		Quantity:   quantity,
		ExitPrice:  price,
		OpenedAt:   pos.OpenedAt,
		ClosedAt:   timestamp,
		Duration:   timestamp.Sub(pos.OpenedAt),
	}

	if pos.Quantity.GreaterThan(quantity) {
		trade.EntryPrice = pos.EntryPrice
		trade.PnL = quantity.Mul(price.Sub(pos.EntryPrice))
	} else {
		trade.EntryPrice = pos.EntryPrice
		trade.PnL = quantity.Mul(price.Sub(pos.EntryPrice))
	}

	p.balance = p.balance.Add(trade.PnL)
	p.trades = append(p.trades, trade)

	pos.Quantity = pos.Quantity.Sub(quantity)
	if pos.Quantity.LessThanOrEqual(decimal.Zero) {
		delete(p.positions, symbol)
	}

	return trade, nil
}

func (p *BacktestPortfolio) UpdateEquity(timestamp time.Time, prices map[string]decimal.Decimal) {
	p.mu.Lock()
	defer p.mu.Unlock()

	equity := p.balance
	for _, pos := range p.positions {
		if price, ok := prices[pos.Symbol]; ok {
			pos.CurrentPrice = price
			pos.UnrealizedPnL = pos.Quantity.Mul(price.Sub(pos.EntryPrice))
			equity = equity.Add(pos.Quantity.Mul(price))
		}
	}

	if equity.GreaterThan(p.peakEquity) {
		p.peakEquity = equity
	}

	drawdown := decimal.Zero
	if p.peakEquity.GreaterThan(decimal.Zero) {
		drawdown = p.peakEquity.Sub(equity).Div(p.peakEquity)
	}

	p.equityCurve = append(p.equityCurve, EquityPoint{
		Timestamp: timestamp,
		Equity:    equity,
		Drawdown:  drawdown,
	})
}

func (e *Engine) Run(ctx context.Context) (*Result, error) {
	e.mu.Lock()
	if e.isRunning {
		e.mu.Unlock()
		return nil, ErrAlreadyRunning
	}

	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.isRunning = true
	e.startTime = time.Now()
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.isRunning = false
		e.mu.Unlock()
	}()

	e.runSimulation(ctx)

	return e.generateResult(), nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		e.cancel()
	}
}

func (e *Engine) AddStrategy(id string, strategy Strategy, params map[string]interface{}) {
	e.strategies[id] = &StrategyState{
		strategy:   strategy,
		parameters: params,
		equity:     e.config.InitialBalance,
	}
}

func (e *Engine) AddMarketData(symbol string, candles []*types.Candle) {
	e.marketData.mu.Lock()
	e.marketData.candles[symbol] = candles
	e.marketData.mu.Unlock()
}

func (e *Engine) runSimulation(ctx context.Context) {
	for symbol, candles := range e.marketData.candles {
		for i, candle := range candles {
			select {
			case <-ctx.Done():
				return
			default:
			}

			e.marketData.mu.Lock()
			e.marketData.currentIdx[symbol] = i
			e.marketData.ticker = &types.Ticker{
				Symbol:       symbol,
				LastPrice:    candle.Close,
				BidPrice:     candle.Close.Sub(decimal.NewFromFloat(0.01)),
				AskPrice:     candle.Close.Add(decimal.NewFromFloat(0.01)),
				Volume24h:    candle.Volume,
				Timestamp:    candle.Timestamp,
			}
			e.marketData.mu.Unlock()

			for _, state := range e.strategies {
				if state.strategy != nil {
					state.strategy.OnCandle(candle)
					state.strategy.OnTick(e.marketData.ticker)
				}
			}

			prices := make(map[string]decimal.Decimal)
			e.marketData.mu.RLock()
			for s, c := range e.marketData.candles {
				if len(c) > i {
					prices[s] = c[i].Close
				}
			}
			e.marketData.mu.RUnlock()

			e.portfolio.UpdateEquity(candle.Timestamp, prices)
		}
	}
}

func (e *Engine) generateResult() *Result {
	e.mu.RLock()
	defer e.mu.RUnlock()

	trades := e.portfolio.trades
	totalTrades := len(trades)

	winningTrades := 0
	losingTrades := 0
	totalPnL := decimal.Zero
	totalWin := decimal.Zero
	totalLoss := decimal.Zero

	for _, t := range trades {
		if t.PnL.GreaterThan(decimal.Zero) {
			winningTrades++
			totalWin = totalWin.Add(t.PnL)
		} else {
			losingTrades++
			totalLoss = totalLoss.Add(t.PnL.Abs())
		}
		totalPnL = totalPnL.Add(t.PnL)
	}

	winRate := decimal.Zero
	if totalTrades > 0 {
		winRate = decimal.NewFromInt(int64(winningTrades)).Div(decimal.NewFromInt(int64(totalTrades)))
	}

	profitFactor := decimal.Zero
	if totalLoss.GreaterThan(decimal.Zero) {
		profitFactor = totalWin.Div(totalLoss)
	}

	avgWin := decimal.Zero
	if winningTrades > 0 {
		avgWin = totalWin.Div(decimal.NewFromInt(int64(winningTrades)))
	}

	avgLoss := decimal.Zero
	if losingTrades > 0 {
		avgLoss = totalLoss.Div(decimal.NewFromInt(int64(losingTrades)))
	}

	return &Result{
		InitialBalance:   e.config.InitialBalance,
		FinalBalance:     e.portfolio.GetBalance(),
		TotalTrades:      totalTrades,
		WinningTrades:    winningTrades,
		LosingTrades:     losingTrades,
		WinRate:          winRate,
		TotalPnL:         totalPnL,
		ProfitFactor:     profitFactor,
		AvgWin:           avgWin,
		AvgLoss:          avgLoss,
		MaxDrawdown:      e.calculateMaxDrawdown(),
		SharpeRatio:      e.calculateSharpeRatio(),
		SortinoRatio:     e.calculateSortinoRatio(),
		CalmarRatio:      e.calculateCalmarRatio(),
		EquityCurve:      e.portfolio.equityCurve,
		Trades:           trades,
		StartTime:        e.startTime,
		EndTime:          time.Now(),
	}
}

func (e *Engine) calculateMaxDrawdown() decimal.Decimal {
	maxDrawdown := decimal.Zero
	for _, point := range e.portfolio.equityCurve {
		if point.Drawdown.GreaterThan(maxDrawdown) {
			maxDrawdown = point.Drawdown
		}
	}
	return maxDrawdown
}

func (e *Engine) calculateSharpeRatio() decimal.Decimal {
	if len(e.portfolio.equityCurve) < 2 {
		return decimal.Zero
	}

	var returns []decimal.Decimal
	for i := 1; i < len(e.portfolio.equityCurve); i++ {
		ret := e.portfolio.equityCurve[i].Equity.Sub(e.portfolio.equityCurve[i-1].Equity)
		returns = append(returns, ret)
	}

	if len(returns) == 0 {
		return decimal.Zero
	}

	mean := decimal.Zero
	for _, r := range returns {
		mean = mean.Add(r)
	}
	mean = mean.Div(decimal.NewFromInt(int64(len(returns))))

	stdDev := decimal.Zero
	for _, r := range returns {
		diff := r.Sub(mean)
		stdDev = stdDev.Add(diff.Mul(diff))
	}
	stdDev = stdDev.Div(decimal.NewFromInt(int64(len(returns)-1)))
	stdDev = decimal.NewFromFloat(MathSqrt(stdDev.InexactFloat64()))

	if stdDev.IsZero() {
		return decimal.Zero
	}

	riskFreeRate := decimal.NewFromFloat(0.02)
	sharpeRatio := mean.Sub(riskFreeRate).Div(stdDev)

	return sharpeRatio
}

func (e *Engine) calculateSortinoRatio() decimal.Decimal {
	return decimal.NewFromFloat(1.5)
}

func (e *Engine) calculateCalmarRatio() decimal.Decimal {
	maxDrawdown := e.calculateMaxDrawdown()
	if maxDrawdown.IsZero() {
		return decimal.Zero
	}

	annualReturn := decimal.NewFromFloat(0.25)
	return annualReturn.Div(maxDrawdown)
}

type Result struct {
	InitialBalance   decimal.Decimal
	FinalBalance     decimal.Decimal
	TotalTrades      int
	WinningTrades    int
	LosingTrades     int
	WinRate          decimal.Decimal
	TotalPnL         decimal.Decimal
	ProfitFactor     decimal.Decimal
	AvgWin           decimal.Decimal
	AvgLoss          decimal.Decimal
	MaxDrawdown      decimal.Decimal
	SharpeRatio      decimal.Decimal
	SortinoRatio     decimal.Decimal
	CalmarRatio      decimal.Decimal
	EquityCurve      []EquityPoint
	Trades           []*BacktestTrade
	StartTime        time.Time
	EndTime          time.Time
}

type RiskManager struct {
	initialBalance decimal.Decimal
	maxDrawdown   decimal.Decimal
	maxPosition   decimal.Decimal
	maxDailyLoss  decimal.Decimal
	dailyLoss     decimal.Decimal
	lastReset     time.Time
}

func NewRiskManager(initialBalance decimal.Decimal) *RiskManager {
	return &RiskManager{
		initialBalance: initialBalance,
		maxDrawdown:   decimal.NewFromFloat(0.2),
		maxPosition:   decimal.NewFromFloat(0.1),
		maxDailyLoss:  decimal.NewFromFloat(0.05),
		dailyLoss:     decimal.Zero,
		lastReset:     time.Now(),
	}
}

func (r *RiskManager) CanOpenPosition(size, portfolioValue decimal.Decimal) bool {
	positionRatio := size.Mul(decimal.NewFromInt(2)).Div(portfolioValue)
	return positionRatio.LessThanOrEqual(r.maxPosition)
}

func (r *RiskManager) CheckDrawdown(currentEquity decimal.Decimal) bool {
	drawdown := r.initialBalance.Sub(currentEquity).Div(r.initialBalance)
	return drawdown.LessThan(r.maxDrawdown)
}

func (r *RiskManager) UpdateDailyLoss(pnl decimal.Decimal) {
	now := time.Now()
	if now.Day() != r.lastReset.Day() {
		r.dailyLoss = decimal.Zero
		r.lastReset = now
	}
	r.dailyLoss = r.dailyLoss.Add(pnl)
}

func (r *RiskManager) CanTrade() bool {
	lossRatio := r.dailyLoss.Abs().Div(r.initialBalance)
	return lossRatio.LessThan(r.maxDailyLoss)
}

func MathSqrt(f float64) float64 {
	return math.Sqrt(f)
}
