package strategies

import (
	"context"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type TWAPStrategy struct {
	BaseStrategy
	mu             sync.RWMutex
	targetQuantity decimal.Decimal
	executedQty    decimal.Decimal
	remainingQty   decimal.Decimal
	startPrice     decimal.Decimal
	endPrice       decimal.Decimal
	slices         int
	sliceInterval  time.Duration
	currentSlice   int
	startTime      time.Time
	endTime        time.Time
	isComplete     bool
}

func NewTWAPStrategy(symbol string, params map[string]interface{}) *TWAPStrategy {
	s := &TWAPStrategy{
		BaseStrategy: *NewBaseStrategy("twap", symbol),
	}

	if v, ok := params["quantity"]; ok {
		s.targetQuantity = decimal.NewFromFloat(v.(float64))
	}
	if v, ok := params["slices"]; ok {
		s.slices = int(v.(float64))
	} else {
		s.slices = 10
	}
	if v, ok := params["duration"]; ok {
		s.sliceInterval = time.Duration(v.(float64)) * time.Second / time.Duration(s.slices)
	} else {
		s.sliceInterval = 6 * time.Second
	}

	s.remainingQty = s.targetQuantity
	s.sliceSize = s.targetQuantity.Div(decimal.NewFromInt(int64(s.slices)))

	return s
}

func (s *TWAPStrategy) OnStart(ctx context.Context) error {
	s.mu.Lock()
	s.startTime = time.Now()
	s.endTime = s.startTime.Add(time.Duration(s.slices) * s.sliceInterval)
	s.mu.Unlock()

	logger.Info("TWAP strategy started",
		"symbol", s.symbol,
		"target_qty", s.targetQuantity,
		"slices", s.slices,
		"slice_size", s.sliceSize,
	)

	return nil
}

func (s *TWAPStrategy) OnTick(ctx context.Context, tick *types.Tick) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isComplete {
		return nil
	}

	if s.executedQty.GreaterThanOrEqual(s.targetQuantity) {
		s.isComplete = true
		logger.Info("TWAP strategy completed - all quantity executed")
		return nil
	}

	now := time.Now()
	elapsed := now.Sub(s.startTime)
	expectedSlice := int(elapsed / s.sliceInterval)

	if expectedSlice > s.currentSlice && s.executedQty.LessThan(s.targetQuantity) {
		orderQty := s.sliceSize
		if s.executedQty.Add(orderQty).GreaterThan(s.targetQuantity) {
			orderQty = s.targetQuantity.Sub(s.executedQty)
		}

		order := &types.Order{
			Symbol:     s.symbol,
			Side:       types.OrderSideBuy,
			Type:       types.OrderTypeLimit,
			Quantity:   orderQty,
			Price:      tick.AskPrice,
			TimeInForce: types.TimeInForceIOC,
		}

		if err := s.engine.SubmitOrder(order); err != nil {
			logger.Error("TWAP order failed", "error", err)
			return err
		}

		s.executedQty = s.executedQty.Add(orderQty)
		s.remainingQty = s.targetQuantity.Sub(s.executedQty)
		s.currentSlice = expectedSlice

		logger.Info("TWAP slice executed",
			"slice", s.currentSlice,
			"quantity", orderQty,
			"executed_total", s.executedQty,
			"remaining", s.remainingQty,
		)
	}

	return nil
}

func (s *TWAPStrategy) OnOrderFilled(ctx context.Context, order *types.Order) error {
	logger.Info("TWAP order filled",
		"order_id", order.ID,
		"quantity", order.FilledQuantity,
		"price", order.AvgFillPrice,
	)
	return nil
}

func (s *TWAPStrategy) OnStop(ctx context.Context) error {
	s.mu.Lock()
	if !s.isComplete {
		logger.Warn("TWAP strategy stopped early",
			"executed", s.executedQty,
			"target", s.targetQuantity,
			"remaining", s.remainingQty,
		)
	}
	s.mu.Unlock()

	return nil
}

func (s *TWAPStrategy) GetProgress() (executed, remaining, target decimal.Decimal, complete bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.executedQty, s.remainingQty, s.targetQuantity, s.isComplete
}

type VWAPStrategy struct {
	BaseStrategy
	mu             sync.RWMutex
	targetQuantity decimal.Decimal
	executedQty    decimal.Decimal
	maxPrice       decimal.Decimal
	minPrice       decimal.Decimal
	volatility      decimal.Decimal
	slices         int
	weights        []decimal.Decimal
	currentSlice   int
	isComplete     bool
}

func NewVWAPStrategy(symbol string, params map[string]interface{}) *VWAPStrategy {
	s := &VWAPStrategy{
		BaseStrategy: *NewBaseStrategy("vwap", symbol),
	}

	if v, ok := params["quantity"]; ok {
		s.targetQuantity = decimal.NewFromFloat(v.(float64))
	}
	if v, ok := params["slices"]; ok {
		s.slices = int(v.(float64))
	} else {
		s.slices = 20
	}
	if v, ok := params["max_price"]; ok {
		s.maxPrice = decimal.NewFromFloat(v.(float64))
	}
	if v, ok := params["min_price"]; ok {
		s.minPrice = decimal.NewFromFloat(v.(float64))
	}

	s.weights = make([]decimal.Decimal, s.slices)
	for i := 0; i < s.slices; i++ {
		s.weights[i] = decimal.NewFromInt(1)
	}

	return s
}

func (s *VWAPStrategy) OnStart(ctx context.Context) error {
	logger.Info("VWAP strategy started",
		"symbol", s.symbol,
		"target_qty", s.targetQuantity,
		"slices", s.slices,
		"price_range", map[string]string{
			"min": s.minPrice.String(),
			"max": s.maxPrice.String(),
		},
	)
	return nil
}

func (s *VWAPStrategy) OnCandle(ctx context.Context, candle *types.Candle) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isComplete {
		return nil
	}

	if s.currentSlice >= s.slices {
		s.isComplete = true
		return nil
	}

	sliceQty := s.targetQuantity.Div(decimal.NewFromInt(int64(s.slices)))
	weight := s.weights[s.currentSlice]
	adjustedQty := sliceQty.Mul(weight)

	order := &types.Order{
		Symbol:     s.symbol,
		Side:       types.OrderSideBuy,
		Type:       types.OrderTypeLimit,
		Quantity:   adjustedQty,
		Price:      candle.Close,
		TimeInForce: types.TimeInForceIOC,
	}

	if err := s.engine.SubmitOrder(order); err != nil {
		return err
	}

	s.executedQty = s.executedQty.Add(adjustedQty)
	s.currentSlice++

	return nil
}

func (s *VWAPStrategy) UpdateWeights(volumes []decimal.Decimal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	totalVol := decimal.Zero
	for _, v := range volumes {
		totalVol = totalVol.Add(v)
	}

	for i, v := range volumes {
		if i < len(s.weights) {
			s.weights[i] = v.Div(totalVol)
		}
	}
}

type BracketOrder struct {
	BaseStrategy
	mu           sync.RWMutex
	parentOrder  *types.Order
	entryOrder   *types.Order
	stopLoss     *types.Order
	takeProfit   *types.Order
	targetProfit decimal.Decimal
	stopLossPct  decimal.Decimal
	takeProfitPct decimal.Decimal
}

func NewBracketStrategy(symbol string, params map[string]interface{}) *BracketOrder {
	s := &BracketOrder{
		BaseStrategy: *NewBaseStrategy("bracket", symbol),
	}

	if v, ok := params["entry_price"]; ok {
		entryPrice := decimal.NewFromFloat(v.(float64))
		s.entryOrder = &types.Order{
			Symbol:     symbol,
			Side:       types.OrderSideBuy,
			Type:       types.OrderTypeLimit,
			Quantity:   decimal.NewFromFloat(params["quantity"].(float64)),
			Price:      entryPrice,
			TimeInForce: types.TimeInForceGTC,
		}
	}

	if v, ok := params["stop_loss_pct"]; ok {
		s.stopLossPct = decimal.NewFromFloat(v.(float64))
	}
	if v, ok := params["take_profit_pct"]; ok {
		s.takeProfitPct = decimal.NewFromFloat(v.(float64))
	}

	return s
}

func (b *BracketOrder) OnStart(ctx context.Context) error {
	if b.entryOrder == nil {
		return nil
	}

	if err := b.engine.SubmitOrder(b.entryOrder); err != nil {
		return err
	}

	logger.Info("Bracket entry order placed",
		"symbol", b.symbol,
		"entry_price", b.entryOrder.Price,
	)

	return nil
}

func (b *BracketOrder) OnOrderFilled(ctx context.Context, order *types.Order) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if order.ID == b.entryOrder.ID {
		b.createBracketOrders(order)
	}
	return nil
}

func (b *BracketOrder) createBracketOrders(filledOrder *types.Order) {
	stopPrice := filledOrder.AvgFillPrice.Mul(decimal.NewFromInt(1).Sub(b.stopLossPct))
	b.stopLoss = &types.Order{
		Symbol:     b.symbol,
		Side:       types.OrderSideSell,
		Type:       types.OrderTypeStop,
		Quantity:   filledOrder.FilledQuantity,
		StopPrice:  stopPrice,
		TimeInForce: types.TimeInForceGTC,
	}

	tpPrice := filledOrder.AvgFillPrice.Mul(decimal.NewFromInt(1).Add(b.takeProfitPct))
	b.takeProfit = &types.Order{
		Symbol:     b.symbol,
		Side:       types.OrderSideSell,
		Type:       types.OrderTypeLimit,
		Quantity:   filledOrder.FilledQuantity,
		Price:      tpPrice,
		TimeInForce: types.TimeInForceGTC,
	}

	b.engine.SubmitOrder(b.stopLoss)
	b.engine.SubmitOrder(b.takeProfit)

	logger.Info("Bracket orders created",
		"stop_loss", stopPrice,
		"take_profit", tpPrice,
	)
}

func (b *BracketOrder) OnStop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopLoss != nil && b.stopLoss.Status == types.OrderStatusOpen {
		b.engine.CancelOrder(b.stopLoss.ID.String())
	}
	if b.takeProfit != nil && b.takeProfit.Status == types.OrderStatusOpen {
		b.engine.CancelOrder(b.takeProfit.ID.String())
	}

	return nil
}

type TrailingStopStrategy struct {
	BaseStrategy
	mu            sync.RWMutex
	position      *types.Position
	trailingPct   decimal.Decimal
	highestPrice  decimal.Decimal
	stopDistance  decimal.Decimal
	isActive      bool
}

func NewTrailingStopStrategy(symbol string, params map[string]interface{}) *TrailingStopStrategy {
	s := &TrailingStopStrategy{
		BaseStrategy: *NewBaseStrategy("trailing_stop", symbol),
	}

	if v, ok := params["trailing_pct"]; ok {
		s.trailingPct = decimal.NewFromFloat(v.(float64))
	} else {
		s.trailingPct = decimal.NewFromFloat(0.02)
	}

	return s
}

func (t *TrailingStopStrategy) OnStart(ctx context.Context) error {
	positions := t.engine.GetPositions()
	for _, pos := range positions {
		if pos.Symbol == t.symbol && pos.Quantity.GreaterThan(decimal.Zero) {
			t.mu.Lock()
			t.position = pos
			t.highestPrice = pos.CurrentPrice
			t.stopDistance = t.highestPrice.Mul(t.trailingPct)
			t.isActive = true
			t.mu.Unlock()

			logger.Info("Trailing stop activated",
				"symbol", t.symbol,
				"entry_price", pos.EntryPrice,
				"current_price", pos.CurrentPrice,
				"trailing_pct", t.trailingPct,
			)
			break
		}
	}
	return nil
}

func (t *TrailingStopStrategy) OnTick(ctx context.Context, tick *types.Tick) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isActive || t.position == nil {
		return nil
	}

	if tick.BidPrice.GreaterThan(t.highestPrice) {
		t.highestPrice = tick.BidPrice
		t.stopDistance = t.highestPrice.Mul(t.trailingPct)
		logger.Debug("Trailing stop updated",
			"new_high", t.highestPrice,
			"stop_distance", t.stopDistance,
		)
	}

	stopTriggerPrice := t.highestPrice.Sub(t.stopDistance)
	if tick.BidPrice.LessThanOrEqual(stopTriggerPrice) {
		logger.Info("Trailing stop triggered",
			"symbol", t.symbol,
			"exit_price", tick.BidPrice,
			"highest_price", t.highestPrice,
		)

		order := &types.Order{
			Symbol:     t.symbol,
			Side:       types.OrderSideSell,
			Type:       types.OrderTypeMarket,
			Quantity:   t.position.Quantity,
		}

		t.engine.SubmitOrder(order)
		t.isActive = false
	}

	return nil
}
