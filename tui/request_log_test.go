package tui

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/sebaguadagna/mocknroll/server"
)

func logEntry(path string, status int, matched bool) server.RequestLogEntry {
	return server.RequestLogEntry{
		Time:     time.Date(2026, 10, 31, 14, 32, 7, 0, time.Local),
		Method:   "GET",
		Path:     path,
		Status:   status,
		Duration: 802 * time.Millisecond,
		Matched:  matched,
	}
}

func sizedTestModel(t *testing.T) model {
	t.Helper()
	m := newEditTestModel(t)
	m.width, m.height, m.listWidth, m.listHeight = 200, 40, 120, 37
	return m
}

func TestRKeyTogglesPanel(t *testing.T) {
	m := sizedTestModel(t)
	if view := m.renderListView(); !strings.Contains(view, "Details") || strings.Contains(view, "No requests yet") {
		t.Fatalf("expected details first: %s", view)
	}

	m = press(t, m, "r")
	if view := m.renderListView(); strings.Contains(view, "Details") || !strings.Contains(view, "No requests yet") {
		t.Errorf("expected the log: %s", view)
	}

	m = press(t, m, "r")
	if view := m.renderListView(); !strings.Contains(view, "Details") {
		t.Errorf("expected details again: %s", view)
	}
}

func TestLogShowsNewestFirstAndOnlyWhatFits(t *testing.T) {
	entries := []server.RequestLogEntry{logEntry("/b", 200, true), logEntry("/a", 200, true), logEntry("/c", 200, true)}

	view := renderRequestLog(entries, 60, 4) // header, blank line and two rows

	if !strings.Contains(view, "/a") || !strings.Contains(view, "/b") || strings.Contains(view, "/c") {
		t.Fatalf("unexpected rows: %s", view)
	}
	if strings.Index(view, "/b") > strings.Index(view, "/a") {
		t.Errorf("order wasn't kept: %s", view)
	}
}

func TestRowShowsAllFields(t *testing.T) {
	row := requestLogRow(logEntry("/a", 201, true), 60)

	for _, want := range []string{"14:32:07", "GET", "/a", "201", "802ms"} {
		if !strings.Contains(row, want) {
			t.Errorf("row %q is missing %q", row, want)
		}
	}
}

func TestOnlyUnmatchedRowsSayNoMock(t *testing.T) {
	if row := requestLogRow(logEntry("/a", 404, false), 60); !strings.Contains(row, "no mock") || strings.Contains(row, "ms") {
		t.Errorf("unmatched row: %q", row)
	}
	if row := requestLogRow(logEntry("/a", 404, true), 60); strings.Contains(row, "no mock") {
		t.Errorf("matched row: %q", row)
	}
}

func TestUnmatchedRowsGiveThePathMatchedRowsRoom(t *testing.T) {
	const path = "/api/v1/ordrs"
	const width = 48
	for _, matched := range []bool{true, false} {
		if row := requestLogRow(logEntry(path, 404, matched), width); !strings.Contains(row, path) {
			t.Errorf("matched=%v: path was cut: %q", matched, row)
		}
	}
}

func TestStatusColors(t *testing.T) {
	tests := []struct {
		status int
		want   lipgloss.Style
	}{
		{200, delayLowStyle},
		{301, lipgloss.NewStyle()},
		{404, delayMidStyle},
		{500, delayHighStyle},
	}
	for _, tt := range tests {
		if got := statusStyle(tt.status); got.GetForeground() != tt.want.GetForeground() {
			t.Errorf("%d: got %v, want %v", tt.status, got.GetForeground(), tt.want.GetForeground())
		}
	}
}

func TestLogRefreshesOnTrafficTick(t *testing.T) {
	m := sizedTestModel(t)
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	if err := server.Start(port); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/from-tick-test", port))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	m = send(t, m, trafficTickMsg{})

	if len(m.requestLog) == 0 || m.requestLog[0].Path != "/from-tick-test" {
		t.Errorf("unexpected log: %+v", m.requestLog)
	}
}

func TestEmptyLogMessage(t *testing.T) {
	if view := renderRequestLog(nil, 60, 10); !strings.Contains(view, "No requests yet") {
		t.Errorf("unexpected view: %s", view)
	}
}

func TestTypingRInFilter(t *testing.T) {
	m := sizedTestModel(t)
	m = press(t, m, "/")
	m = press(t, m, "r")

	if m.showRequestLog || m.list.FilterInput.Value() != "r" {
		t.Errorf("showRequestLog=%v filter=%q", m.showRequestLog, m.list.FilterInput.Value())
	}
}

func TestHintLineListsRequests(t *testing.T) {
	if view := sizedTestModel(t).renderListView(); !strings.Contains(view, "requests: r") {
		t.Errorf("hint line missing key: %s", view)
	}
}

func TestRowRemovesControlCharacters(t *testing.T) {
	entry := logEntry("/a\x1b[31m\x07", 200, true)
	entry.Method = "G\x1bT"

	row := requestLogRow(entry, 60)

	if strings.ContainsAny(row, "\x07") || strings.Contains(row, "\x1b[31m") {
		t.Errorf("control characters kept: %q", row)
	}
	if !strings.Contains(row, "GT") {
		t.Errorf("method not sanitized: %q", row)
	}
}

func TestLongRowFitsOneLineAndKeepsStatus(t *testing.T) {
	const width = 48
	for _, matched := range []bool{true, false} {
		row := requestLogRow(logEntry("/"+strings.Repeat("long/", 50), 201, matched), width)

		if got := lipgloss.Width(row); got > width {
			t.Errorf("row is %d wide, limit %d", got, width)
		}
		if strings.Contains(row, "\n") {
			t.Errorf("row has several lines: %q", row)
		}
		if !strings.Contains(row, "201") || !strings.Contains(row, "…") {
			t.Errorf("expected shortened path and visible status: %q", row)
		}
	}
}

func TestPanelSurvivesPopupsAndEmptyList(t *testing.T) {
	m := sizedTestModel(t)
	m = press(t, m, "r")

	m = press(t, m, "d")
	if view := m.renderView(); !strings.Contains(view, "No requests yet") {
		t.Errorf("popup hid the log: %s", view)
	}
	m = press(t, m, "y")
	m = press(t, m, "d")
	m = press(t, m, "y")

	if len(m.list.Items()) != 0 || !m.showRequestLog {
		t.Fatalf("items=%d showRequestLog=%v", len(m.list.Items()), m.showRequestLog)
	}
	if view := m.renderListView(); !strings.Contains(view, "No requests yet") {
		t.Errorf("empty list hid the log: %s", view)
	}
}
