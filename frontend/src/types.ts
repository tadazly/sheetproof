export interface CellValue {
  present: boolean;
  raw: string;
  display: string;
  formula?: string;
  type: string;
}

export interface CellDiff {
  ref: { sheet: string; row: number; col: number };
  status: string;
  left: CellValue;
  right: CellValue;
}

export interface SheetDiff {
  name: string;
  status: string;
  orderDifferent: boolean;
  differenceCount: number;
  maxRow: number;
  maxCol: number;
}

export interface Summary {
  options: {
    title: string;
    leftLabel: string;
    rightLabel: string;
    readonlyLeft: boolean;
    output: string;
  };
  diff: {
    equal: boolean;
    leftFile: string;
    rightFile: string;
    sheetCount: number;
    differentSheetCount: number;
    differenceCount: number;
    sheets: SheetDiff[];
  };
  dirty: boolean;
  undoCount: number;
  warnings: string[];
  selectedSheet: string;
}

export interface RegionCell {
  row: number;
  col: number;
  axis: string;
  left: CellValue;
  right: CellValue;
  status: string;
}

export interface Region {
  sheet: string;
  fromRow: number;
  toRow: number;
  fromCol: number;
  toCol: number;
  cells: RegionCell[];
}
