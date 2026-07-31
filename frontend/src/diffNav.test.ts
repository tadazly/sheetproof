import { describe, expect, it } from "vitest";
import { nextDiffIndex, preferredDiffFilter } from "./diffNav";
import type { CellDiff, RowStatus } from "./types";

describe("difference navigation", () => {
  it("wraps in both directions", () => {
    expect(nextDiffIndex(2, 3, 1)).toBe(0);
    expect(nextDiffIndex(0, 3, -1)).toBe(2);
  });
  it("handles empty and initial selection", () => {
    expect(nextDiffIndex(-1, 0, 1)).toBe(-1);
    expect(nextDiffIndex(-1, 3, 1)).toBe(0);
  });
  it("prefers conflict, then modified, deleted, and added filters", () => {
    const item = (rowStatus: RowStatus): CellDiff => ({
      ref: { sheet: "S", row: 1, col: 1 },
      status: "modified",
      rowStatus,
      left: { present: true, raw: "L", display: "L", type: "string" },
      right: { present: true, raw: "R", display: "R", type: "string" }
    });
    expect(preferredDiffFilter([item("added"), item("deleted"), item("modified"), item("conflict")])).toBe("conflict");
    expect(preferredDiffFilter([item("added"), item("deleted"), item("modified")])).toBe("modified");
    expect(preferredDiffFilter([item("added"), item("deleted")])).toBe("deleted");
    expect(preferredDiffFilter([item("added")])).toBe("added");
  });
});
