package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	warnStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	stepStyle     = lipgloss.NewStyle().Faint(true)
	enabledStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	disabledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)
	rightBox      = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Padding(1, 2)
	columnGap     = 2

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

			statusBadge := enabledStyle.Render("● Enabled")
			if !selected.enabled {
				statusBadge = disabledStyle.Render("● Disabled")
			}

			detailView = fmt.Sprintf(
				"%s\n\n%s %s\n%s\n\n%s   %s\nStatus:     %s\nJSON File:  %s\n\nResponse preview:\n%s",
				headerStyle.Render("Details"),
				coloredMethod,
				path,
				selected.description,
				statusBadge,
				delayText(selected.delay),
				selected.status,
				selected.jsonFile,
				stepStyle.Render(previewJSON(selected.jsonFile)),
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
	split := strings.SplitN(item.title, " ", 2)
	if len(split) == 2 {
		return split[1]
	}
	return ""
}

func (m model) formMethodIfEmpty(item mockItem) string {
	if m.formMethod != "" {
		return m.formMethod
	}
	split := strings.SplitN(item.title, " ", 2)
	if len(split) > 0 {
		return split[0]
	}
	return ""
}

func delayText(delay string) string {
	if delay == "" {
		return "Responds immediately"
	}
	return fmt.Sprintf("Responds in %sms", delay)
}

func previewJSON(path string) string {
	if path == "" {
		return "(no response file configured)"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(couldn't read %s)", path)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	truncated := len(lines) > 6
	if truncated {
		lines = lines[:6]
	}
	out := strings.Join(lines, "\n")
	if truncated {
		out += "\n…"
	}
	return out
}
