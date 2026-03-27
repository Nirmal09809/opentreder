package analytics

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"time"

	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type PerformanceAnalyzer struct {
	trades       []*types.Trade
	equityCurve  []EquityPoint
	benchmark    []float64
}

type EquityPoint struct {
	Timestamp  time.Time
	Equity     decimal.Decimal
	Drawdown   decimal.Decimal
	Return     decimal.Decimal
}

type PerformanceMetrics struct {
	TotalReturn          decimal.Decimal
	AnnualizedReturn     decimal.Decimal
	Volatility          decimal.Decimal
	SharpeRatio         decimal.Decimal
	SortinoRatio        decimal.Decimal
	CalmarRatio         decimal.Decimal
	MaxDrawdown         decimal.Decimal
	MaxDrawdownDuration time.Duration
	MaxDrawdownDate     time.Time
	WinRate             decimal.Decimal
	ProfitFactor        decimal.Decimal
	AvgWin              decimal.Decimal
	AvgLoss             decimal.Decimal
	LargestWin          decimal.Decimal
	LargestLoss         decimal.Decimal
	AvgTradeDuration    time.Duration
	TotalTrades         int
	WinningTrades       int
	LosingTrades        int
	ConsecutiveWins     int
	ConsecutiveLosses   int
	MaxConsecutiveWins  int
	MaxConsecutiveLosses int
	AvgHoldingPeriod    time.Duration
	RecoveryFactor      decimal.Decimal
	KestrelRatio        decimal.Decimal
	CalmarRatioAnnual   decimal.Decimal
	OmegaRatio          decimal.Decimal
	VaR95               decimal.Decimal
	CVaR95              decimal.Decimal
}

type TradeStats struct {
	EntryTime       time.Time
	ExitTime        time.Time
	Symbol          string
	Side            types.PositionSide
	EntryPrice      decimal.Decimal
	ExitPrice       decimal.Decimal
	Quantity        decimal.Decimal
	Commission      decimal.Decimal
	PnL             decimal.Decimal
	ReturnPct       decimal.Decimal
	Duration        time.Duration
	MaxRunUp        decimal.Decimal
	MaxDrawdown     decimal.Decimal
	ExitReason      string
}

func NewPerformanceAnalyzer() *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		trades:      make([]*types.Trade, 0),
		equityCurve: make([]EquityPoint, 0),
	}
}

func (a *PerformanceAnalyzer) AddTrade(trade *types.Trade) {
	a.trades = append(a.trades, trade)
}

func (a *PerformanceAnalyzer) AddEquityPoint(point EquityPoint) {
	a.equityCurve = append(a.equityCurve, point)
}

func (a *PerformanceAnalyzer) CalculateMetrics() *PerformanceMetrics {
	if len(a.trades) == 0 {
		return &PerformanceMetrics{}
	}

	metrics := &PerformanceMetrics{}

	winningTrades := 0
	losingTrades := 0
	var totalWins, totalLosses, largestWin, largestLoss decimal.Decimal
	consecutiveWins, consecutiveLosses := 0, 0
	maxConsecutiveWins, maxConsecutiveLosses := 0, 0

	tradeReturns := make([]float64, 0)

	for _, trade := range a.trades {
		pnl := trade.Price.Mul(trade.Quantity)

		if pnl.GreaterThan(decimal.Zero) {
			winningTrades++
			totalWins = totalWins.Add(pnl)

			if largestWin.LessThan(pnl) {
				largestWin = pnl
			}

			consecutiveWins++
			consecutiveLosses = 0
			if consecutiveWins > maxConsecutiveWins {
				maxConsecutiveWins = consecutiveWins
			}
		} else {
			losingTrades++
			totalLosses = totalLosses.Add(pnl.Abs())

			if largestLoss.LessThan(pnl.Abs()) {
				largestLoss = pnl.Abs()
			}

			consecutiveLosses++
			consecutiveWins = 0
			if consecutiveLosses > maxConsecutiveLosses {
				maxConsecutiveLosses = consecutiveLosses
			}
		}

		if trade.Price.GreaterThan(decimal.Zero) {
			returnPct := pnl.Div(trade.Price).Mul(decimal.NewFromInt(100))
			tradeReturns = append(tradeReturns, returnPct.InexactFloat64())
		}
	}

	metrics.TotalTrades = len(a.trades)
	metrics.WinningTrades = winningTrades
	metrics.LosingTrades = losingTrades

	if metrics.TotalTrades > 0 {
		metrics.WinRate = decimal.NewFromInt(int64(winningTrades)).Div(decimal.NewFromInt(int64(metrics.TotalTrades))).Mul(decimal.NewFromInt(100))
	}

	if winningTrades > 0 {
		metrics.AvgWin = totalWins.Div(decimal.NewFromInt(int64(winningTrades)))
	}

	if losingTrades > 0 {
		metrics.AvgLoss = totalLosses.Div(decimal.NewFromInt(int64(losingTrades)))
	}

	metrics.LargestWin = largestWin
	metrics.LargestLoss = largestLoss

	if !totalLosses.IsZero() {
		metrics.ProfitFactor = totalWins.Div(totalLosses)
	}

	metrics.MaxConsecutiveWins = maxConsecutiveWins
	metrics.MaxConsecutiveLosses = maxConsecutiveLosses

	if len(a.equityCurve) > 0 {
		peak := a.equityCurve[0].Equity
		maxDD := decimal.Zero
		maxDDDate := a.equityCurve[0].Timestamp

		for _, point := range a.equityCurve {
			if point.Equity.GreaterThan(peak) {
				peak = point.Equity
			}

			drawdown := peak.Sub(point.Equity)
			if drawdown.GreaterThan(maxDD) {
				maxDD = drawdown
				maxDDDate = point.Timestamp
			}
		}

		metrics.MaxDrawdown = maxDD
		metrics.MaxDrawdownDate = maxDDDate

		if peak.GreaterThan(decimal.Zero) {
			metrics.MaxDrawdown = maxDD.Div(peak).Mul(decimal.NewFromInt(100))
		}

		startEquity := a.equityCurve[0].Equity
		endEquity := a.equityCurve[len(a.equityCurve)-1].Equity

		if startEquity.GreaterThan(decimal.Zero) {
			metrics.TotalReturn = endEquity.Sub(startEquity).Div(startEquity).Mul(decimal.NewFromInt(100))
		}

		days := len(a.equityCurve)
		if days > 365 {
			annualReturn := metrics.TotalReturn.Mul(decimal.NewFromInt(365)).Div(decimal.NewFromInt(int64(days)))
			metrics.AnnualizedReturn = annualReturn
		}
	}

	metrics.Volatility = a.calculateVolatility(tradeReturns)
	metrics.SharpeRatio = a.calculateSharpeRatio(tradeReturns)
	metrics.SortinoRatio = a.calculateSortinoRatio(tradeReturns)

	if metrics.MaxDrawdown.GreaterThan(decimal.Zero) && metrics.AnnualizedReturn.GreaterThan(decimal.Zero) {
		metrics.CalmarRatio = metrics.AnnualizedReturn.Div(metrics.MaxDrawdown)
	}

	metrics.VaR95 = a.calculateVaR(tradeReturns, 0.95)
	metrics.CVaR95 = a.calculateCVaR(tradeReturns, 0.95)

	return metrics
}

func (a *PerformanceAnalyzer) calculateVolatility(returns []float64) decimal.Decimal {
	if len(returns) < 2 {
		return decimal.Zero
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)

	stdDev := math.Sqrt(variance)

	return decimal.NewFromFloat(stdDev * math.Sqrt(252))
}

func (a *PerformanceAnalyzer) calculateSharpeRatio(returns []float64) decimal.Decimal {
	if len(returns) < 2 {
		return decimal.Zero
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)

	stdDev := math.Sqrt(variance)
	if stdDev == 0 {
		return decimal.Zero
	}

	sharpe := (mean * 252) / (stdDev * math.Sqrt(252))

	return decimal.NewFromFloat(sharpe)
}

func (a *PerformanceAnalyzer) calculateSortinoRatio(returns []float64) decimal.Decimal {
	if len(returns) < 2 {
		return decimal.Zero
	}

	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	downside := 0.0
	count := 0
	for _, r := range returns {
		if r < 0 {
			downside += r * r
			count++
		}
	}

	if count == 0 {
		return decimal.Zero
	}

	downsideDev := math.Sqrt(downside / float64(count))
	if downsideDev == 0 {
		return decimal.Zero
	}

	sortino := (mean * 252) / (downsideDev * math.Sqrt(252))

	return decimal.NewFromFloat(sortino)
}

func (a *PerformanceAnalyzer) calculateVaR(returns []float64, confidence float64) decimal.Decimal {
	if len(returns) == 0 {
		return decimal.Zero
	}

	sorted := make([]float64, len(returns))
	copy(sorted, returns)
	sort.Float64s(sorted)

	index := int(float64(len(sorted)) * (1 - confidence))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}

	return decimal.NewFromFloat(-sorted[index])
}

func (a *PerformanceAnalyzer) calculateCVaR(returns []float64, confidence float64) decimal.Decimal {
	if len(returns) == 0 {
		return decimal.Zero
	}

	sorted := make([]float64, len(returns))
	copy(sorted, returns)
	sort.Float64s(sorted)

	index := int(float64(len(sorted)) * (1 - confidence))
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}

	sum := 0.0
	for i := 0; i <= index; i++ {
		sum += sorted[i]
	}

	cvar := -sum / float64(index+1)

	return decimal.NewFromFloat(cvar)
}

func (a *PerformanceAnalyzer) GenerateReport() string {
	metrics := a.CalculateMetrics()

	report := `
╔════════════════════════════════════════════════════════════════════════════════╗
║                        PERFORMANCE REPORT                                     ║
╠════════════════════════════════════════════════════════════════════════════════╣
║  RETURNS                                                                    ║
║  Total Return:        %-15s                                    ║
║  Annualized Return:   %-15s                                    ║
║  Volatility:          %-15s                                    ║
╠════════════════════════════════════════════════════════════════════════════════╣
║  RISK METRICS                                                                ║
║  Sharpe Ratio:        %-15s                                    ║
║  Sortino Ratio:       %-15s                                    ║
║  Calmar Ratio:        %-15s                                    ║
║  Max Drawdown:        %-15s                                    ║
║  VaR (95%%):          %-15s                                    ║
║  CVaR (95%%):         %-15s                                    ║
╠════════════════════════════════════════════════════════════════════════════════╣
║  TRADING STATISTICS                                                         ║
║  Total Trades:       %-15d                                    ║
║  Winning Trades:     %-15d                                    ║
║  Losing Trades:      %-15d                                    ║
║  Win Rate:           %-15s                                    ║
║  Profit Factor:      %-15s                                    ║
║  Avg Win:             %-15s                                    ║
║  Avg Loss:            %-15s                                    ║
║  Largest Win:         %-15s                                    ║
║  Largest Loss:       %-15s                                    ║
╠════════════════════════════════════════════════════════════════════════════════╣
║  SEQUENCE METRICS                                                            ║
║  Max Consecutive Wins: %-15d                                    ║
║  Max Consecutive Loss:%-15d                                    ║
╚════════════════════════════════════════════════════════════════════════════════╝
`

	return fmt.Sprintf(report,
		formatDecimal(metrics.TotalReturn)+"%",
		formatDecimal(metrics.AnnualizedReturn)+"%",
		formatDecimal(metrics.Volatility)+"%",
		formatDecimal(metrics.SharpeRatio),
		formatDecimal(metrics.SortinoRatio),
		formatDecimal(metrics.CalmarRatio),
		formatDecimal(metrics.MaxDrawdown)+"%",
		formatDecimal(metrics.VaR95),
		formatDecimal(metrics.CVaR95),
		metrics.TotalTrades,
		metrics.WinningTrades,
		metrics.LosingTrades,
		formatDecimal(metrics.WinRate)+"%",
		formatDecimal(metrics.ProfitFactor),
		"$"+formatDecimal(metrics.AvgWin),
		"$"+formatDecimal(metrics.AvgLoss),
		"$"+formatDecimal(metrics.LargestWin),
		"$"+formatDecimal(metrics.LargestLoss),
		metrics.MaxConsecutiveWins,
		metrics.MaxConsecutiveLosses,
	)
}

func formatDecimal(d decimal.Decimal) string {
	f, _ := d.Float64()
	return fmt.Sprintf("%.2f", f)
}

func (a *PerformanceAnalyzer) CalculateDrawdownPeriods() []DrawdownPeriod {
	if len(a.equityCurve) < 2 {
		return nil
	}

	periods := make([]DrawdownPeriod, 0)
	peak := a.equityCurve[0].Equity
	peakTime := a.equityCurve[0].Timestamp
	inDrawdown := false
	startTime := a.equityCurve[0].Timestamp
	startEquity := a.equityCurve[0].Equity

	for _, point := range a.equityCurve[1:] {
		if point.Equity.GreaterThanOrEqual(peak) {
			if inDrawdown {
				drawdown := peak.Sub(startEquity)
				period := DrawdownPeriod{
					StartTime:     startTime,
					EndTime:       point.Timestamp,
					Peak:         peak,
					Trough:       point.Equity,
					Drawdown:     drawdown,
					Duration:     point.Timestamp.Sub(startTime),
				}
				periods = append(periods, period)
				inDrawdown = false
			}
			peak = point.Equity
			peakTime = point.Timestamp
		} else if !inDrawdown {
			startTime = peakTime
			startEquity = peak
			inDrawdown = true
		}
	}

	return periods
}

type DrawdownPeriod struct {
	StartTime time.Time
	EndTime   time.Time
	Peak     decimal.Decimal
	Trough   decimal.Decimal
	Drawdown decimal.Decimal
	Duration time.Duration
}

func (a *PerformanceAnalyzer) CalculateRollingMetrics(window int) []RollingMetrics {
	if len(a.equityCurve) < window {
		return nil
	}

	metrics := make([]RollingMetrics, 0)

	for i := window; i <= len(a.equityCurve); i++ {
		windowCurve := a.equityCurve[i-window : i]
		startEquity := windowCurve[0].Equity
		endEquity := windowCurve[len(windowCurve)-1].Equity

		returns := make([]float64, len(windowCurve)-1)
		for j := 1; j < len(windowCurve); j++ {
			if windowCurve[j-1].Equity.GreaterThan(decimal.Zero) {
				ret := windowCurve[j].Equity.Sub(windowCurve[j-1].Equity).
					Div(windowCurve[j-1].Equity).InexactFloat64()
				returns = append(returns, ret)
			}
		}

		metric := RollingMetrics{
			Timestamp:      windowCurve[len(windowCurve)-1].Timestamp,
			Return:        endEquity.Sub(startEquity).Div(startEquity).Mul(decimal.NewFromInt(100)),
			Volatility:    decimal.NewFromFloat(a.calculateVolatility(returns).InexactFloat64()),
			Sharpe:       decimal.NewFromFloat(a.calculateSharpeRatio(returns).InexactFloat64()),
			MaxDrawdown:  decimal.Zero,
		}

		peak := windowCurve[0].Equity
		for _, point := range windowCurve {
			if point.Equity.GreaterThan(peak) {
				peak = point.Equity
			}
			dd := peak.Sub(point.Equity)
			if dd.GreaterThan(metric.MaxDrawdown) {
				metric.MaxDrawdown = dd
			}
		}

		metrics = append(metrics, metric)
	}

	return metrics
}

type RollingMetrics struct {
	Timestamp  time.Time
	Return    decimal.Decimal
	Volatility decimal.Decimal
	Sharpe    decimal.Decimal
	MaxDrawdown decimal.Decimal
}

func (a *PerformanceAnalyzer) CompareToBenchmark() *BenchmarkComparison {
	if len(a.benchmark) == 0 {
		return nil
	}

	if len(a.equityCurve) == 0 || len(a.benchmark) == 0 {
		return nil
	}

	portfolioReturns := make([]float64, 0)
	for i := 1; i < len(a.equityCurve); i++ {
		if a.equityCurve[i-1].Equity.GreaterThan(decimal.Zero) {
			ret := a.equityCurve[i].Equity.Sub(a.equityCurve[i-1].Equity).
				Div(a.equityCurve[i-1].Equity).InexactFloat64()
			portfolioReturns = append(portfolioReturns, ret)
		}
	}

	benchmarkReturns := make([]float64, 0)
	for i := 1; i < len(a.benchmark); i++ {
		if a.benchmark[i-1] != 0 {
			ret := (a.benchmark[i] - a.benchmark[i-1]) / a.benchmark[i-1]
			benchmarkReturns = append(benchmarkReturns, ret)
		}
	}

	portfolioMean := mean(portfolioReturns)
	benchmarkMean := mean(benchmarkReturns)

	betterDays := 0
	for i := 0; i < len(portfolioReturns) && i < len(benchmarkReturns); i++ {
		if portfolioReturns[i] > benchmarkReturns[i] {
			betterDays++
		}
	}

	winRate := 0.0
	if len(portfolioReturns) > 0 {
		winRate = float64(betterDays) / float64(len(portfolioReturns)) * 100
	}

	beta := calculateBeta(portfolioReturns, benchmarkReturns)
	alpha := (portfolioMean - beta*benchmarkMean) * 252
	trackingError := stdDev(subtract(portfolioReturns, benchmarkReturns)) * math.Sqrt(252)
	informationRatio := 0.0
	if trackingError != 0 {
		informationRatio = alpha / trackingError
	}

	return &BenchmarkComparison{
		Beta:             decimal.NewFromFloat(beta),
		Alpha:            decimal.NewFromFloat(alpha),
		TrackingError:    decimal.NewFromFloat(trackingError),
		InformationRatio: decimal.NewFromFloat(informationRatio),
		WinRateVsBenchmark: decimal.NewFromFloat(winRate),
	}
}

type BenchmarkComparison struct {
	Beta              decimal.Decimal
	Alpha             decimal.Decimal
	TrackingError     decimal.Decimal
	InformationRatio  decimal.Decimal
	WinRateVsBenchmark decimal.Decimal
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	sum := 0.0
	for _, v := range values {
		diff := v - m
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)-1))
}

func subtract(a, b []float64) []float64 {
	result := make([]float64, len(a))
	for i := range a {
		if i < len(b) {
			result[i] = a[i] - b[i]
		}
	}
	return result
}

func calculateBeta(portfolio, benchmark []float64) float64 {
	if len(portfolio) != len(benchmark) || len(portfolio) == 0 {
		return 1
	}

	cov := covariance(portfolio, benchmark)
	benchVar := variance(benchmark)

	if benchVar == 0 {
		return 1
	}

	return cov / benchVar
}

func covariance(a, b []float64) float64 {
	meanA := mean(a)
	meanB := mean(b)

	sum := 0.0
	for i := range a {
		sum += (a[i] - meanA) * (b[i] - meanB)
	}

	return sum / float64(len(a)-1)
}

func variance(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	sum := 0.0
	for _, v := range values {
		diff := v - m
		sum += diff * diff
	}
	return sum / float64(len(values)-1)
}

func (a *PerformanceAnalyzer) MonteCarloSimulation(numSimulations int, numDays int) *MonteCarloResult {
	if len(a.equityCurve) < 2 {
		return nil
	}

	returns := make([]float64, 0)
	for i := 1; i < len(a.equityCurve); i++ {
		if a.equityCurve[i-1].Equity.GreaterThan(decimal.Zero) {
			ret := a.equityCurve[i].Equity.Sub(a.equityCurve[i-1].Equity).
				Div(a.equityCurve[i-1].Equity).InexactFloat64()
			returns = append(returns, ret)
		}
	}

	meanReturn := mean(returns)
	stdDeviation := stdDev(returns)

	finalValues := make([]float64, numSimulations)

	for i := 0; i < numSimulations; i++ {
		value := 1.0
		for j := 0; j < numDays; j++ {
			gaussian := boxMullerNormal()
			value *= 1 + meanReturn + stdDeviation*gaussian
		}
		finalValues[i] = value
	}

	sort.Float64s(finalValues)

	return &MonteCarloResult{
		Percentile5:    decimal.NewFromFloat(finalValues[int(float64(numSimulations)*0.05)]),
		Percentile25:   decimal.NewFromFloat(finalValues[int(float64(numSimulations)*0.25)]),
		Percentile50:   decimal.NewFromFloat(finalValues[int(float64(numSimulations)*0.50)]),
		Percentile75:   decimal.NewFromFloat(finalValues[int(float64(numSimulations)*0.75)]),
		Percentile95:   decimal.NewFromFloat(finalValues[int(float64(numSimulations)*0.95)]),
		ProbabilityProfit: decimal.NewFromFloat(float64(countAbove(finalValues, 1.0)) / float64(numSimulations) * 100),
	}
}

func boxMullerNormal() float64 {
	u1 := rand.Float64()
	u2 := rand.Float64()
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

func countAbove(values []float64, threshold float64) int {
	count := 0
	for _, v := range values {
		if v > threshold {
			count++
		}
	}
	return count
}

type MonteCarloResult struct {
	Percentile5          decimal.Decimal
	Percentile25         decimal.Decimal
	Percentile50         decimal.Decimal
	Percentile75         decimal.Decimal
	Percentile95         decimal.Decimal
	ProbabilityProfit    decimal.Decimal
}
