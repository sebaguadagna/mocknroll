package tui

import (
	"github.com/charmbracelet/bubbles/list"
)

type mockItem struct {
	title       string
	description string
}

func (m mockItem) Title() string       { return m.title }
func (m mockItem) Description() string { return m.description }
func (m mockItem) FilterValue() string { return m.title }

type model struct {
	list list.Model
}

func initialModel() model {
	items := []list.Item{
		mockItem{title: "GET /api/v1/users", description: "Returns users list"},
		mockItem{title: "POST /api/v1/orders", description: "Creates an order"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Mocks loaded"

	return model{list: l}
}
