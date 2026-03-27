package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opentreder/opentreder/pkg/logger"
)

var (
	borderStyle = lipgloss.RoundedBorder()
	focusStyle  = lipgloss.AdaptiveColor{Light: "12", Dark: "10"}

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#1E1E1E")).
			Bold(true)

	selectedRowStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2D2D2D")).
			Foreground(lipgloss.Color("#FAFAFA"))

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

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#1E1E1E")).
			Bold(true).
			Padding(0, 1)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))
)

type View int

const (
	ViewDashboard View = iota
	ViewPortfolio
	ViewPositions
	ViewOrders
	ViewTrades
	ViewStrategies
	ViewExchanges
	ViewMarketData
	ViewSignals
	ViewAI
	ViewRisk
	ViewSettings
	ViewHelp
)

type Model struct {
	ctx        context.Context
	cancel     context.CancelFunc
	width      int
	height     int
	currentView View
	previousView View
	views      map[View]string
	components map[View]Component
	commandHistory []string
	commandIndex   int
	currentCommand string
	outputBuffer   []string
	statusMessage  string
	refreshInterval time.Duration
	lastRefresh    time.Time
	data      *AppData
	quitting  bool
	mu        sync.RWMutex
}

type Component interface {
	Title() string
	Help() []string
	Render() string
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
}

type AppData struct {
	Portfolio  PortfolioData
	Positions  []PositionRow
	Orders     []OrderRow
	Trades     []TradeRow
	Strategies []StrategyRow
	Exchanges  []ExchangeRow
	Balances   []BalanceRow
	Signals    []SignalRow
	Risk       RiskData
	LastUpdate time.Time
}

type PortfolioData struct {
	TotalValue     string
	DayPnL         string
	DayPnLPercent  string
	TotalPnL       string
	TotalPnLPercent string
	CashBalance    string
	MarginUsed     string
}

type PositionRow struct {
	ID        string
	Symbol    string
	Side      string
	Quantity  string
	EntryPrice string
	MarkPrice  string
	PnL       string
	PnLPercent string
	Leverage  string
}

type OrderRow struct {
	ID        string
	Symbol    string
	Side      string
	Type      string
	Status    string
	Price     string
	Quantity  string
	FilledQty string
	Time      string
}

type TradeRow struct {
	ID        string
	Symbol    string
	Side      string
	Price     string
	Quantity  string
	Commission string
	Time      string
}

type StrategyRow struct {
	ID      string
	Name    string
	Type    string
	Status  string
	Trades  int
	PnL     string
	WinRate string
	State   string
}

type ExchangeRow struct {
	Name       string
	Status     string
	Latency    string
	Connected  string
	LastUpdate string
}

type BalanceRow struct {
	Asset     string
	Free      string
	Locked    string
	Total     string
	USDValue  string
}

type SignalRow struct {
	ID        string
	Strategy  string
	Symbol    string
	Action    string
	Strength  string
	Confidence string
	Reason    string
	Time      string
}

type RiskData struct {
	MaxPositionSize  string
	MaxDailyLoss     string
	MaxDrawdown      string
	MaxExposure      string
	MaxLeverage      string
	CurrentExposure  string
	CurrentDrawdown  string
}

type Command struct {
	Name        string
	Description string
	Shortcut    string
	Category    string
	Handler     func(args []string) error
}

var commands = []Command{
	{"run", "Run trading engine", "r", "Trading", nil},
	{"stop", "Stop trading engine", "s", "Trading", nil},
	{"backtest", "Run backtest", "b", "Trading", nil},
	{"portfolio", "Show portfolio", "p", "Portfolio", nil},
	{"positions", "Show positions", "o", "Portfolio", nil},
	{"orders", "Show orders", "O", "Trading", nil},
	{"trades", "Show trades", "t", "Trading", nil},
	{"buy", "Place buy order", "", "Trading", nil},
	{"sell", "Place sell order", "", "Trading", nil},
	{"cancel", "Cancel order", "c", "Trading", nil},
	{"balance", "Show balances", "B", "Portfolio", nil},
	{"strategy", "Manage strategies", "S", "Strategies", nil},
	{"exchange", "Manage exchanges", "e", "System", nil},
	{"connect", "Connect exchange", "", "System", nil},
	{"disconnect", "Disconnect exchange", "", "System", nil},
	{"status", "System status", "i", "System", nil},
	{"risk", "Risk management", "R", "Risk", nil},
	{"ai", "AI commands", "a", "AI", nil},
	{"analyze", "Analyze market", "", "AI", nil},
	{"chat", "Chat with AI", "", "AI", nil},
	{"signal", "Generate signal", "", "AI", nil},
	{"config", "Configuration", "C", "System", nil},
	{"set", "Set config value", "", "System", nil},
	{"get", "Get config value", "", "System", nil},
	{"help", "Show help", "?", "System", nil},
	{"clear", "Clear screen", "l", "System", nil},
	{"exit", "Exit application", "q", "System", nil},
}

func NewApp() *Model {
	ctx, cancel := context.WithCancel(context.Background())
	
	m := &Model{
		ctx:      ctx,
		cancel:   cancel,
		views:    make(map[View]string),
		components: make(map[View]Component),
		refreshInterval: time.Second,
		lastRefresh: time.Now(),
	}

	m.components[ViewDashboard] = NewDashboard(m)
	m.components[ViewPortfolio] = NewPortfolio(m)
	m.components[ViewPositions] = NewPositions(m)
	m.components[ViewOrders] = NewOrders(m)
	m.components[ViewTrades] = NewTrades(m)
	m.components[ViewStrategies] = NewStrategies(m)
	m.components[ViewExchanges] = NewExchanges(m)
	m.components[ViewMarketData] = NewMarketData(m)
	m.components[ViewSignals] = NewSignals(m)
	m.components[ViewAI] = NewAI(m)
	m.components[ViewRisk] = NewRisk(m)
	m.components[ViewSettings] = NewSettings(m)

	m.data = m.loadInitialData()

	return m
}

func (m *Model) loadInitialData() *AppData {
	return &AppData{
		Portfolio: PortfolioData{
			TotalValue:      "$144,135.68",
			DayPnL:          "+$1,234.56",
			DayPnLPercent:   "+0.86%",
			TotalPnL:        "+$44,135.68",
			TotalPnLPercent: "+44.14%",
			CashBalance:     "$51,000.00",
			MarginUsed:      "$5,000.00",
		},
		Positions: []PositionRow{
			{"1", "BTC/USDT", "LONG", "0.5", "$43,500", "$44,135", "+$317.50", "+1.46%", "3x"},
			{"2", "ETH/USDT", "LONG", "5.0", "$2,350", "$2,380", "+$150.00", "+1.28%", "2x"},
			{"3", "SOL/USDT", "SHORT", "100", "$98.50", "$97.20", "+$130.00", "+1.32%", "2x"},
			{"4", "BNB/USDT", "LONG", "10.0", "$285", "$292", "+$70.00", "+2.46%", "1x"},
			{"5", "XRP/USDT", "LONG", "5000", "$0.52", "$0.51", "-$50.00", "-1.92%", "1x"},
		},
		Orders: []OrderRow{
			{ID: "ord_001", Symbol: "BTC/USDT", Side: "BUY", Type: "LIMIT", Status: "OPEN", Price: "$44,000", Quantity: "0.1", FilledQty: "0.0", Time: "10:30:45"},
			{ID: "ord_002", Symbol: "ETH/USDT", Side: "SELL", Type: "MARKET", Status: "FILLED", Price: "$2,380", Quantity: "2.0", FilledQty: "2.0", Time: "10:25:30"},
			{ID: "ord_003", Symbol: "SOL/USDT", Side: "BUY", Type: "STOP", Status: "OPEN", Price: "$95.00", Quantity: "50.0", FilledQty: "0.0", Time: "10:20:15"},
			{ID: "ord_004", Symbol: "BNB/USDT", Side: "BUY", Type: "LIMIT", Status: "OPEN", Price: "$290", Quantity: "5.0", FilledQty: "0.0", Time: "10:15:00"},
			{ID: "ord_005", Symbol: "XRP/USDT", Side: "SELL", Type: "MARKET", Status: "PARTIAL", Price: "$0.51", Quantity: "1000", FilledQty: "500", Time: "10:10:45"},
		},
		Trades: []TradeRow{
			{"trd_001", "ETH/USDT", "BUY", "$2,380", "2.0", "$0.48", "10:25:30"},
			{"trd_002", "BNB/USDT", "BUY", "$292", "10.0", "$2.92", "10:20:00"},
			{"trd_003", "BTC/USDT", "BUY", "$44,135", "0.5", "$22.07", "10:15:45"},
			{"trd_004", "SOL/USDT", "SELL", "$97.20", "100", "$9.72", "10:10:30"},
			{"trd_005", "XRP/USDT", "SELL", "$0.51", "500", "$0.26", "10:05:15"},
		},
		Strategies: []StrategyRow{
			{ID: "str_001", Name: "Grid BTC", Type: "grid", Status: "ACTIVE", Trades: 45, PnL: "+$1,234.56", WinRate: "68%", State: "ACTIVE"},
			{ID: "str_002", Name: "DCA ETH", Type: "dca", Status: "ACTIVE", Trades: 120, PnL: "+$456.78", WinRate: "72%", State: "ACTIVE"},
			{ID: "str_003", Name: "Trend SOL", Type: "trend", Status: "PAUSED", Trades: 23, PnL: "-$123.45", WinRate: "52%", State: "PAUSED"},
			{ID: "str_004", Name: "Scalper", Type: "scalping", Status: "ACTIVE", Trades: 890, PnL: "+$2,345.67", WinRate: "61%", State: "ACTIVE"},
			{ID: "str_005", Name: "Arbitrage", Type: "arbitrage", Status: "DISABLED", Trades: 0, PnL: "$0.00", WinRate: "0%", State: "DISABLED"},
		},
		Exchanges: []ExchangeRow{
			{"Binance", "Connected", "12ms", "Yes", "2s ago"},
			{"Bybit", "Connected", "23ms", "Yes", "2s ago"},
			{"Coinbase", "Disconnected", "-", "No", "Never"},
			{"Kraken", "Disconnected", "-", "No", "Never"},
		},
		Balances: []BalanceRow{
			{"BTC", "1.2345", "0.0000", "1.2345", "$54,478.91"},
			{"ETH", "15.678", "0.0000", "15.678", "$37,313.64"},
			{"USDT", "50,000.00", "1,000.00", "51,000.00", "$51,000.00"},
			{"BNB", "50.00", "0.00", "50.00", "$14,600.00"},
			{"SOL", "500.00", "0.00", "500.00", "$48,600.00"},
		},
		Signals: []SignalRow{
			{"sig_001", "Grid BTC", "BTC/USDT", "BUY", "Strong", "85%", "RSI oversold", "10:30:00"},
			{"sig_002", "Trend SOL", "SOL/USDT", "SELL", "Medium", "62%", "Trend reversal", "10:25:00"},
			{"sig_003", "AI Model", "ETH/USDT", "BUY", "Strong", "78%", "ML prediction", "10:20:00"},
			{"sig_004", "Scalper", "BNB/USDT", "BUY", "Weak", "55%", "Mean reversion", "10:15:00"},
		},
		Risk: RiskData{
			MaxPositionSize: "100%",
			MaxDailyLoss:    "10%",
			MaxDrawdown:     "20%",
			MaxExposure:     "80%",
			MaxLeverage:     "3x",
			CurrentExposure: "45%",
			CurrentDrawdown: "3.2%",
		},
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tickMsg:
		m.lastRefresh = time.Now()
		return m, m.tick()

	case statusMsg:
		m.statusMessage = string(msg)
		return m, nil

	case dataMsg:
		m.data = msg
		return m, nil
	}

	return m, nil
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.currentView = ViewHelp

	case "1":
		m.previousView = m.currentView
		m.currentView = ViewDashboard
	case "2":
		m.previousView = m.currentView
		m.currentView = ViewPortfolio
	case "3":
		m.previousView = m.currentView
		m.currentView = ViewPositions
	case "4":
		m.previousView = m.currentView
		m.currentView = ViewOrders
	case "5":
		m.previousView = m.currentView
		m.currentView = ViewTrades
	case "6":
		m.previousView = m.currentView
		m.currentView = ViewStrategies
	case "7":
		m.previousView = m.currentView
		m.currentView = ViewExchanges
	case "8":
		m.previousView = m.currentView
		m.currentView = ViewMarketData
	case "9":
		m.previousView = m.currentView
		m.currentView = ViewSignals
	case "0":
		m.previousView = m.currentView
		m.currentView = ViewAI
	case "-":
		m.previousView = m.currentView
		m.currentView = ViewRisk

	case "b":
		m.previousView = m.currentView
		m.currentView = ViewSettings

	case "l":
		return m, nil

	case "r":
		return m, func() tea.Msg {
			return statusMsg("Running trading engine...")
		}

	case "s":
		return m, func() tea.Msg {
			return statusMsg("Stopping trading engine...")
		}

	case "tab":
		m.previousView = m.currentView
		m.currentView = (m.currentView + 1) % 12
		if m.currentView == ViewHelp {
			m.currentView = ViewDashboard
		}

	case "shift+tab":
		m.previousView = m.currentView
		m.currentView--
		if m.currentView < 0 {
			m.currentView = ViewRisk
		}

	case "esc":
		if m.currentView == ViewHelp {
			m.currentView = m.previousView
		}
	}

	return m, nil
}

func (m *Model) View() string {
	if m.quitting {
		return m.quitScreen()
	}

	var sb strings.Builder

	sb.WriteString(m.header())
	sb.WriteString("\n")

	view := m.components[m.currentView]
	if view != nil {
		sb.WriteString(view.Render())
	} else {
		sb.WriteString(m.dashboardView())
	}

	sb.WriteString(m.footer())

	return sb.String()
}

func (m *Model) header() string {
	header := fmt.Sprintf(`╔══════════════════════════════════════════════════════════════════════════════════╗
║  🚀 OpenTrader  v1.0.0                                    %s ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║  [%d]Dashboard [%d]Portfolio [%d]Positions [%d]Orders [%d]Trades [%d]Strategies               ║
║  [%d]Exchanges [%d]Market [%d]Signals [%d]AI [%d]Risk [%d]Settings [%d]Help                    ║
╚══════════════════════════════════════════════════════════════════════════════════╝
`,
		time.Now().Format("15:04:05"),
		ViewDashboard, ViewPortfolio, ViewPositions, ViewOrders, ViewTrades, ViewStrategies,
		ViewExchanges, ViewMarketData, ViewSignals, ViewAI, ViewRisk, ViewSettings, ViewHelp,
	)

	if m.statusMessage != "" {
		header += fmt.Sprintf("📌 %s\n\n", m.statusMessage)
	}

	return header
}

func (m *Model) dashboardView() string {
	var sb strings.Builder
	d := m.data

	cols := 4
	colWidth := (m.width - 20) / cols

	_ = func(title, value, color string) string {
		return fmt.Sprintf("%s%s%s│",
			lipgloss.NewStyle().
				Width(colWidth).
				Render(fmt.Sprintf("┌─%s─┐", strings.Repeat("─", colWidth-2-len(title)))),
			lipgloss.NewStyle().
				Width(colWidth).
				Bold(true).
				Render(fmt.Sprintf("│ %s\n│ %s", title, value)),
			lipgloss.NewStyle().
				Width(colWidth).
				Render("│"),
		)
	}

	sb.WriteString("┌─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Total Value: %s   │ Day P&L: %s      │ Positions: %d       │ Exposure: %s        │\n",
		d.Portfolio.TotalValue, d.Portfolio.DayPnL, len(d.Positions), d.Risk.CurrentExposure))
	sb.WriteString(fmt.Sprintf("│ Cash: %s       │ Win Rate: %s      │ Orders: %d           │ Drawdown: %s       │\n",
		d.Portfolio.CashBalance, "64%", len(d.Orders), d.Risk.CurrentDrawdown))
	sb.WriteString("└─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┘\n\n")

	sb.WriteString("┌─ POSITIONS ────────────────────────────────────────┬─ RECENT TRADES ──────────────────────┐\n")
	for i := 0; i < 5; i++ {
		pos := ""
		trade := ""
		if i < len(d.Positions) {
			p := d.Positions[i]
			pos = fmt.Sprintf("%s %s %s @ %s | PnL: %s (%s)", p.Symbol, p.Side, p.Quantity, p.MarkPrice, p.PnL, p.PnLPercent)
		}
		if i < len(d.Trades) {
			t := d.Trades[i]
			trade = fmt.Sprintf("%s %s %s @ %s", t.Symbol, t.Side, t.Quantity, t.Price)
		}
		sb.WriteString(fmt.Sprintf("│ %-46s │ %-35s │\n", pos, trade))
	}
	sb.WriteString("└────────────────────────────────────────────────────┴─────────────────────────────────────┘\n")

	return sb.String()
}

func (m *Model) footer() string {
	view := m.components[m.currentView]
	_ = []string{"?"}
	if view != nil {
		_ = view.Help()
	}

	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║  Commands: [r]un [s]top [b]acktest [o]rders [t]rades [S]trategies [C]onfig        ║
║  Views: [1-9][0] Navigate │ [tab] Next │ [esc] Back │ [l] Clear │ [q] Quit      ║
╚══════════════════════════════════════════════════════════════════════════════════╝`)
}

func (m *Model) quitScreen() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════╗
║                                                                                  ║
║                      🙏 Thanks for using OpenTrader!                              ║
║                                                                                  ║
║                         Version: %s                                               ║
║                         Uptime: %s                                                ║
║                                                                                  ║
║                    https://github.com/opentreder/opentreder                       ║
║                                                                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝
`, version, time.Since(time.Now().Add(-2*time.Hour)).Round(time.Second))
}

type tickMsg time.Time
type statusMsg string
type dataMsg *AppData

func (m *Model) tick() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(m.refreshInterval)
		return tickMsg(time.Now())
	}
}

func (m *Model) runCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	args := parts[1:]

	for _, c := range commands {
		if c.Name == command || c.Shortcut == command {
			if c.Handler != nil {
				return c.Handler(args)
			}
			logger.Info("Executing command", "command", command, "args", args)
			return nil
		}
	}

	return fmt.Errorf("unknown command: %s", command)
}

func (m *Model) refreshData() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data.LastUpdate = time.Now()
}

type Dashboard struct {
	model *Model
}

func NewDashboard(m *Model) *Dashboard {
	return &Dashboard{model: m}
}

func (d *Dashboard) Title() string { return "Dashboard" }
func (d *Dashboard) Help() []string { return []string{"r: refresh"} }

func (d *Dashboard) Render() string {
	return d.model.dashboardView()
}

func (d *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return d.model, nil }

type Portfolio struct{ model *Model }

func NewPortfolio(m *Model) *Portfolio { return &Portfolio{model: m} }
func (p *Portfolio) Title() string     { return "Portfolio" }
func (p *Portfolio) Help() []string   { return []string{} }

func (p *Portfolio) Render() string {
	var sb strings.Builder
	d := p.model.data

	sb.WriteString("┌─ PORTFOLIO ────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString(fmt.Sprintf("│ Total Value:       %-20s  Cash Balance:     %-20s       │\n", d.Portfolio.TotalValue, d.Portfolio.CashBalance))
	sb.WriteString(fmt.Sprintf("│ Day P&L:           %-20s  Margin Used:     %-20s       │\n", d.Portfolio.DayPnL+" ("+d.Portfolio.DayPnLPercent+")", d.Portfolio.MarginUsed))
	sb.WriteString(fmt.Sprintf("│ Total P&L:         %-20s                                 │\n", d.Portfolio.TotalPnL+" ("+d.Portfolio.TotalPnLPercent+")"))
	sb.WriteString("├───────────────────────────────────────────────────────────────────────────────┤\n")
	sb.WriteString("│                              BALANCES                                          │\n")
	sb.WriteString("├───────────┬───────────────┬───────────────┬───────────────┬───────────────────┤\n")
	sb.WriteString("│ Asset     │ Free          │ Locked        │ Total         │ USD Value         │\n")
	sb.WriteString("├───────────┼───────────────┼───────────────┼───────────────┼───────────────────┤\n")
	for _, b := range d.Balances {
		sb.WriteString(fmt.Sprintf("│ %-9s │ %-13s │ %-13s │ %-13s │ %-17s │\n", b.Asset, b.Free, b.Locked, b.Total, b.USDValue))
	}
	sb.WriteString("└───────────┴───────────────┴───────────────┴───────────────┴───────────────────┘\n")

	return sb.String()
}

func (p *Portfolio) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return p.model, nil }

type Positions struct{ model *Model }

func NewPositions(m *Model) *Positions { return &Positions{model: m} }
func (p *Positions) Title() string     { return "Positions" }
func (p *Positions) Help() []string    { return []string{"Enter: details"} }

func (p *Positions) Render() string {
	var sb strings.Builder
	d := p.model.data

	sb.WriteString("┌─ POSITIONS ─────────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ ID  │ Symbol     │ Side   │ Qty      │ Entry     │ Mark      │ PnL       │ Lvrg │\n")
	sb.WriteString("├─────┼────────────┼────────┼──────────┼───────────┼───────────┼───────────┼──────┤\n")
	for _, pos := range d.Positions {
		sb.WriteString(fmt.Sprintf("│ %-3s │ %-10s │ %-6s │ %-8s │ %-9s │ %-9s │ %-9s │ %-4s │\n",
			pos.ID, pos.Symbol, pos.Side, pos.Quantity, pos.EntryPrice, pos.MarkPrice, pos.PnL, pos.Leverage))
	}
	sb.WriteString("└─────┴────────────┴────────┴──────────┴───────────┴───────────┴───────────┴──────┘\n")

	return sb.String()
}

func (p *Positions) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return p.model, nil }

type Orders struct{ model *Model }

func NewOrders(m *Model) *Orders { return &Orders{model: m} }
func (o *Orders) Title() string  { return "Orders" }
func (o *Orders) Help() []string { return []string{"Enter: details", "c: cancel"} }

func (o *Orders) Render() string {
	var sb strings.Builder
	d := o.model.data

	sb.WriteString("┌─ ORDERS ─────────────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ ID       │ Symbol     │ Side │ Type   │ Status   │ Price    │ Qty     │ Filled │\n")
	sb.WriteString("├──────────┼────────────┼──────┼────────┼──────────┼──────────┼─────────┼────────┤\n")
	for _, ord := range d.Orders {
		status := ord.Status
		if status == "OPEN" {
			status = "⚠️ " + status
		} else if status == "FILLED" {
			status = "✅ " + status
		}
		sb.WriteString(fmt.Sprintf("│ %-8s │ %-10s │ %-4s │ %-6s │ %-8s │ %-8s │ %-7s │ %-6s │\n",
			ord.ID, ord.Symbol, ord.Side, ord.Type, status, ord.Price, ord.Quantity, ord.FilledQty))
	}
	sb.WriteString("└──────────┴────────────┴──────┴────────┴──────────┴──────────┴─────────┴────────┘\n")

	return sb.String()
}

func (o *Orders) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return o.model, nil }

type Trades struct{ model *Model }

func NewTrades(m *Model) *Trades { return &Trades{model: m} }
func (t *Trades) Title() string   { return "Trades" }
func (t *Trades) Help() []string  { return []string{} }

func (t *Trades) Render() string {
	var sb strings.Builder
	d := t.model.data

	sb.WriteString("┌─ RECENT TRADES ─────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ ID       │ Symbol     │ Side │ Price    │ Quantity  │ Commission │ Time     │\n")
	sb.WriteString("├──────────┼────────────┼──────┼──────────┼───────────┼────────────┼──────────┤\n")
	for _, trd := range d.Trades {
		sb.WriteString(fmt.Sprintf("│ %-8s │ %-10s │ %-4s │ %-8s │ %-9s │ %-10s │ %-8s │\n",
			trd.ID, trd.Symbol, trd.Side, trd.Price, trd.Quantity, trd.Commission, trd.Time))
	}
	sb.WriteString("└──────────┴────────────┴──────┴──────────┴───────────┴────────────┴──────────┘\n")

	return sb.String()
}

func (t *Trades) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return t.model, nil }

type Strategies struct{ model *Model }

func NewStrategies(m *Model) *Strategies { return &Strategies{model: m} }
func (s *Strategies) Title() string       { return "Strategies" }
func (s *Strategies) Help() []string      { return []string{"Enter: toggle"} }

func (s *Strategies) Render() string {
	var sb strings.Builder
	d := s.model.data

	sb.WriteString("┌─ STRATEGIES ───────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ ID       │ Name        │ Type     │ Status   │ Trades │ PnL        │ Win Rate │\n")
	sb.WriteString("├──────────┼─────────────┼──────────┼──────────┼────────┼────────────┼──────────┤\n")
	for _, strat := range d.Strategies {
		status := strat.Status
		if status == "ACTIVE" {
			status = "🟢 " + status
		} else if status == "PAUSED" {
			status = "🟡 " + status
		} else {
			status = "⚫ " + status
		}
		sb.WriteString(fmt.Sprintf("│ %-8s │ %-11s │ %-8s │ %-8s │ %-6d │ %-10s │ %-8s │\n",
			strat.ID, strat.Name, strat.Type, status, strat.Trades, strat.PnL, strat.WinRate))
	}
	sb.WriteString("└──────────┴─────────────┴──────────┴──────────┴────────┴────────────┴──────────┘\n")

	return sb.String()
}

func (s *Strategies) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s.model, nil }

type Exchanges struct{ model *Model }

func NewExchanges(m *Model) *Exchanges { return &Exchanges{model: m} }
func (e *Exchanges) Title() string     { return "Exchanges" }
func (e *Exchanges) Help() []string     { return []string{"Enter: details"} }

func (e *Exchanges) Render() string {
	var sb strings.Builder
	d := e.model.data

	sb.WriteString("┌─ EXCHANGES ───────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ Name              │ Status      │ Latency │ Connected │ Last Update        │\n")
	sb.WriteString("├───────────────────┼─────────────┼─────────┼───────────┼────────────────────┤\n")
	for _, ex := range d.Exchanges {
		statusIcon := "⚫"
		if ex.Status == "Connected" {
			statusIcon = "🟢"
		}
		sb.WriteString(fmt.Sprintf("│ %-17s │ %s %-9s │ %-7s │ %-9s │ %-18s │\n",
			ex.Name, statusIcon, ex.Status, ex.Latency, ex.Connected, ex.LastUpdate))
	}
	sb.WriteString("└───────────────────┴─────────────┴─────────┴───────────┴────────────────────┘\n")

	return sb.String()
}

func (e *Exchanges) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return e.model, nil }

type MarketData struct{ model *Model }

func NewMarketData(m *Model) *MarketData { return &MarketData{model: m} }
func (m *MarketData) Title() string       { return "Market Data" }
func (m *MarketData) Help() []string      { return []string{} }

func (m *MarketData) Render() string {
	var sb strings.Builder

	sb.WriteString("┌─ MARKET DATA ──────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ Symbol      │ Price        │ 24h Change  │ Volume      │ High      │ Low       │\n")
	sb.WriteString("├─────────────┼──────────────┼─────────────┼─────────────┼───────────┼───────────┤\n")
	sb.WriteString("│ BTC/USDT    │ $44,135.68   │ +2.34%      │ 28.5B       │ $44,500   │ $43,200   │\n")
	sb.WriteString("│ ETH/USDT    │ $2,380.45    │ +1.28%      │ 15.2B       │ $2,400    │ $2,350    │\n")
	sb.WriteString("│ SOL/USDT    │ $97.20       │ -1.32%      │ 3.5B        │ $99.50    │ $96.80    │\n")
	sb.WriteString("│ BNB/USDT    │ $292.50      │ +2.46%      │ 1.2B        │ $295      │ $285      │\n")
	sb.WriteString("│ XRP/USDT    │ $0.51        │ -1.92%      │ 2.1B        │ $0.53     │ $0.50     │\n")
	sb.WriteString("└─────────────┴──────────────┴─────────────┴─────────────┴───────────┴───────────┘\n")

	return sb.String()
}

func (m *MarketData) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m.model, nil }

type Signals struct{ model *Model }

func NewSignals(m *Model) *Signals { return &Signals{model: m} }
func (s *Signals) Title() string   { return "Signals" }
func (s *Signals) Help() []string  { return []string{"Enter: execute"} }

func (s *Signals) Render() string {
	var sb strings.Builder
	d := s.model.data

	sb.WriteString("┌─ TRADING SIGNALS ─────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ ID       │ Strategy   │ Symbol     │ Action │ Strength │ Confidence │ Reason    │\n")
	sb.WriteString("├──────────┼────────────┼────────────┼────────┼──────────┼────────────┼───────────┤\n")
	for _, sig := range d.Signals {
		action := sig.Action
		if action == "BUY" {
			action = "🟢 BUY"
		} else if action == "SELL" {
			action = "🔴 SELL"
		}
		sb.WriteString(fmt.Sprintf("│ %-8s │ %-10s │ %-10s │ %-6s │ %-8s │ %-10s │ %-9s │\n",
			sig.ID, sig.Strategy, sig.Symbol, action, sig.Strength, sig.Confidence, sig.Reason))
	}
	sb.WriteString("└──────────┴────────────┴────────────┴────────┴──────────┴────────────┴───────────┘\n")

	return sb.String()
}

func (s *Signals) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s.model, nil }

type AI struct{ model *Model }

func NewAI(m *Model) *AI { return &AI{model: m} }
func (a *AI) Title() string { return "AI Brain" }
func (a *AI) Help() []string { return []string{} }

func (a *AI) Render() string {
	var sb strings.Builder

	sb.WriteString("┌─ AI BRAIN ────────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ Status:        🟡 Disabled (Enable in config)                                   │\n")
	sb.WriteString("│ Provider:      OpenAI GPT-4                                                   │\n")
	sb.WriteString("│ Cache:         Enabled                                                       │\n")
	sb.WriteString("├───────────────────────────────────────────────────────────────────────────────┤\n")
	sb.WriteString("│                               MODELS                                         │\n")
	sb.WriteString("├───────────────────────────────────────────────────────────────────────────────┤\n")
	sb.WriteString("│ [ ] LLM Analysis    │ Sentiment Analysis │ Pattern Recognition       │\n")
	sb.WriteString("│ [ ] ML Prediction   │ Risk Assessment    │ Strategy Generation      │\n")
	sb.WriteString("└───────────────────────────────────────────────────────────────────────────────┘\n")

	return sb.String()
}

func (a *AI) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return a.model, nil }

type Risk struct{ model *Model }

func NewRisk(m *Model) *Risk { return &Risk{model: m} }
func (r *Risk) Title() string { return "Risk Management" }
func (r *Risk) Help() []string { return []string{} }

func (r *Risk) Render() string {
	var sb strings.Builder
	d := r.model.data

	sb.WriteString("┌─ RISK LIMITS ──────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│ Limit Type          │ Current Setting │ Current Value │ Status                  │\n")
	sb.WriteString("├─────────────────────┼─────────────────┼───────────────┼─────────────────────────┤\n")
	sb.WriteString(fmt.Sprintf("│ Max Position Size   │ %-15s │ %-13s │ %-23s │\n", d.Risk.MaxPositionSize, "N/A", "✅ OK"))
	sb.WriteString(fmt.Sprintf("│ Max Daily Loss      │ %-15s │ %-13s │ %-23s │\n", d.Risk.MaxDailyLoss, "N/A", "✅ OK"))
	sb.WriteString(fmt.Sprintf("│ Max Drawdown        │ %-15s │ %-13s │ %-23s │\n", d.Risk.MaxDrawdown, d.Risk.CurrentDrawdown, "✅ OK"))
	sb.WriteString(fmt.Sprintf("│ Max Exposure        │ %-15s │ %-13s │ %-23s │\n", d.Risk.MaxExposure, d.Risk.CurrentExposure, "✅ OK"))
	sb.WriteString(fmt.Sprintf("│ Max Leverage        │ %-15s │ %-13s │ %-23s │\n", d.Risk.MaxLeverage, "2x", "✅ OK"))
	sb.WriteString("└─────────────────────┴─────────────────┴───────────────┴─────────────────────────┘\n")

	return sb.String()
}

func (r *Risk) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return r.model, nil }

type Settings struct{ model *Model }

func NewSettings(m *Model) *Settings { return &Settings{model: m} }
func (s *Settings) Title() string     { return "Settings" }
func (s *Settings) Help() []string    { return []string{} }

func (s *Settings) Render() string {
	var sb strings.Builder

	sb.WriteString("┌─ SETTINGS ─────────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│                                                                                  │\n")
	sb.WriteString("│ Theme:           [dark] [light]                                                   │\n")
	sb.WriteString("│ Color Scheme:    [default] [nord] [gruvbox] [monokai]                             │\n")
	sb.WriteString("│ Refresh Rate:    [1s] [5s] [10s] [30s]                                           │\n")
	sb.WriteString("│                                                                                  │\n")
	sb.WriteString("│ Trading Mode:    [paper] [live]                                                  │\n")
	sb.WriteString("│ Risk Level:      [conservative] [moderate] [aggressive]                          │\n")
	sb.WriteString("│                                                                                  │\n")
	sb.WriteString("│ [ ] Enable AI Brain                                                             │\n")
	sb.WriteString("│ [ ] Enable Notifications                                                        │\n")
	sb.WriteString("│ [ ] Auto-start on boot                                                           │\n")
	sb.WriteString("│                                                                                  │\n")
	sb.WriteString("│ [C]onfig File  [E]xport Settings  [I]mport Settings  [R]eset Defaults            │\n")
	sb.WriteString("└──────────────────────────────────────────────────────────────────────────────────┘\n")

	return sb.String()
}

func (s *Settings) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return s.model, nil }

func (m *Model) setStatus(msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusMessage = msg
}

func (m *Model) addOutput(line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputBuffer = append(m.outputBuffer, line)
	if len(m.outputBuffer) > 100 {
		m.outputBuffer = m.outputBuffer[1:]
	}
}

func (m *Model) clearOutput() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputBuffer = nil
}

func (m *Model) addCommand(cmd string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandHistory = append(m.commandHistory, cmd)
	m.commandIndex = len(m.commandHistory)
	m.currentCommand = ""
}

func (m *Model) previousCommand() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.commandIndex > 0 {
		m.commandIndex--
	}
	if m.commandIndex < len(m.commandHistory) {
		return m.commandHistory[m.commandIndex]
	}
	return ""
}

func (m *Model) nextCommand() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.commandIndex < len(m.commandHistory)-1 {
		m.commandIndex++
		return m.commandHistory[m.commandIndex]
	}
	m.commandIndex = len(m.commandHistory)
	return ""
}

var version = "1.0.0"
