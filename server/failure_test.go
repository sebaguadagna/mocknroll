package server

import (
	"net/http"
	"testing"
	"time"
)

const simulatedFailureBody = `{"error": "simulated failure"}`

func TestDelayRangeVaries(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", DelayMs: 20, DelayMaxMs: 60, Enabled: true}})

	for range 20 {
		request("GET", "/a")
	}

	shortest, longest := time.Hour, time.Duration(0)
	for _, entry := range RecentRequests() {
		shortest = min(shortest, entry.Duration)
		longest = max(longest, entry.Duration)
	}
	if shortest < 20*time.Millisecond {
		t.Errorf("waited less than the minimum: %v", shortest)
	}
	if longest-shortest < 5*time.Millisecond {
		t.Errorf("waits barely varied: %v to %v", shortest, longest)
	}
}

func countFailures(requests, rate int) int {
	SetMocks([]Mock{{Method: "GET", Path: "/a", Status: 200, FailureRate: rate, FailureStatus: 503, Enabled: true}})
	failures := 0
	for range requests {
		if request("GET", "/a").Code == http.StatusServiceUnavailable {
			failures++
		}
	}
	return failures
}

func TestFailureRates(t *testing.T) {
	tests := []struct {
		name     string
		rate     int
		requests int
		low      int
		high     int
	}{
		{"never at 0", 0, 200, 0, 0},
		{"always at 100", 100, 200, 200, 200},
		{"about half at 50", 50, 1000, 400, 600},
		{"negative behaves as 0", -5, 200, 0, 0},
		{"above 100 behaves as 100", 150, 200, 200, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countFailures(tt.requests, tt.rate); got < tt.low || got > tt.high {
				t.Errorf("got %d failures, want %d to %d", got, tt.low, tt.high)
			}
		})
	}
}

func TestFailureWaitsDelayAndUsesItsOwnResponse(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", Status: 200, DelayMs: 50, JSONFile: "does-not-exist.json", FailureRate: 100, FailureStatus: 503, Enabled: true}})

	start := time.Now()
	w := request("GET", "/a")

	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("failure didn't wait the delay: %v", elapsed)
	}
	if w.Code != http.StatusServiceUnavailable || w.Body.String() != simulatedFailureBody {
		t.Errorf("unexpected response: %d %q", w.Code, w.Body.String())
	}
	if entry := RecentRequests()[0]; !entry.Simulated || !entry.Matched || entry.Status != 503 {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestRealErrorsAreNotMarkedSimulated(t *testing.T) {
	resetRequestLog(t)
	SetMocks([]Mock{{Method: "GET", Path: "/a", Status: 500, Enabled: true}})

	request("GET", "/a")

	if RecentRequests()[0].Simulated {
		t.Error("a configured 500 was marked as simulated")
	}
}

func TestInvalidFailureStatusAnswers500(t *testing.T) {
	for _, status := range []int{0, 99, 200, 1000} {
		SetMocks([]Mock{{Method: "GET", Path: "/a", FailureRate: 100, FailureStatus: status, Enabled: true}})

		if got := request("GET", "/a").Code; got != http.StatusInternalServerError {
			t.Errorf("status %d: got %d", status, got)
		}
	}
}

func TestFailuresCountAsTraffic(t *testing.T) {
	SetMocks([]Mock{{Method: "GET", Path: "/counted", FailureRate: 100, FailureStatus: 503, Enabled: true}})
	GetAndResetRequestCounts()

	request("GET", "/counted")

	if got := GetAndResetRequestCounts()["GET /counted"]; got != 1 {
		t.Errorf("expected 1 request counted, got %d", got)
	}
}
