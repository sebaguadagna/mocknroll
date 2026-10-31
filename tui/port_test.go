package tui

import (
	"strings"
	"testing"
)

func TestTitleShowsThePort(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{8080, "http://localhost:8080"},
		{18080, "http://localhost:18080"},
	}
	for _, tt := range tests {
		t.Chdir(t.TempDir()) // the model persists to ./mocks.json
		m, err := initialModel(tt.port)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(m.list.Title, tt.want) {
			t.Errorf("title %q doesn't end with %q", m.list.Title, tt.want)
		}
	}
}
