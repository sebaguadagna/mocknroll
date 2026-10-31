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
- **Add a mock** through a guided, step-by-step form (path → method → status → delay → response file) that validates each field as you go.
- **Edit or delete a mock** without touching `mocks.json` by hand.
- **Live request log** (`r`) — method, path, status, and duration of recent requests, including those no mock matched.
- **Simulate an unreliable backend** — random latency within a range and a per-mock failure rate.
- **Path parameters** — one mock for `/users/{id}`, with the captured value available in the response.
- **Toggle a mock on/off** (`t`) without deleting it.
- **Delay severity at a glance** — the configured delay is color-coded: green (≤ 30ms), orange (31–150ms), red (> 150ms).
- **Response preview** — see the first lines of the JSON file a mock will respond with, right in the detail panel.
- **Exit confirmation** so a stray `q` doesn't cost you your in-progress setup.

## Status

Mocks are served over HTTP on port `8080`, respecting the configured status, delay, and response body, and persisted to `mocks.json` in the working directory.

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
| `e`         | Edit the selected mock               |
| `d`         | Delete the selected mock (asks for confirmation) |
| `t`         | Toggle the selected mock enabled/disabled |
| `r`         | Switch the right panel between details and the request log |
| `q` / `Esc` | Quit (asks for confirmation)         |
| `?`         | Toggle full keybinding help          |

While filling out the add or edit form: `Enter` advances to the next field (or saves on the last one), `Esc` cancels and discards the form.

### Simulating an unreliable backend

The form's delay field takes a fixed value (`200`) or a range (`100-500`); with a range, each request waits a random time within it.

The failure rate is the percentage of requests that fail, decided independently for each one. A failing request still waits the configured delay, then answers the failure status (400–599, `500` by default) with `{"error": "simulated failure"}`. Those requests are marked `fail` in the request log. A rate of `0` turns this off.

### Path parameters

A path segment written as `{name}` matches any single segment, so `GET /users/{id}` answers `/users/42` and `/users/abc`, but not `/users` or `/users/42/orders`. A parameter must be a whole segment, so `/files/{id}.json` isn't valid.

When several mocks match a request, the one with the most literal segments answers, whatever their order in the list: `GET /users/me` wins over `GET /users/{id}`. On a tie, the first in the list answers.

In a mock with parameters, each `{name}` in the response file is replaced with the value from the request. With a file holding `{"id": "{id}"}`, `/users/42` answers `{"id": "42"}`. Values are JSON-escaped, and the file itself isn't changed.

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
