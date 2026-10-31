package tui

import (
	tea "charm.land/bubbletea/v2"
)

// Start runs the TUI, which shows port as the address the mock server listens on.
func Start(port int) error {
	m, err := initialModel(port)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
