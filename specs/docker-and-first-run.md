# Spec: Docker and First Run

- **Author:** sebaguadagna, with Claude Code
- **Date:** 2026-10-31
- **Status:** Approved
- **Reviewers:** sebaguadagna

## Context

mocknroll is an open project on GitHub, and it won't publish binaries: people build it from source or run it with Docker. Today building requires Go, so anyone without a Go toolchain can't try it. A Dockerfile in the repo removes that requirement.

Both ways expose a first-run bug that's hidden inside the repo. With no `mocks.json`, the app creates two example mocks that point at `examples/users.json` and `examples/orders.json`, relative to the working directory. Run from any other directory, or with an empty Docker volume, both mocks answer `500` with `failed to read json file`, and the preview shows `couldn't read examples/users.json`. This was reproduced on the current code.

The README also says Go 1.24 is enough, in its badge and its install section, while `go.mod` requires 1.25.0.

## Functional Requirements

### Docker

- FR-1: The repo MUST include a `Dockerfile` that builds the app from source, with no Go toolchain needed on the host.
- FR-2: The container's working directory MUST be `/data`, where the app keeps `mocks.json` and reads relative response files.
- FR-3: The image MUST run as a non-root user by default.
- FR-4: The image's entrypoint MUST be the app, so arguments such as `--port 9000` passed to `docker run` reach it.
- FR-5: The repo MUST include a `.dockerignore` that keeps `.git`, `bin/` and `mocks.json` out of the build context.
- FR-6: CI MUST build the image, so a change that breaks the Dockerfile fails the check.

### First run

- FR-7: When no `mocks.json` exists, the app MUST create the example mocks with response files that exist, in any working directory.
- FR-8: The example files MUST be embedded in the binary and written to `./examples/` only when the app creates the example mocks.
- FR-9: The app MUST NOT overwrite a file that already exists in `./examples/`.
- FR-10: When a `mocks.json` exists, including an empty one, the app MUST NOT write to `./examples/`.

### Documentation

- FR-11: The README MUST explain how to build and run with Docker, including `-it`, publishing the port, and mounting a directory with the user's own UID so mocks persist.
- FR-12: The README MUST explain that the title shows the port inside the container, so the published port should match it.
- FR-13: The README MUST state Go 1.25 as the requirement to build from source, in its badge and its install section, and MUST NOT say that releases are coming.

## Non-Functional Requirements

- NFR-1: The base images MUST be pinned to a specific version.
- NFR-2: `go.mod` MUST have no new requirements. Embedding uses the standard `embed` package.
- NFR-3: The format of `mocks.json` MUST NOT change.
- NFR-4: `go vet` and `go test -race ./...` MUST pass.

## Acceptance Criteria

### AC-1: Build and run (FR-1, FR-2, FR-3, FR-4)
Given a machine with only Docker, when the image is built and run with `-it -p 8080:8080`, then the TUI opens, `GET /api/v1/users` answers 200 from the host, the process isn't running as root, and the working directory is `/data`.

### AC-2: Arguments (FR-4)
Given `docker run -it -p 9000:9000 <image> --port 9000`, when the app starts, then the title shows port 9000 and a mock answers on that port from the host.

### AC-3: Persistence (FR-2, FR-11)
Given a host directory mounted at `/data` and the documented `--user` option, when a mock is added and the container is recreated, then the mock is still there, and `mocks.json` on the host belongs to the user.

### AC-4: Build context (FR-5)
Given the `.dockerignore`, when the image is built with a `mocks.json` in the repo, then that file isn't in the image.

### AC-5: CI (FR-6)
Given the CI workflow, when a push or PR runs, then a step builds the image.

### AC-6: First run outside the repo (FR-7, FR-8)
Given an empty directory, when the app starts and `GET /api/v1/users` is requested, then it answers 200 with the example JSON, the preview shows that JSON, and `examples/users.json` and `examples/orders.json` exist.

### AC-7: Existing files stay (FR-9)
Given a changed `examples/users.json` and no `mocks.json`, when the app starts, then that file keeps its content and `examples/orders.json` is created.

### AC-8: Existing config (FR-10)
Given a `mocks.json`, including one holding an empty list, when the app starts, then `./examples/` isn't created.

### AC-9: Documentation (FR-11, FR-12, FR-13)
Given the README, when it's read, then it covers the Docker commands, the persistence option and the port note, and it no longer mentions Go 1.24 or upcoming releases.

### AC-10: No side effects (NFR-1, NFR-2, NFR-3, NFR-4)
Given the finished feature, when CI runs, then vet and race tests pass, `go.mod` has no new requirements, a `mocks.json` from the previous version loads, and the Dockerfile's base images have fixed versions.

## Edge Cases

- EC-1: If `./examples/` can't be created or written, as in a read-only directory, the app MUST still start. The mocks answer 500 and the preview shows the read error, as they do today.
- EC-2: If `./examples` exists as a file rather than a directory, EC-1 MUST apply, and the file MUST be left unchanged.
- EC-3: A mount the container's user can't write to MUST behave as a failed write does today: the app keeps running, and the README's `--user` option is the documented fix.
- EC-4: Running without `-it` isn't a supported use. It MUST end with an error rather than hang or panic.
- EC-5: A response file outside the mounted directory can't be read in the container. It MUST answer 500 and show the read error, as any missing file does.

## API Contracts

No HTTP routes or flags change. Docker usage:

```ts
// docker build -t mocknroll .
// docker run -it --rm -p 8080:8080 [--user "$(id -u):$(id -g)" -v "$PWD:/data"] mocknroll [--port <n>]
```

## Data Models

N/A — `mocks.json` is unchanged, and the example files keep their current content.

| Field | Type | Constraints |
|---|---|---|
| `/data` (container) | directory | working directory, holds `mocks.json` and relative response files |

## Out of Scope

- OS-1: Prebuilt binaries, GitHub Releases, tags and a `--version` flag. The project is built from source.
- OS-2: Publishing the image to a registry. Users build it from the repo, and a registry can be added later.
- OS-3: Docker Compose. One `docker run` is enough for one service.
- OS-4: Package managers such as Homebrew or apt.
- OS-5: Resolving response files relative to `mocks.json` instead of the working directory. It changes behavior for existing users and deserves its own spec.
- OS-6: A headless mode with no TUI. It would make the container easier to run in the background, and it deserves its own spec.
