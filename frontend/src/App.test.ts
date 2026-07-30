import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App.vue";
import type { Region, RepositoryResult, RepositoryView, Summary } from "./types";

const emptySummary: Summary = {
  options: {
    title: "",
    leftLabel: "本地（可编辑）",
    rightLabel: "对比来源（只读）",
    readonlyLeft: false,
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
  dirty: false,
  undoCount: 0,
  warnings: [],
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
  branches: [
    { name: "develop", fullName: "refs/heads/develop", kind: "local" },
    { name: "origin/main", fullName: "refs/remotes/origin/main", kind: "remote" }
  ],
  defaultRef: "refs/heads/develop",
  selectedFile: "",
  selectedRef: "",
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
    const search = wrapper.get('input[aria-label="筛选仓库文件或目录"]');
    await search.setValue("中文 目录");
    expect(wrapper.get(".repository-tree").text()).toContain("中文 目录");
    expect(wrapper.get(".repository-tree").text()).toContain("配置.xlsx");
    expect(wrapper.get(".repository-tree").text()).not.toContain("reward.xlsx");
    await search.setValue("不存在");
    expect(wrapper.find(".repository-tree").exists()).toBe(false);
    expect(wrapper.text()).toContain("没有匹配的文件或目录");
  });

  it("shows a missing-ref file state without replacing the left workbook with an empty grid", async () => {
    const leftOnly = structuredClone(emptySummary);
    leftOnly.diff.sheetCount = 1;
    leftOnly.diff.sheets = [{
      name: "Sheet1", status: "equal", orderDifferent: false,
      differenceCount: 0, maxRow: 1, maxCol: 1
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
              row: 1, col: 1, axis: "A1", status: "unchanged",
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

  it("renders a loaded worksheet region and difference navigation", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 1;
    loaded.diff.sheets = [{
      name: "数据 表", status: "modified", orderDifferent: false,
      differenceCount: 1, maxRow: 2, maxCol: 2
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
            status: "modified",
            left: { ...empty, present: true, raw: "旧", display: "旧", type: "string" },
            right: { ...empty, present: true, raw: "123", display: "123", type: "string" }
          }],
          Region: async () => ({
            sheet: "数据 表", fromRow: 1, toRow: 1, fromCol: 1, toCol: 1,
            cells: [{
              row: 1, col: 1, axis: "A1", status: editedToMatch ? "unchanged" : "modified",
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
    expect(wrapper.text()).toContain("0 / 1");
    const leftCell = wrapper.findAll(".grid-panel")[0].get(".cell");
    expect(leftCell.classes()).toContain("difference");
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

  it("keeps headers pinned and ignores stale regions during rapid bottom navigation", async () => {
    const loaded = structuredClone(emptySummary);
    loaded.diff.equal = false;
    loaded.diff.sheetCount = 1;
    loaded.diff.differentSheetCount = 1;
    loaded.diff.differenceCount = 2;
    loaded.diff.sheets = [{
      name: "底部差异", status: "modified", orderDifferent: false,
      differenceCount: 2, maxRow: 1000, maxCol: 2
    }];
    loaded.selectedSheet = "底部差异";
    const empty = { present: false, raw: "", display: "", type: "unset" };
    const differences = [900, 1000].map((row) => ({
      ref: { sheet: "底部差异", row, col: 1 },
      status: "modified",
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
        row, col: 1, axis: `A${row}`, status: "modified",
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

    expect(wrapper.get(".col-header").attributes("style")).toContain("top: 120px");
    expect(regionCall.mock.calls.at(-1)?.[1]).toBe(6);
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
      differenceCount: 2, maxRow: 1, maxCol: 2
    }];
    loaded.selectedSheet = "批量";
    const empty = { present: false, raw: "", display: "", type: "unset" };
    const cells = [1, 2].map((col) => ({
      row: 1, col, axis: `${col === 1 ? "A" : "B"}1`, status: "modified",
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
            status: cell.status, left: cell.left, right: cell.right
          })),
          Region: regionCall,
          CopyRightToLeftMany: copyMany,
          Undo: undo
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.get(".zoom-button").text()).toBe("100%");
    wrapper.get(".grid-scroll").element.dispatchEvent(new WheelEvent("wheel", {
      bubbles: true,
      cancelable: true,
      ctrlKey: true,
      deltaY: -100,
      clientX: 40,
      clientY: 40
    }));
    await flushPromises();
    expect(wrapper.get(".zoom-button").text()).toBe("110%");
    expect(window.localStorage.length).toBe(1);
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
    expect(regionCall.mock.calls.at(-1)?.[1]).toBe(11);
    window.dispatchEvent(new KeyboardEvent("keydown", {
      bubbles: true,
      cancelable: true,
      ctrlKey: true,
      key: "z"
    }));
    await flushPromises();
    expect(undo).toHaveBeenCalledOnce();
  });
});
