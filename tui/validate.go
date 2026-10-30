package tui

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

var validMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// normalizePath requires a space-free path (the mock's title is split on
// its first space) and adds the leading slash that requests always have.
func normalizePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is required")
	}
	if strings.ContainsAny(path, " \t") {
		return "", errors.New("path can't contain spaces")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path, nil
}

func normalizeMethod(method string) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if !slices.Contains(validMethods, method) {
		return "", fmt.Errorf("method must be one of %s", strings.Join(validMethods, ", "))
	}
	return method, nil
}

// normalizeStatus defaults to 200. 1xx is excluded because it isn't a final
// response: net/http would still send a 200 after it.
func normalizeStatus(status string) (string, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		return "200", nil
	}
	code, err := strconv.Atoi(status)
	if err != nil || code < 200 || code > 599 {
		return "", errors.New("status must be a number between 200 and 599")
	}
	return strconv.Itoa(code), nil
}

func normalizeDelay(delay string) (string, error) {
	delay = strings.TrimSpace(delay)
	if delay == "" {
		return "0", nil
	}
	ms, err := strconv.Atoi(delay)
	if err != nil || ms < 0 {
		return "", errors.New("delay must be 0 or more milliseconds")
	}
	return strconv.Itoa(ms), nil
}

// normalizeJSONFile accepts an empty value: the server answers "{}" then.
func normalizeJSONFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("can't read %s", path)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}
	return path, nil
}

// commitFormStep validates the current field and stores its normalized value.
func (m *model) commitFormStep() error {
	var err error
	switch m.formStep {
	case formStepPath:
		m.formPath, err = keepOnError(m.formPath, normalizePath)
	case formStepMethod:
		m.formMethod, err = keepOnError(m.formMethod, normalizeMethod)
		if err == nil && m.hasMock(m.formMethod+" "+m.formPath) {
			// The server only ever serves the first match, so a second
			// mock for the same route would never answer.
			err = fmt.Errorf("%s %s already exists", m.formMethod, m.formPath)
		}
	case formStepStatus:
		m.formStatus, err = keepOnError(m.formStatus, normalizeStatus)
	case formStepDelay:
		m.formDelay, err = keepOnError(m.formDelay, normalizeDelay)
	case formStepJSONFile:
		m.formJSONFile, err = keepOnError(m.formJSONFile, normalizeJSONFile)
	}
	return err
}

// keepOnError leaves the user's text in the field when it's rejected, so
// they can fix it instead of retyping it.
func keepOnError(value string, normalize func(string) (string, error)) (string, error) {
	normalized, err := normalize(value)
	if err != nil {
		return value, err
	}
	return normalized, nil
}

func (m model) hasMock(title string) bool {
	for _, it := range m.list.Items() {
		if mi, ok := it.(mockItem); ok && mi.title == title {
			return true
		}
	}
	return false
}
