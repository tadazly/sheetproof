import type {
  BootstrapState,
  CellDiff,
  Region,
  RepositoryResult,
  Summary,
  UGitConfigurationResult
} from "./types";

function controller(): Record<string, (...args: any[]) => Promise<any>> {
  const value = window.go?.main?.Controller;
  if (!value) throw new Error("Wails 后端尚未就绪");
  return value;
}

export const backend = {
  bootstrap: (): Promise<BootstrapState> =>
    controller().Bootstrap(),
  configureUGit: (): Promise<UGitConfigurationResult> =>
    controller().ConfigureUGit(),
  selectAndOpen: (): Promise<Summary> => controller().SelectAndOpen(),
  openFiles: (left: string, right: string): Promise<Summary> =>
    controller().OpenFiles(left, right),
  selectRepository: (): Promise<RepositoryResult> => controller().SelectRepository(),
  openRepository: (path: string): Promise<RepositoryResult> => controller().OpenRepository(path),
  repository: (): Promise<RepositoryResult> => controller().Repository(),
  selectRepositoryFile: (path: string): Promise<RepositoryResult> =>
    controller().SelectRepositoryFile(path),
  selectRepositoryRef: (ref: string): Promise<RepositoryResult> =>
    controller().SelectRepositoryRef(ref),
  refreshRepository: (): Promise<RepositoryResult> => controller().RefreshRepository(),
  setRepositorySidebarWidth: (width: number): Promise<void> =>
    controller().SetRepositorySidebarWidth(width),
  summary: (): Promise<Summary> => controller().Summary(),
  region: (sheet: string, row: number, rows: number, col: number, cols: number): Promise<Region> =>
    controller().Region(sheet, row, rows, col, cols),
  differences: (sheet: string, offset: number, limit: number): Promise<CellDiff[]> =>
    controller().Differences(sheet, offset, limit),
  copy: (sheet: string, row: number, col: number): Promise<Summary> =>
    controller().CopyRightToLeft(sheet, row, col),
  copyMany: (sheet: string, cells: Array<{ row: number; col: number }>): Promise<Summary> =>
    controller().CopyRightToLeftMany(sheet, cells),
  copyRows: (sheet: string, rows: number[]): Promise<Summary> =>
    controller().CopyRowsRightToLeft(sheet, rows),
  appendRows: (sheet: string, rows: number[], ids: string[]): Promise<Summary> =>
    controller().AppendRowsRightToLeft(sheet, rows, ids),
  edit: (sheet: string, row: number, col: number, value: string, type: string): Promise<Summary> =>
    controller().EditLeft(sheet, row, col, value, type),
  undo: (): Promise<Summary> => controller().Undo(),
  save: (): Promise<Summary> => controller().Save(),
  saveAs: (): Promise<Summary> => controller().SaveAs()
};
