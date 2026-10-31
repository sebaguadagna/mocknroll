# Spec: Failure Simulation and Variable Latency

- **Author:** sebaguadagna, with Claude Code
- **Date:** 2026-10-31
- **Status:** Approved
- **Reviewers:** sebaguadagna

## Context

mocknroll exists to reproduce how an app behaves against a slow backend before one exists. Today each mock always answers with the same status after exactly the same delay, which is more predictable than any real backend.

Retry logic, error states and loading indicators only get exercised when responses vary. To test them now, a user has to keep editing a mock by hand between requests. Per-mock latency ranges and a failure rate let those paths run on their own.

## Functional Requirements

### Variable latency

- FR-1: The delay field MUST accept a fixed value such as `200` or a range such as `100-500`.
- FR-2: For a range, the server MUST wait a random duration between both limits, inclusive, chosen per request.
- FR-3: The form MUST reject a range whose minimum exceeds its maximum, and MUST normalize a range with equal limits to a fixed value.
- FR-4: The details panel MUST show the configured range, colored by its maximum.

### Failures

- FR-5: The form MUST add a failure rate step after the delay step, accepting an integer from 0 to 100, with empty meaning 0.
- FR-6: The form MUST add a failure status step after the failure rate step, accepting 400 to 599, with empty meaning 500.
- FR-7: The form MUST skip the failure status step when the failure rate is 0, and MUST clear any stored failure status.
- FR-8: For each request, the server MUST decide independently whether it fails, with the configured probability.
- FR-9: A failing request MUST wait the configured delay and answer with the failure status and the JSON body `{"error": "simulated failure"}`, without reading the mock's JSON file.
- FR-10: The details panel MUST show the failure rate and status when the rate is above 0, and nothing about failures otherwise.
- FR-11: Log rows for simulated failures MUST include the text `fail`, and those requests MUST count in the mock's traffic.
- FR-12: The README MUST describe delay ranges and failure simulation.

## Non-Functional Requirements

- NFR-1: A `mocks.json` written by the previous version MUST load, and its mocks MUST behave as before, with a failure rate of 0.
- NFR-2: Mocks with no range and a failure rate of 0 MUST answer exactly as they do today.
- NFR-3: `go vet` and `go test -race ./...` MUST pass.
- NFR-4: The feature MUST NOT add dependencies.

## Acceptance Criteria

### AC-1: (FR-1)
Given the delay step, when the user submits `200` or `100-500`, then the form accepts it and stores that value.

### AC-2: (FR-2)
Given a mock with delay `20-60`, when it receives 20 requests, then every duration is at least 20 ms and the waits aren't all the same.

### AC-3: (FR-3)
Given the delay step, when the user submits `500-100`, then the form stays on that step and shows an error, and when they submit `100-100`, then it stores `100`.

### AC-4: (FR-4)
Given a mock with delay `100-500`, when the details render, then they show `100-500ms` in the style for a delay of 500 ms.

### AC-5: (FR-5)
Given the failure rate step, when the user submits `101`, `-1` or `abc`, then the form shows an error, and when they submit nothing, then it stores `0`.

### AC-6: (FR-6)
Given the failure status step, when the user submits `200` or `600`, then the form shows an error, and when they submit nothing, then it stores `500`.

### AC-7: (FR-7)
Given a failure rate of 0, when the user presses Enter, then the form moves to the JSON file step and the mock has no failure status.

### AC-8: (FR-8)
Given a mock with rate 0, when it receives 200 requests, then none fail, and given rate 100, then all fail.

### AC-9: (FR-8)
Given a mock with rate 50, when it receives 1000 requests, then between 400 and 600 fail.

### AC-10: (FR-9)
Given a mock with rate 100, status 503, a 50 ms delay and a JSON file, when it's requested, then it takes at least 50 ms and answers 503 with the failure body.

### AC-11: (FR-10)
Given rates 20 and 0, when the details render, then only the mock with rate 20 shows its rate and status.

### AC-12: (FR-11)
Given a simulated failure and a mock configured to answer 500, when the log renders, then only the simulated failure's row contains `fail`.

### AC-13: (FR-11)
Given a mock with rate 100, when it receives a request, then its traffic count increases by one.

### AC-14: (FR-12)
Given the finished feature, when the README is read, then it explains delay ranges and failure simulation.

### AC-15: (NFR-1, NFR-2)
Given a `mocks.json` with no failure fields, when the app loads it, then its mocks have rate 0 and answer with their configured status and body.

### AC-16: (NFR-3, NFR-4)
Given the finished feature, when CI runs, then vet and race tests pass and `go.mod` has no new requirements.

## Edge Cases

- EC-1: Editing a mock MUST prefill its delay range, failure rate and failure status.
- EC-2: A hand-edited invalid delay, rate or status in `mocks.json` MUST NOT panic: the server MUST treat an invalid delay or rate as 0 and an invalid failure status as 500.
- EC-3: A rate above 100 in `mocks.json` MUST behave as 100, and a negative one as 0.
- EC-4: A mock with an unreadable JSON file and rate 100 MUST answer the failure status, not the 500 for the unreadable file.
- EC-5: A delay with spaces around its limits, such as `100 - 500`, MUST be normalized to `100-500`.
- EC-6: A negative limit or a malformed range, such as `-5`, `100-` or `1-2-3`, MUST be rejected by the form.
- EC-7: An edit that sets the rate to 0 MUST clear the failure status, and later requests MUST succeed.

## API Contracts

No HTTP routes change. `server.Mock` and `server.RequestLogEntry` gain fields, shown here in TypeScript style:

```ts
interface Mock {
  // Existing fields stay. DelayMs is the minimum when a range is set.
  DelayMaxMs: number;     // equal to or below DelayMs means a fixed delay
  FailureRate: number;    // 0 to 100
  FailureStatus: number;  // 400 to 599
}

interface RequestLogEntry {
  // Existing fields stay.
  Simulated: boolean;     // true when the failure was simulated
}
```

## Data Models

`mocks.json` gains two optional fields, and `delay` accepts ranges:

| Field | Type | Constraints |
|---|---|---|
| `delay` | string | `200` or `100-500` |
| `failure_rate` | string | Optional, `0` to `100`; absent means 0 |
| `failure_status` | string | Optional, `400` to `599`; absent when the rate is 0 |

## Out of Scope

- OS-1: Network-level failures such as dropped connections or requests that never answer. They need different handling in the server, and deserve their own spec.
- OS-2: Custom bodies for simulated failures. A file per failure would add another form step.
- OS-3: Several failure statuses per mock.
- OS-4: Non-uniform latency, such as occasional spikes. A uniform range covers the common case.
- OS-5: Scheduled failures, such as failing every third request or for a time window.
- OS-6: A seed for reproducible randomness.
