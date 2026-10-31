package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func writeBody(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestPathParamMatching(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		target  string
		want    int
	}{
		{"number", "/users/{id}", "/users/42", http.StatusOK},
		{"text", "/users/{id}", "/users/abc", http.StatusOK},
		{"query string", "/users/{id}", "/users/42?a=1", http.StatusOK},
		{"no segment", "/users/{id}", "/users", http.StatusNotFound},
		{"empty segment", "/users/{id}", "/users/", http.StatusNotFound},
		{"extra segment", "/users/{id}", "/users/42/orders", http.StatusNotFound},
		{"encoded slash", "/users/{id}", "/users/a%2Fb", http.StatusNotFound},
		{"middle param", "/users/{id}/orders", "/users/42/orders", http.StatusOK},
		{"literals keep their case", "/users/{id}/orders", "/users/42/ORDERS", http.StatusNotFound},
		{"invalid param is literal text", "/{1a}", "/x", http.StatusNotFound},
		{"invalid param matches itself", "/{1a}", "/%7B1a%7D", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMocks([]Mock{{Method: "GET", Path: tt.pattern, Status: 200, Enabled: true}})
			if got := request("GET", tt.target).Code; got != tt.want {
				t.Errorf("%s on %s answered %d, want %d", tt.target, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestPathParamPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		mocks  []Mock
		target string
		want   int
	}{
		{
			name:   "literal beats an earlier param",
			mocks:  []Mock{{Path: "/users/{id}", Status: 201, Enabled: true}, {Path: "/users/me", Status: 202, Enabled: true}},
			target: "/users/me",
			want:   202,
		},
		{
			name:   "param still answers other values",
			mocks:  []Mock{{Path: "/users/{id}", Status: 201, Enabled: true}, {Path: "/users/me", Status: 202, Enabled: true}},
			target: "/users/42",
			want:   201,
		},
		{
			name:   "tie goes to the first",
			mocks:  []Mock{{Path: "/a/{x}/c", Status: 201, Enabled: true}, {Path: "/a/b/{y}", Status: 202, Enabled: true}},
			target: "/a/b/c",
			want:   201,
		},
		{
			name:   "disabled literal leaves the route to the param",
			mocks:  []Mock{{Path: "/users/me", Status: 202}, {Path: "/users/{id}", Status: 201, Enabled: true}},
			target: "/users/me",
			want:   201,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.mocks {
				tt.mocks[i].Method = "GET"
			}
			SetMocks(tt.mocks)
			if got := request("GET", tt.target).Code; got != tt.want {
				t.Errorf("answered %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPathParamSubstitution(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		content string
		target  string
		want    string
	}{
		{"value", "/users/{id}", `{"id": "{id}"}`, "/users/42", `{"id": "42"}`},
		{"unknown name unchanged", "/users/{id}", `{"id": "{id}", "x": "{other}"}`, "/users/42", `{"id": "42", "x": "{other}"}`},
		{"not substituted twice", "/{a}/{b}", `"{a} {b}"`, "/%7Bb%7D/x", `"{b} x"`},
		{"decoded value", "/users/{id}", `"{id}"`, "/users/a%20b", `"a b"`},
		{"repeated name uses the last value", "/{id}/{id}", `"{id}"`, "/a/b", `"b"`},
		{"no params leaves braces alone", "/plain", `{"id": "{id}"}`, "/plain", `{"id": "{id}"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMocks([]Mock{{Method: "GET", Path: tt.pattern, Status: 200, JSONFile: writeBody(t, tt.content), Enabled: true}})
			if got := request("GET", tt.target).Body.String(); got != tt.want {
				t.Errorf("body = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCapturedValuesStayValidJSON(t *testing.T) {
	file := writeBody(t, `{"id": "{id}"}`)
	SetMocks([]Mock{{Method: "GET", Path: "/users/{id}", Status: 200, JSONFile: file, Enabled: true}})

	for target, want := range map[string]string{
		"/users/a%22b":    `a"b`,
		"/users/a%5Cb":    `a\b`,
		"/users/a%1b%5Bb": "a\x1b[b",
	} {
		var got struct {
			ID string `json:"id"`
		}
		body := request("GET", target).Body.Bytes()
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("%s: invalid JSON %s: %v", target, body, err)
		}
		if got.ID != want {
			t.Errorf("%s: id = %q, want %q", target, got.ID, want)
		}
	}
}

func TestSubstitutionLeavesFileUnchanged(t *testing.T) {
	content := `{"id": "{id}"}`
	file := writeBody(t, content)
	SetMocks([]Mock{{Method: "GET", Path: "/users/{id}", Status: 200, JSONFile: file, Enabled: true}})

	request("GET", "/users/42")

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("file changed to %s", got)
	}
}

func TestParamMockTrafficAndLog(t *testing.T) {
	resetRequestLog(t)
	GetAndResetRequestCounts()
	SetMocks([]Mock{{Method: "GET", Path: "/users/{id}", Status: 200, Enabled: true}})

	request("GET", "/users/42")

	if got := GetAndResetRequestCounts()["GET /users/{id}"]; got != 1 {
		t.Errorf("traffic = %d, want 1", got)
	}
	if got := RecentRequests()[0].Path; got != "/users/42" {
		t.Errorf("logged path = %s", got)
	}
}

func TestSimulatedFailureHasNoSubstitution(t *testing.T) {
	file := writeBody(t, `{"id": "{id}"}`)
	SetMocks([]Mock{{Method: "GET", Path: "/users/{id}", Status: 200, JSONFile: file, FailureRate: 100, FailureStatus: 503, Enabled: true}})

	if got := request("GET", "/users/42").Body.String(); got != simulatedFailureBody {
		t.Errorf("body = %s", got)
	}
}

func TestParamMockWithoutFileAnswersEmptyObject(t *testing.T) {
	SetMocks([]Mock{{Method: "GET", Path: "/users/{id}", Status: 200, Enabled: true}})

	if got := request("GET", "/users/42").Body.String(); got != "{}" {
		t.Errorf("body = %s", got)
	}
}
