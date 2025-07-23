package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func Start() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
