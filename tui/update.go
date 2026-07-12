package tui

import (
	"math/rand"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Constantes para los pasos del formulario
const (
	formStepPath = iota
	formStepMethod
	formStepStatus
	formStepDelay
	formStepJSONFile
	totalFormSteps
)

// trafficTickMsg simula la llegada de requests a los mocks servidos, un
// segundo a la vez. Cuando server.go sirva tráfico real, esto se reemplaza
// por eventos genuinos en vez de un tea.Tick.
type trafficTickMsg time.Time

func trafficTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return trafficTickMsg(t)
	})
}

// cursorTickMsg alterna la visibilidad del cursor de texto en el campo activo
// del formulario (mismo intervalo que usa bubbles/textinput por defecto).
type cursorTickMsg time.Time

const cursorBlinkInterval = 530 * time.Millisecond

func cursorTick() tea.Cmd {
	return tea.Tick(cursorBlinkInterval, func(t time.Time) tea.Msg {
		return cursorTickMsg(t)
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
		if m.currentMode != formMode {
			// dejar morir la cadena de ticks: no la seguimos re-programando
			// una vez que salimos del formulario.
			return m, nil
		}
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case cursorTickMsg:
		if m.currentMode != formMode {
			// dejar morir la cadena de ticks, igual que con el spinner.
			return m, nil
		}
		m.cursorVisible = !m.cursorVisible
		return m, cursorTick()

	case trafficTickMsg:
		// Corre en cualquier modo: el tráfico "le llega al server" sin
		// importar qué esté mirando el usuario en la TUI. Es por mock (cada
		// API mockeada tiene su propio historial), pero todos rotan de
		// bucket juntos, así que el corte de los 10s es compartido.
		m.trafficElapsed++
		roll := m.trafficElapsed >= trafficBucketDuration
		if roll {
			m.trafficElapsed = 0
		}

		items := m.list.Items()
		for i, it := range items {
			mi := it.(mockItem)
			if len(mi.trafficBuckets) == 0 {
				items[i] = mi
				continue
			}
			if mi.enabled {
				last := len(mi.trafficBuckets) - 1
				mi.trafficBuckets[last] += rand.Intn(4) // 0-3 requests simulados este segundo, por mock
			}
			if roll {
				mi.trafficBuckets = append(mi.trafficBuckets[1:], 0)
			}
			items[i] = mi
		}
		cmd = m.list.SetItems(items)
		return m, tea.Batch(cmd, trafficTick())

	case tea.WindowSizeMsg:
		// listWidth/listHeight: ancho/alto de CONTENIDO CON padding para el panel
		// izquierdo (lo que se le pasa a lipgloss Width()/Height() en view.go).
		// listHeight lo comparten ambos paneles.
		m.width = msg.Width
		m.listWidth = msg.Width * 6 / 10 // la lista es la superficie de navegación principal
		m.listHeight = msg.Height - 3    // -3: borde (2) + 1 línea de margen de seguridad
		// Lo que le pasamos al list es más chico: descontamos nuestro propio
		// padding (2 cols/filas por lado) y 2 filas para nuestra línea de ayuda.
		m.list.SetSize(m.listWidth-4, m.listHeight-4)
		return m, nil

	case tea.KeyMsg:
		switch m.currentMode {

		// LIST MODE
		case listMode:
			switch msg.String() {
			case "q", "Q", "esc":
				if m.list.FilterState() == list.Filtering {
					break // dejar que el list maneje la tecla (escribir "q" o cancelar el filtro con esc)
				}
				m.currentMode = confirmExitMode
				return m, nil
			case "ctrl+c":
				m.currentMode = confirmExitMode
				return m, nil
			case "a":
				m.currentMode = formMode
				m.formStep = formStepPath
				m.cursorVisible = true
				return m, tea.Batch(m.spinner.Tick, cursorTick())
			case "t":
				if m.list.FilterState() == list.Filtering {
					break // dejar que el list escriba la "t" en el filtro
				}
				if selected, ok := m.list.SelectedItem().(mockItem); ok {
					selected.enabled = !selected.enabled
					return m, m.list.SetItem(m.list.Index(), selected)
				}
			}

		// CONFIRM EXIT MODE
		case confirmExitMode:
			switch msg.String() {
			case "y", "Y":
				// Confirmar salida
				return m, tea.Quit
			case "n", "N", "esc":
				// Cancelar salida y volver al modo lista
				m.currentMode = listMode
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
				m.cursorVisible = true // reaparece de entrada en el campo siguiente, en vez de arrancar a mitad de parpadeo
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
						title:          m.formMethod + " " + m.formPath,
						description:    "Status: " + m.formStatus + ", Delay: " + m.formDelay + "ms",
						status:         m.formStatus,
						delay:          m.formDelay,
						jsonFile:       m.formJSONFile,
						enabled:        true,
						trafficBuckets: make([]int, trafficBucketCount), // mock recién creado: sin tráfico todavía
					}
					cmd = tea.Batch(cmd, m.list.InsertItem(len(m.list.Items()), newItem))

					// Reset
					m.currentMode = listMode
					m.formPath, m.formMethod, m.formStatus, m.formDelay, m.formJSONFile = "", "", "", "", ""
					m.formStep = formStepPath
					return m, cmd
				}
			case tea.KeyBackspace:
				m.cursorVisible = true // no queda "apagado" a mitad de parpadeo mientras se edita
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
				// Solo texto imprimible (runas sueltas o espacio) va a los campos;
				// cualquier otra tecla especial (flechas, tab, ctrl+c, F1, ...) se ignora
				// en vez de insertarse como texto literal (era el bug: "up", "tab", etc.
				// terminaban dentro del campo).
				if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
					break
				}
				m.cursorVisible = true // idem: visible de entrada al tipear, no a mitad de parpadeo
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
