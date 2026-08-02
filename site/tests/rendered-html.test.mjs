import assert from "node:assert/strict";
import test from "node:test";

async function render(path = "/") {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);
  return worker.fetch(new Request(`http://localhost${path}`, { headers: { accept: "text/html" } }), {
    ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) },
  }, { waitUntil() {}, passThroughOnException() {} });
}

test("server-renders the SheetProof home page", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  const html = await response.text();
  assert.match(html, /SheetProof/);
  assert.match(html, /看清每一处差异/);
  assert.match(html, /赛季角色数值差异/);
  assert.match(html, /放大查看/);
  assert.doesNotMatch(html, /codex-preview|react-loading-skeleton|Your site is taking shape/i);
});

test("server-renders product routes", async () => {
  for (const [path, text] of [["/features", "支持范围"], ["/guide", "完成一次对比与合并"], ["/download", "安装包正在建设中"], ["/changelog", "更新日志"]]) {
    const response = await render(path);
    assert.equal(response.status, 200, path);
    assert.match(await response.text(), new RegExp(text), path);
  }
});

test("all internal navigation links resolve to a product route", async () => {
  const routes = ["/", "/features", "/guide", "/download", "/changelog"];
  const knownRoutes = new Set(routes);
  for (const route of routes) {
    const response = await render(route);
    const html = await response.text();
    const hrefs = [...html.matchAll(/href="([^"]+)"/g)].map((match) => match[1]);
    for (const href of hrefs) {
      if (!href.startsWith("/")) continue;
      if (href.startsWith("/_") || href.startsWith("/assets/") || href.startsWith("/brand/") || href.startsWith("/screenshots/")) continue;
      const path = href.split("#", 1)[0] || "/";
      assert.ok(knownRoutes.has(path), `${route} links to unknown route ${href}`);
    }
  }
});
