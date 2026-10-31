package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebaguadagna/mocknroll/examples"
)

// assertExampleFile checks that name in ./examples matches the embedded copy.
func assertExampleFile(t *testing.T, name string) {
	t.Helper()
	want, err := examples.Files.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("examples", name))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("%s doesn't match the embedded file: %q", name, got)
	}
}

func TestFirstRunCreatesExampleFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := initialModel(8080); err != nil {
		t.Fatal(err)
	}
	assertExampleFile(t, "users.json")
	assertExampleFile(t, "orders.json")
}

func TestFirstRunKeepsExistingExampleFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("examples", 0755); err != nil {
		t.Fatal(err)
	}
	const mine = `{"mine": true}`
	path := filepath.Join("examples", "users.json")
	if err := os.WriteFile(path, []byte(mine), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := initialModel(8080); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != mine {
		t.Errorf("users.json was overwritten: %q", got)
	}
	assertExampleFile(t, "orders.json")
}

func TestExistingConfigDoesNotCreateExampleFiles(t *testing.T) {
	tests := map[string]string{
		"empty list": `[]`,
		"one mock":   `[{"title": "GET /a", "status": "200", "enabled": true}]`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("mocks.json", []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := initialModel(8080); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat("examples"); !os.IsNotExist(err) {
				t.Errorf("examples was created, or stat failed: %v", err)
			}
		})
	}
}

func TestFirstRunStartsWhenExamplesIsAFile(t *testing.T) {
	t.Chdir(t.TempDir())
	const content = "not a directory"
	if err := os.WriteFile("examples", []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := initialModel(8080); err != nil {
		t.Fatalf("the app should still start: %v", err)
	}

	got, err := os.ReadFile("examples")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("the examples file changed: %q", got)
	}
}

func TestFirstRunStartsInAReadOnlyDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	t.Chdir(dir)

	if _, err := initialModel(8080); err != nil {
		t.Fatalf("the app should still start: %v", err)
	}
	if _, err := os.Stat("examples"); !os.IsNotExist(err) {
		t.Errorf("examples shouldn't exist: %v", err)
	}
}
