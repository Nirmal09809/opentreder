package main

import (
	"fmt"
	"os"

	"github.com/opentreder/opentreder/internal/ui/agent"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(agent.NewModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running agent: %v\n", err)
		os.Exit(1)
	}
}
