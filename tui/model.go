package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"github.com/sebaguadagna/mocknroll/examples"
	"github.com/sebaguadagna/mocknroll/server"
)

// trafficBucketCount * trafficBucketDuration = total window shown (5 min).
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
	editMockKey = key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "edit"),
	)
	deleteMockKey = key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	)
	requestLogKey = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "requests"),
	)
)

type mode int

const (
	listMode          mode = iota //0
	formMode                      //1
	confirmExitMode               // mode for confirming exit
	confirmDeleteMode             // mode for confirming the deletion of a mock
	provisioningMode              // "configuring mock" screen after closing the wizard, before returning to listMode
	togglingMode                  // popup overlaid on listMode while "t" enables/disables a mock
)

type mockItem struct {
	title          string
	description    string
	status         string
	delay          string
	failureRate    string
	failureStatus  string
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
	formFailureRate   string
	formFailureStatus string
	formJSONFile      string
	formError         string                   // why the current field was rejected; shown in formMode until the user edits it
	cursorVisible     bool                     // text cursor blink in formMode, toggled by cursorTick (update.go)
	trafficElapsed    int                      // seconds accumulated within the current bucket, shared: all mocks roll buckets at the same time
	pendingMock       mockItem                 // mock already built by the wizard, waiting for provisioningMode to finish before it's inserted into the list
	provisionPercent  float64                  // UNclamped accumulator (unlike progress.Model.Percent()) so we can detect the overshoot that closes the animation
	toggleLabel       string                   // "Enabling..."/"Disabling...", the text togglingMode's popup shows
	showRequestLog    bool                     // the right panel shows the request log instead of the selected mock's details
	requestLog        []server.RequestLogEntry // newest first, refreshed by trafficTick (update.go)
	editing           bool                     // the form (and the provisioning that follows it) is changing the mock at editIndex instead of adding one
	editIndex         int                      // index in the unfiltered list of the mock being edited
	deleteIndex       int                      // index in the unfiltered list of the mock confirmDeleteMode's popup asks about
	deleteTitle       string                   // title of that same mock, shown in the popup
	saveError         string                   // why the last write to mocks.json failed; empty once a write succeeds
}

func initialModel(port int) (model, error) {
	items, err := loadMocks()
	if err != nil {
		// Never fall through to the defaults here: saving them would
		// overwrite the user's unreadable-but-maybe-recoverable mocks.json.
		return model{}, fmt.Errorf("can't load mocks.json, fix or remove it: %w", err)
	}
	if items == nil { // no mocks.json yet; an existing empty one means the user deleted them all
		items = []list.Item{
			mockItem{
				title:          "GET /api/v1/users",
				description:    "Returns users list",
				status:         "200",
				delay:          "30",
				jsonFile:       "examples/users.json",
				enabled:        true,
				trafficBuckets: make([]int, trafficBucketCount),
			},
			mockItem{
				title:          "POST /api/v1/orders",
				description:    "Creates an order",
				status:         "201",
				delay:          "800",
				jsonFile:       "examples/orders.json",
				enabled:        true,
				trafficBuckets: make([]int, trafficBucketCount),
			},
		}
		writeExampleFiles()
		_ = saveMocks(items)
	}

	l := list.New(items, list.NewDefaultDelegate(), 30, 10) // temporary, visible-enough values
	l.Title = fmt.Sprintf("Mock 'n Roll • Server on http://localhost:%d", port)
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

	m := model{
		list:              l,
		spinner:           sp,
		progress:          pg,
		provisionProgress: newProvisionProgress(),
		toggleSpinner:     tsp,
		currentMode:       listMode,
		formStep:          0,
	}
	m.updateServer()
	return m, nil
}

// newProvisionProgress starts (or resets) provisioningMode's animated
// bar. It's recreated every time that mode is entered instead of reusing
// the previous instance, so the internal spring/percentShown reset to 0
// and it doesn't start "mid-way" if the user adds more than one mock in
// the same session.
func newProvisionProgress() progress.Model {
	return progress.New(progress.WithDefaultBlend(), progress.WithWidth(44))
}

type mockItemPersist struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	Delay         string `json:"delay"`
	FailureRate   string `json:"failure_rate,omitempty"`
	FailureStatus string `json:"failure_status,omitempty"`
	JSONFile      string `json:"json_file"`
	Enabled       bool   `json:"enabled"`
}

func (mi mockItem) toPersist() mockItemPersist {
	return mockItemPersist{
		Title:         mi.title,
		Description:   mi.description,
		Status:        mi.status,
		Delay:         mi.delay,
		FailureRate:   mi.failureRate,
		FailureStatus: mi.failureStatus,
		JSONFile:      mi.jsonFile,
		Enabled:       mi.enabled,
	}
}

func (p mockItemPersist) toItem() mockItem {
	return mockItem{
		title:          p.Title,
		description:    p.Description,
		status:         p.Status,
		delay:          p.Delay,
		failureRate:    p.FailureRate,
		failureStatus:  p.FailureStatus,
		jsonFile:       p.JSONFile,
		enabled:        p.Enabled,
		trafficBuckets: make([]int, trafficBucketCount),
	}
}

func saveMocks(items []list.Item) error {
	listToSave := []mockItemPersist{} // not nil: an empty list must be saved as [], not null
	for _, it := range items {
		mi, ok := it.(mockItem)
		if ok {
			listToSave = append(listToSave, mi.toPersist())
		}
	}
	data, err := json.MarshalIndent(listToSave, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("mocks.json", data, 0644)
}

// writeExampleFiles creates the example mocks' response files in ./examples,
// keeping any that already exist. Failures are ignored on purpose: the app
// still starts, and those mocks show the read error instead.
func writeExampleFiles() {
	entries, err := examples.Files.ReadDir(".")
	if err != nil || os.MkdirAll("examples", 0755) != nil {
		return
	}
	for _, entry := range entries {
		data, err := examples.Files.ReadFile(entry.Name())
		if err != nil {
			continue
		}
		f, err := os.OpenFile(filepath.Join("examples", entry.Name()), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		_, _ = f.Write(data)
		_ = f.Close()
	}
}

func loadMocks() ([]list.Item, error) {
	data, err := os.ReadFile("mocks.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var persisted []mockItemPersist
	if err := json.Unmarshal(data, &persisted); err != nil {
		return nil, err
	}

	items := []list.Item{}
	for _, p := range persisted {
		items = append(items, p.toItem())
	}
	return items, nil
}

// splitTitle returns the method and path of a "METHOD /path" title. A title
// with no method, only possible in a hand-edited mocks.json, counts as GET.
func splitTitle(title string) (method, path string) {
	split := strings.SplitN(title, " ", 2)
	if len(split) == 2 {
		return split[0], split[1]
	}
	return "GET", title
}

func autoDescription(status, delay string) string {
	return "Status: " + status + ", Delay: " + delay + "ms"
}

// parseDelayRange returns the limits of a "200" or "100-500" delay. An invalid
// value, only possible in a hand-edited mocks.json, counts as no delay.
func parseDelayRange(delay string) (minMs, maxMs int) {
	minText, maxText, isRange := strings.Cut(delay, "-")
	minMs, err := strconv.Atoi(minText)
	if err != nil || minMs < 0 {
		return 0, 0
	}
	if !isRange {
		return minMs, minMs
	}
	maxMs, err = strconv.Atoi(maxText)
	if err != nil || maxMs < minMs {
		return 0, 0
	}
	return minMs, maxMs
}

// toServerMock leaves invalid numbers from a hand-edited mocks.json at 0.
func (mi mockItem) toServerMock() server.Mock {
	method, path := splitTitle(mi.title)
	status, _ := strconv.Atoi(mi.status)
	minDelay, maxDelay := parseDelayRange(mi.delay)
	failureRate, _ := strconv.Atoi(mi.failureRate)
	failureStatus, _ := strconv.Atoi(mi.failureStatus)

	return server.Mock{
		Method:        method,
		Path:          path,
		Status:        status,
		DelayMs:       minDelay,
		DelayMaxMs:    maxDelay,
		JSONFile:      mi.jsonFile,
		Enabled:       mi.enabled,
		FailureRate:   failureRate,
		FailureStatus: failureStatus,
	}
}

func (m *model) updateServer() {
	// Persist mock configurations to mocks.json
	m.saveError = ""
	if err := saveMocks(m.list.Items()); err != nil {
		m.saveError = err.Error()
	}

	var mocks []server.Mock
	for _, it := range m.list.Items() {
		mocks = append(mocks, it.(mockItem).toServerMock())
	}
	server.SetMocks(mocks)
}
