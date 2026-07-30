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
5. Do not reset or overwrite the uncommitted second-iteration baseline.

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

- Keep the repository sidebar tabs for files and sheets/differences.
- Keep the compact searchable XLSX-only tree.
- Edit the left grid in place on double-click; submit with Enter or blur and cancel with Esc.
- Keep the bottom two-line value/type inspector instead of the legacy edit form.
- Let the Go diff result determine highlighting.
- Show selected equal cells in blue. Show selected differences with both the yellow/orange
  difference cue and blue selection border.
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
GOCACHE=/tmp/ugxlsx-go-cache \
  go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build
```

Report automated tests, desktop packaging, and real GUI acceptance separately. Do not claim a
manual flow passed unless it was actually exercised.

## Keep handoff material current

When behavior or constraints change, update the relevant parts of:

- `../../../README.md`
- `../../../docs/architecture.md`
- `../../../docs/manual-acceptance.md`
- `../../../docs/iteration-2-handoff.md`
- `../../../docs/new-session-prompt.md` when startup instructions change

Keep `../../../AGENTS.md` concise and stable. Preserve the historical first-iteration handoff.
