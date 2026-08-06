import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

async function readJSON(path) {
  return JSON.parse(await readFile(path, "utf8"));
}

function assetName(url, label) {
  const name = decodeURIComponent(new URL(url).pathname.split("/").at(-1) ?? "");
  if (!name) throw new Error(`${label} download URL has no file name`);
  return name;
}

function titleCase(value) {
  return value ? `${value[0].toUpperCase()}${value.slice(1)}` : value;
}

export function renderReleaseNotes({ facts, releases, english, version }) {
  if (facts.product.version !== version) {
    throw new Error(`product version ${facts.product.version} does not match release ${version}`);
  }

  const release = releases.releases.find((candidate) => candidate.version === version);
  if (!release) throw new Error(`release ${version} is missing from product/changelog/releases.json`);

  const localized = english[version];
  if (!localized) throw new Error(`release ${version} is missing from product/changelog/en.json`);

  const changes = release.changes.map((id) => {
    const change = localized.changes[id];
    if (!change) throw new Error(`release ${version} is missing English change ${id}`);
    return `- ${change}`;
  });

  const downloads = [
    [facts.downloads["windows-amd64"], "Windows amd64"],
    [facts.downloads["macos-universal"], "macOS universal"],
    [facts.downloads.checksums, "checksums"],
  ].map(([url, label]) => `- [\`${assetName(url, label)}\`](${url})`);

  const security = [];
  if (!facts.product.signed) {
    security.push("The Windows build is unsigned.");
    security.push(`The macOS build is unsigned${facts.product.macosNotarized ? "." : " and not notarized."}`);
  } else if (!facts.product.macosNotarized) {
    security.push("The macOS build is not notarized.");
  }
  security.push("Download only from this GitHub Release and verify SHA-256 before running it.");

  const website = facts.product.website.replace(/\/$/, "");
  return [
    `SheetProof ${version} ${titleCase(release.channel)} — ${localized.summary}`,
    "",
    "## What's new",
    "",
    ...changes,
    "",
    "## Downloads",
    "",
    ...downloads,
    "",
    security.join(" "),
    "",
    "## Other languages",
    "",
    `- [简体中文更新日志](${website}/zh-CN/changelog/)`,
    `- [日本語のリリースノート](${website}/ja/changelog/)`,
    "",
  ].join("\n");
}

export async function generateReleaseNotes({ root = repositoryRoot, version } = {}) {
  const [facts, releases, english] = await Promise.all([
    readJSON(resolve(root, "product/product.json")),
    readJSON(resolve(root, "product/changelog/releases.json")),
    readJSON(resolve(root, "product/changelog/en.json")),
  ]);
  return renderReleaseNotes({ facts, releases, english, version: version ?? facts.product.version });
}

function parseArguments(argumentsList) {
  const options = {};
  for (let index = 0; index < argumentsList.length; index += 1) {
    const argument = argumentsList[index];
    if (argument === "--output" || argument === "--version") {
      const value = argumentsList[index + 1];
      if (!value) throw new Error(`${argument} requires a value`);
      options[argument.slice(2)] = value;
      index += 1;
    } else {
      throw new Error(`unknown argument: ${argument}`);
    }
  }
  return options;
}

if (fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const options = parseArguments(process.argv.slice(2));
  const notes = await generateReleaseNotes({ version: options.version });
  if (options.output) {
    const output = resolve(options.output);
    await mkdir(dirname(output), { recursive: true });
    await writeFile(output, notes, "utf8");
  } else {
    process.stdout.write(notes);
  }
}
