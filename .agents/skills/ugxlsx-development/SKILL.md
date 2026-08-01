---
name: ugxlsx-development
description: Continue, review, debug, test, or document the ugxlsx Go/Wails/Vue project, especially its local Git repository mode, XLSX comparison, merge, inline editing, undo, safe save, CLI, asynchronous UI state, and release validation. Use for any implementation or handoff work inside the ugxlsx repository.
---

# Develop ugxlsx

## Establish the baseline

1. Read `../../../AGENTS.md` completely.
2. Read `../../../docs/iteration-2-handoff.md`, `../../../README.md`, and
   `../../../docs/architecture.md` completely.
3. Read `../../../docs/manual-acceptance.md` when changing user-visible behavior.
4. Run `git status --short` before editing. Treat existing changes as user work.
5. Do not reset or overwrite any uncommitted second- or third-iteration baseline changes.

Use `docs/iteration-1-handoff.md` only for first-iteration history. Do not treat it as the
current implementation state.

## Route the change

- Put Git discovery, branch enumeration, status, relative-path validation, and object reads in
  `internal/repository`.
- Put workbook state, diff replacement, edit, merge, undo, and save behavior in
  `internal/app.Session` and the existing core packages.
- Put native dialogs, repository UI state, unsaved-switch confirmation, and Wails methods in
  `controller.go`.
- Put command parsing and validation in `internal/cli`.
- Put view state and interaction in `frontend/src`, without duplicating the Go diff engine.

Preserve both repository mode and direct two-file mode. Reuse the same Session and storage
pipeline for both.

## Preserve safety boundaries

- Read the repository left side from the actual worktree path.
- Read the right side from validated Git refs without checkout, switch, or fetch.
- Pass Git arguments as an array with `git -C <root>` and a timeout; never build a shell command.
- Keep exported ref files outside the repository and clean them up.
- Preserve left-side edits when only the comparison ref changes.
- Represent a missing right file as an explicit detached comparison state, not an empty workbook.
- Keep generation/request guards around repository and Region loads.
- Send only viewport regions to the frontend.
- Route every write through merge/history and safe storage.
- Never add, commit, push, restore, or otherwise mutate Git state automatically.

## Preserve current interaction semantics

- Keep the repository sidebar tabs for files, differing workbooks, and sheets/differences.
- Keep the compact searchable XLSX-only tree.
- Keep the differing-workbook tree on the cached semantic index. Git may select paths present
  on both sides as candidates, but background comparison must apply the Go equality semantics;
  never show unverified or semantically equal workbooks. Calculate exact per-workbook cell counts
  only after opening a workbook. Keep background indexing cancellable so application shutdown
  does not wait for all remaining workbooks, while still cleaning exported temporary files.
- Preserve per-repository comparison-ref preferences and default to the current branch's matching
  remote ref when no valid preference exists.
- Edit the left grid in place on double-click; submit with Enter or blur and cancel with Esc.
- Keep the bottom two-line value/type inspector instead of the legacy edit form.
- Let the Go diff result determine highlighting.
- Use the Go row classification for added, deleted, modified, and conflict colors/counts.
  Follow split Git colors for modified cells (old/left red, new/right green), use orange for
  conflicts, and preserve each semantic background under the blue selection border.
- Keep conflict row copy/overwrite/append operations in Session merge/history. Auto IDs continue
  from the largest integer ID on the left; specified IDs are one per selected source row.
- Keep conflict resolution markers in Session state, remove them with the corresponding undo,
  show the resolution on the right source row, and render appended left target rows as additions.
- When an inline value exactly matches the right raw value, preserve the right text/number type
  so visibly equal numeric text does not remain different only because of inference.

## Implement and verify

Add focused regression coverage with each behavior change. Prefer temporary Git repositories and
generated XLSX fixtures over checked-in binary fixtures.

Run the relevant checks:

```bash
GOCACHE=/tmp/ugxlsx-go-cache go test ./...

cd frontend
npm run lint
npm run typecheck
npm run test
npm run build
```

For core or concurrency changes, also run:

```bash
GOCACHE=/tmp/ugxlsx-go-cache go vet ./...
GOCACHE=/tmp/ugxlsx-go-cache go test -race ./...
```

For desktop delivery, run:

```bash
# Windows
powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build

# macOS/Linux
GOCACHE=/tmp/ugxlsx-go-cache \
  go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build
```

The Windows launcher is offline-first: it checks a matching installed/project-local Wails CLI, then
builds v2.10.2 from the project's `go.mod` and local module cache with `GOPROXY=off`. It also uses
the ignored project-local `build/cache/go-build` cache so stale cross-session ACLs in the default
Windows Go cache cannot break child commands. Always use it on Windows before requesting network
access or claiming that Wails is absent. On macOS/Linux, check `command -v wails` and the exact
versioned `GOMODCACHE` directory first. A failed
`go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 ...` lookup is not evidence that the CLI
or module is missing, because Go may contact the proxy for metadata even with cached sources.
Only report a local absence after the launcher confirms that the versioned module directory is
missing, or that its dependencies are genuinely incomplete.

For every UI layout, styling, visibility, or interaction change, real desktop GUI acceptance is
a mandatory delivery gate. Build and launch the current Wails desktop artifact, exercise the
affected flow with representative data, capture screenshots, and inspect the rendered layout and
state before handing the change to the user. Frontend unit tests, browser-only DOM checks, and a
successful Wails build do not substitute for this acceptance. If the environment cannot complete
the desktop check, keep working on the acceptance setup or report a blocker; do not hand an
unverified UI change to the user for trial-and-error.

On Windows, use `scripts/capture-wails-window.ps1` for the normal screenshot acceptance path. It
starts the exact built executable with test arguments, waits for its window, applies DPI-aware fixed
bounds, captures the complete window, and requests a clean close in one command. Prefer this path
over manually locating, resizing, capturing, cropping, and closing the window. Only fall back to
manual capture when the affected interaction cannot be represented at launch; keep the reusable
launch-and-capture step in the flow even then.

Report automated tests, desktop packaging, and real GUI acceptance separately. Do not claim a
manual flow passed unless it was actually exercised and visually inspected.

## Keep handoff material current

When behavior or constraints change, update the relevant parts of:

- `../../../README.md`
- `../../../docs/architecture.md`
- `../../../docs/manual-acceptance.md`
- `../../../docs/iteration-2-handoff.md`
- `../../../docs/new-session-prompt.md` when startup instructions change

Keep `../../../AGENTS.md` concise and stable. Preserve the historical first-iteration handoff.
