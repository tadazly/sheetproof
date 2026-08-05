<!-- Generated from product facts and locale content. Do not edit directly. -->

English | [简体中文](CHANGELOG.zh-CN.md) | [日本語](CHANGELOG.ja.md)

# Changelog

User-visible SheetProof changes.

## 0.4.1 — 2026-08-05

_Preview · Cleaner edits and more precise copying_

Unchanged cell edits no longer create unsaved state, and Copy to left handles only real differences.

- Leaving a cell edit without changing its content no longer creates an unsaved state or an extra undo step.
- Copy to left now counts and applies only cells with actual semantic differences; equal selections remain available in place with a zero, disabled action.

## 0.4.0 — 2026-08-05

_Preview · English, Simplified Chinese, and Japanese throughout_

The desktop app, CLI, documentation, and website now share a complete three-language experience, with persistent language choice and localized product examples.

- The desktop interface, settings, native confirmations, difference states, merge actions, and generated demonstration workbooks are available in English, Simplified Chinese, and Japanese.
- CLI help, status text, and errors follow --lang en|zh-CN|ja, while JSON keys and enum values remain stable for automation.
- The website, README, changelog, SEO metadata, navigation, and product screenshots now have matching English, Simplified Chinese, and Japanese versions.
- Language choice persists across restarts. An explicit --lang affects only that launch, and Follow system can return to operating-system detection.
- Language, UGit configuration, cache clearing, and saved-app-data clearing now share one responsive settings window with localized confirmations.
- Repository mode now stays in its startup loading state until a preselected XLSX file and revision are fully open, so the comparison cannot be replaced by an empty workspace.
- The mobile site navigation uses native browser disclosure, so it remains available before scripts start.
- Product screenshots open in a dedicated viewer with desktop wheel zoom and mobile pinch and drag. Closing, scroll restoration, and gesture locking were corrected.
- The home-page usage examples now continue rotating with their original lightweight transitions regardless of the browser's reduced-motion preference.

## 0.3.0 — 2026-08-04

_Preview · More accurate and faster review of large workbooks_

Keyed records stay near their original neighbors, key columns can be detected or selected from a column header, and scrollbars show difference locations.

- Horizontal and vertical scrollbars now mark added, deleted, modified, and conflicting positions with the existing semantic colors.
- Large difference views use bounded viewport reads and filtered-row mappings; external-file polling pauses while the app is hidden.
- Direct-file, repository, and UGit/Git comparisons align records when both sides share an id header or one unambiguous *ID header. Ambiguous headers fall back to physical rows.
- Records present on only one side remain between their neighboring shared keys instead of moving to the end. A key column can also be set or cleared from the column header.

## 0.2.0 — 2026-08-03

_Preview · Safer imports, reloads, and bulk editing_

Repositories can be dropped into the app, externally changed workbooks can be reloaded safely, and selections can be cleared as one undoable action.

- Fixed the blank WebView caused by dropping a repository before the app had opened; repository folders can now be selected or dropped.
- Large repository imports immediately show progress, and failures remain visible inside the import dialog.
- The app detects external changes: read-only sources reload after notice, while an editable left workbook asks before discarding session edits.
- Ctrl/Command+A selects the current worksheet, and Backspace/Delete clears the editable selection as one undoable operation.

## 0.1.0 — 2026-08-02

_Preview · First preview_

Review and merge .xlsx differences from direct files, Git repositories, and UGit.

- Open two .xlsx files directly, or compare a worktree file with an existing local or remote-tracking Git reference.
- Differences include values, formulas, cell types, and worksheet order; equivalent shared and inline strings are normalized.
- The repository list includes only workbooks with confirmed semantic differences; one corrupt, encrypted, or legacy file no longer stops the rest.
- Combine added, deleted, modified, and conflict filters for the current worksheet while keeping the reviewed source row in view.
- Git merge sessions align reliable unique IDs and use the common base to distinguish file-level conflicts from changes made by both sides.
- UGit worktree comparisons keep the real worktree editable on the left and the Git snapshot read-only; two snapshots stay read-only.
- Conflicting integer IDs can be appended with automatic or specified IDs; text IDs retain their values, and ordinary modified rows do not offer append actions.
- Hold Before/After or Tab in a grid to see the left workbook as it was opened, then release to return to the current result.
- Background Git commands no longer open console windows on Windows, and native confirmations have compatibility fallbacks.
- Edits, cell copies, row overwrites, and appends can be undone. Saving writes only the left file and never stages, commits, pushes, or switches branches.
- The source is licensed under MIT and the repository and release links now use tadazly/sheetproof.
- The website provides full navigation on phones and tablets and explains its focus on versioned configuration and data workbooks.
- The guide documents row-filter shortcuts as 1–4 for categories and 5 for all differences.
- The website fixes page overflow at mobile zoom and very narrow widths, wraps command examples, and documents UGit configuration.
