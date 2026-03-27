//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/opentreder/opentreder/internal/core/engine"
	"github.com/opentreder/opentreder/internal/core/orders"
	"github.com/opentreder/opentreder/internal/core/portfolio"
	"github.com/opentreder/opentreder/internal/core/risk"
	"github.com/opentreder/opentreder/pkg/config"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEngine(t *testing.T) (*engine.Engine, func()) {
	cfg := &config.Config{
		Trading: config.TradingConfig{
			Mode:             "paper",
			InitialBalance:   decimal.NewFromInt(100000),
			MaxPositions:    10,
			DefaultTimeframe: "1h",
		},
		Risk: config.RiskConfig{
			MaxPositionSize: decimal.NewFromFloat(0.1),
			MaxDailyLoss:    decimal.NewFromFloat(0.05),
			MaxLeverage:     decimal.NewFromInt(3),
		},
		Database: config.DatabaseConfig{
			Mode: "memory",
		},
	}

	eng := engine.New(cfg)
	ctx := context.Background()

	err := eng.Start(ctx)
	require.NoError(t, err)

	cleanup := func() {
		eng.Stop()
	}

	return eng, cleanup
}

func TestEngine_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	assert.Equal(t, engine.StateRunning, eng.Status())

	eng.Stop()
	assert.Equal(t, engine.StateStopped, eng.Status())
}

func TestEngine_SubmitOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	order := &types.Order{
		Symbol:       "BTC/USDT",
		Side:         types.OrderSideBuy,
		Type:         types.OrderTypeLimit,
		Quantity:     decimal.NewFromFloat(0.001),
		Price:        decimal.NewFromFloat(50000),
		TimeInForce:  types.TimeInForceGTC,
	}

	err := eng.SubmitOrder(order)
	require.NoError(t, err)

	orders := eng.GetOrders()
	assert.NotEmpty(t, orders)
	assert.Equal(t, types.OrderStatusOpen, orders[0].Status)
}

func TestEngine_SubmitAndCancelOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	order := &types.Order{
		Symbol:      "ETH/USDT",
		Side:        types.OrderSideSell,
		Type:        types.OrderTypeLimit,
		Quantity:    decimal.NewFromFloat(1),
		Price:       decimal.NewFromFloat(3000),
		TimeInForce: types.TimeInForceGTC,
	}

	err := eng.SubmitOrder(order)
	require.NoError(t, err)

	err = eng.CancelOrder(order.ID.String())
	require.NoError(t, err)

	cancelledOrder := eng.GetOrder(order.ID.String())
	assert.Equal(t, types.OrderStatusCancelled, cancelledOrder.Status)
}

func TestEngine_GetPositions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	positions := eng.GetPositions()
	assert.NotNil(t, positions)
	assert.Empty(t, positions)
}

func TestEngine_InvalidOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	order := &types.Order{
		Symbol:   "INVALID/PAIR",
		Side:     types.OrderSideBuy,
		Type:     types.OrderTypeMarket,
		Quantity: decimal.NewFromFloat(1000),
	}

	err := eng.SubmitOrder(order)
	assert.Error(t, err)
}

func TestEngine_RiskLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	largeOrder := &types.Order{
		Symbol:   "BTC/USDT",
		Side:     types.OrderSideBuy,
		Type:     types.OrderTypeMarket,
		Quantity: decimal.NewFromFloat(10),
	}

	err := eng.SubmitOrder(largeOrder)
	assert.Error(t, err)
}

type mockExchange struct {
	connected bool
}

func (m *mockExchange) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *mockExchange) Disconnect() error {
	m.connected = false
	return nil
}

func TestOrderManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	cfg := &config.Config{}
	eng := engine.New(cfg)
	orderMgr := orders.NewManager(eng)

	order := &types.Order{
		Symbol:      "BTC/USDT",
		Side:        types.OrderSideBuy,
		Type:        types.OrderTypeLimit,
		Quantity:    decimal.NewFromFloat(0.1),
		Price:       decimal.NewFromFloat(50000),
		TimeInForce: types.TimeInForceGTC,
	}

	ctx := context.Background()
	err := orderMgr.Submit(ctx, order)
	require.NoError(t, err)

	activeOrders := orderMgr.GetActiveOrders()
	assert.NotEmpty(t, activeOrders)
}

func TestPortfolioManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	portMgr := portfolio.NewManager()

	position := &types.Position{
		Symbol:    "BTC/USDT",
		Quantity:  decimal.NewFromFloat(1),
		EntryPrice: decimal.NewFromFloat(50000),
	}

	ctx := context.Background()
	err := portMgr.AddPosition(ctx, position)
	require.NoError(t, err)

	positions := portMgr.GetPositions()
	assert.NotEmpty(t, positions)
	assert.Equal(t, "BTC/USDT", positions[0].Symbol)

	balance := portMgr.GetBalance()
	assert.True(t, balance.GreaterThan(decimal.Zero))
}

func TestRiskManager(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	cfg := &config.Config{
		Risk: config.RiskConfig{
			MaxPositionSize: decimal.NewFromFloat(0.1),
			MaxDailyLoss:    decimal.NewFromFloat(0.05),
			MaxLeverage:     decimal.NewFromInt(3),
		},
	}

	eng := engine.New(cfg)
	riskMgr := risk.NewManager(eng)

	position := &types.Position{
		Symbol:      "BTC/USDT",
		Quantity:    decimal.NewFromFloat(0.1),
		EntryPrice:  decimal.NewFromFloat(50000),
	}

	allowed, err := riskMgr.ValidatePosition(context.Background(), position)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestConcurrentOrders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	orderCount := 100
	errors := make(chan error, orderCount)

	for i := 0; i < orderCount; i++ {
		go func(idx int) {
			order := &types.Order{
				Symbol:      "BTC/USDT",
				Side:        types.OrderSideBuy,
				Type:        types.OrderTypeLimit,
				Quantity:    decimal.NewFromFloat(0.001),
				Price:       decimal.NewFromFloat(50000 + float64(idx)),
				TimeInForce: types.TimeInForceGTC,
			}
			errors <- eng.SubmitOrder(order)
		}(i)
	}

	time.Sleep(2 * time.Second)

	orders := eng.GetOrders()
	assert.Len(t, orders, orderCount)
}

func TestOrderLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	order := &types.Order{
		Symbol:      "BTC/USDT",
		Side:        types.OrderSideBuy,
		Type:        types.OrderTypeLimit,
		Quantity:    decimal.NewFromFloat(0.001),
		Price:       decimal.NewFromFloat(50000),
		TimeInForce: types.TimeInForceGTC,
	}

	ctx := context.Background()

	err := eng.SubmitOrder(order)
	require.NoError(t, err)
	assert.Equal(t, types.OrderStatusPending, order.Status)

	time.Sleep(100 * time.Millisecond)
	order = eng.GetOrder(order.ID.String())
	assert.Equal(t, types.OrderStatusOpen, order.Status)

	err = eng.CancelOrder(order.ID.String())
	require.NoError(t, err)
	order = eng.GetOrder(order.ID.String())
	assert.Equal(t, types.OrderStatusCancelled, order.Status)
}
