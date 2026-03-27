package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type View int

const (
	ViewLoading View = iota
	ViewSetup
	ViewLoginSuccess
	ViewChat
	ViewSessions
	ViewThemes
	ViewModels
	ViewProviders
	ViewStatus
	ViewHelp
)

type Message struct {
	role      string
	content   string
	timestamp time.Time
}

type Action struct {
	description string
	timestamp  time.Time
	status     string
}

type Model struct {
	view             View
	loadingStep      string
	loadingProgress int
	apiKey           string
	model            string
	provider         string
	session          string
	sessionID        int
	messages         []Message
	actions          []Action
	viewport         viewport.Model
	textInput        textinput.Model
	currentInput     string
	waitingForInput  bool
	inputPrompt     string
	theme            string
	themes           []string
	models           []string
	providers        []string
	sessions         []string
	selectedIndex    int
	chatScrollOffset int
	width            int
	height           int
}

var themes = []string{"dark", "light", "cyberpunk", "monokai", "nord"}
var models = []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "claude-3-5-sonnet", "claude-3-opus", "gemini-1.5-pro", "ollama/llama3"}
var providers = []string{"OpenAI", "Anthropic", "Google", "Ollama", "Groq", "Together AI", "Perplexity"}
var sessions = []string{"main-trading", "backtest-session", "strategy-dev", "paper-trading"}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter your API key (sk-...)"
	ti.Focus()
	ti.Prompt = cyanStyle.Render("❯ ")
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		BorderStyle(lipgloss.HiddenBorder())

	return Model{
		view:            ViewLoading,
		loadingStep:     "Initializing...",
		loadingProgress: 0,
		model:           "gpt-4o",
		provider:        "OpenAI",
		session:         "main-trading",
		sessionID:       1,
		messages:        []Message{},
		actions:         []Action{},
		viewport:        vp,
		textInput:       ti,
		waitingForInput: false,
		theme:           "dark",
		themes:          themes,
		models:          models,
		providers:       providers,
		sessions:        sessions,
		selectedIndex:   0,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		runLoadingSequence(),
	)
}

func runLoadingSequence() tea.Cmd {
	return func() tea.Msg {
		steps := []struct {
			step     string
			progress int
			delay    int
		}{
			{"Loading core modules...", 10, 100},
			{"Initializing AI Brain...", 25, 150},
			{"Connecting to exchanges...", 40, 200},
			{"Loading trading strategies...", 55, 150},
			{"Starting risk manager...", 70, 100},
			{"Initializing WebSocket feeds...", 85, 150},
			{"Ready!", 100, 100},
		}

		for _, s := range steps {
			time.Sleep(time.Duration(s.delay) * time.Millisecond)
		}
		return LoadingCompleteMsg{}
	}
}

type LoadingCompleteMsg struct{}

type AIGeneratedMsg struct {
	response string
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		return m, nil

	case LoadingCompleteMsg:
		m.view = ViewSetup
		return m, nil

	case AIGeneratedMsg:
		m.messages = append(m.messages, Message{
			role:      "assistant",
			content:   msg.response,
			timestamp: time.Now(),
		})
		m.actions = append(m.actions, Action{
			description: "AI response generated",
			timestamp:   time.Now(),
			status:      "complete",
		})
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) handleKeyMsg(keyMsg tea.KeyMsg) (Model, tea.Cmd) {
	switch keyMsg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		if m.view == ViewChat {
			m.view = ViewSetup
			return m, nil
		}
		if m.view == ViewSetup || m.view == ViewSessions || m.view == ViewThemes || m.view == ViewModels || m.view == ViewProviders || m.view == ViewStatus || m.view == ViewHelp {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyEnter:
		return m.handleEnter()

	case tea.KeyTab:
		m.waitingForInput = !m.waitingForInput
		if m.waitingForInput {
			m.inputPrompt = "API Key"
			m.textInput.Placeholder = "sk-..."
			m.textInput.Focus()
		}
		return m, nil

	case tea.KeyUp:
		if m.view == ViewSessions || m.view == ViewThemes || m.view == ViewModels || m.view == ViewProviders {
			m.selectedIndex = max(0, m.selectedIndex-1)
		} else if m.view == ViewChat && m.chatScrollOffset > 0 {
			m.chatScrollOffset--
		}
		return m, nil

	case tea.KeyDown:
		if m.view == ViewSessions || m.view == ViewThemes || m.view == ViewModels || m.view == ViewProviders {
			maxIdx := len(m.sessions) - 1
			if m.view == ViewThemes {
				maxIdx = len(m.themes) - 1
			} else if m.view == ViewModels {
				maxIdx = len(m.models) - 1
			} else if m.view == ViewProviders {
				maxIdx = len(m.providers) - 1
			}
			m.selectedIndex = min(maxIdx, m.selectedIndex+1)
		} else if m.view == ViewChat && m.chatScrollOffset < len(m.messages)-1 {
			m.chatScrollOffset++
		}
		return m, nil

	case tea.KeyRunes:
		switch keyMsg.String() {
		case "a", "A":
			if m.view == ViewChat {
				m.view = ViewChat
				m.textInput.Focus()
			}
			return m, nil

		case "h", "H":
			if m.view == ViewChat {
				m.view = ViewHelp
				m.selectedIndex = 0
			}
			return m, nil

		case "s", "S":
			if m.view == ViewChat {
				m.view = ViewSessions
				m.selectedIndex = 0
			}
			return m, nil

		case "t", "T":
			if m.view == ViewChat {
				m.view = ViewThemes
				m.selectedIndex = 0
			}
			return m, nil

		case "m", "M":
			if m.view == ViewChat {
				m.view = ViewModels
				m.selectedIndex = 0
			}
			return m, nil

		case "p", "P":
			if m.view == ViewChat {
				m.view = ViewProviders
				m.selectedIndex = 0
			}
			return m, nil

		case "i", "I":
			if m.view == ViewChat {
				m.view = ViewStatus
				m.selectedIndex = 0
			}
			return m, nil

		case "c", "C":
			if m.view == ViewChat {
				m.messages = []Message{}
				m.actions = []Action{}
				m.messages = append(m.messages, Message{
					role:      "system",
					content:   "Conversation cleared. What would you like to do?",
					timestamp: time.Now(),
				})
			}
			return m, nil

		case "n", "N":
			if m.view == ViewSessions {
				m.sessionID++
				m.session = fmt.Sprintf("session-%d", m.sessionID)
				m.messages = []Message{}
				m.messages = append(m.messages, Message{
					role:      "system",
					content:   fmt.Sprintf("Started new session: %s", m.session),
					timestamp: time.Now(),
				})
				m.view = ViewChat
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) handleEnter() (Model, tea.Cmd) {
	if m.view == ViewSetup {
		m.apiKey = m.textInput.Value()
		if m.apiKey == "" {
			m.messages = append(m.messages, Message{
				role:      "system",
				content:   "Please enter your API key!",
				timestamp: time.Now(),
			})
			return m, nil
		}
		if !strings.HasPrefix(m.apiKey, "sk-") {
			m.messages = append(m.messages, Message{
				role:      "system",
				content:   "Invalid API key format. OpenAI keys start with 'sk-'",
				timestamp: time.Now(),
			})
			return m, nil
		}
		m.view = ViewLoginSuccess
		return m, func() tea.Msg {
			time.Sleep(2 * time.Second)
			return LoadingCompleteMsg{}
		}
	}

	if m.view == ViewLoginSuccess {
		m.view = ViewChat
		m.messages = []Message{
			{role: "system", content: "Welcome to OpenTrader Agent! I'm your AI trading assistant.", timestamp: time.Now()},
			{role: "system", content: "I can help you with:", timestamp: time.Now()},
			{role: "system", content: "  • Trading strategies & execution", timestamp: time.Now()},
			{role: "system", content: "  • Portfolio monitoring", timestamp: time.Now()},
			{role: "system", content: "  • Risk management", timestamp: time.Now()},
			{role: "system", content: "  • Backtesting", timestamp: time.Now()},
			{role: "system", content: "  • Exchange connections", timestamp: time.Now()},
			{role: "system", content: "", timestamp: time.Now()},
			{role: "system", content: "What would you like to do today?", timestamp: time.Now()},
		}
		return m, nil
	}

	if m.view == ViewChat && m.textInput.Value() != "" {
		userInput := m.textInput.Value()
		m.messages = append(m.messages, Message{
			role:      "user",
			content:   userInput,
			timestamp: time.Now(),
		})
		m.textInput.Reset()

		return m, m.simulateAIResponse(userInput)
	}

	if m.view == ViewSessions {
		m.session = m.sessions[m.selectedIndex]
		m.messages = append(m.messages, Message{
			role:      "system",
			content:   fmt.Sprintf("Switched to session: %s", m.session),
			timestamp: time.Now(),
		})
		m.view = ViewChat
		return m, nil
	}

	if m.view == ViewThemes {
		m.theme = m.themes[m.selectedIndex]
		m.messages = append(m.messages, Message{
			role:      "system",
			content:   fmt.Sprintf("Theme changed to: %s", m.theme),
			timestamp: time.Now(),
		})
		m.view = ViewChat
		return m, nil
	}

	if m.view == ViewModels {
		m.model = m.models[m.selectedIndex]
		m.messages = append(m.messages, Message{
			role:      "system",
			content:   fmt.Sprintf("Model changed to: %s", m.model),
			timestamp: time.Now(),
		})
		m.view = ViewChat
		return m, nil
	}

	if m.view == ViewProviders {
		m.provider = m.providers[m.selectedIndex]
		m.messages = append(m.messages, Message{
			role:      "system",
			content:   fmt.Sprintf("Provider changed to: %s", m.provider),
			timestamp: time.Now(),
		})
		m.view = ViewChat
		return m, nil
	}

	return m, nil
}

func (m Model) simulateAIResponse(userInput string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(500 * time.Millisecond)
		return AIGeneratedMsg{response: m.generateHinglishResponse(userInput)}
	}
}

func (m Model) generateHinglishResponse(input string) string {
	input = strings.ToLower(input)

	if strings.Contains(input, "strategy") {
		return "Main abhi aapke liye ek momentum strategy bana raha hoon. Parameters set kar raha hoon...\n\n✓ Strategy 'MomentumTrader' created\n✓ Timeframe: 1H\n✓ Indicators: RSI(14), MACD(12,26,9)\n✓ Position size: 2% per trade\n✓ Risk per trade: 1%\n\nAb aap ise start karna chahein toh 'start strategy MomentumTrader' bol do!"
	}

	if strings.Contains(input, "trade") || strings.Contains(input, "buy") || strings.Contains(input, "sell") {
		return "Alright, trade execute kar raha hoon...\n\n📊 Order Details:\n• Symbol: BTC/USDT\n• Type: Market Buy\n• Quantity: 0.1 BTC\n• Estimated cost: ~$6,500\n\n⚠️ Checking risk limits...\n✅ Risk check passed\n\n🔄 Order submitted to Binance...\n✅ Order filled at $65,023.45\n\nPosition opened successfully!"
	}

	if strings.Contains(input, "portfolio") || strings.Contains(input, "position") {
		return "Aapka portfolio overview:\n\n💼 **Positions (3 Open)**\n• BTC/USDT: +2.34% ($65,023)\n• ETH/USDT: -0.85% ($42,100)\n• SOL/USDT: +5.12% ($12,500)\n\n📈 **Total P&L:** +$1,234 (+2.1%)\n\n💰 **Account Balance:** $45,678\n🎯 **Available Margin:** $12,345\n\nKya aap koi specific position ke baare mein jaanna chahein?"
	}

	if strings.Contains(input, "backtest") || strings.Contains(input, "test") {
		return "Backtest shuru kar raha hoon with last 6 months data...\n\n📊 Running simulation...\n\n╔════════════════════════════════╗\n║     Backtest Results           ║\n╠════════════════════════════════╣\n║  Total Trades:      234        ║\n║  Win Rate:         62.5%      ║\n║  Profit Factor:     1.85      ║\n║  Max Drawdown:     -8.3%      ║\n║  Sharpe Ratio:      2.1       ║\n║  Total Return:    +34.2%      ║\n╚════════════════════════════════╝\n\nKaafi impressive results hain! Strategy ko deploy karna chahein?"
	}

	if strings.Contains(input, "risk") || strings.Contains(input, "limit") {
		return "Risk parameters check kar raha hoon...\n\n🛡️ **Risk Dashboard:**\n• Portfolio exposure: 45% (Limit: 80%)\n• Max drawdown: 2.3% (Limit: 15%)\n• Daily loss: 0.5% (Limit: 5%)\n• Single position: 8% (Limit: 10%)\n\n✅ All risk checks PASSED\n\nAapke risk profile ke hisab se sab kuch normal hai. Koi changes chahiye?"
	}

	if strings.Contains(input, "connect") || strings.Contains(input, "exchange") {
		return "Exchange connections check kar raha hoon...\n\n🔗 **Active Connections:**\n• Binance: ✅ Connected (12ms)\n• Bybit: ✅ Connected (18ms)\n• OKX: ✅ Connected (25ms)\n• Kraken: ✅ Connected (45ms)\n• Hyperliquid: ✅ Connected (8ms)\n\nAll exchanges connected! Aap trade kar sakte hain."
	}

	if strings.Contains(input, "help") || strings.Contains(input, "what") {
		return "Main aapki help ke liye yahan hoon! Kuch examples:\n\n📌 **Trading:**\n• \"Buy 0.1 BTC at market\"\n• \"Sell ETH when RSI > 70\"\n• \"Set trailing stop 5%\"\n\n📌 **Strategies:**\n• \"Create a momentum strategy\"\n• \"Start grid trading on BTC\"\n• \"Stop my current strategies\"\n\n📌 **Analysis:**\n• \"Show my portfolio\"\n• \"Run backtest on my strategy\"\n• \"Check risk limits\"\n\n📌 **General:**\n• \"Connect to Binance\"\n• \"Change model to Claude\"\n• \"Switch to paper trading\"\n\nKya aapko kuch specific chahiye?"
	}

	return fmt.Sprintf("Hmm, aapne '%s' bola. Main ise process kar raha hoon...\n\nSamajh gaya! Aapko is task pe kaam karne mein help chahiye. Main abhi ise implement karta hoon...\n\n✅ Task understood\n⚙️ Processing request...\n\nAapke request ke hisab se, main aapki strategy ko analyze kar raha hoon. Thoda aur detail de sakte hain?", input)
}

func (m Model) View() string {
	switch m.view {
	case ViewLoading:
		return RenderLoadingAnimation(m.loadingStep, m.loadingProgress)
	case ViewSetup:
		return m.renderSetupView()
	case ViewLoginSuccess:
		return RenderLoginSuccess()
	case ViewChat:
		return m.renderChatView()
	case ViewSessions:
		return RenderSessionsPanel(m.sessions, m.session)
	case ViewThemes:
		return RenderThemePanel(m.themes, m.theme)
	case ViewModels:
		return RenderModelPanel(m.models, m.model)
	case ViewProviders:
		return RenderProviderPanel(m.providers, m.provider)
	case ViewStatus:
		return RenderStatusPanel()
	case ViewHelp:
		return RenderHelpPanel()
	default:
		return ""
	}
}

func (m Model) renderChatView() string {
	var lines []string

	lines = append(lines, RenderHeader(m.model, m.provider, m.session))

	lines = append(lines, m.renderMessages())

	lines = append(lines, RenderCommandBar())
	lines = append(lines, "")
	lines = append(lines, m.textInput.View())

	footer := fmt.Sprintf(`╠%s╣
║  %s│ Exchanges: 5 Active  │  Strategies: 3 Running  │  Positions: 3  │  Risk: Normal  │  %s  %s║
╚%s╝`,
		strings.Repeat("═", 95),
		purpleStyle.Render("Chat"),
		dimStyle.Render(time.Now().Format("15:04:05")),
		strings.Repeat("─", 10),
		strings.Repeat("═", 95))
	lines = append(lines, footer)

	return lipgloss.NewStyle().Width(m.width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderMessages() string {
	var lines []string

	if len(m.messages) == 0 {
		lines = append(lines, dimStyle.Render("║                                                                                      ║"))
		lines = append(lines, dimStyle.Render("║   Start a conversation by typing your request below...                      ║"))
		lines = append(lines, dimStyle.Render("║                                                                                      ║"))
		return strings.Join(lines, "\n")
	}

	start := 0
	end := len(m.messages)
	if end > 15 {
		start = end - 15
	}
	visibleMessages := m.messages[start:end]

	for _, msg := range visibleMessages {
		timestamp := msg.timestamp.Format("15:04")
		switch msg.role {
		case "user":
			lines = append(lines, fmt.Sprintf("║ %s[%s]%s You: %s                                                                 ║",
				cyanStyle.Render("●"),
				dimStyle.Render(timestamp),
				dimStyle.Render("●"),
				wrapText(msg.content, 80)))
		case "assistant":
			for _, line := range strings.Split(msg.content, "\n") {
				lines = append(lines, fmt.Sprintf("║ %s[%s]%s AI: %s                                                                 ║",
					purpleStyle.Render("●"),
					dimStyle.Render(timestamp),
					dimStyle.Render("●"),
					wrapText(line, 80)))
			}
		case "system":
			lines = append(lines, fmt.Sprintf("║ %s[%s]%s %s                                                            ║",
				infoStyle.Render("●"),
				dimStyle.Render(timestamp),
				dimStyle.Render("●"),
				wrapText(msg.content, 78)))
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderSetupView() string {
	var lines []string

	lines = append(lines, fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                                                                                      ║
║   ██████╗ ██╗██╗  ██╗███████╗██╗     ██╗      █████╗ ██████╗                    ║
║   ██╔══██╗██║╚██╗██╔╝██╔════╝██║     ██║     ██╔══██╗██╔══██╗                   ║
║   ██████╔╝██║ ╚███╔╝ █████╗  ██║     ██║     ███████║██████╔╝                   ║
║   ██╔═══╝ ██║ ██╔██╗ ██╔══╝  ██║     ██║     ██╔══██║██╔══██╗                   ║
║   ██║     ██║██╔╝ ██╗███████╗███████╗███████╗██║  ██║██║  ██║                   ║
║   ╚═╝     ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝                   ║
║                                                                                      ║
║            %sEnterprise AI Trading Agent Framework%s                                    ║
║                 %s10x More Powerful Than NautilusTrader%s                             ║
║                                                                                      ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║
║   %s🚀 Welcome to OpenTrader Agent%s                                                   ║
║                                                                                      ║
║   Supported AI Providers:                                                            ║
║   • OpenAI (GPT-4o) - %sRecommended%s                                                 ║
║   • Anthropic (Claude)                                                              ║
║   • Google Gemini                                                                  ║
║   • Local Models (Ollama)                                                          ║
║                                                                                      ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║
║   %sEnter your OpenAI API Key:%s                                                      ║
║                                                                                      ║`,
		purpleStyle.Render("╔══════════════════════════════════════════════════════════════════════════"),
		purpleStyle.Render("║"),
		cyanStyle.Render("╔══════════════════════════════════════════════════════════════════════════"),
		cyanStyle.Render("║"),
		goldStyle.Render(strings.Repeat("─", 30)),
		goldStyle.Render(strings.Repeat("─", 30)),
		successStyle.Render("●"),
		successStyle.Render(strings.Repeat("─", 30)),
		bannerStyle.Render(strings.Repeat("─", 30)),
		bannerStyle.Render(strings.Repeat("─", 30))))

	lines = append(lines, "║                                                                                      ║")
	lines = append(lines, "║   "+m.textInput.View()+"   ║")
	lines = append(lines, "║                                                                                      ║")

	if len(m.messages) > 0 {
		for _, msg := range m.messages {
			lines = append(lines, fmt.Sprintf("║   %s%s                                                                    ║",
				errorStyle.Render("⚠"), dimStyle.Render(msg.content)))
		}
	}

	lines = append(lines, "║                                                                                      ║")
	lines = append(lines, dimStyle.Render("║   [Enter] Connect   [Esc] Exit                                                   ║"))
	lines = append(lines, "╚══════════════════════════════════════════════════════════════════════════════════════════╝")

	return lipgloss.NewStyle().Width(m.width).Render(strings.Join(lines, "\n"))
}

func wrapText(text string, maxWidth int) string {
	if len(text) > maxWidth {
		return text[:maxWidth]
	}
	return text
}
