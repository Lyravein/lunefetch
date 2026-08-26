# Lunefetch Desktop UI Redesign Plan

Status: shipped in 1.1.0. All phases implemented and reviewed on Linux; manual
Windows review (row action menus, 100/125/150% DPI) is still outstanding.

This document is the source of truth for the Lunefetch desktop GUI redesign.
Implementation decisions should not be changed implicitly. Update this document
first when a decision changes.

## Objective

Redesign the Fyne desktop GUI into a modern, minimal moonlit download manager.
The visual reference uses a near-black navy shell, pale blue and silver accents,
subtle translucency, restrained shadows, spacious rows, and a moon ambience in
the upper-right of the main content area.

The redesign applies to the desktop application only; the browser extension
keeps its current visual system. (The legacy Bubble Tea TUI was unused and has
since been removed from the tree.)

## Locked Decisions

| Area | Decision |
|------|----------|
| Product name | Keep **Lunefetch**. `LunaFetch` in the visual reference is not a rename. |
| GUI framework | Keep Fyne v2.8.0. Do not migrate to Wails, Tauri, Gio, or GTK. |
| Supported platforms | Linux and Windows remain first-class targets. |
| Theme modes | Dark only. Do not add a light-theme switch. |
| Font | Use Fyne's default font initially. Inter may be bundled later. |
| Moon ambience | Use a simple radial/linear gradient first. Do not search for or create a moon image asset yet. |
| Panel treatment | Use lightly translucent surfaces without `canvas.Blur`. |
| Shadows | Use restrained per-object shadows where they improve hierarchy. They must be easy to disable if Windows rendering regresses. |
| Window chrome | Keep native/compositor window decoration. Do not create a borderless custom titlebar. |
| Window sizing | Keep the window resizable and preserve saved geometry. Add a minimum size around 1000x640 so the layout cannot collapse. |
| Existing toolbar | Remove it. Move its surviving actions into the new content header and sidebar. |
| Search | Place it in the main content header, aligned with `+ New Download`. Keep `Ctrl+F`. |
| Detail panel | Remove the current 82px detail panel below the table. The status bar takes its place. |
| Sidebar badges | Do not show download/category counts in sidebar rows. |
| Theme footer icon | Do not show it because the application is dark-only. |
| Multi-select | Hide checkboxes normally. Show them only while selection mode is active. |
| Hidden sorting | Keep Size and Date Added sorting through a separate sort menu. |
| Delivery | Implement and review one phase at a time. Each phase must leave the application buildable and testable. |

## Visual Direction

### Palette and hierarchy

- Base background: near-black navy, not pure black.
- Sidebar: darker than the main content area.
- Main panel and rows: layered navy elevations with subtle transparency.
- Primary accent: pale moonlight blue/silver.
- Primary text: soft white, not pure white.
- Secondary text: blue-gray with sufficient contrast.
- Success, warning, and error colors remain distinct from the primary accent.
- Borders: thin and low-contrast, used to define panels without card-heavy noise.
- Radius: generally 10-14px for panels and controls.
- Spacing: spacious enough for scanning while preserving download-manager density.

### Background ambience

- Use `canvas.NewRadialGradient` and/or `canvas.NewLinearGradient`.
- Offset the brighter radial area toward the upper-right.
- The ambience must not reduce table or header readability.
- Do not add decorative moon imagery to every component.

### Platform rendering notes

- Hyprland already supplies rounded window corners on the primary Linux setup.
- Windows uses normal native Fyne window decoration.
- The mockup's custom in-content minimize/maximize/close controls are out of scope.
- The Linux compositor currently applies `active_opacity = 0.95`; visual checks must
  ensure translucent application panels remain readable under that setting.

## Target Layout

### Sidebar

- Width: approximately 260px.
- Header: moon/crescent mark, `Lunefetch`, and `Download Manager` subtitle.
- Main navigation order:
  1. All Downloads
  2. Downloading
  3. Queued
  4. Scheduled
  5. Completed
  6. Paused
  7. Failed
- Active row: lighter navy surface, rounded corners, clear icon and text contrast.
- Separator before Categories.
- Category order:
  1. All
  2. Compressed
  3. Documents
  4. Media
  5. Programs
  6. Other
- Footer actions: Settings, History, About.
- Do not show count badges.

### Main header

- Time-aware greeting: Good morning, Good afternoon, or Good evening.
- Supporting text: `Manage your downloads with ease.`
- Primary action: `+ New Download`.
- Search field aligned in the same header area.
- Summary cards: Downloading, Completed, Paused, Failed.

### Download list

- Large framed panel with a subtle translucent surface and thin border.
- Visible columns: NAME, STATUS, PROGRESS, SPEED, ETA.
- Size appears as secondary text below the filename.
- Each row includes a file-type icon, status icon/color, progress percentage and
  bar, speed, ETA, and a trailing `...` action button.
- Size and Date Added remain available as sort options through a separate menu.
- Selection-mode checkboxes appear only while selection mode is active.

### Status bar

- Left: moon icon and `Overall Speed: ...`.
- Right: `Concurrent Downloads: N` with a dropdown affordance.
- Changing concurrency updates `cfg.MaxConcurrent` and the queue manager.

## Known Windows UI Bug

A Windows user reported that the old UI looked malformed, the trailing action
button could not be clicked, and downloads therefore could not be removed. The
same UI appears acceptable on Linux.

### Identified root causes

1. The trailing action column is fixed at only 10 units in both `colWidths` and
   `colMinWidths`, while its widget contains both the `Actions` text and a
   vertical-more icon.
2. `proportionalWidths` never expands that column because configured width and
   minimum width are both 10.
3. Progress-cell rectangles are manually resized inside the table update
   callback. This competes with Fyne's layout cycle and is sensitive to platform
   font metrics and Windows DPI scaling.
4. Current Windows CI tests package/install lifecycle but does not render the UI.

The redesign's custom row implementation must eliminate these causes. The bug
is intentionally resolved in Phase 4 rather than patched separately before the
redesign.

## Implementation Phases

### Phase 1: Theme foundation

Status: **complete; Linux review done, Windows review pending**

- Rewrite `internal/ui/theme/navy.go` with the moonlit dark palette.
- Define distinct base, panel, elevated, hover, selection, border, foreground,
  secondary, primary, success, warning, and error roles through Fyne tokens.
- Keep `Font()` delegated to Fyne's default theme.
- Replace hard-coded progress colors in
  `internal/ui/components/table_progress_color.go` with theme tokens.
- Add tests covering important color roles and progress-color mapping.

Acceptance criteria:

- [x] Existing layout still functions.
- [x] Colors are consistent across standard widgets and progress states.
- [x] No light-theme path is introduced.
- [x] Linux build and race tests pass.
- [ ] Windows visual verification remains outstanding; the build and tests run in CI.
- [x] GUI screenshot inspection completed on Linux before Phase 2.

Implementation notes:

- `internal/ui/theme/navy.go` now has separate opaque structural surfaces,
  moonlight interaction overlays, input/scrollbar roles, contrast foreground
  roles, status colors, window borders, and 8/10/12/14 spacing-radius tokens.
- `internal/ui/components/table_progress_color.go` maps progress ranges through
  Fyne semantic theme colors instead of hard-coded hex strings.
- Focused tests cover dark-only variants, surface hierarchy, contrast, interaction
  opacity, radii/spacing, and semantic progress-color mapping.
- Linux screenshot inspection confirmed readable layered surfaces and semantic
  progress/status colors under the active compositor. Existing sidebar overlap
  and the clipped trailing table action remain deferred to Phases 2 and 4.
- The old table's manual progress `Resize()` calls remain intentionally deferred
  to Phase 4, where the table will be replaced with layout-managed custom rows.

### Phase 2: Application shell

Status: **complete; Linux review done, Windows review pending**

- Rebuild the sidebar to the target structure and width.
- Preserve Queued and Scheduled navigation.
- Add footer routes for Settings, History, and About.
- Remove sidebar badges and the non-functional theme icon.
- Replace the top toolbar with the main content header.
- Move Add Download and Search into the new header.
- Keep keyboard shortcuts, especially `Ctrl+N`, `Ctrl+F`, Delete, and Space.
- Add the four summary cards.
- Add radial moonlight ambience behind the main content.
- Replace the detail panel with the target status bar.
- Add concurrent-download control to the status bar.
- Add a minimum window size while preserving saved resizable geometry.
- Replace toolbar-specific tests with tests for the new header and route states.

Acceptance criteria:

- [x] All old toolbar capabilities remain reachable, except redundant visible
  Pause/Resume controls; row menus and Space retain pause/resume behavior.
- [x] History remains reachable from the sidebar footer.
- [x] Search remains visible and `Ctrl+F` focuses it.
- [x] Layout has a content minimum size of 1000x640.
- [x] Summary counts update from real store data.
- [x] Linux GUI screenshot review completed.
- [ ] Windows GUI review is pending.

### Phase 3: Category model and migration

Status: **complete; Linux review done, Windows review pending**

- Replace Videos, Music, and Images with Media.
- Replace Archives with Compressed.
- Keep Documents, Programs, and Other.
- Add an `All` category filter in the sidebar without storing `All` as a record
  category.
- Add a database migration mapping existing values:
  - Videos -> Media
  - Music -> Media
  - Images -> Media
  - Archives -> Compressed
- Do not move files already stored in old category directories.
- Update tests and documentation that use old category names.

Acceptance criteria:

- [x] Existing records remain filterable after migration.
- [x] New URL/filename categorization returns only the new category set.
- [x] Migration is idempotent and preserves all other record data.
- [x] Existing category directories are not renamed or moved.

### Phase 4: Custom download rows

Status: **complete; Linux review done, Windows review pending**

- Replace `widget.Table` with `widget.List` and a fully controlled custom row.
- Remove the 10-unit action column and manual column-proportion machinery.
- Let the trailing action button use a valid natural/minimum size.
- Replace manually resized progress rectangles with layout-managed objects.
- Add filename plus size secondary line, file-type icon, status presentation,
  progress percentage/bar, speed, ETA, and trailing action menu.
- Calculate ETA from remaining bytes and measured speed using a single shared
  duration formatter.
- Preserve row actions, per-download speed limits, clipboard, file/folder open,
  cancel/delete confirmation, and bulk actions.
- Preserve sortable headers for visible columns and add a sort menu for Size and
  Date Added.
- Preserve multi-select behavior with checkboxes visible only in selection mode.
- Audit remaining manual `Resize()` and `Move()` calls in `internal/ui` and keep
  only those required by a documented custom layout.

Acceptance criteria:

- [x] The trailing action button is visually verified on Linux.
- [ ] The trailing action button is visually verified on Windows.
- [x] Remove, pause, resume, cancel, open, copy URL, and speed-limit actions remain wired.
- [x] Rows use layout-managed progress and natural action sizing in code and renderer tests.
- [x] Size and Date Added sorting are available from the row header sort menu.
- [x] Multi-select uses an explicit selection mode with checkboxes hidden otherwise.
- [x] Existing multi-select and sorting behaviors remain covered by tests.
- [ ] Rows are manually checked under Windows 100%, 125%, and 150% display scaling.

### Phase 5: Cross-platform UI safety net

Status: **complete; Linux review done, Windows review pending**

- [x] Add UI-render tests using `fyne.io/fyne/v2/test`.
- [x] Run relevant UI tests on both Linux and Windows CI jobs.
- Assert that action controls have usable minimum sizes, row geometry is valid,
  progress geometry is non-negative, and minimum-window layout does not clip
  required controls.
- [x] Keep Windows installer lifecycle tests.
- Perform manual Windows verification because automated Fyne tests cannot judge
  all visual details.

Acceptance criteria:

- [x] CI exercises GUI layout construction on both supported platforms.
- [ ] A Windows tester verifies action menu, removal, resizing, and common DPI
  scaling manually.

### Documentation closeout

- [x] Update `docs/roadmap.md` and `docs/architecture.md` entries that defer the theme system.
- [x] Update `AGENTS.md` examples and UI references after implementations move.
- Update release notes/changelog only when the redesign is prepared for release.

## Verification Checklist Per Phase

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go build ./...`.
- [ ] Run `go test ./... -race -count=1`.
- [ ] Run targeted UI tests.
- [ ] Start the GUI in the active display session.
- [ ] Inspect the normal desktop viewport.
- [ ] Inspect the minimum supported viewport.
- [ ] Check overflow, clipping, contrast, disabled states, and popup placement.
- [ ] Check console/runtime output for Fyne errors.
- [ ] Confirm unrelated worktree changes were not modified.
- [ ] Stop for visual review before beginning the next phase.

## Out of Scope

- Browser extension redesign.
- Full product rename to `LunaFetch`.
- Light theme or theme switcher.
- Custom borderless window chrome.
- Inter font bundling in the initial phases.
- Photo-realistic moon/cloud/star assets in the initial phases.
- Moving files from legacy category directories.
- Changes to the download engine, API, queue semantics, or storage behavior beyond
  the explicit category migration.

## Change Control

When implementation reveals a conflict with this plan:

1. Record the conflict in this document.
2. Describe the user-visible and cross-platform tradeoff.
3. Obtain a decision before changing a locked requirement.
4. Keep the smallest correct implementation that preserves existing behavior.
