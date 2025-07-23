package tui

import (
	"github.com/charmbracelet/bubbles/list"
)

type mode int

const (
	listMode mode = iota //0
	formMode             //1
)

type mockItem struct {
	title       string
	description string
}

func (m mockItem) Title() string       { return m.title }
func (m mockItem) Description() string { return m.description }
func (m mockItem) FilterValue() string { return m.title }

type model struct {
	list         list.Model
	currentMode  mode
	formStep     int
	formPath     string
	formMethod   string
	formStatus   string
	formDelay    string
	formJSONFile string
}

func initialModel() model {
	items := []list.Item{
		mockItem{title: "GET /api/v1/users", description: "Returns users list"},
		mockItem{title: "POST /api/v1/orders", description: "Creates an order"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 30, 10) // valores temporales visibles
	l.Title = "Mocks loaded"

	return model{
		list:        l,
		currentMode: listMode,
		formStep:    0,
	}
}
