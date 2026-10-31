package tui

import (
	"strings"
	"testing"
)

func TestNormalizePathParams(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "/users/{id}", want: "/users/{id}"},
		{in: "/a/{_b1}/{c}", want: "/a/{_b1}/{c}"},
		{in: "users/{id}/orders", want: "/users/{id}/orders"},
		{in: "/files/{id}.json", wantErr: true},
		{in: "/a{b}", wantErr: true},
		{in: "/{}", wantErr: true},
		{in: "/{1a}", wantErr: true},
		{in: "/{a-b}", wantErr: true},
		{in: "/a}", wantErr: true},
		{in: "/{a", wantErr: true},
		{in: "/{id}/{id}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := normalizePath(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// addRoute fills the path and method of a new mock and presses Enter on the method.
func addRoute(t *testing.T, m model, path, method string) model {
	t.Helper()
	m = press(t, m, "a")
	m = typeText(t, m, path)
	m = pressEnter(t, m)
	m = typeText(t, m, method)
	return pressEnter(t, m)
}

func TestFormStaysOnPathStepForInvalidParam(t *testing.T) {
	m := newEditTestModel(t)
	m = press(t, m, "a")
	m = typeText(t, m, "/files/{id}.json")
	m = pressEnter(t, m)

	if m.formStep != formStepPath || m.formError == "" || m.formPath != "/files/{id}.json" {
		t.Errorf("step=%d error=%q path=%q", m.formStep, m.formError, m.formPath)
	}
}

func TestParamNamesDontMakeARouteDifferent(t *testing.T) {
	m := newEditTestModel(t)
	m = addRoute(t, m, "/users/{id}", "GET")
	m = submitForm(t, m)
	if got := savedMocks(t)[len(savedMocks(t))-1].Title; got != "GET /users/{id}" {
		t.Fatalf("saved title = %q", got)
	}

	duplicate := addRoute(t, m, "/users/{name}", "GET")
	if duplicate.formStep != formStepMethod || !strings.Contains(duplicate.formError, "already exists") {
		t.Errorf("expected duplicate error, got step=%d error=%q", duplicate.formStep, duplicate.formError)
	}

	otherMethod := addRoute(t, m, "/users/{name}", "POST")
	if otherMethod.formStep != formStepStatus || otherMethod.formError != "" {
		t.Errorf("POST should be accepted, got step=%d error=%q", otherMethod.formStep, otherMethod.formError)
	}
}

func TestEditRenamingParamIsNotADuplicateOfItself(t *testing.T) {
	m := newEditTestModel(t)
	m = addRoute(t, m, "/users/{id}", "GET")
	m = submitForm(t, m)

	m.list.Select(len(m.list.Items()) - 1)
	m = press(t, m, "e")
	m = replaceField(t, m, "/users/{name}")
	m = goToStep(t, m, formStepStatus)

	if m.formError != "" {
		t.Errorf("unexpected error: %s", m.formError)
	}
}
