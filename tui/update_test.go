package tui

import (
	"os"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func newTestModel(t *testing.T) model {
	t.Helper()
	t.Chdir(t.TempDir()) // the model persists to ./mocks.json
	m, err := initialModel()
	if err != nil {
		t.Fatalf("initialModel: %v", err)
	}
	return m
}

func press(t *testing.T, m model, text string) model {
	t.Helper()
	msg := tea.KeyPressMsg{Code: []rune(text)[0], Text: text}
	updated, _ := m.Update(msg)
	return updated.(model)
}

func pressEnter(t *testing.T, m model) model {
	t.Helper()
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return updated.(model)
}

func typeText(t *testing.T, m model, text string) model {
	t.Helper()
	for _, r := range text {
		m = press(t, m, string(r))
	}
	return m
}

func TestToggleWithFilterAppliedKeepsOtherMocks(t *testing.T) {
	m := newTestModel(t)

	// Only the second mock matches, so it's at index 0 of the filtered view.
	m.list.SetFilterText("orders")
	selected, ok := m.list.SelectedItem().(mockItem)
	if !ok || selected.title != "POST /api/v1/orders" {
		t.Fatalf("unexpected selection: %+v", m.list.SelectedItem())
	}

	m = press(t, m, "t")

	items := m.list.Items()
	first, second := items[0].(mockItem), items[1].(mockItem)
	if first.title != "GET /api/v1/users" || !first.enabled {
		t.Errorf("first mock was modified: %+v", first)
	}
	if second.title != "POST /api/v1/orders" || second.enabled {
		t.Errorf("second mock should be disabled: %+v", second)
	}
}

func TestTypingAInFilterDoesNotOpenForm(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "/")
	if m.list.FilterState() != list.Filtering {
		t.Fatalf("expected filtering state, got %v", m.list.FilterState())
	}

	m = press(t, m, "a")

	if m.currentMode != listMode {
		t.Errorf("expected listMode, got %v", m.currentMode)
	}
}

func TestFormRejectsInvalidFieldAndKeepsStep(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "a")
	m = typeText(t, m, "/x")
	m = pressEnter(t, m)
	m = typeText(t, m, "nope")
	m = pressEnter(t, m)

	if m.formStep != formStepMethod {
		t.Errorf("expected to stay on method step, got %d", m.formStep)
	}
	if m.formError == "" {
		t.Error("expected an error message")
	}
	if m.formMethod != "nope" {
		t.Errorf("expected the rejected text to stay in the field, got %q", m.formMethod)
	}
}

func TestFormRejectsDuplicateRoute(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "a")
	m = typeText(t, m, "/api/v1/users")
	m = pressEnter(t, m)
	m = typeText(t, m, "get")
	m = pressEnter(t, m)

	if m.formStep != formStepMethod || m.formError == "" {
		t.Errorf("expected duplicate to be rejected, step=%d error=%q", m.formStep, m.formError)
	}
}

func TestFormSubmitNormalizesAndInsertsAfterProvisioning(t *testing.T) {
	m := newTestModel(t)
	m = press(t, m, "a")
	m = typeText(t, m, "ping")
	m = pressEnter(t, m)
	m = typeText(t, m, "get")
	m = pressEnter(t, m)
	m = pressEnter(t, m) // status: default
	m = pressEnter(t, m) // delay: default
	m = pressEnter(t, m) // json file: none

	if m.currentMode != provisioningMode {
		t.Fatalf("expected provisioningMode, got %v", m.currentMode)
	}
	if m.hasMock("GET /ping") {
		t.Error("mock shouldn't be inserted until provisioning finishes")
	}

	for range 5 {
		updated, _ := m.Update(provisionTickMsg{})
		m = updated.(model)
	}

	if m.currentMode != listMode {
		t.Fatalf("expected listMode, got %v", m.currentMode)
	}
	items := m.list.Items()
	added := items[len(items)-1].(mockItem)
	if added.title != "GET /ping" || added.status != "200" || added.delay != "0" {
		t.Errorf("unexpected mock: %+v", added)
	}
}

func TestDefaultMocksStartWithoutTraffic(t *testing.T) {
	m := newTestModel(t)
	for _, it := range m.list.Items() {
		if total := sumInts(it.(mockItem).trafficBuckets); total != 0 {
			t.Errorf("%s starts with %d fake requests", it.(mockItem).title, total)
		}
	}
}

func TestCorruptMocksFileIsReportedAndPreserved(t *testing.T) {
	t.Chdir(t.TempDir())
	const corrupt = `[{"title": `
	if err := os.WriteFile("mocks.json", []byte(corrupt), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := initialModel(); err == nil {
		t.Error("expected an error for a corrupt mocks.json")
	}

	data, err := os.ReadFile("mocks.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != corrupt {
		t.Errorf("mocks.json was overwritten: %q", data)
	}
}
