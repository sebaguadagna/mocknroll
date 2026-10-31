package tui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// newEditTestModel also creates the JSON files the default mocks point to,
// since the form rejects missing ones.
func newEditTestModel(t *testing.T) model {
	t.Helper()
	m := newTestModel(t)
	if err := os.MkdirAll("examples", 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"examples/users.json", "examples/orders.json"} {
		if err := os.WriteFile(f, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func send(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	updated, _ := m.Update(msg)
	return updated.(model)
}

func replaceField(t *testing.T, m model, text string) model {
	t.Helper()
	for range len(currentFieldValue(m)) {
		m = send(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	return typeText(t, m, text)
}

func currentFieldValue(m model) string {
	switch m.formStep {
	case formStepPath:
		return m.formPath
	case formStepMethod:
		return m.formMethod
	case formStepStatus:
		return m.formStatus
	case formStepDelay:
		return m.formDelay
	case formStepFailureRate:
		return m.formFailureRate
	case formStepFailureStatus:
		return m.formFailureStatus
	default:
		return m.formJSONFile
	}
}

func goToStep(t *testing.T, m model, step int) model {
	t.Helper()
	for m.formStep < step {
		before := m.formStep
		m = pressEnter(t, m)
		if m.formStep == before {
			t.Fatalf("form stuck on step %d: %s", before, m.formError)
		}
	}
	return m
}

// submitForm presses Enter through the remaining steps and runs the animation.
func submitForm(t *testing.T, m model) model {
	t.Helper()
	for m.currentMode == formMode {
		before := m.formStep
		m = pressEnter(t, m)
		if m.currentMode == formMode && m.formStep == before {
			t.Fatalf("form stuck on step %d: %s", before, m.formError)
		}
	}
	for range 5 {
		m = send(t, m, provisionTickMsg{})
	}
	if m.currentMode != listMode {
		t.Fatalf("expected listMode after provisioning, got %v", m.currentMode)
	}
	return m
}

func mockAt(m model, i int) mockItem {
	return m.list.Items()[i].(mockItem)
}

func savedMocks(t *testing.T) []mockItemPersist {
	t.Helper()
	data, err := os.ReadFile("mocks.json")
	if err != nil {
		t.Fatal(err)
	}
	var saved []mockItemPersist
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	return saved
}

func setBuckets(m *model, i int, value int) {
	item := mockAt(*m, i)
	item.trafficBuckets = make([]int, trafficBucketCount)
	item.trafficBuckets[0] = value
	m.list.SetItem(i, item)
}

func TestEditPrefillsEveryField(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")

	if m.currentMode != formMode || m.formStep != formStepPath {
		t.Fatalf("expected path step of the form, got mode %v step %d", m.currentMode, m.formStep)
	}
	if m.formPath != "/api/v1/users" || m.formMethod != "GET" || m.formStatus != "200" ||
		m.formDelay != "30" || m.formJSONFile != "examples/users.json" {
		t.Errorf("unexpected prefill: %q %q %q %q %q", m.formPath, m.formMethod, m.formStatus, m.formDelay, m.formJSONFile)
	}
}

func TestEditValidatesFields(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepStatus)
	m = replaceField(t, m, "99")
	m = pressEnter(t, m)

	if m.formStep != formStepStatus || m.formError == "" || m.formStatus != "99" {
		t.Errorf("expected rejection on status step, got step=%d error=%q status=%q", m.formStep, m.formError, m.formStatus)
	}
}

func TestEditKeepingRouteIsNotADuplicate(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepStatus)

	if m.formError != "" {
		t.Errorf("unexpected error: %s", m.formError)
	}
}

func TestEditToAnotherMocksRouteIsADuplicate(t *testing.T) {
	m := newEditTestModel(t)
	m.list.Select(1)
	m = press(t, m, "e")
	m = replaceField(t, m, "/api/v1/users")
	m = pressEnter(t, m)
	m = replaceField(t, m, "GET")
	m = pressEnter(t, m)

	if m.formStep != formStepMethod || !strings.Contains(m.formError, "already exists") {
		t.Errorf("expected duplicate error, got step=%d error=%q", m.formStep, m.formError)
	}
}

func TestEditEscDiscardsChanges(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = replaceField(t, m, "/changed")
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})

	if m.currentMode != listMode || mockAt(m, 0).title != "GET /api/v1/users" {
		t.Errorf("edit wasn't discarded: mode %v, title %q", m.currentMode, mockAt(m, 0).title)
	}

	m = press(t, m, "a")
	if m.editing || m.formPath != "" || m.formMethod != "" {
		t.Errorf("add form didn't start empty: editing=%v path=%q method=%q", m.editing, m.formPath, m.formMethod)
	}
}

func TestEditReplacesInPlaceAndKeepsEnabledState(t *testing.T) {
	m := newEditTestModel(t)
	m.list.Select(1)
	m = press(t, m, "t")
	m = send(t, m, toggleDismissMsg{})

	m = press(t, m, "e")
	m = goToStep(t, m, formStepStatus)
	m = replaceField(t, m, "404")
	m = submitForm(t, m)

	if len(m.list.Items()) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(m.list.Items()))
	}
	got := mockAt(m, 1)
	if got.title != "POST /api/v1/orders" || got.status != "404" || got.enabled {
		t.Errorf("unexpected mock: %+v", got)
	}
	saved := savedMocks(t)
	if saved[1].Status != "404" || saved[1].Enabled {
		t.Errorf("mocks.json doesn't match: %+v", saved[1])
	}
}

func TestEditAppliesOnlyWhenAnimationEnds(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepStatus)
	m = replaceField(t, m, "404")
	for m.currentMode == formMode {
		m = pressEnter(t, m)
	}

	if m.currentMode != provisioningMode || mockAt(m, 0).status != "200" || savedMocks(t)[0].Status != "200" {
		t.Errorf("edit applied too early: mode %v, status %q", m.currentMode, mockAt(m, 0).status)
	}
}

func TestEditKeepsTrafficWhenRouteIsUnchanged(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepStatus)
	m = replaceField(t, m, "404")
	for m.currentMode == formMode {
		m = pressEnter(t, m)
	}
	setBuckets(&m, 0, 7) // traffic that arrives during the animation counts too
	for range 5 {
		m = send(t, m, provisionTickMsg{})
	}

	if got := sumInts(mockAt(m, 0).trafficBuckets); got != 7 {
		t.Errorf("expected 7 requests kept, got %d", got)
	}
}

func TestEditResetsTrafficWhenRouteChanges(t *testing.T) {
	m := newEditTestModel(t)
	setBuckets(&m, 0, 7)
	m = press(t, m, "e")
	m = replaceField(t, m, "/new")
	m = submitForm(t, m)

	got := mockAt(m, 0)
	if got.title != "GET /new" || sumInts(got.trafficBuckets) != 0 || len(got.trafficBuckets) != trafficBucketCount {
		t.Errorf("unexpected mock: %+v", got)
	}
}

func TestEditRegeneratesOnlyAutoDescriptions(t *testing.T) {
	m := newEditTestModel(t)

	// The default mock's description is hand-written.
	m = press(t, m, "e")
	m = goToStep(t, m, formStepDelay)
	m = replaceField(t, m, "50")
	m = submitForm(t, m)
	if got := mockAt(m, 0).description; got != "Returns users list" {
		t.Errorf("hand-written description changed to %q", got)
	}

	item := mockAt(m, 0)
	item.description = autoDescription(item.status, item.delay)
	m.list.SetItem(0, item)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepDelay)
	m = replaceField(t, m, "75")
	m = submitForm(t, m)
	if got := mockAt(m, 0).description; got != "Status: 200, Delay: 75ms" {
		t.Errorf("auto description not regenerated: %q", got)
	}
}

func TestEditWordingOnlyWhenEditing(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "a")
	if view := m.renderView(); !strings.Contains(view, "Creating new mock") {
		t.Errorf("add form lost its wording: %s", view)
	}
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})

	m = press(t, m, "e")
	if view := m.renderView(); !strings.Contains(view, "Editing mock") {
		t.Errorf("edit form wording missing: %s", view)
	}
	for m.currentMode == formMode {
		m = pressEnter(t, m)
	}
	if view := m.renderView(); !strings.Contains(view, "Updating mock") {
		t.Errorf("edit animation wording missing: %s", view)
	}
}

func TestEditRejectsRemovedJSONFile(t *testing.T) {
	m := newEditTestModel(t)
	if err := os.Remove("examples/users.json"); err != nil {
		t.Fatal(err)
	}
	m = press(t, m, "e")
	m = goToStep(t, m, formStepJSONFile)
	m = pressEnter(t, m)

	if m.currentMode != formMode || m.formError == "" {
		t.Errorf("expected rejection, got mode %v error %q", m.currentMode, m.formError)
	}
}

func TestDeleteCancelKeepsListAndPopupNamesMock(t *testing.T) {
	m := newEditTestModel(t)
	m.width, m.height, m.listWidth, m.listHeight = 140, 40, 84, 37
	m = press(t, m, "d")

	if m.currentMode != confirmDeleteMode {
		t.Fatalf("expected confirmDeleteMode, got %v", m.currentMode)
	}
	if view := m.renderView(); !strings.Contains(view, "GET /api/v1/users?") {
		t.Errorf("popup doesn't name the mock: %s", view)
	}
	for _, key := range []string{"n", "N"} {
		m = press(t, m, key)
		if m.currentMode != listMode || len(m.list.Items()) != 2 {
			t.Errorf("%q: mode %v, %d mocks", key, m.currentMode, len(m.list.Items()))
		}
		m = press(t, m, "d")
	}
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.currentMode != listMode || len(m.list.Items()) != 2 {
		t.Errorf("esc: mode %v, %d mocks", m.currentMode, len(m.list.Items()))
	}
}

func TestDeleteRemovesAndPersists(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "d")
	m = press(t, m, "y")

	if m.currentMode != listMode || len(m.list.Items()) != 1 || mockAt(m, 0).title != "POST /api/v1/orders" {
		t.Errorf("unexpected list: %+v", m.list.Items())
	}
	if saved := savedMocks(t); len(saved) != 1 || saved[0].Title != "POST /api/v1/orders" {
		t.Errorf("mocks.json doesn't match: %+v", saved)
	}
}

func TestDeleteLastRowKeepsValidSelection(t *testing.T) {
	m := newEditTestModel(t)
	m.list.Select(1)
	m = press(t, m, "d")
	m = press(t, m, "y")

	if selected, ok := m.list.SelectedItem().(mockItem); !ok || selected.title != "GET /api/v1/users" {
		t.Errorf("unexpected selection: %+v", m.list.SelectedItem())
	}
}

func TestDeleteAllSurvivesRestart(t *testing.T) {
	m := newEditTestModel(t)
	for range 2 {
		m = press(t, m, "d")
		m = press(t, m, "y")
	}
	if m.list.SelectedItem() != nil {
		t.Errorf("expected no selection, got %+v", m.list.SelectedItem())
	}

	data, err := os.ReadFile("mocks.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "[]" {
		t.Errorf("expected [], got %q", data)
	}
	restarted, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}
	if len(restarted.list.Items()) != 0 {
		t.Errorf("examples came back: %+v", restarted.list.Items())
	}
}

func TestDefaultsOnlyWithoutMocksFile(t *testing.T) {
	tests := []struct {
		name    string
		content string // empty: no file
		want    int
	}{
		{"missing file", "", 2},
		{"empty list", "[]", 0},
		{"null", "null", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if tt.content != "" {
				if err := os.WriteFile("mocks.json", []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			m, err := initialModel(8080)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(m.list.Items()); got != tt.want {
				t.Errorf("got %d mocks, want %d", got, tt.want)
			}
		})
	}
}

// threeMocksFiltered leaves a filter applied that matches the first and third
// mocks, so visible and unfiltered indexes differ.
func threeMocksFiltered(t *testing.T) model {
	t.Helper()
	m := newEditTestModel(t)
	m = press(t, m, "a")
	m = typeText(t, m, "/api/v1/users/2")
	m = pressEnter(t, m)
	m = typeText(t, m, "GET")
	m = submitForm(t, m)
	m.list.SetFilterText("users")
	return m
}

func TestDeleteWithFilterRemovesSelectedMock(t *testing.T) {
	m := threeMocksFiltered(t)
	m.list.Select(1)
	selected := m.list.SelectedItem().(mockItem).title

	m = press(t, m, "d")
	m = press(t, m, "y")

	for i := range m.list.Items() {
		if mockAt(m, i).title == selected {
			t.Errorf("%s wasn't deleted", selected)
		}
	}
	if len(m.list.Items()) != 2 || m.list.FilterState() != list.FilterApplied {
		t.Errorf("got %d mocks, filter state %v", len(m.list.Items()), m.list.FilterState())
	}
	for _, it := range m.list.VisibleItems() {
		if it.(mockItem).title == selected {
			t.Errorf("filtered view still shows %s", selected)
		}
	}
}

func TestDeleteOnlyFilterMatchClearsFilter(t *testing.T) {
	m := newEditTestModel(t)
	m.list.SetFilterText("orders")

	m = press(t, m, "d")
	m = press(t, m, "y")

	if m.list.FilterState() != list.Unfiltered || len(m.list.VisibleItems()) != 1 ||
		mockAt(m, 0).title != "GET /api/v1/users" {
		t.Errorf("state %v, visible %+v", m.list.FilterState(), m.list.VisibleItems())
	}
}

func TestEditWithFilterChangesSelectedMock(t *testing.T) {
	m := newEditTestModel(t)
	m.list.SetFilterText("orders")

	m = press(t, m, "e")
	m = goToStep(t, m, formStepStatus)
	m = replaceField(t, m, "404")
	m = submitForm(t, m)

	if mockAt(m, 0).status != "200" || mockAt(m, 1).status != "404" {
		t.Errorf("wrong mock changed: %+v, %+v", mockAt(m, 0), mockAt(m, 1))
	}
}

func TestTypingEAndDInFilter(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "/")
	m = press(t, m, "e")
	m = press(t, m, "d")

	if m.currentMode != listMode || m.list.FilterInput.Value() != "ed" {
		t.Errorf("mode %v, filter %q", m.currentMode, m.list.FilterInput.Value())
	}
}

func TestEditAndDeleteDoNothingOnEmptyList(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("mocks.json", []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"e", "d"} {
		if m = press(t, m, key); m.currentMode != listMode {
			t.Errorf("%q: mode %v", key, m.currentMode)
		}
	}
}

func TestHintLineListsEditAndDelete(t *testing.T) {
	m := newEditTestModel(t)
	m.width, m.height, m.listWidth, m.listHeight = 200, 40, 120, 37
	view := m.renderListView()
	if !strings.Contains(view, "edit: e") || !strings.Contains(view, "delete: d") {
		t.Errorf("hint line missing keys: %s", view)
	}
}

func TestDKeyDoesNotChangePage(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "d")
	if m.currentMode != confirmDeleteMode {
		t.Errorf("expected confirmDeleteMode, got %v", m.currentMode)
	}
}

func TestTrafficTickAfterDelete(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "d")
	m = press(t, m, "y")
	send(t, m, trafficTickMsg{})
}

func TestFilterResultsArriveOutsideListMode(t *testing.T) {
	m := newEditTestModel(t)
	m.list.SetFilterText("orders")
	updated, cmd := m.Update(trafficTickMsg{})
	m = updated.(model)
	m.currentMode = confirmDeleteMode

	for _, msg := range collectMsgs(cmd) {
		if _, ok := msg.(list.FilterMatchesMsg); ok {
			m = send(t, m, msg)
		}
	}
	if len(m.list.VisibleItems()) != 1 {
		t.Errorf("filtered view lost its matches: %+v", m.list.VisibleItems())
	}
}

// collectMsgs runs a command's immediate results, skipping ticks that would
// block the test: only batches and filter commands return right away here.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	msg := <-done
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, c := range batch {
		if c == nil {
			continue
		}
		msgs = append(msgs, collectMsgs(c)...)
	}
	return msgs
}
