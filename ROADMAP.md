# Lunefetch Roadmap

## Current Focus

UI Architecture refactor before any new features.
See [ARCHITECTURE.md](ARCHITECTURE.md) for design decisions.

---

## UI Refactor Phases

These phases must be completed in order.
Each phase must keep the project **buildable and runnable** before moving to the next.

| Phase | Scope | Status |
|-------|-------|--------|
| A | Restructure `internal/ui/` into `components/`, `pages/`, `layout/`, `store/` — no UI changes | done |
| B | Implement `Store` interface + concrete implementation — UI still looks the same, but reads from Store | done |
| C | Sidebar — fixed 220px, filter by status | done |
| D | Download Table — `widget.Table`, sortable columns, progress bar, realtime search | done |
| E | Inspector — `widget.Accordion`, detail panel on row select | done |
| F | Status Bar — global speed, active count, total count | done |

---

## Bug Fixes & Optimization

Hasil audit menyeluruh. Dikerjakan sebelum lanjut ke fitur v1.1 lainnya.

### Kritis
- [x] `FindByURL` SELECT hanya 13 kolom sementara `scanDownload` membaca 16 — setiap panggilan gagal diam-diam (`internal/storage/state.go:153`)
- [x] Data race: `SetRecords`/`SetSpeeds` menulis `dt.records`/`dt.speeds` di luar `fyne.Do` (`internal/ui/components/table.go:405,418`)
- [x] Scheduler tidak pernah memulai download — mengirim `AddURLRequest{}` kosong yang di-skip `addURLLoop` (`main.go:46`)

### Fungsionalitas Hilang
- [x] `queue.EnqueueScheduled` belum digunakan oleh GUI (`internal/queue/manager.go:75`)
- [x] Duplicate detection belum digunakan di `HandleAdd` GUI
- [x] `internal/notify` belum dihubungkan ke UI

### Optimalisasi
- [x] `http.Client` dibuat ulang per chunk tanpa timeout — gunakan satu client dengan `ResponseHeaderTimeout`
- [x] Hapus field mati `DownloadsPage.speeds`
- [x] Konsistenkan penggunaan `downloadEntry.mu` antara Pause dan Cancel
- [x] Gunakan token theme untuk warna status `downloading`, bukan hardcoded cyan

### Ditunda
- Queue reordering di GUI (`MoveQueuePosition` ada tetapi belum diekspos) — fitur baru, bukan bug
- Unit test komponen Fyne interaktif — store sorting sudah memiliki regression test

### Public Release Hardening
- [x] Validate remote/user filenames and enforce destination containment
- [x] Strictly validate HTTP range responses and exact chunk lengths
- [x] Use unique temporary paths and refuse silent destination replacement
- [x] Make queue ownership ID-based and completion idempotent
- [x] Recover interrupted/queued downloads and pause workers on clean shutdown
- [x] Authenticate and bound the local API and native messaging protocol
- [x] Prevent extension fallback recursion and use a stable Chromium identity
- [x] Validate config, secure local files, enable SQLite foreign keys, and make progress transactional
- [x] Add LICENSE, SECURITY policy, accurate Fyne documentation, and CI gates
- [x] Add explicit SSRF policy for private/loopback/link-local destinations and redirects
- [x] Add Keep Both / Replace / Cancel destination-conflict UI
- [x] Bind resume state to ETag/Last-Modified with `If-Range`
- [x] Add interactive Fyne navigation, validation, responsive, and accessibility tests
- [x] Complete clean Linux and Windows packaging smoke tests (native Windows runner builds and installs the Inno Setup package; interactive GUI acceptance remains manual)

---

## Browser Extension Roadmap

The current Firefox and Chromium packages support native-messaging handoff,
automatic interception, browser-download fallback, and a link context menu.
The extension sends only replayable HTTP/HTTPS URLs; browser credentials and
request bodies remain outside the integration boundary.

### Phase 1 — Reliability and Diagnostics

- [x] Introduce a tested browser API adapter for Firefox Promise APIs and Chromium callback APIs
- [x] Validate HTTP/HTTPS URLs before interception or native-messaging handoff
- [x] Deduplicate Firefox `webRequest` and `downloads.onCreated` events for the same download
- [x] Replace URL-count fallback tracking with per-download handoff state that handles concurrent identical URLs
- [x] Return typed native-host outcomes: accepted, app unavailable, invalid URL, unauthorized, queue full, and internal error
- [x] Add native-host health checks, connection status, and actionable install/start diagnostics
- [x] Preserve the browser download when cancellation, erase, or handoff fails

### Phase 2 — User Controls

- [x] Add an action popup showing Lunefetch connection status and interception state
- [x] Add a global enable/disable toggle without requiring extension reload
- [x] Add options for automatic interception, context-menu integration, and browser fallback
- [x] Add editable file-extension and MIME-type capture rules with safe defaults and reset support
- [x] Add site allowlist/blocklist rules and a temporary “do not intercept this site” action
- [x] Show badge/notification feedback for accepted handoffs and recoverable failures
- [x] Keep authenticated download transfer disabled unless a separate consent and secret-handling design is approved

### Phase 3 — Automated Browser Testing

- [x] Unit-test URL/MIME detection, state transitions, fallback recursion, and API compatibility
- [x] Add a deterministic mock native host for success, timeout, malformed-response, and unavailable-host scenarios
- [x] Add Chromium integration tests for context-menu handoff, automatic interception, fallback, and service-worker restart
- [x] Add Firefox integration tests for blocking `webRequest`, duplicate-event suppression, and browser fallback
- [x] Test concurrent identical URLs, redirects, signed query URLs, blob/data exclusions, and app shutdown during handoff
- [x] Run manifest validation, extension linting, packaging, and browser tests in CI

The Node integration harness is runnable locally without a browser binary. The Chromium smoke test is configured in CI with Playwright-managed Chromium; the local environment used during development did not provide a usable Chromium executable. Firefox package validation uses `web-ext lint`, while Firefox event behavior is covered through the Promise-API harness.

### Phase 4 — Installation and Distribution

- [x] Keep extension and application versions in one source of truth and fail builds on version drift
- [x] Produce deterministic Firefox and Chromium archives with checksums and release notes
- [x] Add complete extension icons, store descriptions, screenshots, privacy disclosure, and permission rationale
- [x] Add Firefox Add-ons signing and Chrome Web Store publication workflows (credential-gated; publication itself remains a maintainer action)
- [x] Add Windows native-host installation, including required Chromium registry entries and Firefox manifest paths
- [x] Build a per-user Windows installer containing the Fyne application, native host, shortcuts, browser registration, and clean uninstallation
- [x] Detect and document Chrome, Chromium, Brave, Edge, Vivaldi, Firefox, and Zen installation paths
- [x] Add clean Linux lifecycle smoke tests and Windows lifecycle tests for install, upgrade, uninstall, extension-ID stability, and stale native-host cleanup

Phase 4 distribution is implementation-complete. Store publication requires maintainer-owned AMO/CWS credentials and review approval; the workflow is intentionally manual and secret-gated. Windows lifecycle validation runs on the Windows CI runner because registry behavior cannot be reproduced on Linux.

### Phase 5 — Post-Release Enhancements

- [x] Add “Download all with Lunefetch” for selected page links with a confirmation preview
- [x] Add optional filename and destination hints without transferring credentials
- [x] Expose recent handoff failures and retry actions in the popup
- [x] Evaluate an explicitly opt-in authenticated-download protocol; threat modeling deferred implementation until secure secret storage and origin-bound consent exist

Phase 5 keeps the integration credential-free. The decision criteria and required controls for any future authenticated mode are documented in `docs/authenticated-download-threat-model.md`.

---

## v1.0 — MVP

Everything needed to use Lunefetch comfortably day-to-day.

### Core
- [x] Multi-chunk parallel download
- [x] Pause / Resume / Cancel
- [x] Queue
- [x] Auto retry
- [x] Rename before download
- [x] Choose save folder per download
- [x] Duplicate detection

### Browser Integration
- [x] Firefox extension
- [x] Chromium extension

### Network
- [x] Speed limit (per-download + global)
- [x] Scheduler (start at HH:MM)
- [x] Proxy (HTTP + SOCKS5)

### File Management
- [x] Auto category from file extension
- [x] Save folder per download

### UI
- [x] Sidebar filters and file-category navigation
- [x] Download table with sortable columns and progress bars
- [x] Selected-download detail panel
- [x] Status bar
- [x] Empty state
- [x] History page

---

## v1.1 — UX Improvements

Quality-of-life features after v1.0 is solid.

- [x] Context menu (row actions): Pause, Resume, Cancel, Open File, Open Folder, Copy URL, Remove
- [ ] Double-click row to open folder
- [x] Progress bar visualization in table
- [ ] Desktop notifications on completion
- [ ] Multi-select (Ctrl+A, Shift+Click, Ctrl+Click) with batch operations
- [ ] Queue priority (Highest / High / Normal / Low)
- [ ] History view improvements

---

## v1.2 — Power User Features

Features that differentiate Lunefetch from basic download managers.

- [ ] Proxy per download (override global)
- [ ] Clipboard monitor — detect URL copy, show "Download?" popup
- [ ] Export / Import (JSON, TXT)
- [ ] Theme manager (Dark, Light, and presets)
- [ ] Tags — label downloads, search by `tag:name`
- [ ] Favorites — pin important downloads

---

## Future

No timeline. Considered only after v1.2 is done.

- Torrent support
- Checksum verification (MD5 / SHA-256 / SHA-1)
- Resume validation beyond `If-Range` (for example `If-Match`)
- Download mirrors — fallback to mirror if primary fails
- Plugin system
- Remote API
- Web UI
- Auto shutdown after queue completes
- Download statistics and graphs
- RSS-based auto download
- Plugin downloaders (Google Drive, Mega, etc.)
- Mobile companion app

---

## Deferred Decisions

| Topic | Decision | Revisit when |
|-------|----------|--------------|
| Event-driven refresh | Polling 500ms is sufficient | >500 concurrent downloads |
| Repository layer | Not needed yet | Adding a second storage backend |
| Resizable sidebar | Fixed 220px is enough | User feedback says otherwise |
| Theme system | Deferred to v1.2 | Layout is finalized |
| `components/` subfolders | Flat for now | Any component reaches 4+ files |
