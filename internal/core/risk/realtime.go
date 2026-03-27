package risk

import (
	"context"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type RealTimeRiskManager struct {
	mu              sync.RWMutex
	config          RiskConfig
	portfolio       *PortfolioRisk
	positions       map[string]*PositionRisk
	circuitBreaker  *CircuitBreaker
	varCalculator   *VaRCalculator
	correlationMatrix *CorrelationMatrix
	exposureLimits *ExposureLimits
	eventHandlers  []RiskEventHandler
	metrics        *RiskMetrics
	startTime      time.Time
	isRunning      bool
}

type RiskConfig struct {
	MaxPortfolioRisk     decimal.Decimal
	MaxPositionRisk    decimal.Decimal
	MaxLeverage        decimal.Decimal
	MaxDrawdown        decimal.Decimal
	MaxDailyLoss       decimal.Decimal
	MaxConcentration   decimal.Decimal
	CircuitBreakerThreshold decimal.Decimal
	CircuitBreakerTimeout   time.Duration
	VaRConfidence      float64
	VaRHorizon         time.Duration
	CorrelationThreshold decimal.Decimal
	UpdateInterval     time.Duration
}

type PortfolioRisk struct {
	TotalValue       decimal.Decimal
	CashBalance      decimal.Decimal
	InvestedValue    decimal.Decimal
	UnrealizedPnL   decimal.Decimal
	RealizedPnL     decimal.Decimal
	TotalRisk        decimal.Decimal
	BuyingPower      decimal.Decimal
	DayPnL          decimal.Decimal
	DayReturn       decimal.Decimal
}

type PositionRisk struct {
	Symbol           string
	Quantity         decimal.Decimal
	EntryPrice       decimal.Decimal
	CurrentPrice     decimal.Decimal
	MarketValue     decimal.Decimal
	UnrealizedPnL   decimal.Decimal
	RiskAmount      decimal.Decimal
	RiskPercentage  decimal.Decimal
	VaR             decimal.Decimal
	StopLoss        decimal.Decimal
	TakeProfit      decimal.Decimal
}

type CircuitBreaker struct {
	mu              sync.RWMutex
	state           CircuitState
	triggeredAt     time.Time
	consecutiveLoss int
	lossThreshold  decimal.Decimal
	timeout        time.Duration
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

type VaRCalculator struct {
	mu           sync.RWMutex
	confidence   float64
	horizon      time.Duration
	history      []VaRDataPoint
	maxHistory   int
}

type VaRDataPoint struct {
	Timestamp time.Time
	Value     decimal.Decimal
	Returns   decimal.Decimal
}

type CorrelationMatrix struct {
	mu         sync.RWMutex
	symbols    []string
	correlations map[string]map[string]decimal.Decimal
}

type ExposureLimits struct {
	mu              sync.RWMutex
	sectorExposure  map[string]decimal.Decimal
	assetExposure   map[string]decimal.Decimal
	maxSector       decimal.Decimal
	maxAsset        decimal.Decimal
}

type RiskEventHandler func(event *RiskEvent)

type RiskEvent struct {
	Type        RiskEventType
	Severity    RiskSeverity
	Symbol      string
	Message     string
	Details     map[string]interface{}
	Timestamp   time.Time
}

type RiskEventType string

const (
	EventPositionLimit    RiskEventType = "position_limit"
	EventDrawdownWarning  RiskEventType = "drawdown_warning"
	EventCircuitBreaker   RiskEventType = "circuit_breaker"
	EventVaRBreach        RiskEventType = "var_breach"
	EventCorrelationRisk  RiskEventType = "correlation_risk"
	EventExposureLimit    RiskEventType = "exposure_limit"
	EventDailyLossLimit   RiskEventType = "daily_loss_limit"
)

type RiskSeverity string

const (
	SeverityInfo     RiskSeverity = "info"
	SeverityWarning  RiskSeverity = "warning"
	SeverityCritical RiskSeverity = "critical"
)

type RiskMetrics struct {
	mu                sync.RWMutex
	CurrentVaR        decimal.Decimal
	MaxVaR           decimal.Decimal
	AverageVaR        decimal.Decimal
	MaxDrawdown       decimal.Decimal
	CurrentDrawdown   decimal.Decimal
	CumulativePnL    decimal.Decimal
	SharpeRatio      decimal.Decimal
	SortinoRatio     decimal.Decimal
	LastUpdate       time.Time
}

func NewRealTimeRiskManager(cfg RiskConfig) *RealTimeRiskManager {
	return &RealTimeRiskManager{
		config:           cfg,
		portfolio:        &PortfolioRisk{},
		positions:        make(map[string]*PositionRisk),
		circuitBreaker:   NewCircuitBreaker(cfg),
		varCalculator:    NewVaRCalculator(cfg.VaRConfidence, cfg.VaRHorizon),
		correlationMatrix: NewCorrelationMatrix(),
		exposureLimits:   NewExposureLimits(cfg),
		metrics:          &RiskMetrics{},
		startTime:        time.Now(),
	}
}

func NewCircuitBreaker(cfg RiskConfig) *CircuitBreaker {
	return &CircuitBreaker{
		lossThreshold: cfg.CircuitBreakerThreshold,
		timeout:      cfg.CircuitBreakerTimeout,
		state:        CircuitClosed,
	}
}

func NewVaRCalculator(confidence float64, horizon time.Duration) *VaRCalculator {
	return &VaRCalculator{
		confidence: confidence,
		horizon:    horizon,
		history:    make([]VaRDataPoint, 0),
		maxHistory: 252,
	}
}

func NewCorrelationMatrix() *CorrelationMatrix {
	return &CorrelationMatrix{
		correlations: make(map[string]map[string]decimal.Decimal),
	}
}

func NewExposureLimits(cfg RiskConfig) *ExposureLimits {
	return &ExposureLimits{
		sectorExposure: make(map[string]decimal.Decimal),
		assetExposure:  make(map[string]decimal.Decimal),
		maxSector:      decimal.NewFromFloat(0.3),
		maxAsset:       decimal.NewFromFloat(0.25),
	}
}

func (m *RealTimeRiskManager) Start(ctx context.Context) {
	m.mu.Lock()
	m.isRunning = true
	m.mu.Unlock()

	go m.riskMonitorLoop(ctx)
}

func (m *RealTimeRiskManager) Stop() {
	m.mu.Lock()
	m.isRunning = false
	m.mu.Unlock()
}

func (m *RealTimeRiskManager) riskMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.updateRiskMetrics()
			m.checkRiskLimits()
		}
	}
}

func (m *RealTimeRiskManager) UpdatePosition(symbol string, pos *types.Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	riskPos := &PositionRisk{
		Symbol:       symbol,
		Quantity:     pos.Quantity,
		EntryPrice:  pos.AvgEntryPrice,
		CurrentPrice: pos.CurrentPrice,
	}

	if !pos.Quantity.IsZero() {
		riskPos.MarketValue = pos.Quantity.Mul(pos.CurrentPrice)
		riskPos.UnrealizedPnL = pos.Quantity.Mul(pos.CurrentPrice.Sub(pos.AvgEntryPrice))
		riskPos.RiskAmount = riskPos.MarketValue.Mul(decimal.NewFromFloat(0.02))
	}

	m.positions[symbol] = riskPos
}

func (m *RealTimeRiskManager) CalculateVaR(portfolioValue decimal.Decimal) decimal.Decimal {
	m.varCalculator.mu.RLock()
	defer m.varCalculator.mu.RUnlock()

	if len(m.varCalculator.history) < 2 {
		return portfolioValue.Mul(decimal.NewFromFloat(0.02))
	}

	return m.calculateHistoricalVaR(portfolioValue)
}

func (m *RealTimeRiskManager) calculateHistoricalVaR(portfolioValue decimal.Decimal) decimal.Decimal {
	if len(m.varCalculator.history) < 2 {
		return decimal.Zero
	}

	returns := make([]float64, len(m.varCalculator.history))
	for i, dp := range m.varCalculator.history {
		returns[i] = dp.Returns.InexactFloat64()
	}

	percentile := 100.0 - (m.varCalculator.confidence * 100.0)
	sortedReturns := sortFloats(returns)

	idx := int(float64(len(sortedReturns)) * percentile / 100.0)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sortedReturns) {
		idx = len(sortedReturns) - 1
	}

	varReturn := sortedReturns[idx]
	varAmount := portfolioValue.Mul(decimal.NewFromFloat(varReturn))

	if varAmount.LessThan(decimal.Zero) {
		return varAmount.Abs()
	}
	return decimal.Zero
}

func sortFloats(floats []float64) []float64 {
	for i := 0; i < len(floats)-1; i++ {
		for j := 0; j < len(floats)-i-1; j++ {
			if floats[j] > floats[j+1] {
				floats[j], floats[j+1] = floats[j+1], floats[j]
			}
		}
	}
	return floats
}

func (m *RealTimeRiskManager) CheckPositionLimit(symbol string, size, portfolioValue decimal.Decimal) bool {
	if portfolioValue.IsZero() {
		return false
	}

	positionRatio := size.Div(portfolioValue)
	return positionRatio.LessThanOrEqual(m.config.MaxPositionRisk)
}

func (m *RealTimeRiskManager) CheckDrawdown() bool {
	if m.portfolio.TotalValue.IsZero() {
		return true
	}

	peak := m.portfolio.TotalValue.Add(m.portfolio.UnrealizedPnL)
	drawdown := peak.Sub(m.portfolio.TotalValue).Div(peak)

	return drawdown.LessThan(m.config.MaxDrawdown)
}

func (m *RealTimeRiskManager) UpdateCorrelation(symbols []string, prices map[string]decimal.Decimal) {
	m.correlationMatrix.mu.Lock()
	defer m.correlationMatrix.mu.Unlock()

	m.correlationMatrix.symbols = symbols

	for _, s1 := range symbols {
		if _, ok := m.correlationMatrix.correlations[s1]; !ok {
			m.correlationMatrix.correlations[s1] = make(map[string]decimal.Decimal)
		}

		for _, s2 := range symbols {
			if s1 == s2 {
				m.correlationMatrix.correlations[s1][s2] = decimal.NewFromInt(1)
				continue
			}

			corr := m.calculateCorrelation(s1, s2, prices)
			m.correlationMatrix.correlations[s1][s2] = corr
		}
	}
}

func (m *RealTimeRiskManager) calculateCorrelation(s1, s2 string, prices map[string]decimal.Decimal) decimal.Decimal {
	if len(prices) < 2 {
		return decimal.Zero
	}

	return decimal.NewFromFloat(0.3)
}

func (m *RealTimeRiskManager) CheckCorrelationRisk() []string {
	m.correlationMatrix.mu.RLock()
	defer m.correlationMatrix.mu.RUnlock()

	highCorrPairs := make([]string, 0)
	for s1, correlations := range m.correlationMatrix.correlations {
		for s2, corr := range correlations {
			if corr.GreaterThan(m.config.CorrelationThreshold) && s1 != s2 {
				highCorrPairs = append(highCorrPairs, s1+"-"+s2)
			}
		}
	}

	return highCorrPairs
}

func (m *RealTimeRiskManager) UpdateDailyPnL(pnl decimal.Decimal) {
	m.portfolio.DayPnL = m.portfolio.DayPnL.Add(pnl)
}

func (m *RealTimeRiskManager) IsDailyLossLimitBreached() bool {
	if m.portfolio.TotalValue.IsZero() {
		return false
	}

	lossRatio := m.portfolio.DayPnL.Neg().Div(m.portfolio.TotalValue)
	return lossRatio.GreaterThan(m.config.MaxDailyLoss)
}

func (m *RealTimeRiskManager) CheckCircuitBreaker() bool {
	m.circuitBreaker.mu.RLock()
	defer m.circuitBreaker.mu.RUnlock()

	if m.circuitBreaker.state == CircuitOpen {
		if time.Since(m.circuitBreaker.triggeredAt) > m.circuitBreaker.timeout {
			return true
		}
		return false
	}

	return m.circuitBreaker.state == CircuitClosed
}

func (m *RealTimeRiskManager) TriggerCircuitBreaker() {
	m.circuitBreaker.mu.Lock()
	defer m.circuitBreaker.mu.Unlock()

	m.circuitBreaker.state = CircuitOpen
	m.circuitBreaker.triggeredAt = time.Now()
	m.circuitBreaker.consecutiveLoss++

	m.emitEvent(&RiskEvent{
		Type:      EventCircuitBreaker,
		Severity:  SeverityCritical,
		Message:   "Circuit breaker triggered",
		Timestamp: time.Now(),
	})

	logger.Error("Circuit breaker triggered",
		"consecutive_loss", m.circuitBreaker.consecutiveLoss,
		"timeout", m.circuitBreaker.timeout,
	)
}

func (m *RealTimeRiskManager) ResetCircuitBreaker() {
	m.circuitBreaker.mu.Lock()
	defer m.circuitBreaker.mu.Unlock()

	m.circuitBreaker.state = CircuitClosed
	m.circuitBreaker.consecutiveLoss = 0
}

func (m *RealTimeRiskManager) updateRiskMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	maxVaR := decimal.Zero

	for _, pos := range m.positions {
		if pos.VaR.GreaterThan(maxVaR) {
			maxVaR = pos.VaR
		}
	}

	m.metrics.CurrentVaR = maxVaR
	m.metrics.LastUpdate = time.Now()
}

func (m *RealTimeRiskManager) checkRiskLimits() {
	if !m.CheckDrawdown() {
		m.emitEvent(&RiskEvent{
			Type:      EventDrawdownWarning,
			Severity:  SeverityCritical,
			Message:   "Maximum drawdown threshold reached",
			Timestamp: time.Now(),
		})
	}

	if m.IsDailyLossLimitBreached() {
		m.emitEvent(&RiskEvent{
			Type:      EventDailyLossLimit,
			Severity:  SeverityCritical,
			Message:   "Daily loss limit exceeded",
			Details: map[string]interface{}{
				"daily_pnl": m.portfolio.DayPnL.String(),
			},
			Timestamp: time.Now(),
		})
	}

	if highCorr := m.CheckCorrelationRisk(); len(highCorr) > 0 {
		m.emitEvent(&RiskEvent{
			Type:      EventCorrelationRisk,
			Severity:  SeverityWarning,
			Symbol:    highCorr[0],
			Message:   "High correlation detected between positions",
			Details: map[string]interface{}{
				"pairs": highCorr,
			},
			Timestamp: time.Now(),
		})
	}
}

func (m *RealTimeRiskManager) RegisterHandler(handler RiskEventHandler) {
	m.eventHandlers = append(m.eventHandlers, handler)
}

func (m *RealTimeRiskManager) emitEvent(event *RiskEvent) {
	for _, handler := range m.eventHandlers {
		go handler(event)
	}
}

func (m *RealTimeRiskManager) GetMetrics() *RiskMetrics {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()
	return m.metrics
}

func (m *RealTimeRiskManager) GetPortfolioRisk() *PortfolioRisk {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.portfolio
}

func (m *RealTimeRiskManager) CalculateKellyCriterion(winRate, avgWin, avgLoss decimal.Decimal) decimal.Decimal {
	if avgLoss.IsZero() || avgLoss.LessThan(decimal.Zero) {
		return decimal.Zero
	}

	q := decimal.NewFromInt(1).Sub(winRate)
	b := avgWin.Div(avgLoss)

	kelly := winRate.Sub(q.Div(b))

	if kelly.LessThan(decimal.Zero) {
		return decimal.Zero
	}
	if kelly.GreaterThan(decimal.NewFromFloat(0.25)) {
		return decimal.NewFromFloat(0.25)
	}

	return kelly
}
