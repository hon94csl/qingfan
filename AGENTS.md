# Repository Guidelines

## Project Structure & Module Organization

Qingfan is currently a documentation-only repository. Start with `README.md`, the architecture document in `docs/`, and `docs/plans/2026-10-13-qingfan-mvp-development-plan.md`.

Planned paths, created when their milestone begins:

- `cmd/qingfan-render/`: M1 command-line rendering entry point.
- `internal/`: shared timeline, media, storage, jobs, AI, and reporting packages. Keep media logic independent of Wails.
- `main.go`, `app.go`, `wails.json`, `frontend/`: M2 desktop application and React/TypeScript interface.
- `cmd/qingfan-server/`: M3 AI gateway.
- `tests/fixtures/`, `tests/integration/`: distributable samples and real FFmpeg checks.
- `docs/validation/`: baseline measurements and milestone evidence.

## Build, Test, and Development Commands

No application scaffold exists yet. Use `git diff --check` to check tracked changes for whitespace errors.

After M1 creates the Go module and packages, use:

- `go build -o bin/qingfan-render ./cmd/qingfan-render`: build the renderer.
- `go test ./internal/...`: run core package tests.
- `go test ./tests/integration/...`: run integration checks with FFmpeg/ffprobe installed.

Document working CLI arguments, versions, and frontend commands in README as implementation lands.

## Coding Style & Naming Conventions

Format Go with `gofmt`; use its tab indentation, lowercase package names, and descriptive exported identifiers. Use two-space indentation for TypeScript, PascalCase React components, and camelCase functions. Follow scaffold conventions once established; no frontend formatter or linter is configured yet.

Keep Wails bindings thin. Pass FFmpeg arguments as an array, never through interpolated shell commands. Prefer small implementations over unused abstractions.

## Testing Guidelines

Use Go's `testing` package with `*_test.go` files and `Test...` functions. Frontend component tests should use `*.test.tsx`; select their runner during M2.

Cover invalid timelines, source boundaries, reference-material rejection, missing assets, special-character paths, and failure recovery. Rendering acceptance requires an actual output inspected with ffprobe and human playback. No numeric coverage threshold is established; satisfy the milestone's listed scenarios. Mocked AI responses cannot establish language or content quality.

## Commit & Pull Request Guidelines

History uses concise `docs: ...` subjects. Continue `<type>: <action>` messages, such as `feat: add timeline validation`.

PR descriptions should identify the milestone, behavior, validation, and limitations. Link plans or issues; include screenshots for interface changes and sample-output evidence for rendering changes.

## Security & Scope

Keep credentials, private media, databases, and exports outside Git; use ignored `data/`, `cache/`, and `outputs/` directories. Commit only self-created or redistributable fixtures. Supplier keys belong on the server.

Implement the assigned milestone. TikTok authorization is not a prerequisite for M1–M3. Preserve original media and successful exports; never silently skip required clips, audio, or subtitles.
