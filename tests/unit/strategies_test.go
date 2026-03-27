package opentreder_test

import (
	"context"
	"testing"
	"time"

	"github.com/opentreder/opentreder/internal/strategies"
	"github.com/stretchr/testify/require"
)

func TestGridStrategyCreation(t *testing.T) {
	strategy := strategies.NewGridStrategy("BTC/USDT", 10, 0.01, 0.01)
	require.NotNil(t, strategy)
}

func TestDCAStrategyCreation(t *testing.T) {
	strategy := strategies.NewDCAStrategy("BTC/USDT", 100.0, 24*time.Hour)
	require.NotNil(t, strategy)
}

func TestTrendStrategyCreation(t *testing.T) {
	strategy := strategies.NewTrendStrategy("BTC/USDT", 5, 10, 3)
	require.NotNil(t, strategy)
}

func TestScalperStrategyCreation(t *testing.T) {
	strategy := strategies.NewScalperStrategy("BTC/USDT", 0.005, 0.002)
	require.NotNil(t, strategy)
}

func TestArbitrageStrategyCreation(t *testing.T) {
	strategy := strategies.NewArbitrageStrategy()
	require.NotNil(t, strategy)
}

func TestStrategyManagerCreation(t *testing.T) {
	manager := strategies.NewStrategyManager()
	require.NotNil(t, manager)
}

func TestStrategyFactoryCreation(t *testing.T) {
	factory := strategies.NewStrategyFactory()
	require.NotNil(t, factory)
}

func TestGridStrategyInit(t *testing.T) {
	strategy := strategies.NewGridStrategy("BTC/USDT", 10, 0.01, 0.01)
	ctx := context.Background()
	err := strategy.Init(ctx)
	require.NoError(t, err)
}

func TestDCAStrategyInit(t *testing.T) {
	strategy := strategies.NewDCAStrategy("BTC/USDT", 100.0, 24*time.Hour)
	ctx := context.Background()
	err := strategy.Init(ctx)
	require.NoError(t, err)
}
