import { enableAutoUnmount, flushPromises, mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import UGitRepositoryRepair from "./UGitRepositoryRepair.vue";
import { backend } from "../backend";
import { setLocalePreference } from "../i18n";
import type { UGitRepositoryScanResult } from "../types";

vi.mock("../backend", () => ({ backend: {
  scanUGitRepositories: vi.fn(), cancelUGitRepositoryScan: vi.fn(), repairUGitRepositories: vi.fn()
} }));
enableAutoUnmount(afterEach);
const fixture = (): UGitRepositoryScanResult => ({
  scanID: 7, cancelled: false,
  scan: {
    directory: "/projects", inspected: 4,
    repositories: [{ path: "/projects/old", tools: ["/tools/bcomp"] }, { path: "/projects/wildcard", tools: ["/tools/tool"] }],
    issues: [{ path: "/projects/unreadable", message: "Permission denied" }]
  }
});

describe("UGit repository scan and repair", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    setLocalePreference("zh-CN");
    vi.mocked(backend.scanUGitRepositories).mockResolvedValue(fixture());
    vi.mocked(backend.cancelUGitRepositoryScan).mockResolvedValue(undefined);
  });

  it("lists old tools without writing and repairs only selected paths after confirmation", async () => {
    const wrapper = mount(UGitRepositoryRepair, { props: { disabled: false } });
    await wrapper.get('button[aria-label="扫描仓库目录并修复"]').trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("已检查 4 个仓库，发现 2 个需要修复");
    expect(wrapper.text()).toContain("/tools/bcomp");
    expect(wrapper.text()).toContain("1 个位置未能检查");
    expect(backend.repairUGitRepositories).not.toHaveBeenCalled();
    await wrapper.get('input[aria-label="/projects/wildcard"]').setValue(false);
    vi.mocked(backend.repairUGitRepositories).mockResolvedValueOnce({ cancelled: true, results: [] });
    await wrapper.get("button.primary").trigger("click");
    await flushPromises();
    expect(backend.repairUGitRepositories).toHaveBeenCalledWith(7, ["/projects/old"]);
    expect(wrapper.text()).toContain("已取消修复");
    expect(wrapper.find("button.primary").exists()).toBe(true);
    vi.mocked(backend.repairUGitRepositories).mockResolvedValueOnce({ cancelled: false, results: [{ path: "/projects/old", status: "repaired", message: "" }] });
    await wrapper.get("button.primary").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("已修复 1 个");
    expect(wrapper.text()).toContain("未选择修复");
    expect(wrapper.find("button.primary").exists()).toBe(false);
    expect(wrapper.emitted("busy")).toEqual([[true], [false], [true], [false], [true], [false]]);
  });

  it("cancels an in-flight scan without allowing repair", async () => {
    let resolveScan!: (result: UGitRepositoryScanResult) => void;
    vi.mocked(backend.scanUGitRepositories).mockReturnValueOnce(new Promise(resolve => { resolveScan = resolve; }));
    const wrapper = mount(UGitRepositoryRepair, { props: { disabled: false } });
    await wrapper.get('button[aria-label="扫描仓库目录并修复"]').trigger("click");
    expect(wrapper.text()).toContain("正在扫描仓库配置");
    await wrapper.findAll("button").find(button => button.text() === "取消扫描")!.trigger("click");
    expect(backend.cancelUGitRepositoryScan).toHaveBeenCalledOnce();
    resolveScan({ ...fixture(), cancelled: true });
    await flushPromises();
    expect(wrapper.text()).toContain("已取消扫描");
    expect(wrapper.find(".ugit-scan-list").exists()).toBe(false);
    expect(backend.repairUGitRepositories).not.toHaveBeenCalled();
  });

  it("lists repositories even when the old command has no recognizable executable path", async () => {
    const scan = fixture();
    scan.scan.repositories[0].tools = [];
    vi.mocked(backend.scanUGitRepositories).mockResolvedValueOnce(scan);
    const wrapper = mount(UGitRepositoryRepair, { props: { disabled: false } });
    await wrapper.get('button[aria-label="扫描仓库目录并修复"]').trigger("click");
    await flushPromises();
    expect(wrapper.findAll(".ugit-scan-list li")).toHaveLength(2);
    expect(wrapper.get("button.primary").attributes("disabled")).toBeUndefined();
  });

  it("shows changed and failed repositories and disables writing with no selection", async () => {
    const wrapper = mount(UGitRepositoryRepair, { props: { disabled: false } });
    await wrapper.get('button[aria-label="扫描仓库目录并修复"]').trigger("click");
    await flushPromises();
    await wrapper.get(".ugit-scan-select-all input").setValue(false);
    expect(wrapper.get("button.primary").attributes("disabled")).toBeDefined();
    await wrapper.get(".ugit-scan-select-all input").setValue(true);
    vi.mocked(backend.repairUGitRepositories).mockResolvedValueOnce({ cancelled: false, results: [
      { path: "/projects/old", status: "changed", message: "" },
      { path: "/projects/wildcard", status: "failed", message: "Read-only config" }
    ] });
    await wrapper.get("button.primary").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("配置已变化，请重新扫描");
    expect(wrapper.text()).toContain("Read-only config");
    expect(wrapper.text()).toContain("失败或跳过 2 个");
  });

  it("discards a pending scan when the settings component is destroyed", async () => {
    let resolveScan!: (result: UGitRepositoryScanResult) => void;
    vi.mocked(backend.scanUGitRepositories).mockReturnValueOnce(new Promise(resolve => { resolveScan = resolve; }));
    const onBusy = vi.fn();
    const wrapper = mount(UGitRepositoryRepair, { props: { disabled: false, onBusy } });
    await wrapper.get('button[aria-label="扫描仓库目录并修复"]').trigger("click");
    wrapper.unmount();
    expect(backend.cancelUGitRepositoryScan).toHaveBeenCalledOnce();
    resolveScan(fixture());
    await flushPromises();
    expect(onBusy.mock.calls).toEqual([[true], [false]]);
    expect(backend.repairUGitRepositories).not.toHaveBeenCalled();
  });
});
