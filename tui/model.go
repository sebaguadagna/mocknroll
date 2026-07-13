package tui

import (
	"math/rand"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
)

// trafficBucketCount * trafficBucketDuration = total window shown (5 min).
// TODO: replace with real request counts once server.go actually serves
// the mocks; for now traffic is simulated to test the visualization.
const (
	trafficBucketCount    = 30
	trafficBucketDuration = 10 // seconds per bucket
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
	confirmExitMode              // mode for confirming exit
	provisioningMode             // "configuring mock" screen after closing the wizard, before returning to listMode
	togglingMode                 // popup overlaid on listMode while "t" enables/disables a mock
)

type mockItem struct {
	title          string
	description    string
	status         string
	delay          string
	jsonFile       string
	enabled        bool
	trafficBuckets []int // requests per trafficBucketDuration-second bucket, one per mock; the last one is the "in progress" bucket
}

func (m mockItem) Title() string {
	if !m.enabled {
		// We color the whole string BEFORE list.DefaultDelegate wraps it in
		// its own style (Normal/SelectedTitle): since we only touch
		// Foreground here (never Background), this Render()'s final reset
		// doesn't clobber the padding/border the delegate adds around it,
		// only the text color.
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
	provisionProgress progress.Model // ANIMATED bar (SetPercent + FrameMsg/harmonica) for provisioningMode; m.progress above is the wizard's static one — not reused, so we don't clobber its state
	toggleSpinner     spinner.Model  // spinner for togglingMode's popup, separate from m.spinner (form) so they don't share a lifecycle
	width             int
	height            int // total terminal height (raw msg.Height), needed by togglingMode's overlay to center the popup across the whole screen, not just the left panel
	listWidth         int
	listHeight        int
	currentMode       mode
	formStep          int
	formPath          string
	formMethod        string
	formStatus        string
	formDelay         string
	formJSONFile      string
	cursorVisible     bool     // text cursor blink in formMode, toggled by cursorTick (update.go)
	trafficElapsed    int      // seconds accumulated within the current bucket, shared: all mocks roll buckets at the same time
	pendingMock       mockItem // mock already built by the wizard, waiting for provisioningMode to finish before it's inserted into the list
	provisionPercent  float64  // UNclamped accumulator (unlike progress.Model.Percent()) so we can detect the overshoot that closes the animation
	toggleLabel       string   // "Enabling..."/"Disabling...", the text togglingMode's popup shows
}

// seedTrafficBuckets starts the history with simulated data so a mock's
// sparkline doesn't begin at zero; trafficTick (update.go) rolls it
// live from there.
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

	l := list.New(items, list.NewDefaultDelegate(), 30, 10) // temporary, visible-enough values
	l.Title = "Mocks loaded"
	l.KeyMap.Quit.SetEnabled(false) // replaced by quitKey: q/esc ask for confirmation
	l.SetShowHelp(false)            // the list's own help doesn't truncate well at narrow widths (lib bug); we use our own in view.go

	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerStyle))

	// a different spinner (Points instead of Dot) so the enable/disable
	// popup feels like a separate element from the wizard. No WithStyle on
	// purpose: it has no color of its own, so togglePopupStyle (view.go) is
	// the single source of color for the popup, start to finish.
	tsp := spinner.New(spinner.WithSpinner(spinner.Points))

	// static progress: no Update()/Tick, rendered with ViewAs(percent)
	// derived from m.formStep on every View(), with no animation of its own.
	pg := progress.New(progress.WithDefaultBlend(), progress.WithWidth(40))

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

// newProvisionProgress starts (or resets) provisioningMode's animated
// bar. It's recreated every time that mode is entered instead of reusing
// the previous instance, so the internal spring/percentShown reset to 0
// and it doesn't start "mid-way" if the user adds more than one mock in
// the same session.
func newProvisionProgress() progress.Model {
	return progress.New(progress.WithDefaultBlend(), progress.WithWidth(44))
}
