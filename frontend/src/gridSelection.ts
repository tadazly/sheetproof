export interface CellPoint {
  row: number;
  col: number;
}

export interface SelectionRange {
  startRow: number;
  endRow: number;
  startCol: number;
  endCol: number;
}

export function makeRange(anchor: CellPoint, focus: CellPoint): SelectionRange {
  return {
    startRow: Math.min(anchor.row, focus.row),
    endRow: Math.max(anchor.row, focus.row),
    startCol: Math.min(anchor.col, focus.col),
    endCol: Math.max(anchor.col, focus.col)
  };
}

export function containsCell(range: SelectionRange | null, row: number, col: number): boolean {
  return Boolean(
    range
    && row >= range.startRow
    && row <= range.endRow
    && col >= range.startCol
    && col <= range.endCol
  );
}

export function rangeSize(range: SelectionRange | null): number {
  if (!range) return 0;
  return (range.endRow - range.startRow + 1) * (range.endCol - range.startCol + 1);
}

