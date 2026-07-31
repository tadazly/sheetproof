import type { CellDiff, RowStatus } from "./types";

export type DiffFilter = Exclude<RowStatus, "unchanged">;

export const DIFF_FILTER_ORDER: DiffFilter[] = [
  "conflict",
  "modified",
  "deleted",
  "added"
];

export function preferredDiffFilter(items: CellDiff[]): DiffFilter {
  return DIFF_FILTER_ORDER.find((status) =>
    items.some((item) => item.rowStatus === status)
  ) ?? "modified";
}

export function nextDiffIndex(current: number, length: number, direction: 1 | -1): number {
  if (length <= 0) return -1;
  if (current < 0) return direction === 1 ? 0 : length - 1;
  return (current + direction + length) % length;
}

export function diffKey(item: CellDiff): string {
  return `${item.ref.sheet}!${item.ref.row}:${item.ref.col}`;
}
