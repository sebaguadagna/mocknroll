package main

import (
	"fmt"
	"os"

	"github.com/sebaguadagna/mocknroll/server"
	"github.com/sebaguadagna/mocknroll/tui"
)

const port = 8080

func main() {
	// Start mock HTTP server in the background
	if err := server.Start(port); err != nil {
		fmt.Fprintf(os.Stderr, "mocknroll: can't start mock server on port %d: %v\n", port, err)
		os.Exit(1)
	}

	if err := tui.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "mocknroll: %v\n", err)
		os.Exit(1)
	}
}
