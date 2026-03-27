# Strategy Development Guide

## Overview

OpenTrader supports multiple trading strategies that can be implemented and deployed. This guide covers the strategy interface, lifecycle, and implementation examples.

## Strategy Interface

All strategies must implement the `Strategy` interface:

```go
type Strategy interface {
    // Core lifecycle methods
    Initialize(ctx context.Context, engine *Engine) error
    OnStart(ctx context.Context) error
    OnStop(ctx context.Context) error
    OnTick(ctx context.Context, tick *Tick) error
    OnCandle(ctx context.Context, candle *Candle) error
    OnSignal(ctx context.Context, signal *Signal) error
    
    // Position management
    OnPositionOpened(ctx context.Context, position *Position) error
    OnPositionClosed(ctx context.Context, position *Position, reason CloseReason) error
    OnOrderFilled(ctx context.Context, order *Order) error
    
    // Getters
    GetID() string
    GetName() string
    GetStatus() StrategyStatus
    GetParameters() map[string]interface{}
    
    // Risk management
    ValidateSignal(ctx context.Context, signal *Signal) error
}
```

## Strategy Lifecycle

```
┌─────────────┐
│   Created  │
└──────┬──────┘
       │ Initialize()
       ▼
┌─────────────┐
│Initialized  │◄─────────────────────┐
└──────┬──────┘                      │
       │ Start()                      │
       ▼                              │
┌─────────────┐                       │
│   Running   │                       │
└──────┬──────┘                       │
       │                         OnError()
       ▼                              │
┌─────────────┐                        │
│  Paused    │────────────────────────┘
└──────┬──────┘
       │ Stop()
       ▼
┌─────────────┐
│  Stopped   │
└─────────────┘
```

## Built-in Strategies

### 1. Grid Strategy

Places buy and sell orders at regular intervals around a base price.

```yaml
parameters:
  symbol: BTC/USDT
  grid_levels: 10
  grid_spacing: 0.01  # 1% spacing
  order_size: 0.001
  base_price: 50000
  stop_loss: 0.05  # 5% stop loss
```

### 2. DCA (Dollar Cost Averaging)

Invests fixed amounts at regular intervals.

```yaml
parameters:
  symbol: BTC/USDT
  investment_amount: 100
  investment_interval: 1d
  max_positions: 12
  stop_loss: 0.15
  take_profit: 0.10
```

### 3. Trend Following

Uses technical indicators to identify and follow trends.

```yaml
parameters:
  symbol: BTC/USDT
  timeframe: 1h
  indicators:
    - type: ema
      period: 50
    - type: ema
      period: 200
  entry_threshold: 0.01
  exit_threshold: -0.01
  stop_loss: 0.02
  take_profit: 0.05
```

### 4. Arbitrage

Exploits price differences between exchanges.

```yaml
parameters:
  symbols:
    - BTC/USDT
    - ETH/USDT
  exchanges:
    - binance
    - bybit
  min_profit: 0.001  # 0.1% minimum profit
  max_position: 0.1
  cooldown: 60  # seconds
```

### 5. Scalping

High-frequency trading with small profit targets.

```yaml
parameters:
  symbol: BTC/USDT
  timeframe: 1m
  indicators:
    - type: bollinger_bands
      period: 20
    - type: rsi
      period: 14
  profit_target: 0.0005  # 0.05%
  stop_loss: 0.001
  max_spread: 0.0002
```

## Creating a Custom Strategy

### Step 1: Define the Strategy

```go
package strategies

import (
    "context"
    "github.com/opentreder/opentreder/pkg/types"
    "github.com/shopspring/decimal"
)

type MyStrategy struct {
    id        string
    name      string
    params    StrategyParams
    engine    *Engine
    positions  []*Position
    lastPrice  decimal.Decimal
}

type StrategyParams struct {
    Symbol         string
    BuyThreshold   decimal.Decimal
    SellThreshold  decimal.Decimal
    PositionSize   decimal.Decimal
    StopLoss       decimal.Decimal
}

func NewMyStrategy(id, name string, params StrategyParams) *MyStrategy {
    return &MyStrategy{
        id:     id,
        name:   name,
        params: params,
    }
}
```

### Step 2: Implement Interface

```go
func (s *MyStrategy) Initialize(ctx context.Context, engine *Engine) error {
    s.engine = engine
    
    // Subscribe to market data
    if err := engine.Subscribe(s.params.Symbol, types.SubscribeTicker); err != nil {
        return err
    }
    
    return nil
}

func (s *MyStrategy) OnStart(ctx context.Context) error {
    log.Printf("Strategy %s started", s.name)
    return nil
}

func (s *MyStrategy) OnStop(ctx context.Context) error {
    // Close all positions
    for _, pos := range s.positions {
        if err := s.engine.ClosePosition(pos.ID); err != nil {
            log.Printf("Error closing position %s: %v", pos.ID, err)
        }
    }
    return nil
}

func (s *MyStrategy) OnTick(ctx context.Context, tick *types.Tick) error {
    if tick.Symbol != s.params.Symbol {
        return nil
    }
    
    s.lastPrice = tick.Price
    
    // Check entry conditions
    if s.shouldBuy() {
        return s.placeBuyOrder(ctx)
    }
    
    if s.shouldSell() {
        return s.placeSellOrder(ctx)
    }
    
    return nil
}

func (s *MyStrategy) shouldBuy() bool {
    // Example: Buy when price drops 1% from last price
    if s.lastPrice.IsZero() {
        return false
    }
    
    drop := s.lastPrice.Sub(s.params.BuyThreshold)
    return true
}

func (s *MyStrategy) placeBuyOrder(ctx context.Context) error {
    order := &types.Order{
        Symbol:    s.params.Symbol,
        Side:      types.OrderSideBuy,
        Type:      types.OrderTypeLimit,
        Quantity:  s.params.PositionSize,
        Price:     s.lastPrice,
        TimeInForce: types.TimeInForceGTC,
    }
    
    return s.engine.SubmitOrder(order)
}
```

### Step 3: Implement Position Callbacks

```go
func (s *MyStrategy) OnPositionOpened(ctx context.Context, position *types.Position) error {
    log.Printf("Position opened: %s %s @ %s", 
        position.Symbol, position.Quantity, position.EntryPrice)
    
    s.positions = append(s.positions, position)
    
    // Set stop loss order
    stopPrice := position.EntryPrice.Mul(decimal.NewFromFloat(1).Sub(s.params.StopLoss))
    
    stopOrder := &types.Order{
        Symbol:    position.Symbol,
        Side:      types.OrderSideSell,
        Type:      types.OrderTypeStop,
        Quantity:  position.Quantity,
        StopPrice: stopPrice,
    }
    
    return s.engine.SubmitOrder(stopOrder)
}

func (s *MyStrategy) OnPositionClosed(ctx context.Context, position *types.Position, reason types.CloseReason) error {
    log.Printf("Position closed: %s, reason: %s, PnL: %s", 
        position.Symbol, reason, position.UnrealizedPnL)
    
    // Remove from positions list
    for i, p := range s.positions {
        if p.ID == position.ID {
            s.positions = append(s.positions[:i], s.positions[i+1:]...)
            break
        }
    }
    
    return nil
}
```

### Step 4: Risk Validation

```go
func (s *MyStrategy) ValidateSignal(ctx context.Context, signal *types.Signal) error {
    // Check position limit
    if len(s.positions) >= s.params.MaxPositions {
        return ErrMaxPositionsReached
    }
    
    // Check daily loss limit
    dailyLoss, err := s.engine.GetDailyLoss()
    if err != nil {
        return err
    }
    
    if dailyLoss.GreaterThan(s.params.MaxDailyLoss) {
        return ErrDailyLossLimit
    }
    
    return nil
}
```

### Step 5: Register Strategy

```go
// In strategy factory
func RegisterStrategy(name string, factory func(id string, params map[string]interface{}) Strategy) {
    strategies[name] = factory
}

func init() {
    RegisterStrategy("my_strategy", func(id string, params map[string]interface{}) Strategy {
        return NewMyStrategy(
            id,
            params["name"].(string),
            StrategyParams{
                Symbol:        params["symbol"].(string),
                BuyThreshold:   decimal.NewFromFloat(params["buy_threshold"].(float64)),
                SellThreshold: decimal.NewFromFloat(params["sell_threshold"].(float64)),
                PositionSize:  decimal.NewFromFloat(params["position_size"].(float64)),
                StopLoss:      decimal.NewFromFloat(params["stop_loss"].(float64)),
            },
        )
    })
}
```

## Strategy Parameters

### Common Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `symbol` | string | Trading symbol (e.g., BTC/USDT) |
| `timeframe` | string | Candle timeframe |
| `position_size` | decimal | Order quantity |
| `stop_loss` | decimal | Stop loss percentage |
| `take_profit` | decimal | Take profit percentage |

### Indicator Parameters

Each indicator has specific parameters:

```yaml
# EMA
indicators:
  - type: ema
    period: 50
    source: close

# RSI
indicators:
  - type: rsi
    period: 14
    overbought: 70
    oversold: 30

# MACD
indicators:
  - type: macd
    fast_period: 12
    slow_period: 26
    signal_period: 9

# Bollinger Bands
indicators:
  - type: bollinger_bands
    period: 20
    std_dev: 2
```

## Backtesting

### Running a Backtest

```bash
# Via CLI
opentreder backtest \
  --strategy grid \
  --symbol BTC/USDT \
  --start 2024-01-01 \
  --end 2024-12-31 \
  --initial-balance 100000 \
  --output results.json

# Via API
curl -X POST http://localhost:8080/api/v1/backtest \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "strategyId": "grid-001",
    "symbol": "BTC/USDT",
    "startDate": "2024-01-01",
    "endDate": "2024-12-31",
    "parameters": {
      "grid_levels": 10,
      "grid_spacing": 0.01
    }
  }'
```

### Backtest Results

```json
{
  "jobId": "backtest-123",
  "status": "completed",
  "result": {
    "initialBalance": "100000.00",
    "finalBalance": "125430.50",
    "totalTrades": 245,
    "winningTrades": 142,
    "losingTrades": 103,
    "winRate": 0.58,
    "profitFactor": 1.45,
    "maxDrawdown": "8.5%",
    "sharpeRatio": 1.82,
    "sortinoRatio": 2.15,
    "calmarRatio": 0.85,
    "avgWin": "125.50",
    "avgLoss": "-85.20"
  }
}
```

## Strategy Performance Metrics

| Metric | Description | Good Range |
|--------|-------------|------------|
| Win Rate | % of profitable trades | > 50% |
| Profit Factor | Gross profit / Gross loss | > 1.2 |
| Sharpe Ratio | Risk-adjusted return | > 1.0 |
| Sortino Ratio | Downside risk-adjusted return | > 1.5 |
| Max Drawdown | Largest peak-to-trough | < 20% |
| Calmar Ratio | Return / Max drawdown | > 1.0 |

## Best Practices

1. **Always set stop losses** - Protect against adverse moves
2. **Validate signals** - Check risk limits before placing orders
3. **Use position limits** - Don't overexpose to single symbol
4. **Log everything** - Debugging is essential for strategies
5. **Test thoroughly** - Backtest before going live
6. **Start small** - Use paper trading first
7. **Monitor continuously** - Watch for anomalies

## Examples

See `internal/strategies/` for implementation examples:
- `grid.go` - Grid trading strategy
- `dca.go` - Dollar cost averaging
- `trend.go` - Trend following
- `arbitrage.go` - Exchange arbitrage
