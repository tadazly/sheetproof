---
name: ugxlsx-development
description: Continue, review, debug, test, release, deploy, or document the ugxlsx Go/Wails/Vue project and SheetProof website. Use for implementation and handoff work, and whenever the user says to release, publish a formal version, push a version such as v0.1.0, infer the next release version, build GitHub Release executables, or synchronize and deploy the release website to Lightsail/Caddy.
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

## Keep website and releases in sync

- Treat `../../../product/product.json` and `../../../product/changelog.json` as the public facts.
  Record only user-visible behavior and fixes. Do not publish prompts, internal tasks, test plans,
  implementation chatter, private paths, credentials, or infrastructure identities.
- Update the website and changelog for every user-visible feature or fix. Run the content sync,
  website lint, static build, and rendered-page tests before deployment.
- Keep production as a static export served by Caddy at `https://sheetproof.luyilabs.com/` unless
  the maintainer explicitly redesigns hosting. Do not add a production Node process without such a
  decision.
- Deploy each website change with `../../../scripts/deploy-site-lightsail.ps1`, passing the SSH
  target at runtime. Verify the origin, Cloudflare URL, important routes/assets, and existing Caddy
  virtual hosts. Do not publish to the former Sites URL unless the maintainer explicitly switches
  back.
- Read and back up the live Caddy configuration before adding this site. Preserve existing blocks,
  validate the candidate and final configuration, reload only after validation, and restore the
  backup if reload fails.
- Keep server IPs, SSH usernames and aliases, local user paths, local authentication configuration,
  private keys, origin certificates, and tokens out of the repository and logs intended for
  publication.
- Keep `.github/workflows/release.yml` aligned with the product version. A `v*` tag creates a draft
  release with Windows/macOS assets and checksums. Do not describe unsigned or unnotarized builds
  as signed releases.

## Prepare a formal release

Treat requests such as “发布”, “发布正式版本”, “推送正式版本”, or “推送 v0.1.0 版本” as a
two-phase formal release. A request that explicitly says only “发布官网” remains a website-only
deployment.

1. Identify the latest published, non-draft GitHub Release and its tag. Compare that release with
   the intended release state using Git history, code diffs, tests, and current handoff material.
   Include approved release-scope working-tree changes, but keep unrelated user changes out.
   If no prior Release exists, prepare a first-release summary from the implemented product state.
2. Collect only user-visible features, behavior changes, performance improvements, and bug fixes.
   Exclude prompts, internal tasks, test work, screenshots, implementation chatter, and private
   information.
3. Honor an explicit semantic version after removing an optional leading `v`; require it to be
   valid and newer than the previous published version. When no version is supplied, choose the
   highest applicable SemVer change:
   - patch: compatible bug fixes, performance corrections, copy, or documentation;
   - minor: backward-compatible user-facing capabilities or meaningful workflow expansion;
   - major: incompatible stable public contracts or a deliberate new product generation.

   Before 1.0, treat incompatible preview changes as at least a minor bump. For the first public
   Release, default to `0.1.0` unless the product facts already justify a newer version.
4. Update `product/product.json` and `product/changelog.json`, including predictable final asset
   URLs, then run `scripts/sync-product-content.mjs`. Synchronize README, CHANGELOG, CLI/package
   versions, release documentation, handoff material, and all website content. Keep a `0.x` release
   labeled preview unless product maturity independently justifies a stable channel.
5. Run the release-appropriate Go, frontend, website, Wails packaging, privacy, and acceptance
   checks. Exercise affected desktop UI flows when the release contains UI changes. Do not claim
   macOS signing, notarization, or Windows signing unless actually configured and verified. Scan
   the candidate files, diff, generated artifacts, release notes, website output, and relevant Git
   history for credentials, private paths, account/server identities, and personal files. Never
   print a discovered secret value into commentary, logs, commits, release notes, or the report.
   Stop before any push when an actual credential or private file is in scope. Remove it safely and
   require credential revocation/rotation when exposure is possible. Treat history rewriting as a
   separate destructive operation that requires explicit user approval.
6. Present one release-candidate summary containing the previous tag, proposed version and bump
   rationale, curated changelog, included file/change scope, validation results, unsigned status,
   privacy audit result, and the exact planned commit, branch push, tag, GitHub Release, and website
   deployment actions. If sensitive data was found, report its location/category, exposure risk,
   remediation and rotation status without reproducing its value, even when already removed.
   Ask “是否确认正式发布 vX.Y.Z？” and stop. Do not commit, push, tag, publish a Release, update
   production downloads, or deploy the prepared website before explicit confirmation. This pause
   is an intentional exception to the normal same-turn website deployment rule.

## Execute only after confirmation

1. Recheck the worktree, release scope, current/default branch relationship, remote, credentials,
   sensitive-data scan, and tag/Release nonexistence. Never force-push, move an existing tag, expose
   local authentication details, or include unrelated files. Stop if the intended tag points to a
   different commit or the release state has changed since confirmation.
2. Stage only the confirmed scope, create an intentional `Release SheetProof vX.Y.Z` commit when
   needed, and push the release commit without force. Run the GitHub Release workflow manually on
   that commit first and wait for its verification plus Windows/macOS artifacts. If it fails, do
   not create the version tag; report the failure and return to release preparation if changes are
   required.
3. Create and push the confirmed `vX.Y.Z` tag only after the manual workflow succeeds. Wait for the
   tag-triggered workflow. Do not publish anything if verification, either platform build,
   checksums, or Draft Release creation fails.
4. Inspect the Draft Release assets and `SHA256SUMS.txt`. Replace auto-generated notes with the
   curated user-facing changelog so internal commit/task wording is not published. Confirm the tag,
   asset names, download URLs, and checksums, then publish the Draft Release through GitHub.
5. After the Release is publicly downloadable, rerun content synchronization and website lint,
   static build, and rendered-page tests if final URLs or notes changed. Deploy with
   `scripts/deploy-site-lightsail.ps1`, passing the local SSH target only at runtime. Verify the
   origin, Cloudflare public routes, downloads, favicon/Open Graph assets, TLS, and existing Caddy
   virtual hosts. Keep the previous site available through the deployment script rollback path.
6. Report the release commit, tag, public Release URL, assets/checksums, Actions results, official
   website deployment, signing limitations, and final privacy audit. Always state whether sensitive
   data was found. When it was, report the affected location/category and final remediation or
   credential-rotation status without reproducing the sensitive value. A release is complete only
   when both GitHub Release and the official website pass verification.

## Keep handoff material current

When behavior or constraints change, update the relevant parts of:

- `../../../README.md`
- `../../../docs/architecture.md`
- `../../../docs/manual-acceptance.md`
- `../../../docs/iteration-2-handoff.md`
- `../../../docs/new-session-prompt.md` when startup instructions change

Keep `../../../AGENTS.md` concise and stable. Preserve the historical first-iteration handoff.
