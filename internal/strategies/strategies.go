package strategies

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type BaseStrategy struct {
	ID          string
	Name        string
	Type        string
	Config      StrategyConfig
	state       StrategyState
	positions   map[string]*types.Position
	signals     []*types.Signal
	candles     map[string][]*types.Candle
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

type StrategyConfig struct {
	Symbol          string
	Exchange        types.Exchange
	Timeframe       types.Timeframe
	Enabled         bool
	AutoRestart     bool
	RiskPerTrade    decimal.Decimal
	MaxPositions    int
	StopLoss        decimal.Decimal
	TakeProfit      decimal.Decimal
	Parameters      map[string]interface{}
}

type StrategyState string

const (
	StateIdle    StrategyState = "idle"
	StateRunning StrategyState = "running"
	StatePaused  StrategyState = "paused"
	StateStopped StrategyState = "stopped"
	StateError   StrategyState = "error"
)

type Strategy interface {
	Init(ctx context.Context) error
	Start() error
	Stop() error
	OnCandle(candle *types.Candle)
	OnSignal(signal *types.Signal)
	GetSignals() []*types.Signal
	GetState() StrategyState
	GetMetrics() StrategyMetrics
	GetID() string
	GetName() string
}

type StrategyMetrics struct {
	TotalTrades      int
	WinningTrades    int
	LosingTrades     int
	TotalPnL         decimal.Decimal
	WinRate          decimal.Decimal
	AvgWin           decimal.Decimal
	AvgLoss          decimal.Decimal
	MaxDrawdown      decimal.Decimal
	SharpeRatio      decimal.Decimal
	ProfitFactor     decimal.Decimal
	LastTradeTime    time.Time
	UpTime           time.Duration
}

func NewBaseStrategy(name, strategyType string) *BaseStrategy {
	return &BaseStrategy{
		ID:        fmt.Sprintf("strat_%s_%d", name, time.Now().Unix()),
		Name:      name,
		Type:      strategyType,
		state:     StateIdle,
		positions: make(map[string]*types.Position),
		signals:   make([]*types.Signal, 0),
		candles:   make(map[string][]*types.Candle),
	}
}

func (s *BaseStrategy) Init(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.state = StateIdle
	return nil
}

func (s *BaseStrategy) Start() error {
	if s.state == StateRunning {
		return fmt.Errorf("strategy already running")
	}

	s.state = StateRunning
	logger.Info("Strategy started", "name", s.Name, "type", s.Type)
	return nil
}

func (s *BaseStrategy) Stop() error {
	if s.state != StateRunning {
		return nil
	}

	s.cancel()
	s.state = StateStopped

	logger.Info("Strategy stopped", "name", s.Name)
	return nil
}

func (s *BaseStrategy) OnCandle(candle *types.Candle) {
	key := candleKey(candle.Symbol, candle.Timeframe)
	
	s.mu.Lock()
	s.candles[key] = append(s.candles[key], candle)
	if len(s.candles[key]) > 1000 {
		s.candles[key] = s.candles[key][1:]
	}
	s.mu.Unlock()
}

func (s *BaseStrategy) OnSignal(signal *types.Signal) {
	s.AddSignal(signal)
}

func (s *BaseStrategy) GetCandles(symbol, timeframe string, limit int) []*types.Candle {
	key := candleKey(symbol, timeframe)
	
	s.mu.RLock()
	defer s.mu.RUnlock()

	candles := s.candles[key]
	if limit <= 0 || limit >= len(candles) {
		return candles
	}

	return candles[len(candles)-limit:]
}

func (s *BaseStrategy) AddSignal(signal *types.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.signals = append(s.signals, signal)
	if len(s.signals) > 100 {
		s.signals = s.signals[1:]
	}
}

func (s *BaseStrategy) GetSignals() []*types.Signal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*types.Signal, len(s.signals))
	copy(result, s.signals)
	return result
}

func (s *BaseStrategy) GetState() StrategyState {
	return s.state
}

func (s *BaseStrategy) GetMetrics() StrategyMetrics {
	return StrategyMetrics{}
}

func (s *BaseStrategy) GetID() string {
	return s.ID
}

func (s *BaseStrategy) GetName() string {
	return s.Name
}

func candleKey(symbol, timeframe string) string {
	return fmt.Sprintf("%s:%s", symbol, timeframe)
}

type GridStrategy struct {
	*BaseStrategy
	gridLevels  int
	gridSpacing decimal.Decimal
	orderSize   decimal.Decimal
	upperBound  decimal.Decimal
	lowerBound  decimal.Decimal
	orders      map[string]*types.Order
	nextBuyAt   decimal.Decimal
	nextSellAt  decimal.Decimal
	mu          sync.Mutex
}

func NewGridStrategy(symbol string, levels int, spacing float64, orderSize float64) *GridStrategy {
	return &GridStrategy{
		BaseStrategy: NewBaseStrategy("Grid_"+symbol, "grid"),
		gridLevels:  levels,
		gridSpacing: decimal.NewFromFloat(spacing),
		orderSize:   decimal.NewFromFloat(orderSize),
		orders:      make(map[string]*types.Order),
	}
}

func (g *GridStrategy) Init(ctx context.Context) error {
	if err := g.BaseStrategy.Init(ctx); err != nil {
		return err
	}

	g.lowerBound = decimal.NewFromFloat(43000)
	g.upperBound = decimal.NewFromFloat(45000)
	g.nextBuyAt = g.lowerBound
	g.nextSellAt = g.upperBound

	return nil
}

func (g *GridStrategy) OnCandle(candle *types.Candle) {
	g.BaseStrategy.OnCandle(candle)

	if len(g.GetCandles(candle.Symbol, candle.Timeframe, 2)) < 2 {
		return
	}

	currentPrice := candle.Close

	g.mu.Lock()
	defer g.mu.Unlock()

	for i := 0; i < g.gridLevels; i++ {
		gridPrice := g.lowerBound.Add(g.gridSpacing.Mul(decimal.NewFromInt(int64(i))))

		if currentPrice.LessThanOrEqual(gridPrice) {
			if currentPrice.GreaterThanOrEqual(g.nextBuyAt) {
				g.placeGridBuy(gridPrice)
				g.nextBuyAt = gridPrice.Add(g.gridSpacing)
			}
		} else {
			if currentPrice.LessThanOrEqual(g.nextSellAt) {
				g.placeGridSell(gridPrice)
				g.nextSellAt = gridPrice.Sub(g.gridSpacing)
			}
		}
	}
}

func (g *GridStrategy) placeGridBuy(price decimal.Decimal) {
	logger.Debug("Grid buy order", "price", price)
}

func (g *GridStrategy) placeGridSell(price decimal.Decimal) {
	logger.Debug("Grid sell order", "price", price)
}

type DCAStrategy struct {
	*BaseStrategy
	amount       decimal.Decimal
	frequency    time.Duration
	nextBuyTime  time.Time
	lastBuyPrice decimal.Decimal
}

func NewDCAStrategy(symbol string, amount float64, frequency time.Duration) *DCAStrategy {
	return &DCAStrategy{
		BaseStrategy: NewBaseStrategy("DCA_"+symbol, "dca"),
		amount:      decimal.NewFromFloat(amount),
		frequency:   frequency,
		nextBuyTime: time.Now(),
	}
}

func (d *DCAStrategy) Init(ctx context.Context) error {
	if err := d.BaseStrategy.Init(ctx); err != nil {
		return err
	}

	d.nextBuyTime = time.Now().Add(d.frequency)
	return nil
}

func (d *DCAStrategy) Start() error {
	go d.runSchedule()
	return d.BaseStrategy.Start()
}

func (d *DCAStrategy) runSchedule() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			if time.Now().After(d.nextBuyTime) {
				d.executeDCA()
				d.nextBuyTime = time.Now().Add(d.frequency)
			}
		}
	}
}

func (d *DCAStrategy) executeDCA() {
	logger.Info("DCA executing", "amount", d.amount)

	d.AddSignal(&types.Signal{
		Action:   types.SignalActionBuy,
		Quantity: d.amount,
		Reason:   "DCA scheduled",
	})
}

type TrendStrategy struct {
	*BaseStrategy
	fastPeriod   int
	slowPeriod   int
	signalPeriod int
	position     *types.Position
}

func NewTrendStrategy(symbol string, fast, slow, signal int) *TrendStrategy {
	return &TrendStrategy{
		BaseStrategy: NewBaseStrategy("Trend_"+symbol, "trend"),
		fastPeriod:   fast,
		slowPeriod:   slow,
		signalPeriod: signal,
	}
}

func (t *TrendStrategy) OnCandle(candle *types.Candle) {
	t.BaseStrategy.OnCandle(candle)

	candles := t.GetCandles(candle.Symbol, candle.Timeframe, t.slowPeriod+1)
	if len(candles) < t.slowPeriod+1 {
		return
	}

	fastMA := t.calculateSMA(candles, t.fastPeriod)
	slowMA := t.calculateSMA(candles, t.slowPeriod)

	var signal *types.Signal

	if fastMA.GreaterThan(slowMA) && t.position == nil {
		signal = &types.Signal{
			Action:  types.SignalActionBuy,
			Reason:  fmt.Sprintf("Fast MA crossed above Slow MA (%s > %s)", fastMA.StringFixed(2), slowMA.StringFixed(2)),
		}
	} else if fastMA.LessThan(slowMA) && t.position != nil {
		signal = &types.Signal{
			Action:  types.SignalActionClose,
			Reason:  fmt.Sprintf("Fast MA crossed below Slow MA (%s < %s)", fastMA.StringFixed(2), slowMA.StringFixed(2)),
		}
	}

	if signal != nil {
		t.AddSignal(signal)
	}
}

func (t *TrendStrategy) calculateSMA(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period {
		return decimal.Zero
	}

	sum := decimal.Zero
	for i := len(candles) - period; i < len(candles); i++ {
		sum = sum.Add(candles[i].Close)
	}

	return sum.Div(decimal.NewFromInt(int64(period)))
}

type ScalperStrategy struct {
	*BaseStrategy
	profitTarget decimal.Decimal
	stopLoss     decimal.Decimal
	position     *types.Position
}

func NewScalperStrategy(symbol string, profitTarget, stopLoss float64) *ScalperStrategy {
	return &ScalperStrategy{
		BaseStrategy: NewBaseStrategy("Scalper_"+symbol, "scalping"),
		profitTarget: decimal.NewFromFloat(profitTarget),
		stopLoss:     decimal.NewFromFloat(stopLoss),
	}
}

func (s *ScalperStrategy) OnCandle(candle *types.Candle) {
	s.BaseStrategy.OnCandle(candle)

	if s.position == nil {
		rsi := s.calculateRSI(candle)

		if rsi.LessThan(decimal.NewFromInt(30)) {
			s.AddSignal(&types.Signal{
				Action: types.SignalActionBuy,
				Reason: fmt.Sprintf("RSI oversold: %s", rsi.StringFixed(2)),
			})
		} else if rsi.GreaterThan(decimal.NewFromInt(70)) {
			s.AddSignal(&types.Signal{
				Action: types.SignalActionSell,
				Reason: fmt.Sprintf("RSI overbought: %s", rsi.StringFixed(2)),
			})
		}
	}
}

func (s *ScalperStrategy) calculateRSI(candle *types.Candle) decimal.Decimal {
	return decimal.NewFromFloat(45)
}

type ArbitrageStrategy struct {
	*BaseStrategy
	opportunities   []ArbitrageOpportunity
	exchanges       []types.Exchange
	minProfitMargin decimal.Decimal
}

type ArbitrageOpportunity struct {
	BuyExchange  types.Exchange
	SellExchange types.Exchange
	Symbol       string
	BuyPrice     decimal.Decimal
	SellPrice    decimal.Decimal
	ProfitMargin decimal.Decimal
	Timestamp    time.Time
}

func NewArbitrageStrategy() *ArbitrageStrategy {
	return &ArbitrageStrategy{
		BaseStrategy:    NewBaseStrategy("Arbitrage", "arbitrage"),
		exchanges:       []types.Exchange{types.ExchangeBinance, types.ExchangeBybit},
		minProfitMargin: decimal.NewFromFloat(0.001),
	}
}

func (a *ArbitrageStrategy) CheckOpportunities() {
	for _, symbol := range []string{"BTC/USDT", "ETH/USDT"} {
		prices := make(map[types.Exchange]decimal.Decimal)

		for _, exchange := range a.exchanges {
			price := decimal.NewFromFloat(44135.68)
			prices[exchange] = price
		}

		var bestBuy, bestSell types.Exchange
		var minPrice, maxPrice decimal.Decimal

		for ex, price := range prices {
			if minPrice.IsZero() || price.LessThan(minPrice) {
				minPrice = price
				bestBuy = ex
			}
			if price.GreaterThan(maxPrice) {
				maxPrice = price
				bestSell = ex
			}
		}

		profitMargin := maxPrice.Sub(minPrice).Div(minPrice)

		if profitMargin.GreaterThan(a.minProfitMargin) {
			a.opportunities = append(a.opportunities, ArbitrageOpportunity{
				BuyExchange:  bestBuy,
				SellExchange: bestSell,
				Symbol:       symbol,
				BuyPrice:     minPrice,
				SellPrice:    maxPrice,
				ProfitMargin: profitMargin,
				Timestamp:    time.Now(),
			})

			a.AddSignal(&types.Signal{
				Symbol: symbol,
				Action: types.SignalActionBuy,
				Reason: fmt.Sprintf("Arbitrage: buy @ %s (%s), sell @ %s (%s), profit: %s%%",
					bestBuy, minPrice.StringFixed(2), bestSell, maxPrice.StringFixed(2), profitMargin.Mul(decimal.NewFromInt(100)).StringFixed(2)),
			})
		}
	}
}

type StrategyManager struct {
	strategies map[string]Strategy
	factory    *StrategyFactory
	mu         sync.RWMutex
}

func NewStrategyManager() *StrategyManager {
	return &StrategyManager{
		strategies: make(map[string]Strategy),
		factory:    NewStrategyFactory(),
	}
}

func (m *StrategyManager) AddStrategy(name string, strategy Strategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.strategies[name]; exists {
		return fmt.Errorf("strategy %s already exists", name)
	}

	m.strategies[name] = strategy
	return nil
}

func (m *StrategyManager) RemoveStrategy(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	strategy, exists := m.strategies[name]
	if !exists {
		return fmt.Errorf("strategy %s not found", name)
	}

	if err := strategy.Stop(); err != nil {
		return err
	}

	delete(m.strategies, name)
	return nil
}

func (m *StrategyManager) GetStrategy(name string) Strategy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.strategies[name]
}

func (m *StrategyManager) GetAllStrategies() map[string]Strategy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]Strategy, len(m.strategies))
	for k, v := range m.strategies {
		result[k] = v
	}
	return result
}

func (m *StrategyManager) StartAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, strategy := range m.strategies {
		if err := strategy.Start(); err != nil {
			logger.Error("Failed to start strategy", "name", name, "error", err)
			continue
		}
	}

	return nil
}

func (m *StrategyManager) StopAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, strategy := range m.strategies {
		if err := strategy.Stop(); err != nil {
			logger.Error("Failed to stop strategy", "name", name, "error", err)
		}
	}

	return nil
}

type StrategyFactory struct{}

func NewStrategyFactory() *StrategyFactory {
	return &StrategyFactory{}
}

func (f *StrategyFactory) Create(strategyType, symbol string, params map[string]interface{}) (Strategy, error) {
	switch strategyType {
	case "grid":
		levels := 10
		spacing := 0.01
		size := 0.01
		if v, ok := params["levels"]; ok {
			levels = int(v.(float64))
		}
		return NewGridStrategy(symbol, levels, spacing, size), nil

	case "dca":
		amount := 100.0
		frequency := 24 * time.Hour
		return NewDCAStrategy(symbol, amount, frequency), nil

	case "trend":
		return NewTrendStrategy(symbol, 10, 20, 9), nil

	case "scalping":
		return NewScalperStrategy(symbol, 0.005, 0.002), nil

	case "market_making":
		return NewMarketMakingStrategy(symbol, params), nil

	case "mean_reversion":
		return NewMeanReversionStrategy(symbol, params), nil

	case "breakout":
		return NewBreakoutStrategy(symbol, params), nil

	case "momentum":
		return NewMomentumStrategy(symbol, params), nil

	case "pairs":
		return NewPairsTradingStrategy(symbol, params), nil

	default:
		return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
	}
}

type MarketMakingStrategy struct {
	*BaseStrategy
	bidSpread      decimal.Decimal
	askSpread     decimal.Decimal
	positionLimit decimal.Decimal
	currentBid    *types.Order
	currentAsk    *types.Order
	orderSize     decimal.Decimal
	rebalance     bool
	position      *types.Position
}

func NewMarketMakingStrategy(symbol string, params map[string]interface{}) *MarketMakingStrategy {
	bidSpread := decimal.NewFromFloat(0.0005)
	askSpread := decimal.NewFromFloat(0.0005)
	positionLimit := decimal.NewFromFloat(1.0)
	orderSize := decimal.NewFromFloat(0.1)

	if v, ok := params["bid_spread"]; ok {
		bidSpread = decimal.NewFromFloat(v.(float64))
	}
	if v, ok := params["ask_spread"]; ok {
		askSpread = decimal.NewFromFloat(v.(float64))
	}
	if v, ok := params["position_limit"]; ok {
		positionLimit = decimal.NewFromFloat(v.(float64))
	}
	if v, ok := params["order_size"]; ok {
		orderSize = decimal.NewFromFloat(v.(float64))
	}

	return &MarketMakingStrategy{
		BaseStrategy: NewBaseStrategy("MarketMaking_"+symbol, "market_making"),
		bidSpread:    bidSpread,
		askSpread:   askSpread,
		positionLimit: positionLimit,
		orderSize:   orderSize,
		rebalance:   true,
	}
}

func (m *MarketMakingStrategy) OnCandle(candle *types.Candle) {
	m.BaseStrategy.OnCandle(candle)

	if m.position != nil {
		positionSize := m.position.Quantity.Abs()
		if positionSize.GreaterThan(m.positionLimit) {
			if m.currentBid != nil {
				logger.Info("Position limit reached, cancelling bid", "size", positionSize)
				m.currentBid = nil
			}
			if m.currentAsk != nil {
				logger.Info("Position limit reached, cancelling ask", "size", positionSize)
				m.currentAsk = nil
			}
			return
		}
	}

	midPrice := candle.Close
	bidPrice := midPrice.Sub(midPrice.Mul(m.bidSpread))
	askPrice := midPrice.Add(midPrice.Mul(m.askSpread))

	if m.currentBid == nil {
		m.placeBid(bidPrice)
	}
	if m.currentAsk == nil {
		m.placeAsk(askPrice)
	}
}

func (m *MarketMakingStrategy) placeBid(price decimal.Decimal) {
	logger.Debug("MM placing bid", "price", price, "size", m.orderSize)
	m.AddSignal(&types.Signal{
		Action:   types.SignalActionBuy,
		Price:    price,
		Quantity: m.orderSize,
		Reason:   "Market making bid",
	})
}

func (m *MarketMakingStrategy) placeAsk(price decimal.Decimal) {
	logger.Debug("MM placing ask", "price", price, "size", m.orderSize)
	m.AddSignal(&types.Signal{
		Action:   types.SignalActionSell,
		Price:    price,
		Quantity: m.orderSize,
		Reason:   "Market making ask",
	})
}

type MeanReversionStrategy struct {
	*BaseStrategy
	lookbackPeriod int
	stdDevMultiplier decimal.Decimal
	bollingerPeriod int
	oversoldThreshold decimal.Decimal
	overboughtThreshold decimal.Decimal
	position *types.Position
}

func NewMeanReversionStrategy(symbol string, params map[string]interface{}) *MeanReversionStrategy {
	lookback := 20
	stdDev := decimal.NewFromFloat(2.0)
	bollinger := 20
	oversold := decimal.NewFromFloat(30)
	overbought := decimal.NewFromFloat(70)

	if v, ok := params["lookback"]; ok {
		lookback = int(v.(float64))
	}
	if v, ok := params["std_dev"]; ok {
		stdDev = decimal.NewFromFloat(v.(float64))
	}

	return &MeanReversionStrategy{
		BaseStrategy:    NewBaseStrategy("MeanReversion_"+symbol, "mean_reversion"),
		lookbackPeriod: lookback,
		stdDevMultiplier: stdDev,
		bollingerPeriod: bollinger,
		oversoldThreshold: oversold,
		overboughtThreshold: overbought,
	}
}

func (m *MeanReversionStrategy) OnCandle(candle *types.Candle) {
	m.BaseStrategy.OnCandle(candle)

	candles := m.GetCandles(candle.Symbol, candle.Timeframe, m.lookbackPeriod+1)
	if len(candles) < m.lookbackPeriod+1 {
		return
	}

	mean := m.calculateMean(candles)
	stdDev := m.calculateStdDev(candles, mean)
	upperBand := mean.Add(stdDev.Mul(m.stdDevMultiplier))
	lowerBand := mean.Sub(stdDev.Mul(m.stdDevMultiplier))

	var signal *types.Signal
	currentPrice := candle.Close

	if m.position == nil && currentPrice.LessThanOrEqual(lowerBand) {
		signal = &types.Signal{
			Action: types.SignalActionBuy,
			Reason: fmt.Sprintf("Price below lower band: %s < %s", currentPrice.StringFixed(2), lowerBand.StringFixed(2)),
		}
	} else if m.position != nil && currentPrice.GreaterThanOrEqual(upperBand) {
		signal = &types.Signal{
			Action: types.SignalActionClose,
			Reason: fmt.Sprintf("Price above upper band: %s >= %s", currentPrice.StringFixed(2), upperBand.StringFixed(2)),
		}
	} else if m.position != nil {
		profit := currentPrice.Sub(m.position.AvgEntryPrice)
		stopLoss := m.position.AvgEntryPrice.Mul(decimal.NewFromFloat(0.98))
		if profit.LessThanOrEqual(stopLoss.Sub(m.position.AvgEntryPrice)) {
			signal = &types.Signal{
				Action: types.SignalActionClose,
				Reason: "Stop loss hit",
			}
		}
	}

	if signal != nil {
		m.AddSignal(signal)
	}
}

func (m *MeanReversionStrategy) calculateMean(candles []*types.Candle) decimal.Decimal {
	sum := decimal.Zero
	for _, c := range candles {
		sum = sum.Add(c.Close)
	}
	return sum.Div(decimal.NewFromInt(int64(len(candles))))
}

func (m *MeanReversionStrategy) calculateStdDev(candles []*types.Candle, mean decimal.Decimal) decimal.Decimal {
	sumSquares := decimal.Zero
	for _, c := range candles {
		diff := c.Close.Sub(mean)
		sumSquares = sumSquares.Add(diff.Mul(diff))
	}
	variance := sumSquares.Div(decimal.NewFromInt(int64(len(candles)-1)))
	return decimal.NewFromFloat(math.Sqrt(variance.InexactFloat64()))
}

type BreakoutStrategy struct {
	*BaseStrategy
	lookbackPeriod int
	volumePeriod int
	volumeMultiplier decimal.Decimal
	position *types.Position
}

func NewBreakoutStrategy(symbol string, params map[string]interface{}) *BreakoutStrategy {
	lookback := 20
	volumePeriod := 20
	volumeMult := decimal.NewFromFloat(1.5)

	if v, ok := params["lookback"]; ok {
		lookback = int(v.(float64))
	}
	if v, ok := params["volume_period"]; ok {
		volumePeriod = int(v.(float64))
	}

	return &BreakoutStrategy{
		BaseStrategy: NewBaseStrategy("Breakout_"+symbol, "breakout"),
		lookbackPeriod: lookback,
		volumePeriod: volumePeriod,
		volumeMultiplier: volumeMult,
	}
}

func (b *BreakoutStrategy) OnCandle(candle *types.Candle) {
	b.BaseStrategy.OnCandle(candle)

	candles := b.GetCandles(candle.Symbol, candle.Timeframe, b.lookbackPeriod+1)
	if len(candles) < b.lookbackPeriod+1 {
		return
	}

	highestHigh := decimal.Zero
	lowestLow := decimal.NewFromFloat(1e18)
	avgVolume := decimal.Zero

	for i := 0; i < b.lookbackPeriod; i++ {
		c := candles[i]
		if c.High.GreaterThan(highestHigh) {
			highestHigh = c.High
		}
		if c.Low.LessThan(lowestLow) {
			lowestLow = c.Low
		}
		avgVolume = avgVolume.Add(c.Volume)
	}
	avgVolume = avgVolume.Div(decimal.NewFromInt(int64(b.lookbackPeriod)))

	var signal *types.Signal
	currentPrice := candle.Close

	if b.position == nil {
		if currentPrice.GreaterThan(highestHigh) && candle.Volume.GreaterThan(avgVolume.Mul(b.volumeMultiplier)) {
			signal = &types.Signal{
				Action: types.SignalActionBuy,
				Reason: fmt.Sprintf("Breakout above %s with volume %s > avg %s", highestHigh.StringFixed(2), candle.Volume.StringFixed(2), avgVolume.StringFixed(2)),
			}
		}
	} else {
		if currentPrice.LessThan(lowestLow) {
			signal = &types.Signal{
				Action: types.SignalActionClose,
				Reason: fmt.Sprintf("Breakdown below %s", lowestLow.StringFixed(2)),
			}
		}
	}

	if signal != nil {
		b.AddSignal(signal)
	}
}

type MomentumStrategy struct {
	*BaseStrategy
	rsiPeriod int
	rsiOverbought decimal.Decimal
	rsiOversold decimal.Decimal
	macdFast int
	macdSlow int
	macdSignal int
	position *types.Position
}

func NewMomentumStrategy(symbol string, params map[string]interface{}) *MomentumStrategy {
	rsiPeriod := 14
	macdFast := 12
	macdSlow := 26
	macdSignal := 9

	if v, ok := params["rsi_period"]; ok {
		rsiPeriod = int(v.(float64))
	}

	return &MomentumStrategy{
		BaseStrategy: NewBaseStrategy("Momentum_"+symbol, "momentum"),
		rsiPeriod: rsiPeriod,
		rsiOverbought: decimal.NewFromFloat(70),
		rsiOversold: decimal.NewFromFloat(30),
		macdFast: macdFast,
		macdSlow: macdSlow,
		macdSignal: macdSignal,
	}
}

func (m *MomentumStrategy) OnCandle(candle *types.Candle) {
	m.BaseStrategy.OnCandle(candle)

	candles := m.GetCandles(candle.Symbol, candle.Timeframe, m.rsiPeriod+5)
	if len(candles) < m.rsiPeriod+5 {
		return
	}

	rsi := m.calculateRSI(candles, m.rsiPeriod)
	macdLine, signalLine := m.calculateMACD(candles)

	var signal *types.Signal

	if m.position == nil {
		if rsi.LessThan(m.rsiOversold) && macdLine.GreaterThan(signalLine) {
			signal = &types.Signal{
				Action: types.SignalActionBuy,
				Reason: fmt.Sprintf("RSI oversold (%s) and MACD bullish crossover", rsi.StringFixed(2)),
			}
		}
	} else {
		if rsi.GreaterThan(m.rsiOverbought) && macdLine.LessThan(signalLine) {
			signal = &types.Signal{
				Action: types.SignalActionClose,
				Reason: fmt.Sprintf("RSI overbought (%s) and MACD bearish crossover", rsi.StringFixed(2)),
			}
		}
	}

	if signal != nil {
		m.AddSignal(signal)
	}
}

func (m *MomentumStrategy) calculateRSI(candles []*types.Candle, period int) decimal.Decimal {
	gains := decimal.Zero
	losses := decimal.Zero

	for i := len(candles) - period; i < len(candles)-1; i++ {
		change := candles[i+1].Close.Sub(candles[i].Close)
		if change.GreaterThan(decimal.Zero) {
			gains = gains.Add(change)
		} else {
			losses = losses.Add(change.Abs())
		}
	}

	avgGain := gains.Div(decimal.NewFromInt(int64(period)))
	avgLoss := losses.Div(decimal.NewFromInt(int64(period)))

	if avgLoss.IsZero() {
		return decimal.NewFromInt(100)
	}

	rs := avgGain.Div(avgLoss)
	return decimal.NewFromFloat(100).Sub(decimal.NewFromFloat(100).Div(decimal.NewFromFloat(1).Add(rs)))
}

func (m *MomentumStrategy) calculateMACD(candles []*types.Candle) (decimal.Decimal, decimal.Decimal) {
	fastEMA := m.calculateEMA(candles, m.macdFast)
	slowEMA := m.calculateEMA(candles, m.macdSlow)
	macdLine := fastEMA.Sub(slowEMA)
	signalLine := macdLine

	return macdLine, signalLine
}

func (m *MomentumStrategy) calculateEMA(candles []*types.Candle, period int) decimal.Decimal {
	multiplier := decimal.NewFromFloat(2.0).Div(decimal.NewFromInt(int64(period + 1)))
	sum := decimal.Zero

	for i := 0; i < period && i < len(candles); i++ {
		sum = sum.Add(candles[i].Close)
	}

	ema := sum.Div(decimal.NewFromInt(int64(period)))

	for i := period; i < len(candles); i++ {
		ema = candles[i].Close.Sub(ema).Mul(multiplier).Add(ema)
	}

	return ema
}

type PairsTradingStrategy struct {
	*BaseStrategy
	symbol1 string
	symbol2 string
	spreadWindow int
	entryThreshold decimal.Decimal
	exitThreshold decimal.Decimal
	zscoreHistory []decimal.Decimal
	position *types.Position
}

func NewPairsTradingStrategy(symbol string, params map[string]interface{}) *PairsTradingStrategy {
	spreadWindow := 20
	entryThreshold := decimal.NewFromFloat(2.0)
	exitThreshold := decimal.NewFromFloat(0.5)
	symbol1 := symbol
	symbol2 := ""

	if v, ok := params["symbol2"]; ok {
		symbol2 = v.(string)
	}

	return &PairsTradingStrategy{
		BaseStrategy: NewBaseStrategy("Pairs_"+symbol+"_"+symbol2, "pairs"),
		symbol1: symbol1,
		symbol2: symbol2,
		spreadWindow: spreadWindow,
		entryThreshold: entryThreshold,
		exitThreshold: exitThreshold,
		zscoreHistory: make([]decimal.Decimal, 0),
	}
}

func (p *PairsTradingStrategy) OnCandle(candle *types.Candle) {
	p.BaseStrategy.OnCandle(candle)

	candles1 := p.GetCandles(p.symbol1, candle.Timeframe, p.spreadWindow+1)
	candles2 := p.GetCandles(p.symbol2, candle.Timeframe, p.spreadWindow+1)

	if len(candles1) < p.spreadWindow+1 || len(candles2) < p.spreadWindow+1 {
		return
	}

	spread := candles1[len(candles1)-1].Close.Div(candles2[len(candles2)-1].Close)
	mean := p.calculateMean(candles1, candles2)
	stdDev := p.calculateStdDev(candles1, candles2, mean)

	zscore := decimal.Zero
	if !stdDev.IsZero() {
		zscore = spread.Sub(mean).Div(stdDev)
	}

	p.zscoreHistory = append(p.zscoreHistory, zscore)
	if len(p.zscoreHistory) > 100 {
		p.zscoreHistory = p.zscoreHistory[1:]
	}

	var signal *types.Signal

	if p.position == nil {
		if zscore.GreaterThan(p.entryThreshold) || zscore.LessThan(p.entryThreshold.Neg()) {
			action := types.SignalActionBuy
			reason := "Z-score entry"
			if zscore.GreaterThan(decimal.Zero) {
				action = types.SignalActionSell
				reason = "Z-score entry (short)"
			}
			signal = &types.Signal{
				Symbol: fmt.Sprintf("%s/%s", p.symbol1, p.symbol2),
				Action: action,
				Reason: fmt.Sprintf("%s: z-score %s beyond threshold", reason, zscore.StringFixed(2)),
			}
		}
	} else {
		if zscore.Abs().LessThan(p.exitThreshold) {
			signal = &types.Signal{
				Symbol: fmt.Sprintf("%s/%s", p.symbol1, p.symbol2),
				Action: types.SignalActionClose,
				Reason: fmt.Sprintf("Z-score %s converged to mean", zscore.StringFixed(2)),
			}
		}
	}

	if signal != nil {
		p.AddSignal(signal)
	}
}

func (p *PairsTradingStrategy) calculateMean(c1, c2 []*types.Candle) decimal.Decimal {
	sum := decimal.Zero
	for i := 0; i < len(c1) && i < len(c2); i++ {
		if !c2[i].Close.IsZero() {
			sum = sum.Add(c1[i].Close.Div(c2[i].Close))
		}
	}
	return sum.Div(decimal.NewFromInt(int64(len(c1))))
}

func (p *PairsTradingStrategy) calculateStdDev(c1, c2 []*types.Candle, mean decimal.Decimal) decimal.Decimal {
	sumSquares := decimal.Zero
	for i := 0; i < len(c1) && i < len(c2); i++ {
		if !c2[i].Close.IsZero() {
			spread := c1[i].Close.Div(c2[i].Close)
			diff := spread.Sub(mean)
			sumSquares = sumSquares.Add(diff.Mul(diff))
		}
	}
	variance := sumSquares.Div(decimal.NewFromInt(int64(len(c1))))
	return decimal.NewFromFloat(math.Sqrt(variance.InexactFloat64()))
}
