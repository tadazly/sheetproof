import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { generateReleaseNotes, renderReleaseNotes } from "./generate-release-notes.mjs";

test("generates the current release in English with localized changelog links at the end", async () => {
  const notes = await generateReleaseNotes();

  assert.match(notes, /^SheetProof 0\.8\.0 Preview — Frozen headers, keyboard editing, system clipboard actions, and external open commands now fit everyday configuration review\./);
  assert.match(notes, /## What's new[\s\S]*selected row or column headers[\s\S]*other parse failures remain isolated/);
  assert.match(notes, /## Downloads[\s\S]*SheetProof-windows-amd64\.exe[\s\S]*SheetProof-macos-universal\.zip[\s\S]*SHA256SUMS\.txt/);
  assert.match(notes, /The Windows build is unsigned\. The macOS build is unsigned and not notarized\./);
  assert.match(notes, /## Other languages\n\n- \[简体中文更新日志\]\(https:\/\/sheetproof\.luyilabs\.com\/zh-CN\/changelog\/\)\n- \[日本語のリリースノート\]\(https:\/\/sheetproof\.luyilabs\.com\/ja\/changelog\/\)\n$/);
  assert.doesNotMatch(notes, /\?{3,}/);
});

test("rejects a tag that does not match the product version", () => {
  assert.throws(() => renderReleaseNotes({
    facts: { product: { version: "1.0.0" } },
    releases: { releases: [] },
    english: {},
    version: "1.0.1",
  }), /does not match release/);
});

test("keeps Wails package metadata aligned with the product facts", async () => {
  const [facts, wails] = await Promise.all([
    readFile(new URL("../product/product.json", import.meta.url), "utf8").then(JSON.parse),
    readFile(new URL("../wails.json", import.meta.url), "utf8").then(JSON.parse),
  ]);

  assert.equal(wails.info.productName, facts.product.name);
  assert.equal(wails.info.productVersion, facts.product.version);
  assert.equal(wails.info.copyright, facts.product.copyright);
});
