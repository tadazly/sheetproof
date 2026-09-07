import { describe, expect, it } from "vitest";
import { bodyIndex, bodyWindow, logicalIndex } from "./frozenAxis";

describe("pinned axis layout", () => {
  it("packs non-contiguous pinned items ahead of the body without holes or duplicates", () => {
    const frozen = [1, 3, 7];
    const body = bodyWindow(1, 6, 9, frozen);
    expect(body).toEqual([2, 4, 5, 6, 8, 9]);
    expect(body.map((item) => bodyIndex(item, frozen))).toEqual([1, 2, 3, 4, 5, 6]);
    expect(body.map((_, index) => logicalIndex(index + 1, frozen))).toEqual(body);
  });

  it("fills a bounded viewport across a large pinned range and preserves the tail", () => {
    const frozen = Array.from({ length: 300 }, (_, index) => index + 10);
    expect(bodyWindow(8, 5, 1_000_000, frozen)).toEqual([8, 9, 310, 311, 312]);
    expect(logicalIndex(10, frozen)).toBe(310);
    expect(bodyWindow(999_999, 48, 1_000_000, frozen)).toEqual([999_999, 1_000_000]);
    expect(bodyIndex(1_000_000, frozen)).toBe(999_700);
  });
});
