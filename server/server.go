package server

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"strings"
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

	DelayMaxMs    int // above DelayMs, requests wait a random time between the two
	FailureRate   int // percentage of requests that answer FailureStatus
	FailureStatus int
	FailureType   string // how a failing request fails; any unknown value means FailureHTTP
}

// Ways a simulated failure can reach the client.
const (
	FailureHTTP  = "http"  // answers the failure status
	FailureReset = "reset" // aborts the TCP connection, with no response
	FailureHang  = "hang"  // never responds, and waits for the client to leave
)

// RequestLogEntry describes one completed request.
type RequestLogEntry struct {
	Time      time.Time
	Method    string
	Path      string
	Status    int
	Duration  time.Duration
	Matched   bool // false when no enabled mock matched
	Simulated bool // true when the failure was simulated

	FailureType string // FailureReset or FailureHang, which send no response; Status is 0 then
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

// takeConn takes the connection from the HTTP server. If it can't, it aborts
// the handler, which also closes the connection without a response.
func takeConn(w http.ResponseWriter) net.Conn {
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		panic(http.ErrAbortHandler)
	}
	return conn
}

// resetConnection closes the connection with an RST instead of a clean FIN,
// so the client sees "connection reset".
func resetConnection(w http.ResponseWriter) {
	conn := takeConn(w)
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0) // without it, the close is clean instead of a reset
	}
	conn.Close()
}

// hangConnection sends nothing and returns once the client closes. It reads
// the connection itself because a request's context may not notice a client
// leaving while the body is unread, and reading r.Body may send "100 Continue".
func hangConnection(w http.ResponseWriter) {
	conn := takeConn(w)
	defer conn.Close()
	_, _ = io.Copy(io.Discard, conn) // any read error also means the client is gone
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

// ParamName returns the name of a "{name}" path segment. A name starts with a
// letter or underscore and contains only letters, digits and underscores.
func ParamName(segment string) (string, bool) {
	if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		return "", false
	}
	name := segment[1 : len(segment)-1]
	for i, c := range name {
		isLetter := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		if !isLetter && (!isDigit || i == 0) {
			return "", false
		}
	}
	return name, true
}

// matchPath compares a request path to a mock's path, segment by segment. It
// returns the captured parameters and how many segments matched literally.
func matchPath(pattern, path string) (map[string]string, int, bool) {
	patternSegments := strings.Split(pattern, "/")
	pathSegments := strings.Split(path, "/")
	if len(patternSegments) != len(pathSegments) {
		return nil, 0, false
	}

	var params map[string]string
	literals := 0
	for i, segment := range patternSegments {
		if name, ok := ParamName(segment); ok {
			if pathSegments[i] == "" {
				return nil, 0, false
			}
			if params == nil {
				params = make(map[string]string)
			}
			params[name] = pathSegments[i]
			continue
		}
		if segment != pathSegments[i] {
			return nil, 0, false
		}
		literals++
	}
	return params, literals, true
}

// substituteParams replaces each "{name}" in a JSON body with its captured
// value, escaped to stay valid inside a JSON string. A replacer makes a single
// pass, so text inside a captured value is never substituted again.
func substituteParams(body []byte, params map[string]string) []byte {
	if len(params) == 0 {
		return body
	}
	pairs := make([]string, 0, len(params)*2)
	for name, value := range params {
		escaped, _ := json.Marshal(value) // marshaling a string can't fail
		pairs = append(pairs, "{"+name+"}", string(escaped[1:len(escaped)-1]))
	}
	return []byte(strings.NewReplacer(pairs...).Replace(string(body)))
}

func handleMock(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	conn := w // statusRecorder doesn't expose Hijack
	recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	w = recorder
	matched := false
	simulated := false
	failureType := ""
	defer func() {
		recordRequest(RequestLogEntry{
			Time:      time.Now(),
			Method:    r.Method,
			Path:      r.URL.Path,
			Status:    recorder.status,
			Duration:  time.Since(start),
			Matched:   matched,
			Simulated: simulated,

			FailureType: failureType,
		})
	}()

	reg.mu.RLock()
	var matchedMock *Mock
	var params map[string]string
	mostLiterals := -1
	for i := range reg.mocks {
		m := &reg.mocks[i]
		if !m.Enabled || r.Method != m.Method {
			continue
		}
		// Only strictly more literals replace a match: on a tie, the first wins.
		if mockParams, literals, ok := matchPath(m.Path, r.URL.Path); ok && literals > mostLiterals {
			matchedMock = m
			params = mockParams
			mostLiterals = literals
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
	delayMs := matchedMock.DelayMs
	if matchedMock.DelayMaxMs > matchedMock.DelayMs {
		delayMs += rand.IntN(matchedMock.DelayMaxMs - matchedMock.DelayMs + 1)
	}
	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	// Simulate a failure if it's this request's turn. Rates at or below 0
	// never pass this check, and rates at or above 100 always do.
	if rand.IntN(100) < matchedMock.FailureRate {
		simulated = true
		if matchedMock.FailureType == FailureReset || matchedMock.FailureType == FailureHang {
			failureType = matchedMock.FailureType
			recorder.status = 0
			if failureType == FailureReset {
				resetConnection(conn)
			} else {
				hangConnection(conn)
			}
			return
		}
		status := matchedMock.FailureStatus
		if status < 400 || status > 599 { // mocks.json can be hand-edited
			status = http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, `{"error": "simulated failure"}`)
		return
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
		body = substituteParams(body, params)
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
