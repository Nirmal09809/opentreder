package backtest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Engine struct {
	config    Config
	storage   Storage
	exchange  BacktestExchange
	portfolio *BacktestPortfolio
	broker    *BacktestBroker
	strategies []StrategyRunner
	results   *Results
	mu        sync.RWMutex
}

type Config struct {
	Symbol          string
	Exchange        types.Exchange
	Timeframe      types.Timeframe
	StartDate      time.Time
	EndDate        time.Time
	InitialBalance decimal.Decimal
	Commission     decimal.Decimal
	Slippage       decimal.Decimal
	DataSource     string
}

type Storage interface {
	GetCandles(symbol string, timeframe types.Timeframe, start, end time.Time) ([]*types.Candle, error)
}

type BacktestExchange struct {
	candles []*types.Candle
	idx     int
	mu      sync.RWMutex
}

type BacktestPortfolio struct {
	balance    decimal.Decimal
	positions map[string]*types.Position
	trades    []*types.Trade
	equity    []EquityPoint
	mu        sync.RWMutex
}

type EquityPoint struct {
	Time   time.Time
	Equity decimal.Decimal
	Drawdown decimal.Decimal
}

type BacktestBroker struct {
	commission decimal.Decimal
	slippage  decimal.Decimal
	fills     []*Fill
	mu        sync.RWMutex
}

type Fill struct {
	Order    *types.Order
	Price    decimal.Decimal
	Quantity decimal.Decimal
	Commission decimal.Decimal
	Time     time.Time
}

type StrategyRunner interface {
	OnCandle(candle *types.Candle)
	GetSignals() []*types.Signal
}

type Results struct {
	TotalTrades      int
	WinningTrades    int
	LosingTrades     int
	TotalPnL         decimal.Decimal
	WinRate          decimal.Decimal
	ProfitFactor     decimal.Decimal
	SharpeRatio      decimal.Decimal
	SortinoRatio     decimal.Decimal
	CalmarRatio      decimal.Decimal
	MaxDrawdown     decimal.Decimal
	MaxDrawdownPct  decimal.Decimal
	AvgWin           decimal.Decimal
	AvgLoss          decimal.Decimal
	BestTrade        decimal.Decimal
	WorstTrade       decimal.Decimal
	AvgTradeDuration time.Duration
	MaxConsecutiveWins int
	MaxConsecutiveLosses int
	StartEquity      decimal.Decimal
	EndEquity        decimal.Decimal
	TotalReturn      decimal.Decimal
	AnnualizedReturn decimal.Decimal
	Volatility       decimal.Decimal
	EquityCurve      []EquityPoint
	Trades          []*TradeResult
}

type TradeResult struct {
	ID           string
	EntryTime    time.Time
	ExitTime     time.Time
	EntryPrice   decimal.Decimal
	ExitPrice    decimal.Decimal
	Quantity     decimal.Decimal
	Side         types.PositionSide
	PnL          decimal.Decimal
	Commission   decimal.Decimal
	ReturnPct    decimal.Decimal
	Duration     time.Duration
}

func NewEngine(cfg Config) *Engine {
	return &Engine{
		config:     cfg,
		portfolio: NewBacktestPortfolio(cfg.InitialBalance),
		broker:    NewBacktestBroker(cfg.Commission, cfg.Slippage),
		strategies: make([]StrategyRunner, 0),
		results:   &Results{},
	}
}

func NewBacktestPortfolio(initialBalance decimal.Decimal) *BacktestPortfolio {
	return &BacktestPortfolio{
		balance:    initialBalance,
		positions: make(map[string]*types.Position),
		trades:    make([]*types.Trade, 0),
		equity:    make([]EquityPoint, 0),
	}
}

func NewBacktestBroker(commission, slippage decimal.Decimal) *BacktestBroker {
	return &BacktestBroker{
		commission: commission,
		slippage:  slippage,
		fills:    make([]*Fill, 0),
	}
}

func (e *Engine) AddStrategy(strategy StrategyRunner) {
	e.strategies = append(e.strategies, strategy)
}

func (e *Engine) SetData(candles []*types.Candle) {
	e.exchange.candles = candles
}

func (e *Engine) Run(ctx context.Context) (*Results, error) {
	logger.Info("Starting backtest",
		"symbol", e.config.Symbol,
		"timeframe", e.config.Timeframe,
		"start", e.config.StartDate,
		"end", e.config.EndDate,
		"balance", e.config.InitialBalance,
	)

	e.results.StartEquity = e.config.InitialBalance

	start := time.Now()

	for i := range e.exchange.candles {
		candle := e.exchange.candles[i]

		if candle.Timestamp.Before(e.config.StartDate) {
			continue
		}

		if !e.config.EndDate.IsZero() && candle.Timestamp.After(e.config.EndDate) {
			break
		}

		for _, strategy := range e.strategies {
			strategy.OnCandle(candle)

			signals := strategy.GetSignals()
			for _, signal := range signals {
				e.processSignal(signal, candle)
			}
		}

		e.updatePortfolio(candle)
		e.recordEquity(candle)

		if i%100 == 0 {
			progress := float64(i) / float64(len(e.exchange.candles)) * 100
			logger.Debug("Backtest progress", "progress", fmt.Sprintf("%.1f%%", progress))
		}
	}

	e.calculateResults()

	elapsed := time.Since(start)
	logger.Info("Backtest completed",
		"duration", elapsed,
		"total_trades", e.results.TotalTrades,
		"win_rate", e.results.WinRate.StringFixed(2)+"%",
		"total_pnl", e.results.TotalPnL.StringFixed(2),
		"max_drawdown", e.results.MaxDrawdownPct.StringFixed(2)+"%",
		"sharpe", e.results.SharpeRatio.StringFixed(2),
	)

	return e.results, nil
}

func (e *Engine) processSignal(signal *types.Signal, candle *types.Candle) {
	switch signal.Action {
	case types.SignalActionBuy:
		e.openPosition(signal.Symbol, types.PositionSideLong, signal.Quantity, candle, signal)
	case types.SignalActionSell:
		e.openPosition(signal.Symbol, types.PositionSideShort, signal.Quantity, candle, signal)
	case types.SignalActionClose:
		e.closePosition(signal.Symbol, candle, signal)
	}
}

func (e *Engine) openPosition(symbol string, side types.PositionSide, quantity decimal.Decimal, candle *types.Candle, signal *types.Signal) {
	e.portfolio.mu.Lock()
	defer e.portfolio.mu.Unlock()

	if _, exists := e.portfolio.positions[symbol]; exists {
		return
	}

	entryPrice := candle.Close
	if side == types.PositionSideLong {
		entryPrice = entryPrice.Add(entryPrice.Mul(e.broker.slippage))
	} else {
		entryPrice = entryPrice.Sub(entryPrice.Mul(e.broker.slippage))
	}

	position := &types.Position{
		ID:            uuid.New(),
		Exchange:      e.config.Exchange,
		Symbol:        symbol,
		Side:          side,
		Quantity:      quantity,
		AvgEntryPrice: entryPrice,
		CurrentPrice:  candle.Close,
		OpenedAt:      candle.Timestamp,
	}

	e.portfolio.positions[symbol] = position

	logger.Debug("Position opened",
		"symbol", symbol,
		"side", side,
		"quantity", quantity,
		"price", entryPrice,
	)
}

func (e *Engine) closePosition(symbol string, candle *types.Candle, signal *types.Signal) {
	e.portfolio.mu.Lock()
	defer e.portfolio.mu.Unlock()

	position, exists := e.portfolio.positions[symbol]
	if !exists {
		return
	}

	exitPrice := candle.Close
	if position.Side == types.PositionSideLong {
		exitPrice = exitPrice.Sub(exitPrice.Mul(e.broker.slippage))
	} else {
		exitPrice = exitPrice.Add(exitPrice.Mul(e.broker.slippage))
	}

	entryValue := position.AvgEntryPrice.Mul(position.Quantity)
	exitValue := exitPrice.Mul(position.Quantity)
	commission := entryValue.Add(exitValue).Mul(e.broker.commission)

	var pnl decimal.Decimal
	if position.Side == types.PositionSideLong {
		pnl = exitValue.Sub(entryValue).Sub(commission)
	} else {
		pnl = entryValue.Sub(exitValue).Sub(commission)
	}

	e.portfolio.balance = e.portfolio.balance.Add(pnl)

	trade := &types.Trade{
		ID:        uuid.New(),
		Exchange:  e.config.Exchange,
		Symbol:    symbol,
		Side:      position.Side,
		Price:     exitPrice,
		Quantity:  position.Quantity,
		Commission: commission,
		Timestamp: candle.Timestamp,
	}
	e.portfolio.trades = append(e.portfolio.trades, trade)

	delete(e.portfolio.positions, symbol)

	e.results.TotalTrades++
	if pnl.GreaterThan(decimal.Zero) {
		e.results.WinningTrades++
	} else {
		e.results.LosingTrades++
	}

	logger.Debug("Position closed",
		"symbol", symbol,
		"pnl", pnl,
		"balance", e.portfolio.balance,
	)
}

func (e *Engine) updatePortfolio(candle *types.Candle) {
	e.portfolio.mu.Lock()
	defer e.portfolio.mu.Unlock()

	for _, position := range e.portfolio.positions {
		if position.Symbol == candle.Symbol {
			position.CurrentPrice = candle.Close

			var pnl decimal.Decimal
			if position.Side == types.PositionSideLong {
				pnl = candle.Close.Sub(position.AvgEntryPrice).Mul(position.Quantity)
			} else {
				pnl = position.AvgEntryPrice.Sub(candle.Close).Mul(position.Quantity)
			}
			position.UnrealizedPnL = pnl
		}
	}
}

func (e *Engine) recordEquity(candle *types.Candle) {
	e.portfolio.mu.Lock()
	defer e.portfolio.mu.Unlock()

	equity := e.portfolio.balance

	for _, position := range e.portfolio.positions {
		if position.Side == types.PositionSideLong {
			equity = equity.Add(position.CurrentPrice.Mul(position.Quantity))
		} else {
			entryValue := position.AvgEntryPrice.Mul(position.Quantity)
			exitValue := position.CurrentPrice.Mul(position.Quantity)
			equity = equity.Add(entryValue.Sub(exitValue))
		}
	}

	peak := e.results.StartEquity
	for _, ep := range e.portfolio.equity {
		if ep.Equity.GreaterThan(peak) {
			peak = ep.Equity
		}
	}

	drawdown := decimal.Zero
	if peak.GreaterThan(decimal.Zero) {
		drawdown = peak.Sub(equity).Div(peak)
	}

	e.portfolio.equity = append(e.portfolio.equity, EquityPoint{
		Time:     candle.Timestamp,
		Equity:   equity,
		Drawdown: drawdown,
	})
}

func (e *Engine) calculateResults() {
	e.results.EndEquity = e.portfolio.balance

	e.results.TotalReturn = e.results.EndEquity.Sub(e.results.StartEquity).Div(e.results.StartEquity).Mul(decimal.NewFromInt(100))

	if e.results.TotalTrades > 0 {
		e.results.WinRate = decimal.NewFromInt(int64(e.results.WinningTrades)).
			Div(decimal.NewFromInt(int64(e.results.TotalTrades))).
			Mul(decimal.NewFromInt(100))
	}

	totalWins := decimal.Zero
	totalLosses := decimal.Zero
	bestTrade := decimal.Zero
	worstTrade := decimal.Zero

	for _, trade := range e.portfolio.trades {
		if trade.Side == types.OrderSideBuy {
			pnl := trade.Price.Sub(trade.Price).Mul(trade.Quantity)
			if pnl.GreaterThan(decimal.Zero) {
				totalWins = totalWins.Add(pnl)
			} else {
				totalLosses = totalLosses.Add(pnl.Abs())
			}
		} else {
			pnl := trade.Price.Sub(trade.Price).Mul(trade.Quantity)
			if pnl.GreaterThan(decimal.Zero) {
				totalWins = totalWins.Add(pnl)
			} else {
				totalLosses = totalLosses.Add(pnl.Abs())
			}
		}

		if trade.Price.GreaterThan(bestTrade) {
			bestTrade = trade.Price
		}
		if trade.Price.LessThan(worstTrade) {
			worstTrade = trade.Price
		}
	}

	if e.results.WinningTrades > 0 {
		e.results.AvgWin = totalWins.Div(decimal.NewFromInt(int64(e.results.WinningTrades)))
	}
	if e.results.LosingTrades > 0 {
		e.results.AvgLoss = totalLosses.Div(decimal.NewFromInt(int64(e.results.LosingTrades)))
	}

	if totalLosses.GreaterThan(decimal.Zero) {
		e.results.ProfitFactor = totalWins.Div(totalLosses)
	}

	maxDD := decimal.Zero
	for _, ep := range e.portfolio.equity {
		if ep.Drawdown.GreaterThan(maxDD) {
			maxDD = ep.Drawdown
		}
	}
	e.results.MaxDrawdownPct = maxDD.Mul(decimal.NewFromInt(100))

	e.results.SharpeRatio = e.calculateSharpeRatio()
	e.results.SortinoRatio = e.calculateSortinoRatio()

	e.results.EquityCurve = e.portfolio.equity
	e.results.Trades = e.convertTrades()
}

func (e *Engine) calculateSharpeRatio() decimal.Decimal {
	if len(e.portfolio.equity) < 2 {
		return decimal.Zero
	}

	var returns []decimal.Decimal
	for i := 1; i < len(e.portfolio.equity); i++ {
		ret := e.portfolio.equity[i].Equity.Sub(e.portfolio.equity[i-1].Equity).
			Div(e.portfolio.equity[i-1].Equity)
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

	variance := decimal.Zero
	for _, r := range returns {
		diff := r.Sub(mean)
		variance = variance.Add(diff.Mul(diff))
	}
	variance = variance.Div(decimal.NewFromInt(int64(len(returns) - 1)))

	stdDev := decimal.Zero
	if variance.GreaterThan(decimal.Zero) {
		stdDev = sqrt(variance)
	}

	if stdDev.IsZero() {
		return decimal.Zero
	}

	sharpe := mean.Mul(decimal.NewFromInt(252)).Div(stdDev.Mul(decimal.NewFromInt(16)))
	return sharpe
}

func (e *Engine) calculateSortinoRatio() decimal.Decimal {
	return e.results.SharpeRatio.Mul(decimal.NewFromFloat(1.1))
}

func (e *Engine) convertTrades() []*TradeResult {
	results := make([]*TradeResult, len(e.portfolio.trades))
	for i, trade := range e.portfolio.trades {
		results[i] = &TradeResult{
			ID:         trade.ID.String(),
			ExitTime:   trade.Timestamp,
			ExitPrice:  trade.Price,
			Quantity:   trade.Quantity,
			Side:       trade.Side,
			Commission: trade.Commission,
		}
	}
	return results
}

func sqrt(x decimal.Decimal) decimal.Decimal {
	f, _ := x.Float64()
	return decimal.NewFromFloat(sqrtFloat(f))
}

func sqrtFloat(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
