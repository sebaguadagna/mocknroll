package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Init() tea.Cmd {
	m.list.SetSize(120, 30)
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		listWidth := msg.Width / 2
		listHeight := msg.Height - 4
		m.list.SetSize(listWidth, listHeight)
		return m, nil

	case tea.KeyMsg:
		switch m.currentMode {

		// LIST MODE
		case listMode:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "a":
				m.currentMode = formMode
				m.formStep = 0
				return m, nil

			}

		// FORM MODE
		case formMode:
			switch msg.Type {
			case tea.KeyEsc:
				m.currentMode = listMode
				m.formPath, m.formMethod, m.formStatus, m.formDelay, m.formJSONFile = "", "", "", "", ""
				m.formStep = 0
				return m, nil
			case tea.KeyEnter:
				switch m.formStep {
				case 0:
					m.formStep++
				case 1:
					m.formStep++
				case 2:
					m.formStep++
				case 3:
					m.formStep++
				case 4:
					// Crear nuevo mockItem
					newItem := mockItem{
						title:       m.formMethod + " " + m.formPath,
						description: "Status: " + m.formStatus + ", Delay: " + m.formDelay + "ms",
						status:      m.formStatus,
						delay:       m.formDelay,
						jsonFile:    m.formJSONFile,
					}
					cmd = tea.Batch(cmd, m.list.InsertItem(len(m.list.Items()), newItem))

					// Reset
					m.currentMode = listMode
					m.formPath, m.formMethod, m.formStatus, m.formDelay, m.formJSONFile = "", "", "", "", ""
					m.formStep = 0
					return m, cmd
				}
			case tea.KeyBackspace:
				// Permitir borrar carácter
				switch m.formStep {
				case 0:
					if len(m.formPath) > 0 {
						m.formPath = m.formPath[:len(m.formPath)-1]
					}
				case 1:
					if len(m.formMethod) > 0 {
						m.formMethod = m.formMethod[:len(m.formMethod)-1]
					}
				case 2:
					if len(m.formStatus) > 0 {
						m.formStatus = m.formStatus[:len(m.formStatus)-1]
					}
				case 3:
					if len(m.formDelay) > 0 {
						m.formDelay = m.formDelay[:len(m.formDelay)-1]
					}
				case 4:
					if len(m.formJSONFile) > 0 {
						m.formJSONFile = m.formJSONFile[:len(m.formJSONFile)-1]
					}
				}

			default:
				// Agregar letras a cada campo
				switch m.formStep {
				case 0:
					m.formPath += msg.String()
				case 1:
					m.formMethod += msg.String()
				case 2:
					m.formStatus += msg.String()
				case 3:
					m.formDelay += msg.String()
				case 4:
					m.formJSONFile += msg.String()
				}
			}
		}
	}

	if m.currentMode == listMode {
		m.list, cmd = m.list.Update(msg)
	}

	return m, cmd
}
