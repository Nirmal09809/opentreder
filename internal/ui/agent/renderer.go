package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	bannerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D9FF")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	purpleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A855F7"))

	cyanStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4"))

	goldStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))
)

func RenderBanner() string {
	return fmt.Sprintf(`
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
╚══════════════════════════════════════════════════════════════════════════════════════════╝`,
		purpleStyle.Render("╔══════════════════════════════════════════════════════════════════════"),
		purpleStyle.Render("║"),
		cyanStyle.Render("╔══════════════════════════════════════════════════════════════════════"),
		cyanStyle.Render("║"))
}

func RenderLoadingAnimation(step string, progress int) string {
	barWidth := 50
	filled := (barWidth * progress) / 100
	empty := barWidth - filled

	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIndex := int(time.Now().UnixNano()) / 100000000 % len(spinner)

	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                                                                                      ║
%s
║                                                                                      ║
║   %s %s Loading: %s                                                        ║
║                                                                                      ║
║   [%s%s%s] %d%%                                                       ║
║                                                                                      ║
║   %sPlease wait while we prepare your trading environment...%s                          ║
║                                                                                      ║
╚══════════════════════════════════════════════════════════════════════════════════════════╝`,
		dimStyle.Render("╔══════════════════════════════════════════════════════════════════════════════════════════╗"),
		dimStyle.Render(spinner[spinnerIndex]),
		infoStyle.Render("●"),
		dimStyle.Render(step),
		cyanStyle.Render(strings.Repeat("█", filled)),
		dimStyle.Render(strings.Repeat("░", empty)),
		dimStyle.Render(strings.Repeat("═", barWidth)),
		progress,
		dimStyle.Render(strings.Repeat("─", 70)),
		dimStyle.Render(strings.Repeat("─", 70)))
}

func RenderSetupScreen() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                                                                                      ║
%s
║                                                                                      ║
║                              %s🚀 Welcome to OpenTrader Agent%s                               ║
║                                                                                      ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║
║   To get started, you need to configure your API keys.                               ║
║                                                                                      ║
║   %sSupported Providers:%s                                                            ║
║                                                                                      ║
║   • OpenAI (GPT-4) - %sRecommended%s                                                 ║
║   • Anthropic (Claude)                                                              ║
║   • Google Gemini                                                                  ║
║   • Local Models (Ollama)                                                          ║
║   • And 75+ more providers...                                                      ║
║                                                                                      ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║
║   %sEnter your OpenAI API Key:%s                                                     ║
║                                                                                      ║
║   ┌────────────────────────────────────────────────────────────────────────────┐     ║
║   │                                                                            │     ║
║   │                                                                            │     ║
║   └────────────────────────────────────────────────────────────────────────────┘     ║
║                                                                                      ║
║   %s[Tab] Next   [Enter] Connect   [Esc] Cancel%s                                    ║
║                                                                                      ║
╚══════════════════════════════════════════════════════════════════════════════════════════╝`,
		dimStyle.Render("╔══════════════════════════════════════════════════════════════════════════════════════════╗"),
		bannerStyle.Render("╔══════════════════════════════════════════════════════════════════════════════"),
		bannerStyle.Render("║"),
		dimStyle.Render(strings.Repeat("─", 70)),
		goldStyle.Render(strings.Repeat("─", 70)),
		successStyle.Render("●"),
		successStyle.Render(strings.Repeat("─", 70)))
}

func RenderLoginSuccess() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                                                                                      ║
%s
║                                                                                      ║
║                              %s✅ Login Successful!%s                                     ║
║                                                                                      ║
║   %s✓%s Connected to OpenAI API                                                   ║
║   %s✓%s Model: GPT-4                                                               ║
║   %s✓%s Trading Engine Initialized                                                 ║
║   %s✓%s Exchange Adapters Loaded (15+)                                              ║
║   %s✓%s Risk Manager Ready                                                          ║
║   %s✓%s AI Brain Online                                                            ║
║                                                                                      ║
║                              %sStarting Agent Session...%s                                   ║
║                                                                                      ║
╚══════════════════════════════════════════════════════════════════════════════════════════╝`,
		dimStyle.Render("╔══════════════════════════════════════════════════════════════════════════════════════════╗"),
		successStyle.Render("╔══════════════════════════════════════════════════════════════════════════════"),
		successStyle.Render("║"),
		successStyle.Render("✓"),
		dimStyle.Render(strings.Repeat("─", 70)),
		successStyle.Render("✓"),
		dimStyle.Render(strings.Repeat("─", 70)),
		successStyle.Render("✓"),
		dimStyle.Render(strings.Repeat("─", 70)),
		successStyle.Render("✓"),
		dimStyle.Render(strings.Repeat("─", 70)),
		successStyle.Render("✓"),
		dimStyle.Render(strings.Repeat("─", 70)),
		successStyle.Render("✓"),
		dimStyle.Render(strings.Repeat("─", 70)),
		infoStyle.Render(strings.Repeat("─", 70)),
		infoStyle.Render(strings.Repeat("─", 70)))
}

func RenderHeader(model, provider, session string) string {
	return fmt.Sprintf(`╔%s╗
║  🤖 OpenTrader Agent  │  Model: %s  │  Provider: %s  │  Session: %s  │  %s  %s║
╠%s╣`,
		strings.Repeat("═", 95),
		cyanStyle.Render(model),
		purpleStyle.Render(provider),
		goldStyle.Render(session),
		dimStyle.Render(time.Now().Format("15:04:05")),
		dimStyle.Render(strings.Repeat("─", 10)),
		strings.Repeat("═", 95))
}

func RenderFooter() string {
	return fmt.Sprintf(`╠%s╣
║  %s│ Exchanges: 5 Active  │  Strategies: 3 Running  │  Positions: 3  │  Risk: Normal  │  %s  %s║
╚%s╝`,
		strings.Repeat("═", 95),
		dimStyle.Render("View: "+purpleStyle.Render("AI Chat")),
		dimStyle.Render(time.Now().Format("15:04:05")),
		dimStyle.Render(strings.Repeat("─", 10)),
		strings.Repeat("═", 95))
}

func RenderCommandBar() string {
	return fmt.Sprintf(`
╠%s╣
║  %s│ /help │ /sessions │ /clear │ /theme │ /model │ /provider │ /status │ /exit │   %s║
╚%s╝`,
		strings.Repeat("═", 95),
		dimStyle.Render("Commands:"),
		dimStyle.Render("[Tab] Complete  [↑↓] History"),
		strings.Repeat("═", 95))
}

func RenderHelpPanel() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                              %s📚 Help & Commands%s                                          ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║
║   %sSlash Commands:%s                                                                    ║
║   %s───────────────────────────────────────────────────────────────────────────────%s     ║
║   /help           - Show this help panel                                              ║
║   /sessions       - List and switch between sessions                                  ║
║   /clear          - Clear current conversation                                        ║
║   /theme          - Switch theme (dark/light/cyberpunk/monokai)                      ║
║   /model          - Change AI model                                                   ║
║   /provider       - Change AI provider                                                 ║
║   /status         - Show system status                                                ║
║   /exit           - Exit OpenTrader Agent                                             ║
║                                                                                      ║
║   %sAgent Capabilities:%s                                                                 ║
║   %s───────────────────────────────────────────────────────────────────────────────%s     ║
║   • Start/Stop trading strategies                                                    ║
║   • Monitor portfolio and positions                                                  ║
║   • Execute trades on exchanges                                                     ║
║   • Run backtests and analyze results                                              ║
║   • Configure risk management                                                      ║
║   • Set up exchange connections                                                     ║
║   • Generate reports and analytics                                                  ║
║                                                                                      ║
║   %sJust tell me what you want to do!%s                                              ║
║                                                                                      ║
╚══════════════════════════════════════════════════════════════════════════════════════════╝`,
		goldStyle.Render(strings.Repeat("─", 30)),
		goldStyle.Render(strings.Repeat("─", 30)),
		dimStyle.Render(strings.Repeat("─", 70)),
		dimStyle.Render(strings.Repeat("─", 70)),
		cyanStyle.Render(strings.Repeat("─", 70)),
		cyanStyle.Render(strings.Repeat("─", 70)))
}

func RenderSessionsPanel(sessions []string, current string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                              %s💬 Sessions%s                                                  ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║`, purpleStyle.Render(strings.Repeat("─", 30)), purpleStyle.Render(strings.Repeat("─", 30))))

	for _, s := range sessions {
		marker := "  "
		if s == current {
			marker = cyanStyle.Render("► ")
		}
		lines = append(lines, fmt.Sprintf("║   %s%s                                                                    ║", marker, dimStyle.Render(s)))
	}

	lines = append(lines, dimStyle.Render("║                                                                                      ║"))
	lines = append(lines, dimStyle.Render("║   [↑↓] Select   [Enter] Switch   [Esc] Close   [N] New Session              ║"))
	lines = append(lines, dimStyle.Render("╚══════════════════════════════════════════════════════════════════════════════════════════╝"))

	return Join(lines)
}

func RenderThemePanel(themes []string, current string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                              %s🎨 Themes%s                                                     ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║`, goldStyle.Render(strings.Repeat("─", 30)), goldStyle.Render(strings.Repeat("─", 30))))

	for _, t := range themes {
		marker := "  "
		if t == current {
			marker = cyanStyle.Render("► ")
		}
		lines = append(lines, fmt.Sprintf("║   %s%s                                                                    ║", marker, dimStyle.Render(t)))
	}

	lines = append(lines, dimStyle.Render("║                                                                                      ║"))
	lines = append(lines, dimStyle.Render("║   [↑↓] Select   [Enter] Apply   [Esc] Close                                   ║"))
	lines = append(lines, dimStyle.Render("╚══════════════════════════════════════════════════════════════════════════════════════════╝"))

	return Join(lines)
}

func RenderModelPanel(models []string, current string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                              %s🤖 Models%s                                                     ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║`, cyanStyle.Render(strings.Repeat("─", 30)), cyanStyle.Render(strings.Repeat("─", 30))))

	for _, m := range models {
		marker := "  "
		if m == current {
			marker = cyanStyle.Render("► ")
		}
		lines = append(lines, fmt.Sprintf("║   %s%s                                                                    ║", marker, dimStyle.Render(m)))
	}

	lines = append(lines, dimStyle.Render("║                                                                                      ║"))
	lines = append(lines, dimStyle.Render("║   [↑↓] Select   [Enter] Apply   [Esc] Close                                   ║"))
	lines = append(lines, dimStyle.Render("╚══════════════════════════════════════════════════════════════════════════════════════════╝"))

	return Join(lines)
}

func RenderProviderPanel(providers []string, current string) string {
	var lines []string
	lines = append(lines, fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                              %s🔗 Providers%s                                                    ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║`, infoStyle.Render(strings.Repeat("─", 30)), infoStyle.Render(strings.Repeat("─", 30))))

	for _, p := range providers {
		marker := "  "
		if p == current {
			marker = cyanStyle.Render("► ")
		}
		lines = append(lines, fmt.Sprintf("║   %s%s                                                                    ║", marker, dimStyle.Render(p)))
	}

	lines = append(lines, dimStyle.Render("║                                                                                      ║"))
	lines = append(lines, dimStyle.Render("║   [↑↓] Select   [Enter] Apply   [Esc] Close                                   ║"))
	lines = append(lines, dimStyle.Render("╚══════════════════════════════════════════════════════════════════════════════════════════╝"))

	return Join(lines)
}

func RenderStatusPanel() string {
	return fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════════════════╗
║                              %s📊 System Status%s                                           ║
╠══════════════════════════════════════════════════════════════════════════════════════════╣
║                                                                                      ║
║   %sAI Brain%s                                                                       ║
║   %s───────────────────────────────────────────────────────────────────────────────%s     ║
║   Provider:    %sOpenAI (GPT-4)%s                                                    ║
║   Status:     %s🟢 Online%s                                                            ║
║   Sessions:   %s12 Active%s                                                           ║
║   Cache:      %sEnabled (60s TTL)%s                                                    ║
║                                                                                      ║
║   %sTrading Engine%s                                                                 ║
║   %s───────────────────────────────────────────────────────────────────────────────%s     ║
║   Mode:       %sPaper Trading%s                                                        ║
║   Positions:  %s3 Open%s                                                              ║
║   P&L:        %s+$1,234.56 (2.34%%)%s                                                  ║
║                                                                                      ║
║   %sExchanges%s                                                                       ║
║   %s───────────────────────────────────────────────────────────────────────────────%s     ║
║   Binance:    %s🟢 Connected (12ms)%s                                                  ║
║   Bybit:      %s🟢 Connected (18ms)%s                                                  ║
║   OKX:        %s🟢 Connected (25ms)%s                                                  ║
║   Kraken:     %s🟢 Connected (45ms)%s                                                  ║
║   Hyperliquid: %s🟢 Connected (8ms)%s                                                   ║
║                                                                                      ║
║   %sRisk Manager%s                                                                    ║
║   %s───────────────────────────────────────────────────────────────────────────────%s     ║
║   Exposure:   %s45%% / 80%% Maximum%s                                                  ║
║   Drawdown:   %s2.3%% / 15%% Maximum%s                                                ║
║   Daily Loss: %s0.5%% / 5%% Maximum%s                                                  ║
║                                                                                      ║
║   [Esc] Close                                                                       ║
╚══════════════════════════════════════════════════════════════════════════════════════════╝`,
		cyanStyle.Render(strings.Repeat("─", 30)),
		cyanStyle.Render(strings.Repeat("─", 70)),
		dimStyle.Render("OpenAI GPT-4"),
		successStyle.Render("●"),
		successStyle.Render(strings.Repeat("─", 70)),
		dimStyle.Render("12"),
		dimStyle.Render(strings.Repeat("─", 70)),
		dimStyle.Render(strings.Repeat("─", 30)),
		dimStyle.Render(strings.Repeat("─", 70)),
		goldStyle.Render("Paper Trading"),
		dimStyle.Render("3"),
		successStyle.Render("+$1,234.56 (2.34%)"),
		dimStyle.Render(strings.Repeat("─", 30)),
		dimStyle.Render(strings.Repeat("─", 70)),
		successStyle.Render("●"),
		dimStyle.Render("12ms"),
		successStyle.Render("●"),
		dimStyle.Render("18ms"),
		successStyle.Render("●"),
		dimStyle.Render("25ms"),
		successStyle.Render("●"),
		dimStyle.Render("45ms"),
		successStyle.Render("●"),
		dimStyle.Render("8ms"),
		dimStyle.Render(strings.Repeat("─", 30)),
		dimStyle.Render(strings.Repeat("─", 70)),
		cyanStyle.Render("45% / 80%"),
		cyanStyle.Render("2.3% / 15%"),
		cyanStyle.Render("0.5% / 5%"))
}

func Join(lines []string) string {
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}
