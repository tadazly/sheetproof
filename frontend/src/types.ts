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
  rowStatus: RowStatus;
  left: CellValue;
  right: CellValue;
}

export type RowStatus = "unchanged" | "added" | "deleted" | "modified" | "conflict";

export interface RowDiff {
  row: number;
  leftRow?: number;
  rightRow?: number;
  id?: string;
  status: RowStatus;
}

export type ResolutionKind =
  | "overwrite-cells"
  | "overwrite-row"
  | "append-row"
  | "append-auto"
  | "append-specified";

export interface RowResolution {
  sheet: string;
  sourceRow: number;
  targetRow?: number;
  targetSourceRow?: number;
  targetId?: string;
  kind: ResolutionKind;
  cellCount?: number;
}

export interface SheetDiff {
  name: string;
  status: string;
  orderDifferent: boolean;
  differenceCount: number;
  maxRow: number;
  maxCol: number;
  idColumn: number;
  nextId: number;
  addedRowCount: number;
  deletedRowCount: number;
  modifiedRowCount: number;
  conflictRowCount: number;
  rows?: RowDiff[];
}

export interface Summary {
  options: {
    locale?: string;
    title: string;
    leftLabel: string;
    rightLabel: string;
    readonlyLeft: boolean;
    gitDiff: boolean;
    ugitWorktree: boolean;
    gitMerge: boolean;
    output: string;
    repositoryPath?: string;
    repositoryFile?: string;
    repositoryRef?: string;
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
  resolutions: RowResolution[];
  dirty: boolean;
  undoCount: number;
  warnings: string[];
  rowAlignment: {
    mode: "auto" | "position";
    available: boolean;
    applied: boolean;
    moved: number;
    sheets: Record<string, {
      available: boolean;
      applied: boolean;
      moved: number;
      keyColumn: number;
    }>;
  };
  mergeNotice: string;
  selectedSheet: string;
}

export interface RegionCell {
  row: number;
  sourceRow?: number;
  leftRow?: number;
  rightRow?: number;
  col: number;
  axis: string;
  left: CellValue;
  originalLeft?: CellValue;
  right: CellValue;
  status: string;
  rowStatus: RowStatus;
  leftMatch?: boolean;
  rightMatch?: boolean;
}

export interface Region {
  sheet: string;
  fromRow: number;
  toRow: number;
  fromCol: number;
  toCol: number;
  filtered?: boolean;
  totalRows?: number;
  cells: RegionCell[];
}

export interface SearchRef {
  row: number;
  sourceRow: number;
  col: number;
  axis: string;
}

export interface SearchSummary {
  count: number;
  currentIndex: number;
  currentRef?: SearchRef;
  error?: string;
}

export interface RepositoryBranch {
  name: string;
  fullName: string;
  kind: "local" | "remote";
}

export interface RepositoryView {
  name: string;
  path: string;
  currentBranch: string;
  detached: boolean;
  workspaceDirty: boolean;
  operation: string;
  files: string[];
  differenceFiles: string[];
  differenceIndexing: boolean;
  branches: RepositoryBranch[];
  defaultRef: string;
  selectedFile: string;
  selectedRef: string;
  leftState: string;
  rightState: string;
  leftMessage: string;
  rightMessage: string;
  fileModified: boolean;
  sidebarWidth: number;
  notice: string;
  loading: boolean;
  loadGeneration: number;
  comparisonActive: boolean;
}

export interface RepositoryResult {
  repository: RepositoryView;
  summary: Summary | null;
}

export interface RecentRepository {
  name: string;
  path: string;
  available: boolean;
}

export interface BootstrapState {
  loading: boolean;
  hasSession: boolean;
  error: string;
  mode?: "" | "files" | "repository";
  repository?: RepositoryView;
}

export interface UGitConfigurationResult {
  configured: boolean;
  changed: boolean;
  cancelled: boolean;
  executablePath: string;
  message: string;
}

export interface ExternalFileChange {
  changed: boolean;
  path: string;
  signature: string;
  writable: boolean;
}

export interface ExternalChanges {
  left: ExternalFileChange;
  right: ExternalFileChange;
}

export interface ExternalReloadResult {
  summary: Summary;
  repository?: RepositoryView;
  notice: string;
}
