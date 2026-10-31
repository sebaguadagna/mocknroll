package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sebaguadagna/mocknroll/server"
	"github.com/sebaguadagna/mocknroll/tui"
)

const defaultPort = 8080

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, server.Start, tui.Start))
}

// run returns the process exit status. The start functions are parameters so
// tests can check they aren't called, without opening a real TUI.
func run(args []string, stderr io.Writer, startServer, startTUI func(port int) error) int {
	port, err := parsePort(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2 // parsePort already printed the error and usage
	}

	// Start mock HTTP server in the background
	if err := startServer(port); err != nil {
		fmt.Fprintf(stderr, "mocknroll: can't start mock server on port %d: %v\n", port, err)
		return 1
	}

	if err := startTUI(port); err != nil {
		fmt.Fprintf(stderr, "mocknroll: %v\n", err)
		return 1
	}
	return 0
}

// parsePort reads the command line, printing any error and the usage to stderr.
// It returns flag.ErrHelp when help was requested.
func parsePort(args []string, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("mocknroll", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", defaultPort, "port the mock server listens on (1-65535)")

	if err := fs.Parse(args); err != nil {
		return 0, err // fs already printed the error and usage
	}

	var err error
	switch {
	case fs.NArg() > 0:
		err = fmt.Errorf("unexpected argument %q", fs.Arg(0))
	case *port < 1 || *port > 65535:
		err = fmt.Errorf("invalid port %d: must be from 1 to 65535", *port)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		fs.Usage()
		return 0, err
	}
	return *port, nil
}
