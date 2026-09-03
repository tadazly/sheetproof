import type {
  BootstrapState,
  CellDiff,
  ExternalChanges,
  ExternalReloadResult,
  Region,
  RecentRepository,
  RepositoryResult,
  SearchSummary,
  Summary,
  UGitConfigurationResult
} from "./types";
import { t } from "./i18n";

export interface FrontendDiagnosticEvent {
  id: string;
  timestamp: string;
  name: string;
  fields: Record<string, string | number | boolean | null>;
}

function controller(): Record<string, (...args: any[]) => Promise<any>> {
  const value = window.go?.main?.Controller;
  if (!value) throw new Error(t("errors.backendUnavailable"));
  return value;
}

export const backend = {
  recordDiagnostics: (events: FrontendDiagnosticEvent[]): Promise<void> => {
    const api = controller();
    if (typeof api.RecordDiagnostics !== "function") return Promise.resolve();
    return api.RecordDiagnostics(events);
  },
  languagePreference: (): Promise<string> => controller().LanguagePreference(),
  setLanguagePreference: (preference: string): Promise<void> => controller().SetLanguagePreference(preference),
  setRuntimeLocale: (locale: string): Promise<void> => controller().SetRuntimeLocale(locale),
  clearDifferenceIndexCache: (): Promise<void> => controller().ClearDifferenceIndexCache(),
  clearAllData: (): Promise<void> => controller().ClearAllData(),
  bootstrap: (): Promise<BootstrapState> =>
    controller().Bootstrap(),
  configureUGit: (): Promise<UGitConfigurationResult> =>
    controller().ConfigureUGit(),
  selectAndOpen: (): Promise<Summary> => controller().SelectAndOpen(),
  openFiles: (left: string, right: string): Promise<Summary> =>
    controller().OpenFiles(left, right),
  selectRepository: (): Promise<RepositoryResult> => controller().SelectRepository(),
  recentRepositories: (): Promise<RecentRepository[]> => controller().RecentRepositories(),
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
  setRowAlignment: (mode: "auto" | "position"): Promise<Summary> =>
    controller().SetRowAlignment(mode),
  setKeyColumn: (sheet: string, column: number): Promise<Summary> =>
    controller().SetKeyColumn(sheet, column),
  checkExternalChanges: (): Promise<ExternalChanges> => controller().CheckExternalChanges(),
  reloadExternal: (side: "left" | "right"): Promise<ExternalReloadResult> =>
    controller().ReloadExternal(side),
  region: (sheet: string, row: number, rows: number, col: number, cols: number): Promise<Region> =>
    controller().Region(sheet, row, rows, col, cols),
  pinnedRegion: async (sheet: string, rows: number[], columns: number[]): Promise<Region> => {
    const api = controller();
    if (typeof api.PinnedRegion === "function") {
      return api.PinnedRegion(sheet, rows, columns);
    }
    // Keep the frontend compatible with an older desktop bridge during hot reload.
    // One-row Region calls preserve arbitrary, non-contiguous pinned selections.
    const normalizedRows = [...new Set(rows)].filter((row) => row > 0).sort((a, b) => a - b);
    const normalizedColumns = [...new Set(columns)].filter((col) => col > 0).sort((a, b) => a - b);
    if (!normalizedRows.length || !normalizedColumns.length) {
      return { sheet, fromRow: 0, toRow: 0, fromCol: 0, toCol: 0, cells: [] };
    }
    const fromCol = normalizedColumns[0];
    const toCol = normalizedColumns.at(-1)!;
    const selectedColumns = new Set(normalizedColumns);
    const regions = await Promise.all(normalizedRows.map((row) =>
      api.Region(sheet, row, 1, fromCol, toCol - fromCol + 1) as Promise<Region>
    ));
    return {
      sheet,
      fromRow: normalizedRows[0],
      toRow: normalizedRows.at(-1)!,
      fromCol,
      toCol,
      columns: regions[0]?.columns?.filter((column) => selectedColumns.has(column.col)),
      cells: regions.flatMap((item) => item.cells.filter((cell) => selectedColumns.has(cell.col)))
    };
  },
  filteredRegion: (
    sheet: string,
    statuses: string[],
    row: number,
    rows: number,
    col: number,
    cols: number
  ): Promise<Region> => controller().FilteredRegion(sheet, statuses, row, rows, col, cols),
  find: (
    sheet: string,
    side: "left" | "right",
    query: string,
    caseSensitive: boolean,
    wholeWord: boolean,
    regex: boolean,
    statuses: string[],
    anchorRow: number,
    anchorCol: number
  ): Promise<SearchSummary> => controller().Find(
    sheet, side, query, caseSensitive, wholeWord, regex, statuses, anchorRow, anchorCol
  ),
  navigateFind: (side: "left" | "right", direction: 1 | -1): Promise<SearchSummary> =>
    controller().NavigateFind(side, direction),
  clearFind: (side: "left" | "right"): Promise<void> => controller().ClearFind(side),
  differences: (sheet: string, offset: number, limit: number): Promise<CellDiff[]> =>
    controller().Differences(sheet, offset, limit),
  copySelectionToClipboard: (
    sheet: string,
    side: "left" | "right",
    startRow: number,
    endRow: number,
    startCol: number,
    endCol: number,
    rows: number[]
  ): Promise<void> => controller().CopySelectionToClipboard(
    sheet, side, startRow, endRow, startCol, endCol, rows
  ),
  pasteClipboard: (sheet: string, row: number, col: number): Promise<Summary> =>
    controller().PasteClipboard(sheet, row, col),
  cutSelectionToClipboard: (
    sheet: string,
    startRow: number,
    endRow: number,
    startCol: number,
    endCol: number,
    rows: number[],
    clearRows: number[]
  ): Promise<Summary> => controller().CutSelectionToClipboard(
    sheet, startRow, endRow, startCol, endCol, rows, clearRows
  ),
  openWorkbook: (relative = ""): Promise<void> => controller().OpenWorkbook(relative),
  revealWorkbook: (relative = ""): Promise<void> => controller().RevealWorkbook(relative),
  copy: (sheet: string, row: number, col: number): Promise<Summary> =>
    controller().CopyRightToLeft(sheet, row, col),
  copyMany: (sheet: string, cells: Array<{ row: number; col: number }>): Promise<Summary> =>
    controller().CopyRightToLeftMany(sheet, cells),
  copyRows: (sheet: string, rows: number[]): Promise<Summary> =>
    controller().CopyRowsRightToLeft(sheet, rows),
  copyColumns: (sheet: string, columns: number[]): Promise<Summary> =>
    controller().CopyColumnsRightToLeft(sheet, columns),
  deleteColumns: (sheet: string, columns: number[]): Promise<Summary> =>
    controller().DeleteLeftColumns(sheet, columns),
  appendRows: (sheet: string, rows: number[], ids: string[]): Promise<Summary> =>
    controller().AppendRowsRightToLeft(sheet, rows, ids),
  edit: (sheet: string, row: number, col: number, value: string, type: string): Promise<Summary> =>
    controller().EditLeft(sheet, row, col, value, type),
  clearSelection: (
    sheet: string,
    startRow: number,
    endRow: number,
    startCol: number,
    endCol: number,
    rows: number[]
  ): Promise<Summary> =>
    controller().ClearLeftSelection(sheet, startRow, endRow, startCol, endCol, rows),
  undo: (): Promise<Summary> => controller().Undo(),
  save: (): Promise<Summary> => controller().Save(),
  saveAs: (): Promise<Summary> => controller().SaveAs()
};
