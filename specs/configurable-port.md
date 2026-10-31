# Spec: Configurable Port

- **Author:** sebaguadagna, with Claude Code
- **Date:** 2026-10-31
- **Status:** Approved
- **Reviewers:** sebaguadagna

## Context

The mock server always listens on port 8080, set by a constant in `main.go`. 8080 is one of the most common development ports, so mocknroll often can't start next to the backend or proxy a user is already running. When that happens, the only option is to stop the other process or edit the source and rebuild.

It also blocks two mocknroll instances from running side by side, for example to fake two services, since each one needs its own port. The next roadmap item is a first release with binaries, and users of a binary can't edit the constant at all.

The TUI's title also hardcodes `http://localhost:8080`, so it must follow the real port or it would point users at the wrong address.

## Functional Requirements

### Flag

- FR-1: The program MUST accept a `--port <n>` flag, also written `--port=<n>`, `-port <n>` or `-port=<n>`, that sets the port the mock server listens on.
- FR-2: Without the flag, the server MUST listen on port 8080.
- FR-3: The port MUST be an integer from 1 to 65535.
- FR-4: The program MUST reject an invalid port, an unknown flag, or a positional argument before starting the server or the TUI, printing the error and usage to stderr and exiting with status 2.
- FR-5: `--help` or `-h` MUST print the usage, including the flag and its default, and exit with status 0 without starting the server or the TUI.

### Startup and display

- FR-6: If the server can't listen on the chosen port, the program MUST exit with status 1 and an error on stderr naming that port, without starting the TUI.
- FR-7: The list title MUST show the port the server is listening on, as `http://localhost:<port>`.

### Documentation

- FR-8: The README MUST document the flag and its default, and `CLAUDE.md` MUST stop describing the port as fixed.

## Non-Functional Requirements

- NFR-1: Running without flags MUST behave as it does today, on port 8080 and with the same title.
- NFR-2: The format of `mocks.json` MUST NOT change, and the port MUST NOT be persisted.
- NFR-3: The feature MUST NOT add dependencies, so `go.mod` has no new requirements. It uses the standard `flag` package.
- NFR-4: `go vet` and `go test -race ./...` MUST pass.

## Acceptance Criteria

### AC-1: Custom port (FR-1)
Given no flag parsing error, when the arguments are each of `--port 9000`, `--port=9000`, `-port 9000` and `-port=9000`, then the parsed port is 9000.

### AC-2: Server listens on the chosen port (FR-1)
Given the app started with `--port 18080`, when a client requests a configured mock on port 18080, then the mock answers, and port 8080 isn't opened by the app.

### AC-3: Default port (FR-2, NFR-1)
Given no arguments, when they're parsed, then the port is 8080 and the title shows `http://localhost:8080`.

### AC-4: Port limits (FR-3)
Given the arguments `--port 1` and `--port 65535`, when they're parsed, then each is accepted.

### AC-5: Invalid values (FR-3, FR-4)
Given a port of `0`, `65536`, `-1`, `abc` or an empty value, when the arguments are parsed, then an error is returned, and running the program prints the error and usage to stderr and exits with status 2.

### AC-6: Unknown flags and positional arguments (FR-4)
Given the arguments `--nope` or `9000`, when they're parsed, then an error is returned and the program exits with status 2 without starting the server or the TUI.

### AC-7: Help (FR-5)
Given `--help` or `-h`, when the program runs, then the usage mentions `port` and its default of 8080, the exit status is 0, and neither the server nor the TUI starts.

### AC-8: Port in use (FR-6)
Given a port that another process is listening on, when the app starts with `--port` set to it, then it exits with status 1, stderr names that port, and the TUI doesn't start.

### AC-9: Title follows the port (FR-7)
Given the TUI started on port 18080, when the list renders, then its title shows `http://localhost:18080` and doesn't show `8080` as a separate port.

### AC-10: Documentation (FR-8)
Given the finished feature, when the docs are read, then the README describes `--port` and its default, and `CLAUDE.md` doesn't state a fixed port.

### AC-11: No side effects (NFR-2, NFR-3, NFR-4)
Given the finished feature, when CI runs, then vet and race tests pass, `go.mod` has no new requirements, and `mocks.json` holds no port after running with `--port`.

## Edge Cases

- EC-1: A port the user has no permission to open, such as 80 without privileges, MUST follow FR-6: status 1 and an error naming the port.
- EC-2: A repeated flag, as in `--port 9000 --port 9001`, MUST use the last value, as the `flag` package does.
- EC-3: A value beyond the integer range, such as `99999999999999999999`, MUST be rejected as in FR-4, with no panic.
- EC-4: A valid flag followed by a positional argument, as in `--port 9000 extra`, MUST be rejected as in FR-4.
- EC-5: Two instances started in the same directory on different ports MUST both start. They share `mocks.json`, so the last one to write wins. This is existing file behavior, and this feature doesn't change it.

## API Contracts

No HTTP routes change. The command line and one exported Go signature do:

```ts
// Command line
// mocknroll [--port <1-65535>]
// Exit status: 0 on success or help, 1 on a startup or TUI error, 2 on invalid usage.

// Go, shown in TypeScript style
function Start(port: number): error; // tui.Start, previously took no arguments
```

`server.Start(port)` already takes the port and doesn't change.

## Data Models

N/A — the port only exists for the process's lifetime, and `mocks.json` is unchanged.

| Field | Type | Constraints |
|---|---|---|
| `port` (runtime only) | int | 1–65535, default 8080 |

## Out of Scope

- OS-1: A short `-p` alias. It doubles the usage output, and can be added without breaking anything.
- OS-2: Setting the port through an environment variable or config file. A flag covers the reported need.
- OS-3: Persisting the port per project in `mocks.json`. It would change the file format.
- OS-4: Choosing a bind address. The server keeps listening on all interfaces.
- OS-5: Port `0` for a random free port. It isn't needed yet, and a client has to know the port in advance.
- OS-6: Changing the port from the TUI while running. It would mean restarting the listener.
- OS-7: Separating `mocks.json` per instance, such as with a `--config` flag. It deserves its own spec.
