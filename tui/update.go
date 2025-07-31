package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Constantes para los pasos del formulario
const (
	formStepPath = iota
	formStepMethod
	formStepStatus
	formStepDelay
	formStepJSONFile
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
				m.formStep = formStepPath
				return m, nil

			}

		// FORM MODE
		case formMode:
			switch msg.Type {
			case tea.KeyEsc:
				m.currentMode = listMode
				m.formPath, m.formMethod, m.formStatus, m.formDelay, m.formJSONFile = "", "", "", "", ""
				m.formStep = formStepPath
				return m, nil
			case tea.KeyEnter:
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
					m.formStep = formStepPath
					return m, cmd
				}
			case tea.KeyBackspace:
				// Permitir borrar carácter
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
				// Agregar letras a cada campo
				switch m.formStep {
				case formStepPath:
					m.formPath += msg.String()
				case formStepMethod:
					m.formMethod += msg.String()
				case formStepStatus:
					m.formStatus += msg.String()
				case formStepDelay:
					m.formDelay += msg.String()
				case formStepJSONFile:
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
