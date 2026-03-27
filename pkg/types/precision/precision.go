package precision

import (
	"time"

	"github.com/shopspring/decimal"
)

type UnixNano int64

func NewUnixNano(t time.Time) UnixNano {
	return UnixNano(t.UnixNano())
}

func Now() UnixNano {
	return UnixNano(time.Now().UnixNano())
}

func FromUnix(t int64) UnixNano {
	return UnixNano(t * 1e9)
}

func (u UnixNano) Time() time.Time {
	return time.Unix(0, int64(u))
}

func (u UnixNano) Unix() int64 {
	return int64(u) / 1e9
}

func (u UnixNano) Nano() int64 {
	return int64(u)
}

func (u UnixNano) MarshalJSON() ([]byte, error) {
	return []byte(`"` + u.Time().Format(time.RFC3339Nano) + `"`), nil
}

func (u *UnixNano) UnmarshalJSON(data []byte) error {
	t, err := time.Parse(`"`+time.RFC3339Nano+`"`, string(data))
	if err != nil {
		return err
	}
	*u = NewUnixNano(t)
	return nil
}

type Price struct {
	value int64
	scale int32
}

const (
	PriceScale0   = 0
	PriceScale1   = 1
	PriceScale2   = 2
	PriceScale3   = 3
	PriceScale4   = 4
	PriceScale5   = 5
	PriceScale6   = 6
	PriceScale7   = 7
	PriceScale8   = 8
	PriceScale9   = 9
	PriceScaleDec = 10
)

func NewPrice(value int64, scale int32) Price {
	return Price{value: value, scale: scale}
}

func PriceFromFloat(f float64, scale int32) Price {
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(scale)))
	d := decimal.NewFromFloat(f).Mul(multiplier)
	val := d.CoefficientInt64()
	return Price{value: val, scale: scale}
}

func PriceFromDecimal(d decimal.Decimal, scale int32) Price {
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(scale)))
	rounded := d.Mul(multiplier)
	val := rounded.CoefficientInt64()
	return Price{value: val, scale: scale}
}

func (p Price) Decimal() decimal.Decimal {
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(p.scale)))
	return decimal.NewFromInt(p.value).Div(multiplier)
}

func (p Price) Float64() float64 {
	return p.Decimal().InexactFloat64()
}

func (p Price) Int64() int64 {
	return p.value
}

func (p Price) Scale() int32 {
	return p.scale
}

func (p Price) Add(other Price) Price {
	if p.scale > other.scale {
		other = other.Rescale(p.scale)
	} else if other.scale > p.scale {
		p = p.Rescale(other.scale)
	}
	return Price{value: p.value + other.value, scale: p.scale}
}

func (p Price) Sub(other Price) Price {
	if p.scale > other.scale {
		other = other.Rescale(p.scale)
	} else if other.scale > p.scale {
		p = p.Rescale(other.scale)
	}
	return Price{value: p.value - other.value, scale: p.scale}
}

func (p Price) Mul(factor int64) Price {
	return Price{value: p.value * factor, scale: p.scale}
}

func (p Price) Div(divisor int64) Price {
	if divisor == 0 {
		return Price{}
	}
	return Price{value: p.value / divisor, scale: p.scale}
}

func (p Price) Rescale(newScale int32) Price {
	if p.scale == newScale {
		return p
	}
	if newScale > p.scale {
		diff := newScale - p.scale
		return Price{value: p.value * pow10(diff), scale: newScale}
	}
	diff := p.scale - newScale
	return Price{value: p.value / pow10(diff), scale: newScale}
}

func (p Price) Round(scale int32) Price {
	if scale >= p.scale {
		return p
	}
	diff := p.scale - scale
	return Price{value: (p.value + pow10(diff)/2) / pow10(diff), scale: scale}
}

func (p Price) IsZero() bool {
	return p.value == 0
}

func (p Price) IsPositive() bool {
	return p.value > 0
}

func (p Price) IsNegative() bool {
	return p.value < 0
}

func (p Price) GreaterThan(other Price) bool {
	if p.scale > other.scale {
		other = other.Rescale(p.scale)
	} else if other.scale > p.scale {
		p = p.Rescale(other.scale)
	}
	return p.value > other.value
}

func (p Price) LessThan(other Price) bool {
	if p.scale > other.scale {
		other = other.Rescale(p.scale)
	} else if other.scale > p.scale {
		p = p.Rescale(other.scale)
	}
	return p.value < other.value
}

func (p Price) Equal(other Price) bool {
	if p.scale > other.scale {
		other = other.Rescale(p.scale)
	} else if other.scale > p.scale {
		p = p.Rescale(other.scale)
	}
	return p.value == other.value
}

func (p Price) GreaterOrEqual(other Price) bool {
	return p.GreaterThan(other) || p.Equal(other)
}

func (p Price) LessOrEqual(other Price) bool {
	return p.LessThan(other) || p.Equal(other)
}

func (p Price) String() string {
	return p.Decimal().String()
}

func (p Price) FormatPrecise() string {
	d := p.Decimal()
	return d.String()
}

func pow10(n int32) int64 {
	if n < 0 {
		return 0
	}
	result := int64(1)
	for i := int32(0); i < n; i++ {
		result *= 10
	}
	return result
}

type Quantity struct {
	value int64
	scale int32
}

func NewQuantity(value int64, scale int32) Quantity {
	return Quantity{value: value, scale: scale}
}

func QuantityFromFloat(f float64, scale int32) Quantity {
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(scale)))
	d := decimal.NewFromFloat(f).Mul(multiplier)
	val := d.CoefficientInt64()
	return Quantity{value: val, scale: scale}
}

func QuantityFromDecimal(d decimal.Decimal, scale int32) Quantity {
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(scale)))
	rounded := d.Mul(multiplier)
	val := rounded.CoefficientInt64()
	return Quantity{value: val, scale: scale}
}

func (q Quantity) Decimal() decimal.Decimal {
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(q.scale)))
	return decimal.NewFromInt(q.value).Div(multiplier)
}

func (q Quantity) Float64() float64 {
	return q.Decimal().InexactFloat64()
}

func (q Quantity) Int64() int64 {
	return q.value
}

func (q Quantity) Scale() int32 {
	return q.scale
}

func (q Quantity) Add(other Quantity) Quantity {
	if q.scale > other.scale {
		other = other.Rescale(q.scale)
	} else if other.scale > q.scale {
		q = q.Rescale(other.scale)
	}
	return Quantity{value: q.value + other.value, scale: q.scale}
}

func (q Quantity) Sub(other Quantity) Quantity {
	if q.scale > other.scale {
		other = other.Rescale(q.scale)
	} else if other.scale > q.scale {
		q = q.Rescale(other.scale)
	}
	return Quantity{value: q.value - other.value, scale: q.scale}
}

func (q Quantity) Mul(factor int64) Quantity {
	return Quantity{value: q.value * factor, scale: q.scale}
}

func (q Quantity) Div(divisor int64) Quantity {
	if divisor == 0 {
		return Quantity{}
	}
	return Quantity{value: q.value / divisor, scale: q.scale}
}

func (q Quantity) Rescale(newScale int32) Quantity {
	if q.scale == newScale {
		return q
	}
	if newScale > q.scale {
		diff := newScale - q.scale
		return Quantity{value: q.value * pow10(diff), scale: newScale}
	}
	diff := q.scale - newScale
	return Quantity{value: q.value / pow10(diff), scale: newScale}
}

func (q Quantity) Round(scale int32) Quantity {
	if scale >= q.scale {
		return q
	}
	diff := q.scale - scale
	return Quantity{value: (q.value + pow10(diff)/2) / pow10(diff), scale: scale}
}

func (q Quantity) IsZero() bool {
	return q.value == 0
}

func (q Quantity) IsPositive() bool {
	return q.value > 0
}

func (q Quantity) IsNegative() bool {
	return q.value < 0
}

func (q Quantity) GreaterThan(other Quantity) bool {
	if q.scale > other.scale {
		other = other.Rescale(q.scale)
	} else if other.scale > q.scale {
		q = q.Rescale(other.scale)
	}
	return q.value > other.value
}

func (q Quantity) LessThan(other Quantity) bool {
	if q.scale > other.scale {
		other = other.Rescale(q.scale)
	} else if other.scale > q.scale {
		q = q.Rescale(other.scale)
	}
	return q.value < other.value
}

func (q Quantity) Equal(other Quantity) bool {
	if q.scale > other.scale {
		other = other.Rescale(q.scale)
	} else if other.scale > q.scale {
		q = q.Rescale(other.scale)
	}
	return q.value == other.value
}

func (q Quantity) String() string {
	return q.Decimal().String()
}

type Quote struct {
	Bid    Price
	Ask    Price
	BidQty Quantity
	AskQty Quantity
	Time   UnixNano
}

func NewQuote(bid, ask Price, bidQty, askQty Quantity, time UnixNano) Quote {
	return Quote{
		Bid:    bid,
		Ask:    ask,
		BidQty: bidQty,
		AskQty: askQty,
		Time:   time,
	}
}

func (q Quote) Spread() Price {
	return q.Ask.Sub(q.Bid)
}

func (q Quote) MidPrice() Price {
	return q.Bid.Add(q.Ask).Div(2)
}

func (q Quote) SpreadPct() decimal.Decimal {
	mid := q.MidPrice().Decimal()
	spread := q.Spread().Decimal()
	if mid.IsZero() {
		return decimal.Zero
	}
	return spread.Div(mid).Mul(decimal.NewFromInt(100))
}

type Trade struct {
	ID        string
	Price     Price
	Quantity  Quantity
	Side      string
	Time      UnixNano
	Exchange  string
	Symbol    string
}

func NewTrade(id string, price Price, qty Quantity, side string, time UnixNano, exchange, symbol string) Trade {
	return Trade{
		ID:       id,
		Price:    price,
		Quantity: qty,
		Side:     side,
		Time:     time,
		Exchange: exchange,
		Symbol:   symbol,
	}
}

type Bar struct {
	Symbol    string
	Exchange  string
	Open      Price
	High      Price
	Low       Price
	Close     Price
	Volume    Quantity
	QuoteVol  Quantity
	StartTime UnixNano
	EndTime   UnixNano
}

func NewBar(symbol, exchange string, open, high, low, close Price, vol, quoteVol Quantity, start, end UnixNano) Bar {
	return Bar{
		Symbol:    symbol,
		Exchange:  exchange,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     close,
		Volume:    vol,
		QuoteVol:  quoteVol,
		StartTime: start,
		EndTime:   end,
	}
}

func (b Bar) Range() Price {
	return b.High.Sub(b.Low)
}

func (b Bar) Body() Price {
	return b.Close.Sub(b.Open)
}

func (b Bar) UpperWick() Price {
	if b.Body().IsPositive() {
		return b.High.Sub(b.Close)
	}
	return b.High.Sub(b.Open)
}

func (b Bar) LowerWick() Price {
	if b.Body().IsPositive() {
		return b.Open.Sub(b.Low)
	}
	return b.Close.Sub(b.Low)
}

func (b Bar) IsBullish() bool {
	return b.Close.GreaterThan(b.Open)
}

func (b Bar) IsBearish() bool {
	return b.Close.LessThan(b.Open)
}

func (b Bar) Duration() int64 {
	return b.EndTime.Nano() - b.StartTime.Nano()
}

type OrderBookLevel struct {
	Price    Price
	Quantity Quantity
	Orders   int64
}

type OrderBook struct {
	Symbol   string
	Exchange string
	Bids     []OrderBookLevel
	Asks     []OrderBookLevel
	Time     UnixNano
}

func NewOrderBook(symbol, exchange string, bids, asks []OrderBookLevel, time UnixNano) OrderBook {
	return OrderBook{
		Symbol:   symbol,
		Exchange: exchange,
		Bids:     bids,
		Asks:     asks,
		Time:     time,
	}
}

func (ob OrderBook) BestBid() Price {
	if len(ob.Bids) == 0 {
		return Price{}
	}
	return ob.Bids[0].Price
}

func (ob OrderBook) BestAsk() Price {
	if len(ob.Asks) == 0 {
		return Price{}
	}
	return ob.Asks[0].Price
}

func (ob OrderBook) Spread() Price {
	return ob.BestAsk().Sub(ob.BestBid())
}

func (ob OrderBook) MidPrice() Price {
	return ob.BestBid().Add(ob.BestAsk()).Div(2)
}

func (ob OrderBook) Imbalance() decimal.Decimal {
	var bidVol, askVol decimal.Decimal
	for _, bid := range ob.Bids {
		bidVol = bidVol.Add(bid.Quantity.Decimal())
	}
	for _, ask := range ob.Asks {
		askVol = askVol.Add(ask.Quantity.Decimal())
	}
	total := bidVol.Add(askVol)
	if total.IsZero() {
		return decimal.Zero
	}
	return bidVol.Sub(askVol).Div(total)
}

func (ob OrderBook) VWAP(side string) Price {
	if side == "buy" {
		return ob.MidPrice()
	}
	return ob.MidPrice()
}

type Position struct {
	Symbol       string
	Exchange     string
	Side         string
	Quantity     Quantity
	AvgEntryPx   Price
	CurrentPx    Price
	UnrealizedPnL decimal.Decimal
	RealizedPnL  decimal.Decimal
	OpenTime     UnixNano
	UpdatedTime  UnixNano
}

func (p Position) MarketValue() decimal.Decimal {
	return p.Quantity.Decimal().Mul(p.CurrentPx.Decimal())
}

func (p Position) CostBasis() decimal.Decimal {
	return p.Quantity.Decimal().Mul(p.AvgEntryPx.Decimal())
}

func (p Position) UnrealizedPnLPct() decimal.Decimal {
	cost := p.CostBasis()
	if cost.IsZero() {
		return decimal.Zero
	}
	return p.UnrealizedPnL.Div(cost).Mul(decimal.NewFromInt(100))
}

type Order struct {
	ID            string
	ClientOrderID string
	Symbol        string
	Exchange      string
	Side          string
	Type          string
	Status        string
	Price         Price
	Quantity      Quantity
	FilledQty     Quantity
	AvgFillPx     Price
	CreatedTime   UnixNano
	UpdatedTime   UnixNano
	SubmittedTime UnixNano
}

func (o Order) RemainingQty() Quantity {
	return o.Quantity.Sub(o.FilledQty)
}

func (o Order) FillPct() decimal.Decimal {
	qty := o.Quantity.Decimal()
	if qty.IsZero() {
		return decimal.Zero
	}
	return o.FilledQty.Decimal().Div(qty).Mul(decimal.NewFromInt(100))
}

func (o Order) IsFilled() bool {
	return o.Status == "filled" || o.FilledQty.Equal(o.Quantity)
}

func (o Order) IsOpen() bool {
	return o.Status == "open" || o.Status == "partial"
}

type Account struct {
	NetLiquidation  decimal.Decimal
	BuyingPower     decimal.Decimal
	Equity          decimal.Decimal
	Cash            decimal.Decimal
	MarginUsed      decimal.Decimal
	AvailableFunds  decimal.Decimal
	InitMarginReq   decimal.Decimal
	MaintMarginReq  decimal.Decimal
	UpdatedTime     UnixNano
}

func (a Account) MarginUtilization() decimal.Decimal {
	if a.NetLiquidation.IsZero() {
		return decimal.Zero
	}
	return a.MarginUsed.Div(a.NetLiquidation).Mul(decimal.NewFromInt(100))
}

func (a Account) Leverage() decimal.Decimal {
	if a.Equity.IsZero() {
		return decimal.Zero
	}
	return a.NetLiquidation.Div(a.Equity)
}
