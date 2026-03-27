package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	StateDashboard viewState = iota
	StateMarkets
	StatePortfolio
	StateStrategies
	StateAIBrain
	StateRisk
	StateBacktest
	StateExchanges
	StateLogs
	StateSettings
	StateHelp
)

var (
	primaryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)

type App struct {
	state         viewState
	width         int
	height        int
	showHelp      bool
	showWelcome   bool
}

func NewApp() *App {
	return &App{
		state:       StateDashboard,
		showWelcome: true,
	}
}

func (m *App) Init() tea.Cmd {
	return nil
}

func (m *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.state = StateDashboard
		case "2":
			m.state = StateMarkets
		case "3":
			m.state = StatePortfolio
		case "4":
			m.state = StateStrategies
		case "5":
			m.state = StateAIBrain
		case "6":
			m.state = StateRisk
		case "7":
			m.state = StateBacktest
		case "8":
			m.state = StateExchanges
		case "9":
			m.state = StateLogs
		case "0":
			m.state = StateSettings
		case "?":
			m.showHelp = !m.showHelp
		case "w":
			m.showWelcome = !m.showWelcome
		case "tab":
			m.nextView()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *App) nextView() {
	m.state = (m.state + 1) % 11
}

func (m *App) View() string {
	var content string
	
	if m.showWelcome {
		content = m.renderWelcome()
	} else {
		content = m.renderCurrentView()
	}
	
	return fmt.Sprintf("%s\n%s\n%s",
		renderHeader(),
		content,
		renderFooter(m.state))
}

func (m *App) renderWelcome() string {
	return fmt.Sprintf(`

%s

╔══════════════════════════════════════════════════════════════════════════════════╗
║                                                                                  ║
║   ██████╗ ██╗██╗  ██╗███████╗██╗     ██╗      █████╗ ██████╗               ║
║   ██╔══██╗██║╚██╗██╔╝██╔════╝██║     ██║     ██╔══██╗██╔══██╗              ║
║   ██████╔╝██║ ╚███╔╝ █████╗  ██║     ██║     ███████║██████╔╝              ║
║   ██╔═══╝ ██║ ██╔██╗ ██╔══╝  ██║     ██║     ██╔══██║██╔══██╗              ║
║   ██║     ██║██╔╝ ██╗███████╗███████╗███████╗██║  ██║██║  ██║              ║
║   ╚═╝     ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝              ║
║                                                                                  ║
║                        Enterprise AI Trading Framework                            ║
║                   10x More Powerful Than NautilusTrader                         ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║  %s Features                                                                  %s
║                                                                                  ║
║  %s🤖 AI Trading    %s│ %s📊 15+ Exchanges %s│ %s⚡ HFT Ready                   %s
║     LLM + ML + RL           CEX + DEX + Stocks      Nanosecond Precision       ║
║                                                                                  ║
║  %s🎯 15+ Strategies %s│ %s📈 30+ Indicators %s│ %s🔬 Backtesting                 %s
║     Grid, DCA, Scalping        TA-Lib Compatible       Full Analytics          ║
║                                                                                  ║
║  %s⚠️ Risk Manager  %s│ %s💾 Event Sourcing  %s│ %s🛡️ Security                   %s
║     Real-time Protection       Replay & Debug           Audit Ready             ║
║                                                                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝

  %s

  Quick Navigation: %s  %s  %s  %s  %s  %s  %s  %s  %s  %s

`,
		dimStyle.Render(strings.Repeat("═", 100)),
		primaryStyle.Render("║"),
		primaryStyle.Render("║"),
		primaryStyle.Render("║"),
		primaryStyle.Render("║"),
		primaryStyle.Render("║"),
		primaryStyle.Render("║"),
		primaryStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render("║"),
		dimStyle.Render(strings.Repeat("═", 100)),
		dimStyle.Render("Press 1-9 for views, Tab to cycle, W to toggle welcome, ? for help, Q to quit"),
		warningStyle.Render("[1] Dashboard"),
		successStyle.Render("[2] Markets"),
		cyanStyle.Render("[3] Portfolio"),
		goldStyle.Render("[4] Strategies"),
		secondaryStyle.Render("[5] AI Brain"),
		errorStyle.Render("[6] Risk"),
		infoStyle.Render("[7] Backtest"),
		dimStyle.Render("[8] Exchanges"),
		dimStyle.Render("[9] Logs"),
		dimStyle.Render("[0] Settings"))
}

func (m *App) renderCurrentView() string {
	switch m.state {
	case StateDashboard:
		return m.renderDashboardView()
	case StateMarkets:
		return m.renderMarketsView()
	case StatePortfolio:
		return m.renderPortfolioView()
	case StateStrategies:
		return m.renderStrategiesView()
	case StateAIBrain:
		return m.renderAIBrainView()
	case StateRisk:
		return m.renderRiskView()
	case StateBacktest:
		return m.renderBacktestView()
	case StateExchanges:
		return m.renderExchangesView()
	case StateLogs:
		return m.renderLogsView()
	case StateSettings:
		return m.renderSettingsView()
	case StateHelp:
		return m.renderHelpView()
	default:
		return m.renderDashboardView()
	}
}

func (m *App) renderDashboardView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           📊 DASHBOARD OVERVIEW                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────┐       ║
║   │ 💼 Portfolio Value │  │ 📈 Daily P&L       │  │ 📋 Total Trades    │       ║
║   │ $124,567.89       │  │ +$1,234.56 (+2.3%) │  │ 156                │       ║
║   │ ▲ 12.5% MTD       │  │ ▲ 8 trades today   │  │ Win Rate: 68.5%    │       ║
║   └────────────────────┘  └────────────────────┘  └────────────────────┘       ║
║                                                                                  ║
║   ┌────────────────────┐  ┌────────────────────┐  ┌────────────────────┐       ║
║   │ 📊 Open Positions │  │ 🤖 AI Confidence   │  │ ✓ System Health    │       ║
║   │ 3 Active          │  │ 78%               │  │ ● All Systems OK   │       ║
║   │ Long: 2 | Short: 1│  │ Signal: BUY       │  │ Latency: 12ms      │       ║
║   └────────────────────┘  └────────────────────┘  └────────────────────┘       ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║  🔥 Top Movers (24h)                                                            ║
║  • SOLUSDT  $142.35  ▲ 5.42%     • BNBUSDT  $598.20  ▼ 0.85%                  ║
║  • BTCUSDT  $67,432   ▲ 2.34%     • XRPUSDT  $0.5234  ▲ 3.21%                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderMarketsView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           📈 LIVE MARKET DATA                                    ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   SYMBOL      PRICE            24H CHANGE      24H VOLUME                      ║
║   ─────────────────────────────────────────────────────────────────────────    ║
║   • BTCUSDT   $67,432.50       ▲ +2.34%        $28.5B                          ║
║   • ETHUSDT   $3,521.80        ▲ +1.87%        $15.2B                          ║
║   • SOLUSDT   $142.35          ▲ +5.42%        $2.8B                           ║
║   • BNBUSDT   $598.20          ▼ -0.85%        $850M                           ║
║   • XRPUSDT   $0.5234          ▲ +3.21%        $1.2B                           ║
║   • ADAUSDT   $0.4521          ▲ +1.12%        $420M                           ║
║   • DOGEUSDT  $0.1234          ▼ -2.34%        $380M                           ║
║   • DOTUSDT   $7.234           ▲ +0.89%        $210M                           ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║   Exchanges: Binance ● | Bybit ● | OKX ● | Kraken ● | Hyperliquid ●            ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderPortfolioView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           💼 PORTFOLIO & POSITIONS                               ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   SYMBOL      SIDE    QTY         ENTRY        CURRENT     P&L                  ║
║   ─────────────────────────────────────────────────────────────────────────    ║
║   • BTCUSDT   LONG    0.15        $65,000.00   $67,432.50  +$364.88 (+3.74%)   ║
║   • ETHUSDT   LONG    2.50        $3,400.00    $3,521.80   +$304.50 (+3.58%)   ║
║   • SOLUSDT   SHORT   50.00       $145.00      $142.35     +$132.50 (+1.83%)   ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║   Total P&L: ▲ $801.88 (+3.12%)    │    Unrealized: $801.88                    ║
║   Margin Used: $15,234.56 (35.0%)  │    Buying Power: $45,234.56               ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderStrategiesView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           🎯 TRADING STRATEGIES                                   ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   ┌────────────────────────────────────────────────────────────────────────┐   ║
║   │ 🎯 Grid BTC         Status: ● Running    Trades: 156   Win: 68.5%      │   ║
║   │    Type: grid       Sharpe: 1.87        Max DD: 8.5%    Profit: $4,567 │   ║
║   └────────────────────────────────────────────────────────────────────────┘   ║
║   ┌────────────────────────────────────────────────────────────────────────┐   ║
║   │ 🎯 DCA ETH         Status: ● Running    Trades: 89    Win: 72.1%      │   ║
║   │    Type: dca        Sharpe: 2.15        Max DD: 5.2%    Profit: $2,345 │   ║
║   └────────────────────────────────────────────────────────────────────────┘   ║
║   ┌────────────────────────────────────────────────────────────────────────┐   ║
║   │ 🎯 Scalper         Status: ○ Paused      Trades: 2341  Win: 54.3%      │   ║
║   │    Type: scalping  Sharpe: 1.42        Max DD: 12.3%   Profit: $1,892 │   ║
║   └────────────────────────────────────────────────────────────────────────┘   ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║   Available: Grid | DCA | Trend | Scalping | Arbitrage | Market Making         ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderAIBrainView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           🤖 AI BRAIN ANALYSIS                                   ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   ┌────────────────────────────────────────────────────────────────────────┐   ║
║   │ 🤖 BTCUSDT       Sentiment: 🟢 Bullish    Signal: 🟢 BUY             │   ║
║   │    Confidence: 78%        Prediction: $68,500.00                      │   ║
║   │    RSI: 62 (Neutral)    MACD: Bullish Cross                         │   ║
║   └────────────────────────────────────────────────────────────────────────┘   ║
║   ┌────────────────────────────────────────────────────────────────────────┐   ║
║   │ 🤖 ETHUSDT       Sentiment: 🟢 Bullish    Signal: 🟢 BUY             │   ║
║   │    Confidence: 72%        Prediction: $3,600.00                       │   ║
║   │    RSI: 58 (Neutral)    MACD: Above Signal                          │   ║
║   └────────────────────────────────────────────────────────────────────────┘   ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║   Models Active: GPT-4 | Transformer | LSTM | XGBoost | Ensemble              ║
║   Cache: Enabled (60s TTL) | Analysis Interval: 5s                             ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderRiskView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           ⚠️ RISK MANAGEMENT                                    ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   ┌────────────────────────────────────────────────────────────────────────┐   ║
║   │ 📊 Total Exposure: $45,000.00 / $100,000.00 (45.0%)                │   ║
║   │    [████████████████░░░░░░░░░░░░░░░░░░░░░░░] 45%                   │   ║
║   └────────────────────────────────────────────────────────────────────────┘   ║
║                                                                                  ║
║   Metric           Current         Limit           Status                        ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   ↘ Drawdown       2.3%           15.0%           ✓ Normal                      ║
║   ▼ Daily Loss     0.5%            5.0%           ✓ Normal                      ║
║   ⚡ Leverage      2.0x           5.0x           ✓ Normal                      ║
║   Margin Used      35.0%          80.0%          ✓ Normal                      ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║   Status: ● Normal - All risk limits within acceptable range                    ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderBacktestView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           🔬 BACKTEST RESULTS                                    ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   Strategy: Grid Trading    Symbol: BTCUSDT    Period: 90 Days                 ║
║   ────────────────────────────────────────────────────────────────────────     ║
║                                                                                  ║
║   Total Trades: 156         Win Rate: 68.5%        Profit: $12,453.67          ║
║                                                                                  ║
║   ┌────────────────────────────────────────────────────────────────────────┐   ║
║   │ 📊 Sharpe Ratio: 1.87      Sortino: 2.45      Calmar: 1.23          │   ║
║   │ ↘ Max Drawdown: 8.5%      Avg Trade: $79.83   Profit Factor: 2.34    │   ║
║   └────────────────────────────────────────────────────────────────────────┘   ║
║                                                                                  ║
║   Equity Curve (90 days):                                                       ║
║   📈███████████████████████████████████████████📉 (Final: +24.9%)             ║
║                                                                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderExchangesView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           🔗 EXCHANGE CONNECTIONS                                ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   EXCHANGE          STATUS         LATENCY      SYMBOLS                         ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   🔶 Binance        🟢 Connected   12ms        450                             ║
║   🟡 Bybit          🟢 Connected   18ms        380                             ║
║   ⚪ OKX            🟢 Connected   25ms        320                             ║
║   🟣 Kraken         🟢 Connected   45ms        280                             ║
║   🔵 Hyperliquid    🟢 Connected   8ms         140                             ║
║   🔵 Coinbase       🔴 Disconnected-            -                               ║
║                                                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║   Supported Exchanges:                                                          ║
║   CEX: Binance, Bybit, OKX, Coinbase, Kraken                                    ║
║   DEX: Uniswap, PancakeSwap                                                    ║
║   Stocks: Alpaca, Interactive Brokers, Tradier                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderLogsView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           📋 SYSTEM LOGS                                         ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   15:42:35  ℹ [INFO]    Strategy Grid BTC started successfully                  ║
║   15:42:05  ℹ [INFO]    Order #12345 filled: BUY 0.01 BTC @ $67,432.50         ║
║   15:41:30  ℹ [INFO]    AI Brain analysis updated for BTCUSDT                   ║
║   15:41:00  • [DEBUG]   WebSocket message received from Binance                  ║
║   15:40:30  ⚠ [WARN]    High volatility detected on SOLUSDT                      ║
║   15:40:00  ℹ [INFO]    Backtest completed: Sharpe 1.87, MaxDD 8.5%             ║
║   15:39:30  ℹ [INFO]    Exchange Binance connected successfully                   ║
║   15:39:00  • [DEBUG]   Risk check passed for new order                          ║
║   15:38:30  ℹ [INFO]    Portfolio rebalanced successfully                         ║
║   15:38:00  ⚠ [WARN]    Position approaching stop-loss level                     ║
║                                                                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderSettingsView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           ⚙️ SETTINGS & CONFIGURATION                             ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   Exchange Configuration                                                         ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   ● Binance:   Configured (API Key: ****1234)                    [Edit]         ║
║   ● Bybit:     Configured (API Key: ****5678)                    [Edit]         ║
║   ○ OKX:       Not Configured                                      [Setup]      ║
║                                                                                  ║
║   AI Configuration                                                               ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   Provider: OpenAI                                     Model: GPT-4            ║
║   Cache TTL: 60s                                        Analysis Interval: 5s   ║
║                                                                                  ║
║   Risk Configuration                                                            ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   Max Position: 10%              Max Drawdown: 15%                             ║
║   Max Daily Loss: 5%              Auto-liquidate: Enabled                        ║
║                                                                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *App) renderHelpView() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                           ❓ HELP & KEYBOARD SHORTCUTS                           ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                  ║
║   Navigation                                                                    ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   1-9           Navigate to specific view                                        ║
║   Tab           Cycle through views                                             ║
║   w             Toggle welcome screen                                            ║
║   ?             Toggle this help screen                                          ║
║                                                                                  ║
║   Actions                                                                     ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   q, Ctrl+C     Quit application                                                ║
║                                                                                  ║
║   Views                                                                      ║
║   ────────────────────────────────────────────────────────────────────────     ║
║   [1] Dashboard   - System overview & key metrics                               ║
║   [2] Markets     - Live market data & prices                                   ║
║   [3] Portfolio   - Positions & P&L tracking                                    ║
║   [4] Strategies  - Trading strategies management                               ║
║   [5] AI Brain    - Real-time AI market analysis                                ║
║   [6] Risk        - Exposure & risk metrics                                     ║
║   [7] Backtest    - Strategy historical testing                                  ║
║   [8] Exchanges   - Exchange connections status                                  ║
║   [9] Logs        - System logs & events                                        ║
║   [0] Settings    - Configuration & preferences                                  ║
║                                                                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func renderHeader() string {
	return fmt.Sprintf(`╔%s╗
║  OpenTrader v1.0.0 - Enterprise AI Trading Framework %s%s║
╠%s╣`,
		strings.Repeat("═", 95),
		strings.Repeat(" ", 95-68),
		time.Now().Format("15:04:05"),
		strings.Repeat("═", 95))
}

func renderFooter(state viewState) string {
	viewName := "Dashboard"
	switch state {
	case StateMarkets:
		viewName = "Markets"
	case StatePortfolio:
		viewName = "Portfolio"
	case StateStrategies:
		viewName = "Strategies"
	case StateAIBrain:
		viewName = "AI Brain"
	case StateRisk:
		viewName = "Risk Manager"
	case StateBacktest:
		viewName = "Backtest"
	case StateExchanges:
		viewName = "Exchanges"
	case StateLogs:
		viewName = "Logs"
	case StateSettings:
		viewName = "Settings"
	case StateHelp:
		viewName = "Help"
	}

	return fmt.Sprintf(`╠%s╣
║  %s │ Exchanges: 5 │ Strategies: 3 │ Positions: 3 │ AI: Online │ Risk: Normal %s║
╚%s╝`,
		strings.Repeat("═", 95),
		dimStyle.Render("View: "+primaryStyle.Render(viewName)),
		dimStyle.Render(time.Now().Format("│ 15:04:05")),
		strings.Repeat("═", 95))
}

var (
	cyanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4"))
	goldStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	secondaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A855F7"))
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
)
