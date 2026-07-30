import { describe, expect, it } from "vitest";
import { containsCell, makeRange, rangeSize } from "./gridSelection";

describe("grid selection", () => {
  it("normalizes reverse drag ranges", () => {
    const range = makeRange({ row: 8, col: 5 }, { row: 3, col: 2 });
    expect(range).toEqual({ startRow: 3, endRow: 8, startCol: 2, endCol: 5 });
    expect(containsCell(range, 5, 4)).toBe(true);
    expect(containsCell(range, 2, 4)).toBe(false);
    expect(rangeSize(range)).toBe(24);
  });

  it("supports single cells and empty selection", () => {
    const range = makeRange({ row: 2, col: 3 }, { row: 2, col: 3 });
    expect(rangeSize(range)).toBe(1);
    expect(containsCell(range, 2, 3)).toBe(true);
    expect(rangeSize(null)).toBe(0);
  });
});
