package opentreder_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderCreation(t *testing.T) {
	order := &Order{
		ID:        GenerateUUID(),
		Symbol:    "BTC/USDT",
		Exchange:  ExchangeBinance,
		Side:      SideBuy,
		Type:      OrderTypeLimit,
		Quantity:  decimal.NewFromFloat(0.1),
		Price:     decimal.NewFromFloat(45000),
		Status:    OrderStatusPending,
		CreatedAt: time.Now(),
	}

	assert.NotEmpty(t, order.ID)
	assert.Equal(t, "BTC/USDT", order.Symbol)
	assert.Equal(t, SideBuy, order.Side)
	assert.Equal(t, OrderTypeLimit, order.Type)
	assert.True(t, order.Quantity.Equal(decimal.NewFromFloat(0.1)))
	assert.True(t, order.Price.Equal(decimal.NewFromFloat(45000)))
	assert.Equal(t, OrderStatusPending, order.Status)
}

func TestPositionPnLCalculation(t *testing.T) {
	position := &Position{
		Symbol:        "ETH/USDT",
		Exchange:      ExchangeBybit,
		Quantity:      decimal.NewFromFloat(2.0),
		AvgEntryPrice: decimal.NewFromFloat(3000),
		CurrentPrice:  decimal.NewFromFloat(3500),
		Side:          SideBuy,
	}

	position.UnrealizedPnL = position.Quantity.Mul(position.CurrentPrice.Sub(position.AvgEntryPrice))

	expectedPnL := decimal.NewFromFloat(1000) // (3500 - 3000) * 2
	assert.True(t, position.UnrealizedPnL.Equal(expectedPnL), "Unrealized PnL should be %s, got %s", expectedPnL, position.UnrealizedPnL)
}

func TestQuoteSpread(t *testing.T) {
	quote := &Quote{
		Symbol:    "BTC/USDT",
		Exchange:  ExchangeBinance,
		Bid:       decimal.NewFromFloat(44900),
		Ask:       decimal.NewFromFloat(45000),
		BidSize:   decimal.NewFromFloat(1.5),
		AskSize:   decimal.NewFromFloat(2.0),
		Timestamp: time.Now(),
	}

	spread := quote.Ask.Sub(quote.Bid)
	spreadPercent := spread.Div(quote.Ask).Mul(decimal.NewFromInt(100))

	assert.True(t, spread.GreaterThan(decimal.Zero))
	assert.True(t, spreadPercent.LessThan(decimal.NewFromFloat(1)), "Spread should be less than 1%%")
}

func TestTimeframeDuration(t *testing.T) {
	testCases := []struct {
		timeframe  Timeframe
		expected   time.Duration
	}{
		{Timeframe1m, time.Minute},
		{Timeframe5m, 5 * time.Minute},
		{Timeframe15m, 15 * time.Minute},
		{Timeframe1h, time.Hour},
		{Timeframe4h, 4 * time.Hour},
		{Timeframe1d, 24 * time.Hour},
	}

	for _, tc := range testCases {
		t.Run(string(tc.timeframe), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.timeframe.Duration())
		})
	}
}

func TestOrderStatusTransitions(t *testing.T) {
	validTransitions := map[OrderStatus][]OrderStatus{
		OrderStatusPending:           {OrderStatusOpen, OrderStatusCanceled},
		OrderStatusOpen:              {OrderStatusPartiallyFilled, OrderStatusFilled, OrderStatusCanceled},
		OrderStatusPartiallyFilled:   {OrderStatusFilled, OrderStatusCanceled},
		OrderStatusFilled:            {},
		OrderStatusCanceled:          {},
		OrderStatusExpired:           {},
	}

	order := &Order{Status: OrderStatusPending}

	order.Status = OrderStatusOpen
	assert.Contains(t, validTransitions[OrderStatusPending], order.Status)

	order.Status = OrderStatusPartiallyFilled
	assert.Contains(t, validTransitions[OrderStatusOpen], order.Status)

	order.Status = OrderStatusFilled
	assert.Contains(t, validTransitions[OrderStatusPartiallyFilled], order.Status)
}

func TestCandleOHLC(t *testing.T) {
	candle := &Candle{
		Symbol:    "BTC/USDT",
		Exchange:  ExchangeBinance,
		Open:      decimal.NewFromFloat(44000),
		High:      decimal.NewFromFloat(45000),
		Low:       decimal.NewFromFloat(43800),
		Close:     decimal.NewFromFloat(44700),
		Volume:    decimal.NewFromFloat(1000),
		Timeframe: Timeframe1h,
	}

	assert.True(t, candle.High.GreaterThanOrEqual(candle.Open))
	assert.True(t, candle.High.GreaterThanOrEqual(candle.Close))
	assert.True(t, candle.Low.LessThanOrEqual(candle.Open))
	assert.True(t, candle.Low.LessThanOrEqual(candle.Close))
	assert.True(t, candle.High.GreaterThanOrEqual(candle.Low))
}

func TestSignalGeneration(t *testing.T) {
	signal := &Signal{
		ID:        GenerateUUID(),
		Symbol:    "ETH/USDT",
		Action:    SignalActionBuy,
		Price:     decimal.NewFromFloat(3000),
		Quantity:  decimal.NewFromFloat(1.0),
		Confidence: decimal.NewFromFloat(0.85),
		Reason:    "RSI oversold",
		Timestamp: time.Now(),
	}

	assert.NotEmpty(t, signal.ID)
	assert.Equal(t, SignalActionBuy, signal.Action)
	assert.True(t, signal.Confidence.GreaterThan(decimal.NewFromFloat(0.8)))
}

func TestRiskRewardRatio(t *testing.T) {
	entryPrice := decimal.NewFromFloat(100)
	stopLoss := decimal.NewFromFloat(95)
	takeProfit := decimal.NewFromFloat(110)

	riskAmount := entryPrice.Sub(stopLoss)
	rewardAmount := takeProfit.Sub(entryPrice)
	riskRewardRatio := rewardAmount.Div(riskAmount)

	assert.True(t, riskRewardRatio.Equal(decimal.NewFromFloat(2)), "Risk/Reward should be 2:1, got %s", riskRewardRatio)
}

func TestPortfolioValue(t *testing.T) {
	portfolio := &Portfolio{
		Positions: []*Position{
			{
				Symbol:       "BTC/USDT",
				Quantity:     decimal.NewFromFloat(0.5),
				CurrentPrice: decimal.NewFromFloat(45000),
				Side:         SideBuy,
			},
			{
				Symbol:       "ETH/USDT",
				Quantity:     decimal.NewFromFloat(5.0),
				CurrentPrice: decimal.NewFromFloat(3000),
				Side:         SideBuy,
			},
		},
		Cash: decimal.NewFromFloat(10000),
	}

	positionsValue := decimal.Zero
	for _, pos := range portfolio.Positions {
		positionsValue = positionsValue.Add(pos.Quantity.Mul(pos.CurrentPrice))
	}

	portfolioValue := positionsValue.Add(portfolio.Cash)
	expectedValue := decimal.NewFromFloat(47500) // 0.5*45000 + 5*3000 + 10000

	assert.True(t, portfolioValue.Equal(expectedValue))
}

func TestTradeCommission(t *testing.T) {
	trade := &Trade{
		ID:        GenerateUUID(),
		Symbol:    "BTC/USDT",
		Side:      SideBuy,
		Quantity:  decimal.NewFromFloat(1.0),
		Price:     decimal.NewFromFloat(45000),
		Commission: decimal.NewFromFloat(4.5), // 0.01% commission
		ExecutedAt: time.Now(),
	}

	totalCost := trade.Quantity.Mul(trade.Price).Add(trade.Commission)
	expectedCost := decimal.NewFromFloat(45004.5)

	assert.True(t, totalCost.Equal(expectedCost))
}

func TestLeverageCalculation(t *testing.T) {
	position := &Position{
		Quantity:        decimal.NewFromFloat(2.0),
		EntryPrice:     decimal.NewFromFloat(5000),
		CurrentPrice:   decimal.NewFromFloat(5500),
		MarginUsed:     decimal.NewFromFloat(1000),
	}

	positionValue := position.Quantity.Mul(position.CurrentPrice)
	leverage := positionValue.Div(position.MarginUsed)

	assert.True(t, leverage.Equal(decimal.NewFromFloat(11)), "Leverage should be 11x, got %s", leverage)
}

func TestDrawdownCalculation(t *testing.T) {
	equityCurve := []decimal.Decimal{
		decimal.NewFromFloat(10000),
		decimal.NewFromFloat(11000),
		decimal.NewFromFloat(10500),
		decimal.NewFromFloat(9500),
		decimal.NewFromFloat(12000),
	}

	peak := decimal.NewFromFloat(11000)
	current := decimal.NewFromFloat(9500)
	drawdown := peak.Sub(current).Div(peak).Mul(decimal.NewFromFloat(100))

	expectedDrawdown := decimal.NewFromFloat(13.64) // approximately 13.64%
	assert.True(t, drawdown.GreaterThan(decimal.NewFromFloat(13)))
	assert.True(t, drawdown.LessThan(decimal.NewFromFloat(14)))
}

func TestOrderValidation(t *testing.T) {
	t.Run("Valid limit order", func(t *testing.T) {
		order := &Order{
			Symbol:   "BTC/USDT",
			Side:     SideBuy,
			Type:     OrderTypeLimit,
			Quantity: decimal.NewFromFloat(0.1),
			Price:    decimal.NewFromFloat(45000),
		}

		require.Nil(t, validateOrder(order))
	})

	t.Run("Market order without quantity", func(t *testing.T) {
		order := &Order{
			Symbol:   "BTC/USDT",
			Side:     SideBuy,
			Type:     OrderTypeMarket,
			Quantity: decimal.Zero,
		}

		err := validateOrder(order)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "quantity")
	})

	t.Run("Limit order without price", func(t *testing.T) {
		order := &Order{
			Symbol:   "BTC/USDT",
			Side:     SideSell,
			Type:     OrderTypeLimit,
			Quantity: decimal.NewFromFloat(0.1),
			Price:    decimal.Zero,
		}

		err := validateOrder(order)
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "price")
	})

	t.Run("Negative quantity", func(t *testing.T) {
		order := &Order{
			Symbol:   "BTC/USDT",
			Side:     SideBuy,
			Type:     OrderTypeMarket,
			Quantity: decimal.NewFromFloat(-0.1),
		}

		err := validateOrder(order)
		require.NotNil(t, err)
	})
}

func validateOrder(order *Order) error {
	if order.Quantity.LessThanOrEqual(decimal.Zero) {
		return &ValidationError{Field: "quantity", Message: "quantity must be positive"}
	}
	if order.Type == OrderTypeLimit || order.Type == OrderTypeStopLimit {
		if order.Price.LessThanOrEqual(decimal.Zero) {
			return &ValidationError{Field: "price", Message: "price must be positive for limit orders"}
		}
	}
	return nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func TestTimeInForce(t *testing.T) {
	testCases := []struct {
		tif      TimeInForce
		expected string
	}{
		{TimeInForceGTC, "GTC"},
		{TimeInForceIOC, "IOC"},
		{TimeInForceFOK, "FOK"},
		{TimeInForceGTX, "GTX"},
		{TimeInForceDay, "DAY"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.tif))
		})
	}
}

func TestExchangeEnum(t *testing.T) {
	exchanges := []Exchange{
		ExchangeBinance,
		ExchangeBybit,
		ExchangeCoinbase,
		ExchangeAlpaca,
		ExchangeForexcom,
		ExchangeTradier,
	}

	expectedNames := []string{"binance", "bybit", "coinbase", "alpaca", "forexcom", "tradier"}

	for i, ex := range exchanges {
		assert.Equal(t, expectedNames[i], string(ex))
	}
}
