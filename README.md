<!-- Generated from product facts and locale content. Do not edit directly. -->

English | [简体中文](README.zh-CN.md) | [日本語](README.ja.md)

# SheetProof

> Review and apply XLSX changes in Git.

SheetProof compares .xlsx workbooks in a Git worktree with validated Git revisions, aligns records by key, and applies approved changes to the editable left workbook.

For developers and content teams who review versioned configuration workbooks. Files are processed locally, and Excel is not required.

[![Key-aligned character configuration comparison](site/public/screenshots/en/key-alignment.png)](site/public/screenshots/en/key-alignment.png)

_The demo uses localized character growth, stage reward, and skill tuning workbooks._

## Why SheetProof

XLSX is a compressed OOXML package. Text diffs confuse serialization noise with workbook changes and cannot selectively merge cells in context. SheetProof reads workbook semantics into synchronized grids and keeps the user in control of what enters the left result.

## Key alignment

SheetProof detects a shared `id` header or one unambiguous `*ID` header. You can also choose a key column from a column header. Records with a reliable key are aligned; ambiguous records fall back to physical rows without disabling alignment for the rest of the sheet. One-sided records remain near their shared neighbors.

## Current features

- **Workbook-aware comparison**: Compare worksheets, values, formulas, explicit blanks, and types. Display formatting does not determine whether cells are equal.
- **Side-by-side review**: Use synchronized virtual grids, focused navigation, and scrollbar markers for added, deleted, modified, and conflicting data.
- **Current-sheet row filters**: Combine added, deleted, modified, and conflict filters without losing the source row being reviewed.
- **Controlled merge and undo**: Write only selected cells or rows to the left; edits and merge operations remain undoable.
- **Hold to compare before and after**: Hold Before/After or Tab in a grid to see the left workbook exactly as it was opened.
- **Local Git repository mode**: Read the real worktree against a validated local or remote-tracking reference without checkout, fetch, or branch switching.
- **Guarded local saving**: Detect external changes, write a temporary file in the same directory, reopen it for validation, then replace the original atomically.

## Download

**0.3.0 Preview**

- [Windows amd64](https://github.com/tadazly/sheetproof/releases/download/v0.3.0/SheetProof-windows-amd64.exe)
- [macOS universal](https://github.com/tadazly/sheetproof/releases/download/v0.3.0/SheetProof-macos-universal.zip)
- [SHA256SUMS.txt](https://github.com/tadazly/sheetproof/releases/download/v0.3.0/SHA256SUMS.txt)
- [Source](https://github.com/tadazly/sheetproof/archive/refs/tags/v0.3.0.zip)

The current Windows build is unsigned. The macOS build is unsigned and not notarized. Download only from GitHub Releases and verify SHA-256.

## Quick start

Open a local Git repository, or choose two .xlsx workbooks. The left side is the only writable source; the right side is always read-only.

## Git repository mode

Repository mode reads the real worktree on the left and a validated Git object on the right. It does not fetch, checkout, switch, stage, commit, or push.

## Direct-file mode

Use the welcome screen or run `sheetproof compare --left current.xlsx --right target.xlsx`.

## UGit

Place the app in a stable location, then use Configure UGit. SheetProof replaces only *.xlsx diff and merge entries and verifies the result.

## CLI

Text output follows `--lang en|zh-CN|ja`; JSON keys and enum values remain language-independent.

## Known limitations

Only .xlsx is supported; .xlsm is not supported. SheetProof is not an Excel-level editor and does not guarantee full fidelity for charts, images, pivot tables, external connections, or complex conditional formatting.

## Build

Requires Go 1.24+, Node.js 20+, and Wails 2.10.2. On Windows use `powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build`.

## License

MIT License. Copyright (c) 2026 tadazly.
