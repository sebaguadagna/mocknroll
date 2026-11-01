# Spec: Network Failures

- **Author:** sebaguadagna, with Claude Code
- **Date:** 2026-10-31
- **Status:** Approved
- **Reviewers:** sebaguadagna

## Context

A mock can already fail a share of its requests, but only with an HTTP status. A client that handles a `500` correctly can still break when the connection drops or the server never answers, because those cases reach it through different code: connection errors, timeouts and retries. `specs/failure-simulation.md` left this out on purpose (OS-1), as it needs different handling in the server.

Clients talk to the mocknroll server directly, so it can cause these failures itself, with no proxy in between. Each mock gains a failure type, and its failure rate keeps deciding which requests fail.

## Functional Requirements

### Failure types

- FR-1: A mock with a failure rate above 0 MUST have one failure type: `http`, `reset` or `hang`.
- FR-2: `http` MUST behave as simulated failures do today, answering the failure status and the fixed body.
- FR-3: `reset` MUST abort the TCP connection with an RST and send no HTTP response bytes.
- FR-4: `hang` MUST send no HTTP response bytes and keep the connection open until the client closes it.
- FR-5: A `hang` request MUST release its goroutine when the client disconnects, including a request whose body wasn't read yet.
- FR-6: The configured delay MUST pass before a `reset` or `hang` takes effect, as it does for `http`.
- FR-7: Requests with a network failure MUST count in the mock's traffic.

### Form

- FR-8: When the failure rate is above 0, the form MUST ask for the failure type, accepting `http`, `reset` or `hang` in any case. An empty value MUST mean `http`.
- FR-9: The form MUST reject any other failure type, showing an error and staying on that step.
- FR-10: The form MUST ask for the failure status only when the type is `http`.
- FR-11: When the rate is 0, the form MUST skip both steps and store neither value.
- FR-12: Editing a mock MUST prefill its failure type.

### Display

- FR-13: The detail panel MUST show the type for network failures, as in `Failures: 10% with reset`, and keep showing the status for `http`.
- FR-14: The request log MUST show a network failure with `---` as its status, and `reset` or `hang` in place of `fail`.
- FR-15: A `reset` MUST be logged when the connection is aborted, and a `hang` when the client disconnects, with the time it waited as its duration.

### Documentation

- FR-16: The README MUST describe the failure types, and warn that a client with no timeout waits on a `hang` until it's stopped.

## Non-Functional Requirements

- NFR-1: A `mocks.json` from the previous version MUST load, and its mocks MUST behave as before, with type `http`.
- NFR-2: A mock with type `http` or no failures MUST NOT change its responses.
- NFR-3: 100 finished `hang` requests MUST leave no goroutines behind, within 2 seconds of their clients disconnecting.
- NFR-4: `go.mod` MUST have no new requirements, and `go vet` and `go test -race ./...` MUST pass.

## Acceptance Criteria

### AC-1: Reset (FR-1, FR-3)
Given a mock with rate 100 and type `reset`, when a client requests it, then the client gets a connection error, no status or body, and a raw TCP client reads a reset rather than a clean close.

### AC-2: Hang (FR-1, FR-4)
Given a mock with rate 100 and type `hang`, when a client with a 200ms timeout requests it, then the client times out with no response bytes.

### AC-3: Hang ends with the client (FR-5, FR-15, NFR-3)
Given 100 `hang` requests in flight, including `POST` requests with an unread body, when their clients disconnect, then the goroutine count returns to its earlier level within 2 seconds, and each request is logged with type `hang`.

### AC-4: Delay (FR-6)
Given a mock with delay 200 and type `reset`, when it's requested, then the connection breaks after at least 200ms.

### AC-5: Traffic (FR-7)
Given a mock with type `reset` or `hang`, when it's requested three times and the clients have finished, then its count rose by three.

### AC-6: HTTP type unchanged (FR-2, NFR-2)
Given a mock with type `http` and status 503, when it fails, then it answers 503 with `{"error": "simulated failure"}`.

### AC-7: Rate (FR-1)
Given a mock with rate 0 and type `reset`, when it's requested, then it answers normally.

### AC-8: Form steps (FR-8, FR-10, FR-11)
Given the form, when the rate is 50 and the type is `reset`, then the status step is skipped. When the type is empty or `HTTP`, the status step appears and the type is stored as `http`. When the rate is 0, neither step appears and neither value is stored.

### AC-9: Form validation (FR-9)
Given the failure type step, when the user enters `timeout` and presses Enter, then an error appears and the step doesn't change.

### AC-10: Editing (FR-12)
Given a mock with type `hang`, when it's edited, then the type step starts with `hang`, and saving keeps it.

### AC-11: Details (FR-13)
Given mocks with rate 10 and types `reset` and `http` with status 503, when their details render, then they show `Failures: 10% with reset` and `Failures: 10% with 503`.

### AC-12: Request log (FR-14, FR-15)
Given a finished `reset` request and a `hang` request whose client gave up after 300ms, when the log renders, then each row shows `---` and its type, and the `hang` shows a duration of at least 300ms.

### AC-13: Documentation (FR-16)
Given the README, when it's read, then it describes the three types and the `hang` warning.

### AC-14: Compatibility (NFR-1, NFR-4)
Given a `mocks.json` with no failure type, when it loads, then its failing mocks answer their status as before. CI passes vet and race tests, and `go.mod` has no new requirements.

## Edge Cases

- EC-1: A hand-edited unknown failure type in `mocks.json` MUST behave as `http`, with no panic.
- EC-2: If the connection can't be taken over for a `reset`, the server MUST close it by aborting the handler, so the client still gets no response.
- EC-3: A client that disconnects during the delay MUST NOT cause a panic. The request is logged when its failure takes effect.
- EC-4: Changing or deleting a mock while a `hang` is in flight MUST NOT end that request or cause a panic.
- EC-5: A `hang` in flight when the app quits ends with the process, and it isn't logged.
- EC-6: A `reset` or `hang` on a `HEAD` request MUST behave as on any other method.
- EC-7: A mock with parameters MUST fail the same way, matching its route first.
- EC-8: An unmatched request MUST still answer 404, since failures only apply to a mock that matched.
- EC-9: Docker Desktop's published ports turn a `reset` into a clean close, which the app can't prevent. A client on the host sees an empty reply, and the README MUST say so.

## API Contracts

No HTTP routes change. The Go types gain fields, shown here in TypeScript style:

```ts
interface Mock {
  // Existing fields stay.
  FailureType: "" | "http" | "reset" | "hang"; // "" and unknown values mean "http"
}

interface RequestLogEntry {
  // Existing fields stay. Simulated is true for every failure type.
  Status: number;      // 0 for "reset" and "hang", which send no response
  FailureType: string; // "reset" or "hang"; "" for any other request
}
```

## Data Models

`mocks.json` gains one optional field:

| Field | Type | Constraints |
|---|---|---|
| `failure_type` | string | Optional, `http`, `reset` or `hang`; absent means `http` |
| `failure_status` | string | Optional, `400` to `599`; absent unless the type is `http` |

## Out of Scope

- OS-1: A bridge or proxy in front of a real backend. It's a different product feature, and clients already reach the mock server directly.
- OS-2: Responses cut partway through the body. It's easy to add as a fourth type later.
- OS-3: Several failure types on one mock, each with its own rate. It would complicate the form, and one type covers most tests.
- OS-4: Network failures across the whole server, such as a "everything is down" switch. It deserves its own spec.
- OS-5: Slow responses that send bytes gradually. The delay already covers slow servers.
- OS-6: A maximum duration for `hang`. A hang that ends on its own wouldn't test a client's timeout.
