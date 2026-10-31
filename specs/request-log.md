# Spec: Live Request Log

- **Author:** sebaguadagna, with Claude Code
- **Date:** 2026-10-31
- **Status:** Approved
- **Reviewers:** sebaguadagna

## Context

mocknroll shows how many requests each mock received, but not what those requests were. When a client gets an unexpected response, the user can't tell from the TUI whether the request reached the server, which route it asked for, or which status it got.

Requests that match no mock are the hardest case: the client receives a 404 and the TUI shows nothing at all, because the sparklines only count matched requests. A log of recent requests answers these questions without leaving the terminal and makes the configured delays visible.

## Functional Requirements

### Recording

- FR-1: The server MUST record every completed request with its time, method, path, status, duration and whether it matched a mock.
- FR-2: The recorded status MUST be the one actually sent, including 404 for unmatched requests and 500 for an unreadable JSON file.
- FR-3: The recorded duration MUST include the mock's configured delay.
- FR-4: The server MUST keep only the 200 most recent entries, discarding the oldest first.

### Display

- FR-5: Pressing `r` in `listMode` MUST switch the right panel between the mock details and the request log.
- FR-6: The log MUST show entries newest first, as many as fit in the panel.
- FR-7: Each row MUST show the time as `HH:MM:SS`, method, path and status. Rows for matched requests MUST also show the duration in milliseconds.
- FR-8: Rows for requests that matched no mock MUST show the text `no mock` in place of the duration, leaving more room for the path.
- FR-9: The status MUST be colored by class: green for 2xx, the default color for 3xx, orange for 4xx and red for 5xx.
- FR-10: The log MUST refresh at least once per second while it's visible, in every mode that shows the panel.
- FR-11: With no entries, the panel MUST show `No requests yet`.
- FR-12: While the user is typing in the filter, `r` MUST be entered as filter text.
- FR-13: The list's hint line and the README keybinding table MUST include `r`.

## Non-Functional Requirements

- NFR-1: Recording MUST NOT change the status, headers or body of any response.
- NFR-2: Memory for the log MUST stay bounded at 200 entries under any request volume.
- NFR-3: `go vet` and `go test -race ./...` MUST pass, including with concurrent requests.
- NFR-4: The feature MUST NOT add dependencies or change the format of `mocks.json`.

## Acceptance Criteria

### AC-1: (FR-1)
Given a mock `GET /a`, when a client requests it, then the log has one entry with its time, method `GET`, path `/a`, status, duration and matched set to true.

### AC-2: (FR-2)
Given no matching mock, when a client requests `/nope`, then the entry has status 404 and matched set to false.

### AC-3: (FR-2)
Given a mock whose JSON file can't be read, when a client requests it, then the entry has status 500 and matched set to true.

### AC-4: (FR-3)
Given a mock with a 50 ms delay, when a client requests it, then the entry's duration is at least 50 ms.

### AC-5: (FR-4, NFR-2)
Given 201 completed requests, when the log is read, then it has 200 entries and the first request is gone.

### AC-6: (FR-5)
Given the details panel, when the user presses `r`, then the panel shows the log, and when they press `r` again, then it shows the details.

### AC-7: (FR-6)
Given requests to `/a` and then `/b`, when the log renders, then `/b` appears above `/a`.

### AC-8: (FR-7)
Given a matched entry, when its row renders, then it contains the time as `HH:MM:SS`, method, path, status and duration ending in `ms`.

### AC-9: (FR-8)
Given an unmatched entry and a matched entry that also returned 404, when they render, then only the unmatched row contains `no mock`, and it shows no duration.

### AC-10: (FR-9)
Given statuses 200, 301, 404 and 500, when the status color is chosen, then they get the green, default, orange and red styles.

### AC-11: (FR-10)
Given the log is visible, when a request completes and a traffic tick fires, then the new entry is shown.

### AC-12: (FR-11)
Given an empty log, when the panel renders, then it shows `No requests yet`.

### AC-13: (FR-12)
Given the user is typing in the filter, when they press `r`, then the panel doesn't change and `r` is added to the filter.

### AC-14: (FR-13)
Given `listMode`, when the view renders, then the hint line includes the `r` key.

### AC-15: (NFR-1)
Given a mock with status 201 and a JSON file, when a client requests it, then the response's status, content type and body are the same as without recording.

### AC-16: (NFR-3, NFR-4)
Given 50 concurrent requests under the race detector, when they finish, then the log has 50 entries, no race is reported and `go.mod` has no new requirements.

## Edge Cases

- EC-1: A path containing control characters, such as a URL-encoded escape sequence (`%1b`), MUST be displayed with those characters removed, so a client can't alter the terminal.
- EC-2: An arbitrary HTTP method MUST be sanitized the same way as a path and logged as received.
- EC-3: A row wider than the panel MUST be truncated to one line, so it can't break the layout.
- EC-4: A request still in progress MUST NOT appear until it completes.
- EC-5: A client that disconnects during a delay MUST still produce an entry once the handler ends.
- EC-6: Deleting or editing a mock MUST NOT remove its existing log entries.
- EC-7: The panel MUST keep its selected view while a popup or a list filter is active, and on an empty mock list.
- EC-8: A query string MUST NOT be recorded, since only the path is logged.

## API Contracts

No HTTP routes change. The server package gains one function for the TUI, shown here in TypeScript style:

```ts
interface RequestLogEntry {
  Time: Date;
  Method: string;
  Path: string;
  Status: number;
  Duration: number;
  Matched: boolean;
}

// Returns a copy, newest first. Never fails; empty when there are no requests.
function RecentRequests(): RequestLogEntry[];
```

## Data Models

`RequestLogEntry`, held in memory only:

| Field | Type | Constraints |
|---|---|---|
| `Time` | `time.Time` | When the request completed |
| `Method` | `string` | As received |
| `Path` | `string` | `r.URL.Path`, no query string |
| `Status` | `int` | Status actually sent |
| `Duration` | `time.Duration` | Includes the configured delay |
| `Matched` | `bool` | False when no enabled mock matched |

`mocks.json` is unchanged.

## Out of Scope

- OS-1: Scrolling the log. Only the rows that fit are shown, because scrolling needs its own focus handling.
- OS-2: Filtering or searching the log. It can be added once real use shows it's needed.
- OS-3: Clearing the log from the TUI. It's bounded, and restarting clears it.
- OS-4: Persisting the log across restarts, since it's for live debugging.
- OS-5: Recording request or response bodies and headers. They would add memory use and could capture sensitive data.
- OS-6: Showing the log and the details at the same time, because the right panel doesn't have the height for both.
