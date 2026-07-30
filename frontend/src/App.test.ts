import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App.vue";
import type { Region, Summary } from "./types";

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

  it("opens file selection from the welcome screen and displays both paths", async () => {
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.text()).toContain("比较并安全合并 Excel");
    await wrapper.get("button.primary.large").trigger("click");
    await flushPromises();
    expect(selectAndOpen).toHaveBeenCalledOnce();
    expect(wrapper.text()).toContain("/tmp/左侧 文件.xlsx");
    expect(wrapper.text()).toContain("/tmp/右侧 文件.xlsx");
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
    window.go = {
      main: {
        Controller: {
          Bootstrap: async () => ({ loading: false, hasSession: true, error: "" }),
          Summary: async () => loaded,
          Differences: async () => [{
            ref: { sheet: "数据 表", row: 1, col: 1 },
            status: "modified",
            left: { ...empty, present: true, raw: "旧", display: "旧", type: "string" },
            right: { ...empty, present: true, raw: "新", display: "新", type: "string" }
          }],
          Region: async () => ({
            sheet: "数据 表", fromRow: 1, toRow: 1, fromCol: 1, toCol: 1,
            cells: [{
              row: 1, col: 1, axis: "A1", status: "modified",
              left: { ...empty, present: true, raw: "旧", display: "旧", type: "string" },
              right: { ...empty, present: true, raw: "新", display: "新", type: "string" }
            }]
          })
        }
      }
    };
    const wrapper = mount(App);
    await flushPromises();
    expect(wrapper.text()).toContain("数据 表");
    expect(wrapper.text()).toContain("旧");
    expect(wrapper.text()).toContain("新");
    expect(wrapper.text()).toContain("0 / 1");
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
