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

test("download copy explains unsigned first launch before optional verification in every locale", async () => {
  const [en, zh, ja] = await Promise.all([render("/download"), render("/zh-CN/download"), render("/ja/download")]);
  assert.match(en, /unsigned/); assert.match(en, /More info/); assert.match(en, /Run anyway/); assert.match(en, /Open Anyway/);
  assert.match(zh, /未签名/); assert.match(zh, /更多信息/); assert.match(zh, /仍要运行/); assert.match(zh, /仍要打开/);
  assert.match(ja, /未署名/); assert.match(ja, /詳細情報/); assert.match(ja, /このまま開く/);
  for (const html of [en, zh, ja]) {
    assert.match(html, /Smart App Control/);
    assert.match(html, /Gatekeeper/);
    assert.match(html, /SHA256SUMS\.txt/);
    assert.match(html, /Get-FileHash/);
    assert.match(html, /shasum -a 256/);
  }
  assert.ok(en.indexOf("Run anyway") < en.indexOf("Optional download verification"));
  assert.ok(zh.indexOf("仍要运行") < zh.indexOf("下载文件校验（可选）"));
  assert.ok(ja.indexOf("このまま開く") < ja.indexOf("ダウンロードファイルの確認（任意）"));
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
    assert.match(html, /Ctrl \/ ⌘ \+ C/); assert.match(html, /Ctrl \/ ⌘ \+ V \/ X/);
    assert.match(html, /Arrow keys|方向键|矢印キー/);
    assert.match(html, /mouse wheel|鼠标滚轮|マウスホイール/);
    assert.match(html, /Help|帮助|ヘルプ/);
  }
});

test("v0.8 feature and guide pages cover the complete desktop workflow", async () => {
  const [featureEn, featureZh, featureJa, guideEn, guideZh, guideJa] = await Promise.all([
    render("/features"), render("/zh-CN/features"), render("/ja/features"),
    render("/guide"), render("/zh-CN/guide"), render("/ja/guide"),
  ]);
  assert.match(featureEn, /Pin any selected rows/); assert.match(featureEn, /UI text size and per-sheet zoom/); assert.match(featureEn, /paste and cut only on the left/);
  assert.match(featureZh, /任意选中的行固定在上方/); assert.match(featureZh, /UI 字体和每个 Sheet 的缩放/); assert.match(featureZh, /粘贴和剪切仅限左侧/);
  assert.match(featureJa, /選択した行を上部/); assert.match(featureJa, /UI の文字サイズとシートごとのズーム/); assert.match(featureJa, /貼り付けと切り取りは左側のみ/);
  assert.match(guideEn, /right-click selected row or column headers/); assert.match(guideEn, /default spreadsheet app/);
  assert.match(guideZh, /右键所选行头或列头/); assert.match(guideZh, /默认表格工具/);
  assert.match(guideJa, /行見出しまたは列見出しを右クリック/); assert.match(guideJa, /既定の表計算アプリ/);
});

test("localized changelogs include v0.8, column alignment, whole-column copy, and in-app Help", async () => {
  const [en, zh, ja] = await Promise.all([render("/changelog"), render("/zh-CN/changelog"), render("/ja/changelog")]);
  for (const html of [en, zh, ja]) assert.match(html, /v0\.8\.0/);
  assert.match(en, /unexpected exit/); assert.match(zh, /无操作闪退/); assert.match(ja, /予期せず終了/);
  assert.match(en, /Column-aware comparison and merging/); assert.match(en, /complete columns/);
  assert.match(zh, /按列语义对比与合并/); assert.match(zh, /整列复制/);
  assert.match(ja, /列を対応付けた比較と反映/); assert.match(ja, /列全体/);
  assert.match(en, /In-app help/);
  assert.match(zh, /应用内帮助/);
  assert.match(ja, /アプリ内ヘルプ/);
  assert.match(en, /Switching between Settings and mouse-wheel worksheet zoom/);
  assert.match(zh, /在设置和主界面滚轮之间切换表格缩放/);
  assert.match(ja, /設定とメイングリッドのホイール操作を切り替えても/);
  assert.match(en, /Local rolling diagnostic logs/);
  assert.match(zh, /本地轮转诊断日志/);
  assert.match(ja, /ローカル診断ログ/);
});

test("public page copy does not describe AI or generated implementation", async () => {
  const html = (await Promise.all([...routes.keys()].map((route) => render(route)))).join("\n");
  assert.doesNotMatch(html, /\b(?:AI|Agent|Codex|ChatGPT)\b|人工智能|大模型|生成式|模型生成|生成デモ|generated demonstration/iu);
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
  assert.match(readme, /## First launch on Windows and macOS/);
  assert.match(readmeZh, /## Windows 与 macOS 首次打开/);
  assert.match(readmeJa, /## Windows／macOS で初めて開くとき/);
  assert.match(readme, /More info[\s\S]*Run anyway[\s\S]*Optional download verification/);
  assert.match(readmeZh, /更多信息[\s\S]*仍要运行[\s\S]*下载文件校验（可选）/);
  assert.match(readmeJa, /詳細情報[\s\S]*このまま開く[\s\S]*ダウンロードファイルの確認（任意）/);
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
