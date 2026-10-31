package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sebaguadagna/mocknroll/server"
)

func TestFailureNormalizers(t *testing.T) {
	tests := []struct {
		name      string
		normalize func(string) (string, error)
		in        string
		want      string
		wantErr   bool
	}{
		{"range", normalizeDelay, "100-500", "100-500", false},
		{"fixed", normalizeDelay, "200", "200", false},
		{"inverted range", normalizeDelay, "500-100", "", true},
		{"equal limits", normalizeDelay, "100-100", "100", false},
		{"spaces in range", normalizeDelay, " 100 - 500 ", "100-500", false},
		{"negative", normalizeDelay, "-5", "", true},
		{"open range", normalizeDelay, "100-", "", true},
		{"three limits", normalizeDelay, "1-2-3", "", true},
		{"rate empty", normalizeFailureRate, "", "0", false},
		{"rate valid", normalizeFailureRate, "20", "20", false},
		{"rate limit", normalizeFailureRate, "100", "100", false},
		{"rate high", normalizeFailureRate, "101", "", true},
		{"rate negative", normalizeFailureRate, "-1", "", true},
		{"rate text", normalizeFailureRate, "abc", "", true},
		{"status empty", normalizeFailureStatus, "", "500", false},
		{"status valid", normalizeFailureStatus, "503", "503", false},
		{"status low", normalizeFailureStatus, "200", "", true},
		{"status high", normalizeFailureStatus, "600", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.normalize(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormRejectsInvertedRange(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepDelay)
	m = replaceField(t, m, "500-100")
	m = pressEnter(t, m)

	if m.formStep != formStepDelay || m.formError == "" {
		t.Errorf("step=%d error=%q", m.formStep, m.formError)
	}
}

func TestFormSkipsFailureStatusAtRateZero(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepFailureRate)
	m.formFailureStatus = "503" // left over: must not survive the skip
	m = pressEnter(t, m)

	if m.formStep != formStepJSONFile || m.formFailureStatus != "" {
		t.Errorf("step=%d failureStatus=%q", m.formStep, m.formFailureStatus)
	}
}

func createFailingMock(t *testing.T, m model) model {
	t.Helper()
	m = press(t, m, "a")
	m = typeText(t, m, "/flaky")
	m = pressEnter(t, m)
	m = typeText(t, m, "GET")
	m = pressEnter(t, m)
	m = pressEnter(t, m) // status
	m = typeText(t, m, "100-500")
	m = pressEnter(t, m)
	m = typeText(t, m, "20")
	m = pressEnter(t, m)
	if m.formStep != formStepFailureStatus {
		t.Fatalf("expected failure status step, got %d: %s", m.formStep, m.formError)
	}
	m = typeText(t, m, "503")
	return submitForm(t, m)
}

func TestFormStoresAndPersistsFailureSettings(t *testing.T) {
	m := createFailingMock(t, newEditTestModel(t))

	got := mockAt(m, 2)
	if got.delay != "100-500" || got.failureRate != "20" || got.failureStatus != "503" {
		t.Errorf("unexpected mock: %+v", got)
	}
	saved := savedMocks(t)[2]
	if saved.Delay != "100-500" || saved.FailureRate != "20" || saved.FailureStatus != "503" {
		t.Errorf("unexpected saved mock: %+v", saved)
	}

	mock := got.toServerMock()
	if mock.DelayMs != 100 || mock.DelayMaxMs != 500 || mock.FailureRate != 20 || mock.FailureStatus != 503 {
		t.Errorf("unexpected server mock: %+v", mock)
	}
}

func TestEditPrefillsFailureSettings(t *testing.T) {
	m := createFailingMock(t, newEditTestModel(t))
	m.list.Select(2)
	m = press(t, m, "e")

	if m.formDelay != "100-500" || m.formFailureRate != "20" || m.formFailureStatus != "503" {
		t.Errorf("unexpected prefill: %q %q %q", m.formDelay, m.formFailureRate, m.formFailureStatus)
	}
}

func TestEditToRateZeroClearsFailureStatus(t *testing.T) {
	m := createFailingMock(t, newEditTestModel(t))
	m.list.Select(2)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepFailureRate)
	m = replaceField(t, m, "0")
	m = submitForm(t, m)

	got := mockAt(m, 2)
	if got.failureRate != "0" || got.failureStatus != "" || got.toServerMock().FailureRate != 0 {
		t.Errorf("unexpected mock: %+v", got)
	}
}

func TestDelayTextUsesRangeMaximum(t *testing.T) {
	want := "Responds in " + delayHighStyle.Render("100-500ms")
	if got := delayText("100-500"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDetailsShowFailuresOnlyWhenRateIsSet(t *testing.T) {
	m := createFailingMock(t, sizedTestModel(t))

	m.list.Select(2)
	if view := m.renderListView(); !strings.Contains(view, "20% with 503") {
		t.Errorf("failures missing: %s", view)
	}
	m.list.Select(0)
	if view := m.renderListView(); strings.Contains(view, "Failures:") {
		t.Errorf("unexpected failures line: %s", view)
	}
}

func TestOnlySimulatedRowsSayFail(t *testing.T) {
	entry := logEntry("/a", 500, true)
	if row := requestLogRow(entry, 48); strings.Contains(row, "fail") {
		t.Errorf("configured 500 marked: %q", row)
	}
	entry.Simulated = true
	row := requestLogRow(entry, 48)
	if !strings.Contains(row, "fail") || !strings.Contains(row, "802ms") {
		t.Errorf("simulated failure row: %q", row)
	}
}

func TestOldMocksFileHasNoFailures(t *testing.T) {
	t.Chdir(t.TempDir())
	const old = `[{"title":"GET /a","description":"x","status":"201","delay":"30","json_file":"","enabled":true}]`
	if err := os.WriteFile("mocks.json", []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}

	got := mockAt(m, 0).toServerMock()
	want := server.Mock{Method: "GET", Path: "/a", Status: 201, DelayMs: 30, DelayMaxMs: 30, Enabled: true}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	m.updateServer()
	data, err := os.ReadFile("mocks.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "failure") {
		t.Errorf("failure fields were added: %s", data)
	}
}

func TestInvalidValuesFromMocksFile(t *testing.T) {
	item := mockItem{title: "GET /a", delay: "fast", failureRate: "often", failureStatus: "bad", enabled: true}
	got := item.toServerMock()
	if got.DelayMs != 0 || got.DelayMaxMs != 0 || got.FailureRate != 0 || got.FailureStatus != 0 {
		t.Errorf("unexpected mock: %+v", got)
	}
}

func TestEscClearsFailureFields(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepFailureRate)
	m = typeText(t, m, "20")
	m = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})

	if m.formFailureRate != "" {
		t.Errorf("rate kept: %q", m.formFailureRate)
	}
}
