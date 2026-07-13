package tui

import (
	"math/rand"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
)

// trafficBucketCount * trafficBucketDuration = ventana total mostrada (5 min).
// TODO: reemplazar por conteo real de requests una vez que server.go sirva
// los mocks; por ahora se simula tráfico para probar la visualización.
const (
	trafficBucketCount    = 30
	trafficBucketDuration = 10 // segundos por bucket
)

var (
	addMockKey = key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "add mock"),
	)
	quitKey = key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "quit"),
	)
	toggleEnabledKey = key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "toggle enabled"),
	)
)

type mode int

const (
	listMode         mode = iota //0
	formMode                     //1
	confirmExitMode              // nuevo modo para confirmar salida
	provisioningMode             // pantalla de "configurando mock" tras cerrar el wizard, antes de volver a listMode
	togglingMode                 // popup superpuesto a listMode mientras "t" habilita/deshabilita un mock
)

type mockItem struct {
	title          string
	description    string
	status         string
	delay          string
	jsonFile       string
	enabled        bool
	trafficBuckets []int // requests por bucket de trafficBucketDuration segundos, uno por mock; el último es el bucket "en curso"
}

func (m mockItem) Title() string {
	if !m.enabled {
		// Coloreamos todo el string ANTES de que list.DefaultDelegate lo
		// envuelva en su propio estilo (Normal/SelectedTitle): como acá sólo
		// tocamos Foreground (nunca Background), el reset final de este
		// Render() no pisa el padding/borde que agrega el delegate por
		// afuera, sólo el color del texto.
		return disabledTitleStyle.Render(m.title + " (disabled)")
	}
	return m.title
}
func (m mockItem) Description() string { return m.description }
func (m mockItem) FilterValue() string { return m.title }

type model struct {
	list              list.Model
	spinner           spinner.Model
	progress          progress.Model
	provisionProgress progress.Model // barra ANIMADA (SetPercent + FrameMsg/harmonica) de provisioningMode; m.progress de arriba es la estática del wizard, no la reusamos para no pisar su estado
	toggleSpinner     spinner.Model  // spinner del popup de togglingMode, aparte de m.spinner (form) para no compartir su ciclo de vida
	width             int
	height            int // alto total de terminal (msg.Height crudo), lo necesita el overlay de togglingMode para centrar el popup en toda la pantalla, no sólo en el panel izquierdo
	listWidth         int
	listHeight        int
	currentMode       mode
	formStep          int
	formPath          string
	formMethod        string
	formStatus        string
	formDelay         string
	formJSONFile      string
	cursorVisible     bool     // parpadeo del cursor de texto en formMode, alternado por cursorTick (update.go)
	trafficElapsed    int      // segundos acumulados dentro del bucket en curso, compartido: todos los mocks rotan buckets al mismo tiempo
	pendingMock       mockItem // mock ya armado por el wizard, en espera de que termine provisioningMode para insertarse en la lista
	provisionPercent  float64  // acumulador SIN clampear (a diferencia de progress.Model.Percent()) para poder detectar el overshoot que cierra la animación
	toggleLabel       string   // "Enabling..."/"Disabling...", el texto que muestra el popup de togglingMode
}

// seedTrafficBuckets arranca el historial con datos simulados para que el
// sparkline de un mock no empiece en cero; trafficTick (update.go) lo va
// rotando en vivo.
func seedTrafficBuckets() []int {
	buckets := make([]int, trafficBucketCount)
	for i := range buckets {
		buckets[i] = rand.Intn(20)
	}
	return buckets
}

func initialModel() model {
	items := []list.Item{
		mockItem{
			title:          "GET /api/v1/users",
			description:    "Returns users list",
			status:         "200",
			delay:          "30",
			jsonFile:       "examples/users.json",
			enabled:        true,
			trafficBuckets: seedTrafficBuckets(),
		},
		mockItem{
			title:          "POST /api/v1/orders",
			description:    "Creates an order",
			status:         "201",
			delay:          "800",
			jsonFile:       "examples/orders.json",
			enabled:        true,
			trafficBuckets: seedTrafficBuckets(),
		},
	}

	l := list.New(items, list.NewDefaultDelegate(), 30, 10) // valores temporales visibles
	l.Title = "Mocks loaded"
	l.KeyMap.Quit.SetEnabled(false) // reemplazado por quitKey: q/esc piden confirmación
	l.SetShowHelp(false)            // el help propio del list no trunca bien en anchos angostos (bug de la lib); usamos el nuestro en view.go

	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))

	// spinner distinto (Points en vez de Dot) para que el popup de
	// habilitar/deshabilitar se sienta un elemento aparte del wizard. Sin
	// WithStyle a propósito: va sin color propio para que togglePopupStyle
	// (view.go) sea la única fuente de color del popup, de punta a punta.
	tsp := spinner.New(spinner.WithSpinner(spinner.Points))

	// progress-static: sin Update()/Tick, se renderiza con ViewAs(percent) a
	// partir de m.formStep en cada View(), sin animación propia.
	pg := progress.New(progress.WithDefaultGradient(), progress.WithWidth(40))

	return model{
		list:              l,
		spinner:           sp,
		progress:          pg,
		provisionProgress: newProvisionProgress(),
		toggleSpinner:     tsp,
		currentMode:       listMode,
		formStep:          0,
	}
}

// newProvisionProgress arranca (o reinicia) la barra animada de
// provisioningMode. Se recrea cada vez que se entra a ese modo en vez de
// reusar la instancia anterior, para que el spring/percentShown internos
// vuelvan a 0 y no arranque "a mitad de camino" si el usuario agrega más de
// un mock en la misma sesión.
func newProvisionProgress() progress.Model {
	return progress.New(progress.WithDefaultGradient(), progress.WithWidth(44))
}
