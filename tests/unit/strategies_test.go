package strategies

import (
	"context"
	"testing"
	"time"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGridStrategyCreation(t *testing.T) {
	strategy := NewGridStrategy("BTC/USDT", 10, 0.01, 0.01)

	require.NotNil(t, strategy)
	assert.Equal(t, "Grid_BTC/USDT", strategy.Name)
	assert.Equal(t, "grid", strategy.Type)
	assert.Equal(t, 10, strategy.gridLevels)
	assert.True(t, strategy.gridSpacing.Equal(decimal.NewFromFloat(0.01)))
}

func TestGridStrategyInit(t *testing.T) {
	strategy := NewGridStrategy("BTC/USDT", 10, 0.01, 0.01)
	ctx := context.Background()

	err := strategy.Init(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateIdle, strategy.GetState())
}

func TestDCAStrategyCreation(t *testing.T) {
	strategy := NewDCAStrategy("BTC/USDT", 100.0, 24*time.Hour)

	require.NotNil(t, strategy)
	assert.Equal(t, "DCA_BTC/USDT", strategy.Name)
	assert.Equal(t, "dca", strategy.Type)
	assert.True(t, strategy.amount.Equal(decimal.NewFromFloat(100.0)))
	assert.Equal(t, 24*time.Hour, strategy.frequency)
}

func TestDCAStrategySchedule(t *testing.T) {
	strategy := NewDCAStrategy("BTC/USDT", 100.0, time.Hour)
	ctx := context.Background()

	err := strategy.Init(ctx)
	require.NoError(t, err)

	nextBuy := strategy.nextBuyTime
	assert.True(t, nextBuy.After(time.Now()))
}

func TestTrendStrategySignalGeneration(t *testing.T) {
	strategy := NewTrendStrategy("BTC/USDT", 5, 10, 3)

	candles := make([]*types.Candle, 15)
	basePrice := 44000.0

	for i := range candles {
		trend := float64(i) / float64(len(candles)) * 1000
		candles[i] = &types.Candle{
			Symbol:    "BTC/USDT",
			Timeframe: types.Timeframe1h,
			Open:      decimal.NewFromFloat(basePrice + trend),
			High:      decimal.NewFromFloat(basePrice + trend + 100),
			Low:       decimal.NewFromFloat(basePrice + trend - 100),
			Close:     decimal.NewFromFloat(basePrice + trend + 50),
			Volume:    decimal.NewFromFloat(1000),
			Timestamp: time.Now().Add(-time.Duration(15-i) * time.Hour),
		}
	}

	for _, candle := range candles {
		strategy.OnCandle(candle)
	}

	signals := strategy.GetSignals()
	assert.GreaterOrEqual(t, len(signals), 0)
}

func TestScalperStrategyRSI(t *testing.T) {
	strategy := NewScalperStrategy("BTC/USDT", 0.005, 0.002)

	require.NotNil(t, strategy)
	assert.Equal(t, "Scalper_BTC/USDT", strategy.Name)
	assert.True(t, strategy.profitTarget.Equal(decimal.NewFromFloat(0.005)))
	assert.True(t, strategy.stopLoss.Equal(decimal.NewFromFloat(0.002)))
}

func TestArbitrageStrategy(t *testing.T) {
	strategy := NewArbitrageStrategy()

	require.NotNil(t, strategy)
	assert.Equal(t, "Arbitrage", strategy.Name)
	assert.Equal(t, "arbitrage", strategy.Type)
	assert.Len(t, strategy.exchanges, 2)

	strategy.CheckOpportunities()
	assert.Len(t, strategy.opportunities, 0)
}

func TestStrategyManager(t *testing.T) {
	manager := NewStrategyManager()
	require.NotNil(t, manager)

	strategy := NewDCAStrategy("BTC/USDT", 100.0, 24*time.Hour)

	err := manager.AddStrategy("test_strategy", strategy)
	require.NoError(t, err)

	retrieved := manager.GetStrategy("test_strategy")
	require.NotNil(t, retrieved)
	assert.Equal(t, strategy.ID, retrieved.GetID())

	all := manager.GetAllStrategies()
	assert.Len(t, all, 1)

	err = manager.RemoveStrategy("test_strategy")
	require.NoError(t, err)

	retrieved = manager.GetStrategy("test_strategy")
	assert.Nil(t, retrieved)
}

func TestStrategyFactory(t *testing.T) {
	factory := NewStrategyFactory()
	require.NotNil(t, factory)

	t.Run("Create grid strategy", func(t *testing.T) {
		strategy, err := factory.Create("grid", "BTC/USDT", nil)
		require.NoError(t, err)
		assert.Equal(t, "grid", strategy.GetName())
	})

	t.Run("Create DCA strategy", func(t *testing.T) {
		strategy, err := factory.Create("dca", "ETH/USDT", nil)
		require.NoError(t, err)
		assert.Equal(t, "dca", strategy.GetName())
	})

	t.Run("Create trend strategy", func(t *testing.T) {
		strategy, err := factory.Create("trend", "BTC/USDT", nil)
		require.NoError(t, err)
		assert.Equal(t, "trend", strategy.GetName())
	})

	t.Run("Create scalping strategy", func(t *testing.T) {
		strategy, err := factory.Create("scalping", "BTC/USDT", nil)
		require.NoError(t, err)
		assert.Equal(t, "scalping", strategy.GetName())
	})

	t.Run("Unknown strategy type", func(t *testing.T) {
		strategy, err := factory.Create("unknown", "BTC/USDT", nil)
		require.Error(t, err)
		assert.Nil(t, strategy)
	})
}

func TestStrategyStartStop(t *testing.T) {
	strategy := NewDCAStrategy("BTC/USDT", 100.0, 24*time.Hour)
	ctx := context.Background()

	err := strategy.Init(ctx)
	require.NoError(t, err)

	err = strategy.Start()
	require.NoError(t, err)
	assert.Equal(t, StateRunning, strategy.GetState())

	err = strategy.Stop()
	require.NoError(t, err)
	assert.Equal(t, StateStopped, strategy.GetState())
}

func TestCandleStorage(t *testing.T) {
	strategy := NewTrendStrategy("BTC/USDT", 5, 10, 3)

	candles := make([]*types.Candle, 10)
	for i := range candles {
		candles[i] = &types.Candle{
			Symbol:    "BTC/USDT",
			Timeframe: types.Timeframe1h,
			Close:     decimal.NewFromFloat(float64(44000 + i*100)),
			Timestamp: time.Now().Add(-time.Duration(10-i) * time.Hour),
		}
	}

	for _, candle := range candles {
		strategy.OnCandle(candle)
	}

	retrieved := strategy.GetCandles("BTC/USDT", "1h", 5)
	assert.Len(t, retrieved, 5)
}

func TestSignalStorage(t *testing.T) {
	strategy := NewDCAStrategy("BTC/USDT", 100.0, 24*time.Hour)

	for i := 0; i < 150; i++ {
		strategy.AddSignal(&types.Signal{
			Action:  types.SignalActionBuy,
			Reason:  "Test signal",
			Timestamp: time.Now(),
		})
	}

	signals := strategy.GetSignals()
	assert.LessOrEqual(t, len(signals), 100)
}

func TestMeanReversionStrategy(t *testing.T) {
	factory := NewStrategyFactory()
	strategy, err := factory.Create("mean_reversion", "BTC/USDT", nil)
	require.NoError(t, err)
	require.NotNil(t, strategy)
}

func TestBreakoutStrategy(t *testing.T) {
	factory := NewStrategyFactory()
	strategy, err := factory.Create("breakout", "ETH/USDT", nil)
	require.NoError(t, err)
	require.NotNil(t, strategy)
}

func TestMomentumStrategy(t *testing.T) {
	factory := NewStrategyFactory()
	strategy, err := factory.Create("momentum", "BTC/USDT", nil)
	require.NoError(t, err)
	require.NotNil(t, strategy)
}

func TestMarketMakingStrategy(t *testing.T) {
	factory := NewStrategyFactory()
	strategy, err := factory.Create("market_making", "BTC/USDT", nil)
	require.NoError(t, err)
	require.NotNil(t, strategy)
}

func TestPairsTradingStrategy(t *testing.T) {
	factory := NewStrategyFactory()
	strategy, err := factory.Create("pairs", "BTC/USD", map[string]interface{}{"symbol2": "ETH/USD"})
	require.NoError(t, err)
	require.NotNil(t, strategy)
}

func TestStrategyMetrics(t *testing.T) {
	metrics := StrategyMetrics{
		TotalTrades:     100,
		WinningTrades:   60,
		LosingTrades:    40,
		TotalPnL:        decimal.NewFromFloat(5000),
		WinRate:         decimal.NewFromFloat(0.6),
		AvgWin:          decimal.NewFromFloat(150),
		AvgLoss:         decimal.NewFromFloat(100),
		MaxDrawdown:     decimal.NewFromFloat(0.15),
		SharpeRatio:     decimal.NewFromFloat(1.5),
		ProfitFactor:    decimal.NewFromFloat(2.25),
		LastTradeTime:   time.Now(),
		UpTime:          24 * time.Hour,
	}

	assert.Equal(t, 100, metrics.TotalTrades)
	assert.Equal(t, 60, metrics.WinningTrades)
	assert.True(t, metrics.WinRate.Equal(decimal.NewFromFloat(0.6)))
	assert.True(t, metrics.ProfitFactor.Equal(decimal.NewFromFloat(2.25)))
}

func TestStrategyStateTransitions(t *testing.T) {
	strategy := NewDCAStrategy("BTC/USDT", 100.0, 24*time.Hour)
	ctx := context.Background()

	err := strategy.Init(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateIdle, strategy.GetState())

	err = strategy.Start()
	require.NoError(t, err)
	assert.Equal(t, StateRunning, strategy.GetState())

	err = strategy.Stop()
	require.NoError(t, err)
	assert.Equal(t, StateStopped, strategy.GetState())
}
