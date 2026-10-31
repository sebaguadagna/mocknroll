package tui

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/sebaguadagna/mocknroll/server"
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
	if err := checkPathParams(path); err != nil {
		return "", err
	}
	return path, nil
}

// checkPathParams only allows braces as whole "{name}" segments, each name once.
func checkPathParams(path string) error {
	seen := make(map[string]bool)
	for _, segment := range strings.Split(path, "/") {
		if !strings.ContainsAny(segment, "{}") {
			continue
		}
		name, ok := server.ParamName(segment)
		if !ok {
			return fmt.Errorf("invalid segment %s: a parameter is a whole segment, like {id}", segment)
		}
		if seen[name] {
			return fmt.Errorf("parameter {%s} is repeated", name)
		}
		seen[name] = true
	}
	return nil
}

// routeKey blanks parameter names, because /a/{x} and /a/{y} match the same
// requests. It takes a whole title, so the method is compared too.
func routeKey(title string) string {
	segments := strings.Split(title, "/")
	for i, segment := range segments {
		if _, ok := server.ParamName(segment); ok {
			segments[i] = "{}"
		}
	}
	return strings.Join(segments, "/")
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

// normalizeDelay accepts a fixed value ("200") or a range ("100-500").
func normalizeDelay(delay string) (string, error) {
	delay = strings.TrimSpace(delay)
	if delay == "" {
		return "0", nil
	}
	errInvalid := errors.New("delay must be milliseconds, like 200, or a range, like 100-500")

	minText, maxText, isRange := strings.Cut(delay, "-")
	minMs, err := strconv.Atoi(strings.TrimSpace(minText))
	if err != nil || minMs < 0 {
		return "", errInvalid
	}
	if !isRange {
		return strconv.Itoa(minMs), nil
	}
	maxMs, err := strconv.Atoi(strings.TrimSpace(maxText))
	if err != nil {
		return "", errInvalid
	}
	if minMs > maxMs {
		return "", errors.New("delay range must go from lowest to highest")
	}
	if minMs == maxMs {
		return strconv.Itoa(minMs), nil
	}
	return strconv.Itoa(minMs) + "-" + strconv.Itoa(maxMs), nil
}

func normalizeFailureRate(rate string) (string, error) {
	rate = strings.TrimSpace(rate)
	if rate == "" {
		return "0", nil
	}
	percent, err := strconv.Atoi(rate)
	if err != nil || percent < 0 || percent > 100 {
		return "", errors.New("failure rate must be a number between 0 and 100")
	}
	return strconv.Itoa(percent), nil
}

func normalizeFailureStatus(status string) (string, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		return "500", nil
	}
	code, err := strconv.Atoi(status)
	if err != nil || code < 400 || code > 599 {
		return "", errors.New("failure status must be a number between 400 and 599")
	}
	return strconv.Itoa(code), nil
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
	case formStepFailureRate:
		m.formFailureRate, err = keepOnError(m.formFailureRate, normalizeFailureRate)
	case formStepFailureStatus:
		m.formFailureStatus, err = keepOnError(m.formFailureStatus, normalizeFailureStatus)
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

// hasMock ignores the mock being edited, so it can keep its own route.
func (m model) hasMock(title string) bool {
	for i, it := range m.list.Items() {
		if m.editing && i == m.editIndex {
			continue
		}
		if mi, ok := it.(mockItem); ok && routeKey(mi.title) == routeKey(title) {
			return true
		}
	}
	return false
}
