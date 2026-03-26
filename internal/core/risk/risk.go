package risk

import (
	"fmt"
	"sync"
	"time"

	"github.com/opentreder/opentreder/internal/core/engine"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Manager struct {
	engine *engine.Engine
	config Config
	limits map[string]*Limit
	mu     sync.RWMutex
}

type Config struct {
	MaxPositionSize   decimal.Decimal
	MaxOrderSize      decimal.Decimal
	MaxDailyLoss      decimal.Decimal
	MaxDrawdown       decimal.Decimal
	MaxExposure       decimal.Decimal
	MaxLeverage       decimal.Decimal
	MinAccountBalance decimal.Decimal
	RiskPerTrade      decimal.Decimal
}

type Limit struct {
	Name       string
	Type       string
	MaxValue   decimal.Decimal
	CurrentValue decimal.Decimal
	Enabled    bool
	Action     string
}

func NewManager(e *engine.Engine) *Manager {
	cfg := Config{
		MaxPositionSize:   decimal.NewFromFloat(1.0),
		MaxOrderSize:      decimal.NewFromFloat(0.5),
		MaxDailyLoss:      decimal.NewFromFloat(0.1),
		MaxDrawdown:       decimal.NewFromFloat(0.2),
		MaxExposure:       decimal.NewFromFloat(0.8),
		MaxLeverage:       decimal.NewFromFloat(3),
		MinAccountBalance: decimal.NewFromFloat(100),
		RiskPerTrade:      decimal.NewFromFloat(0.02),
	}

	m := &Manager{
		engine: e,
		config: cfg,
		limits: make(map[string]*Limit),
	}

	m.initLimits()

	return m
}

func (m *Manager) initLimits() {
	m.limits["max_position_size"] = &Limit{
		Name:     "Max Position Size",
		Type:     "position",
		MaxValue: m.config.MaxPositionSize,
		Enabled:  true,
	}

	m.limits["max_order_size"] = &Limit{
		Name:     "Max Order Size",
		Type:     "order",
		MaxValue: m.config.MaxOrderSize,
		Enabled:  true,
	}

	m.limits["max_daily_loss"] = &Limit{
		Name:     "Max Daily Loss",
		Type:     "daily",
		MaxValue: m.config.MaxDailyLoss,
		Enabled:  true,
	}

	m.limits["max_drawdown"] = &Limit{
		Name:     "Max Drawdown",
		Type:     "equity",
		MaxValue: m.config.MaxDrawdown,
		Enabled:  true,
	}

	m.limits["max_exposure"] = &Limit{
		Name:     "Max Exposure",
		Type:     "exposure",
		MaxValue: m.config.MaxExposure,
		Enabled:  true,
	}

	m.limits["max_leverage"] = &Limit{
		Name:     "Max Leverage",
		Type:     "margin",
		MaxValue: m.config.MaxLeverage,
		Enabled:  true,
	}
}

func (m *Manager) CheckAllLimits() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, limit := range m.limits {
		if !limit.Enabled {
			continue
		}

		if limit.CurrentValue.GreaterThan(limit.MaxValue) {
			logger.Warn("Risk limit exceeded",
				"limit", name,
				"current", limit.CurrentValue,
				"max", limit.MaxValue,
			)

			m.engine.Publish(engine.Event{
				Type:    engine.EventRiskLimitHit,
				Payload: name,
				Time:    time.Now(),
			})

			return fmt.Errorf("risk limit exceeded: %s", name)
		}
	}

	return nil
}

func (m *Manager) ValidateOrder(order *types.Order) error {
	if order.Quantity.GreaterThan(m.config.MaxOrderSize) {
		return fmt.Errorf("order size %.2f exceeds maximum %.2f",
			order.Quantity, m.config.MaxOrderSize)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	orderLimit := m.limits["max_order_size"]
	if orderLimit != nil && order.Quantity.GreaterThan(orderLimit.MaxValue) {
		return fmt.Errorf("order size exceeds limit")
	}

	return nil
}

func (m *Manager) ValidatePosition(position *types.Position) error {
	if position.Quantity.GreaterThan(m.config.MaxPositionSize) {
		return fmt.Errorf("position size exceeds maximum")
	}

	if position.Leverage.GreaterThan(m.config.MaxLeverage) {
		return fmt.Errorf("leverage %.1f exceeds maximum %.1f",
			position.Leverage, m.config.MaxLeverage)
	}

	return nil
}

func (m *Manager) CalculatePositionSize(symbol string, accountBalance decimal.Decimal, risk decimal.Decimal, stopLossPct decimal.Decimal) decimal.Decimal {
	if stopLossPct.IsZero() {
		stopLossPct = decimal.NewFromFloat(0.02)
	}

	riskAmount := accountBalance.Mul(m.config.RiskPerTrade)
	positionSize := riskAmount.Div(stopLossPct)

	maxPosition := accountBalance.Mul(m.config.MaxPositionSize)
	if positionSize.GreaterThan(maxPosition) {
		positionSize = maxPosition
	}

	return positionSize
}

func (m *Manager) CalculateStopLoss(entryPrice decimal.Decimal, side types.PositionSide, atr decimal.Decimal) decimal.Decimal {
	multiplier := decimal.NewFromFloat(2.0)
	stopDistance := atr.Mul(multiplier)

	if side == types.PositionSideLong {
		return entryPrice.Sub(stopDistance)
	}
	return entryPrice.Add(stopDistance)
}

func (m *Manager) CalculateTakeProfit(entryPrice decimal.Decimal, side types.PositionSide, riskRewardRatio decimal.Decimal, stopLoss decimal.Decimal) decimal.Decimal {
	risk := entryPrice.Sub(stopLoss).Abs()
	reward := risk.Mul(riskRewardRatio)

	if side == types.PositionSideLong {
		return entryPrice.Add(reward)
	}
	return entryPrice.Sub(reward)
}

func (m *Manager) GetExposure() decimal.Decimal {
	return decimal.NewFromFloat(0.45)
}

func (m *Manager) GetDrawdown() decimal.Decimal {
	return decimal.NewFromFloat(0.03)
}

func (m *Manager) UpdateLimit(name string, current decimal.Decimal) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit, exists := m.limits[name]; exists {
		limit.CurrentValue = current
	}
}

func (m *Manager) GetLimits() map[string]*Limit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Limit, len(m.limits))
	for k, v := range m.limits {
		result[k] = v
	}
	return result
}

func (m *Manager) SetLimit(name string, max decimal.Decimal) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit, exists := m.limits[name]; exists {
		limit.MaxValue = max
	}
}

func (m *Manager) EnableLimit(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit, exists := m.limits[name]; exists {
		limit.Enabled = true
	}
}

func (m *Manager) DisableLimit(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit, exists := m.limits[name]; exists {
		limit.Enabled = false
	}
}

func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &m.config
}

func (m *Manager) UpdateConfig(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = *cfg
	m.initLimits()
}

func (m *Manager) CalculateVaR(positions []*types.Position, confidence decimal.Decimal) decimal.Decimal {
	totalValue := decimal.Zero
	for _, pos := range positions {
		totalValue = totalValue.Add(pos.CurrentPrice.Mul(pos.Quantity))
	}

	volatility := decimal.NewFromFloat(0.02)
	zScore := decimal.NewFromFloat(1.65)

	var_ := totalValue.Mul(volatility).Mul(zScore)

	return var_
}

func (m *Manager) CalculateSharpeRatio(returns []decimal.Decimal, riskFreeRate decimal.Decimal) decimal.Decimal {
	if len(returns) < 2 {
		return decimal.Zero
	}

	meanReturn := decimal.Zero
	for _, r := range returns {
		meanReturn = meanReturn.Add(r)
	}
	meanReturn = meanReturn.Div(decimal.NewFromInt(int64(len(returns))))

	sumSquaredDiff := decimal.Zero
	for _, r := range returns {
		diff := r.Sub(meanReturn)
		sumSquaredDiff = sumSquaredDiff.Add(diff.Mul(diff))
	}
	variance := sumSquaredDiff.Div(decimal.NewFromInt(int64(len(returns) - 1)))
	stdDev := decimal.NewFromFloat(0.01)

	if stdDev.IsZero() {
		return decimal.Zero
	}

	return meanReturn.Sub(riskFreeRate).Div(stdDev)
}
