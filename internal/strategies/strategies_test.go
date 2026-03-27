package strategies

import (
	"context"
	"testing"
	"time"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNewBaseStrategy(t *testing.T) {
	strategy := NewBaseStrategy("TestStrategy", "test")

	assert.NotEmpty(t, strategy.ID)
	assert.Equal(t, "TestStrategy", strategy.Name)
	assert.Equal(t, "test", strategy.Type)
	assert.Equal(t, StateIdle, strategy.state)
	assert.NotNil(t, strategy.positions)
	assert.NotNil(t, strategy.signals)
	assert.NotNil(t, strategy.candles)
}

func TestBaseStrategyInit(t *testing.T) {
	strategy := NewBaseStrategy("TestStrategy", "test")
	ctx := context.Background()

	err := strategy.Init(ctx)

	assert.NoError(t, err)
	assert.Equal(t, StateIdle, strategy.GetState())
}

func TestBaseStrategyStartStop(t *testing.T) {
	strategy := NewBaseStrategy("TestStrategy", "test")
	ctx := context.Background()
	strategy.Init(ctx)

	err := strategy.Start()
	assert.NoError(t, err)
	assert.Equal(t, StateRunning, strategy.GetState())

	err = strategy.Stop()
	assert.NoError(t, err)
	assert.Equal(t, StateStopped, strategy.GetState())
}

func TestBaseStrategyOnCandle(t *testing.T) {
	strategy := NewBaseStrategy("TestStrategy", "test")

	candle := &types.Candle{
		Symbol:    "BTCUSDT",
		Timeframe: "1h",
		Close:     decimal.NewFromFloat(50000.0),
	}

	strategy.OnCandle(candle)

	candles := strategy.GetCandles("BTCUSDT", "1h", 100)
	assert.Len(t, candles, 1)
	assert.True(t, candles[0].Close.Equal(candle.Close))
}

func TestBaseStrategyOnSignal(t *testing.T) {
	strategy := NewBaseStrategy("TestStrategy", "test")

	signal := &types.Signal{
		Action: types.SignalActionBuy,
		Reason: "Test signal",
	}

	strategy.OnSignal(signal)

	signals := strategy.GetSignals()
	assert.Len(t, signals, 1)
	assert.Equal(t, signal.Action, signals[0].Action)
}

func TestNewGridStrategy(t *testing.T) {
	grid := NewGridStrategy("BTCUSDT", 10, 0.01, 0.01)

	assert.NotNil(t, grid.BaseStrategy)
	assert.Equal(t, 10, grid.gridLevels)
	assert.True(t, decimal.NewFromFloat(0.01).Equal(grid.gridSpacing))
	assert.True(t, decimal.NewFromFloat(0.01).Equal(grid.orderSize))
}

func TestNewDCAStrategy(t *testing.T) {
	dca := NewDCAStrategy("BTCUSDT", 100.0, 24*time.Hour)

	assert.NotNil(t, dca.BaseStrategy)
	assert.True(t, decimal.NewFromFloat(100.0).Equal(dca.amount))
	assert.Equal(t, 24*time.Hour, dca.frequency)
}

func TestNewTrendStrategy(t *testing.T) {
	trend := NewTrendStrategy("BTCUSDT", 10, 20, 9)

	assert.NotNil(t, trend.BaseStrategy)
	assert.Equal(t, 10, trend.fastPeriod)
	assert.Equal(t, 20, trend.slowPeriod)
	assert.Equal(t, 9, trend.signalPeriod)
}

func TestNewScalperStrategy(t *testing.T) {
	scalper := NewScalperStrategy("BTCUSDT", 0.005, 0.002)

	assert.NotNil(t, scalper.BaseStrategy)
	assert.True(t, decimal.NewFromFloat(0.005).Equal(scalper.profitTarget))
	assert.True(t, decimal.NewFromFloat(0.002).Equal(scalper.stopLoss))
}

func TestNewArbitrageStrategy(t *testing.T) {
	arb := NewArbitrageStrategy()

	assert.NotNil(t, arb.BaseStrategy)
	assert.Len(t, arb.exchanges, 2)
	assert.True(t, decimal.NewFromFloat(0.001).Equal(arb.minProfitMargin))
}

func TestStrategyManager(t *testing.T) {
	manager := NewStrategyManager()

	strategy1 := NewBaseStrategy("Strategy1", "test")
	strategy2 := NewBaseStrategy("Strategy2", "test")

	err := manager.AddStrategy("strat1", strategy1)
	assert.NoError(t, err)

	err = manager.AddStrategy("strat1", strategy2)
	assert.Error(t, err)

	err = manager.AddStrategy("strat2", strategy2)
	assert.NoError(t, err)

	retrieved := manager.GetStrategy("strat1")
	assert.Equal(t, strategy1, retrieved)

	all := manager.GetAllStrategies()
	assert.Len(t, all, 2)

	err = manager.RemoveStrategy("strat1")
	assert.NoError(t, err)

	removed := manager.GetStrategy("strat1")
	assert.Nil(t, removed)
}

func TestStrategyFactory(t *testing.T) {
	factory := NewStrategyFactory()

	tests := []struct {
		name       string
		strategyType string
		symbol     string
		shouldErr  bool
	}{
		{"grid", "grid", "BTCUSDT", false},
		{"dca", "dca", "ETHUSDT", false},
		{"trend", "trend", "BTCUSDT", false},
		{"scalping", "scalping", "BTCUSDT", false},
		{"unknown", "unknown", "BTCUSDT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := factory.Create(tt.strategyType, tt.symbol, nil)
			if tt.shouldErr {
				assert.Error(t, err)
				assert.Nil(t, strategy)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, strategy)
			}
		})
	}
}

func TestGridStrategyOnCandle(t *testing.T) {
	grid := NewGridStrategy("BTCUSDT", 5, 100.0, 0.01)
	ctx := context.Background()
	grid.Init(ctx)
	grid.Start()

	candle := &types.Candle{
		Symbol:    "BTCUSDT",
		Timeframe: "1h",
		Close:     decimal.NewFromFloat(44000.0),
	}

	grid.OnCandle(candle)

	grid.Stop()
}

func TestDCAStrategy(t *testing.T) {
	dca := NewDCAStrategy("BTCUSDT", 100.0, time.Millisecond*50)
	ctx := context.Background()
	dca.Init(ctx)

	err := dca.Start()
	assert.NoError(t, err)

	time.Sleep(150 * time.Millisecond)

	err = dca.Stop()
	assert.NoError(t, err)

	signals := dca.GetSignals()
	assert.NotNil(t, signals)
}

func TestTrendStrategy(t *testing.T) {
	trend := NewTrendStrategy("BTCUSDT", 3, 6, 3)
	ctx := context.Background()
	trend.Init(ctx)

	for i := 0; i < 10; i++ {
		price := 44000.0 + float64(i)*100
		candle := &types.Candle{
			Symbol:    "BTCUSDT",
			Timeframe: "1h",
			Close:     decimal.NewFromFloat(price),
		}
		trend.OnCandle(candle)
	}
}

func TestHotReloadConfig(t *testing.T) {
	config := HotReloadConfig{
		WatchDir:         "./strategies",
		WatchInterval:    5 * time.Second,
		EnableAutoReload: true,
		BackupDir:       "./backups",
		MaxBackups:      5,
	}

	assert.Equal(t, "./strategies", config.WatchDir)
	assert.Equal(t, 5*time.Second, config.WatchInterval)
	assert.True(t, config.EnableAutoReload)
	assert.Equal(t, "./backups", config.BackupDir)
	assert.Equal(t, 5, config.MaxBackups)
}
