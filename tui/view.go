package tui

func (m model) View() string {
	return m.list.View()
}
