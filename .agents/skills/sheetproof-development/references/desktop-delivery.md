# Desktop delivery

Read this reference only for desktop integration, UI, native interaction, packaging, or delivery work.

## Build

On Windows, use the offline-first launcher before requesting network access or claiming Wails is absent:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build
```

It checks a matching installed or project-local Wails CLI, then the fixed v2.10.2 module and dependency
cache with `GOPROXY=off`, and uses the ignored `build/cache/go-build` cache. A failed standalone
`go run ...@v2.10.2` lookup is not proof that Wails or its local sources are missing.

On macOS/Linux, first check `command -v wails` and the exact versioned `GOMODCACHE` directory, then run:

```bash
GOCACHE=/tmp/sheetproof-go-cache \
  go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build
```

## Real GUI acceptance

Every UI layout, style, visibility, or interaction change must pass the affected flow in the current
Wails desktop artifact. Use representative data, perform the real interaction, capture fresh screenshots,
inspect the rendered layout and state, and close all started processes. Frontend tests, browser-only DOM
checks, and a successful Wails build are separate evidence and do not substitute for desktop acceptance.

On Windows, prefer `scripts/capture-wails-window.ps1` for launch-time states. It starts the exact artifact,
applies DPI-aware bounds, captures the window, and requests a clean close. Use manual Computer Use only
when the interaction cannot be represented at launch, while retaining deterministic launch/capture where
possible.

Read only the relevant heading in `../../../docs/manual-acceptance.md`; do not load the entire manual by
default. Report automated checks, packaging, and real GUI acceptance as separate results. If the environment
cannot complete required GUI acceptance after in-scope recovery attempts, report a blocker instead of
handing unverified UI work to the user.
