package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	// mismo estilo que list.DefaultStyles().Title usa para "Mocks loaded" en
	// la pantalla principal, para que "Details" se sienta el mismo tipo de título.
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)

	// mismo tono que el fondo de headerStyle (62), pero como color de texto
	// en vez de sombreado: para líneas donde queremos "el mismo acento" sin
	// el pill completo (fondo + padding).
	shadeTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	// mismo color que list.NewDefaultDelegate() usa para el título del ítem
	// seleccionado en la pantalla principal (SelectedTitle, #EE6FF8), para que
	// el spinner y el encabezado del formulario se sientan "el mismo acento".
	selectedAccent     = lipgloss.Color("#EE6FF8")
	spinnerStyle       = lipgloss.NewStyle().Foreground(selectedAccent)
	formHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(selectedAccent)
	trafficStyle       = lipgloss.NewStyle().Foreground(selectedAccent)
	cursorStyle        = lipgloss.NewStyle().Reverse(true) // bloque tipo terminal, mismo truco que bubbles/textinput
	stepStyle          = lipgloss.NewStyle().Faint(true)
	enabledStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	disabledStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	disabledTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // fila roja en la lista para mocks con "t" desactivado
	borderStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)

	// SIN fondo: sólo borde y texto, para que el popup no tape la lista de
	// atrás con un color sólido. Borde en el mismo tono que headerStyle (62),
	// texto en blanco para que se lea bien encima de lo que sea que haya
	// detrás. Todo el contenido de este popup tiene que quedar SIN estilo
	// propio (ver toggleSpinner en model.go), para que este único Render()
	// sea el que pinte de punta a punta y no queden "parches" de otro color.
	togglePopupStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 3)

	// Mismo estilo que togglePopupStyle, pero con el borde en rojo para que
	// se note que es una confirmación destructiva.
	exitPopupStyle = togglePopupStyle.BorderForeground(lipgloss.Color("9"))
	columnGap      = 2

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
		// Popup superpuesto igual que togglingMode: el contenido va sin
		// estilo propio para que el único Render() de exitPopupStyle sea el
		// que pinte todo el bloque.
		popup := exitPopupStyle.Render("Quit mocknroll?\n\nAre you sure you want to quit? (y/n)")
		return overlayCenter(m.renderListView(), popup, m.listWidth+2)
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
		percent := float64(m.formStep+1) / float64(totalFormSteps)
		content := fmt.Sprintf(
			"%s %s  %s\n\n%s\n\n%s: %s\n\n(Type and press Enter to continue)\n[Esc: cancel]",
			m.spinner.View(),
			formHeaderStyle.Render("Creating new mock..."),
			stepStyle.Render(fmt.Sprintf("Step %d/%d", m.formStep+1, totalFormSteps)),
			m.progress.ViewAs(percent),
			label, fieldWithCursor(value, m.cursorVisible),
		)
		return borderStyle.Render(content)
	case provisioningMode:
		content := fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			formHeaderStyle.Render("Configuring new mock..."),
			stepStyle.Render("Reloading the mock server so it can serve "+m.pendingMock.title),
			m.provisionProgress.View(),
		)
		return borderStyle.Render(content)
	case togglingMode:
		// Mismo fondo que listMode, con el popup superpuesto encima (ver
		// overlayCenter): a diferencia de confirmExitMode/formMode/
		// provisioningMode, acá el usuario tiene que seguir viendo la lista
		// detrás mientras se "aplica" el toggle.
		popup := togglePopupStyle.Render(fmt.Sprintf("%s %s", m.toggleSpinner.View(), m.toggleLabel))
		return overlayCenter(m.renderListView(), popup, m.listWidth+2)
	case listMode:
		return m.renderListView()
	}
	return ""
}

func (m model) renderListView() string {
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
	// shadeTextStyle: mismo tono que el sombreado de "Details"/"Mocks loaded",
	// pero como color de texto, sin el pill de fondo.
	hintLine := shadeTextStyle.Render(fmt.Sprintf(
		"%s: %s • filter: / • %s: %s • %s: %s",
		addMockKey.Help().Desc, addMockKey.Help().Key,
		toggleEnabledKey.Help().Desc, toggleEnabledKey.Help().Key,
		quitKey.Help().Desc, quitKey.Help().Key,
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

		// El sparkline va ANTES de "Response preview" a propósito: el
		// preview ya se autolimita a 6 líneas + "…", pero si igual no
		// entra todo en el panel, lipgloss trunca por abajo (MaxHeight
		// más adelante) — así lo que se corta es la cola del preview, no
		// el sparkline.
		trafficLine := fmt.Sprintf(
			"%s %s %s",
			stepStyle.Render("Requests (5m):"),
			trafficStyle.Render(sparkline(selected.trafficBuckets)),
			stepStyle.Render(fmt.Sprintf("%d total", sumInts(selected.trafficBuckets))),
		)

		detailView = fmt.Sprintf(
			"%s\n\n%s %s\n%s\n\n%s   %s\nStatus:     %s\nJSON File:  %s\n\n%s\n\nResponse preview:\n%s",
			headerStyle.Render("Details"),
			coloredMethod,
			path,
			selected.description,
			statusBadge,
			delayText(selected.delay),
			selected.status,
			selected.jsonFile,
			trafficLine,
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

// fieldWithCursor le agrega al valor de un campo del formulario un cursor de
// edición al final (siempre al final: los campos sólo se editan por
// append/backspace, nunca en medio). Reserva el espacio del bloque aunque
// esté "apagado" para que el resto del contenido no salte al parpadear.
func fieldWithCursor(value string, visible bool) string {
	if visible {
		return value + cursorStyle.Render(" ")
	}
	return value + " "
}

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkline renderiza buckets (conteos por intervalo) como una franja de
// caracteres de bloque Unicode escalada al máximo del propio slice, un bucket
// por carácter (sin ejes ni labels: pensado para caber en el ancho angosto
// del panel izquierdo).
func sparkline(buckets []int) string {
	if len(buckets) == 0 {
		return ""
	}
	max := 0
	for _, v := range buckets {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		max = 1
	}
	runes := make([]rune, len(buckets))
	for i, v := range buckets {
		idx := v * (len(sparkBlocks) - 1) / max
		runes[i] = sparkBlocks[idx]
	}
	return string(runes)
}

func sumInts(vals []int) int {
	total := 0
	for _, v := range vals {
		total += v
	}
	return total
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

// overlayCenter superpone fg sobre bg, centrado horizontalmente dentro de
// centerWidth (no del ancho total de bg) y verticalmente en toda la altura de
// bg. lipgloss (v1, la versión acá vendorizada) no tiene compositing con
// z-index, así que esto empalma línea a línea "a mano": cada fila de bg se
// recorta con ansi.Cut, que —a diferencia de un slice de string plano—
// entiende los códigos ANSI y no los rompe ni los pierde, así que el
// color/borde de lo que quede a los costados de fg sigue intacto.
//
// centerWidth es el ancho del panel izquierdo (con borde), no m.width entero:
// si centráramos sobre las dos columnas completas, un popup con texto largo
// (p.ej. la confirmación de salida) termina pisando el borde del panel
// derecho a la mitad de una fila en vez de quedar contenido en uno de los dos.
func overlayCenter(bg, fg string, centerWidth int) string {
	bgHeight := lipgloss.Height(bg)
	fgWidth := lipgloss.Width(fg)
	fgHeight := lipgloss.Height(fg)

	x := max(0, (centerWidth-fgWidth)/2)
	y := max(0, (bgHeight-fgHeight)/2)

	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	for i, fgLine := range fgLines {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		fgLineWidth := lipgloss.Width(fgLine)

		bgLine := bgLines[row]
		bgLineWidth := lipgloss.Width(bgLine)
		if pad := x + fgLineWidth - bgLineWidth; pad > 0 {
			// La fila de fondo termina antes de donde arranca el popup (pasa
			// en las últimas filas, más cortas que el resto del panel): la
			// completamos con espacios en blanco antes de cortarla.
			bgLine += strings.Repeat(" ", pad)
			bgLineWidth += pad
		}

		left := ansi.Cut(bgLine, 0, x)
		right := ansi.Cut(bgLine, x+fgLineWidth, bgLineWidth)
		bgLines[row] = left + fgLine + right
	}
	return strings.Join(bgLines, "\n")
}
