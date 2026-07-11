package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
)

var (
	addMockKey = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add mock"),
	)
	quitKey = key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "quit"),
	)
	toggleEnabledKey = key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle enabled"),
	)
)

type mode int

const (
	listMode        mode = iota //0
	formMode                    //1
	confirmExitMode             // nuevo modo para confirmar salida
)

type mockItem struct {
	title       string
	description string
	status      string
	delay       string
	jsonFile    string
	enabled     bool
}

func (m mockItem) Title() string {
	if !m.enabled {
		return m.title + " (disabled)"
	}
	return m.title
}
func (m mockItem) Description() string { return m.description }
func (m mockItem) FilterValue() string { return m.title }

type model struct {
	list         list.Model
	width        int
	listWidth    int
	listHeight   int
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
		mockItem{
			title:       "GET /api/v1/users",
			description: "Returns users list",
			status:      "200",
			delay:       "30",
			jsonFile:    "examples/users.json",
			enabled:     true,
		},
		mockItem{
			title:       "POST /api/v1/orders",
			description: "Creates an order",
			status:      "201",
			delay:       "800",
			jsonFile:    "examples/orders.json",
			enabled:     true,
		},
	}

	l := list.New(items, list.NewDefaultDelegate(), 30, 10) // valores temporales visibles
	l.Title = "Mocks loaded"
	l.KeyMap.Quit.SetEnabled(false) // reemplazado por quitKey: q/esc piden confirmación
	l.SetShowHelp(false)            // el help propio del list no trunca bien en anchos angostos (bug de la lib); usamos el nuestro en view.go

	return model{
		list:        l,
		currentMode: listMode,
		formStep:    0,
	}
}
