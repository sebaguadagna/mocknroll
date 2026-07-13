package tui

import (
	tea "charm.land/bubbletea/v2"
)

func Start() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}
