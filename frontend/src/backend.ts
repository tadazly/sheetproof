import type { CellDiff, Region, Summary } from "./types";

function controller(): Record<string, (...args: any[]) => Promise<any>> {
  const value = window.go?.main?.Controller;
  if (!value) throw new Error("Wails 后端尚未就绪");
  return value;
}

export const backend = {
  bootstrap: (): Promise<{ loading: boolean; hasSession: boolean; error: string }> =>
    controller().Bootstrap(),
  selectAndOpen: (): Promise<Summary> => controller().SelectAndOpen(),
  openFiles: (left: string, right: string): Promise<Summary> =>
    controller().OpenFiles(left, right),
  summary: (): Promise<Summary> => controller().Summary(),
  region: (sheet: string, row: number, rows: number, col: number, cols: number): Promise<Region> =>
    controller().Region(sheet, row, rows, col, cols),
  differences: (sheet: string, offset: number, limit: number): Promise<CellDiff[]> =>
    controller().Differences(sheet, offset, limit),
  copy: (sheet: string, row: number, col: number): Promise<Summary> =>
    controller().CopyRightToLeft(sheet, row, col),
  copyMany: (sheet: string, cells: Array<{ row: number; col: number }>): Promise<Summary> =>
    controller().CopyRightToLeftMany(sheet, cells),
  edit: (sheet: string, row: number, col: number, value: string, type: string): Promise<Summary> =>
    controller().EditLeft(sheet, row, col, value, type),
  undo: (): Promise<Summary> => controller().Undo(),
  save: (): Promise<Summary> => controller().Save(),
  saveAs: (): Promise<Summary> => controller().SaveAs()
};
