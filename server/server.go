package server

import (
	"fmt"
	"io"
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

type registry struct {
	mu            sync.RWMutex
	mocks         []Mock
	requestCounts map[string]int
}

var reg = &registry{
	requestCounts: make(map[string]int),
}

// Start runs the mock HTTP server on the specified port.
func Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleMock)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Run the HTTP server in a separate background goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

func handleMock(w http.ResponseWriter, r *http.Request) {
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
