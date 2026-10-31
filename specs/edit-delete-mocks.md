# Spec: Edit and Delete Mocks

- **Author:** sebaguadagna, with Claude Code
- **Date:** 2026-10-30
- **Status:** Approved
- **Reviewers:** sebaguadagna

## Context

mocknroll lets users create mocks and toggle them on or off, but a mock can't be changed or removed from the TUI. A typo in a path or a wrong status forces the user to quit, edit `mocks.json` by hand and restart.

The form validation added in #11 already defines which values are valid, so editing can reuse it. Deletion exposes one existing behavior that must change: `initialModel` recreates the example mocks whenever `mocks.json` holds no mocks, so a user who deleted every mock would get the examples back on the next start.

## Functional Requirements

### Editing

- FR-1: Pressing `e` in `listMode` MUST open the form for the selected mock, with all five fields prefilled from it.
- FR-2: The edit form MUST apply the same per-field validation and normalization as the add form.
- FR-3: The duplicate-route check MUST ignore the mock being edited, so keeping its route is valid.
- FR-4: Pressing `Esc` in the edit form MUST discard all changes and return to `listMode`.
- FR-5: Submitting the edit form MUST replace the mock in place, keeping its position and enabled state.
- FR-6: Submitting the edit form MUST show the existing provisioning animation, and the change MUST apply when it ends.
- FR-7: After the change applies, the mock MUST be persisted to `mocks.json` and served with its new values.
- FR-8: Traffic history MUST be kept when the method and path are unchanged, and MUST reset to zero when either changes.
- FR-9: An auto-generated description (`Status: X, Delay: Yms`) MUST be regenerated from the new values. Any other description MUST be kept.
- FR-10: The form header and animation text MUST say the mock is being edited, not created.

### Deleting

- FR-11: Pressing `d` in `listMode` MUST open a confirmation popup naming the selected mock.
- FR-12: Pressing `y` or `Y` in the popup MUST remove the mock, persist the list and stop serving it, with no animation.
- FR-13: Pressing `n`, `N` or `Esc` in the popup MUST return to `listMode` with no changes.
- FR-14: The example mocks MUST only be created when `mocks.json` doesn't exist. A `mocks.json` holding an empty list MUST start with an empty list.

### Shared

- FR-15: `e` and `d` MUST act on the mock selected in the list when a filter is applied, not on the mock at the same index of the unfiltered list.
- FR-16: While the user is typing in the filter, `e` and `d` MUST be entered as filter text.
- FR-17: `e` and `d` MUST do nothing when no mock is selected.
- FR-18: The list's hint line and the README keybinding table MUST include `e` and `d`.

## Non-Functional Requirements

- NFR-1: Edits and deletions MUST NOT require restarting the server or dropping its listener.
- NFR-2: `go vet` and `go test -race ./...` MUST pass.
- NFR-3: The feature MUST NOT add dependencies.
- NFR-4: The format of `mocks.json` MUST NOT change, so existing files keep loading.

## Acceptance Criteria

### AC-1: (FR-1)
Given a selected mock, when the user presses `e`, then the form opens at the path step and each step shows that mock's value.
### AC-2: (FR-2)
Given the edit form on the status step, when the user submits `99`, then the form stays on that step, shows an error and keeps the text.
### AC-3: (FR-3)
Given mock `GET /a`, when the user edits it and keeps method and path, then no duplicate error appears.
### AC-4: (FR-3)
Given mocks `GET /a` and `GET /b`, when the user edits `GET /b` to path `/a`, then a duplicate error appears on the method step.
### AC-5: (FR-4)
Given changes typed in the edit form, when the user presses `Esc`, then the list is unchanged and a later add form starts empty.
### AC-6: (FR-5, FR-7)
Given a disabled second mock, when the user edits its status to `404` and the animation ends, then it remains second and disabled, has status `404`, and `mocks.json` matches.
### AC-7: (FR-6)
Given a submitted edit, when the animation is running, then the server still answers with the old values, and when it ends, the server answers with the new ones.
### AC-8: (FR-8)
Given a mock with traffic, when only its status changes, then its traffic buckets are kept.
### AC-9: (FR-8)
Given a mock with traffic, when its path changes, then its traffic buckets are all zero.
### AC-10: (FR-9)
Given a mock with description `Status: 200, Delay: 0ms`, when its delay becomes `50`, then its description becomes `Status: 200, Delay: 50ms`.
### AC-11: (FR-9)
Given a mock with description `Returns users list`, when its delay changes, then its description is unchanged.
### AC-12: (FR-10)
Given the edit form or its animation, when it renders, then it shows edit wording, and the add flow keeps its current wording.
### AC-13: (FR-11, FR-13)
Given a selected mock, when the user presses `d` then `n`, then the popup names the mock and the list is unchanged.
### AC-14: (FR-12, NFR-1)
Given the delete popup, when the user presses `y`, then the mock is gone from the list and `mocks.json`, and the running server answers its route with 404.
### AC-15: (FR-14)
Given the user deleted every mock, when the app starts again, then the list is empty and `mocks.json` still holds an empty list.
### AC-16: (FR-14)
Given no `mocks.json`, when the app starts, then the example mocks are created.
### AC-17: (FR-15)
Given a filter showing only the second mock, when the user deletes the selection, then only the second mock is removed.
### AC-18: (FR-15)
Given a filter showing only the second mock, when the user edits the selection, then only the second mock changes.
### AC-19: (FR-16)
Given the user is typing in the filter, when they press `e` or `d`, then the mode stays `listMode` and the letter is added to the filter.
### AC-20: (FR-17)
Given an empty list, when the user presses `e` or `d`, then the mode stays `listMode`.
### AC-21: (FR-18)
Given `listMode`, when the view renders, then the hint line includes the edit and delete keys.
### AC-22: (NFR-2, NFR-3, NFR-4)
Given the finished feature, when CI runs, then vet and race tests pass, `go.mod` has no new requirements, and a `mocks.json` from the previous version loads.

## Edge Cases

- EC-1: The `d` key is bound to next page in the list. It MUST open the delete popup in `listMode`; paging stays available through its other keys.
- EC-2: Deleting the last row MUST leave a valid selection, or none if the list is empty, with no panic.
- EC-3: Deleting a mock with a filter applied MUST keep the filter, and the filtered view MUST no longer show that mock. If it was the only match, the filter MUST clear and show the full list.
- EC-4: A request in flight when its mock is deleted or edited MUST finish with the values it matched.
- EC-5: Counts pending for a deleted route MUST be discarded without error.
- EC-6: An edited mock whose JSON file has since been removed MUST be rejected on the JSON file step until the path is fixed or cleared.
- EC-7: Editing a hand-written mock with invalid values MUST prefill them and require fixing them before submitting.
- EC-8: A `mocks.json` holding `null` MUST be treated as an empty list.

## API Contracts

N/A: no HTTP routes change. Mocks are still pushed to the server through the existing `server.SetMocks`.

## Data Models

`mocks.json` is unchanged:

| Field | Type | Constraints |
|---|---|---|
| `title` | string | `METHOD /path`, unique among the form's checks |
| `description` | string | Free text or auto-generated |
| `status` | string | Numeric status, validated by the form |
| `delay` | string | Milliseconds, validated by the form |
| `json_file` | string | Optional path, must exist when submitted |
| `enabled` | bool | Not editable in the form |

## Out of Scope

- OS-1: Undoing a deletion. The confirmation popup covers accidental presses.
- OS-2: Editing the description or enabled state in the form. `t` already toggles, and adding a description field also affects the add form.
- OS-3: Moving the cursor inside a field. The form only supports append and backspace today.
- OS-4: Deleting several mocks at once.
- OS-5: Deleting the response JSON file from disk, since it may be shared or hand-written.
- OS-6: Reporting `mocks.json` write failures in the UI. `updateServer` already ignores them, so a failed write is only noticed on restart. This deserves its own fix.
