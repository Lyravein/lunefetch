# Lunefetch Todo

Last verified: 2026-08-18

## In Progress

- [x] Integrate multi-select into table input and bulk actions. Rows now have keyboard-focusable checkboxes, and row menus expose Pause/Resume selected actions.
- [x] Add bulk Delete/Cancel actions with one confirmation dialog per batch.
- [ ] Complete the accessibility audit: keyboard focus, labels/tooltips, contrast, and keyboard-only workflows. Initial pass added labels to Actions/Settings and keyboard-focusable row checkboxes.
- [ ] Add E2E GUI coverage for add, pause, resume, cancel, fresh restart, and bulk actions. No Fyne GUI driver is configured in this repo; current coverage is unit/component-level.
- [x] Add system tray background mode. Closing the window hides it; tray menu provides Open, Add download, and Quit. Config key: `close_to_tray` (default `true`).

## Completed

- [x] Fix all five downloader core regressions.
  - Unknown `Content-Length` is accepted by metadata discovery.
  - Unknown-size responses use a sequential stream without a `Range` header.
  - The discovered size and progress are updated while streaming.
  - Empty response bodies are rejected.
  - Per-download cookies survive HTTP redirects.
- [x] Add per-task and global bandwidth limiting.
- [x] Add retry and resumable progress handling.
- [x] Add the fresh-restart dialog.
- [x] Add progress color helpers.
- [x] Add project documentation (`README.md`, `CHANGELOG.md`, and `AGENTS.md`).

## Skipped

- [ ] Drag-and-drop reordering. Skipped by user request.

## Verification

- [x] `go test ./internal/core -count=1`
- [x] `go test ./... -count=1`
- [x] `go test -race ./internal/core -count=1`
- [x] `go vet ./internal/core`
- [x] `go build ./...`
- [x] `go build -o lunefetch .`

Current result: all automated tests pass. The remaining work is UI integration, accessibility, and GUI-level coverage.
