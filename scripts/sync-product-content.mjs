import { access, mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const check = process.argv.includes("--check");
const locales = ["en", "zh-CN", "ja"];
const generatedNotice = "Generated from product facts and locale content. Do not edit directly.";

const readJSON = async (path) => JSON.parse(await readFile(resolve(root, path), "utf8"));
const facts = await readJSON("product/product.json");
const glossary = await readJSON("product/glossary.json");
const releases = await readJSON("product/changelog/releases.json");
const localeContent = Object.fromEntries(await Promise.all(locales.map(async (locale) => [locale, await readJSON(`product/locales/${locale}.json`)])));
const changelogContent = Object.fromEntries(await Promise.all(locales.map(async (locale) => [locale, await readJSON(`product/changelog/${locale}.json`)])));

function fail(message) {
  throw new Error(`content validation failed: ${message}`);
}

function shape(value, path = "") {
  if (Array.isArray(value)) return { kind: "array", values: value.map((item, index) => shape(item, `${path}[${index}]`)) };
  if (value && typeof value === "object") return { kind: "object", keys: Object.keys(value).sort().map((key) => [key, shape(value[key], path ? `${path}.${key}` : key)]) };
  return { kind: typeof value };
}

function compareShape(reference, candidate, label) {
  if (JSON.stringify(shape(reference)) !== JSON.stringify(shape(candidate))) fail(`${label} keys do not match English`);
}

for (const locale of locales.slice(1)) {
  compareShape(localeContent.en, localeContent[locale], `product/locales/${locale}.json`);
  compareShape(changelogContent.en, changelogContent[locale], `product/changelog/${locale}.json`);
}

const productVersion = facts.product.version;
for (const [platform, url] of Object.entries(facts.downloads)) {
  if (platform !== "releases" && !url.includes(`v${productVersion}`)) fail(`${platform} download does not use v${productVersion}`);
}
for (const release of releases.releases) {
  for (const locale of locales) {
    const localized = changelogContent[locale][release.version];
    if (!localized) fail(`${release.version} is missing from ${locale} changelog`);
    const expected = [...release.changes].sort();
    const actual = Object.keys(localized.changes).sort();
    if (JSON.stringify(expected) !== JSON.stringify(actual)) fail(`${release.version} change IDs do not match in ${locale}`);
    if (!localized.title || !localized.summary || actual.some((id) => !localized.changes[id])) fail(`${release.version} has empty ${locale} changelog text`);
  }
}

for (const [concept, translations] of Object.entries(glossary)) {
  if (JSON.stringify(Object.keys(translations).sort()) !== JSON.stringify([...locales].sort())) fail(`glossary.${concept} does not contain all locales`);
  if (locales.some((locale) => !translations[locale])) fail(`glossary.${concept} has an empty translation`);
}

for (const locale of locales) {
  for (const [id, screenshot] of Object.entries(facts.screenshots)) {
    if (!localeContent[locale].screenshots[id]) fail(`screenshot ${id} is missing ${locale} metadata`);
    if (check) {
      const path = resolve(root, "site/public/screenshots", locale, screenshot.file);
      try { await access(path); } catch { fail(`missing screenshot asset site/public/screenshots/${locale}/${screenshot.file}`); }
    }
  }
}

function screenshotPath(locale, id) {
  return `site/public/screenshots/${locale}/${facts.screenshots[id].file}`;
}

function languageNav(locale, kind) {
  const names = { en: "English", "zh-CN": "简体中文", ja: "日本語" };
  const filename = (target) => `${kind}${target === "en" ? "" : `.${target}`}.md`;
  return locales.map((target) => target === locale ? names[target] : `[${names[target]}](${filename(target)})`).join(" | ");
}

function bullets(items) {
  return Object.values(items).map((item) => `- **${item.title}**: ${item.summary}`).join("\n");
}

function numbered(items) {
  return items.map((item, index) => `${index + 1}. ${item}`).join("\n");
}

function fenced(language, value) {
  return `\`\`\`${language}\n${value}\n\`\`\``;
}

const readmeCommon = {
  en: {
    title: "# SheetProof", why: "## Why SheetProof", whyText: "XLSX is a compressed OOXML package. Text diffs confuse serialization noise with workbook changes and cannot selectively merge cells in context. SheetProof reads workbook semantics into synchronized grids and keeps the user in control of what enters the left result.",
    audience: "For developers and content teams who review versioned configuration workbooks. Files are processed locally, and Excel is not required.", keyHeading: "## Key alignment", keyText: "SheetProof detects a shared `id` header or one unambiguous `*ID` header. You can also choose a key column from a column header. Records with a reliable key are aligned; ambiguous records fall back to physical rows without disabling alignment for the rest of the sheet. One-sided records remain near their shared neighbors.",
    features: "## Current features", download: "## Download", quick: "## Quick start", git: "## Git repository mode", files: "## Direct-file mode", ugit: "## UGit", cli: "## CLI", limits: "## Known limitations", build: "## Build", license: "## License",
    install: {
      heading: "## First launch on Windows and macOS",
      intro: "SheetProof is currently an unsigned preview. Windows and macOS may show a security warning the first time you open it because the system cannot verify the publisher. This warning does not by itself mean that the system found malware.",
      safety: "Download SheetProof only from this project's GitHub Releases. If the system explicitly reports malware or a damaged file, stop and download it again.",
      windowsHeading: "### Windows",
      windowsReason: "The current executable is not code-signed, so Windows may identify it as an unrecognized app or show an unknown publisher.",
      windowsSteps: ["Download and double-click `SheetProof-windows-amd64.exe`.", "If Windows says it protected your PC, select **More info**.", "Select **Run anyway**."],
      windowsFallback: "If **Run anyway** is unavailable, Smart App Control or an organization policy may be blocking the app. Contact your administrator; do not disable Windows security globally just to run SheetProof.",
      macHeading: "### macOS",
      macReason: "The current app is not signed with an Apple Developer ID and is not notarized, so Gatekeeper cannot automatically verify the developer or notarization status.",
      macSteps: ["Download and extract `SheetProof-macos-universal.zip`.", "Move `SheetProof.app` to **Applications**, then double-click it.", "If macOS blocks it, open **System Settings → Privacy & Security**.", "In the Security section, find SheetProof, select **Open Anyway**, authenticate, then select **Open**."],
      macFallback: "If **Open Anyway** is not visible, try opening SheetProof once more and return to Privacy & Security. A managed Mac may require administrator approval.",
      checksumHeading: "## Optional download verification",
      checksumIntro: "For an additional integrity check, compare the downloaded file with `SHA256SUMS.txt` from the same GitHub Release.",
      checksumWindows: "(Get-FileHash .\\SheetProof-windows-amd64.exe -Algorithm SHA256).Hash",
      checksumMac: "shasum -a 256 SheetProof-macos-universal.zip",
      checksumNote: "If the values do not match, do not run the file. Download it again from GitHub Releases."
    },
    screenshot: "Key-aligned character configuration comparison", caption: "The demo uses localized character growth, stage reward, and skill tuning workbooks.",
    quickText: "Open a local Git repository, or choose two .xlsx workbooks. The left side is the only writable source; the right side is always read-only.", gitText: "Repository mode reads the real worktree on the left and a validated Git object on the right. It does not fetch, checkout, switch, stage, commit, or push.", filesText: "Use the welcome screen or run `sheetproof compare --left current.xlsx --right target.xlsx`.", ugitText: "Place the app in a stable location, then use Configure UGit. SheetProof replaces only *.xlsx diff and merge entries and verifies the result.", cliText: "Text output follows `--lang en|zh-CN|ja`; JSON keys and enum values remain language-independent.", limitsText: "Only .xlsx is supported; .xlsm is not supported. SheetProof is not an Excel-level editor and does not guarantee full fidelity for charts, images, pivot tables, external connections, or complex conditional formatting.", buildText: "Requires Go 1.24+, Node.js 20+, and Wails 2.10.2. On Windows use `powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build`. GitHub Release notes are generated from the English changelog and end with links to the Simplified Chinese and Japanese website changelogs.", licenseText: "MIT License. Copyright (c) 2026 tadazly."
  },
  "zh-CN": {
    title: "# SheetProof · 表鉴", why: "## 为什么需要 SheetProof", whyText: "XLSX 是压缩的 OOXML 包。文本 diff 容易把序列化噪声误判为工作簿变化，也无法在单元格上下文中选择性合并。SheetProof 读取工作簿语义，以同步双栏呈现，并由用户决定哪些内容进入左侧结果。",
    audience: "适合需要审阅版本化配置表的开发者和内容团队。文件只在本机处理，也不依赖 Excel。", keyHeading: "## 主键对齐", keyText: "SheetProof 会识别双方共同的 `id` 表头，或唯一且无歧义的 `*ID` 表头；也可从列标题菜单指定主键列。可靠记录按主键对齐，歧义记录仅自身回退物理行，不会让整张表放弃对齐。单侧记录会保留在相邻共同记录附近。",
    features: "## 当前功能", download: "## 下载", quick: "## 快速开始", git: "## Git 仓库模式", files: "## 双文件模式", ugit: "## UGit", cli: "## CLI", limits: "## 已知限制", build: "## 构建", license: "## License",
    install: {
      heading: "## Windows 与 macOS 首次打开",
      intro: "SheetProof 当前是未签名的预览版，因此 Windows 和 macOS 在首次打开时可能显示安全提示。这是因为系统暂时无法验证应用发布者，并不代表系统已经确认应用含有恶意软件。",
      safety: "请只从本项目的 GitHub Releases 下载 SheetProof。如果系统明确报告恶意软件或文件损坏，请停止运行并重新下载。",
      windowsHeading: "### Windows",
      windowsReason: "当前可执行文件尚未进行代码签名，因此 Windows 可能将它显示为无法识别的应用或未知发布者。",
      windowsSteps: ["下载并双击 `SheetProof-windows-amd64.exe`。", "如果出现“Windows 已保护你的电脑”，点击**更多信息**。", "点击**仍要运行**。"],
      windowsFallback: "如果没有“仍要运行”按钮，设备可能启用了 Smart App Control 或组织安全策略。请联系系统管理员；不建议为了运行 SheetProof 全局关闭 Windows 安全功能。",
      macHeading: "### macOS",
      macReason: "当前应用尚未使用 Apple Developer ID 签名，也未经过 Apple 公证，因此 Gatekeeper 无法自动验证开发者和公证状态。",
      macSteps: ["下载并解压 `SheetProof-macos-universal.zip`。", "将 `SheetProof.app` 移到“应用程序”，然后双击打开。", "如果系统阻止打开，请进入“系统设置 → 隐私与安全性”。", "在“安全性”区域找到 SheetProof，点击“仍要打开”，完成验证后再次点击“打开”。"],
      macFallback: "如果没有看到“仍要打开”，请再次尝试打开 SheetProof，然后立即返回“隐私与安全性”。受公司或学校管理的 Mac 可能需要管理员授权。",
      checksumHeading: "## 下载文件校验（可选）",
      checksumIntro: "如果需要进一步确认下载文件与 GitHub Release 中发布的文件一致，可以将计算结果与同一版本的 `SHA256SUMS.txt` 对照。",
      checksumWindows: "(Get-FileHash .\\SheetProof-windows-amd64.exe -Algorithm SHA256).Hash",
      checksumMac: "shasum -a 256 SheetProof-macos-universal.zip",
      checksumNote: "如果两者不一致，请不要运行该文件，并重新从 GitHub Releases 下载。"
    },
    screenshot: "按主键对齐的角色配置表对比", caption: "演示使用本地化的角色成长、关卡掉落和技能参数工作簿。",
    quickText: "打开本地 Git 仓库，或选择两个 .xlsx 工作簿。左侧是唯一可写来源，右侧始终只读。", gitText: "仓库模式左侧读取真实工作区，右侧读取经过校验的 Git 对象；不会 fetch、checkout、切换分支、暂存、提交或推送。", filesText: "可使用欢迎页，或运行 `sheetproof compare --left current.xlsx --right target.xlsx`。", ugitText: "把应用放在固定位置后使用“配置 UGit”。SheetProof 只替换 *.xlsx 差异与合并项，并在写入后校验。", cliText: "文本输出遵循 `--lang en|zh-CN|ja`；JSON 字段和枚举值保持语言无关。", limitsText: "只支持 .xlsx，不支持 .xlsm。SheetProof 不是 Excel 级编辑器，也不保证图表、图片、数据透视表、外部连接或复杂条件格式等高级对象能够完整保真。", buildText: "需要 Go 1.24+、Node.js 20+ 和 Wails 2.10.2。Windows 使用 `powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build`。GitHub Release 说明由英文更新日志自动生成，并在结尾链接简体中文和日文官网更新日志。", licenseText: "MIT License。Copyright (c) 2026 tadazly。"
  },
  ja: {
    title: "# SheetProof（シートプルーフ）", why: "## SheetProof が必要な理由", whyText: "XLSX は圧縮された OOXML パッケージです。テキスト diff では再保存による構造差を内容変更と誤認しやすく、セルを文脈の中で選択的に反映できません。SheetProof はブックの意味を同期した 2 ペインに表示し、左側へ取り込む内容をユーザーが決められるようにします。",
    audience: "バージョン管理している設定ブックをレビューする開発者やコンテンツチーム向けです。ファイルはローカルで処理し、Excel は必要ありません。", keyHeading: "## キー列によるレコード照合", keyText: "両側に共通する `id` 見出し、または一意で曖昧さのない `*ID` 見出しを検出します。列見出しのメニューからキー列を指定することもできます。信頼できるレコードはキーで揃え、曖昧なレコードだけを物理行で比較します。片側だけのレコードは、共通する前後のレコード付近に残ります。",
    features: "## 現在の機能", download: "## ダウンロード", quick: "## クイックスタート", git: "## Git リポジトリモード", files: "## 2 ファイルモード", ugit: "## UGit", cli: "## CLI", limits: "## 既知の制限", build: "## ビルド", license: "## License",
    install: {
      heading: "## Windows／macOS で初めて開くとき",
      intro: "現在の SheetProof は未署名のプレビュー版です。そのため、Windows や macOS で初めて開くと、システムが発行元を確認できないという警告が表示される場合があります。この警告だけでマルウェアが検出されたことを意味するわけではありません。",
      safety: "SheetProof は、このプロジェクトの GitHub Releases からのみダウンロードしてください。マルウェアまたはファイル破損が明示された場合は実行せず、ダウンロードし直してください。",
      windowsHeading: "### Windows",
      windowsReason: "現在の実行ファイルはコード署名されていないため、Windows で認識されないアプリまたは不明な発行元として表示される場合があります。",
      windowsSteps: ["`SheetProof-windows-amd64.exe` をダウンロードしてダブルクリックします。", "「Windows によって PC が保護されました」と表示されたら、**詳細情報**を選択します。", "**実行**を選択します。"],
      windowsFallback: "**実行**が表示されない場合は、Smart App Control または組織のポリシーによってブロックされている可能性があります。管理者に連絡し、SheetProof のためだけに Windows の保護機能全体を無効にしないでください。",
      macHeading: "### macOS",
      macReason: "現在のアプリは Apple Developer ID で署名されておらず、公証も受けていないため、Gatekeeper は開発元と公証状態を自動確認できません。",
      macSteps: ["`SheetProof-macos-universal.zip` をダウンロードして展開します。", "`SheetProof.app` を「アプリケーション」へ移動し、ダブルクリックします。", "macOS によってブロックされたら、「システム設定 → プライバシーとセキュリティ」を開きます。", "「セキュリティ」で SheetProof の**このまま開く**を選択し、認証後にもう一度**開く**を選択します。"],
      macFallback: "**このまま開く**が表示されない場合は、SheetProof をもう一度開いてから、すぐに「プライバシーとセキュリティ」へ戻ってください。管理対象の Mac では管理者の許可が必要な場合があります。",
      checksumHeading: "## ダウンロードファイルの確認（任意）",
      checksumIntro: "ダウンロードしたファイルが GitHub Release のものと一致することを追加で確認する場合は、同じバージョンの `SHA256SUMS.txt` と計算結果を比較します。",
      checksumWindows: "(Get-FileHash .\\SheetProof-windows-amd64.exe -Algorithm SHA256).Hash",
      checksumMac: "shasum -a 256 SheetProof-macos-universal.zip",
      checksumNote: "値が一致しない場合は実行せず、GitHub Releases からダウンロードし直してください。"
    },
    screenshot: "キーで対応付けたキャラクター設定表の比較", caption: "ローカライズしたキャラクター成長、ステージ報酬、スキル設定ブックを使用しています。",
    quickText: "ローカル Git リポジトリを開くか、2 つの .xlsx ブックを選びます。書き込み可能なのは左側だけで、右側は常に読み取り専用です。", gitText: "左側は実際のワークツリー、右側は検証済み Git オブジェクトです。fetch、checkout、ブランチ切り替え、stage、commit、push は行いません。", filesText: "開始画面から選ぶか、`sheetproof compare --left current.xlsx --right target.xlsx` を実行します。", ugitText: "アプリを固定した場所へ置き、「UGit を設定」を実行します。*.xlsx の差分・マージ項目だけを置き換え、書き込み後に検証します。", cliText: "テキスト出力は `--lang en|zh-CN|ja` に従います。JSON のキーと列挙値は翻訳しません。", limitsText: ".xlsx のみ対応し、.xlsm には対応していません。SheetProof は Excel 相当のエディターではなく、グラフ、画像、ピボットテーブル、外部接続、複雑な条件付き書式などの高度なオブジェクトを完全に保持することは保証しません。", buildText: "Go 1.24+、Node.js 20+、Wails 2.10.2 が必要です。Windows では `powershell -ExecutionPolicy Bypass -File scripts/invoke-wails.ps1 build` を使います。GitHub Release の本文は英語の変更履歴から自動生成し、末尾に簡体字中国語と日本語の公式サイト変更履歴へのリンクを追加します。", licenseText: "MIT License。Copyright (c) 2026 tadazly。"
  }
};

function renderReadme(locale) {
  const l = localeContent[locale];
  const c = readmeCommon[locale];
  const d = facts.downloads;
  const i = c.install;
  return `<!-- ${generatedNotice} -->\n\n${languageNav(locale, "README")}\n\n${c.title}\n\n> ${l.product.tagline}\n\n${l.product.description}\n\n${c.audience}\n\n[![${c.screenshot}](${screenshotPath(locale, "keyAlignment")})](${screenshotPath(locale, "keyAlignment")})\n\n_${c.caption}_\n\n${c.why}\n\n${c.whyText}\n\n${c.keyHeading}\n\n${c.keyText}\n\n${c.features}\n\n${bullets(l.features)}\n\n${c.download}\n\n**${facts.product.version} ${l.product.channel}**\n\n- [Windows amd64](${d["windows-amd64"]})\n- [macOS universal](${d["macos-universal"]})\n- [SHA256SUMS.txt](${d.checksums})\n- [Source](${d.source})\n\n${i.heading}\n\n${i.intro}\n\n${i.safety}\n\n${i.windowsHeading}\n\n${i.windowsReason}\n\n${numbered(i.windowsSteps)}\n\n${i.windowsFallback}\n\n${i.macHeading}\n\n${i.macReason}\n\n${numbered(i.macSteps)}\n\n${i.macFallback}\n\n${i.checksumHeading}\n\n${i.checksumIntro}\n\n**Windows**\n\n${fenced("powershell", i.checksumWindows)}\n\n**macOS**\n\n${fenced("bash", i.checksumMac)}\n\n${i.checksumNote}\n\n${c.quick}\n\n${c.quickText}\n\n${c.git}\n\n${c.gitText}\n\n${c.files}\n\n${c.filesText}\n\n${c.ugit}\n\n${c.ugitText}\n\n${c.cli}\n\n${c.cliText}\n\n${c.limits}\n\n${c.limitsText}\n\n${c.build}\n\n${c.buildText}\n\n${c.license}\n\n${c.licenseText}\n`;
}

function renderChangelog(locale) {
  const intro = { en: "User-visible SheetProof changes.", "zh-CN": "SheetProof 的用户可见变化记录。", ja: "SheetProof のユーザー向け変更履歴です。" }[locale];
  const unreleased = { en: "Unreleased", "zh-CN": "未发布", ja: "未リリース" }[locale];
  const sections = releases.releases.map((release) => {
    const text = changelogContent[locale][release.version];
    const changes = release.changes.map((id) => `- ${text.changes[id]}`).join("\n");
    const heading = release.version === "unreleased" ? unreleased : `${release.version} — ${release.date}`;
    return `## ${heading}\n\n_${localeContent[locale].product.channel} · ${text.title}_\n\n${text.summary}\n\n${changes}`;
  }).join("\n\n");
  return `<!-- ${generatedNotice} -->\n\n${languageNav(locale, "CHANGELOG")}\n\n# Changelog\n\n${intro}\n\n${sections}\n`;
}

const outputs = new Map();
for (const locale of locales) {
  const suffix = locale === "en" ? "" : `.${locale}`;
  outputs.set(`README${suffix}.md`, renderReadme(locale));
  outputs.set(`CHANGELOG${suffix}.md`, renderChangelog(locale));
}

const generatedBundle = { notice: generatedNotice, facts, locales: localeContent, glossary, changelog: { releases: releases.releases, locales: changelogContent } };
outputs.set("site/app/content/generated/content.json", `${JSON.stringify(generatedBundle, null, 2)}\n`);
outputs.set("internal/cli/version_generated.go", `// ${generatedNotice}\n\npackage cli\n\nconst Version = "${facts.product.version}"\n`);

for (const path of ["frontend/package.json", "site/package.json"]) {
  const manifest = await readJSON(path);
  manifest.version = facts.product.version;
  manifest.license = facts.product.license;
  outputs.set(path, `${JSON.stringify(manifest, null, 2)}\n`);
  const lockPath = path.replace(/package\.json$/, "package-lock.json");
  const lock = await readJSON(lockPath);
  lock.version = facts.product.version;
  lock.packages[""].version = facts.product.version;
  lock.packages[""].license = facts.product.license;
  outputs.set(lockPath, `${JSON.stringify(lock, null, 2)}\n`);
}

let drift = false;
for (const [path, expected] of outputs) {
  const full = resolve(root, path);
  if (check) {
    let actual = "";
    try { actual = await readFile(full, "utf8"); } catch { drift = true; console.error(`missing generated file: ${path}`); continue; }
    if (actual !== expected) { drift = true; console.error(`generated content drift: ${path}`); }
  } else {
    await mkdir(dirname(full), { recursive: true });
    await writeFile(full, expected, "utf8");
  }
}
if (check && drift) process.exitCode = 1;
if (!check) console.log(`Synced SheetProof ${facts.product.version} content for ${locales.join(", ")}.`);
