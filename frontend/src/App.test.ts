import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App.vue";
import type { CellDiff, Region, RepositoryResult, RepositoryView, Summary } from "./types";

const emptySummary: Summary = {
  options: {
    title: "",
    leftLabel: "本地（可编辑）",
    rightLabel: "对比来源（只读）",
    readonlyLeft: false,
    gitDiff: false,
    ugitWorktree: false,
    gitMerge: false,
    output: ""
  },
  diff: {
    equal: true,
    leftFile: "/tmp/左侧 文件.xlsx",
    rightFile: "/tmp/右侧 文件.xlsx",
    sheetCount: 0,
    differentSheetCount: 0,
    differenceCount: 0,
    sheets: []
  },
  resolutions: [],
  dirty: false,
  undoCount: 0,
  warnings: [],
  mergeNotice: "",
  selectedSheet: ""
};

const repositoryView: RepositoryView = {
  name: "中文 仓库",
  path: "/tmp/中文 仓库",
  currentBranch: "main",
  detached: false,
  workspaceDirty: true,
  operation: "",
  files: ["config/activity/reward.xlsx", "中文 目录/配置.xlsx"],
  differenceFiles: ["config/activity/reward.xlsx"],
  differenceIndexing: false,
  branches: [
    { name: "develop", fullName: "refs/heads/develop", kind: "local" },
    { name: "origin/main", fullName: "refs/remotes/origin/main", kind: "remote" }
  ],
  defaultRef: "refs/heads/develop",
  selectedFile: "",
  selectedRef: "refs/heads/develop",
  leftState: "no-file",
  rightState: "no-file",
  leftMessage: "请先在左侧目录树中选择一个 XLSX 表格",
  rightMessage: "请先在左侧目录树中选择一个 XLSX 表格",
  fileModified: false,
  sidebarWidth: 280,
  notice: "",
  loading: false,
  loadGeneration: 1,
  comparisonActive: false
};

enableAutoUnmount(afterEach);
afterEach(() => vi.useRealTimers());

describe("App", () => {
  const selectAndOpen = vi.fn(async () => emptySummary);

  beforeEach(() => {
    selectAndOpen.mockClear();
    window.localStorage.clear();
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: false, error: "" }),
          SelectAndOpen: selectAndOpen
        }
      }
    };
  });

  it("keeps direct two-file comparison as the secondary welcome action", async () => {
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.text()).toContain("打开本地 Git 仓库");
    await wrapper.get("button.secondary-entry").trigger("click");
    await flushPromises();
    expect(selectAndOpen).toHaveBeenCalledOnce();
    expect(wrapper.text()).toContain("/tmp/左侧 文件.xlsx");
    expect(wrapper.text()).toContain("/tmp/右侧 文件.xlsx");
  });

  it("configures UGit from the persistent low-frequency action and reports success", async () => {
    const configureUGit = vi.fn(async () => ({
      configured: true,
      changed: true,
      cancelled: false,
      executablePath: "/Applications/ugxlsx.app/Contents/MacOS/ugxlsx",
      message: "UGit 的 *.xlsx 差异与合并工具已更新。"
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: false, error: "" }),
          ConfigureUGit: configureUGit
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();

    const button = wrapper.get('button[aria-label="配置 UGit"]');
    expect(button.attributes("title")).toContain("*.xlsx");
    await button.trigger("click");
    await flushPromises();

    expect(configureUGit).toHaveBeenCalledOnce();
    expect(wrapper.get(".success-banner").text()).toContain("*.xlsx 差异与合并工具已更新");
  });

  it("renders Git difftool snapshots as read-only without exposing temporary directories", async () => {
    const gitDiff = structuredClone(emptySummary);
    gitDiff.options.gitDiff = true;
    gitDiff.options.readonlyLeft = true;
    gitDiff.diff.leftFile = "/var/folders/random/git-blob-a/配置.xlsx";
    gitDiff.diff.rightFile = "/var/folders/random/ugxlsx-git-null/missing-right.xlsx";
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => gitDiff
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.text()).toContain("Git 差异快照 · 只读");
    expect(wrapper.text()).toContain("配置.xlsx");
    expect(wrapper.text()).toContain("Git 快照 · 该版本不存在");
    expect(wrapper.text()).not.toContain("missing-right.xlsx");
    expect(wrapper.text()).not.toContain("/var/folders/random");
    expect(wrapper.findAll(".file-strip .readonly-source")).toHaveLength(2);
    expect(wrapper.findAll(".grid-panel")[0].get(".panel-permission").text()).toBe("只读");
    expect(wrapper.text()).not.toContain("双击单元格编辑");
    for (const button of wrapper.findAll(".save-actions button")) {
      expect(button.attributes("disabled")).toBeDefined();
    }
    expect(wrapper.get(".merge-action").attributes("disabled")).toBeDefined();
  });

  it("renders a verified UGit worktree on the editable left and hides the snapshot directory", async () => {
    const ugitWorktree = structuredClone(emptySummary);
    ugitWorktree.options.ugitWorktree = true;
    ugitWorktree.options.leftLabel = "当前工作区";
    ugitWorktree.options.rightLabel = "HEAD";
    ugitWorktree.diff.leftFile = "D:/repo 中文/config/reward.xlsx";
    ugitWorktree.diff.rightFile = "D:/repo 中文/.git/ugit/diff/temp-HEAD-config-reward.xlsx";
    ugitWorktree.dirty = true;
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => ugitWorktree
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();

    expect(wrapper.text()).toContain("当前工作区 · 可编辑");
    expect(wrapper.text()).toContain("Git 版本快照 · 只读");
    expect(wrapper.text()).toContain("D:/repo 中文/config/reward.xlsx");
    expect(wrapper.text()).toContain("Git 快照 · temp-HEAD-config-reward.xlsx");
    expect(wrapper.text()).not.toContain("/.git/ugit/diff/");
    expect(wrapper.findAll(".file-strip .editable-source")).toHaveLength(1);
    expect(wrapper.findAll(".file-strip .readonly-source")).toHaveLength(1);
    expect(wrapper.findAll(".grid-panel")[0].get(".panel-permission").text()).toBe("可编辑");
    expect(wrapper.text()).toContain("双击单元格编辑");
    expect(wrapper.text()).toContain("保存到当前工作区");
  });

  it("opens a repository and shows the tree plus two explicit no-file states", async () => {
    const selectRepository = vi.fn(async (): Promise<RepositoryResult> => ({
      repository: structuredClone(repositoryView),
      summary: null
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: false, error: "" }),
          SelectRepository: selectRepository
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    await wrapper.get("button.primary.large").trigger("click");
    await flushPromises();
    expect(selectRepository).toHaveBeenCalledOnce();
    expect(wrapper.text()).toContain("中文 仓库");
    expect(wrapper.text()).toContain("reward.xlsx");
    expect(wrapper.text().match(/请先在左侧目录树中选择一个 XLSX 表格/g)).toHaveLength(2);
    expect(wrapper.find(".repository-resizer").exists()).toBe(true);
    const differenceTab = wrapper.findAll('[role="tab"]').find((tab) => tab.text().includes("差异表"));
    expect(differenceTab?.text()).toContain("1");
    await differenceTab!.trigger("click");
    expect(wrapper.get(".repository-tree").text()).toContain("reward.xlsx");
    expect(wrapper.get(".repository-tree").text()).not.toContain("配置.xlsx");
    expect(wrapper.find(".tree-difference-count").exists()).toBe(false);
    const filesTab = wrapper.findAll('[role="tab"]').find((tab) => tab.text().includes("仓库文件"));
    await filesTab!.trigger("click");
    const search = wrapper.get('input[aria-label="筛选仓库文件或目录"]');
    await search.setValue("中文 目录");
    expect(wrapper.get(".repository-tree").text()).toContain("中文 目录");
    expect(wrapper.get(".repository-tree").text()).toContain("配置.xlsx");
    expect(wrapper.get(".repository-tree").text()).not.toContain("reward.xlsx");
    await search.setValue("不存在");
    expect(wrapper.find(".repository-tree").exists()).toBe(false);
    expect(wrapper.text()).toContain("没有匹配的文件或目录");
  });

  it("distinguishes a Git file conflict from semantic row conflicts", async () => {
    const mergeSummary = structuredClone(emptySummary);
    mergeSummary.options.gitMerge = true;
    mergeSummary.mergeNotice = "Git 已将此 XLSX 标记为文件级冲突，但右侧与共同基线语义一致；当前只有左侧存在实际表格变化，没有双方语义冲突。";
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => mergeSummary
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.text()).toContain("Git 合并来源 · 可编辑");
    expect(wrapper.get(".comparison-summary-stack").element.children).toHaveLength(2);
    expect(wrapper.get(".merge-semantic-notice").text()).toContain("没有双方语义冲突");
  });

  it("keeps five recent searches per repository and provides a larger clear action", async () => {
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false,
            hasSession: false,
            error: "",
            mode: "repository",
            repository: structuredClone(repositoryView)
          })
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();

    const search = wrapper.get('input[aria-label="筛选仓库文件或目录"]');
    await search.trigger("focus");
    expect(wrapper.get('[role="listbox"][aria-label="最近搜索"]').text()).toContain("暂无搜索记录");

    for (let index = 1; index <= 6; index++) {
      await search.setValue(`搜索 ${index}`);
      await search.trigger("blur");
      expect(wrapper.find('[role="listbox"][aria-label="最近搜索"]').exists()).toBe(false);
      if (index < 6) await search.trigger("focus");
    }

    await search.trigger("focus");
    const options = wrapper.findAll('[role="option"]');
    expect(options).toHaveLength(5);
    expect(options.map((option) => option.text())).toEqual([
      "搜索 6", "搜索 5", "搜索 4", "搜索 3", "搜索 2"
    ]);
    expect(window.localStorage.getItem(
      `ugxlsx:repository-search-history:${encodeURIComponent(repositoryView.path)}`
    )).toBe(JSON.stringify(["搜索 6", "搜索 5", "搜索 4", "搜索 3", "搜索 2"]));

    const historyItem = options.find((option) => option.text() === "搜索 4");
    await historyItem!.trigger("mousedown");
    await historyItem!.trigger("click");
    await flushPromises();
    expect((search.element as HTMLInputElement).value).toBe("搜索 4");
    expect(wrapper.find('[role="listbox"][aria-label="最近搜索"]').exists()).toBe(false);

    const clear = wrapper.get('button[aria-label="清空搜索"]');
    await clear.trigger("mousedown");
    await clear.trigger("click");
    expect((search.element as HTMLInputElement).value).toBe("");
    expect(wrapper.find('[role="listbox"][aria-label="最近搜索"]').exists()).toBe(true);

    wrapper.unmount();
    const otherRepository = { ...structuredClone(repositoryView), path: "/tmp/另一个仓库" };
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false,
            hasSession: false,
            error: "",
            mode: "repository",
            repository: otherRepository
          })
        }
      }
    };
    const otherWrapper = mount(App);
    await flushPromises();
    await otherWrapper.get('input[aria-label="筛选仓库文件或目录"]').trigger("focus");
    expect(otherWrapper.get('[role="listbox"][aria-label="最近搜索"]').text()).toContain("暂无搜索记录");
  });

  it("opens the recent repository dialog from switch repository without changing the open action", async () => {
    const otherRepository = {
      ...structuredClone(repositoryView),
      name: "第二个仓库",
      path: "/tmp/第二个仓库"
    };
    const recentRepositories = vi.fn(async () => [
      { name: repositoryView.name, path: repositoryView.path, available: true },
      { name: otherRepository.name, path: otherRepository.path, available: true },
      { name: "已移动仓库", path: "/tmp/已移动仓库", available: false }
    ]);
    const openRepository = vi.fn(async (): Promise<RepositoryResult> => ({
      repository: otherRepository,
      summary: null
    }));
    const selectRepository = vi.fn(async (): Promise<RepositoryResult> => ({
      repository: structuredClone(repositoryView),
      summary: null
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false,
            hasSession: false,
            error: "",
            mode: "repository",
            repository: structuredClone(repositoryView)
          }),
          RecentRepositories: recentRepositories,
          OpenRepository: openRepository,
          SelectRepository: selectRepository
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();

    const switchButton = wrapper.findAll("button").find((button) => button.text() === "切换仓库");
    await switchButton!.trigger("click");
    await flushPromises();

    expect(recentRepositories).toHaveBeenCalledOnce();
    expect(selectRepository).not.toHaveBeenCalled();
    expect(wrapper.get(".repository-switch-dialog").text()).toContain("最近打开的仓库");
    expect(wrapper.get(`button.recent-repository-item[title="${repositoryView.path}"]`).attributes("disabled")).toBeDefined();
    expect(wrapper.get('button.recent-repository-item[title="/tmp/已移动仓库"]').attributes("disabled")).toBeDefined();

    await wrapper.get(`button.recent-repository-item[title="${otherRepository.path}"]`).trigger("click");
    await flushPromises();
    expect(openRepository).toHaveBeenCalledWith(otherRepository.path);
    expect(wrapper.find(".repository-switch-dialog").exists()).toBe(false);
    expect(wrapper.text()).toContain(otherRepository.name);

    const openButton = wrapper.findAll("button").find((button) => button.text().includes("打开本地仓库"));
    await openButton!.trigger("click");
    await flushPromises();
    expect(selectRepository).toHaveBeenCalledOnce();
  });

  it("keeps unverified workbooks hidden while the semantic difference index is building", async () => {
    vi.useFakeTimers();
    const indexing = {
      ...structuredClone(repositoryView),
      differenceFiles: [],
      differenceIndexing: true
    };
    const repository = vi.fn(async (): Promise<RepositoryResult> => ({
      repository: structuredClone(repositoryView),
      summary: null
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false,
            hasSession: false,
            error: "",
            mode: "repository",
            repository: indexing
          }),
          Repository: repository
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    const differenceTab = wrapper.findAll('[role="tab"]').find((tab) => tab.text().includes("差异表"));
    expect(differenceTab?.text()).toContain("…");
    await differenceTab!.trigger("click");
    expect(wrapper.text()).toContain("正在后台建立差异表索引");
    expect(wrapper.find(".repository-tree").exists()).toBe(false);

    await vi.advanceTimersByTimeAsync(200);
    await flushPromises();
    expect(repository).toHaveBeenCalledOnce();
    expect(differenceTab?.text()).toContain("1");
    expect(wrapper.get(".repository-tree").text()).toContain("reward.xlsx");
    wrapper.unmount();
    vi.useRealTimers();
  });

  it("preserves collapsed repository folders while the difference index is polling", async () => {
    vi.useFakeTimers();
    const indexing = {
      ...structuredClone(repositoryView),
      differenceFiles: [],
      differenceIndexing: true
    };
    const repository = vi.fn(async (): Promise<RepositoryResult> => ({
      repository: structuredClone(repositoryView),
      summary: null
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false,
            hasSession: false,
            error: "",
            mode: "repository",
            repository: indexing
          }),
          Repository: repository
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    const configDirectory = wrapper.findAll(".tree-row").find((row) => row.attributes("title") === "config");
    expect(configDirectory?.attributes("aria-expanded")).toBe("true");
    await configDirectory!.trigger("click");
    expect(wrapper.get(".repository-tree").text()).not.toContain("reward.xlsx");

    await vi.advanceTimersByTimeAsync(200);
    await flushPromises();

    expect(repository).toHaveBeenCalledOnce();
    const collapsedDirectory = wrapper.findAll(".tree-row").find((row) => row.attributes("title") === "config");
    expect(collapsedDirectory?.attributes("aria-expanded")).toBe("false");
    expect(wrapper.get(".repository-tree").text()).not.toContain("reward.xlsx");
    const differenceTab = wrapper.findAll('[role="tab"]').find((tab) => tab.text().includes("差异表"));
    expect(differenceTab?.text()).toContain("1");
    wrapper.unmount();
    vi.useRealTimers();
  });

  it("shows a missing-ref file state without replacing the left workbook with an empty grid", async () => {
    const leftOnly = structuredClone(emptySummary);
    leftOnly.diff.sheetCount = 1;
    leftOnly.diff.sheets = [{
      name: "Sheet1", status: "equal", orderDifferent: false,
      differenceCount: 0, maxRow: 1, maxCol: 1, idColumn: 0, nextId: 0,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 0, conflictRowCount: 0, rows: []
    }];
    leftOnly.selectedSheet = "Sheet1";
    leftOnly.diff.rightFile = "";
    const missingView = {
      ...structuredClone(repositoryView),
      selectedFile: "config/activity/reward.xlsx",
      selectedRef: "refs/heads/develop",
      leftState: "ready",
      rightState: "missing",
      rightMessage: "该分支中不存在此表格\n分支：develop\n路径：config/activity/reward.xlsx"
    };
    const empty = { present: false, raw: "", display: "", type: "unset" };
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false, hasSession: true, error: "", mode: "repository", repository: missingView
          }),
          Summary: async () => leftOnly,
          Differences: async () => [],
          Region: async () => ({
            sheet: "Sheet1", fromRow: 1, toRow: 1, fromCol: 1, toCol: 1,
            cells: [{
              row: 1, col: 1, axis: "A1", status: "unchanged", rowStatus: "unchanged",
              left: { ...empty, present: true, raw: "工作区内容", display: "工作区内容", type: "string" },
              right: empty
            }]
          })
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.text()).toContain("工作区内容");
    expect(wrapper.text()).toContain("该分支中不存在此表格");
    expect(wrapper.findAll(".grid-panel")[1].find(".canvas").exists()).toBe(false);
    const copy = wrapper.findAll("button").find((button) => button.text().includes("复制到左侧"));
    expect(copy?.attributes("disabled")).toBeDefined();
    for (const label of ["上一处", "下一处"]) {
      const navigation = wrapper.findAll("button").find((button) => button.text() === label);
      expect(navigation?.attributes("disabled")).toBeDefined();
    }
    const sheetTab = wrapper.findAll('[role="tab"]').find((tab) => tab.text().includes("工作表/差异"));
    expect(sheetTab).toBeDefined();
    await sheetTab!.trigger("click");
    expect(wrapper.get(".repository-sheet-pane").text()).toContain("Sheet1");
    expect(wrapper.get(".repository-sheet-pane").text()).toContain("差异索引");
  });

  it("ignores an older repository file load that finishes after a newer selection", async () => {
    const pending = new Map<string, (result: RepositoryResult) => void>();
    const selectFile = vi.fn((path: string) => new Promise<RepositoryResult>((resolve) => {
      pending.set(path, resolve);
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false,
            hasSession: false,
            error: "",
            mode: "repository",
            repository: structuredClone(repositoryView)
          }),
          SelectRepositoryFile: selectFile
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    const fileRows = wrapper.findAll(".tree-row").filter((row) => row.text().includes(".xlsx"));
    expect(fileRows).toHaveLength(2);
    await fileRows.find((row) => row.text().includes("reward.xlsx"))!.trigger("click");
    await fileRows.find((row) => row.text().includes("配置.xlsx"))!.trigger("click");
    const makeResult = (path: string): RepositoryResult => ({
      repository: {
        ...structuredClone(repositoryView),
        selectedFile: path,
        selectedRef: "refs/heads/develop",
        leftState: "ready",
        rightState: "ready",
        comparisonActive: true
      },
      summary: { ...structuredClone(emptySummary), diff: { ...structuredClone(emptySummary.diff), leftFile: `/tmp/${path}` } }
    });
    pending.get("中文 目录/配置.xlsx")?.(makeResult("中文 目录/配置.xlsx"));
    await flushPromises();
    pending.get("config/activity/reward.xlsx")?.(makeResult("config/activity/reward.xlsx"));
    await flushPromises();
    expect(wrapper.get(".tree-row.selected").text()).toContain("配置.xlsx");
    expect(wrapper.text()).not.toContain("/tmp/config/activity/reward.xlsx");
  });

  it("shows a centered loading dialog until startup completes", async () => {
    let resolveBootstrap: ((value: { loading: boolean; hasSession: boolean; error: string }) => void) | undefined;
    window.go = {
      main: {
        Controller: {
          Bootstrap: () => new Promise((resolve) => {
            resolveBootstrap = resolve;
          })
        }
      }
    };
    const wrapper = mount(App);
    expect(wrapper.get(".loading-dialog").text()).toContain("正在加载并比较工作簿");
    resolveBootstrap?.({ loading: false, hasSession: false, error: "" });
    await flushPromises();
    expect(wrapper.find(".loading-overlay").exists()).toBe(false);
  });

  it("opens Save As with Ctrl+Shift+S", async () => {
    const saveAs = vi.fn(async () => emptySummary);
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => emptySummary,
          SaveAs: saveAs
        }
      }
    };
    mount(App);
    await flushPromises();
    window.dispatchEvent(new KeyboardEvent("keydown", {
      bubbles: true,
      cancelable: true,
      ctrlKey: true,
      shiftKey: true,
      key: "s"
    }));
    await flushPromises();
    expect(saveAs).toHaveBeenCalledOnce();
  });

  it("saves the current workbook with Ctrl+S", async () => {
    const dirty = structuredClone(emptySummary);
    dirty.dirty = true;
    const save = vi.fn(async () => ({ ...dirty, dirty: false }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => dirty,
          Save: save
        }
      }
    };
    mount(App);
    await flushPromises();
    window.dispatchEvent(new KeyboardEvent("keydown", {
      bubbles: true,
      cancelable: true,
      ctrlKey: true,
      key: "s"
    }));
    await flushPromises();
    expect(save).toHaveBeenCalledOnce();
  });

  it("allows Save As to export a repository workbook copy", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.dirty = true;
    const repository = {
      ...structuredClone(repositoryView),
      selectedFile: "config/activity/reward.xlsx",
      leftState: "ready",
      rightState: "ready",
      comparisonActive: true
    };
    const saveAs = vi.fn(async () => loaded);
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({
            loading: false,
            hasSession: true,
            error: "",
            mode: "repository",
            repository
          }),
          Summary: async () => loaded,
          SaveAs: saveAs
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    const button = wrapper.get('button[aria-label="另存为"]');
    expect(button.attributes("disabled")).toBeUndefined();
    expect(button.text()).toContain("导出副本");
    await button.trigger("click");
    await flushPromises();
    expect(saveAs).toHaveBeenCalledOnce();
  });

  it("renders a loaded worksheet region and difference navigation", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 1;
    loaded.diff.sheets = [{
      name: "数据 表", status: "modified", orderDifferent: false,
      differenceCount: 1, maxRow: 2, maxCol: 2, idColumn: 0, nextId: 0,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 1, conflictRowCount: 0,
      rows: [{ row: 1, id: "", status: "modified" }]
    }];
    loaded.selectedSheet = "数据 表";
    loaded.warnings = null as unknown as string[];
    const empty = { present: false, raw: "", display: "", type: "unset" };
    let editedToMatch = false;
    const editLeft = vi.fn(async () => {
      editedToMatch = true;
      const matched = structuredClone(loaded);
      matched.diff.equal = true;
      matched.diff.differentSheetCount = 0;
      matched.diff.differenceCount = 0;
      matched.diff.sheets[0].status = "equal";
      matched.diff.sheets[0].differenceCount = 0;
      matched.dirty = true;
      matched.undoCount = 1;
      return matched;
    });
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => editedToMatch ? [] : [{
            ref: { sheet: "数据 表", row: 1, col: 1 },
            status: "modified", rowStatus: "modified",
            left: { ...empty, present: true, raw: "旧", display: "旧", type: "string" },
            right: { ...empty, present: true, raw: "123", display: "123", type: "string" }
          }],
          Region: async () => ({
            sheet: "数据 表", fromRow: 1, toRow: 1, fromCol: 1, toCol: 1,
            cells: [{
              row: 1, col: 1, axis: "A1", status: editedToMatch ? "unchanged" : "modified",
              rowStatus: editedToMatch ? "unchanged" : "modified",
              left: {
                ...empty,
                present: true,
                raw: editedToMatch ? "123" : "旧",
                display: editedToMatch ? "123" : "旧",
                type: "string"
              },
              right: { ...empty, present: true, raw: "123", display: "123", type: "string" }
            }]
          }),
          EditLeft: editLeft
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.text()).toContain("数据 表");
    expect(wrapper.text()).toContain("旧");
    expect(wrapper.text()).toContain("123");
    expect(wrapper.text()).toContain("1 / 1");
    const activeFilter = wrapper.findAll(".diff-filter-tabs button")
      .find((button) => button.attributes("aria-selected") === "true");
    expect(activeFilter?.text()).toContain("修改");
    const scrolls = wrapper.findAll(".grid-scroll");
    expect(wrapper.findAll(".row-header-layer")).toHaveLength(2);
    expect(wrapper.findAll(".col-header-layer")).toHaveLength(2);
    (scrolls[0].element as HTMLElement).scrollLeft = 160;
    await scrolls[0].trigger("scroll");
    expect((scrolls[1].element as HTMLElement).scrollLeft).toBe(160);
    (scrolls[1].element as HTMLElement).scrollLeft = 240;
    await scrolls[1].trigger("scroll");
    expect((scrolls[0].element as HTMLElement).scrollLeft).toBe(240);
    expect(wrapper.get(".row-header").attributes("style")).toContain("left: 0px");
    const leftCell = wrapper.findAll(".grid-panel")[0].get(".cell");
    const rightCell = wrapper.findAll(".grid-panel")[1].get(".cell");
    expect(leftCell.classes()).toEqual(expect.arrayContaining(["difference", "cell-deleted"]));
    expect(rightCell.classes()).toEqual(expect.arrayContaining(["difference", "cell-added"]));
    await leftCell.trigger("dblclick");
    expect(leftCell.classes()).toEqual(expect.arrayContaining(["difference", "selected", "active"]));
    expect((wrapper.get(".inline-cell-editor").element as HTMLInputElement).value).toBe("旧");
    expect(wrapper.findAll(".difference-line")).toHaveLength(2);
    expect(wrapper.findAll(".difference-line")[0].text()).toContain("旧");
    expect(wrapper.findAll(".difference-line")[1].text()).toContain("123");
    await wrapper.get(".inline-cell-editor").setValue("123");
    await wrapper.get(".inline-cell-editor").trigger("keydown", { key: "Enter" });
    await flushPromises();
    expect(editLeft).toHaveBeenCalledWith("数据 表", 1, 1, "123", "text");
    const matchedCell = wrapper.findAll(".grid-panel")[0].get(".cell");
    expect(matchedCell.classes()).toEqual(expect.arrayContaining(["selected", "active"]));
    expect(matchedCell.classes()).not.toContain("difference");
    await matchedCell.trigger("dblclick");
    await wrapper.get(".inline-cell-editor").setValue("不提交");
    await wrapper.get(".inline-cell-editor").trigger("keydown", { key: "Escape" });
    expect(wrapper.find(".inline-cell-editor").exists()).toBe(false);
    expect(editLeft).toHaveBeenCalledTimes(1);
    expect(wrapper.find(".editor").exists()).toBe(false);
  });

  it("shows the opening left state only while the preview button or grid-scoped Tab is held", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.dirty = true;
    loaded.undoCount = 1;
    loaded.diff.sheetCount = 1;
    loaded.diff.sheets = [{
      name: "数据 表", status: "equal", orderDifferent: false,
      differenceCount: 0, maxRow: 1, maxCol: 2, idColumn: 0, nextId: 0,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 0, conflictRowCount: 0,
      rows: []
    }];
    loaded.selectedSheet = "数据 表";
    const empty = { present: false, raw: "", display: "", type: "unset" };
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => [],
          Region: async () => ({
            sheet: "数据 表", fromRow: 1, toRow: 1, fromCol: 1, toCol: 2,
            cells: [{
              row: 1, col: 1, axis: "A1", status: "unchanged", rowStatus: "unchanged",
              left: { ...empty, present: true, raw: "最新状态", display: "最新状态", type: "string" },
              originalLeft: { ...empty, present: true, raw: "打开时状态", display: "打开时状态", type: "string" },
              right: { ...empty }
            }, {
              row: 1, col: 2, axis: "B1", status: "unchanged", rowStatus: "unchanged",
              left: { ...empty, present: true, raw: "一直没改", display: "一直没改", type: "string" },
              originalLeft: { ...empty, present: true, raw: "一直没改", display: "一直没改", type: "string" },
              right: { ...empty }
            }]
          })
        }
      }
    };
    const wrapper = mount(App, { attachTo: document.body });
    await flushPromises();
    const leftPanel = wrapper.findAll(".grid-panel")[0];
    const previewButton = leftPanel.get(".original-preview-button");
    expect(leftPanel.text()).toContain("最新状态");
    expect(leftPanel.text()).not.toContain("打开时状态");

    await previewButton.trigger("pointerdown");
    expect(leftPanel.classes()).toContain("original-preview");
    expect(leftPanel.text()).toContain("打开时状态");
    const previewCells = leftPanel.findAll(".cell");
    expect(previewCells[0].classes()).toContain("original-preview-changed");
    expect(previewCells[1].classes()).not.toContain("original-preview-changed");
    window.dispatchEvent(new Event("pointerup"));
    await wrapper.vm.$nextTick();
    expect(leftPanel.text()).toContain("最新状态");

    const outsideTab = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
    document.body.dispatchEvent(outsideTab);
    expect(outsideTab.defaultPrevented).toBe(false);
    expect(leftPanel.text()).toContain("最新状态");

    const leftGrid = leftPanel.get(".grid-scroll");
    (leftGrid.element as HTMLElement).focus();
    const heldTab = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
    leftGrid.element.dispatchEvent(heldTab);
    await wrapper.vm.$nextTick();
    expect(heldTab.defaultPrevented).toBe(true);
    expect(leftPanel.text()).toContain("打开时状态");
    leftGrid.element.dispatchEvent(new KeyboardEvent("keyup", { key: "Tab", bubbles: true }));
    await wrapper.vm.$nextTick();
    expect(leftPanel.text()).toContain("最新状态");

    const rightGrid = wrapper.findAll(".grid-panel")[1].get(".grid-scroll");
    (rightGrid.element as HTMLElement).focus();
    rightGrid.element.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true }));
    await wrapper.vm.$nextTick();
    expect(leftPanel.text()).toContain("打开时状态");
    rightGrid.element.dispatchEvent(new KeyboardEvent("keyup", { key: "Tab", bubbles: true }));
    await wrapper.vm.$nextTick();
    expect(leftPanel.text()).toContain("最新状态");
  });

  it("prefetches a buffered region while scrolling before the current cells leave the viewport", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.sheetCount = 1;
    loaded.diff.sheets = [{
      name: "长表", status: "equal", orderDifferent: false,
      differenceCount: 0, maxRow: 500, maxCol: 8, idColumn: 0, nextId: 0,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 0, conflictRowCount: 0,
      rows: []
    }];
    loaded.selectedSheet = "长表";
    let resolvePrefetch: ((value: Region) => void) | undefined;
    const regionCall = vi.fn((
      name: string, fromRow: number, rowCount: number, fromCol: number, colCount: number
    ): Promise<Region> => {
      const value = {
        sheet: name,
        fromRow,
        toRow: fromRow + rowCount - 1,
        fromCol,
        toCol: fromCol + colCount - 1,
        cells: []
      };
      if (fromRow === 1) return Promise.resolve(value);
      return new Promise((resolve) => {
        resolvePrefetch = resolve;
      });
    });
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => [],
          Region: regionCall
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(regionCall).toHaveBeenCalledWith("长表", 1, 48, 1, 20);

    const scrolls = wrapper.findAll(".grid-scroll");
    for (const item of scrolls) {
      Object.defineProperty(item.element, "clientHeight", { configurable: true, value: 230 });
      Object.defineProperty(item.element, "clientWidth", { configurable: true, value: 480 });
    }
    vi.useFakeTimers();
    (scrolls[0].element as HTMLElement).scrollTop = 30 * 23;
    await scrolls[0].trigger("scroll");
    (scrolls[0].element as HTMLElement).scrollTop = 32 * 23;
    await scrolls[0].trigger("scroll");
    expect(regionCall).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(16);
    await flushPromises();
    expect(regionCall).toHaveBeenCalledTimes(2);
    expect(regionCall).toHaveBeenLastCalledWith("长表", 13, 48, 1, 20);
    const openRepository = wrapper.findAll("button").find((button) => button.text().includes("打开本地仓库"));
    expect(openRepository?.attributes("disabled")).toBeUndefined();
    resolvePrefetch?.({
      sheet: "长表", fromRow: 13, toRow: 60, fromCol: 1, toCol: 20, cells: []
    });
    await flushPromises();
  });

  it("opens the first differing sheet at its first prioritized difference", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 2;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 2;
    loaded.diff.sheets = [
      {
        name: "说明", status: "equal", orderDifferent: false,
        differenceCount: 0, maxRow: 8, maxCol: 2, idColumn: 0, nextId: 0,
        addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 0, conflictRowCount: 0,
        rows: []
      },
      {
        name: "数据", status: "modified", orderDifferent: false,
        differenceCount: 2, maxRow: 200, maxCol: 6, idColumn: 1, nextId: 0,
        addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 1, conflictRowCount: 1,
        rows: [
          { row: 5, id: "5", status: "modified" },
          { row: 120, id: "120", status: "conflict" }
        ]
      }
    ];
    loaded.selectedSheet = "说明";
    const empty = { present: false, raw: "", display: "", type: "unset" };
    const differences: CellDiff[] = [
      {
        ref: { sheet: "数据", row: 5, col: 1 },
        status: "modified", rowStatus: "modified",
        left: { ...empty, present: true, raw: "旧", display: "旧", type: "string" },
        right: { ...empty, present: true, raw: "新", display: "新", type: "string" }
      },
      {
        ref: { sheet: "数据", row: 120, col: 4 },
        status: "modified", rowStatus: "conflict",
        left: { ...empty, present: true, raw: "左", display: "左", type: "string" },
        right: { ...empty, present: true, raw: "右", display: "右", type: "string" }
      }
    ];
    const differencesCall = vi.fn(async (name: string) => name === "数据" ? differences : []);
    const regionCall = vi.fn(async (
      name: string, fromRow: number, rowCount: number, fromCol: number, colCount: number
    ): Promise<Region> => ({
      sheet: name,
      fromRow,
      toRow: fromRow + rowCount - 1,
      fromCol,
      toCol: fromCol + colCount - 1,
      cells: name === "数据" ? [{
        row: 120, col: 4, axis: "D120", status: "modified", rowStatus: "conflict",
        left: differences[1].left,
        right: differences[1].right
      }] : []
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: differencesCall,
          Region: regionCall
        }
      }
    };

    const wrapper = mount(App);
    await flushPromises();

    expect(differencesCall).toHaveBeenCalledWith("数据", 0, 10000);
    expect(regionCall).toHaveBeenCalledWith("数据", 85, 48, 1, 20);
    expect(wrapper.get(".sheet-item.active").text()).toContain("数据");
    expect(wrapper.text()).toContain("D120");
    const activeFilter = wrapper.findAll(".diff-filter-tabs button")
      .find((button) => button.attributes("aria-selected") === "true");
    expect(activeFilter?.text()).toContain("冲突");
  });

  it("keeps headers pinned and ignores stale regions during rapid bottom navigation", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 2;
    loaded.diff.sheets = [{
      name: "底部差异", status: "modified", orderDifferent: false,
      differenceCount: 2, maxRow: 1000, maxCol: 2, idColumn: 0, nextId: 0,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 2, conflictRowCount: 0,
      rows: [
        { row: 900, id: "", status: "modified" },
        { row: 1000, id: "", status: "modified" }
      ]
    }];
    loaded.selectedSheet = "底部差异";
    const empty = { present: false, raw: "", display: "", type: "unset" };
    const differences = [900, 1000].map((row) => ({
      ref: { sheet: "底部差异", row, col: 1 },
      status: "modified", rowStatus: "modified",
      left: { ...empty },
      right: { ...empty, present: true, raw: String(row), display: String(row), type: "number" }
    }));
    let regionCalls = 0;
    const pendingRegions: Array<(value: Region) => void> = [];
    const regionCall = vi.fn((...args: [string, number, number, number, number]): Promise<Region> => {
      void args;
      regionCalls++;
      if (regionCalls === 1) {
        return Promise.resolve({
          sheet: "底部差异", fromRow: 1, toRow: 48, fromCol: 1, toCol: 2, cells: []
        });
      }
      return new Promise((resolve) => pendingRegions.push(resolve));
    });
    const makeRegion = (row: number, display: string): Region => ({
      sheet: "底部差异", fromRow: 6, toRow: 53, fromCol: 1, toCol: 2,
      cells: [{
        row, col: 1, axis: `A${row}`, status: "modified", rowStatus: "modified",
        left: { ...empty },
        right: { ...empty, present: true, raw: display, display, type: "string" }
      }]
    });
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => differences,
          Region: regionCall
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    for (const item of wrapper.findAll(".grid-scroll")) {
      let actualTop = 0;
      Object.defineProperty(item.element, "scrollTop", {
        configurable: true,
        get: () => actualTop,
        set: (value: number) => {
          actualTop = Math.min(value, 120);
        }
      });
    }
    const next = wrapper.findAll("button").find((button) => button.text() === "下一处");
    expect(next).toBeDefined();
    next!.element.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    next!.element.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    expect(pendingRegions).toHaveLength(2);

    pendingRegions[1](makeRegion(1000, "最新区域"));
    await flushPromises();
    pendingRegions[0](makeRegion(900, "过期区域"));
    await flushPromises();

    for (const item of wrapper.findAll(".grid-scroll")) {
      expect((item.element as HTMLElement).scrollTop).toBe(120);
    }
    expect(wrapper.findAll(".col-header-layer")).toHaveLength(2);
    expect(wrapper.get(".col-header").attributes("style")).toContain("top: 0px");
    expect(regionCall.mock.calls.at(-1)?.[1]).toBe(1);
    expect(wrapper.text()).toContain("最新区域");
    expect(wrapper.text()).not.toContain("过期区域");
  });

  it("supports drag selection, context-menu copy, and batch coordinates", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 2;
    loaded.diff.sheets = [{
      name: "批量", status: "modified", orderDifferent: false,
      differenceCount: 2, maxRow: 1, maxCol: 2, idColumn: 0, nextId: 0,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 1, conflictRowCount: 0,
      rows: [{ row: 1, id: "", status: "modified" }]
    }];
    loaded.selectedSheet = "批量";
    const empty = { present: false, raw: "", display: "", type: "unset" };
    const cells = [1, 2].map((col) => ({
      row: 1, col, axis: `${col === 1 ? "A" : "B"}1`, status: "modified",
      rowStatus: "modified" as const,
      left: { ...empty, present: true, raw: `旧${col}`, display: `旧${col}`, type: "string" },
      right: { ...empty, present: true, raw: `新${col}`, display: `新${col}`, type: "string" }
    }));
    const copyMany = vi.fn(async () => ({ ...loaded, dirty: true, undoCount: 1 }));
    const undo = vi.fn(async () => ({ ...loaded, dirty: false, undoCount: 0 }));
    const regionCall = vi.fn(async (...args: [string, number, number, number, number]) => {
      void args;
      return { sheet: "批量", fromRow: 1, toRow: 1, fromCol: 1, toCol: 2, cells };
    });
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => cells.map((cell) => ({
            ref: { sheet: "批量", row: cell.row, col: cell.col },
            status: cell.status, rowStatus: cell.rowStatus, left: cell.left, right: cell.right
          })),
          Region: regionCall,
          CopyRightToLeftMany: copyMany,
          Undo: undo
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.get(".zoom-button").text()).toBe("缩放 100%");
    wrapper.get(".grid-scroll").element.dispatchEvent(new WheelEvent("wheel", {
      bubbles: true,
      cancelable: true,
      ctrlKey: true,
      deltaY: -100,
      clientX: 40,
      clientY: 40
    }));
    await flushPromises();
    expect(wrapper.get(".zoom-button").text()).toBe("缩放 110%");
    expect(window.localStorage.length).toBe(1);
    await wrapper.get(".zoom-button").trigger("click");
    expect(wrapper.get(".zoom-button").text()).toBe("缩放 100%");
    const gridScrolls = wrapper.findAll(".grid-scroll");
    const initialTop = (gridScrolls[0].element as HTMLElement).scrollTop;
    const initialLeft = (gridScrolls[0].element as HTMLElement).scrollLeft;
    const leftWheel = new WheelEvent("wheel", {
      bubbles: true,
      cancelable: true,
      deltaX: 36,
      deltaY: 92
    });
    gridScrolls[0].element.dispatchEvent(leftWheel);
    expect(leftWheel.defaultPrevented).toBe(true);
    await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));
    expect((gridScrolls[0].element as HTMLElement).scrollTop).toBe(initialTop + 92);
    expect((gridScrolls[1].element as HTMLElement).scrollTop).toBe(initialTop + 92);
    expect((gridScrolls[0].element as HTMLElement).scrollLeft).toBe(initialLeft + 36);
    expect((gridScrolls[1].element as HTMLElement).scrollLeft).toBe(initialLeft + 36);

    const rightWheel = new WheelEvent("wheel", {
      bubbles: true,
      cancelable: true,
      deltaY: -24
    });
    gridScrolls[1].element.dispatchEvent(rightWheel);
    expect(rightWheel.defaultPrevented).toBe(true);
    await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));
    expect((gridScrolls[0].element as HTMLElement).scrollTop).toBe(initialTop + 68);
    expect((gridScrolls[1].element as HTMLElement).scrollTop).toBe(initialTop + 68);
    const rightCells = wrapper.findAll(".grid-panel")[1].findAll(".cell");
    await rightCells[0].trigger("pointerdown", { button: 0 });
    await rightCells[1].trigger("pointerenter");
    expect(wrapper.text()).toContain("已选 2 个单元格");
    (wrapper.get(".grid-scroll").element as HTMLElement).scrollTop = 250;
    await rightCells[1].trigger("contextmenu", { clientX: 120, clientY: 120 });
    expect(wrapper.find(".context-menu").exists()).toBe(true);
    await wrapper.get(".context-menu button").trigger("click");
    await flushPromises();
    expect(copyMany).toHaveBeenCalledWith("批量", [
      { row: 1, col: 1 },
      { row: 1, col: 2 }
    ]);
    expect(regionCall.mock.calls.at(-1)?.[1]).toBe(1);
    window.dispatchEvent(new KeyboardEvent("keydown", {
      bubbles: true,
      cancelable: true,
      ctrlKey: true,
      key: "z"
    }));
    await flushPromises();
    expect(undo).toHaveBeenCalledOnce();
  });

  it("shows conflict colors, row counts, and multi-row ID append actions", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 5;
    loaded.diff.sheets = [{
      name: "冲突", status: "modified", orderDifferent: false,
      differenceCount: 5, maxRow: 6, maxCol: 3, idColumn: 1, nextId: 3,
      addedRowCount: 1, deletedRowCount: 0, modifiedRowCount: 0, conflictRowCount: 2,
      rows: [
        { row: 2, id: "1", status: "conflict" },
        { row: 3, id: "2", status: "conflict" },
        { row: 6, id: "6", status: "added" }
      ]
    }];
    loaded.selectedSheet = "冲突";
    const value = (raw: string) => ({
      present: true, raw, display: raw, type: /^\d+$/.test(raw) ? "number" : "string"
    });
    const cells = [
      { row: 1, col: 1, axis: "A1", status: "unchanged", rowStatus: "unchanged", left: value("id"), right: value("id") },
      { row: 1, col: 2, axis: "B1", status: "unchanged", rowStatus: "unchanged", left: value("name"), right: value("name") },
      { row: 1, col: 3, axis: "C1", status: "unchanged", rowStatus: "unchanged", left: value("value"), right: value("value") },
      { row: 2, col: 1, axis: "A2", status: "unchanged", rowStatus: "conflict", left: value("1"), right: value("1") },
      { row: 2, col: 2, axis: "B2", status: "modified", rowStatus: "conflict", left: value("左一"), right: value("右一") },
      { row: 2, col: 3, axis: "C2", status: "modified", rowStatus: "conflict", left: value("左二"), right: value("右二") },
      { row: 3, col: 1, axis: "A3", status: "unchanged", rowStatus: "conflict", left: value("2"), right: value("2") },
      { row: 3, col: 2, axis: "B3", status: "modified", rowStatus: "conflict", left: value("左三"), right: value("右三") },
      { row: 3, col: 3, axis: "C3", status: "modified", rowStatus: "conflict", left: value("左四"), right: value("右四") },
      { row: 6, col: 1, axis: "A6", status: "right-added", rowStatus: "added", left: { present: false, raw: "", display: "", type: "unset" }, right: value("6") }
    ] as Region["cells"];
    const missing = { present: false, raw: "", display: "", type: "unset" };
    const appendedCells = [
      { row: 4, col: 1, axis: "A4", status: "left-added", rowStatus: "deleted", left: value("10"), right: missing },
      { row: 4, col: 2, axis: "B4", status: "left-added", rowStatus: "deleted", left: value("右一"), right: missing },
      { row: 4, col: 3, axis: "C4", status: "left-added", rowStatus: "deleted", left: value("右二"), right: missing },
      { row: 5, col: 1, axis: "A5", status: "left-added", rowStatus: "deleted", left: value("11"), right: missing },
      { row: 5, col: 2, axis: "B5", status: "left-added", rowStatus: "deleted", left: value("右三"), right: missing },
      { row: 5, col: 3, axis: "C5", status: "left-added", rowStatus: "deleted", left: value("右四"), right: missing }
    ] as Region["cells"];
    const handled = structuredClone(loaded);
    handled.diff.sheets[0].maxRow = 6;
    handled.diff.sheets[0].deletedRowCount = 2;
    handled.diff.sheets[0].rows = [
      ...(handled.diff.sheets[0].rows ?? []),
      { row: 4, id: "10", status: "deleted" },
      { row: 5, id: "11", status: "deleted" }
    ];
    handled.resolutions = [
      { sheet: "冲突", sourceRow: 2, targetRow: 4, targetId: "10", kind: "append-specified" },
      { sheet: "冲突", sourceRow: 3, targetRow: 5, targetId: "11", kind: "append-specified" }
    ];
    handled.dirty = true;
    handled.undoCount = 1;
    let appended = false;
    const appendRows = vi.fn(async () => {
      appended = true;
      return handled;
    });
    const filteredRegion = vi.fn(async (
      _sheet: string,
      statuses: string[],
      fromRow: number,
      _rowCount: number,
      fromCol: number,
      colCount: number
    ) => {
      const currentSummary = appended ? handled : loaded;
      const sourceRows = (currentSummary.diff.sheets[0].rows ?? [])
        .filter((row) => statuses.includes(row.status) && row.row <= 3);
      const currentCells = appended ? [...cells, ...appendedCells] : cells;
      const packed = sourceRows.flatMap((row, index) => {
        const resolution = currentSummary.resolutions.find((item) => item.sourceRow === row.row);
        const leftRow = resolution?.targetRow ?? row.row;
        return Array.from({ length: colCount }, (_, colIndex) => {
          const col = fromCol + colIndex;
          const sourceCell = currentCells.find((cell) => cell.row === row.row && cell.col === col) ?? {
            row: row.row,
            col,
            axis: `${String.fromCharCode(64 + col)}${row.row}`,
            status: "unchanged",
            rowStatus: row.status,
            left: missing,
            right: missing
          };
          const leftCell = currentCells.find((cell) => cell.row === leftRow && cell.col === col) ?? sourceCell;
          return {
            ...sourceCell,
            row: index + 1,
            sourceRow: row.row,
            leftRow,
            rightRow: row.row,
            left: leftCell.left
          };
        });
      });
      return {
        sheet: "冲突", fromRow, toRow: sourceRows.length,
        fromCol, toCol: fromCol + colCount - 1,
        filtered: true, totalRows: sourceRows.length, cells: packed
      };
    });
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => (appended ? [...cells, ...appendedCells] : cells)
            .filter((cell) => cell.status !== "unchanged")
            .map((cell) => ({
              ref: { sheet: "冲突", row: cell.row, col: cell.col },
              status: cell.status,
              rowStatus: cell.rowStatus,
              left: cell.left,
              right: cell.right
            })),
          Region: async () => ({
            sheet: "冲突", fromRow: 1, toRow: 6, fromCol: 1, toCol: 3,
            cells: appended ? [...cells, ...appendedCells] : cells
          }),
          FilteredRegion: filteredRegion,
          AppendRowsRightToLeft: appendRows
        }
      }
    };

    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.get(".row-status-chip.conflict").text()).toBe("冲突 2");
    expect(wrapper.findAll(".grid-panel")[1].findAll(".cell.row-conflict")).toHaveLength(6);
    const activeConflictFilter = wrapper.findAll(".diff-filter-tabs button")
      .find((button) => button.attributes("aria-selected") === "true");
    expect(activeConflictFilter?.text()).toContain("冲突");
    const addedFilter = wrapper.findAll(".diff-filter-tabs button")
      .find((button) => button.text().includes("增加"));
    await addedFilter!.trigger("click");
    expect(wrapper.findAll(".diff-list button")).toHaveLength(1);
    expect(wrapper.get(".diff-list button").text()).toContain("6:A");
    const conflictFilter = wrapper.findAll(".diff-filter-tabs button")
      .find((button) => button.text().includes("冲突"));
    await conflictFilter!.trigger("click");

    const rightPanel = wrapper.findAll(".grid-panel")[1];
    const rowHeaders = rightPanel.findAll(".row-header");
    await rowHeaders[0].trigger("pointerdown", { button: 0 });
    await rowHeaders[1].trigger("pointerenter");
    const rowThreeCell = rightPanel.findAll(".cell").find((cell) => cell.text() === "2");
    await rowThreeCell!.trigger("contextmenu", { clientX: 120, clientY: 120 });
    const menuText = wrapper.get(".context-menu").text();
    expect(menuText).toContain("覆盖单元格到左侧");
    expect(menuText).toContain("覆盖整行到左侧");
    expect(menuText).toContain("将整行新增为 id:3~4 到左侧");
    expect(menuText).toContain("将整行新增到指定 id 到左侧");

    const specified = wrapper.findAll(".context-menu button")
      .find((button) => button.text().includes("指定 id"));
    await specified!.trigger("click");
    const inputs = wrapper.findAll(".id-dialog input");
    expect(inputs).toHaveLength(2);
    await inputs[0].setValue("10");
    await inputs[1].setValue("11");
    await wrapper.get(".id-dialog").trigger("submit");
    await flushPromises();
    expect(appendRows).toHaveBeenCalledWith("冲突", [2, 3], ["10", "11"]);
    const markers = wrapper.findAll(".resolution-marker");
    expect(markers.map((marker) => marker.text())).toEqual(expect.arrayContaining([
      "已新增为指定 ID 10",
      "已新增为指定 ID 11"
    ]));
    const appendedLeftCell = wrapper.findAll(".grid-panel")[0].findAll(".cell")
      .find((cell) => cell.text() === "10");
    expect(appendedLeftCell?.classes()).toContain("cell-added");
    expect(appendedLeftCell?.classes()).not.toContain("cell-deleted");
  });

  it("filters packed rows with current-sheet metrics, shortcuts, and no persistence", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 37;
    loaded.diff.sheets = [{
      name: "筛选", status: "modified", orderDifferent: false,
      differenceCount: 4, maxRow: 95, maxCol: 1, idColumn: 0, nextId: 0,
      addedRowCount: 1, deletedRowCount: 1, modifiedRowCount: 1, conflictRowCount: 1,
      rows: [
        { row: 2, status: "added" },
        { row: 3, status: "deleted" },
        { row: 95, status: "modified" },
        { row: 5, status: "conflict" }
      ]
    }, {
      name: "其他表", status: "modified", orderDifferent: false,
      differenceCount: 33, maxRow: 34, maxCol: 1, idColumn: 1, nextId: 35,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 0, conflictRowCount: 33,
      rows: []
    }];
    loaded.selectedSheet = "筛选";
    const value = (raw: string) => ({ present: true, raw, display: raw, type: "string" });
    const missing = { present: false, raw: "", display: "", type: "unset" };
    const cells = [
      { row: 1, col: 1, axis: "A1", status: "unchanged", rowStatus: "unchanged", left: value("表头"), right: value("表头") },
      { row: 2, col: 1, axis: "A2", status: "right-added", rowStatus: "added", left: missing, right: value("新增值") },
      { row: 3, col: 1, axis: "A3", status: "left-added", rowStatus: "deleted", left: value("删除值"), right: missing },
      { row: 95, col: 1, axis: "A95", status: "modified", rowStatus: "modified", left: value("修改前"), right: value("修改后") },
      { row: 5, col: 1, axis: "A5", status: "modified", rowStatus: "conflict", left: value("冲突左"), right: value("冲突右") }
    ] as Region["cells"];
    const differences = cells.slice(1).map((cell) => ({
      ref: { sheet: "筛选", row: cell.row, col: cell.col },
      status: cell.status,
      rowStatus: cell.rowStatus,
      left: cell.left,
      right: cell.right
    }));
    const region = vi.fn(async () => ({
      sheet: "筛选", fromRow: 1, toRow: 5, fromCol: 1, toCol: 20, cells
    }));
    const filteredRegion = vi.fn(async (
      _sheet: string,
      statuses: string[],
      fromRow: number,
      _rowCount: number,
      fromCol: number,
      colCount: number
    ) => {
      const rows = (loaded.diff.sheets[0].rows ?? []).filter((row) => statuses.includes(row.status));
      const packed = rows.flatMap((row, index) => Array.from({ length: colCount }, (_, offset) => {
        const col = fromCol + offset;
        const source = cells.find((cell) => cell.row === row.row && cell.col === col) ?? {
          row: row.row, col, axis: `A${row.row}`, status: "unchanged", rowStatus: row.status,
          left: missing, right: missing
        };
        return {
          ...source,
          row: index + 1,
          sourceRow: row.row,
          leftRow: row.row,
          rightRow: row.row
        };
      }));
      return {
        sheet: "筛选", fromRow, toRow: rows.length,
        fromCol, toCol: fromCol + colCount - 1,
        filtered: true, totalRows: rows.length, cells: packed
      };
    });
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => differences,
          Region: region,
          FilteredRegion: filteredRegion
        }
      }
    };

    let wrapper = mount(App);
    await flushPromises();
    expect(wrapper.get(".summary-context").text()).toContain("全部数据");
    expect(wrapper.findAll(".summary-metric").every((button) => button.attributes("aria-pressed") === "false")).toBe(true);
    expect(wrapper.get(".result-summary-title strong").text()).toBe("4 处差异");
    expect(wrapper.findAll(".summary-metric").map((button) => button.get("strong").text())).toEqual(["1", "1", "1", "1"]);

    await wrapper.findAll(".summary-metric")[0].trigger("click");
    await wrapper.findAll(".summary-metric")[1].trigger("click");
    await flushPromises();
    expect(filteredRegion).toHaveBeenLastCalledWith("筛选", ["added", "deleted"], 1, 48, 1, 20);
    expect(wrapper.get(".summary-context").text()).toContain("增加行、删除行");
    expect(wrapper.findAll(".grid-panel")[0].text()).toContain("删除值");
    expect(wrapper.findAll(".grid-panel")[1].text()).toContain("新增值");
    expect(window.localStorage.getItem("ugxlsx:row-filters:v1")).toBeNull();

    window.dispatchEvent(new KeyboardEvent("keydown", { key: "5" }));
    await flushPromises();
    expect(wrapper.get(".summary-context").text()).toContain("全部差异行");
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "5" }));
    await flushPromises();
    expect(wrapper.get(".summary-context").text()).toContain("全部数据");
    expect(region).toHaveBeenCalled();

    const grids = wrapper.findAll<HTMLElement>(".grid-scroll");
    grids[0].element.scrollTop = 94 * 23;
    grids[1].element.scrollTop = 94 * 23;
    await wrapper.findAll(".summary-metric")[2].trigger("click");
    await flushPromises();
    expect(grids[0].element.scrollTop).toBe(0);
    await wrapper.findAll(".summary-metric")[2].trigger("click");
    await flushPromises();
    expect(grids[0].element.scrollTop).toBe(94 * 23);
    expect(grids[1].element.scrollTop).toBe(94 * 23);

    window.dispatchEvent(new KeyboardEvent("keydown", { key: "3" }));
    await flushPromises();
    expect(window.localStorage.getItem("ugxlsx:row-filters:v1")).toBeNull();
    wrapper.unmount();
    wrapper = mount(App);
    await flushPromises();
    expect(wrapper.get(".summary-context").text()).toContain("全部数据");
    expect(wrapper.findAll(".summary-metric").every((button) => button.attributes("aria-pressed") === "false")).toBe(true);
  });

  it("keeps copy actions for selections containing unchanged rows and separates conflicts", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 4;
    loaded.diff.sheets = [{
      name: "混合选择", status: "modified", orderDifferent: false,
      differenceCount: 4, maxRow: 5, maxCol: 1, idColumn: 0, nextId: 0,
      addedRowCount: 1, deletedRowCount: 1, modifiedRowCount: 1, conflictRowCount: 1,
      rows: [
        { row: 2, id: "", status: "added" },
        { row: 3, id: "", status: "deleted" },
        { row: 4, id: "", status: "modified" },
        { row: 5, id: "", status: "conflict" }
      ]
    }];
    loaded.selectedSheet = "混合选择";
    const value = (raw: string) => ({ present: true, raw, display: raw, type: "string" });
    const cells = [
      { row: 1, col: 1, axis: "A1", status: "unchanged", rowStatus: "unchanged", left: value("相同"), right: value("相同") },
      { row: 2, col: 1, axis: "A2", status: "right-added", rowStatus: "added", left: value(""), right: value("增加") },
      { row: 3, col: 1, axis: "A3", status: "left-deleted", rowStatus: "deleted", left: value("删除"), right: value("") },
      { row: 4, col: 1, axis: "A4", status: "modified", rowStatus: "modified", left: value("旧值"), right: value("新值") },
      { row: 5, col: 1, axis: "A5", status: "modified", rowStatus: "conflict", left: value("左冲突"), right: value("右冲突") }
    ] as Region["cells"];
    const differences = cells.slice(1).map((cell) => ({
      ref: { sheet: "混合选择", row: cell.row, col: cell.col },
      status: cell.status,
      rowStatus: cell.rowStatus,
      left: cell.left,
      right: cell.right
    }));
    const copyMany = vi.fn(async () => ({ ...loaded, dirty: true, undoCount: 1 }));
    const copyRows = vi.fn(async () => ({ ...loaded, dirty: true, undoCount: 1 }));
    const appendRows = vi.fn(async () => ({
      ...loaded,
      dirty: true,
      undoCount: 1,
      resolutions: [{ sheet: "混合选择", sourceRow: 2, targetRow: 6, kind: "append-row" }]
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => differences,
          Region: async () => ({
            sheet: "混合选择", fromRow: 1, toRow: 5, fromCol: 1, toCol: 1, cells
          }),
          CopyRightToLeftMany: copyMany,
          CopyRowsRightToLeft: copyRows,
          AppendRowsRightToLeft: appendRows
        }
      }
    };

    const wrapper = mount(App);
    await flushPromises();
    let rightPanel = wrapper.findAll(".grid-panel")[1];
    let rowHeaders = rightPanel.findAll(".row-header");
    await rowHeaders[0].trigger("pointerdown", { button: 0 });
    await rowHeaders[3].trigger("pointerenter");
    await rightPanel.findAll(".cell")[0].trigger("contextmenu", { clientX: 120, clientY: 120 });
    expect(wrapper.get(".context-menu").text()).toContain("复制单元格到左侧");
    expect(wrapper.get(".context-menu").text()).toContain("复制整行到左侧");
    expect(wrapper.get(".context-menu").text()).not.toContain("将整行新增到左侧");
    await wrapper.findAll(".context-menu button")[0].trigger("click");
    await flushPromises();
    expect(copyMany).toHaveBeenCalledWith("混合选择", [
      { row: 2, col: 1 },
      { row: 3, col: 1 },
      { row: 4, col: 1 }
    ]);

    rightPanel = wrapper.findAll(".grid-panel")[1];
    await rightPanel.findAll(".cell")[0].trigger("contextmenu", { clientX: 120, clientY: 120 });
    const copyRowButton = wrapper.findAll(".context-menu button")
      .find((button) => button.text() === "复制整行到左侧");
    await copyRowButton!.trigger("click");
    await flushPromises();
    expect(copyRows).toHaveBeenCalledWith("混合选择", [2, 3, 4]);

    expect(appendRows).not.toHaveBeenCalled();

    rightPanel = wrapper.findAll(".grid-panel")[1];
    rowHeaders = rightPanel.findAll(".row-header");
    await rowHeaders[0].trigger("pointerdown", { button: 0 });
    await rowHeaders[4].trigger("pointerenter");
    await rightPanel.findAll(".cell")[0].trigger("contextmenu", { clientX: 120, clientY: 120 });
    const menu = wrapper.get(".context-menu");
    expect(menu.text()).toContain("选中的内容中包含冲突，请单独处理冲突部分");
    expect(menu.findAll("button")).toHaveLength(0);
  });

  it("uses raw row append only for conflicts whose id column is not numeric", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 1;
    loaded.diff.sheets = [{
      name: "属性", status: "modified", orderDifferent: false,
      differenceCount: 1, maxRow: 2, maxCol: 2, idColumn: 1, nextId: 0,
      addedRowCount: 0, deletedRowCount: 0, modifiedRowCount: 0, conflictRowCount: 1,
      rows: [{ row: 2, id: "活动id", status: "conflict" }]
    }];
    loaded.selectedSheet = "属性";
    const value = (raw: string) => ({ present: true, raw, display: raw, type: "string" });
    const cells = [
      { row: 1, col: 1, axis: "A1", status: "unchanged", rowStatus: "unchanged", left: value("id"), right: value("id") },
      { row: 1, col: 2, axis: "B1", status: "unchanged", rowStatus: "unchanged", left: value("name"), right: value("name") },
      { row: 2, col: 1, axis: "A2", status: "unchanged", rowStatus: "conflict", left: value("活动id"), right: value("活动id") },
      { row: 2, col: 2, axis: "B2", status: "modified", rowStatus: "conflict", left: value("旧值"), right: value("新值") }
    ] as Region["cells"];
    const appendRows = vi.fn(async () => ({
      ...loaded,
      dirty: true,
      undoCount: 1,
      resolutions: [{ sheet: "属性", sourceRow: 2, targetRow: 3, kind: "append-row" as const }]
    }));
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => [{
            ref: { sheet: "属性", row: 2, col: 2 },
            status: "modified", rowStatus: "conflict", left: value("旧值"), right: value("新值")
          }],
          Region: async () => ({
            sheet: "属性", fromRow: 1, toRow: 2, fromCol: 1, toCol: 2, cells
          }),
          AppendRowsRightToLeft: appendRows
        }
      }
    };

    const wrapper = mount(App);
    await flushPromises();
    const rightPanel = wrapper.findAll(".grid-panel")[1];
    const sourceCell = rightPanel.findAll(".cell").find((cell) => cell.text() === "活动id");
    await sourceCell!.trigger("contextmenu", { clientX: 120, clientY: 120 });
    const menu = wrapper.get(".context-menu");
    expect(menu.text()).toContain("将整行新增到左侧");
    expect(menu.text()).not.toContain("新增为 id:");
    expect(menu.text()).not.toContain("指定 id");
    const appendButton = menu.findAll("button")
      .find((button) => button.text() === "将整行新增到左侧");
    await appendButton!.trigger("click");
    await flushPromises();
    expect(appendRows).toHaveBeenCalledWith("属性", [2], []);
  });
});
