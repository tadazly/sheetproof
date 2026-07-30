import { describe, expect, it } from "vitest";
import { nextDiffIndex } from "./diffNav";

describe("difference navigation", () => {
  it("wraps in both directions", () => {
    expect(nextDiffIndex(2, 3, 1)).toBe(0);
    expect(nextDiffIndex(0, 3, -1)).toBe(2);
  });
  it("handles empty and initial selection", () => {
    expect(nextDiffIndex(-1, 0, 1)).toBe(-1);
    expect(nextDiffIndex(-1, 3, 1)).toBe(0);
  });
});
