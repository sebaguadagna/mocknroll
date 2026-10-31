package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func resetRequestLog(t *testing.T) {
	t.Helper()
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.requestLog = nil
}

func request(method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handleMock(w, httptest.NewRequest(method, target, nil))
	return w
}

func TestLogRecordsMatchedRequest(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", Status: 201, Enabled: true}})

	before := time.Now()
	request("GET", "/a")

	entries := RecentRequests()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	got := entries[0]
	if got.Method != "GET" || got.Path != "/a" || got.Status != 201 || !got.Matched {
		t.Errorf("unexpected entry: %+v", got)
	}
	if got.Time.Before(before) || got.Duration < 0 {
		t.Errorf("unexpected time or duration: %+v", got)
	}
}

func TestLogRecordsUnmatchedRequest(t *testing.T) {
	resetRequestLog(t)
	SetMocks(nil)

	request("GET", "/nope")

	got := RecentRequests()[0]
	if got.Status != http.StatusNotFound || got.Matched {
		t.Errorf("unexpected entry: %+v", got)
	}
}

func TestLogRecordsUnreadableFileStatus(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", Status: 200, JSONFile: "does-not-exist.json", Enabled: true}})

	request("GET", "/a")

	got := RecentRequests()[0]
	if got.Status != http.StatusInternalServerError || !got.Matched {
		t.Errorf("unexpected entry: %+v", got)
	}
}

func TestLogDurationIncludesDelay(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", DelayMs: 50, Enabled: true}})

	request("GET", "/a")

	if got := RecentRequests()[0].Duration; got < 50*time.Millisecond {
		t.Errorf("expected at least 50ms, got %v", got)
	}
}

func TestLogIsBoundedAndNewestFirst(t *testing.T) {
	resetRequestLog(t)
	SetMocks(nil)

	request("GET", "/first")
	for range requestLogSize - 1 {
		request("GET", "/middle")
	}
	request("GET", "/last")

	entries := RecentRequests()
	if len(entries) != requestLogSize {
		t.Fatalf("expected %d entries, got %d", requestLogSize, len(entries))
	}
	if entries[0].Path != "/last" {
		t.Errorf("expected newest first, got %s", entries[0].Path)
	}
	for _, entry := range entries {
		if entry.Path == "/first" {
			t.Fatal("oldest entry wasn't discarded")
		}
	}
}

func TestRecentRequestsReturnsACopy(t *testing.T) {
	resetRequestLog(t)
	SetMocks(nil)
	request("GET", "/a")

	RecentRequests()[0].Path = "/changed"

	if got := RecentRequests()[0].Path; got != "/a" {
		t.Errorf("log was modified through the copy: %s", got)
	}
}

func TestLoggingDoesNotChangeResponse(t *testing.T) {
	resetRequestLog(t)
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte(`{"ok":true}`), 0644); err != nil {
		t.Fatal(err)
	}
	SetMocks([]Mock{{Method: "GET", Path: "/a", Status: 201, JSONFile: file, Enabled: true}})

	w := request("GET", "/a")

	if w.Code != http.StatusCreated || w.Header().Get("Content-Type") != "application/json" || w.Body.String() != `{"ok":true}` {
		t.Errorf("unexpected response: %d %q %q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
}

func TestLogWithConcurrentRequests(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", Enabled: true}})

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request("GET", "/a")
			RecentRequests()
		}()
	}
	wg.Wait()

	if got := len(RecentRequests()); got != 50 {
		t.Errorf("expected 50 entries, got %d", got)
	}
}

func TestRequestInProgressIsNotLogged(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/slow", DelayMs: 100, Enabled: true}})

	done := make(chan struct{})
	go func() {
		request("GET", "/slow")
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	if got := len(RecentRequests()); got != 0 {
		t.Errorf("expected no entries during the delay, got %d", got)
	}
	<-done
	if got := len(RecentRequests()); got != 1 {
		t.Errorf("expected 1 entry after completion, got %d", got)
	}
}

func TestDisconnectedClientIsStillLogged(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/slow", DelayMs: 100, Enabled: true}})
	srv := httptest.NewServer(http.HandlerFunc(handleMock))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/slow", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp, err := http.DefaultClient.Do(req); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		t.Fatal("expected the client to time out")
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(RecentRequests()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := len(RecentRequests()); got != 1 {
		t.Errorf("expected 1 entry, got %d", got)
	}
}

func TestChangingMocksKeepsLog(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", Enabled: true}})
	request("GET", "/a")

	SetMocks(nil)

	if got := len(RecentRequests()); got != 1 {
		t.Errorf("expected 1 entry, got %d", got)
	}
}

func TestLogOmitsQueryString(t *testing.T) {
	resetRequestLog(t)
	SetMocks(nil)

	request("GET", "/a?token=secret")

	if got := RecentRequests()[0].Path; got != "/a" {
		t.Errorf("expected /a, got %q", got)
	}
}
