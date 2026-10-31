package tui

import (
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/sebaguadagna/mocknroll/server"
)

// Constants for the form steps
const (
	formStepPath = iota
	formStepMethod
	formStepStatus
	formStepDelay
	formStepJSONFile
	totalFormSteps
)

// trafficTickMsg pulls the server's request counts into the mocks'
// sparklines, one second at a time.
type trafficTickMsg time.Time

func trafficTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return trafficTickMsg(t)
	})
}

// cursorTickMsg toggles the text cursor's visibility in the form's active
// field (same interval bubbles/textinput uses by default).
type cursorTickMsg time.Time

const cursorBlinkInterval = 530 * time.Millisecond

func cursorTick() tea.Cmd {
	return tea.Tick(cursorBlinkInterval, func(t time.Time) tea.Msg {
		return cursorTickMsg(t)
	})
}

// provisionTickMsg advances provisioningMode's animated bar. Same pattern
// as bubbletea's progress-animated example: each tick adds
// provisionIncrement to an UNclamped accumulator; once it passes 1.0 (one
// tick after visually reaching 100%, giving the spring time to settle)
// the screen closes and the pending mock gets inserted. The wait is purely
// a visual effect: the server picks up new mocks instantly.
type provisionTickMsg time.Time

const (
	provisionTickInterval = time.Second
	provisionIncrement    = 0.25 // 4 ticks to reach 1.0 + 1 tick of margin ≈ 5s total
)

func provisionTick() tea.Cmd {
	return tea.Tick(provisionTickInterval, func(t time.Time) tea.Msg {
		return provisionTickMsg(t)
	})
}

// toggleDismissMsg closes togglingMode's popup. Unlike this file's other
// ticks, it's a one-shot (it doesn't reschedule itself): a plain timer is
// enough, no step-by-step animation needed.
type toggleDismissMsg time.Time

const toggleDismissDelay = time.Second

func toggleDismissAfter() tea.Cmd {
	return tea.Tick(toggleDismissDelay, func(t time.Time) tea.Msg {
		return toggleDismissMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	m.list.SetSize(120, 30)
	return trafficTick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		// Two independent spinners share this message type (bubbles allows it:
		// each spinner.Model has its own id and only reacts to the ticks it
		// started). Each one only reschedules itself while it's still the
		// active mode; otherwise we let that chain die.
		var cmds []tea.Cmd
		if m.currentMode == formMode {
			var c tea.Cmd
			m.spinner, c = m.spinner.Update(msg)
			cmds = append(cmds, c)
		}
		if m.currentMode == togglingMode {
			var c tea.Cmd
			m.toggleSpinner, c = m.toggleSpinner.Update(msg)
			cmds = append(cmds, c)
		}
		return m, tea.Batch(cmds...)

	case toggleDismissMsg:
		if m.currentMode == togglingMode {
			m.currentMode = listMode
		}
		return m, nil

	case cursorTickMsg:
		if m.currentMode != formMode {
			// let the tick chain die, same as with the spinner.
			return m, nil
		}
		m.cursorVisible = !m.cursorVisible
		return m, cursorTick()

	case list.FilterMatchesMsg:
		// Handled in any mode: trafficTick refreshes the filter every second,
		// and dropping the result would leave the filtered view empty.
		m.list, cmd = m.list.Update(msg)
		return m, cmd

	case provisionTickMsg:
		if m.currentMode != provisioningMode {
			// let the tick chain die, same as with the spinner/cursor.
			return m, nil
		}
		m.provisionPercent += provisionIncrement
		if m.provisionPercent > 1.0 {
			// The bar has already arrived (and had a margin tick to settle
			// visually): close the screen and insert the mock.
			if m.editing {
				// Taken now, not on submit, so traffic received during the
				// animation isn't lost. A new route starts from the zeroed
				// buckets already in pendingMock.
				current := m.list.Items()[m.editIndex].(mockItem)
				if current.title == m.pendingMock.title {
					m.pendingMock.trafficBuckets = current.trafficBuckets
				}
				cmd = m.list.SetItem(m.editIndex, m.pendingMock)
				m.editing = false
			} else {
				cmd = m.list.InsertItem(len(m.list.Items()), m.pendingMock)
			}
			m.updateServer() // update server registry with newly provisioned mock
			m.pendingMock = mockItem{}
			m.provisionPercent = 0
			m.currentMode = listMode
			return m, cmd
		}
		cmd = m.provisionProgress.SetPercent(m.provisionPercent)
		return m, tea.Batch(cmd, provisionTick())

	case progress.FrameMsg:
		if m.currentMode != provisioningMode {
			return m, nil
		}
		newProgress, cmd := m.provisionProgress.Update(msg)
		m.provisionProgress = newProgress
		return m, cmd

	case trafficTickMsg:
		// Runs in any mode: traffic arrives at the server regardless of
		// what the user is looking at in the TUI. It's per-mock (each
		// mocked API has its own history), but they all roll buckets
		// together, so the 10s cutoff is shared.
		m.trafficElapsed++
		roll := m.trafficElapsed >= trafficBucketDuration
		if roll {
			m.trafficElapsed = 0
		}

		counts := server.GetAndResetRequestCounts()
		m.requestLog = server.RecentRequests()

		items := m.list.Items()
		for i, it := range items {
			mi := it.(mockItem)
			if len(mi.trafficBuckets) == 0 {
				items[i] = mi
				continue
			}
			if mi.enabled {
				last := len(mi.trafficBuckets) - 1
				mi.trafficBuckets[last] += counts[mi.title]
			}
			if roll {
				mi.trafficBuckets = append(mi.trafficBuckets[1:], 0)
			}
			items[i] = mi
		}
		cmd = m.list.SetItems(items)
		return m, tea.Batch(cmd, trafficTick())

	case tea.WindowSizeMsg:
		// listWidth/listHeight: content width/height WITH padding for the left
		// panel (what gets passed to lipgloss Width()/Height() in view.go).
		// listHeight is shared by both panels.
		m.width = msg.Width
		m.height = msg.Height
		m.listWidth = msg.Width * 6 / 10 // the list is the main navigation surface
		m.listHeight = msg.Height - 3    // -3: border (2) + 1 line of safety margin
		// What we pass to the list is smaller: we subtract our own padding
		// (2 cols/rows per side) and 2 rows for our own help line.
		m.list.SetSize(m.listWidth-4, m.listHeight-4)
		return m, nil

	case tea.KeyPressMsg:
		switch m.currentMode {

		// LIST MODE
		case listMode:
			switch msg.String() {
			case "q", "Q", "esc":
				if m.list.FilterState() == list.Filtering {
					break // let the list handle the key (typing "q" or canceling the filter with esc)
				}
				m.currentMode = confirmExitMode
				return m, nil
			case "ctrl+c":
				m.currentMode = confirmExitMode
				return m, nil
			case "a":
				if m.list.FilterState() == list.Filtering {
					break // let the list write the "a" into the filter
				}
				m.currentMode = formMode
				m.formStep = formStepPath
				m.cursorVisible = true
				return m, tea.Batch(m.spinner.Tick, cursorTick())
			case "r":
				if m.list.FilterState() == list.Filtering {
					break // let the list write the "r" into the filter
				}
				m.showRequestLog = !m.showRequestLog
				return m, nil
			case "e":
				if m.list.FilterState() == list.Filtering {
					break // let the list write the "e" into the filter
				}
				if selected, ok := m.list.SelectedItem().(mockItem); ok {
					m.editing = true
					m.editIndex = m.list.GlobalIndex()
					m.formMethod, m.formPath = splitTitle(selected.title)
					m.formStatus = selected.status
					m.formDelay = selected.delay
					m.formJSONFile = selected.jsonFile
					m.currentMode = formMode
					m.formStep = formStepPath
					m.cursorVisible = true
					return m, tea.Batch(m.spinner.Tick, cursorTick())
				}
			case "d":
				if m.list.FilterState() == list.Filtering {
					break // let the list write the "d" into the filter
				}
				if selected, ok := m.list.SelectedItem().(mockItem); ok {
					m.deleteIndex = m.list.GlobalIndex()
					m.deleteTitle = selected.title
					m.currentMode = confirmDeleteMode
				}
				return m, nil // never reaches the list, which binds "d" to next page
			case "t":
				if m.list.FilterState() == list.Filtering {
					break // let the list write the "t" into the filter
				}
				if selected, ok := m.list.SelectedItem().(mockItem); ok {
					selected.enabled = !selected.enabled
					cmdSet := m.list.SetItem(m.list.GlobalIndex(), selected) // GlobalIndex: Index() points into the filtered view when a filter is active
					m.updateServer()                                         // update server registry with modified enabled status

					m.currentMode = togglingMode
					if selected.enabled {
						m.toggleLabel = "Enabling..."
					} else {
						m.toggleLabel = "Disabling..."
					}
					return m, tea.Batch(cmdSet, m.toggleSpinner.Tick, toggleDismissAfter())
				}
			}

		// CONFIRM DELETE MODE
		case confirmDeleteMode:
			switch msg.String() {
			case "y", "Y":
				m.deleteConfirmedMock()
				m.updateServer() // stop serving the deleted mock
				m.currentMode = listMode
				return m, nil
			case "n", "N", "esc":
				m.currentMode = listMode
				return m, nil
			}

		// CONFIRM EXIT MODE
		case confirmExitMode:
			switch msg.String() {
			case "y", "Y":
				// Confirm exit
				return m, tea.Quit
			case "n", "N", "esc":
				// Cancel exit and go back to list mode
				m.currentMode = listMode
				return m, nil
			}

		// FORM MODE
		case formMode:
			switch msg.Code {
			case tea.KeyEsc:
				m.currentMode = listMode
				m.editing = false
				m.resetForm()
				return m, nil
			case tea.KeyEnter:
				m.cursorVisible = true // reappears right away in the next field, instead of starting mid-blink
				if err := m.commitFormStep(); err != nil {
					m.formError = err.Error()
					return m, nil
				}
				m.formError = ""
				switch m.formStep {
				case formStepPath:
					m.formStep = formStepMethod
				case formStepMethod:
					m.formStep = formStepStatus
				case formStepStatus:
					m.formStep = formStepDelay
				case formStepDelay:
					m.formStep = formStepJSONFile
				case formStepJSONFile:
					// The mock is built but not inserted yet: it's stored in
					// pendingMock and only added to the list once provisioningMode
					// finishes (see provisionTickMsg above).
					m.pendingMock = mockItem{
						title:          m.formMethod + " " + m.formPath,
						description:    autoDescription(m.formStatus, m.formDelay),
						status:         m.formStatus,
						delay:          m.formDelay,
						jsonFile:       m.formJSONFile,
						enabled:        true,
						trafficBuckets: make([]int, trafficBucketCount), // freshly created mock: no traffic yet
					}

					if m.editing {
						old := m.list.Items()[m.editIndex].(mockItem)
						m.pendingMock.enabled = old.enabled
						if old.description != autoDescription(old.status, old.delay) {
							m.pendingMock.description = old.description
						}
					}

					m.currentMode = provisioningMode
					m.provisionPercent = 0
					m.provisionProgress = newProvisionProgress()
					m.resetForm()
					return m, provisionTick()
				}
			case tea.KeyBackspace:
				m.cursorVisible = true // doesn't stay "off" mid-blink while editing
				m.formError = ""
				// Allow deleting a character
				switch m.formStep {
				case formStepPath:
					if len(m.formPath) > 0 {
						m.formPath = m.formPath[:len(m.formPath)-1]
					}
				case formStepMethod:
					if len(m.formMethod) > 0 {
						m.formMethod = m.formMethod[:len(m.formMethod)-1]
					}
				case formStepStatus:
					if len(m.formStatus) > 0 {
						m.formStatus = m.formStatus[:len(m.formStatus)-1]
					}
				case formStepDelay:
					if len(m.formDelay) > 0 {
						m.formDelay = m.formDelay[:len(m.formDelay)-1]
					}
				case formStepJSONFile:
					if len(m.formJSONFile) > 0 {
						m.formJSONFile = m.formJSONFile[:len(m.formJSONFile)-1]
					}
				}

			default:
				// Only printable text goes into the fields; any other special key
				// (arrows, tab, ctrl+c, F1, ...) is ignored instead of being
				// inserted as literal text (that was the bug: "up", "tab", etc.
				// used to end up inside the field). msg.Text is only populated
				// for keys that represent printable characters.
				if msg.Text == "" {
					break
				}
				m.cursorVisible = true // same idea: visible right away when typing, not mid-blink
				m.formError = ""
				switch m.formStep {
				case formStepPath:
					m.formPath += msg.Text
				case formStepMethod:
					m.formMethod += msg.Text
				case formStepStatus:
					m.formStatus += msg.Text
				case formStepDelay:
					m.formDelay += msg.Text
				case formStepJSONFile:
					m.formJSONFile += msg.Text
				}
			}
		}
	}

	if m.currentMode == listMode {
		m.list, cmd = m.list.Update(msg)
	}

	return m, cmd
}

func (m *model) resetForm() {
	m.formPath, m.formMethod, m.formStatus, m.formDelay, m.formJSONFile = "", "", "", "", ""
	m.formStep = formStepPath
	m.formError = ""
}

// deleteConfirmedMock removes the mock at deleteIndex and keeps the selection
// valid. An applied filter is re-run afterwards because list.RemoveItem trims
// its matches by the unfiltered index, which can drop the wrong row.
func (m *model) deleteConfirmedMock() {
	cursor := m.list.Index()
	filterApplied := m.list.FilterState() == list.FilterApplied
	filterText := m.list.FilterInput.Value()

	m.list.RemoveItem(m.deleteIndex)

	if filterApplied {
		m.list.SetFilterText(filterText)
		if len(m.list.VisibleItems()) == 0 {
			m.list.ResetFilter() // it was the only match
		}
	}
	m.list.Select(max(0, min(cursor, len(m.list.VisibleItems())-1)))
}
