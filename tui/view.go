package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	//titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	borderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)
	rightBox    = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(1, 2)
	columnGap   = 2

	methodColor = map[string]string{
		"GET":    "10", // verde
		"POST":   "13", // magenta
		"PUT":    "11", // amarillo
		"DELETE": "9",  // rojo
	}
)

func (m model) View() string {
	switch m.currentMode {
	case confirmExitMode:
		return "Are you sure you want to quit? (y/n)"
	case formMode:
		// Modo formulario
		label := ""
		value := ""
		switch m.formStep {
		case formStepPath:
			label = "Path"
			value = m.formPath
		case formStepMethod:
			label = "Method"
			value = m.formMethod
		case formStepStatus:
			label = "Status"
			value = m.formStatus
		case formStepDelay:
			label = "Delay (ms)"
			value = m.formDelay
		case formStepJSONFile:
			label = "JSON File"
			value = m.formJSONFile
		}
		return fmt.Sprintf(
			"\n📝 Creating new mock...\n\n%s: %s\n\n(Type and press Enter to continue)\n[Esc: cancel]",
			label, value,
		)
	case listMode:
		// Parte izquierda: lista con título
		left := borderStyle.Render(m.list.View())

		// Parte derecha: detalle
		selected, ok := m.list.SelectedItem().(mockItem)
		var detailView string
		if ok {
			method := m.formMethodIfEmpty(selected)
			path := m.formPathIfEmpty(selected)
			color := methodColor[method]
			if color == "" {
				color = "7"
			}

			coloredMethod := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(method)

			detailView = fmt.Sprintf(
				"🔍 Details\n\nPath:       %s\nMethod:     %s\nStatus:     %s\nDelay:      %s ms\nJSON File:  %s\n",
				path,
				coloredMethod,
				selected.status,
				selected.delay,
				selected.jsonFile,
			)
		} else {
			detailView = "Seleccioná un mock para ver detalles"
		}
		right := rightBox.Render(detailView)

		return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", columnGap), right)
	}
	return ""
}

func (m model) formPathIfEmpty(item mockItem) string {
	if m.formPath != "" {
		return m.formPath
	}
	split := strings.SplitN(item.Title(), " ", 2)
	if len(split) == 2 {
		return split[1]
	}
	return ""
}

func (m model) formMethodIfEmpty(item mockItem) string {
	if m.formMethod != "" {
		return m.formMethod
	}
	split := strings.SplitN(item.Title(), " ", 2)
	if len(split) > 0 {
		return split[0]
	}
	return ""
}
