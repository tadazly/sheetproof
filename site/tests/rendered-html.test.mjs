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
  assert.match(html, /逐格看清 XLSX 差异/);
  assert.match(html, /赛季角色数值差异/);
  assert.match(html, /放大查看/);
  assert.match(html, /https:\/\/sheetproof\.luyilabs\.com\/og\.png/);
  assert.doesNotMatch(html, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("exports product routes", async () => {
  for (const [path, text] of [["/features", "支持范围"], ["/guide", "完成一次对比与合并"], ["/download", "Windows 预览版"], ["/changelog", "更新日志"]]) {
    assert.match(await render(path), new RegExp(text), path);
  }
});

test("release download page contains stable v0.1.0 assets and signing limitations", async () => {
  const html = await render("/download");
  assert.match(html, /github\.com\/tadazly\/sheetproof\/releases\/download\/v0\.1\.0/);
  assert.match(html, /releases\/download\/v0\.1\.0\/SheetProof-windows-amd64\.exe/);
  assert.match(html, /releases\/download\/v0\.1\.0\/SheetProof-macos-universal\.zip/);
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
