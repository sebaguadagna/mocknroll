package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sebaguadagna/mocknroll/server"
)

func TestNormalizeFailureType(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{"", "http", false},
		{"http", "http", false},
		{"HTTP", "http", false},
		{" Reset ", "reset", false},
		{"hang", "hang", false},
		{"timeout", "", true},
	}
	for _, tt := range tests {
		got, err := normalizeFailureType(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("%q: err = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("%q: got %q, want %q", tt.in, got, tt.want)
		}
	}
}

// startFailingForm leaves a new mock's form on the failure type step.
func startFailingForm(t *testing.T, m model) model {
	t.Helper()
	m = press(t, m, "a")
	m = typeText(t, m, "/net")
	m = pressEnter(t, m)
	m = typeText(t, m, "GET")
	m = pressEnter(t, m)
	m = goToStep(t, m, formStepFailureRate)
	m = typeText(t, m, "50")
	m = pressEnter(t, m)
	if m.formStep != formStepFailureType {
		t.Fatalf("expected failure type step, got %d: %s", m.formStep, m.formError)
	}
	return m
}

func TestNetworkTypeSkipsFailureStatus(t *testing.T) {
	m := startFailingForm(t, newEditTestModel(t))
	m = typeText(t, m, "RESET")
	m = pressEnter(t, m)
	if m.formStep != formStepJSONFile {
		t.Fatalf("expected JSON step, got %d", m.formStep)
	}
	m = submitForm(t, m)

	got := mockAt(m, 2)
	if got.failureType != "reset" || got.failureStatus != "" {
		t.Errorf("unexpected mock: %+v", got)
	}
	if s := got.toServerMock(); s.FailureType != server.FailureReset {
		t.Errorf("unexpected server mock: %+v", s)
	}
}

func TestHTTPTypeAsksForFailureStatus(t *testing.T) {
	for _, typed := range []string{"", "HTTP"} {
		m := startFailingForm(t, newEditTestModel(t))
		m = typeText(t, m, typed)
		m = pressEnter(t, m)
		if m.formStep != formStepFailureStatus || m.formFailureType != "http" {
			t.Errorf("%q: step=%d type=%q", typed, m.formStep, m.formFailureType)
		}
	}
}

func TestRateZeroStoresNoFailureType(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "a")
	m = typeText(t, m, "/net")
	m = pressEnter(t, m)
	m = typeText(t, m, "GET")
	m = pressEnter(t, m)
	m = goToStep(t, m, formStepFailureRate)
	m = pressEnter(t, m)
	if m.formStep != formStepJSONFile {
		t.Fatalf("expected JSON step, got %d", m.formStep)
	}
	m = submitForm(t, m)
	if got := mockAt(m, 2); got.failureType != "" || got.failureStatus != "" {
		t.Errorf("unexpected mock: %+v", got)
	}
}

func TestInvalidFailureTypeStaysOnStep(t *testing.T) {
	m := startFailingForm(t, newEditTestModel(t))
	m = typeText(t, m, "timeout")
	m = pressEnter(t, m)
	if m.formStep != formStepFailureType || m.formError == "" {
		t.Errorf("step=%d error=%q", m.formStep, m.formError)
	}
}

func TestEditKeepsFailureTypeAndPersistsIt(t *testing.T) {
	m := startFailingForm(t, newEditTestModel(t))
	m = typeText(t, m, "hang")
	m = submitForm(t, m)

	m.list.Select(2)
	m = press(t, m, "e")
	if m.formFailureType != "hang" {
		t.Errorf("type not prefilled: %q", m.formFailureType)
	}
	m = submitForm(t, m)
	if got := mockAt(m, 2); got.failureType != "hang" {
		t.Errorf("type lost: %+v", got)
	}

	restarted, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}
	if got := mockAt(restarted, 2); got.failureType != "hang" {
		t.Errorf("type not persisted: %+v", got)
	}
}

func TestEditFromNetworkToHTTPAsksForStatus(t *testing.T) {
	m := startFailingForm(t, newEditTestModel(t))
	m = typeText(t, m, "hang")
	m = submitForm(t, m)

	m.list.Select(2)
	m = press(t, m, "e")
	m = goToStep(t, m, formStepFailureType)
	m = replaceField(t, m, "http")
	m = pressEnter(t, m)
	if m.formStep != formStepFailureStatus {
		t.Errorf("expected failure status step, got %d", m.formStep)
	}
}

func TestFailureLineShowsTypeOrStatus(t *testing.T) {
	tests := []struct {
		item mockItem
		want string
	}{
		{mockItem{failureRate: "10", failureType: "reset"}, "Failures:   10% with reset"},
		{mockItem{failureRate: "10", failureType: "hang"}, "Failures:   10% with hang"},
		{mockItem{failureRate: "10", failureType: "http", failureStatus: "503"}, "Failures:   10% with 503"},
		{mockItem{failureRate: "10", failureStatus: "503"}, "Failures:   10% with 503"},
		{mockItem{failureRate: "10", failureType: "nope", failureStatus: "503"}, "Failures:   10% with 503"},
	}
	for _, tt := range tests {
		if got := failureLine(tt.item); !strings.Contains(got, tt.want) {
			t.Errorf("%+v: got %q, want %q", tt.item, got, tt.want)
		}
	}
}

func TestRequestLogRowShowsNetworkFailures(t *testing.T) {
	for _, failureType := range []string{server.FailureReset, server.FailureHang} {
		entry := logEntry("/a", 0, true)
		entry.Simulated = true
		entry.FailureType = failureType
		entry.Duration = 300 * time.Millisecond

		row := requestLogRow(entry, 70)
		for _, want := range []string{"---", failureType, "300ms"} {
			if !strings.Contains(row, want) {
				t.Errorf("row %q doesn't contain %q", row, want)
			}
		}
		if strings.Contains(row, "fail") {
			t.Errorf("row %q shouldn't say fail", row)
		}
	}
}

func TestMocksWithoutFailureTypeKeepWorking(t *testing.T) {
	t.Chdir(t.TempDir())
	const old = `[{"title": "GET /old", "status": "200", "enabled": true, "failure_rate": "100", "failure_status": "503"}]`
	if err := os.WriteFile("mocks.json", []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}
	got := mockAt(m, 0)
	if s := got.toServerMock(); s.FailureType != "" || s.FailureStatus != 503 {
		t.Errorf("unexpected server mock: %+v", s)
	}
	if line := failureLine(got); !strings.Contains(line, "503") {
		t.Errorf("unexpected line: %q", line)
	}
}
