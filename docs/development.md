# Development

## Layout

```
internal/
├── core/            # Download engine: chunking, ranges, limiter, destination policy
├── queue/           # Concurrency manager; decides when a download starts
├── storage/         # SQLite persistence for downloads and chunks
├── config/          # YAML config with validation and atomic writes
├── api/             # Authenticated loopback API for the browser extension
├── notify/          # Desktop notifications
├── filecat/         # Filename to category mapping
├── userpath/        # Per-user directories for Linux and Windows
├── singleinstance/  # Advisory lock preventing a second instance
└── ui/
    ├── components/  # Sidebar, header, download list, status bar, dialogs
    ├── pages/       # Downloads and history pages
    ├── layout/      # Window assembly, shortcuts, tray
    ├── store/       # The only bridge between UI and backend
    ├── theme/       # Moonlit dark theme tokens
    ├── assets/      # Embedded icon
    └── fatal/       # Startup-error window for GUI builds

extension/
├── src/             # Background worker, popup, options, batch page
├── manifests/       # Per-browser manifest (Firefox and Chromium)
├── test/            # node --test suites and a Chromium smoke test
└── store/           # Listing copy, privacy and permission rationale
```

Layering rules and design decisions are in
[../ARCHITECTURE.md](../ARCHITECTURE.md). Conventions and known pitfalls worth
reading before changing anything are in [../AGENTS.md](../AGENTS.md).

## Desktop tests

```bash
go build ./...
go vet ./...
go test ./... -race -count=1
```

The GUI tests need a headless-safe environment. To reproduce what CI runs:

```bash
env -u DISPLAY -u WAYLAND_DISPLAY HOME=/tmp/lf-cihome \
  go test ./... -race -count=1
```

Installer lifecycle, covering install, upgrade, and uninstall:

```bash
bash scripts/test-install-linux.sh
```

Windows-only code paths cross-compile without a Windows machine. The GUI packages
need cgo, so they only build on a real Windows runner, which is what CI is for.

```bash
GOOS=windows go build ./internal/userpath/ ./internal/singleinstance/ \
  ./internal/notify/ ./internal/storage/ ./internal/config/ ./internal/api/ \
  ./internal/core/ ./internal/queue/ ./cmd/native-host/
```

## Visual checks

UI components render to PNG so layout can be inspected without a screenshot by
hand:

```bash
LUNEFETCH_VISUAL_DIR=/tmp/lunefetch-visual \
  go test ./internal/ui/components -run 'Visual|DesktopShellRenders' -count=1
```

## Extension tests

```bash
cd extension
npm ci
npm test              # unit and lifecycle suites
npm run check         # syntax check every source file
./build.sh            # produces dist/lunefetch-{firefox,chromium}.zip
npm run lint:firefox  # web-ext lint, must report 0 errors and 0 warnings
npm run browser:chromium  # loads the built extension in Chromium
```

Run the Chromium smoke test after any change to `background.js`: it has caught
regressions the unit tests could not, because its sender objects are real.

`build.sh` copies source files verbatim and packages them deterministically, with
a pinned `SOURCE_DATE_EPOCH`, normalized timestamps, and a fixed entry order.
Two runs of the same source produce byte-identical archives.

## Versioning

`VERSION` is the single source of truth. `scripts/check-version.sh` holds both
extension manifests to it and, on a tag build, verifies the tag matches.

## CI

`.github/workflows/safety-net.yml` runs the full race-enabled suite and `go vet`
on Linux and Windows, plus the installer lifecycle and a dedicated extension job.
Pushing a `v*` tag runs `release.yml`, which builds both installers and publishes
a GitHub release. Store publication is a separate manual workflow dispatch.
