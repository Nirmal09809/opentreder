package indicators

import (
	"testing"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSMA(t *testing.T) {
	candles := generateTestCandles(20, 100.0)

	sma := CalculateSMA(candles, 10)
	require.NotNil(t, sma)
	assert.True(t, sma.GreaterThan(decimal.Zero))
}

func TestEMA(t *testing.T) {
	candles := generateTestCandles(20, 100.0)

	ema := CalculateEMA(candles, 10)
	require.NotNil(t, ema)
	assert.True(t, ema.GreaterThan(decimal.Zero))
}

func TestRSI(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	rsi := CalculateRSI(candles, 14)
	require.NotNil(t, rsi)

	rsiValue, _ := rsi.Float64()
	assert.GreaterOrEqual(t, rsiValue, 0.0)
	assert.LessOrEqual(t, rsiValue, 100.0)
}

func TestMACD(t *testing.T) {
	candles := generateTestCandles(50, 100.0)

	macd := CalculateMACD(candles, 12, 26, 9)
	require.NotNil(t, macd)
	assert.NotNil(t, macd.MACD)
	assert.NotNil(t, macd.Signal)
	assert.NotNil(t, macd.Histogram)
}

func TestBollingerBands(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	bb := CalculateBollingerBands(candles, 20, 2.0)
	require.NotNil(t, bb)
	assert.True(t, bb.Upper.GreaterThanOrEqual(bb.Middle))
	assert.True(t, bb.Middle.GreaterThanOrEqual(bb.Lower))
}

func TestATR(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	atr := CalculateATR(candles, 14)
	require.NotNil(t, atr)
	assert.True(t, atr.GreaterThan(decimal.Zero))
}

func TestStochastic(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	stoch := CalculateStochastic(candles, 14, 3)
	require.NotNil(t, stoch)
	assert.True(t, stoch.K.GreaterThanOrEqual(decimal.Zero))
	assert.True(t, stoch.K.LessThanOrEqual(decimal.NewFromInt(100)))
	assert.True(t, stoch.D.GreaterThanOrEqual(decimal.Zero))
	assert.True(t, stoch.D.LessThanOrEqual(decimal.NewFromInt(100)))
}

func TestADX(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	adx := CalculateADX(candles, 14)
	require.NotNil(t, adx)
	assert.True(t, adx.GreaterThanOrEqual(decimal.Zero))
	assert.True(t, adx.LessThanOrEqual(decimal.NewFromInt(100)))
}

func TestOBV(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	obv := CalculateOBV(candles)
	require.NotNil(t, obv)
}

func TestCCI(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	cci := CalculateCCI(candles, 20)
	require.NotNil(t, cci)
}

func TestVWAP(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	vwap := CalculateVWAP(candles)
	require.NotNil(t, vwap)
	assert.True(t, vwap.GreaterThan(decimal.Zero))
}

func TestIchimoku(t *testing.T) {
	candles := generateTestCandles(60, 100.0)

	ichimoku := CalculateIchimoku(candles)
	require.NotNil(t, ichimoku)
	assert.True(t, ichimoku.Tenkan.GreaterThan(decimal.Zero))
	assert.True(t, ichimoku.Kijun.GreaterThan(decimal.Zero))
}

func TestFibonacci(t *testing.T) {
	levels := CalculateFibonacciLevels(decimal.NewFromFloat(100), decimal.NewFromFloat(50))
	require.Len(t, levels, 5)

	assert.True(t, levels[0].Equal(decimal.NewFromFloat(50)))
	assert.True(t, levels[4].Equal(decimal.NewFromFloat(100)))
}

func TestPivotPoints(t *testing.T) {
	candle := &types.Candle{
		High:  decimal.NewFromFloat(110),
		Low:   decimal.NewFromFloat(90),
		Close: decimal.NewFromFloat(105),
	}

	pivot := CalculatePivotPoints(candle)
	require.NotNil(t, pivot)
	assert.True(t, pivot.PP.GreaterThan(decimal.NewFromFloat(90)))
	assert.True(t, pivot.R1.GreaterThan(pivot.PP))
	assert.True(t, pivot.S1.LessThan(pivot.PP))
}

func TestVolumeProfile(t *testing.T) {
	candles := generateTestCandles(100, 100.0)

	profile := CalculateVolumeProfile(candles, 10)
	require.NotNil(t, profile)
	assert.Len(t, profile.Bins, 10)
}

func TestHeikinAshi(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	ha := CalculateHeikinAshi(candles)
	require.NotNil(t, ha)
	require.Len(t, ha, len(candles))
}

func TestSuperTrend(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	st := CalculateSuperTrend(candles, 10, 3.0)
	require.NotNil(t, st)
}

func TestKeltnerChannels(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	kc := CalculateKeltnerChannels(candles, 20, 2.0)
	require.NotNil(t, kc)
	assert.True(t, kc.Upper.GreaterThan(kc.Middle))
	assert.True(t, kc.Middle.GreaterThan(kc.Lower))
}

func TestAlligator(t *testing.T) {
	candles := generateTestCandles(100, 100.0)

	alligator := CalculateAlligator(candles)
	require.NotNil(t, alligator)
	assert.True(t, alligator.Jaw.GreaterThan(decimal.Zero))
	assert.True(t, alligator.Teeth.GreaterThan(decimal.Zero))
	assert.True(t, alligator.Lips.GreaterThan(decimal.Zero))
}

func TestMFI(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	mfi := CalculateMFI(candles, 14)
	require.NotNil(t, mfi)

	mfiValue, _ := mfi.Float64()
	assert.GreaterOrEqual(t, mfiValue, 0.0)
	assert.LessOrEqual(t, mfiValue, 100.0)
}

func TestWPR(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	wpr := CalculateWPR(candles, 14)
	require.NotNil(t, wpr)

	wprValue, _ := wpr.Float64()
	assert.GreaterOrEqual(t, wprValue, -100.0)
	assert.LessOrEqual(t, wprValue, 0.0)
}

func TestCMF(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	cmf := CalculateCMF(candles, 20)
	require.NotNil(t, cmf)
	assert.True(t, cmf.GreaterThanOrEqual(decimal.NewFromFloat(-1)))
	assert.True(t, cmf.LessThanOrEqual(decimal.NewFromFloat(1)))
}

func TestSTOCHRSI(t *testing.T) {
	candles := generateTestCandles(50, 100.0)

	stochrsi := CalculateSTOCHRSI(candles, 14, 14, 3)
	require.NotNil(t, stochrsi)
}

func TestParabolicSAR(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	sar := CalculateParabolicSAR(candles, 0.02, 0.2)
	require.NotNil(t, sar)
}

func TestAroon(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	aroon := CalculateAroon(candles, 25)
	require.NotNil(t, aroon)
	assert.True(t, aroon.Up.GreaterThanOrEqual(decimal.Zero))
	assert.True(t, aroon.Up.LessThanOrEqual(decimal.NewFromInt(100)))
	assert.True(t, aroon.Down.GreaterThanOrEqual(decimal.Zero))
	assert.True(t, aroon.Down.LessThanOrEqual(decimal.NewFromInt(100)))
}

func TestTRIX(t *testing.T) {
	candles := generateTestCandles(50, 100.0)

	trix := CalculateTRIX(candles, 15)
	require.NotNil(t, trix)
}

func TestMomentum(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	momentum := CalculateMomentum(candles, 10)
	require.NotNil(t, momentum)
}

func TestROC(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	roc := CalculateROC(candles, 10)
	require.NotNil(t, roc)
}

func TestVolumeWeight(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	weight := CalculateVolumeWeight(candles, 14)
	require.NotNil(t, weight)
	assert.True(t, weight.GreaterThan(decimal.Zero))
}

func TestZigZag(t *testing.T) {
	candles := generateTestCandles(100, 100.0)

	zz := CalculateZigZag(candles, 5.0)
	require.NotNil(t, zz)
}

func TestHTTrendLine(t *testing.T) {
	candles := generateTestCandles(50, 100.0)

	trendline := CalculateHTTrendLine(candles)
	require.NotNil(t, trendline)
}

func TestTrendIntensity(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	intensity := CalculateTrendIntensity(candles, 14, 0.5)
	require.NotNil(t, intensity)
	assert.True(t, intensity.GreaterThanOrEqual(decimal.Zero))
	assert.True(t, intensity.LessThanOrEqual(decimal.NewFromInt(100)))
}

func TestVolumeWeightedMA(t *testing.T) {
	candles := generateTestCandles(30, 100.0)

	vwma := CalculateVolumeWeightedMA(candles, 20)
	require.NotNil(t, vwma)
	assert.True(t, vwma.GreaterThan(decimal.Zero))
}

func TestDEMA(t *testing.T) {
	candles := generateTestCandles(50, 100.0)

	dema := CalculateDEMA(candles, 20)
	require.NotNil(t, dema)
	assert.True(t, dema.GreaterThan(decimal.Zero))
}

func TestTEMA(t *testing.T) {
	candles := generateTestCandles(50, 100.0)

	tema := CalculateTEMA(candles, 20)
	require.NotNil(t, tema)
	assert.True(t, tema.GreaterThan(decimal.Zero))
}

func generateTestCandles(count int, basePrice float64) []*types.Candle {
	candles := make([]*types.Candle, count)
	for i := range candles {
		offset := float64(i) * 0.5
		volatility := float64(i%5) * 0.1
		candles[i] = &types.Candle{
			Open:     decimal.NewFromFloat(basePrice + offset - volatility),
			High:     decimal.NewFromFloat(basePrice + offset + 2 + volatility),
			Low:      decimal.NewFromFloat(basePrice + offset - 2 - volatility),
			Close:    decimal.NewFromFloat(basePrice + offset + 1),
			Volume:   decimal.NewFromFloat(1000 + float64(i)*10),
			TradeCount: int64(100 + i),
			Vwap:     decimal.NewFromFloat(basePrice + offset + 1),
		}
	}
	return candles
}
