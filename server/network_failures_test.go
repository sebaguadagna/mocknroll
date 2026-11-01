package server

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func dialTestServer(t *testing.T) (addr string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handleMock))
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().String()
}

// sendRaw opens a connection and writes a request's text, with no HTTP client
// in between to hide what the server does.
func sendRaw(t *testing.T, addr, text string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := io.WriteString(conn, text); err != nil {
		t.Fatal(err)
	}
	return conn
}

const rawGet = "GET /a HTTP/1.1\r\nHost: x\r\n\r\n"

func failingMock(failureType string) Mock {
	return Mock{Method: "GET", Path: "/a", Status: 200, FailureRate: 100, FailureStatus: 503, FailureType: failureType, Enabled: true}
}

func waitForLog(t *testing.T, want int) []RequestLogEntry {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		entries := RecentRequests()
		if len(entries) >= want {
			return entries
		}
		if time.Now().After(deadline) {
			t.Fatalf("got %d log entries, want %d", len(entries), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func assertReset(t *testing.T, conn net.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	data, err := io.ReadAll(conn)
	if len(data) != 0 {
		t.Errorf("got response bytes: %q", data)
	}
	if !errors.Is(err, syscall.ECONNRESET) {
		t.Errorf("expected a connection reset, got %v", err)
	}
}

func TestResetAbortsTheConnection(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{failingMock(FailureReset)})
	assertReset(t, sendRaw(t, dialTestServer(t), rawGet))

	entry := waitForLog(t, 1)[0]
	if entry.Status != 0 || entry.FailureType != FailureReset || !entry.Simulated || !entry.Matched {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestResetWaitsTheDelay(t *testing.T) {
	mock := failingMock(FailureReset)
	mock.DelayMs = 200
	SetMocks([]Mock{mock})

	start := time.Now()
	assertReset(t, sendRaw(t, dialTestServer(t), rawGet))
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Errorf("reset didn't wait the delay: %v", elapsed)
	}
}

func TestResetFailsWithAnHTTPClient(t *testing.T) {
	SetMocks([]Mock{failingMock(FailureReset)})
	resp, err := http.Get("http://" + dialTestServer(t) + "/a")
	if err == nil {
		resp.Body.Close()
		t.Fatalf("expected a connection error, got status %d", resp.StatusCode)
	}
}

func TestHangSendsNothingUntilTheClientLeaves(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{failingMock(FailureHang)})
	conn := sendRaw(t, dialTestServer(t), rawGet)

	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	buf := make([]byte, 1)
	n, err := conn.Read(buf)
	var netErr net.Error
	if n != 0 || !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("expected a timeout with no bytes, got %d bytes and %v", n, err)
	}
	if got := len(RecentRequests()); got != 0 {
		t.Errorf("logged before the client left: %d entries", got)
	}

	conn.Close()
	entry := waitForLog(t, 1)[0]
	if entry.Status != 0 || entry.FailureType != FailureHang || entry.Duration < 300*time.Millisecond {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestHangTimesOutAnHTTPClient(t *testing.T) {
	SetMocks([]Mock{failingMock(FailureHang)})
	client := http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Get("http://" + dialTestServer(t) + "/a")
	if err == nil {
		resp.Body.Close()
		t.Fatalf("expected a timeout, got status %d", resp.StatusCode)
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("expected a timeout, got %v", err)
	}
}

func TestHangLeavesNoGoroutinesBehind(t *testing.T) {
	resetRequestLog(t)
	post := failingMock(FailureHang)
	post.Method = "POST"
	SetMocks([]Mock{failingMock(FailureHang), post})
	GetAndResetRequestCounts()
	addr := dialTestServer(t)
	baseline := runtime.NumGoroutine()

	const requests = 100
	conns := make([]net.Conn, 0, requests)
	for i := range requests {
		text := rawGet
		if i%2 == 0 { // announces a body it never finishes sending
			text = "POST /a HTTP/1.1\r\nHost: x\r\nContent-Length: 1000\r\n\r\npartial"
		}
		conns = append(conns, sendRaw(t, addr, text))
	}
	// Traffic is counted when a request reaches the handler. Counting goroutines
	// here wouldn't work, since connections from earlier tests may be closing.
	started := 0
	deadline := time.Now().Add(2 * time.Second)
	for started < requests {
		counts := GetAndResetRequestCounts()
		started += counts["GET /a"] + counts["POST /a"]
		if time.Now().After(deadline) {
			t.Fatalf("only %d of %d requests started", started, requests)
		}
		time.Sleep(5 * time.Millisecond)
	}

	for _, conn := range conns {
		conn.Close()
	}
	deadline = time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > baseline {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines left, started with %d", runtime.NumGoroutine(), baseline)
		}
		time.Sleep(5 * time.Millisecond)
	}

	entries := waitForLog(t, requests)
	for _, entry := range entries {
		if entry.FailureType != FailureHang {
			t.Fatalf("unexpected entry: %+v", entry)
		}
	}
}

func TestDeletingAMockDoesNotEndAHang(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{failingMock(FailureHang)})
	conn := sendRaw(t, dialTestServer(t), rawGet)
	time.Sleep(50 * time.Millisecond)

	SetMocks(nil)

	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var netErr net.Error
	if _, err := conn.Read(make([]byte, 1)); !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("expected the request to stay open, got %v", err)
	}
	conn.Close()
	waitForLog(t, 1)
}

func TestClientLeavingDuringDelayIsHarmless(t *testing.T) {
	for _, failureType := range []string{FailureReset, FailureHang} {
		resetRequestLog(t)
		mock := failingMock(failureType)
		mock.DelayMs = 100
		SetMocks([]Mock{mock})
		sendRaw(t, dialTestServer(t), rawGet).Close()

		if entry := waitForLog(t, 1)[0]; entry.FailureType != failureType {
			t.Errorf("unexpected entry: %+v", entry)
		}
	}
}

func TestNetworkFailuresCountAsTraffic(t *testing.T) {
	SetMocks([]Mock{failingMock(FailureReset)})
	GetAndResetRequestCounts()
	addr := dialTestServer(t)
	for range 3 {
		assertReset(t, sendRaw(t, addr, rawGet))
	}
	if got := GetAndResetRequestCounts()["GET /a"]; got != 3 {
		t.Errorf("got %d requests counted, want 3", got)
	}
}

func TestHTTPAndUnknownTypesAnswerTheStatus(t *testing.T) {
	addr := dialTestServer(t)
	for _, failureType := range []string{"", FailureHTTP, "nope"} {
		SetMocks([]Mock{failingMock(failureType)})
		resp, err := http.Get("http://" + addr + "/a")
		if err != nil {
			t.Fatalf("%q: %v", failureType, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable || string(body) != simulatedFailureBody {
			t.Errorf("%q: got %d %q", failureType, resp.StatusCode, body)
		}
	}
}

func TestRateZeroNeverFails(t *testing.T) {
	mock := failingMock(FailureReset)
	mock.FailureRate = 0
	SetMocks([]Mock{mock})
	resp, err := http.Get("http://" + dialTestServer(t) + "/a")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d", resp.StatusCode)
	}
}

func TestResetOnHeadAndParameterRoutes(t *testing.T) {
	head := failingMock(FailureReset)
	head.Method = "HEAD"
	param := failingMock(FailureReset)
	param.Path = "/users/{id}"
	SetMocks([]Mock{head, param})
	addr := dialTestServer(t)

	assertReset(t, sendRaw(t, addr, "HEAD /a HTTP/1.1\r\nHost: x\r\n\r\n"))
	assertReset(t, sendRaw(t, addr, "GET /users/42 HTTP/1.1\r\nHost: x\r\n\r\n"))
}

func TestUnmatchedRequestStillAnswers404(t *testing.T) {
	SetMocks([]Mock{failingMock(FailureReset)})
	conn := sendRaw(t, dialTestServer(t), "GET /other HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	data, err := io.ReadAll(conn)
	if err != nil || !strings.Contains(string(data), "404") {
		t.Errorf("got %q, %v", data, err)
	}
}
