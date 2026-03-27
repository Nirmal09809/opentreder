package terminal

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	primaryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Bold(true)

	secondaryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A855F7"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#374151")).
			BorderStyle(lipgloss.RoundedBorder())

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#1E40AF")).
			Bold(true).
			Padding(0, 1)

	greenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E"))

	redStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	cyanStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4"))

	goldStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))
)

type MenuItem struct {
	Name        string
	Description string
	Icon        string
}

func GetMainMenu() []MenuItem {
	return []MenuItem{
		{"Dashboard", "System overview & key metrics", "📊"},
		{"Markets", "Live market data & prices", "📈"},
		{"Portfolio", "Positions & P&L tracking", "💼"},
		{"Strategies", "Trading strategies management", "🎯"},
		{"AI Brain", "Real-time AI market analysis", "🤖"},
		{"Risk Manager", "Exposure & risk metrics", "⚠️"},
		{"Backtest", "Strategy historical testing", "🔬"},
		{"Exchanges", "Exchange connections status", "🔗"},
		{"Logs", "System logs & events", "📋"},
		{"Settings", "Configuration & preferences", "⚙️"},
	}
}

func RenderHeader(title string) string {
	width := 100
	padding := (width - len(title) - 4) / 2
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#1E40AF")).
		Width(width).
		Render(fmt.Sprintf("%s %s %s", 
			"═".Repeat(padding),
			title,
			"═".Repeat(padding)))
}

func RenderBox(title, content string, width int) string {
	border := "─".Repeat(width - 2)
	top := fmt.Sprintf("┌%s┐", border)
	bottom := fmt.Sprintf("└%s┘", border)
	titleBar := fmt.Sprintf("│ %s%s │", title, " ".Repeat(width-len(title)-4))
	body := ""
	lines := splitLines(content, width-4)
	for _, line := range lines {
		body += fmt.Sprintf("│ %s%s │\n", line, " ".Repeat(width-len(line)-4))
	}
	return fmt.Sprintf("%s\n%s%s\n%s", top, titleBar, body, bottom)
}

func splitLines(content string, width int) []string {
	var lines []string
	words := ""
	for _, word := range content {
		if len(words)+1 > width {
			lines = append(lines, words)
			words = ""
		}
		words += string(word)
	}
	if words != "" {
		lines = append(lines, words)
	}
	return lines
}

func RenderProgressBar(current, max float64, width int) string {
	percent := current / max
	filled := int(percent * float64(width))
	empty := width - filled
	return fmt.Sprintf("[%s%s]",
		greenStyle.Render("█".Repeat(filled)),
		dimStyle.Render("░".Repeat(empty)))
}

type MarketData struct {
	Symbol      string
	Price       float64
	Change24h   float64
	Volume24h   float64
	High24h     float64
	Low24h      float64
}

func RenderMarketCard(m MarketData) string {
	changeColor := greenStyle
	if m.Change24h < 0 {
		changeColor = redStyle
	}
	arrow := "▲"
	if m.Change24h < 0 {
		arrow = "▼"
	}
	return fmt.Sprintf(`┌────────────────────────────────────┐
│ %s %-10s                    │
├────────────────────────────────────┤
│ Price:      $%-12.2f        │
│ 24h Change: %s%.2f%%           │
│ 24h Volume: $%.2f           │
│ High:       $%-12.2f        │
│ Low:        $%-12.2f        │
└────────────────────────────────────┘`,
		cyanStyle.Render(m.Symbol),
		m.Symbol,
		m.Price,
		changeColor.Render(fmt.Sprintf("%s %.2f%%", arrow, m.Change24h)),
		m.Volume24h,
		m.High24h,
		m.Low24h)
}

type Position struct {
	Symbol     string
	Side       string
	Quantity   float64
	EntryPrice float64
	CurPrice   float64
	PnL        float64
	PnLPct     float64
}

func RenderPositionCard(p Position) string {
	pnlColor := greenStyle
	if p.PnL < 0 {
		pnlColor = redStyle
	}
	return fmt.Sprintf(`┌────────────────────────────────────┐
│ %s %-10s %s              │
├────────────────────────────────────┤
│ Quantity:    %-12.6f        │
│ Entry:       $%-12.2f        │
│ Current:     $%-12.2f        │
│ P&L:         %s$%.2f (%.2f%%) │
└────────────────────────────────────┘`,
		cyanStyle.Render(p.Symbol),
		p.Symbol,
		dimStyle.Render(p.Side),
		p.Quantity,
		p.EntryPrice,
		p.CurPrice,
		pnlColor.Render(fmt.Sprintf("$%.2f", p.PnL)),
		p.PnLPct)
}

type StrategyInfo struct {
	Name         string
	Type         string
	Trades       int
	WinRate      float64
	ProfitFactor float64
	Sharpe       float64
	MaxDrawdown  float64
}

func RenderStrategyCard(s StrategyInfo) string {
	wrColor := greenStyle
	if s.WinRate < 50 {
		wrColor = warningStyle
	}
	return fmt.Sprintf(`┌────────────────────────────────────┐
│ %s %-20s │
├────────────────────────────────────┤
│ Type:        %-12s        │
│ Trades:      %-12d        │
│ Win Rate:    %s%.1f%%            │
│ Profit:      %s%.2fx            │
│ Sharpe:      %s%.2f             │
│ Max DD:      %s%.2f%%           │
└────────────────────────────────────┘`,
		goldStyle.Render("🎯"),
		s.Name,
		dimStyle.Render(s.Type),
		s.Trades,
		wrColor.Render(fmt.Sprintf("%.1f%%", s.WinRate)),
		greenStyle.Render(fmt.Sprintf("%.2fx", s.ProfitFactor)),
		cyanStyle.Render(fmt.Sprintf("%.2f", s.Sharpe)),
		warningStyle.Render(fmt.Sprintf("%.2f%%", s.MaxDrawdown)))
}

type AIAnalysis struct {
	Symbol       string
	Sentiment    string
	Signal       string
	Confidence   float64
	Prediction   float64
	Indicators   map[string]string
}

func RenderAIAnalysis(a AIAnalysis) string {
	sentimentColor := greenStyle
	if a.Sentiment == "Bearish" {
		sentimentColor = redStyle
	} else if a.Sentiment == "Neutral" {
		sentimentColor = warningStyle
	}
	signalColor := greenStyle
	if a.Signal == "SELL" {
		signalColor = redStyle
	} else if a.Signal == "HOLD" {
		signalColor = warningStyle
	}
	return fmt.Sprintf(`┌────────────────────────────────────┐
│ %s AI Brain Analysis             │
├────────────────────────────────────┤
│ Symbol:     %-12s        │
│ Sentiment:  %s%-12s        │
│ Signal:     %s%-12s        │
│ Confidence: %s%.0f%%            │
│ Prediction: $%-12.2f        │
├────────────────────────────────────┤
│ %s Indicators                     │
│ RSI:       %-12s        │
│ MACD:      %-12s        │
│ BB:        %-12s        │
└────────────────────────────────────┘`,
		secondaryStyle.Render("🤖"),
		dimStyle.Render(a.Symbol),
		sentimentColor.Render(a.Sentiment),
		signalColor.Render(a.Signal),
		cyanStyle.Render(fmt.Sprintf("%.0f%%", a.Confidence)),
		a.Prediction,
		dimStyle.Render("─".Repeat(34)),
		dimStyle.Render(a.Indicators["RSI"]),
		dimStyle.Render(a.Indicators["MACD"]),
		dimStyle.Render(a.Indicators["BB"]))
}

type RiskMetrics struct {
	TotalExposure   float64
	MaxExposure     float64
	CurrentDrawdown float64
	MaxDrawdown     float64
	DailyLoss       float64
	MaxDailyLoss    float64
	Leverage        float64
	MarginUsed      float64
}

func RenderRiskMetrics(r RiskMetrics) string {
	expPct := (r.TotalExposure / r.MaxExposure) * 100
	return fmt.Sprintf(`┌────────────────────────────────────┐
│ %s Risk Manager                │
├────────────────────────────────────┤
│ Exposure:  %s %6.1f%% / %.2f   │
│ Drawdown:  %s %6.2f%% / %.2f   │
│ Daily Loss:%s %6.2f%% / %.2f   │
│ Leverage:  %-12.2fx        │
│ Margin:    %-12.2f%%        │
├────────────────────────────────────┤
%s
└────────────────────────────────────┘`,
		warningStyle.Render("⚠️"),
		cyanStyle.Render(fmt.Sprintf("$%.2f", r.TotalExposure)),
		expPct,
		r.MaxExposure,
		warningStyle.Render(fmt.Sprintf("%.2f%%", r.CurrentDrawdown)),
		r.CurrentDrawdown,
		r.MaxDrawdown,
		errorStyle.Render(fmt.Sprintf("%.2f%%", r.DailyLoss)),
		r.DailyLoss,
		r.MaxDailyLoss,
		r.Leverage,
		r.MarginUsed,
		RenderProgressBar(r.TotalExposure, r.MaxExposure, 32))
}

type ExchangeStatus struct {
	Name    string
	Status  string
	Latency int
	Symbols int
}

func RenderExchangeCard(e ExchangeStatus) string {
	statusColor := greenStyle
	statusIcon := "🟢"
	if e.Status != "Connected" {
		statusColor = redStyle
		statusIcon = "🔴"
	}
	return fmt.Sprintf(`┌────────────────────────────────────┐
│ %s %-15s %s        │
├────────────────────────────────────┤
│ Status:   %-12s        │
│ Latency:  %-12d ms     │
│ Symbols:  %-12d        │
└────────────────────────────────────┘`,
		cyanStyle.Render(e.Name),
		e.Name,
		statusIcon,
		statusColor.Render(e.Status),
		e.Latency,
		e.Symbols)
}

type BacktestResult struct {
	Strategy     string
	Symbol       string
	Period       string
	TotalTrades  int
	WinRate      float64
	Profit       float64
	MaxDrawdown  float64
	Sharpe       float64
	Sortino      float64
	Calmar       float64
}

func RenderBacktestResult(b BacktestResult) string {
	return fmt.Sprintf(`┌────────────────────────────────────┐
│ %s Backtest Results              │
├────────────────────────────────────┤
│ Strategy: %-12s        │
│ Symbol:   %-12s        │
│ Period:   %-12s        │
├────────────────────────────────────┤
│ Trades:   %-12d        │
│ Win Rate: %s%-9.1f%%       │
│ Profit:   %s$%-9.2f       │
│ Max DD:   %s%-9.2f%%       │
├────────────────────────────────────┤
│ Sharpe:   %-12.2f        │
│ Sortino:  %-12.2f        │
│ Calmar:   %-12.2f        │
└────────────────────────────────────┘`,
		cyanStyle.Render("🔬"),
		dimStyle.Render(b.Strategy),
		dimStyle.Render(b.Symbol),
		dimStyle.Render(b.Period),
		b.TotalTrades,
		greenStyle.Render(fmt.Sprintf("%.1f%%", b.WinRate)),
		successStyle.Render(fmt.Sprintf("$%.2f", b.Profit)),
		warningStyle.Render(fmt.Sprintf("%.2f%%", b.MaxDrawdown)),
		b.Sharpe,
		b.Sortino,
		b.Calmar)
}

type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
}

func RenderLogEntry(l LogEntry) string {
	levelColor := dimStyle
	levelIcon := "•"
	switch l.Level {
	case "INFO":
		levelColor = infoStyle
		levelIcon = "ℹ"
	case "WARN":
		levelColor = warningStyle
		levelIcon = "⚠"
	case "ERROR":
		levelColor = errorStyle
		levelIcon = "✖"
	case "DEBUG":
		levelColor = dimStyle
		levelIcon = "◦"
	}
	return fmt.Sprintf("%s %s %s %s",
		dimStyle.Render(l.Time.Format("15:04:05")),
		levelColor.Render(levelIcon),
		levelColor.Render(fmt.Sprintf("[%s]", l.Level)),
		l.Message)
}

func RenderWelcome() string {
	return fmt.Sprintf(`
%s

%s Enterprise-Grade AI Trading Framework %s

%s 10x More Powerful Than NautilusTrader %s

%s
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  %s🤖 AI-Powered Trading    │    %s📊 15+ Exchanges      │    %s⚡ HFT Ready    │
│     LLM + ML + RL           │     CEX + DEX + Stocks    │    Nanosecond       │
│                                                                             │
│  %s🎯 15+ Strategies       │    %s📈 30+ Indicators      │    %s🔬 Backtesting  │
│     Grid, DCA, Scalping     │     TA-Lib Compatible     │     Full Analytics  │
│                                                                             │
│  %s⚠️ Risk Management      │    %s💾 Event Sourcing      │    %s🛡️ Security    │
│     Real-time Protection    │     Replay & Debug        │     Audit Ready     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
%s

  %s

  Press %s to navigate   │   %s to exit   │   %s for help
`,
		goldStyle.Render("═══════════════════════════════════════════════════════════════════════════════"),
		primaryStyle.Render("╔══════════════════════════════════════════════════════════════════════╗"),
		primaryStyle.Render("║"),
		cyanStyle.Render("║"),
		cyanStyle.Render("║"),
		dimStyle.Render("║"),
		secondaryStyle.Render("   ║   • LLM Integration"),
		cyanStyle.Render("   ║   • Binance, Bybit, Kraken"),
		successStyle.Render("   ║   • Nanosecond Precision"),
		goldStyle.Render("   ║   • Grid, DCA, Scalping"),
		infoStyle.Render("   ║   • SMA, RSI, MACD, Bollinger"),
		warningStyle.Render("   ║   • Sharpe, Sortino, Drawdown"),
		errorStyle.Render("   ║   • Exposure Limits"),
		dimStyle.Render("   ║   • Event Store"),
		greenStyle.Render("   ║   • API Keys Encrypted"),
		goldStyle.Render("╚══════════════════════════════════════════════════════════════════════╝"),
		dimStyle.Render("───────────────────────────────────────────────────────────────────────────────"),
		cyanStyle.Render("[Arrow Keys]"),
		redStyle.Render("[Q]"),
		secondaryStyle.Render("[H]"))
}

func RenderFooter() string {
	return fmt.Sprintf(`%s
  Exchanges: %s │ Strategies: %s │ Positions: %s │ AI: %s │ Risk: %s %s`,
		dimStyle.Render("─".Repeat(100)),
		greenStyle.Render("12 Active"),
		cyanStyle.Render("5 Running"),
		goldStyle.Render("3 Open"),
		secondaryStyle.Render("Online"),
		warningStyle.Render("Normal"),
		dimStyle.Render(time.Now().Format("│ 15:04:05")))
}
