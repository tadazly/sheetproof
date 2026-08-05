import { spawnSync } from "node:child_process";
import { resolve } from "node:path";

const command = process.argv[2];
if (!new Set(["dev", "build", "start"]).has(command)) {
  console.error("Usage: node scripts/run-vinext.mjs dev|build|start");
  process.exit(2);
}

const result = spawnSync(
  process.execPath,
  [resolve("node_modules/vinext/dist/cli.js"), command],
  {
    stdio: "inherit",
    env: {
      ...process.env,
      WRANGLER_LOG_PATH: process.env.WRANGLER_LOG_PATH ?? ".wrangler/wrangler.log",
    },
  },
);

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
if (result.status === 0 && command === "build") {
  const postprocess = spawnSync(process.execPath, [resolve("scripts/postprocess-static-locales.mjs")], { stdio: "inherit" });
  if (postprocess.error) console.error(postprocess.error.message);
  if (postprocess.status !== 0) process.exit(postprocess.status ?? 1);
}
process.exit(result.status ?? 1);
