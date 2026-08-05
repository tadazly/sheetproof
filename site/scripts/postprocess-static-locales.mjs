import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const routes = ["", "features", "guide", "download", "changelog", "zh-CN", "zh-CN/features", "zh-CN/guide", "zh-CN/download", "zh-CN/changelog", "ja", "ja/features", "ja/guide", "ja/download", "ja/changelog"];
for (const route of routes) {
  const file = resolve("dist/client", route, "index.html");
  const locale = route === "zh-CN" || route.startsWith("zh-CN/") ? "zh-CN" : route === "ja" || route.startsWith("ja/") ? "ja" : "en";
  const source = await readFile(file, "utf8");
  const output = source.replace(/<html\s+lang="[^"]*"/, `<html lang="${locale}"`);
  if (output === source && !source.includes(`<html lang="${locale}"`)) throw new Error(`Could not set html lang for ${route || "/"}`);
  await writeFile(file, output);
}
