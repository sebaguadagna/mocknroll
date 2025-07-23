package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	borderStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	sectionStyle = lipgloss.NewStyle().Margin(1, 2, 1, 2)
	columnGap    = 2
)

func (m model) View() string {
	// Modo formulario
	if m.currentMode == formMode {
		label := ""
		value := ""
		switch m.formStep {
		case 0:
			label = "Path"
			value = m.formPath
		case 1:
			label = "Method"
			value = m.formMethod
		case 2:
			label = "Status"
			value = m.formStatus
		case 3:
			label = "Delay (ms)"
			value = m.formDelay
		case 4:
			label = "JSON File"
			value = m.formJSONFile
		}
		return fmt.Sprintf(
			"\n📝 Creando nuevo mock...\n\n%s: %s\n\n(Escribí y presioná Enter para continuar)\n[Esc: cancelar]",
			label, value,
		)
	}

	// Parte izquierda: lista
	listTitle := titleStyle.Render(m.list.Title)
	listWithTitle := listTitle + "\n\n" + m.list.View()
	left := borderStyle.Render(listWithTitle)

	// Parte derecha: detalle
	selected, ok := m.list.SelectedItem().(mockItem)
	var detailView string
	if ok {
		detailView = fmt.Sprintf(
			"Path:       %s\nMethod:     %s\nStatus:     %s\nDelay:      %s ms\nJSON File:  %s\n",
			m.formPathIfEmpty(selected),
			m.formMethodIfEmpty(selected),
			m.formStatus,
			m.formDelay,
			m.formJSONFile,
		)
	} else {
		detailView = "Seleccioná un mock para ver detalles"
	}
	right := borderStyle.Render(sectionStyle.Render(detailView))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", columnGap), right)
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
