package tui

import (
	"fmt"
	"os"
	"strconv"
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
	columnGap     = 2

	// Usados sólo en listMode: el contenido se acota (Width/Height/MaxWidth/
	// MaxHeight) ANTES de agregar el borde, para garantizar que el borde de
	// cierre nunca se recorte por accidente (ver comentario en View()).
	panelPadding     = lipgloss.NewStyle().Padding(1, 2)
	leftPanelBorder  = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	rightPanelBorder = lipgloss.NewStyle().Border(lipgloss.DoubleBorder())

	delayLowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))  // verde: <= 30ms
	delayMidStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // naranja: 31-150ms
	delayHighStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))   // rojo: > 150ms

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
		// Truncar por altura DESPUÉS de agregar el borde puede recortar justo
		// la fila del borde de cierre (nos pasó). Por eso acotamos el
		// contenido (padding incluido, sin borde) primero con Width/Height/
		// MaxWidth/MaxHeight —todo en las mismas unidades, sin ambigüedad—, y
		// recién then envolvemos el resultado YA acotado en el borde, que así
		// nunca puede terminar recortado.

		// Ayuda propia en vez de la del list: bubbles/help (vendored) no trunca
		// bien su línea de ayuda en anchos angostos (deja de agregar el "…" y
		// termina renderizando el texto completo sin límite), lo que rompe el
		// layout entero. Width() de lipgloss sí hace word-wrap real.
		hintLine := stepStyle.Render(fmt.Sprintf(
			"%s %s • / filter • %s %s • %s %s",
			addMockKey.Help().Key, addMockKey.Help().Desc,
			toggleEnabledKey.Help().Key, toggleEnabledKey.Help().Desc,
			quitKey.Help().Key, quitKey.Help().Desc,
		))
		listBody := m.list.View() + "\n\n" + hintLine
		leftContent := panelPadding.
			Width(m.listWidth).MaxWidth(m.listWidth).
			Height(m.listHeight).MaxHeight(m.listHeight).
			Render(listBody)
		left := leftPanelBorder.Render(leftContent)

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

		// m.listWidth+2: ancho total ya renderizado del panel izquierdo (borde
		// incluido), para calcular cuánto le queda disponible al derecho.
		rightContentWidth := m.width - (m.listWidth + 2) - columnGap - 2
		if rightContentWidth < 10 {
			rightContentWidth = 10
		}
		rightContent := panelPadding.
			Width(rightContentWidth).MaxWidth(rightContentWidth).
			Height(m.listHeight).MaxHeight(m.listHeight).
			Render(detailView)
		right := rightPanelBorder.Render(rightContent)

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
	ms, _ := strconv.Atoi(delay) // valor no numérico o vacío -> 0 (verde)

	style := delayLowStyle
	switch {
	case ms > 150:
		style = delayHighStyle
	case ms > 30:
		style = delayMidStyle
	}

	if delay == "" {
		return style.Render("Responds immediately")
	}
	return "Responds in " + style.Render(delay+"ms")
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
