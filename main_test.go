package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/sebaguadagna/mocknroll/server"
)

func TestParsePortValid(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"default", nil, 8080},
		{"double dash", []string{"--port", "9000"}, 9000},
		{"double dash equals", []string{"--port=9000"}, 9000},
		{"single dash", []string{"-port", "9000"}, 9000},
		{"single dash equals", []string{"-port=9000"}, 9000},
		{"lowest", []string{"--port", "1"}, 1},
		{"highest", []string{"--port", "65535"}, 65535},
		{"repeated uses the last", []string{"--port", "9000", "--port", "9001"}, 9001},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePort(tt.args, io.Discard)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got port %d, want %d", got, tt.want)
			}
		})
	}
}

// fakeStarts records which start functions run, and with which port.
type fakeStarts struct {
	serverErr, tuiErr     error
	serverPort, tuiPort   int
	serverCalls, tuiCalls int
}

func (f *fakeStarts) server(port int) error {
	f.serverCalls++
	f.serverPort = port
	return f.serverErr
}

func (f *fakeStarts) tui(port int) error {
	f.tuiCalls++
	f.tuiPort = port
	return f.tuiErr
}

func TestRunRejectsInvalidUsage(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"zero", []string{"--port", "0"}, "invalid port 0"},
		{"too high", []string{"--port", "65536"}, "invalid port 65536"},
		{"negative", []string{"--port", "-1"}, "invalid port -1"},
		{"not a number", []string{"--port", "abc"}, "invalid value"},
		{"empty", []string{"--port="}, "invalid value"},
		{"no value", []string{"--port"}, "needs an argument"},
		{"overflow", []string{"--port", "99999999999999999999"}, "invalid value"},
		{"unknown flag", []string{"--nope"}, "not defined"},
		{"positional", []string{"9000"}, `unexpected argument "9000"`},
		{"flag and positional", []string{"--port", "9000", "extra"}, `unexpected argument "extra"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			var f fakeStarts
			if got := run(tt.args, &stderr, f.server, f.tui); got != 2 {
				t.Errorf("got status %d, want 2", got)
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr %q doesn't contain %q", stderr.String(), tt.wantErr)
			}
			if !strings.Contains(stderr.String(), "Usage of mocknroll") {
				t.Errorf("stderr doesn't show the usage: %q", stderr.String())
			}
			if f.serverCalls+f.tuiCalls != 0 {
				t.Errorf("started server %d times and TUI %d times, want none", f.serverCalls, f.tuiCalls)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var stderr bytes.Buffer
			var f fakeStarts
			if got := run([]string{arg}, &stderr, f.server, f.tui); got != 0 {
				t.Errorf("got status %d, want 0", got)
			}
			for _, want := range []string{"port", "default 8080"} {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("usage %q doesn't contain %q", stderr.String(), want)
				}
			}
			if f.serverCalls+f.tuiCalls != 0 {
				t.Errorf("started server %d times and TUI %d times, want none", f.serverCalls, f.tuiCalls)
			}
		})
	}
}

func TestRunPassesThePortToServerAndTUI(t *testing.T) {
	var f fakeStarts
	if got := run([]string{"--port", "18080"}, io.Discard, f.server, f.tui); got != 0 {
		t.Fatalf("got status %d, want 0", got)
	}
	if f.serverPort != 18080 || f.tuiPort != 18080 {
		t.Errorf("server got port %d and TUI got %d, want 18080", f.serverPort, f.tuiPort)
	}
}

func TestRunServerFailure(t *testing.T) {
	var stderr bytes.Buffer
	f := fakeStarts{serverErr: errors.New("permission denied")}
	if got := run([]string{"--port", "80"}, &stderr, f.server, f.tui); got != 1 {
		t.Errorf("got status %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "port 80:") {
		t.Errorf("stderr doesn't name the port: %q", stderr.String())
	}
	if f.tuiCalls != 0 {
		t.Error("the TUI started after the server failed")
	}
}

func TestRunPortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	var stderr bytes.Buffer
	var f fakeStarts
	if got := run([]string{"--port", strconv.Itoa(port)}, &stderr, server.Start, f.tui); got != 1 {
		t.Errorf("got status %d, want 1", got)
	}
	if want := fmt.Sprintf("port %d:", port); !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr %q doesn't contain %q", stderr.String(), want)
	}
	if f.tuiCalls != 0 {
		t.Error("the TUI started after the server failed")
	}
}

func TestRunTUIFailure(t *testing.T) {
	var stderr bytes.Buffer
	f := fakeStarts{tuiErr: errors.New("boom")}
	if got := run(nil, &stderr, f.server, f.tui); got != 1 {
		t.Errorf("got status %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Errorf("stderr doesn't show the error: %q", stderr.String())
	}
}

func TestParsePortHelpReturnsErrHelp(t *testing.T) {
	if _, err := parsePort([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("got error %v, want flag.ErrHelp", err)
	}
}
