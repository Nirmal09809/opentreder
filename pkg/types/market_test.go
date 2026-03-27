package types

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

var testTime = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

func TestPriceFromFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected decimal.Decimal
	}{
		{"positive", 123.45, decimal.NewFromFloat(123.45)},
		{"negative", -123.45, decimal.NewFromFloat(-123.45)},
		{"zero", 0, decimal.Zero},
		{"large", 1000000.123, decimal.NewFromFloat(1000000.123)},
		{"small", 0.00001, decimal.NewFromFloat(0.00001)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PriceFromFloat(tt.input)
			assert.True(t, tt.expected.Equal(result), "expected %v, got %v", tt.expected, result)
		})
	}
}

func TestQuantityFromFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected decimal.Decimal
	}{
		{"simple", 1.5, decimal.NewFromFloat(1.5)},
		{"zero", 0, decimal.Zero},
		{"fraction", 0.001, decimal.NewFromFloat(0.001)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := QuantityFromFloat(tt.input)
			assert.True(t, tt.expected.Equal(result))
		})
	}
}

func TestMaxPrice(t *testing.T) {
	tests := []struct {
		name     string
		a        decimal.Decimal
		b        decimal.Decimal
		expected decimal.Decimal
	}{
		{"a greater", decimal.NewFromFloat(100), decimal.NewFromFloat(50), decimal.NewFromFloat(100)},
		{"b greater", decimal.NewFromFloat(50), decimal.NewFromFloat(100), decimal.NewFromFloat(100)},
		{"equal", decimal.NewFromFloat(100), decimal.NewFromFloat(100), decimal.NewFromFloat(100)},
		{"negative", decimal.NewFromFloat(-50), decimal.NewFromFloat(-100), decimal.NewFromFloat(-50)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaxPrice(tt.a, tt.b)
			assert.True(t, tt.expected.Equal(result))
		})
	}
}

func TestMinPrice(t *testing.T) {
	tests := []struct {
		name     string
		a        decimal.Decimal
		b        decimal.Decimal
		expected decimal.Decimal
	}{
		{"a smaller", decimal.NewFromFloat(50), decimal.NewFromFloat(100), decimal.NewFromFloat(50)},
		{"b smaller", decimal.NewFromFloat(100), decimal.NewFromFloat(50), decimal.NewFromFloat(50)},
		{"equal", decimal.NewFromFloat(100), decimal.NewFromFloat(100), decimal.NewFromFloat(100)},
		{"negative", decimal.NewFromFloat(-100), decimal.NewFromFloat(-50), decimal.NewFromFloat(-100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MinPrice(tt.a, tt.b)
			assert.True(t, tt.expected.Equal(result))
		})
	}
}

func TestDecimalToFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    decimal.Decimal
		expected float64
	}{
		{"simple", decimal.NewFromFloat(123.45), 123.45},
		{"zero", decimal.Zero, 0.0},
		{"negative", decimal.NewFromFloat(-123.45), -123.45},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecimalToFloat(tt.input)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestUUIDFromString(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"
	result := UUIDFromString(validUUID)
	assert.NotEmpty(t, result.String())

	invalidUUID := "invalid-uuid"
	result2 := UUIDFromString(invalidUUID)
	assert.NotEmpty(t, result2.String())
}

func TestUUIDFromInt64(t *testing.T) {
	result := UUIDFromInt64(12345)
	assert.NotEmpty(t, result.String())
}

func TestNewTicker(t *testing.T) {
	ticker := NewTicker("BTCUSDT", ExchangeBinance, 50000.0)
	assert.Equal(t, "BTCUSDT", ticker.Symbol)
	assert.Equal(t, ExchangeBinance, ticker.Exchange)
	assert.True(t, decimal.NewFromFloat(50000.0).Equal(ticker.LastPrice))
	assert.False(t, ticker.Timestamp.IsZero())
}

func TestNewCandle(t *testing.T) {
	symbol := "ETHUSDT"
	exchange := ExchangeBinance
	tf := Timeframe1h

	candle := NewCandle(symbol, exchange, tf, testTime)

	assert.Equal(t, symbol, candle.Symbol)
	assert.Equal(t, exchange, candle.Exchange)
	assert.Equal(t, string(tf), candle.Timeframe)
	assert.Equal(t, testTime, candle.Timestamp)
	assert.Equal(t, testTime.Add(time.Hour), candle.EndTime)
	assert.False(t, candle.Closed)
}

func TestNewOrder(t *testing.T) {
	order := NewOrder("BTCUSDT", ExchangeBinance, OrderSideBuy, OrderTypeLimit, 0.1, 50000.0)

	assert.NotEmpty(t, order.ID.String())
	assert.NotEmpty(t, order.ClientOrderID)
	assert.Equal(t, "BTCUSDT", order.Symbol)
	assert.Equal(t, ExchangeBinance, order.Exchange)
	assert.Equal(t, OrderSideBuy, order.Side)
	assert.Equal(t, OrderTypeLimit, order.Type)
	assert.Equal(t, OrderStatusPending, order.Status)
	assert.True(t, decimal.NewFromFloat(0.1).Equal(order.Quantity))
	assert.True(t, decimal.NewFromFloat(50000.0).Equal(order.Price))
	assert.Equal(t, TimeInForceGTC, order.TimeInForce)
}

func TestNewPosition(t *testing.T) {
	pos := NewPosition("BTCUSDT", ExchangeBinance, PositionSideLong, 0.5, 45000.0)

	assert.NotEmpty(t, pos.ID.String())
	assert.Equal(t, "BTCUSDT", pos.Symbol)
	assert.Equal(t, ExchangeBinance, pos.Exchange)
	assert.Equal(t, PositionSideLong, pos.Side)
	assert.True(t, decimal.NewFromFloat(0.5).Equal(pos.Quantity))
	assert.True(t, decimal.NewFromFloat(45000.0).Equal(pos.AvgEntryPrice))
	assert.True(t, decimal.NewFromInt(1).Equal(pos.Leverage))
}

func TestOrderIsFilled(t *testing.T) {
	tests := []struct {
		name     string
		status   OrderStatus
		expected bool
	}{
		{"filled", OrderStatusFilled, true},
		{"pending", OrderStatusPending, false},
		{"open", OrderStatusOpen, false},
		{"cancelled", OrderStatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &Order{Status: tt.status}
			assert.Equal(t, tt.expected, order.IsFilled())
		})
	}
}

func TestOrderIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   OrderStatus
		expected bool
	}{
		{"pending", OrderStatusPending, true},
		{"open", OrderStatusOpen, true},
		{"partial", OrderStatusPartiallyFilled, true},
		{"filled", OrderStatusFilled, false},
		{"cancelled", OrderStatusCancelled, false},
		{"rejected", OrderStatusRejected, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &Order{Status: tt.status}
			assert.Equal(t, tt.expected, order.IsActive())
		})
	}
}

func TestOrderRemainingPercent(t *testing.T) {
	tests := []struct {
		name          string
		quantity      decimal.Decimal
		remainingQty  decimal.Decimal
		expected      decimal.Decimal
	}{
		{"half filled", decimal.NewFromFloat(1.0), decimal.NewFromFloat(0.5), decimal.NewFromFloat(50)},
		{"nothing filled", decimal.NewFromFloat(1.0), decimal.NewFromFloat(1.0), decimal.NewFromFloat(100)},
		{"fully filled", decimal.NewFromFloat(1.0), decimal.NewFromFloat(0), decimal.NewFromFloat(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &Order{
				Quantity:    tt.quantity,
				RemainingQty: tt.remainingQty,
			}
			result := order.RemainingPercent()
			assert.True(t, tt.expected.Equal(result), "expected %v, got %v", tt.expected, result)
		})
	}
}

func TestPositionUnrealizedReturn(t *testing.T) {
	tests := []struct {
		name        string
		entryPrice  decimal.Decimal
		currentPrice decimal.Decimal
		expected    decimal.Decimal
	}{
		{"10% profit", decimal.NewFromFloat(100), decimal.NewFromFloat(110), decimal.NewFromFloat(10)},
		{"10% loss", decimal.NewFromFloat(100), decimal.NewFromFloat(90), decimal.NewFromFloat(-10)},
		{"no change", decimal.NewFromFloat(100), decimal.NewFromFloat(100), decimal.NewFromFloat(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := &Position{
				AvgEntryPrice: tt.entryPrice,
				CurrentPrice:  tt.currentPrice,
			}
			result := pos.UnrealizedReturn()
			assert.True(t, tt.expected.Equal(result), "expected %v, got %v", tt.expected, result)
		})
	}
}

func TestPositionUpdatePnL(t *testing.T) {
	pos := NewPosition("BTCUSDT", ExchangeBinance, PositionSideLong, 0.5, 45000.0)
	pos.UpdatePnL(50000.0)

	assert.True(t, decimal.NewFromFloat(50000.0).Equal(pos.CurrentPrice))
	expectedPnL := decimal.NewFromFloat(0.5).Mul(decimal.NewFromFloat(5000.0))
	assert.True(t, expectedPnL.Equal(pos.UnrealizedPnL), "expected %v, got %v", expectedPnL, pos.UnrealizedPnL)
}

func TestTimeframeDuration(t *testing.T) {
	tests := []struct {
		timeframe Timeframe
		expected  time.Duration
	}{
		{Timeframe1m, time.Minute},
		{Timeframe5m, 5 * time.Minute},
		{Timeframe15m, 15 * time.Minute},
		{Timeframe30m, 30 * time.Minute},
		{Timeframe1h, time.Hour},
		{Timeframe4h, 4 * time.Hour},
		{Timeframe1d, 24 * time.Hour},
		{Timeframe1w, 7 * 24 * time.Hour},
		{Timeframe1M, 30 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(string(tt.timeframe), func(t *testing.T) {
			result := tt.timeframe.Duration()
			assert.Equal(t, tt.expected, result)
		})
	}
}
