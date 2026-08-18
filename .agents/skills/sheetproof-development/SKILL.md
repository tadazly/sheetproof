---
name: sheetproof-development
description: Implement, debug, review, test, or document SheetProof Go/Wails/Vue desktop behavior. Use for repository mode, direct-file mode, core packages, CLI, or desktop UI work. Do not use for formal releases or website-only deployment.
---

# Develop SheetProof

## Establish the baseline

1. Read `../../../AGENTS.md` and run `git status --short --branch` before editing.
2. Inspect the task's code, tests, callers, and current behavior. Preserve unrelated user changes.
3. Use `rg` to find only relevant sections of `../../../docs/architecture.md` and
   `../../../docs/iteration-2-handoff.md`; read whole documents only for a global audit.
4. For desktop integration, packaging, or UI behavior, read
   [references/desktop-delivery.md](references/desktop-delivery.md).
5. For a user-visible feature or fix, also use `../sheetproof-site/SKILL.md`. For a formal version,
   GitHub Release, or release notes, use `../sheetproof-release/SKILL.md` instead.

## Route the change

- Put Git discovery, branch enumeration, status, relative-path validation, and object reads in
  `internal/repository`.
- Put workbook state, diff replacement, edit, merge, undo, and save behavior in
  `internal/app.Session` and the existing core packages.
- Put native dialogs, repository UI state, unsaved-switch confirmation, and Wails methods in
  `controller.go`.
- Put command parsing and validation in `internal/cli`.
- Put view state and interaction in `frontend/src`, without duplicating the Go diff engine.

Preserve both repository mode and direct two-file mode. Reuse the same Session, diff, merge,
history, and storage pipeline for both. Treat `../../../AGENTS.md` as the authoritative invariant
list; consult the relevant architecture section when a change touches one of those boundaries.

## Implement safely

- Prefer generated XLSX fixtures and temporary Git repositories over checked-in binaries.
- Keep right-side Git exports outside the repository, read them as detached comparisons, and clean
  them up. Preserve left edits when only the comparison ref changes.
- Keep request-generation guards around repository, background index, and Region loads. Send only
  bounded viewport regions to the frontend.
- Route writes through Session, merge/history, and safe storage. Never mutate Git state implicitly.
- Let Go own equality, row/column mapping, conflict classification, and diff colors/counts; do not
  duplicate these rules in the frontend.
- When an inline value matches the right raw value, preserve the right text/number type so equal
  visible values do not remain different because of inference.

## Implement and verify

Add focused regression coverage with each behavior change. Run checks that match the changed
surface and report skipped gates explicitly.

For Go changes:

```bash
gofmt -w <changed-go-files>
GOCACHE=/tmp/sheetproof-go-cache go test ./...
```

For frontend changes:

```bash
cd frontend
npm run lint
npm run typecheck
npm run test
npm run build
```

For core or concurrency changes, also run:

```bash
GOCACHE=/tmp/sheetproof-go-cache go vet ./...
GOCACHE=/tmp/sheetproof-go-cache go test -race ./...
```

For instruction- or documentation-only changes, run the applicable validator plus
`git diff --check`; do not run unrelated application builds merely to create activity. Desktop and
UI delivery gates are defined in [references/desktop-delivery.md](references/desktop-delivery.md).

## Keep handoff material current

When behavior, commands, or constraints change, update only the relevant parts of:

- `../../../README.md`
- `../../../docs/architecture.md`
- `../../../docs/manual-acceptance.md`
- `../../../docs/iteration-2-handoff.md`
- `../../../docs/new-session-prompt.md` when startup instructions change

Keep `../../../AGENTS.md` concise and stable. Preserve `docs/iteration-1-handoff.md` as history.
