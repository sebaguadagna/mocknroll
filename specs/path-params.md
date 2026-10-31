# Spec: Path Parameters

- **Author:** sebaguadagna, with Claude Code
- **Date:** 2026-10-31
- **Status:** Approved
- **Reviewers:** sebaguadagna

## Context

A mock only answers the exact path it was created with. To fake `GET /users/1`, `GET /users/2` and `GET /users/3`, a user needs three mocks, and any other ID gets a 404. Real APIs identify resources in the path, so this is the first limit users reach when pointing a client at mocknroll.

A route such as `/users/{id}` covers every ID with one mock. Echoing the captured value in the response also keeps clients working when they check that the resource they received is the one they asked for, such as a detail screen comparing IDs.

## Functional Requirements

### Matching

- FR-1: A path segment written as `{name}` MUST match exactly one non-empty segment of the request path.
- FR-2: All other segments MUST match literally and case-sensitively, as they do today.
- FR-3: When several enabled mocks with the request's method match, the server MUST use the one with the most literal segments, and on a tie the first in the list.

### Response

- FR-4: In a mock with parameters, each `{name}` in its JSON file's content that names one of its parameters MUST be replaced with the captured value.
- FR-5: A captured value MUST be JSON-escaped before being inserted, and MUST NOT be substituted again.
- FR-6: Any other text in braces MUST be left unchanged.
- FR-7: The JSON file MUST NOT be modified on disk.

### Form

- FR-8: The form's path step MUST accept parameter names that start with a letter or underscore and contain only letters, digits and underscores.
- FR-9: The form's path step MUST reject a path with a brace outside a whole `{name}` segment, an invalid name, or a name repeated in the path.
- FR-10: The duplicate-route check MUST ignore parameter names, so `GET /users/{id}` and `GET /users/{name}` count as the same route.

### Traffic and documentation

- FR-11: Traffic MUST count on the mock that answered, and the request log MUST show the path actually requested.
- FR-12: The README MUST describe path parameters, their precedence and their substitution.

## Non-Functional Requirements

- NFR-1: Mocks with no parameters MUST answer exactly as they do today, including a JSON file that contains text in braces.
- NFR-2: The format of `mocks.json` MUST NOT change.
- NFR-3: `go vet` and `go test -race ./...` MUST pass.
- NFR-4: The feature MUST NOT add dependencies.

## Acceptance Criteria

### AC-1: (FR-1)
Given a mock `GET /users/{id}`, when a client requests `/users/42` or `/users/abc`, then the mock answers.

### AC-2: (FR-1)
Given a mock `GET /users/{id}`, when a client requests `/users`, `/users/` or `/users/42/orders`, then the server answers 404.

### AC-3: (FR-1, FR-2)
Given a mock `GET /users/{id}/orders`, when a client requests `/users/42/orders`, then the mock answers, and when they request `/users/42/ORDERS`, then the server answers 404.

### AC-4: (FR-3)
Given `GET /users/{id}` listed before `GET /users/me`, when a client requests `/users/me`, then `GET /users/me` answers, and `/users/42` is answered by `GET /users/{id}`.

### AC-5: (FR-3)
Given `GET /a/{x}/c` and `GET /a/b/{y}` in that order, when a client requests `/a/b/c`, then `GET /a/{x}/c` answers.

### AC-6: (FR-4)
Given a mock `GET /users/{id}` whose file holds `{"id": "{id}"}`, when a client requests `/users/42`, then the body is `{"id": "42"}`.

### AC-7: (FR-5)
Given a mock `GET /users/{id}` whose file holds `{"id": "{id}"}`, when a client requests `/users/a%22b`, then the body is valid JSON and its `id` is `a"b`.

### AC-8: (FR-5)
Given a mock `GET /{a}/{b}` whose file holds `"{a} {b}"`, when a client requests `/%7Bb%7D/x`, then the body is `"{b} x"`.

### AC-9: (FR-6)
Given a mock `GET /users/{id}` whose file holds `{"id": "{id}", "x": "{other}"}`, when it answers, then `{other}` is unchanged.

### AC-10: (FR-7)
Given a mock with parameters that has answered a request, when its JSON file is read, then its content is unchanged.

### AC-11: (FR-8)
Given the path step, when the user submits `/users/{id}` or `/a/{_b1}/{c}`, then the form accepts it.

### AC-12: (FR-9)
Given the path step, when the user submits `/files/{id}.json`, `/a{b}`, `/{}`, `/{1a}`, `/a}` or `/{id}/{id}`, then the form stays on that step and shows an error.

### AC-13: (FR-10)
Given a mock `GET /users/{id}`, when the user creates `GET /users/{name}`, then a duplicate error appears on the method step, and when they create `POST /users/{name}`, then none appears.

### AC-14: (FR-11)
Given a mock `GET /users/{id}`, when a client requests `/users/42`, then its traffic count increases by one and the log row shows `/users/42`.

### AC-15: (FR-12)
Given the finished feature, when the README is read, then it explains path parameters, precedence and substitution.

### AC-16: (NFR-1)
Given a mock `GET /plain` whose file holds `{"id": "{id}"}`, when it's requested, then the body is returned unchanged.

### AC-17: (NFR-2, NFR-3, NFR-4)
Given the finished feature, when CI runs, then vet and race tests pass, `go.mod` has no new requirements, and a `mocks.json` from the previous version loads.

## Edge Cases

- EC-1: A disabled mock MUST NOT match, so a disabled `GET /users/me` MUST leave `/users/me` to an enabled `GET /users/{id}`.
- EC-2: A query string MUST NOT affect matching or captured values.
- EC-3: An encoded slash, as in `/users/a%2Fb`, counts as a separator because the server matches the decoded path, so it MUST NOT match `/users/{id}`.
- EC-4: Captured values MUST be URL-decoded, so `/users/a%20b` captures `a b`.
- EC-5: A hand-edited path with an invalid parameter, such as `/{1a}`, MUST be treated as literal text, with no panic.
- EC-6: A hand-edited path that repeats a name, such as `/{id}/{id}`, MUST match and substitute the last value, with no panic.
- EC-7: A simulated failure MUST keep its fixed body, with no substitution.
- EC-8: A mock with parameters and no JSON file MUST answer `{}`.
- EC-9: Editing a mock from `/users/{id}` to `/users/{name}` MUST NOT report a duplicate with itself.
- EC-10: A control character in a captured value MUST NOT alter the terminal, since the log already sanitizes paths. In the body, it MUST be JSON-escaped.

## API Contracts

No HTTP routes or exported Go signatures change. `server.Mock.Path` may now contain `{name}` segments, shown here in TypeScript style:

```ts
interface Mock {
  // Existing fields stay.
  Path: string; // such as "/users/{id}/orders"
}
```

## Data Models

`mocks.json` is unchanged. Only the values accepted in the path part of `title` grow:

| Field | Type | Constraints |
|---|---|---|
| `title` | string | `METHOD /path`, where a whole segment may be `{name}` |

## Out of Scope

- OS-1: Parameters inside a segment, such as `/files/{id}.json`. Whole segments cover REST routes and keep precedence simple.
- OS-2: Wildcards that span segments, such as `/files/{path...}`.
- OS-3: Type constraints, such as digits only. They can be added if real use needs them.
- OS-4: Substituting query strings, headers or request bodies into the response.
- OS-5: A template language with conditionals or defaults. Substitution only replaces names.
- OS-6: Different responses per parameter value. A literal mock such as `GET /users/me` already covers one-off values.
