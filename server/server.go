package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Mock represents a single mock configuration.
type Mock struct {
	Method   string
	Path     string
	Status   int
	DelayMs  int
	JSONFile string
	Enabled  bool
}

// RequestLogEntry describes one completed request.
type RequestLogEntry struct {
	Time     time.Time
	Method   string
	Path     string
	Status   int
	Duration time.Duration
	Matched  bool // false when no enabled mock matched
}

// requestLogSize bounds the memory the log can use under any request volume.
const requestLogSize = 200

type registry struct {
	mu            sync.RWMutex
	mocks         []Mock
	requestCounts map[string]int
	requestLog    []RequestLogEntry // oldest first
}

var reg = &registry{
	requestCounts: make(map[string]int),
}

// Start binds the port and serves mocks in the background. Binding happens
// before returning so callers learn about a busy port right away.
func Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleMock)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}

	// Run the HTTP server in a separate background goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Mock server error: %v\n", err)
		}
	}()

	return nil
}

// SetMocks updates the list of active mocks on the server.
func SetMocks(mocks []Mock) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.mocks = mocks
}

// GetAndResetRequestCounts returns the request counts since the last call and resets them.
func GetAndResetRequestCounts() map[string]int {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	counts := make(map[string]int)
	for k, v := range reg.requestCounts {
		counts[k] = v
		reg.requestCounts[k] = 0
	}
	return counts
}

// RecentRequests returns a copy of the logged requests, newest first.
func RecentRequests() []RequestLogEntry {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	entries := make([]RequestLogEntry, len(reg.requestLog))
	for i, entry := range reg.requestLog {
		entries[len(entries)-1-i] = entry
	}
	return entries
}

func recordRequest(entry RequestLogEntry) {
	reg.mu.Lock()
	defer reg.mu.Unlock()

	if len(reg.requestLog) == requestLogSize {
		reg.requestLog = append(reg.requestLog[:0], reg.requestLog[1:]...)
	}
	reg.requestLog = append(reg.requestLog, entry)
}

// statusRecorder captures the status a handler sends. A handler that never
// calls WriteHeader sends 200.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func handleMock(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	w = recorder
	matched := false
	defer func() {
		recordRequest(RequestLogEntry{
			Time:     time.Now(),
			Method:   r.Method,
			Path:     r.URL.Path,
			Status:   recorder.status,
			Duration: time.Since(start),
			Matched:  matched,
		})
	}()

	reg.mu.RLock()
	var matchedMock *Mock
	for i := range reg.mocks {
		m := &reg.mocks[i]
		if m.Enabled && r.Method == m.Method && r.URL.Path == m.Path {
			matchedMock = m
			break
		}
	}
	reg.mu.RUnlock()

	if matchedMock == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error": "no matching mock found"}`)
		return
	}

	matched = true

	// Increment the request count
	reg.mu.Lock()
	key := matchedMock.Method + " " + matchedMock.Path
	reg.requestCounts[key]++
	reg.mu.Unlock()

	// Simulate latency if configured
	if matchedMock.DelayMs > 0 {
		time.Sleep(time.Duration(matchedMock.DelayMs) * time.Millisecond)
	}

	// Read and serve the JSON file
	var body []byte
	var err error
	if matchedMock.JSONFile != "" {
		body, err = os.ReadFile(matchedMock.JSONFile)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "failed to read json file: %v"}`, err)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	// WriteHeader panics outside this range, and mocks.json can be hand-edited.
	if matchedMock.Status != 0 && (matchedMock.Status < 100 || matchedMock.Status > 999) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "invalid status code in mock: %d"}`, matchedMock.Status)
		return
	}
	if matchedMock.Status > 0 {
		w.WriteHeader(matchedMock.Status)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if len(body) > 0 {
		w.Write(body)
	} else {
		io.WriteString(w, "{}")
	}
}
