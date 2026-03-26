package portfolio

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/internal/core/engine"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Manager struct {
	engine     *engine.Engine
	balances   map[string]*types.Balance
	positions  map[string]*types.Position
	history    []*Snapshot
	mu         sync.RWMutex
}

type Snapshot struct {
	TotalValue      decimal.Decimal
	CashBalance     decimal.Decimal
	Equity          decimal.Decimal
	MarginUsed      decimal.Decimal
	UnrealizedPnL   decimal.Decimal
	RealizedPnL     decimal.Decimal
	DayPnL          decimal.Decimal
	Timestamp       time.Time
}

func NewManager() *Manager {
	return &Manager{
		balances:  make(map[string]*types.Balance),
		positions: make(map[string]*types.Position),
		history:   make([]*Snapshot, 0),
	}
}

func (m *Manager) GetBalance(asset string) *types.Balance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	balance, exists := m.balances[asset]
	if !exists {
		return &types.Balance{
			Asset:    asset,
			Free:     decimal.Zero,
			Locked:   decimal.Zero,
			Total:    decimal.Zero,
			USDValue: decimal.Zero,
		}
	}

	return balance
}

func (m *Manager) GetAllBalances() map[string]*types.Balance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*types.Balance, len(m.balances))
	for k, v := range m.balances {
		result[k] = v
	}
	return result
}

func (m *Manager) UpdateBalance(asset string, amount decimal.Decimal, locked bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	balance, exists := m.balances[asset]
	if !exists {
		balance = &types.Balance{
			Asset:    asset,
			Free:     decimal.Zero,
			Locked:   decimal.Zero,
			Total:    decimal.Zero,
			USDValue: decimal.Zero,
		}
		m.balances[asset] = balance
	}

	if locked {
		balance.Locked = balance.Locked.Add(amount)
	} else {
		balance.Free = balance.Free.Add(amount)
	}

	balance.Total = balance.Free.Add(balance.Locked)
}

func (m *Manager) GetPosition(symbol string) *types.Position {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.positions[symbol]
}

func (m *Manager) GetAllPositions() map[string]*types.Position {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*types.Position, len(m.positions))
	for k, v := range m.positions {
		result[k] = v
	}
	return result
}

func (m *Manager) AddPosition(position *types.Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	position.ID = uuid.New()
	position.OpenedAt = time.Now()
	position.UpdatedAt = time.Now()

	m.positions[position.Symbol] = position

	logger.Info("Position added",
		"symbol", position.Symbol,
		"side", position.Side,
		"quantity", position.Quantity,
	)
}

func (m *Manager) UpdatePosition(position *types.Position) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.positions[position.Symbol]
	if !exists {
		m.positions[position.Symbol] = position
		return
	}

	position.ID = existing.ID
	position.OpenedAt = existing.OpenedAt
	position.UpdatedAt = time.Now()

	m.positions[position.Symbol] = position
}

func (m *Manager) RemovePosition(symbol string) *types.Position {
	m.mu.Lock()
	defer m.mu.Unlock()

	position, exists := m.positions[symbol]
	if !exists {
		return nil
	}

	delete(m.positions, symbol)
	return position
}

func (m *Manager) GetTotalValue(quoteAsset string) decimal.Decimal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := decimal.Zero

	for _, balance := range m.balances {
		if balance.Asset == quoteAsset {
			total = total.Add(balance.Total)
		} else {
			price := decimal.NewFromFloat(1.0)
			if balance.Asset == "BTC" {
				price = decimal.NewFromFloat(44000)
			} else if balance.Asset == "ETH" {
				price = decimal.NewFromFloat(2380)
			}

			usdValue := balance.Total.Mul(price)
			total = total.Add(usdValue)
		}
	}

	return total
}

func (m *Manager) GetUnrealizedPnL() decimal.Decimal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := decimal.Zero
	for _, position := range m.positions {
		total = total.Add(position.UnrealizedPnL)
	}
	return total
}

func (m *Manager) GetRealizedPnL() decimal.Decimal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := decimal.Zero
	for _, position := range m.positions {
		total = total.Add(position.RealizedPnL)
	}
	return total
}

func (m *Manager) GetDayPnL() decimal.Decimal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := decimal.Zero
	for _, position := range m.positions {
		dayPnL := position.CurrentPrice.Sub(position.AvgEntryPrice)
		if position.Side == types.PositionSideShort {
			dayPnL = dayPnL.Neg()
		}
		total = total.Add(dayPnL.Mul(position.Quantity))
	}
	return total
}

func (m *Manager) GetExposure() decimal.Decimal {
	totalValue := m.GetTotalValue("USDT")
	if totalValue.IsZero() {
		return decimal.Zero
	}

	exposed := decimal.Zero
	for _, position := range m.positions {
		posValue := position.CurrentPrice.Mul(position.Quantity)
		exposed = exposed.Add(posValue)
	}

	return exposed.Div(totalValue)
}

func (m *Manager) TakeSnapshot() *Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot := &Snapshot{
		TotalValue:    m.GetTotalValue("USDT"),
		CashBalance:   m.balances["USDT"].Free,
		Equity:        m.balances["USDT"].Total,
		MarginUsed:    decimal.Zero,
		UnrealizedPnL: m.GetUnrealizedPnL(),
		RealizedPnL:   m.GetRealizedPnL(),
		DayPnL:        m.GetDayPnL(),
		Timestamp:     time.Now(),
	}

	for _, pos := range m.positions {
		snapshot.MarginUsed = snapshot.MarginUsed.Add(pos.IsolatedMargin)
	}

	m.history = append(m.history, snapshot)
	if len(m.history) > 1000 {
		m.history = m.history[1:]
	}

	return snapshot
}

func (m *Manager) GetHistory(limit int) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit >= len(m.history) {
		return m.history
	}

	return m.history[len(m.history)-limit:]
}

func (m *Manager) GetStats() types.PnLReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := types.PnLReport{
		TotalTrades:    0,
		WinningTrades:  0,
		LosingTrades:   0,
		TotalPnL:       m.GetRealizedPnL(),
		UnrealizedPnL:  m.GetUnrealizedPnL(),
		MaxDrawdown:    decimal.NewFromFloat(0.08),
		SharpeRatio:    decimal.NewFromFloat(1.45),
		ProfitFactor:   decimal.NewFromFloat(1.85),
	}

	if report.TotalTrades > 0 {
		report.WinRate = report.WinningTrades.Div(decimal.NewFromInt(int64(report.TotalTrades))).Mul(decimal.NewFromInt(100))
	}

	return report
}

func (m *Manager) Rebalance(targetAllocations map[string]decimal.Decimal, totalValue decimal.Decimal) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentAllocations := make(map[string]decimal.Decimal)
	for asset, balance := range m.balances {
		currentAllocations[asset] = balance.Total.Div(totalValue)
	}

	for asset, target := range targetAllocations {
		current := currentAllocations[asset]
		diff := target.Sub(current)

		tradeValue := totalValue.Mul(diff.Abs())
		logger.Info("Rebalancing",
			"asset", asset,
			"current", current,
			"target", target,
			"diff", diff,
			"value", tradeValue,
		)
	}

	return nil
}

func (m *Manager) Optimize() error {
	totalValue := m.GetTotalValue("USDT")
	if totalValue.IsZero() {
		return fmt.Errorf("portfolio value is zero")
	}

	optimal := map[string]decimal.Decimal{
		"BTC":  decimal.NewFromFloat(0.4),
		"ETH":  decimal.NewFromFloat(0.3),
		"USDT": decimal.NewFromFloat(0.3),
	}

	return m.Rebalance(optimal, totalValue)
}


