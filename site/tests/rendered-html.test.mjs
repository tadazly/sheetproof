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
  for (const [path, text] of [["/features", "支持范围"], ["/guide", "完成一次对比与合并"], ["/download", "当前尚未发布"], ["/changelog", "更新日志"]]) {
    assert.match(await render(path), new RegExp(text), path);
  }
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
