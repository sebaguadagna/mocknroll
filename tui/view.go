package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	warnStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	stepStyle   = lipgloss.NewStyle().Faint(true)
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
		content := fmt.Sprintf(
			"%s\n\nAre you sure you want to quit? (y/n)",
			warnStyle.Render("Quit mocknroll?"),
		)
		return borderStyle.Render(content)
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
		content := fmt.Sprintf(
			"%s  %s\n\n%s: %s\n\n(Type and press Enter to continue)\n[Esc: cancel]",
			headerStyle.Render("Creating new mock..."),
			stepStyle.Render(fmt.Sprintf("Step %d/%d", m.formStep+1, totalFormSteps)),
			label, value,
		)
		return borderStyle.Render(content)
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
				"%s\n\nPath:       %s\nMethod:     %s\nStatus:     %s\nDelay:      %s ms\nJSON File:  %s\n",
				headerStyle.Render("Details"),
				path,
				coloredMethod,
				selected.status,
				selected.delay,
				selected.jsonFile,
			)
		} else {
			detailView = "Seleccioná un mock para ver detalles"
		}

		rightStyle := rightBox
		if rightWidth := m.width - lipgloss.Width(left) - columnGap; rightWidth > 2 {
			rightStyle = rightStyle.Width(rightWidth - 2) // -2: ancho del borde izq/der
		}
		if leftHeight := lipgloss.Height(left); leftHeight > 2 {
			rightStyle = rightStyle.Height(leftHeight - 2) // -2: alto del borde sup/inf
		}
		right := rightStyle.Render(detailView)

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
