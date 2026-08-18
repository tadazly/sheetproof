---
name: sheetproof-site
description: Update, test, synchronize, or deploy the SheetProof website and public product content. Use for site changes, product facts, changelogs, or user-visible app changes that require public documentation. Do not use for formal version releases.
---

# Maintain the SheetProof site

## Establish scope

1. Read `../../../AGENTS.md` and run `git status --short` before editing.
2. Treat `../../../product/product.json`, `../../../product/changelog/releases.json`, and
   `../../../product/changelog/{en,zh-CN,ja}.json` as the public facts.
3. Record only user-visible behavior and fixes. Exclude prompts, internal tasks, test plans,
   implementation chatter, private paths, credentials, and infrastructure identities.
4. For deployment, read `../../../docs/deployment.md`. Do not load release instructions unless the
   request is a formal version; `sheetproof-release` owns that workflow and its confirmation pause.

## Update and verify

- Update the facts and localized content rather than hand-editing generated version text.
- Run the content synchronizer, inspect its diff, and keep README, CHANGELOG, desktop facts, and site
  content consistent where the facts generator owns them.
- Validate the website with:

```bash
node scripts/sync-product-content.mjs
cd site
npm run lint
npm test
```

`npm test` performs the static build and rendered-page checks. If site implementation changed beyond
generated content, run any additional targeted test or type check that the affected area requires.

## Deploy when delivery requires it

The production site is the static Caddy deployment at `https://sheetproof.luyilabs.com/`; do not add a
production Node process or use the former Sites URL without an explicit hosting decision.

For a completed user-visible feature, fix, or website change, deploy with
`../../../scripts/deploy-site-lightsail.ps1` after local checks pass. The SSH target must be supplied at
runtime by the maintainer; never infer it from history or store it in the repository or publishable logs.

Verify the origin, Cloudflare public routes, important assets and downloads, TLS, and all existing virtual
hosts on the same Caddy instance. Preserve the deployment script's rollback path. When adding or changing
Caddy configuration, back it up, format and validate the candidate, reload only after validation, and
restore the backup if reload fails.

Do not deploy a formal release candidate before the explicit confirmation required by
`sheetproof-release`.
