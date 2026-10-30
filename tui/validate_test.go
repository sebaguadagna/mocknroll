package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizers(t *testing.T) {
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		normalize func(string) (string, error)
		in        string
		want      string
		wantErr   bool
	}{
		{"path as is", normalizePath, "/a", "/a", false},
		{"path gets slash", normalizePath, "a/b", "/a/b", false},
		{"path empty", normalizePath, "  ", "", true},
		{"path with space", normalizePath, "/a b", "", true},
		{"method uppercased", normalizeMethod, " get ", "GET", false},
		{"method unknown", normalizeMethod, "FETCH", "", true},
		{"method empty", normalizeMethod, "", "", true},
		{"status default", normalizeStatus, "", "200", false},
		{"status valid", normalizeStatus, "404", "404", false},
		{"status low", normalizeStatus, "99", "", true},
		{"status informational", normalizeStatus, "100", "", true},
		{"status high", normalizeStatus, "600", "", true},
		{"status text", normalizeStatus, "abc", "", true},
		{"delay default", normalizeDelay, "", "0", false},
		{"delay valid", normalizeDelay, "250", "250", false},
		{"delay negative", normalizeDelay, "-1", "", true},
		{"delay text", normalizeDelay, "fast", "", true},
		{"file empty", normalizeJSONFile, "", "", false},
		{"file exists", normalizeJSONFile, file, file, false},
		{"file missing", normalizeJSONFile, file + ".nope", "", true},
		{"file is dir", normalizeJSONFile, filepath.Dir(file), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.normalize(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
