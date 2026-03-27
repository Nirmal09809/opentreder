package precision

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestUnixNano(t *testing.T) {
	t.Run("Now returns current time", func(t *testing.T) {
		start := Now()
		time.Sleep(time.Millisecond)
		end := Now()
		assert.True(t, end.Nano() > start.Nano())
	})

	t.Run("Time conversion", func(t *testing.T) {
		now := time.Now()
		unix := NewUnixNano(now)
		assert.Equal(t, now.UnixNano(), int64(unix))
		assert.Equal(t, now.Unix(), unix.Unix())
	})

	t.Run("FromUnix", func(t *testing.T) {
		unixTime := int64(1609459200)
		unix := FromUnix(unixTime)
		assert.Equal(t, unixTime*1e9, unix.Nano())
	})
}

func TestPrice(t *testing.T) {
	t.Run("NewPrice", func(t *testing.T) {
		p := NewPrice(123456, 5)
		assert.Equal(t, int64(123456), p.Int64())
		assert.Equal(t, int32(5), p.Scale())
	})

	t.Run("PriceFromFloat", func(t *testing.T) {
		p := PriceFromFloat(1.23456, 5)
		assert.Equal(t, int32(5), p.Scale())
		d := p.Decimal()
		assert.True(t, d.GreaterThan(decimal.NewFromFloat(1.23)))
	})

	t.Run("PriceFromDecimal", func(t *testing.T) {
		d := decimal.NewFromFloat(99.99)
		p := PriceFromDecimal(d, 2)
		assert.Equal(t, int32(2), p.Scale())
		dec := p.Decimal()
		assert.True(t, dec.GreaterThan(decimal.NewFromFloat(99)))
	})

	t.Run("Decimal conversion", func(t *testing.T) {
		p := NewPrice(12345, 3)
		d := p.Decimal()
		assert.True(t, d.Equal(decimal.NewFromFloat(12.345)))
	})

	t.Run("Float64 conversion", func(t *testing.T) {
		p := NewPrice(12345, 3)
		f := p.Float64()
		assert.InDelta(t, 12.345, f, 0.001)
	})

	t.Run("Add", func(t *testing.T) {
		p1 := NewPrice(1000, 2)
		p2 := NewPrice(500, 2)
		result := p1.Add(p2)
		assert.Equal(t, int64(1500), result.Int64())
	})

	t.Run("Add with different scales", func(t *testing.T) {
		p1 := NewPrice(1000, 2)
		p2 := NewPrice(50, 1)
		result := p1.Add(p2)
		assert.Equal(t, int32(2), result.Scale())
		assert.Equal(t, int64(1500), result.Int64())
	})

	t.Run("Sub", func(t *testing.T) {
		p1 := NewPrice(1000, 2)
		p2 := NewPrice(300, 2)
		result := p1.Sub(p2)
		assert.Equal(t, int64(700), result.Int64())
	})

	t.Run("Mul", func(t *testing.T) {
		p := NewPrice(1000, 2)
		result := p.Mul(3)
		assert.Equal(t, int64(3000), result.Int64())
	})

	t.Run("Div", func(t *testing.T) {
		p := NewPrice(3000, 2)
		result := p.Div(3)
		assert.Equal(t, int64(1000), result.Int64())
	})

	t.Run("Rescale", func(t *testing.T) {
		p := NewPrice(12345, 3)
		rescaled := p.Rescale(5)
		assert.Equal(t, int64(1234500), rescaled.Int64())
		assert.Equal(t, int32(5), rescaled.Scale())
	})

	t.Run("Rescale down", func(t *testing.T) {
		p := NewPrice(12345, 3)
		rescaled := p.Rescale(1)
		assert.Equal(t, int32(1), rescaled.Scale())
	})

	t.Run("Round", func(t *testing.T) {
		p := NewPrice(123456, 4)
		rounded := p.Round(2)
		assert.Equal(t, int32(2), rounded.Scale())
	})

	t.Run("IsZero", func(t *testing.T) {
		p := NewPrice(0, 2)
		assert.True(t, p.IsZero())
	})

	t.Run("IsPositive", func(t *testing.T) {
		p := NewPrice(100, 2)
		assert.True(t, p.IsPositive())
		assert.False(t, p.IsNegative())
	})

	t.Run("IsNegative", func(t *testing.T) {
		p := NewPrice(-100, 2)
		assert.True(t, p.IsNegative())
		assert.False(t, p.IsPositive())
	})

	t.Run("Comparison", func(t *testing.T) {
		p1 := NewPrice(1000, 2)
		p2 := NewPrice(2000, 2)
		p3 := NewPrice(1000, 2)

		assert.True(t, p2.GreaterThan(p1))
		assert.True(t, p1.LessThan(p2))
		assert.True(t, p1.Equal(p3))
		assert.True(t, p1.GreaterOrEqual(p3))
		assert.True(t, p1.LessOrEqual(p3))
	})
}

func TestQuantity(t *testing.T) {
	t.Run("NewQuantity", func(t *testing.T) {
		q := NewQuantity(1000, 3)
		assert.Equal(t, int64(1000), q.Int64())
		assert.Equal(t, int32(3), q.Scale())
	})

	t.Run("QuantityFromFloat", func(t *testing.T) {
		q := QuantityFromFloat(1.5, 2)
		assert.Equal(t, int32(2), q.Scale())
		d := q.Decimal()
		assert.True(t, d.GreaterThan(decimal.NewFromFloat(1.4)))
	})

	t.Run("Decimal conversion", func(t *testing.T) {
		q := NewQuantity(1234, 2)
		d := q.Decimal()
		assert.True(t, d.Equal(decimal.NewFromFloat(12.34)))
	})

	t.Run("Add", func(t *testing.T) {
		q1 := NewQuantity(1000, 3)
		q2 := NewQuantity(500, 3)
		result := q1.Add(q2)
		assert.Equal(t, int64(1500), result.Int64())
	})

	t.Run("Sub", func(t *testing.T) {
		q1 := NewQuantity(1000, 3)
		q2 := NewQuantity(300, 3)
		result := q1.Sub(q2)
		assert.Equal(t, int64(700), result.Int64())
	})

	t.Run("Mul", func(t *testing.T) {
		q := NewQuantity(1000, 3)
		result := q.Mul(2)
		assert.Equal(t, int64(2000), result.Int64())
	})

	t.Run("Div", func(t *testing.T) {
		q := NewQuantity(1000, 3)
		result := q.Div(2)
		assert.Equal(t, int64(500), result.Int64())
	})
}

func TestQuote(t *testing.T) {
	t.Run("NewQuote", func(t *testing.T) {
		bid := NewPrice(10000, 2)
		ask := NewPrice(10005, 2)
		bidQty := NewQuantity(100, 0)
		askQty := NewQuantity(100, 0)
		time := Now()

		q := NewQuote(bid, ask, bidQty, askQty, time)
		assert.Equal(t, bid, q.Bid)
		assert.Equal(t, ask, q.Ask)
	})

	t.Run("Spread", func(t *testing.T) {
		bid := NewPrice(10000, 2)
		ask := NewPrice(10005, 2)
		q := NewQuote(bid, ask, NewQuantity(0, 0), NewQuantity(0, 0), Now())

		spread := q.Spread()
		assert.Equal(t, int64(5), spread.Int64())
	})

	t.Run("MidPrice", func(t *testing.T) {
		bid := NewPrice(10000, 2)
		ask := NewPrice(10010, 2)
		q := NewQuote(bid, ask, NewQuantity(0, 0), NewQuantity(0, 0), Now())

		mid := q.MidPrice()
		assert.Equal(t, int64(10005), mid.Int64())
	})
}

func TestTrade(t *testing.T) {
	t.Run("NewTrade", func(t *testing.T) {
		p := NewPrice(50000, 2)
		q := NewQuantity(100, 0)
		time := Now()

		trade := NewTrade("trade-1", p, q, "buy", time, "binance", "BTCUSDT")
		assert.Equal(t, "trade-1", trade.ID)
		assert.Equal(t, "buy", trade.Side)
		assert.Equal(t, "binance", trade.Exchange)
	})
}

func TestBar(t *testing.T) {
	t.Run("NewBar", func(t *testing.T) {
		open := NewPrice(10000, 2)
		high := NewPrice(10100, 2)
		low := NewPrice(9900, 2)
		close := NewPrice(10050, 2)
		vol := NewQuantity(1000, 0)
		start := Now()
		end := Now()

		bar := NewBar("BTCUSDT", "binance", open, high, low, close, vol, NewQuantity(0, 0), start, end)
		assert.Equal(t, "BTCUSDT", bar.Symbol)
	})

	t.Run("Range", func(t *testing.T) {
		high := NewPrice(10100, 2)
		low := NewPrice(9900, 2)
		bar := newTestBar(high, low)

		rng := bar.Range()
		assert.Equal(t, int64(200), rng.Int64())
	})

	t.Run("IsBullish", func(t *testing.T) {
		open := NewPrice(10000, 2)
		close := NewPrice(10100, 2)
		bar := newTestBarWithPrices(open, close)

		assert.True(t, bar.IsBullish())
		assert.False(t, bar.IsBearish())
	})

	t.Run("IsBearish", func(t *testing.T) {
		open := NewPrice(10100, 2)
		close := NewPrice(10000, 2)
		bar := newTestBarWithPrices(open, close)

		assert.False(t, bar.IsBullish())
		assert.True(t, bar.IsBearish())
	})

	t.Run("Duration", func(t *testing.T) {
		start := NewUnixNano(time.Unix(0, 1000000000))
		end := NewUnixNano(time.Unix(0, 2000000000))
		bar := NewBar("BTCUSDT", "binance", NewPrice(0, 0), NewPrice(0, 0), NewPrice(0, 0), NewPrice(0, 0), NewQuantity(0, 0), NewQuantity(0, 0), start, end)

		assert.Equal(t, int64(1e9), bar.Duration())
	})
}

func newTestBar(high, low Price) Bar {
	return NewBar("BTCUSDT", "binance",
		NewPrice(0, 0), high, low, NewPrice(0, 0),
		NewQuantity(0, 0), NewQuantity(0, 0),
		Now(), Now())
}

func newTestBarWithPrices(open, close Price) Bar {
	return NewBar("BTCUSDT", "binance",
		open, NewPrice(0, 0), NewPrice(0, 0), close,
		NewQuantity(0, 0), NewQuantity(0, 0),
		Now(), Now())
}

func TestOrderBook(t *testing.T) {
	t.Run("NewOrderBook", func(t *testing.T) {
		bids := []OrderBookLevel{
			{Price: NewPrice(10000, 2), Quantity: NewQuantity(100, 0)},
			{Price: NewPrice(9990, 2), Quantity: NewQuantity(50, 0)},
		}
		asks := []OrderBookLevel{
			{Price: NewPrice(10010, 2), Quantity: NewQuantity(80, 0)},
			{Price: NewPrice(10020, 2), Quantity: NewQuantity(60, 0)},
		}

		ob := NewOrderBook("BTCUSDT", "binance", bids, asks, Now())
		assert.Equal(t, "BTCUSDT", ob.Symbol)
	})

	t.Run("BestBid", func(t *testing.T) {
		bids := []OrderBookLevel{
			{Price: NewPrice(10000, 2), Quantity: NewQuantity(100, 0)},
			{Price: NewPrice(9990, 2), Quantity: NewQuantity(50, 0)},
		}
		ob := NewOrderBook("BTCUSDT", "binance", bids, []OrderBookLevel{}, Now())

		best := ob.BestBid()
		assert.Equal(t, int64(10000), best.Int64())
	})

	t.Run("BestAsk", func(t *testing.T) {
		asks := []OrderBookLevel{
			{Price: NewPrice(10010, 2), Quantity: NewQuantity(80, 0)},
			{Price: NewPrice(10020, 2), Quantity: NewQuantity(60, 0)},
		}
		ob := NewOrderBook("BTCUSDT", "binance", []OrderBookLevel{}, asks, Now())

		best := ob.BestAsk()
		assert.Equal(t, int64(10010), best.Int64())
	})

	t.Run("Spread", func(t *testing.T) {
		bids := []OrderBookLevel{
			{Price: NewPrice(10000, 2), Quantity: NewQuantity(100, 0)},
		}
		asks := []OrderBookLevel{
			{Price: NewPrice(10010, 2), Quantity: NewQuantity(80, 0)},
		}
		ob := NewOrderBook("BTCUSDT", "binance", bids, asks, Now())

		spread := ob.Spread()
		assert.Equal(t, int64(10), spread.Int64())
	})

	t.Run("MidPrice", func(t *testing.T) {
		bids := []OrderBookLevel{
			{Price: NewPrice(10000, 2), Quantity: NewQuantity(100, 0)},
		}
		asks := []OrderBookLevel{
			{Price: NewPrice(10010, 2), Quantity: NewQuantity(80, 0)},
		}
		ob := NewOrderBook("BTCUSDT", "binance", bids, asks, Now())

		mid := ob.MidPrice()
		assert.Equal(t, int64(10005), mid.Int64())
	})
}

func TestPosition(t *testing.T) {
	t.Run("MarketValue", func(t *testing.T) {
		pos := &Position{
			Quantity:  NewQuantity(100, 0),
			CurrentPx: NewPrice(50000, 2),
		}

		mv := pos.MarketValue()
		assert.True(t, mv.GreaterThan(decimal.NewFromFloat(0)))
	})

	t.Run("CostBasis", func(t *testing.T) {
		pos := &Position{
			Quantity:  NewQuantity(100, 0),
			AvgEntryPx: NewPrice(45000, 2),
		}

		cb := pos.CostBasis()
		assert.True(t, cb.GreaterThan(decimal.NewFromFloat(0)))
	})

	t.Run("UnrealizedPnLPct", func(t *testing.T) {
		pos := &Position{
			Quantity:    NewQuantity(100, 0),
			AvgEntryPx: NewPrice(45000, 2),
			UnrealizedPnL: decimal.NewFromFloat(50000),
		}

		pct := pos.UnrealizedPnLPct()
		assert.True(t, pct.GreaterThan(decimal.Zero))
	})
}

func TestOrder(t *testing.T) {
	t.Run("RemainingQty", func(t *testing.T) {
		o := &Order{
			Quantity:  NewQuantity(100, 0),
			FilledQty: NewQuantity(30, 0),
		}

		remaining := o.RemainingQty()
		assert.Equal(t, int64(70), remaining.Int64())
	})

	t.Run("FillPct", func(t *testing.T) {
		o := &Order{
			Quantity:  NewQuantity(100, 0),
			FilledQty: NewQuantity(50, 0),
		}

		pct := o.FillPct()
		assert.True(t, pct.Equal(decimal.NewFromFloat(50)))
	})

	t.Run("IsFilled", func(t *testing.T) {
		o := &Order{
			Quantity:  NewQuantity(100, 0),
			FilledQty: NewQuantity(100, 0),
			Status:    "filled",
		}

		assert.True(t, o.IsFilled())
	})

	t.Run("IsOpen", func(t *testing.T) {
		o := &Order{
			Quantity: NewQuantity(100, 0),
			FilledQty: NewQuantity(50, 0),
			Status:    "partial",
		}

		assert.True(t, o.IsOpen())
	})
}

func TestAccount(t *testing.T) {
	t.Run("MarginUtilization", func(t *testing.T) {
		a := &Account{
			NetLiquidation: decimal.NewFromFloat(100000),
			MarginUsed:     decimal.NewFromFloat(25000),
		}

		mu := a.MarginUtilization()
		assert.True(t, mu.Equal(decimal.NewFromFloat(25)))
	})

	t.Run("Leverage", func(t *testing.T) {
		a := &Account{
			NetLiquidation: decimal.NewFromFloat(100000),
			Equity:         decimal.NewFromFloat(10000),
		}

		lev := a.Leverage()
		assert.True(t, lev.Equal(decimal.NewFromFloat(10)))
	})
}

func BenchmarkPriceArithmetic(b *testing.B) {
	p1 := NewPrice(10000, 2)
	p2 := NewPrice(5000, 2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p1.Add(p2)
		_ = p1.Sub(p2)
		_ = p1.Mul(3)
		_ = p1.Div(2)
	}
}

func BenchmarkPriceComparison(b *testing.B) {
	p1 := NewPrice(10000, 2)
	p2 := NewPrice(15000, 2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p1.GreaterThan(p2)
		_ = p1.LessThan(p2)
		_ = p1.Equal(p2)
	}
}
