package backtest

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type OrderBookSimulator struct {
	symbol     string
	exchange   types.Exchange
	bids       PriceHeap
	asks       PriceHeap
	trades     []SimTrade
	mu         sync.RWMutex
	config     OrderBookConfig
	lastPrice  decimal.Decimal
	spread     decimal.Decimal
	midPrice   decimal.Decimal
	volatility decimal.Decimal
}

type OrderBookConfig struct {
	InitialMidPrice   decimal.Decimal
	SpreadBps         int
	DepthLevels       int
	OrderSizeMin      decimal.Decimal
	OrderSizeMax      decimal.Decimal
	VolatilityBps     int
	UpdateIntervalMs  int
	FillProbability   float64
	MakerFee          decimal.Decimal
	TakerFee          decimal.Decimal
}

type PriceLevel struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
	Orders   int
}

type SimTrade struct {
	Price     decimal.Decimal
	Quantity  decimal.Decimal
	Side      string
	Timestamp time.Time
	IsMaker   bool
}

func NewOrderBookSimulator(symbol string, exchange types.Exchange, config OrderBookConfig) *OrderBookSimulator {
	sim := &OrderBookSimulator{
		symbol:   symbol,
		exchange: exchange,
		bids:     make(PriceHeap, 0),
		asks:     make(PriceHeap, 0),
		trades:   make([]SimTrade, 0),
		config:   config,
	}

	if config.InitialMidPrice.IsZero() {
		sim.lastPrice = decimal.NewFromFloat(44000.0)
	} else {
		sim.lastPrice = config.InitialMidPrice
	}

	sim.midPrice = sim.lastPrice
	sim.spread = sim.midPrice.Mul(decimal.NewFromInt(int64(config.SpreadBps))).Div(decimal.NewFromInt(10000))
	sim.volatility = sim.midPrice.Mul(decimal.NewFromInt(int64(config.VolatilityBps))).Div(decimal.NewFromInt(10000))

	sim.initializeOrderBook()

	return sim
}

func (s *OrderBookSimulator) initializeOrderBook() {
	halfSpread := s.spread.Div(decimal.NewFromInt(2))
	
	for i := 0; i < s.config.DepthLevels; i++ {
		priceImpact := math.Pow(1.01, float64(i+1))
		
		bidPrice := s.midPrice.Sub(halfSpread).Sub(s.midPrice.Mul(decimal.NewFromFloat(0.0001)).Mul(decimal.NewFromFloat(priceImpact)))
		askPrice := s.midPrice.Add(halfSpread).Add(s.midPrice.Mul(decimal.NewFromFloat(0.0001)).Mul(decimal.NewFromFloat(priceImpact)))
		
		bidSize := decimal.NewFromFloat(0.1).Mul(decimal.NewFromFloat(priceImpact))
		askSize := decimal.NewFromFloat(0.1).Mul(decimal.NewFromFloat(priceImpact))

		heap.Push(&s.bids, &PriceLevel{
			Price:    bidPrice.Round(2),
			Quantity: bidSize.Round(6),
			Orders:   int(math.Max(1, float64(i+1)/2.0)),
		})

		heap.Push(&s.asks, &PriceLevel{
			Price:    askPrice.Round(2),
			Quantity: askSize.Round(6),
			Orders:   int(math.Max(1, float64(i+1)/2.0)),
		})
	}

	heap.Init(&s.bids)
	heap.Init(&s.asks)
}

func (s *OrderBookSimulator) UpdateMarketPrice(price decimal.Decimal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastPrice = price
	s.midPrice = price
}

func (s *OrderBookSimulator) UpdateMidPrice() {
	s.mu.Lock()
	defer s.mu.Unlock()

	priceChange := decimal.NewFromFloat((rand.Float64() - 0.5) * 2)
	priceChange = priceChange.Mul(s.volatility)
	
	s.midPrice = s.midPrice.Add(priceChange)
	
	if s.midPrice.LessThan(decimal.NewFromFloat(1)) {
		s.midPrice = decimal.NewFromFloat(1)
	}

	spread := s.spread
	halfSpread := spread.Div(decimal.NewFromInt(2))

	for i := range s.bids {
		priceImpact := decimal.NewFromFloat(math.Pow(1.01, float64(i+1)))
		s.bids[i].Price = s.midPrice.Sub(halfSpread).Sub(s.midPrice.Mul(decimal.NewFromFloat(0.0001)).Mul(priceImpact))
		s.bids[i].Price = s.bids[i].Price.Round(2)
	}

	for i := range s.asks {
		priceImpact := decimal.NewFromFloat(math.Pow(1.01, float64(i+1)))
		s.asks[i].Price = s.midPrice.Add(halfSpread).Add(s.midPrice.Mul(decimal.NewFromFloat(0.0001)).Mul(priceImpact))
		s.asks[i].Price = s.asks[i].Price.Round(2)
	}

	heap.Init(&s.bids)
	heap.Init(&s.asks)
}

func (s *OrderBookSimulator) GetOrderBook() *types.OrderBook {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bids := make([]types.PriceLevel, len(s.bids))
	for i, level := range s.bids {
		bids[i] = types.PriceLevel{
			Price:    level.Price,
			Quantity: level.Quantity,
		}
	}

	asks := make([]types.PriceLevel, len(s.asks))
	for i, level := range s.asks {
		asks[i] = types.PriceLevel{
			Price:    level.Price,
			Quantity: level.Quantity,
		}
	}

	return &types.OrderBook{
		Symbol:    s.symbol,
		Exchange: s.exchange,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}
}

func (s *OrderBookSimulator) GetBestBid() decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.bids) == 0 {
		return decimal.Zero
	}
	return s.bids[0].Price
}

func (s *OrderBookSimulator) GetBestAsk() decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.asks) == 0 {
		return decimal.Zero
	}
	return s.asks[0].Price
}

func (s *OrderBookSimulator) GetSpread() decimal.Decimal {
	return s.GetBestAsk().Sub(s.GetBestBid())
}

func (s *OrderBookSimulator) GetMidPrice() decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.midPrice
}

func (s *OrderBookSimulator) GetVWAP(levels int) decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.bids) == 0 || len(s.asks) == 0 {
		return s.midPrice
	}

	bidTotal := decimal.Zero
	bidQty := decimal.Zero
	askTotal := decimal.Zero
	askQty := decimal.Zero

	for i := 0; i < levels && i < len(s.bids); i++ {
		bidTotal = bidTotal.Add(s.bids[i].Price.Mul(s.bids[i].Quantity))
		bidQty = bidQty.Add(s.bids[i].Quantity)
	}

	for i := 0; i < levels && i < len(s.asks); i++ {
		askTotal = askTotal.Add(s.asks[i].Price.Mul(s.asks[i].Quantity))
		askQty = askQty.Add(s.asks[i].Quantity)
	}

	totalQty := bidQty.Add(askQty)
	if totalQty.IsZero() {
		return s.midPrice
	}

	return (bidTotal.Add(askTotal)).Div(totalQty)
}

func (s *OrderBookSimulator) SimulateLimitOrder(side string, price decimal.Decimal, size decimal.Decimal) (*SimTrade, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filled bool
	var fillPrice decimal.Decimal

	if side == "buy" {
		if len(s.asks) > 0 && price.GreaterThanOrEqual(s.asks[0].Price) {
			fillPrice = s.asks[0].Price
			s.asks[0].Quantity = s.asks[0].Quantity.Sub(size)
			
			if s.asks[0].Quantity.LessThanOrEqual(decimal.Zero) {
				heap.Pop(&s.asks)
			}
			filled = true
		}
	} else {
		if len(s.bids) > 0 && price.LessThanOrEqual(s.bids[0].Price) {
			fillPrice = s.bids[0].Price
			s.bids[0].Quantity = s.bids[0].Quantity.Sub(size)
			
			if s.bids[0].Quantity.LessThanOrEqual(decimal.Zero) {
				heap.Pop(&s.bids)
			}
			filled = true
		}
	}

	if filled {
		trade := SimTrade{
			Price:     fillPrice,
			Quantity:  size,
			Side:      side,
			Timestamp: time.Now(),
			IsMaker:   false,
		}
		s.trades = append(s.trades, trade)
		s.lastPrice = fillPrice
		
		return &trade, true
	}

	return nil, false
}

func (s *OrderBookSimulator) SimulateMarketOrder(side string, size decimal.Decimal) (*SimTrade, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var totalCost decimal.Decimal
	var filledQty decimal.Decimal

	if side == "buy" {
		remaining := size
		for remaining.GreaterThan(decimal.Zero) && len(s.asks) > 0 {
			level := s.asks[0]
			consumeQty := decimal.Min(remaining, level.Quantity)
			
			totalCost = totalCost.Add(level.Price.Mul(consumeQty))
			filledQty = filledQty.Add(consumeQty)
			remaining = remaining.Sub(consumeQty)

			level.Quantity = level.Quantity.Sub(consumeQty)
			if level.Quantity.LessThanOrEqual(decimal.Zero) {
				heap.Pop(&s.asks)
			}
		}
	} else {
		remaining := size
		for remaining.GreaterThan(decimal.Zero) && len(s.bids) > 0 {
			level := s.bids[0]
			consumeQty := decimal.Min(remaining, level.Quantity)
			
			totalCost = totalCost.Add(level.Price.Mul(consumeQty))
			filledQty = filledQty.Add(consumeQty)
			remaining = remaining.Sub(consumeQty)

			level.Quantity = level.Quantity.Sub(consumeQty)
			if level.Quantity.LessThanOrEqual(decimal.Zero) {
				heap.Pop(&s.bids)
			}
		}
	}

	if filledQty.GreaterThan(decimal.Zero) {
		avgPrice := totalCost.Div(filledQty)
		
		trade := SimTrade{
			Price:     avgPrice,
			Quantity:  filledQty,
			Side:      side,
			Timestamp: time.Now(),
			IsMaker:   false,
		}
		s.trades = append(s.trades, trade)
		s.lastPrice = avgPrice

		return &trade, true
	}

	return nil, false
}

func (s *OrderBookSimulator) GetRecentTrades(count int) []SimTrade {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if count <= 0 || count >= len(s.trades) {
		return s.trades
	}

	return s.trades[len(s.trades)-count:]
}

func (s *OrderBookSimulator) GetTradesVolume(window time.Duration) decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	total := decimal.Zero

	for _, trade := range s.trades {
		if trade.Timestamp.After(cutoff) {
			total = total.Add(trade.Quantity)
		}
	}

	return total
}

func (s *OrderBookSimulator) GetOrderFlow(side string, window time.Duration) decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	total := decimal.Zero

	for _, trade := range s.trades {
		if trade.Timestamp.After(cutoff) && trade.Side == side {
			total = total.Add(trade.Quantity)
		}
	}

	return total
}

func (s *OrderBookSimulator) GetImbalance() decimal.Decimal {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bidQty := decimal.Zero
	askQty := decimal.Zero

	for i := 0; i < 5 && i < len(s.bids); i++ {
		bidQty = bidQty.Add(s.bids[i].Quantity)
	}

	for i := 0; i < 5 && i < len(s.asks); i++ {
		askQty = askQty.Add(s.asks[i].Quantity)
	}

	total := bidQty.Add(askQty)
	if total.IsZero() {
		return decimal.Zero
	}

	return (bidQty.Sub(askQty)).Div(total)
}

func (s *OrderBookSimulator) AddLiquidity(side string, price decimal.Decimal, size decimal.Decimal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	level := &PriceLevel{
		Price:    price,
		Quantity: size,
		Orders:   1,
	}

	if side == "buy" {
		heap.Push(&s.bids, level)
	} else {
		heap.Push(&s.asks, level)
	}
}

func (s *OrderBookSimulator) RemoveLiquidity(side string, price decimal.Decimal, size decimal.Decimal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if side == "buy" {
		for i := 0; i < len(s.bids); i++ {
			if s.bids[i].Price.Equal(price) {
				s.bids[i].Quantity = s.bids[i].Quantity.Sub(size)
				if s.bids[i].Quantity.LessThanOrEqual(decimal.Zero) {
					heap.Remove(&s.bids, i)
				}
				break
			}
		}
	} else {
		for i := 0; i < len(s.asks); i++ {
			if s.asks[i].Price.Equal(price) {
				s.asks[i].Quantity = s.asks[i].Quantity.Sub(size)
				if s.asks[i].Quantity.LessThanOrEqual(decimal.Zero) {
					heap.Remove(&s.asks, i)
				}
				break
			}
		}
	}
}

type PriceHeap []*PriceLevel

func (h PriceHeap) Len() int { return len(h) }

func (h PriceHeap) Less(i, j int) bool {
	return h[i].Price.GreaterThan(h[j].Price)
}

func (h PriceHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *PriceHeap) Push(x any) {
	*h = append(*h, x.(*PriceLevel))
}

func (h *PriceHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return item
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
