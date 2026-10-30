package main

import (
	"github.com/sebaguadagna/mocknroll/server"
	"github.com/sebaguadagna/mocknroll/tui"
)

func main() {
	// Start mock HTTP server in the background on port 8080
	_ = server.Start(8080)

	tui.Start()
}
