import { describe, expect, it } from "vitest";
import { en } from "./messages/en";
import { zhCN } from "./messages/zh-CN";
import { ja } from "./messages/ja";

const catalogs = { en, "zh-CN": zhCN, ja } as const;

describe("desktop message catalogs", () => {
  it("keeps exactly the same semantic keys in every locale", () => {
    const expected = Object.keys(en).sort();
    for (const messages of Object.values(catalogs)) expect(Object.keys(messages).sort()).toEqual(expected);
  });

  it("contains localized welcome, toolbar, repository, context, external-change, and UGit text", () => {
    for (const messages of Object.values(catalogs)) {
      for (const key of ["welcome.title", "toolbar.nextDifference", "repository.currentBranch", "context.copyCells", "externalChange.reload", "toolbar.configureUGit"] as const) {
        expect(messages[key]).toBeTypeOf("string");
        expect(messages[key]).not.toBe(key);
      }
    }
    expect(zhCN["welcome.title"]).toContain("仓库");
    expect(ja["welcome.title"]).toContain("リポジトリ");
  });

  it("uses locale-aware complete templates for dynamic quantities", () => {
    expect(en["toolbar.copyCellsToLeft"]({ count: 1 })).toBe("Copy left (1)");
    expect(en["toolbar.copyCellsToLeft"]({ count: 3 })).toBe("Copy left (3)");
    expect(zhCN["toolbar.copyCellsToLeft"]({ count: 3 })).toBe("复制到左侧（3）");
    expect(ja["toolbar.copyCellsToLeft"]({ count: 3 })).toBe("左側へ反映（3）");
  });
});
