package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	// same style list.DefaultStyles().Title uses for "Mocks loaded" on
	// the main screen, so "Details" feels like the same kind of title.
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)

	// same hue as headerStyle's background (62), but as text color instead
	// of shading: for lines where we want "the same accent" without the
	// full pill (background + padding).
	shadeTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	// same color list.NewDefaultDelegate() uses for the selected item's
	// title on the main screen (SelectedTitle, #EE6FF8), so the spinner and
	// the form header feel like "the same accent".
	selectedAccent     = lipgloss.Color("#EE6FF8")
	spinnerStyle       = lipgloss.NewStyle().Foreground(selectedAccent)
	formHeaderStyle    = lipgloss.NewStyle().Bold(true).Foreground(selectedAccent)
	trafficStyle       = lipgloss.NewStyle().Foreground(selectedAccent)
	cursorStyle        = lipgloss.NewStyle().Reverse(true) // terminal-style block, same trick as bubbles/textinput
	stepStyle          = lipgloss.NewStyle().Faint(true)
	enabledStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	disabledStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	disabledTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // red row in the list for mocks disabled via "t"
	borderStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 2)

	// NO background: just border and text, so the popup doesn't cover the
	// list behind it with a solid color. Border in the same hue as
	// headerStyle (62), text in white so it reads well over whatever is
	// behind it. All of this popup's content has to stay WITHOUT its own
	// style (see toggleSpinner in model.go), so this single Render() is
	// the one that paints it start to finish, with no "patches" of another
	// color.
	togglePopupStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 3)

	// Same style as togglePopupStyle, but with a red border so it's
	// obvious this is a destructive confirmation.
	exitPopupStyle = togglePopupStyle.BorderForeground(lipgloss.Color("9"))
	columnGap      = 2

	// Used only in listMode: the content is bounded (Width/Height/MaxWidth/
	// MaxHeight) BEFORE adding the border, to guarantee the closing border
	// never gets clipped by accident (see the comment in View()).
	panelPadding     = lipgloss.NewStyle().Padding(1, 2)
	leftPanelBorder  = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	rightPanelBorder = lipgloss.NewStyle().Border(lipgloss.DoubleBorder())

	delayLowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))  // green: <= 30ms
	delayMidStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // orange: 31-150ms
	delayHighStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))   // red: > 150ms

	methodColor = map[string]string{
		"GET":    "10", // green
		"POST":   "13", // magenta
		"PUT":    "11", // yellow
		"DELETE": "9",  // red
	}
)

func (m model) View() tea.View {
	v := tea.NewView(m.renderView())
	v.AltScreen = true
	return v
}

func (m model) renderView() string {
	switch m.currentMode {
	case confirmExitMode:
		// Overlaid popup just like togglingMode: the content has no style of
		// its own, so exitPopupStyle's single Render() is the one that paints
		// the whole block.
		popup := exitPopupStyle.Render("Quit mocknroll?\n\nAre you sure you want to quit? (y/n)")
		return overlayCenter(m.renderListView(), popup, m.listWidth+2)
	case formMode:
		// Form mode
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
		// Same background as listMode, with the popup overlaid on top (see
		// overlayCenter): unlike confirmExitMode/formMode/provisioningMode,
		// here the user has to keep seeing the list behind it while the
		// toggle "applies".
		popup := togglePopupStyle.Render(fmt.Sprintf("%s %s", m.toggleSpinner.View(), m.toggleLabel))
		return overlayCenter(m.renderListView(), popup, m.listWidth+2)
	case listMode:
		return m.renderListView()
	}
	return ""
}

func (m model) renderListView() string {
	// Truncating by height AFTER adding the border can clip exactly the
	// closing border row (happened to us). So we bound the content
	// (padding included, no border) first with Width/Height/MaxWidth/
	// MaxHeight — all in the same units, no ambiguity — and only then do
	// we wrap the already-bounded result in the border, which means it
	// can never end up clipped.

	// Our own help line instead of the list's: bubbles/help (vendored)
	// doesn't truncate its help line well at narrow widths (it stops
	// adding the "…" and ends up rendering the full text with no limit),
	// which breaks the whole layout. lipgloss's Width() does real
	// word-wrap.
	// shadeTextStyle: same hue as the "Details"/"Mocks loaded" shading,
	// but as text color, without the background pill.
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

	// Right side: detail
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

		// The sparkline goes BEFORE "Response preview" on purpose: the
		// preview already self-limits to 6 lines + "…", but if it still
		// doesn't all fit in the panel, lipgloss truncates from the bottom
		// (MaxHeight further down) — this way what gets cut is the tail of
		// the preview, not the sparkline.
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
		detailView = "Select a mock to see details"
	}

	// m.listWidth+2: total already-rendered width of the left panel
	// (border included), to calculate how much is left available for the
	// right one.
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
	ms, _ := strconv.Atoi(delay) // non-numeric or empty value -> 0 (green)

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

// fieldWithCursor appends an editing cursor to a form field's value
// (always at the end: fields are only edited via append/backspace,
// never in the middle). Reserves the block's space even when it's
// "off" so the rest of the content doesn't jump while blinking.
func fieldWithCursor(value string, visible bool) string {
	if visible {
		return value + cursorStyle.Render(" ")
	}
	return value + " "
}

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkline renders buckets (per-interval counts) as a strip of Unicode
// block characters scaled to the slice's own max, one bucket per
// character (no axes or labels: meant to fit the left panel's narrow
// width).
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

// overlayCenter overlays fg on top of bg, centered horizontally within
// centerWidth (not bg's full width) and vertically across bg's whole
// height, using lipgloss v2's native Layer/Compositor (real z-index,
// no manual splicing of ANSI lines).
//
// centerWidth is the left panel's width (border included), not the
// full m.width: if we centered over both full columns, a popup with
// long text (e.g. the exit confirmation) would end up stepping on the
// right panel's border mid-row instead of staying contained within one
// of the two.
func overlayCenter(bg, fg string, centerWidth int) string {
	bgHeight := lipgloss.Height(bg)
	fgWidth := lipgloss.Width(fg)
	fgHeight := lipgloss.Height(fg)

	x := max(0, (centerWidth-fgWidth)/2)
	y := max(0, (bgHeight-fgHeight)/2)

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(bg),
		lipgloss.NewLayer(fg).X(x).Y(y).Z(1),
	).Render()
}
