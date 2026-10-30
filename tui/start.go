package tui

import (
	tea "charm.land/bubbletea/v2"
)

func Start() error {
	m, err := initialModel()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
