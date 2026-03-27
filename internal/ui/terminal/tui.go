package terminal

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	StateMenu state = iota
	StateDashboard
	StateMarkets
	StatePortfolio
	StateStrategies
	StateAIBrain
	StateRisk
	StateBacktest
	StateExchanges
	StateLogs
	StateSettings
)

type model struct {
	state          state
	menuIndex      int
	menuItems      []MenuItem
	width          int
	height         int
	marketData     []MarketData
	positions      []Position
	strategies     []StrategyInfo
	aiAnalysis    []AIAnalysis
	riskMetrics    RiskMetrics
	exchangeStatus []ExchangeStatus
	backtestResult BacktestResult
	logs           []LogEntry
	searchQuery    string
}

func NewModel() *model {
	return &model{
		state:     StateMenu,
		menuIndex: 0,
		menuItems: GetMainMenu(),
		marketData: []MarketData{
			{Symbol: "BTCUSDT", Price: 67432.50, Change24h: 2.34, Volume24h: 28500000000, High24h: 68100.00, Low24h: 65800.00},
			{Symbol: "ETHUSDT", Price: 3521.80, Change24h: 1.87, Volume24h: 15200000000, High24h: 3580.00, Low24h: 3450.00},
			{Symbol: "SOLUSDT", Price: 142.35, Change24h: 5.42, Volume24h: 2800000000, High24h: 148.00, Low24h: 135.00},
			{Symbol: "BNBUSDT", Price: 598.20, Change24h: -0.85, Volume24h: 850000000, High24h: 605.00, Low24h: 592.00},
			{Symbol: "XRPUSDT", Price: 0.5234, Change24h: 3.21, Volume24h: 1200000000, High24h: 0.54, Low24h: 0.50},
		},
		positions: []Position{
			{Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.15, EntryPrice: 65000.00, CurPrice: 67432.50, PnL: 364.88, PnLPct: 3.74},
			{Symbol: "ETHUSDT", Side: "LONG", Quantity: 2.5, EntryPrice: 3400.00, CurPrice: 3521.80, PnL: 304.50, PnLPct: 3.58},
			{Symbol: "SOLUSDT", Side: "SHORT", Quantity: 50, EntryPrice: 145.00, CurPrice: 142.35, PnL: 132.50, PnLPct: 1.83},
		},
		strategies: []StrategyInfo{
			{Name: "Grid BTC", Type: "grid", Trades: 156, WinRate: 68.5, ProfitFactor: 2.34, Sharpe: 1.87, MaxDrawdown: 8.5},
			{Name: "DCA ETH", Type: "dca", Trades: 89, WinRate: 72.1, ProfitFactor: 2.89, Sharpe: 2.15, MaxDrawdown: 5.2},
			{Name: "Scalper", Type: "scalping", Trades: 2341, WinRate: 54.3, ProfitFactor: 1.67, Sharpe: 1.42, MaxDrawdown: 12.3},
		},
		aiAnalysis: []AIAnalysis{
			{Symbol: "BTCUSDT", Sentiment: "Bullish", Signal: "BUY", Confidence: 78, Prediction: 68500.00, Indicators: map[string]string{"RSI": "62 - Neutral", "MACD": "Bullish Cross", "BB": "Upper Band"}},
			{Symbol: "ETHUSDT", Sentiment: "Bullish", Signal: "BUY", Confidence: 72, Prediction: 3600.00, Indicators: map[string]string{"RSI": "58 - Neutral", "MACD": "Above Signal", "BB": "Middle Band"}},
		},
		riskMetrics: RiskMetrics{
			TotalExposure:   45000.00,
			MaxExposure:     100000.00,
			CurrentDrawdown: 2.3,
			MaxDrawdown:     15.0,
			DailyLoss:       0.5,
			MaxDailyLoss:    5.0,
			Leverage:        2.0,
			MarginUsed:      35.0,
		},
		exchangeStatus: []ExchangeStatus{
			{Name: "Binance", Status: "Connected", Latency: 12, Symbols: 450},
			{Name: "Bybit", Status: "Connected", Latency: 18, Symbols: 380},
			{Name: "OKX", Status: "Connected", Latency: 25, Symbols: 320},
			{Name: "Kraken", Status: "Connected", Latency: 45, Symbols: 280},
			{Name: "Hyperliquid", Status: "Connected", Latency: 8, Symbols: 140},
		},
		backtestResult: BacktestResult{
			Strategy:    "Grid Trading",
			Symbol:      "BTCUSDT",
			Period:      "90 Days",
			TotalTrades: 156,
			WinRate:     68.5,
			Profit:      12453.67,
			MaxDrawdown: 8.5,
			Sharpe:      1.87,
			Sortino:     2.45,
			Calmar:      1.23,
		},
		logs: []LogEntry{
			{Time: time.Now().Add(-time.Second * 10), Level: "INFO", Message: "Strategy Grid BTC started successfully"},
			{Time: time.Now().Add(-time.Second * 30), Level: "INFO", Message: "Order #12345 filled: BUY 0.01 BTC @ 67432.50"},
			{Time: time.Now().Add(-time.Minute * 1), Level: "INFO", Message: "AI Brain analysis updated for BTCUSDT"},
			{Time: time.Now().Add(-time.Minute * 2), Level: "DEBUG", Message: "WebSocket message received from Binance"},
			{Time: time.Now().Add(-time.Minute * 3), Level: "WARN", Message: "High volatility detected on SOLUSDT"},
			{Time: time.Now().Add(-time.Minute * 5), Level: "INFO", Message: "Backtest completed: Sharpe 1.87, MaxDD 8.5%"},
		},
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.state == StateMenu {
				m.menuIndex--
				if m.menuIndex < 0 {
					m.menuIndex = len(m.menuItems) - 1
				}
			}
		case "down", "j":
			if m.state == StateMenu {
				m.menuIndex++
				if m.menuIndex >= len(m.menuItems) {
					m.menuIndex = 0
				}
			}
		case "enter", "right", "l":
			m.selectMenuItem()
		case "esc", "left", "h", "backspace":
			if m.state != StateMenu {
				m.state = StateMenu
			}
		case "1":
			if m.state == StateMenu {
				m.state = StateDashboard
			}
		case "2":
			if m.state == StateMenu {
				m.state = StateMarkets
			}
		case "3":
			if m.state == StateMenu {
				m.state = StatePortfolio
			}
		}
	}
	return m, nil
}

func (m *model) selectMenuItem() {
	switch m.menuIndex {
	case 0:
		m.state = StateDashboard
	case 1:
		m.state = StateMarkets
	case 2:
		m.state = StatePortfolio
	case 3:
		m.state = StateStrategies
	case 4:
		m.state = StateAIBrain
	case 5:
		m.state = StateRisk
	case 6:
		m.state = StateBacktest
	case 7:
		m.state = StateExchanges
	case 8:
		m.state = StateLogs
	case 9:
		m.state = StateSettings
	}
}

func (m *model) View() string {
	var content string

	switch m.state {
	case StateMenu:
		content = m.renderMenu()
	case StateDashboard:
		content = m.renderDashboard()
	case StateMarkets:
		content = m.renderMarkets()
	case StatePortfolio:
		content = m.renderPortfolio()
	case StateStrategies:
		content = m.renderStrategies()
	case StateAIBrain:
		content = m.renderAIBrain()
	case StateRisk:
		content = m.renderRisk()
	case StateBacktest:
		content = m.renderBacktest()
	case StateExchanges:
		content = m.renderExchanges()
	case StateLogs:
		content = m.renderLogs()
	case StateSettings:
		content = m.renderSettings()
	}

	return fmt.Sprintf("%s\n%s\n%s",
		RenderHeader("OpenTrader - Enterprise AI Trading Framework"),
		content,
		RenderFooter())
}

func (m *model) renderMenu() string {
	var menuLines []string
	menuLines = append(menuLines, dimStyle.Render("\n  Use ↑↓ to navigate, ENTER to select, Q to quit\n"))
	menuLines = append(menuLines, dimStyle.Render("  ┌─────────────────────────────────────────────────────────────────────────┐\n"))

	for i, item := range m.menuItems {
		prefix := "  │  "
		suffix := "  │"
		
		cursor := "  "
		if i == m.menuIndex {
			cursor = goldStyle.Render(" ► ")
			prefix = primaryStyle.Render("  │► ")
		}

		icon := dimStyle.Render(item.Icon)
		name := primaryStyle.Render(fmt.Sprintf("%-12s", item.Name))
		desc := dimStyle.Render(item.Description)
		
		menuLines = append(menuLines, fmt.Sprintf("%s%s %s %s %s", prefix, cursor, icon, name, desc, suffix))
	}

	menuLines = append(menuLines, dimStyle.Render("  └─────────────────────────────────────────────────────────────────────────┘\n"))

	return fmt.Sprintf("%s\n%s\n\n  %s\n  %s",
		RenderWelcome(),
		dimStyle.Render("════════════════════════════════════════════════════════════════════════════════"),
		dimStyle.Render("Featured: Press 1-9 for quick access, or use arrow keys"),
		lipgloss.JoinVertical(lipgloss.Left, menuLines...))
}

func (m *model) renderDashboard() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    📊 DASHBOARD OVERVIEW                          ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	
	lines = append(lines, m.renderDashboardRow1())
	lines = append(lines, m.renderDashboardRow2())
	lines = append(lines, m.renderDashboardRow3())
	
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderDashboardRow1() string {
	portfolio := fmt.Sprintf(`  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
  │ %s Portfolio Value    │  │ %s Daily P&L      │  │ %s Total Trades    │
  │ $124,567.89          │  │ +$1,234.56 (2.3%) │  │ 156                │
  │ ▲ 12.5%              │  │ ▲ 8 Trades        │  │ Win Rate: 68.5%    │
  └──────────────────┘  └──────────────────┘  └──────────────────┘`,
		successStyle.Render("💼"),
		greenStyle.Render("📈"),
		cyanStyle.Render("📋"))
	return portfolio
}

func (m *model) renderDashboardRow2() string {
	return fmt.Sprintf(`  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
  │ %s Open Positions   │  │ %s AI Confidence   │  │ %s System Health   │
  │ 3 Active            │  │ 78%               │  │ ● All Systems OK   │
  │ Long: 2 | Short: 1  │  │ Signal: BUY       │  │ Latency: 12ms      │
  └──────────────────┘  └──────────────────┘  └──────────────────┘`,
		goldStyle.Render("📊"),
		secondaryStyle.Render("🤖"),
		greenStyle.Render("✓"))
}

func (m *model) renderDashboardRow3() string {
	return fmt.Sprintf(`  ┌──────────────────────────────────────────────────────────────────────┐
  │ %s Top Movers (24h)                                          │
  ├──────────────────────────────────────────────────────────────────────┤
  │  %-8s %s $%.2f  ▲ %.2f%%  │  %-8s %s $%.2f  ▼ %.2f%%  │
  │  %-8s %s $%.2f  ▲ %.2f%%  │  %-8s %s $%.2f  ▲ %.2f%%  │
  └──────────────────────────────────────────────────────────────────────┘`,
		cyanStyle.Render("🔥"),
		"SOL", greenStyle.Render("$"), 142.35, 5.42,
		"BNB", dimStyle.Render("$"), 598.20, 0.85,
		"BTC", greenStyle.Render("$"), 67432.50, 2.34,
		"XRP", greenStyle.Render("$"), 0.5234, 3.21)
}

func (m *model) renderMarkets() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    📈 LIVE MARKET DATA                           ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))

	for _, market := range m.marketData {
		changeColor := greenStyle
		arrow := "▲"
		if market.Change24h < 0 {
			changeColor = redStyle
			arrow = "▼"
		}
		lines = append(lines, fmt.Sprintf(`  │  %s %-10s %s$%-12.2f  %s%s %.2f%%  │`,
			cyanStyle.Render("•"),
			market.Symbol,
			primaryStyle.Render("$"),
			market.Price,
			changeColor.Render(arrow),
			changeColor.Render(fmt.Sprintf("%.2f%%", market.Change24h)),
			dimStyle.Render(" Vol: $"+formatVolume(market.Volume24h))))
	}

	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderPortfolio() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    💼 PORTFOLIO & POSITIONS                      ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))

	for _, pos := range m.positions {
		pnlColor := greenStyle
		if pos.PnL < 0 {
			pnlColor = redStyle
		}
		lines = append(lines, fmt.Sprintf(`  │  %s %-10s %s%-6s %-12.6f  Entry: $%-10.2f  Current: $%-10.2f  P&L: %s$%-10.2f (%.2f%%)  │`,
			cyanStyle.Render("•"),
			pos.Symbol,
			dimStyle.Render(pos.Side),
			pos.Quantity,
			pos.EntryPrice,
			pos.CurPrice,
			pnlColor.Render(fmt.Sprintf("$%.2f", pos.PnL)),
			pnlColor.Render(fmt.Sprintf("%.2f%%", pos.PnLPct))))
	}

	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render(fmt.Sprintf("  │  Total P&L: %s $802.38 (3.12%%)                                        │",
		greenStyle.Render("▲"))))
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderStrategies() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    🎯 TRADING STRATEGIES                        ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))

	for _, strat := range m.strategies {
		lines = append(lines, fmt.Sprintf(`  │  %s %-12s │ Type: %-10s │ Trades: %-6d │ Win: %s%.1f%% │ Sharpe: %.2f │ DD: %.1f%% │`,
			goldStyle.Render("🎯"),
			primaryStyle.Render(strat.Name),
			dimStyle.Render(strat.Type),
			strat.Trades,
			greenStyle.Render(fmt.Sprintf("%.1f%%", strat.WinRate)),
			strat.Sharpe,
			strat.MaxDrawdown))
	}

	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render("  │  ● Grid BTC: Running  │  ● DCA ETH: Running  │  ● Scalper: Paused        │"))
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderAIBrain() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    🤖 AI BRAIN ANALYSIS                          ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))

	for _, ai := range m.aiAnalysis {
		sentimentColor := greenStyle
		if ai.Sentiment == "Bearish" {
			sentimentColor = redStyle
		}
		signalColor := greenStyle
		if ai.Signal == "SELL" {
			signalColor = redStyle
		} else if ai.Signal == "HOLD" {
			signalColor = warningStyle
		}
		lines = append(lines, fmt.Sprintf(`  │  %s %-10s │ Sentiment: %s%-10s │ Signal: %s%-6s │ Confidence: %.0f%% │`,
			secondaryStyle.Render("🤖"),
			dimStyle.Render(ai.Symbol),
			sentimentColor.Render(ai.Sentiment),
			signalColor.Render(ai.Signal),
			cyanStyle.Render(fmt.Sprintf("%.0f%%", ai.Confidence))))
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  │  RSI: %s | MACD: %s | Bollinger: %s | Prediction: $%.2f │",
			dimStyle.Render(ai.Indicators["RSI"]),
			dimStyle.Render(ai.Indicators["MACD"]),
			dimStyle.Render(ai.Indicators["BB"]),
			ai.Prediction)))
	}

	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render("  │  Models: GPT-4 │ LLM │ Transformer │ Ensemble │ Active             │"))
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderRisk() string {
	r := m.riskMetrics
	expPct := (r.TotalExposure / r.MaxExposure) * 100

	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    ⚠️ RISK MANAGEMENT                            ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, fmt.Sprintf("  │  %s Total Exposure: $%-12.2f / $%-12.2f (%.1f%%)           │",
		cyanStyle.Render("📊"),
		r.TotalExposure,
		r.MaxExposure,
		expPct))
	lines = append(lines, fmt.Sprintf("  │  %s Current Drawdown: %s%-12.2f%% / %s%.2f%%                     │",
		warningStyle.Render("↘"),
		dimStyle.Render(""),
		r.CurrentDrawdown,
		dimStyle.Render(""),
		r.MaxDrawdown))
	lines = append(lines, fmt.Sprintf("  │  %s Daily Loss: %s%-12.2f%% / %s%.2f%%                          │",
		errorStyle.Render("▼"),
		dimStyle.Render(""),
		r.DailyLoss,
		dimStyle.Render(""),
		r.MaxDailyLoss))
	lines = append(lines, fmt.Sprintf("  │  %s Leverage: %.2fx  │  Margin Used: %.2f%%                          │",
		goldStyle.Render("⚡"),
		r.Leverage,
		r.MarginUsed))
	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render(fmt.Sprintf("  │  Exposure Bar: %s %s %d%%                          │",
		greenStyle.Render("█".Repeat(int(expPct/5))),
		dimStyle.Render("░".Repeat(20-int(expPct/5))),
		int(expPct))))
	lines = append(lines, dimStyle.Render("  │  Status: ● Normal - All risk limits within acceptable range         │"))
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderBacktest() string {
	b := m.backtestResult

	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    🔬 BACKTEST RESULTS                           ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, fmt.Sprintf("  │  Strategy: %-12s │ Symbol: %-10s │ Period: %s       │",
		goldStyle.Render(b.Strategy),
		dimStyle.Render(b.Symbol),
		dimStyle.Render(b.Period)))
	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, fmt.Sprintf("  │  Total Trades: %-6d  │  Win Rate: %s%-9.1f%%  │  Profit: %s$%-9.2f  │",
		b.TotalTrades,
		greenStyle.Render(""),
		b.WinRate,
		successStyle.Render(""),
		b.Profit))
	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, fmt.Sprintf("  │  %s Sharpe Ratio:  %-12.2f │  Sortino:  %-12.2f  │", cyanStyle.Render("📊"), b.Sharpe, b.Sortino))
	lines = append(lines, fmt.Sprintf("  │  %s Max Drawdown: %-12.2f%% │  Calmar:   %-12.2f  │", warningStyle.Render("↘"), b.MaxDrawdown, b.Calmar))
	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render("  │  Equity Curve: 📈📈📈📈📈📈📈📈📉 (90 days visualization)             │"))
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderExchanges() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    🔗 EXCHANGE CONNECTIONS                       ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))

	for _, ex := range m.exchangeStatus {
		statusColor := greenStyle
		statusIcon := "🟢"
		if ex.Status != "Connected" {
			statusColor = redStyle
			statusIcon = "🔴"
		}
		lines = append(lines, fmt.Sprintf("  │  %s %-15s %s %-12s │ Latency: %-5dms │ Symbols: %-5d │",
			cyanStyle.Render(exchangeIcon(ex.Name)),
			dimStyle.Render(ex.Name),
			statusIcon,
			statusColor.Render(ex.Status),
			ex.Latency,
			ex.Symbols))
	}

	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render("  │  Supported: CEX (Binance, Bybit, OKX, Kraken, Coinbase)            │"))
	lines = append(lines, dimStyle.Render("  │  Supported: DEX (Uniswap, PancakeSwap)                             │"))
	lines = append(lines, dimStyle.Render("  │  Supported: Stocks (Alpaca, Interactive Brokers, Tradier)           │"))
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderLogs() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    📋 SYSTEM LOGS                              ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))

	for _, log := range m.logs {
		lines = append(lines, fmt.Sprintf("  │  %s  %s  %-40s │",
			dimStyle.Render(log.Time.Format("15:04:05")),
			formatLogLevel(log.Level),
			dimStyle.Render(log.Message)))
	}

	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *model) renderSettings() string {
	var lines []string
	lines = append(lines, primaryStyle.Render("\n  ╔══════════════════════════════════════════════════════════════════╗"))
	lines = append(lines, primaryStyle.Render("  ║                    ⚙️ SETTINGS & CONFIGURATION                  ║"))
	lines = append(lines, primaryStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render("  │  Exchange Configuration                                            │"))
	lines = append(lines, dimStyle.Render("  │    ● Binance: Configured (API Key: ****1234)                       │"))
	lines = append(lines, dimStyle.Render("  │    ● Bybit: Configured (API Key: ****5678)                        │"))
	lines = append(lines, dimStyle.Render("  │    ● OKX: Not Configured                                          │"))
	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render("  │  AI Configuration                                                  │"))
	lines = append(lines, dimStyle.Render("  │    ● Provider: OpenAI                                              │"))
	lines = append(lines, dimStyle.Render("  │    ● Model: GPT-4                                                  │"))
	lines = append(lines, dimStyle.Render("  │    ● Cache TTL: 60s                                                │"))
	lines = append(lines, dimStyle.Render("  ╠══════════════════════════════════════════════════════════════════╣"))
	lines = append(lines, dimStyle.Render("  │  Risk Configuration                                                │"))
	lines = append(lines, dimStyle.Render("  │    ● Max Position: 10%                                              │"))
	lines = append(lines, dimStyle.Render("  │    ● Max Drawdown: 15%                                              │"))
	lines = append(lines, dimStyle.Render("  │    ● Max Daily Loss: 5%                                             │"))
	lines = append(lines, dimStyle.Render("  ╚══════════════════════════════════════════════════════════════════╝"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func formatVolume(v float64) string {
	if v >= 1e9 {
		return fmt.Sprintf("%.2fB", v/1e9)
	}
	if v >= 1e6 {
		return fmt.Sprintf("%.2fM", v/1e6)
	}
	if v >= 1e3 {
		return fmt.Sprintf("%.2fK", v/1e3)
	}
	return fmt.Sprintf("%.2f", v)
}

func formatLogLevel(level string) string {
	switch level {
	case "INFO":
		return infoStyle.Render("ℹ INFO ")
	case "WARN":
		return warningStyle.Render("⚠ WARN ")
	case "ERROR":
		return errorStyle.Render("✖ ERROR")
	case "DEBUG":
		return dimStyle.Render("• DEBUG")
	default:
		return dimStyle.Render(level)
	}
}

func exchangeIcon(name string) string {
	icons := map[string]string{
		"Binance":       "🔶",
		"Bybit":         "🟡",
		"OKX":           "⚪",
		"Kraken":        "🟣",
		"Hyperliquid":   "🔵",
		"Uniswap":       "🦄",
		"Alpaca":        "🦬",
		"Coinbase":      "🔵",
		"Deribit":       "🔴",
		"Interactive":    "🏦",
	}
	if icon, ok := icons[name]; ok {
		return icon
	}
	return "🔗"
}

func Run() error {
	model := NewModel()
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}
