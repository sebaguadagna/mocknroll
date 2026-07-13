# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make build` — build binary to `bin/mocknroll`
- `make run` — `go run .` (launches the TUI directly)
- `make test` — `go test -v ./...` (no test files exist yet)
- `make deps` — `go mod tidy && go mod vendor` (this repo vendors dependencies; run after adding/changing imports)
- `make fmt` / `make vet` — `go fmt ./...` / `go vet ./...`
- `make clean` — remove `bin/`

CI (`.github/workflows/ci.yml`) runs `go build ./...` and `go test ./...` on push/PR to `main`.

Note: `package.json` / `package-lock.json` at the repo root are stray/empty and unrelated to this Go module — there is no JS toolchain here.

## Architecture

Terminal UI built on the Bubble Tea Elm architecture (bubbletea/bubbles/lipgloss v2, imported as `charm.land/{bubbletea,lipgloss,bubbles}/v2` — the v2 line moved its canonical module path off `github.com/charmbracelet/...`). Entry point `main.go` calls `tui.Start()` (`tui/start.go`), which runs `tea.NewProgram(initialModel())`. Alt-screen mode is set per-frame via `tea.View.AltScreen = true` inside `View()` (`tui/view.go`), not as a `tea.NewProgram` option.

The `tui` package is split by Elm-architecture role:
- `model.go` — `model` struct (app state) and `mockItem` (a configured mock: title/description/status/delay/jsonFile), plus `initialModel()`.
- `update.go` — `Update()`, dispatching on `m.currentMode`.
- `view.go` — `View()`, rendering per mode.

State machine (`m.currentMode`, defined in `model.go`):
- `listMode` — browse the list of configured mocks (left pane) with detail view (right pane, `view.go`). `a` → `formMode`; `q`/`ctrl+c` → `confirmExitMode`.
- `formMode` — multi-step form to add a new mock, one field per key press cycling through `formStepPath → formStepMethod → formStepStatus → formStepDelay → formStepJSONFile` (constants in `update.go`); `Enter` advances/submits, `Esc` cancels back to `listMode`. On submit, builds a `mockItem` and inserts it into `m.list`.
- `confirmExitMode` — `y`/`Y` quits, `n`/`N`/`Esc` returns to `listMode`.

`server/server.go` is currently an empty stub (`package server`, no code) — the intended location for actually serving the mocks configured via the TUI over HTTP; not implemented yet.

`modelview` (plain text, no extension, in Spanish) is an existing ASCII diagram of the Start()→View() flow — useful as a quick visual reference for the mode-based rendering logic.
