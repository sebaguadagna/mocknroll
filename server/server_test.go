package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMockServer(t *testing.T) {
	// Create a temporary JSON file for test responses
	tmpFile, err := os.CreateTemp("", "test_mock_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testJSON := `{"message": "hello world"}`
	if _, err := tmpFile.Write([]byte(testJSON)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Set up mock configurations
	mocks := []Mock{
		{
			Method:   "GET",
			Path:     "/api/test",
			Status:   200,
			DelayMs:  10,
			JSONFile: tmpFile.Name(),
			Enabled:  true,
		},
		{
			Method:   "POST",
			Path:     "/api/create",
			Status:   201,
			DelayMs:  0,
			JSONFile: "",
			Enabled:  true,
		},
		{
			Method:   "GET",
			Path:     "/api/disabled",
			Status:   200,
			DelayMs:  0,
			JSONFile: tmpFile.Name(),
			Enabled:  false,
		},
	}

	SetMocks(mocks)

	t.Run("GET enabled mock", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/test", nil)
		w := httptest.NewRecorder()

		start := time.Now()
		handleMock(w, req)
		duration := time.Since(start)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
		if string(body) != testJSON {
			t.Errorf("expected body %q, got %q", testJSON, string(body))
		}
		if duration < 10*time.Millisecond {
			t.Errorf("expected delay of at least 10ms, took %v", duration)
		}

		// Check traffic tracking
		counts := GetAndResetRequestCounts()
		if counts["GET /api/test"] != 1 {
			t.Errorf("expected traffic count to be 1, got %d", counts["GET /api/test"])
		}

		// Ensure it was reset
		counts2 := GetAndResetRequestCounts()
		if counts2["GET /api/test"] != 0 {
			t.Errorf("expected traffic count to be reset to 0, got %d", counts2["GET /api/test"])
		}
	})

	t.Run("POST enabled mock with no json file", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/create", nil)
		w := httptest.NewRecorder()

		handleMock(w, req)

		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("expected status 201, got %d", resp.StatusCode)
		}
		if string(body) != "{}" {
			t.Errorf("expected default empty json {}, got %q", string(body))
		}

		counts := GetAndResetRequestCounts()
		if counts["POST /api/create"] != 1 {
			t.Errorf("expected traffic count to be 1, got %d", counts["POST /api/create"])
		}
	})

	t.Run("GET disabled mock", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/disabled", nil)
		w := httptest.NewRecorder()

		handleMock(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404 for disabled mock, got %d", resp.StatusCode)
		}
	})

	t.Run("GET non-existent mock", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/not-found", nil)
		w := httptest.NewRecorder()

		handleMock(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404 for non-existent mock, got %d", resp.StatusCode)
		}
	})
}
