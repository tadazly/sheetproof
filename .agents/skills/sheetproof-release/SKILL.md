---
name: sheetproof-release
description: Prepare or execute a formal SheetProof version, GitHub Release, release notes, or release assets. Use when the user says release, publish a version, push vX.Y.Z, repair Release notes, or build official release executables. Do not use for ordinary commits or website-only deployment.
---

# Release SheetProof

Formal releases are a two-phase, high-assurance workflow. Read `../../../AGENTS.md`, run
`git status --short`, and read `../../../docs/releasing.md` before preparing a release. Use
`../../../docs/iteration-2-handoff.md` only for relevant changes since the previous public tag; locate
sections or commits first instead of reading the cumulative file in full.

## Prepare the candidate

1. Identify the latest published, non-draft GitHub Release and compare its tag with the intended state,
   including approved working-tree changes while excluding unrelated user work.
2. Validate an explicit SemVer or infer the highest applicable bump: compatible fixes are patch,
   backward-compatible features are minor, and incompatible stable contracts are major. Before 1.0,
   incompatible preview changes are at least minor.
3. Curate only user-visible features, behavior changes, performance improvements, and fixes.
4. Update `../../../product/product.json`, `../../../product/changelog/releases.json`, and the localized
   `../../../product/changelog/{en,zh-CN,ja}.json` facts, then run the content synchronizer.
5. Run the release-appropriate Go, frontend, site, Wails, GUI, release-note, and privacy checks described
   in `../../../docs/releasing.md`. Keep Windows signing, macOS signing, and notarization claims factual.
6. Scan candidate files, diffs, relevant history, generated notes, site output, and final desktop artifacts
   for credentials, private paths, account/server identities, and personal files. Never print a discovered
   secret value; stop before push until exposure is remediated and credentials are rotated when necessary.

For Release Notes, run the generator and its test, pass the UTF-8 result via `--notes-file`, and read the
public Release back after any write. Do not use `--generate-notes` or a PowerShell text pipeline. A notes
repair authorizes only the body change unless the user explicitly expands the scope.

Present one candidate summary with the previous tag, proposed version and rationale, curated changelog,
included scope, validation, signing limits, privacy result, and exact planned commit/push/tag/Release/site
actions. Ask `是否确认正式发布 vX.Y.Z？` and stop. Before that confirmation, do not commit, push, tag,
publish a Release, update production downloads, or deploy the candidate site.

## Execute only after confirmation

1. Recheck the worktree, confirmed scope, branch/default-branch relationship, remote, authentication,
   privacy result, and tag/Release nonexistence. Never force-push or move an existing tag.
2. Commit and push only the confirmed scope. Manually run the Release workflow on that commit and wait
   for verified Windows and macOS artifacts before creating a tag.
3. Create and push the confirmed annotated tag only after the commit workflow passes. Wait for the
   tag-triggered workflow and stop if validation, either build, checksums, or Draft Release creation fails.
4. Inspect the Draft Release, assets, `SHA256SUMS.txt`, generated English notes, localized changelog links,
   tag, target commit, and public download URLs before publishing it.
5. After the Release is public, use `sheetproof-site` to synchronize final URLs, rebuild/test the site,
   deploy through Lightsail/Caddy, and verify origin, Cloudflare routes, downloads, assets, TLS, and existing
   virtual hosts.
6. Report commit, tag, public Release, artifacts/checksums, Actions results, site deployment, signing limits,
   and the final privacy conclusion. A formal release is complete only when the public Release and official
   site both pass verification.
