package ta

import (
	"math"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Indicators struct{}

func New() *Indicators {
	return &Indicators{}
}

func (i *Indicators) SMA(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period {
		return decimal.Zero
	}

	sum := decimal.Zero
	for j := len(candles) - period; j < len(candles); j++ {
		sum = sum.Add(candles[j].Close)
	}

	return sum.Div(decimal.NewFromInt(int64(period)))
}

func (i *Indicators) EMA(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period {
		return decimal.Zero
	}

	multiplier := decimal.NewFromFloat(2.0).Div(decimal.NewFromInt(int64(period + 1)))
	ema := i.SMA(candles[:period], period)

	for j := period; j < len(candles); j++ {
		ema = candles[j].Close.Sub(ema).Mul(multiplier).Add(ema)
	}

	return ema
}

func (i *Indicators) RSI(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period+1 {
		return decimal.NewFromFloat(50)
	}

	var gains, losses decimal.Decimal

	for j := len(candles) - period; j < len(candles); j++ {
		change := candles[j].Close.Sub(candles[j-1].Close)
		if change.GreaterThan(decimal.Zero) {
			gains = gains.Add(change)
		} else {
			losses = losses.Add(change.Abs())
		}
	}

	avgGain := gains.Div(decimal.NewFromInt(int64(period)))
	avgLoss := losses.Div(decimal.NewFromInt(int64(period)))

	if avgLoss.IsZero() {
		return decimal.NewFromInt(100)
	}

	rs := avgGain.Div(avgLoss)
	rsi := decimal.NewFromInt(100).Sub(decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(rs)))

	return rsi
}

func (i *Indicators) MACD(candles []*types.Candle, fast, slow, signal int) (macd, signalLine, histogram decimal.Decimal) {
	if len(candles) < slow {
		return decimal.Zero, decimal.Zero, decimal.Zero
	}

	emaFast := i.EMA(candles, fast)
	emaSlow := i.EMA(candles, slow)
	macd = emaFast.Sub(emaSlow)

	emaPeriod := signal
	if len(candles) < slow+signal {
		emaPeriod = len(candles) - slow
	}

	macdCandles := make([]*types.Candle, len(candles))
	for j := range candles {
		macdCandles[j] = &types.Candle{Close: macd}
	}

	signalLine = i.EMA(macdCandles, emaPeriod)
	histogram = macd.Sub(signalLine)

	return macd, signalLine, histogram
}

func (i *Indicators) BollingerBands(candles []*types.Candle, period int, stdDevMultiplier float64) (upper, middle, lower decimal.Decimal) {
	if len(candles) < period {
		return decimal.Zero, decimal.Zero, decimal.Zero
	}

	middle = i.SMA(candles, period)

	sumSquares := decimal.Zero
	for j := len(candles) - period; j < len(candles); j++ {
		diff := candles[j].Close.Sub(middle)
		sumSquares = sumSquares.Add(diff.Mul(diff))
	}

	variance := sumSquares.Div(decimal.NewFromInt(int64(period)))
	stdDev := sqrtDecimal(variance)

	upper = middle.Add(stdDev.Mul(decimal.NewFromFloat(stdDevMultiplier)))
	lower = middle.Sub(stdDev.Mul(decimal.NewFromFloat(stdDevMultiplier)))

	return upper, middle, lower
}

func (i *Indicators) ATR(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period+1 {
		return decimal.Zero
	}

	trueRanges := make([]decimal.Decimal, len(candles)-1)

	for j := 1; j < len(candles); j++ {
		high := candles[j].High
		low := candles[j].Low
		prevClose := candles[j-1].Close

		tr1 := high.Sub(low)
		tr2 := high.Sub(prevClose).Abs()
		tr3 := low.Sub(prevClose).Abs()

		trueRanges[j-1] = maxDecimal(maxDecimal(tr1, tr2), tr3)
	}

	sum := decimal.Zero
	for j := 0; j < period && j < len(trueRanges); j++ {
		sum = sum.Add(trueRanges[j])
	}

	if len(trueRanges) < period {
		return sum
	}

	atr := sum.Div(decimal.NewFromInt(int64(period)))

	for j := period; j < len(trueRanges); j++ {
		atr = atr.Mul(decimal.NewFromInt(int64(period-1))).
			Add(trueRanges[j]).
			Div(decimal.NewFromInt(int64(period)))
	}

	return atr
}

func (i *Indicators) Stochastic(candles []*types.Candle, kPeriod, dPeriod int) (k, d decimal.Decimal) {
	if len(candles) < kPeriod {
		return decimal.NewFromFloat(50), decimal.NewFromFloat(50)
	}

	var kValues []decimal.Decimal

	for j := kPeriod - 1; j < len(candles); j++ {
		highestHigh := candles[j].High
		lowestLow := candles[j].Low

		for l := j - kPeriod + 1; l <= j; l++ {
			if candles[l].High.GreaterThan(highestHigh) {
				highestHigh = candles[l].High
			}
			if candles[l].Low.LessThan(lowestLow) {
				lowestLow = candles[l].Low
			}
		}

		range_ := highestHigh.Sub(lowestLow)
		if range_.IsZero() {
			kValues = append(kValues, decimal.NewFromFloat(50))
		} else {
			k := candles[j].Close.Sub(lowestLow).Div(range_).Mul(decimal.NewFromInt(100))
			kValues = append(kValues, k)
		}
	}

	if len(kValues) == 0 {
		return decimal.NewFromFloat(50), decimal.NewFromFloat(50)
	}

	k = kValues[len(kValues)-1]

	kCandles := make([]*types.Candle, len(kValues))
	for j := range kValues {
		kCandles[j] = &types.Candle{Close: kValues[j]}
	}

	d = i.SMA(kCandles, dPeriod)

	return k, d
}

func (i *Indicators) ADX(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period*2+1 {
		return decimal.Zero
	}

	plusDM := make([]decimal.Decimal, len(candles)-1)
	minusDM := make([]decimal.Decimal, len(candles)-1)
	trueRanges := make([]decimal.Decimal, len(candles)-1)

	for j := 1; j < len(candles); j++ {
		high := candles[j].High
		low := candles[j].Low
		prevHigh := candles[j-1].High
		prevLow := candles[j-1].Low
		prevClose := candles[j-1].Close

		upMove := high.Sub(prevHigh)
		downMove := prevLow.Sub(low)

		if upMove.GreaterThan(downMove) && upMove.GreaterThan(decimal.Zero) {
			plusDM[j-1] = upMove
		} else {
			plusDM[j-1] = decimal.Zero
		}

		if downMove.GreaterThan(upMove) && downMove.GreaterThan(decimal.Zero) {
			minusDM[j-1] = downMove
		} else {
			minusDM[j-1] = decimal.Zero
		}

		tr1 := high.Sub(low)
		tr2 := high.Sub(prevClose).Abs()
		tr3 := low.Sub(prevClose).Abs()
		trueRanges[j-1] = maxDecimal(maxDecimal(tr1, tr2), tr3)
	}

	smoothedPlusDM := i.smoothSeries(plusDM, period)
	smoothedMinusDM := i.smoothSeries(minusDM, period)
	smoothedTR := i.smoothSeries(trueRanges, period)

	if smoothedTR.IsZero() {
		return decimal.Zero
	}

	plusDI := smoothedPlusDM.Div(smoothedTR).Mul(decimal.NewFromInt(100))
	minusDI := smoothedMinusDM.Div(smoothedTR).Mul(decimal.NewFromInt(100))

	diSum := plusDI.Add(minusDI)
	if diSum.IsZero() {
		return decimal.Zero
	}

	dx := plusDI.Sub(minusDI).Abs().Div(diSum).Mul(decimal.NewFromInt(100))

	return dx
}

func (i *Indicators) CCI(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period {
		return decimal.Zero
	}

	var typicalPrices []decimal.Decimal
	for _, c := range candles {
		tp := c.High.Add(c.Low).Add(c.Close).Div(decimal.NewFromInt(3))
		typicalPrices = append(typicalPrices, tp)
	}

	periodPrices := typicalPrices[len(typicalPrices)-period:]
	smma := i.SMAFromValues(periodPrices, period)

	var meanDeviation decimal.Decimal
	for _, tp := range periodPrices {
		meanDeviation = meanDeviation.Add(tp.Sub(smma).Abs())
	}
	meanDeviation = meanDeviation.Div(decimal.NewFromInt(int64(period)))

	if meanDeviation.IsZero() {
		return decimal.Zero
	}

	currentTP := typicalPrices[len(typicalPrices)-1]
	cci := currentTP.Sub(smma).Div(meanDeviation.Mul(decimal.NewFromFloat(0.015)))

	return cci
}

func (i *Indicators) OBV(candles []*types.Candle) decimal.Decimal {
	if len(candles) == 0 {
		return decimal.Zero
	}

	obv := decimal.Zero

	for j := 1; j < len(candles); j++ {
		if candles[j].Close.GreaterThan(candles[j-1].Close) {
			obv = obv.Add(candles[j].Volume)
		} else if candles[j].Close.LessThan(candles[j-1].Close) {
			obv = obv.Sub(candles[j].Volume)
		}
	}

	return obv
}

func (i *Indicators) VWAP(candles []*types.Candle) decimal.Decimal {
	if len(candles) == 0 {
		return decimal.Zero
	}

	var cumulativeTPV, cumulativeVolume decimal.Decimal

	for _, c := range candles {
		tpv := c.High.Add(c.Low).Add(c.Close).Div(decimal.NewFromInt(3)).Mul(c.Volume)
		cumulativeTPV = cumulativeTPV.Add(tpv)
		cumulativeVolume = cumulativeVolume.Add(c.Volume)
	}

	if cumulativeVolume.IsZero() {
		return decimal.Zero
	}

	return cumulativeTPV.Div(cumulativeVolume)
}

func (i *Indicators) WilliamR(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period {
		return decimal.NewFromFloat(-50)
	}

	highestHigh := candles[len(candles)-1].High
	lowestLow := candles[len(candles)-1].Low

	for j := len(candles) - period; j < len(candles); j++ {
		if candles[j].High.GreaterThan(highestHigh) {
			highestHigh = candles[j].High
		}
		if candles[j].Low.LessThan(lowestLow) {
			lowestLow = candles[j].Low
		}
	}

	range_ := highestHigh.Sub(lowestLow)
	if range_.IsZero() {
		return decimal.NewFromFloat(-50)
	}

	willR := highestHigh.Sub(candles[len(candles)-1].Close).
		Div(range_).
		Mul(decimal.NewFromInt(100)).
		Neg().
		Add(decimal.NewFromInt(100))

	return willR
}

func (i *Indicators) MFI(candles []*types.Candle, period int) decimal.Decimal {
	if len(candles) < period+1 {
		return decimal.NewFromFloat(50)
	}

	var positiveFlow, negativeFlow decimal.Decimal

	for j := len(candles) - period; j < len(candles); j++ {
		typicalPrice := candles[j].High.Add(candles[j].Low).Add(candles[j].Close).
			Div(decimal.NewFromInt(3))
		prevTypicalPrice := candles[j-1].High.Add(candles[j-1].Low).Add(candles[j-1].Close).
			Div(decimal.NewFromInt(3))

		rawMoneyFlow := typicalPrice.Mul(candles[j].Volume)

		if typicalPrice.GreaterThan(prevTypicalPrice) {
			positiveFlow = positiveFlow.Add(rawMoneyFlow)
		} else {
			negativeFlow = negativeFlow.Add(rawMoneyFlow)
		}
	}

	if negativeFlow.IsZero() {
		return decimal.NewFromInt(100)
	}

	moneyRatio := positiveFlow.Div(negativeFlow)
	mfi := decimal.NewFromInt(100).Sub(decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(moneyRatio)))

	return mfi
}

func (i *Indicators) ParabolicSAR(candles []*types.Candle, acceleration, maximum float64) []decimal.Decimal {
	if len(candles) < 2 {
		return nil
	}

	sar := make([]decimal.Decimal, len(candles))

	trend := 1
	af := decimal.NewFromFloat(acceleration)
	maxAF := decimal.NewFromFloat(maximum)

	ep := candles[0].High
	sar[0] = candles[0].Low

	for j := 1; j < len(candles); j++ {
		prevSar := sar[j-1]
		prevEp := ep

		sar[j] = prevSar.Add(af.Mul(prevEp.Sub(prevSar)))

		if trend == 1 {
			if candles[j].Low.LessThan(sar[j]) {
				trend = -1
				sar[j] = ep
				ep = candles[j].Low
				af = decimal.NewFromFloat(acceleration)
			} else {
				if candles[j].High.GreaterThan(ep) {
					ep = candles[j].High
					af = minDecimal(af.Add(decimal.NewFromFloat(acceleration)), maxAF)
				}
			}
		} else {
			if candles[j].High.GreaterThan(sar[j]) {
				trend = 1
				sar[j] = ep
				ep = candles[j].High
				af = decimal.NewFromFloat(acceleration)
			} else {
				if candles[j].Low.LessThan(ep) {
					ep = candles[j].Low
					af = minDecimal(af.Add(decimal.NewFromFloat(acceleration)), maxAF)
				}
			}
		}
	}

	return sar
}

func (i *Indicators) KeltnerChannels(candles []*types.Candle, emaPeriod, atrPeriod int, multiplier float64) (upper, middle, lower decimal.Decimal) {
	middle = i.EMA(candles, emaPeriod)
	atr := i.ATR(candles, atrPeriod)
	mult := decimal.NewFromFloat(multiplier)

	upper = middle.Add(atr.Mul(mult))
	lower = middle.Sub(atr.Mul(mult))

	return upper, middle, lower
}

func (i *Indicators) DonchianChannel(candles []*types.Candle, period int) (upper, middle, lower decimal.Decimal) {
	if len(candles) < period {
		return decimal.Zero, decimal.Zero, decimal.Zero
	}

	upper = candles[len(candles)-1].High
	lower = candles[len(candles)-1].Low

	for j := len(candles) - period; j < len(candles); j++ {
		if candles[j].High.GreaterThan(upper) {
			upper = candles[j].High
		}
		if candles[j].Low.LessThan(lower) {
			lower = candles[j].Low
		}
	}

	middle = upper.Add(lower).Div(decimal.NewFromInt(2))

	return upper, middle, lower
}

func (i *Indicators) Aroon(candles []*types.Candle, period int) (aroonUp, aroonDown decimal.Decimal) {
	if len(candles) < period {
		return decimal.NewFromFloat(50), decimal.NewFromFloat(50)
	}

	highestIdx := -1
	highestHigh := candles[len(candles)-1].High

	lowestIdx := -1
	lowestLow := candles[len(candles)-1].Low

	for j := len(candles) - period; j < len(candles); j++ {
		if candles[j].High.GreaterThanOrEqual(highestHigh) {
			highestHigh = candles[j].High
			highestIdx = j
		}
		if candles[j].Low.LessThanOrEqual(lowestLow) {
			lowestLow = candles[j].Low
			lowestIdx = j
		}
	}

	aroonUp = decimal.NewFromInt(int64(period - (len(candles) - 1 - highestIdx))).
		Div(decimal.NewFromInt(int64(period))).
		Mul(decimal.NewFromInt(100))

	aroonDown = decimal.NewFromInt(int64(period - (len(candles) - 1 - lowestIdx))).
		Div(decimal.NewFromInt(int64(period))).
		Mul(decimal.NewFromInt(100))

	return aroonUp, aroonDown
}

func (i *Indicators) smoothSeries(values []decimal.Decimal, period int) decimal.Decimal {
	if len(values) < period {
		return decimal.Zero
	}

	sum := decimal.Zero
	for j := 0; j < period; j++ {
		sum = sum.Add(values[j])
	}

	return sum
}

func (i *Indicators) EMAFromValues(values []decimal.Decimal, period int) decimal.Decimal {
	if len(values) < period {
		return decimal.Zero
	}

	multiplier := decimal.NewFromFloat(2.0).Div(decimal.NewFromInt(int64(period + 1)))
	ema := decimal.Zero

	for j := 0; j < period; j++ {
		ema = ema.Add(values[j])
	}
	ema = ema.Div(decimal.NewFromInt(int64(period)))

	for j := period; j < len(values); j++ {
		ema = values[j].Sub(ema).Mul(multiplier).Add(ema)
	}

	return ema
}

func (i *Indicators) SMAFromValues(values []decimal.Decimal, period int) decimal.Decimal {
	if len(values) < period {
		return decimal.Zero
	}

	sum := decimal.Zero
	start := len(values) - period
	for j := start; j < len(values); j++ {
		sum = sum.Add(values[j])
	}

	return sum.Div(decimal.NewFromInt(int64(period)))
}

func maxDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

func minDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

func sqrtDecimal(d decimal.Decimal) decimal.Decimal {
	f, _ := d.Float64()
	return decimal.NewFromFloat(math.Sqrt(f))
}
