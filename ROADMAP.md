# Roadmap

Where mocknroll is going. It's a plan, not a promise, and ideas are welcome in [Discussions](https://github.com/sebaguadagna/mocknroll/discussions) or issues.

## Now: ready to share

- [ ] Repo settings: description, topics, required checks and Dependabot
- [ ] Stricter CI: formatting, vet, race tests, vulnerability checks and lint
- [ ] CORS support, so web apps on another port can call the mocks
- [ ] Listen on `localhost` by default, with an option to open it up
- [ ] Publish the Docker image to GHCR, and tag `v0.1.0`
- [ ] Community files: `CONTRIBUTING`, `CODE_OF_CONDUCT`, `SECURITY`, and issue and PR templates
- [ ] A demo GIF in the README

## Next: more useful mocks

- [ ] Custom response headers per mock, such as `Set-Cookie` or `Retry-After`
- [ ] A mode with no TUI, for CI and background containers
- [ ] Align the status column in the request log

## Later: ideas

- [ ] Import mocks from an OpenAPI spec
- [ ] Responses cut partway through the body, as a fourth failure type
- [ ] A switch to take the whole server down at once

## Not planned

- Prebuilt binaries: build from source or use Docker.
- A proxy in front of a real backend: other tools already do this well.

## Done

- Mock server with delays, persisted to `mocks.json`
- Add, edit, delete and toggle mocks from the TUI
- Live request log
- Simulated failures: HTTP status, dropped connection and requests that never answer
- Path parameters
- `--port` flag
- Docker support
