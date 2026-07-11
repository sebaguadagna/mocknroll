# mocknroll

> Configure mock HTTP endpoints — with realistic response delays — without leaving your terminal.

[![CI](https://github.com/sebaguadagna/mocknroll/actions/workflows/ci.yml/badge.svg)](https://github.com/sebaguadagna/mocknroll/actions/workflows/ci.yml)
[![Go Reference](https://img.shields.io/badge/go-1.24-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`mocknroll` is a terminal UI, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), for defining mock API endpoints: method, path, status code, response body, and — the whole point of the project — an artificial delay, so you can reproduce how your app behaves against a slow backend before you ever have one.

## Preview

```
┌────────────────────────────────────────────────────────────────────┐  ╔══════════════════════════════════════╗
│                                                                    │  ║                                      ║
│     Mocks loaded                                                   │  ║  Details                             ║
│                                                                    │  ║                                      ║
│    2 items                                                         │  ║  GET /api/v1/users                   ║
│                                                                    │  ║  Returns users list                  ║
│  │ GET /api/v1/users                                               │  ║                                      ║
│  │ Returns users list                                              │  ║  ● Enabled   Responds in 30ms        ║
│                                                                    │  ║  Status:     200                     ║
│    POST /api/v1/orders                                             │  ║  JSON File:  examples/users.json     ║
│    Creates an order                                                │  ║                                      ║
│                                                                    │  ║  Response preview:                   ║
│                                                                    │  ║  [                                   ║
│                                                                    │  ║    { "id": 1, "name": "Ada           ║
│                                                                    │  ║  Lovelace", "email":                 ║
│                                                                    │  ║  "ada@example.com" },                ║
│                                                                    │  ║    { "id": 2, "name": "Alan          ║
│                                                                    │  ║  Turing", "email":                   ║
│                                                                    │  ║  "alan@example.com" }                ║
│                                                                    │  ║  ]                                   ║
│                                                                    │  ║                                      ║
│    ↑/k up • ↓/j down • / filter • a add mock • t toggle enabled …  │  ║                                      ║
└────────────────────────────────────────────────────────────────────┘  ╚══════════════════════════════════════╝
```

## Features

- **Browse and filter** your configured mocks in a searchable list.
- **Add a mock** through a guided, step-by-step form (path → method → status → delay → response file).
- **Toggle a mock on/off** (`t`) without deleting it.
- **Delay severity at a glance** — the configured delay is color-coded: green (≤ 30ms), orange (31–150ms), red (> 150ms).
- **Response preview** — see the first lines of the JSON file a mock will respond with, right in the detail panel.
- **Exit confirmation** so a stray `q` doesn't cost you your in-progress setup.

## Status

The TUI for defining and inspecting mocks is functional. Actually **serving** those mocks over HTTP (respecting the configured status, delay, and response body) is the next milestone — see `server/server.go`.

## Installation

There are no tagged releases yet, so build from source:

```sh
git clone https://github.com/sebaguadagna/mocknroll.git
cd mocknroll
make build   # -> bin/mocknroll
```

Requires Go 1.24+ (see `go.mod`).

## Usage

```sh
make run
```

| Key         | Action                              |
|-------------|--------------------------------------|
| `↑/k` `↓/j` | Move through the mock list           |
| `/`         | Filter mocks                         |
| `a`         | Add a new mock                       |
| `t`         | Toggle the selected mock enabled/disabled |
| `q` / `Esc` | Quit (asks for confirmation)         |
| `?`         | Toggle full keybinding help          |

While filling out the "add mock" form: `Enter` advances to the next field (or saves on the last one), `Esc` cancels and discards the form.

## Development

```sh
make build   # build to bin/mocknroll
make run     # go run .
make test    # go test -v ./...
make fmt     # go fmt ./...
make vet     # go vet ./...
make deps    # go mod tidy && go mod vendor
```

See [`CLAUDE.md`](CLAUDE.md) for a closer look at the project's architecture.

## Contributing

Issues and pull requests are welcome — this is a small, actively evolving project, so it's a good time to shape where it goes.

## License

[MIT](LICENSE) © Sebastián
