package tui

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

const saveErrorText = "can't save"

// breakSaves puts a directory in mocks.json's place, which makes writes fail
// even as root.
func breakSaves(t *testing.T) {
	t.Helper()
	if err := os.Remove("mocks.json"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("mocks.json", 0755); err != nil {
		t.Fatal(err)
	}
}

func TestSaveErrorIsShownAndClearsOnSuccess(t *testing.T) {
	t.Chdir(t.TempDir())
	m, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}
	m = send(t, m, tea.WindowSizeMsg{Width: 150, Height: 40})
	if strings.Contains(m.renderListView(), saveErrorText) {
		t.Fatal("error shown before any failure")
	}

	breakSaves(t)
	m.updateServer()
	if view := m.renderListView(); !strings.Contains(view, saveErrorText) {
		t.Errorf("the failed save isn't shown:\n%s", view)
	}

	if err := os.Remove("mocks.json"); err != nil {
		t.Fatal(err)
	}
	m.updateServer()
	if m.saveError != "" {
		t.Errorf("the error stayed after a successful save: %q", m.saveError)
	}
	if strings.Contains(m.renderListView(), saveErrorText) {
		t.Error("the error is still shown after a successful save")
	}
}

func TestToggleReportsSaveError(t *testing.T) {
	t.Chdir(t.TempDir())
	m, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}
	m = send(t, m, tea.WindowSizeMsg{Width: 150, Height: 40})
	breakSaves(t)

	m = press(t, m, "t")
	if m.saveError == "" {
		t.Error("the error from a key press was lost")
	}
}

func TestStartupReportsSaveError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	t.Chdir(dir)

	m, err := initialModel(8080)
	if err != nil {
		t.Fatal(err)
	}
	if m.saveError == "" {
		t.Error("the failed first save isn't reported")
	}
}
