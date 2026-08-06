import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";

const routeList = ["/", "/features", "/guide", "/download", "/changelog", "/zh-CN", "/zh-CN/features", "/zh-CN/guide", "/zh-CN/download", "/zh-CN/changelog", "/ja", "/ja/features", "/ja/guide", "/ja/download", "/ja/changelog"];
const routes = new Map(routeList.map((route) => [route, `../dist/client${route === "/" ? "" : route}/index.html`]));

async function render(path = "/") {
  const file = routes.get(path);
  assert.ok(file, `unknown route ${path}`);
  return readFile(fileURLToPath(new URL(file, import.meta.url)), "utf8");
}

test("exports localized home pages with independent product copy", async () => {
  const [en, zh, ja] = await Promise.all([render(), render("/zh-CN"), render("/ja")]);
  assert.match(en, /Review XLSX changes/);
  assert.match(en, /in Git—then apply/);
  assert.equal([...en.matchAll(/class="hero-title-line"/g)].length, 3);
  assert.match(en, /Key: Map ID/);
  assert.match(zh, /审阅 Git 中的 XLSX，/);
  assert.match(zh, /只应用确认过的修改。/);
  assert.equal([...zh.matchAll(/class="hero-title-line"/g)].length, 2);
  assert.match(zh, /主键：地图 ID/);
  assert.match(ja, /Git 上の XLSX 変更を確認。/);
  assert.match(ja, /承認した変更だけを反映。/);
  assert.equal([...ja.matchAll(/class="hero-title-line"/g)].length, 2);
  assert.match(ja, /キー：マップ ID/);
  assert.doesNotMatch(`${en}\n${ja}`, /让 Git 中的 XLSX|主键：地图ID/);
});

test("exports localized feature, guide, download, and changelog pages", async () => {
  const expected = [
    ["/features", /Review, apply, and save XLSX changes from Git/], ["/guide", /Review and apply changes in three steps/], ["/download", /Windows preview/], ["/changelog", /Release notes/],
    ["/zh-CN/features", /比较、应用并保存 Git 中的 XLSX 修改/], ["/zh-CN/guide", /三步完成差异审阅/], ["/zh-CN/download", /Windows 预览版/], ["/zh-CN/changelog", /更新日志/],
    ["/ja/features", /Git 上の XLSX を比較し、必要な変更だけを保存/], ["/ja/guide", /3 ステップで差分を確認/], ["/ja/download", /Windows プレビュー/], ["/ja/changelog", /リリースノート/],
  ];
  for (const [route, pattern] of expected) assert.match(await render(route), pattern, route);
});

test("static exports use the correct document language", async () => {
  assert.match(await render(), /<html lang="en">/);
  assert.match(await render("/zh-CN"), /<html lang="zh-CN">/);
  assert.match(await render("/ja"), /<html lang="ja">/);
});

test("download copy keeps signing and notarization warnings in every locale", async () => {
  const [en, zh, ja] = await Promise.all([render("/download"), render("/zh-CN/download"), render("/ja/download")]);
  assert.match(en, /unsigned/); assert.match(en, /not notarized/);
  assert.match(zh, /未经签名/); assert.match(zh, /未经公证/);
  assert.match(ja, /未署名/); assert.match(ja, /公証を受けていない/);
  for (const html of [en, zh, ja]) assert.match(html, /SHA-256/);
});

test("feature pages preserve the 90, 16, and 15 difference scenario", async () => {
  for (const route of ["/features", "/zh-CN/features", "/ja/features"]) {
    const html = await render(route);
    assert.match(html, /90/); assert.match(html, /16/); assert.match(html, /15/);
    assert.match(html, /physical-rows\.png/); assert.match(html, /key-alignment\.png/); assert.match(html, /merge-result\.png/);
  }
});

test("guide documents actual shortcuts, the in-app Help entry, and UGit entries", async () => {
  for (const route of ["/guide", "/zh-CN/guide", "/ja/guide"]) {
    const html = await render(route);
    assert.match(html, /1(?:–|～)4/); assert.match(html, /5/); assert.match(html, /SpreadsheetCompare/); assert.match(html, /Custom/);
    assert.match(html, /Ctrl \/ ⌘ \+ Shift \+ S/);
    assert.match(html, /Ctrl \/ ⌘ \+ F/); assert.match(html, /F3/); assert.match(html, /Shift \+ F3/);
    assert.match(html, /Help|帮助|ヘルプ/);
  }
});

test("localized changelogs include the unreleased in-app Help entry", async () => {
  const [en, zh, ja] = await Promise.all([render("/changelog"), render("/zh-CN/changelog"), render("/ja/changelog")]);
  assert.match(en, /In-app help/);
  assert.match(zh, /应用内帮助/);
  assert.match(ja, /アプリ内ヘルプ/);
});

test("screenshots remain accessible through the standalone viewer", async () => {
  const [html, viewer, css] = await Promise.all([render("/features"), readFile(fileURLToPath(new URL("../app/components/ScreenshotViewer.tsx", import.meta.url)), "utf8"), readFile(fileURLToPath(new URL("../app/globals.css", import.meta.url)), "utf8")]);
  assert.match(html, /role="dialog"/);
  assert.match(viewer, /addEventListener\("wheel", handleWheel, \{ passive: false \}\)/);
  assert.match(viewer, /onPointerMove=\{handlePointerMove\}/);
  assert.match(viewer, /gesturestart/);
  assert.match(css, /\.lightbox-stage \{[^}]*touch-action: none;/);
  assert.doesNotMatch(css, /user-scalable\s*=\s*no/i);
});

test("mobile navigation, canonical repository, and copyright are present", async () => {
  for (const route of routes.keys()) {
    const html = await render(route);
    assert.match(html, /aria-controls="mobile-site-menu"/);
    assert.match(html, /https:\/\/github\.com\/tadazly\/sheetproof/);
    assert.match(html, /Copyright \(c\) 2026 tadazly/);
  }
});

test("generated facts remain synchronized with the product source", async () => {
  const [sourceText, generatedText, readme, readmeZh, readmeJa] = await Promise.all([
    readFile(fileURLToPath(new URL("../../product/product.json", import.meta.url)), "utf8"),
    readFile(fileURLToPath(new URL("../app/content/generated/content.json", import.meta.url)), "utf8"),
    readFile(fileURLToPath(new URL("../../README.md", import.meta.url)), "utf8"),
    readFile(fileURLToPath(new URL("../../README.zh-CN.md", import.meta.url)), "utf8"),
    readFile(fileURLToPath(new URL("../../README.ja.md", import.meta.url)), "utf8"),
  ]);
  const source = JSON.parse(sourceText);
  assert.deepEqual(JSON.parse(generatedText).facts, source);
  assert.match(readme, /## Key alignment/);
  assert.match(readmeZh, /## 主键对齐/);
  assert.match(readmeJa, /## キー列によるレコード照合/);
  assert.doesNotMatch(`${readmeZh}\n${readmeJa}`, /SheetProof processes workbooks locally/);
});

test("all internal navigation links resolve to a localized product route", async () => {
  const known = new Set(routes.keys());
  for (const route of routes.keys()) {
    const hrefs = [...(await render(route)).matchAll(/href="([^"]+)"/g)].map((match) => match[1]);
    for (const href of hrefs) {
      if (!href.startsWith("/") || /^\/(?:assets|brand|screenshots)\//.test(href)) continue;
      const path = (href.split("#", 1)[0] || "/").replace(/\/$/, "") || "/";
      assert.ok(known.has(path), `${route} links to unknown route ${href}`);
    }
  }
});
