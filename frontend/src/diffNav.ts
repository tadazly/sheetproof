import type { CellDiff } from "./types";

export function nextDiffIndex(current: number, length: number, direction: 1 | -1): number {
  if (length <= 0) return -1;
  if (current < 0) return direction === 1 ? 0 : length - 1;
  return (current + direction + length) % length;
}

export function diffKey(item: CellDiff): string {
  return `${item.ref.sheet}!${item.ref.row}:${item.ref.col}`;
}
