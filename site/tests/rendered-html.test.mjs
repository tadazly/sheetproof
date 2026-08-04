import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";

const routes = new Map([
  ["/", "../dist/client/index.html"],
  ["/features", "../dist/client/features/index.html"],
  ["/guide", "../dist/client/guide/index.html"],
  ["/download", "../dist/client/download/index.html"],
  ["/changelog", "../dist/client/changelog/index.html"],
]);

async function render(path = "/") {
  const file = routes.get(path);
  assert.ok(file, `unknown route ${path}`);
  return readFile(fileURLToPath(new URL(file, import.meta.url)), "utf8");
}

test("exports the SheetProof home page", async () => {
  const html = await render();
  assert.match(html, /SheetProof/);
  assert.match(html, /面向 Git 工作流的 XLSX 差异审阅/);
  assert.match(html, /让配置表变更也能逐格审阅/);
  assert.match(html, /帮助游戏开发和数据团队核对变更/);
  assert.match(html, /理解 XLSX 语义/);
  assert.match(html, /UGit 集成/);
  assert.match(html, /赛季角色数值差异/);
  assert.match(html, /放大查看/);
  assert.match(html, /https:\/\/sheetproof\.luyilabs\.com\/og\.png/);
  assert.doesNotMatch(html, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("exports mobile navigation affordance, canonical GitHub link, and copyright", async () => {
  for (const route of routes.keys()) {
    const html = await render(route);
    assert.match(html, /aria-controls="mobile-site-menu"/);
    assert.match(html, /https:\/\/github\.com\/tadazly\/sheetproof/);
    assert.match(html, /Copyright \(c\) 2026 tadazly/);
  }
});

test("normalizes static-export trailing slashes before marking the current navigation item", async () => {
  const shell = await readFile(fileURLToPath(new URL("../app/components/SiteShell.tsx", import.meta.url)), "utf8");
  assert.match(shell, /pathname\.replace\(\/\\\/\$\/, ""\) \|\| "\/"/);
  assert.match(shell, /aria-current=\{isCurrent\(href\) \? "page"/);
});

test("exports product routes", async () => {
  for (const [path, text] of [["/features", "支持范围"], ["/guide", "完成一次对比与合并"], ["/download", "Windows 预览版"], ["/changelog", "更新日志"]]) {
    assert.match(await render(path), new RegExp(text), path);
  }
});

test("guide keeps the difference-row shortcut concise", async () => {
  const html = await render("/guide");
  assert.match(html, /差异行筛选/);
  assert.match(html, /1–4 切换分类，5 全选/);
  assert.match(html, /全选单元格/);
  assert.match(html, /Ctrl \/ Command \+ A/);
  assert.match(html, /清空左侧选区/);
  assert.match(html, /Backspace \/ Delete/);
  assert.doesNotMatch(html, /5 切换全部 \/ 不筛选/);
});

test("changelog publishes alignment and scrollbar work as the v0.3.0 preview", async () => {
  const html = await render("/changelog");
  assert.match(html, />v0\.3\.0</);
  assert.match(html, />预览版</);
  assert.match(html, /横向和纵向滚动条/);
  assert.match(html, /地图ID/);
  assert.doesNotMatch(html, />未发布</);
});

test("guide documents UGit setup and links directly to it", async () => {
  const [home, guide, css] = await Promise.all([
    render(),
    render("/guide"),
    readFile(fileURLToPath(new URL("../app/globals.css", import.meta.url)), "utf8"),
  ]);
  assert.match(home, /href="\/guide\/#ugit"/);
  assert.match(guide, /id="ugit"/);
  assert.match(guide, /点击“配置 UGit”/);
  assert.match(guide, /只会更新 <code>\*\.xlsx<\/code>/);
  assert.match(guide, /SpreadsheetCompare/);
  assert.match(guide, /看到两条 XLSX 差异工具是正常的/);
  assert.match(css, /\.mode-grid article \{ min-width: 0;/);
  assert.match(css, /\.mode-grid pre \{ max-width: 100%; min-width: 0;/);
  assert.match(css, /white-space: pre-wrap;/);
});

test("release download page contains the current stable assets and signing limitations", async () => {
  const html = await render("/download");
  const source = JSON.parse(
    await readFile(fileURLToPath(new URL("../../product/product.json", import.meta.url)), "utf8"),
  );
  assert.ok(html.includes(source.downloads.windows));
  assert.ok(html.includes(source.downloads.macos));
  assert.ok(html.includes(source.downloads.source));
  assert.match(html, /SHA-256/);
  assert.match(html, /未进行代码签名/);
  assert.match(html, /未公证/);
});

test("product facts, generated content, repository links, and MIT license stay in sync", async () => {
  const [sourceText, generatedText, readme, changelog] = await Promise.all([
    readFile(fileURLToPath(new URL("../../product/product.json", import.meta.url)), "utf8"),
    readFile(fileURLToPath(new URL("../app/content/product.json", import.meta.url)), "utf8"),
    readFile(fileURLToPath(new URL("../../README.md", import.meta.url)), "utf8"),
    readFile(fileURLToPath(new URL("../../CHANGELOG.md", import.meta.url)), "utf8"),
  ]);
  const source = JSON.parse(sourceText);
  assert.deepEqual(JSON.parse(generatedText), source);
  assert.equal(source.product.repository, "https://github.com/tadazly/sheetproof");
  assert.equal(source.product.issues, "https://github.com/tadazly/sheetproof/issues");
  assert.equal(source.product.license, "MIT");
  assert.equal(source.product.legacyName, "ugxlsx");
  assert.match(readme, /https:\/\/github\.com\/tadazly\/sheetproof\/releases/);
  assert.match(readme, /MIT License/);
  assert.match(changelog, /tadazly\/sheetproof/);
  assert.doesNotMatch(`${readme}\n${changelog}`, /github\.com\/tadazly\/ugxlsx|github\.com\/ug-tools\/ugxlsx/);
});

test("all internal navigation links resolve to a product route", async () => {
  const knownRoutes = new Set(routes.keys());
  for (const route of routes.keys()) {
    const html = await render(route);
    const hrefs = [...html.matchAll(/href="([^"]+)"/g)].map((match) => match[1]);
    for (const href of hrefs) {
      if (!href.startsWith("/")) continue;
      if (href.startsWith("/_") || href.startsWith("/assets/") || href.startsWith("/brand/") || href.startsWith("/screenshots/")) continue;
      const path = (href.split("#", 1)[0] || "/").replace(/\/$/, "") || "/";
      assert.ok(knownRoutes.has(path), `${route} links to unknown route ${href}`);
    }
  }
});
