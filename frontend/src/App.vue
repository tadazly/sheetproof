<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { EventsOn } from "../wailsjs/runtime/runtime";
import AppIcon from "./components/AppIcon.vue";
import EmptyState from "./components/EmptyState.vue";
import { backend } from "./backend";
import { nextDiffIndex, preferredDiffFilter, type DiffFilter } from "./diffNav";
import { containsCell, makeRange, rangeSize, type CellPoint, type SelectionRange } from "./gridSelection";
import { scrollMarkerGradient, type ScrollMarker } from "./scrollMarkers";
import { initializeLocale, useI18n, type LocalePreference } from "./i18n";
import { localizeBackendError } from "./i18n/errors";
import type {
  CellDiff,
  ExternalFileChange,
  RecentRepository,
  Region,
  RegionCell,
  RepositoryResult,
  RepositoryView,
  RowResolution,
  RowStatus,
  Summary
} from "./types";

const BASE_ROW_HEIGHT = 23;
const BASE_COL_WIDTH = 96;
const BASE_ROW_HEADER_WIDTH = 42;
const MIN_COL_WIDTH = 48;
const MAX_COL_WIDTH = 420;
const MIN_ZOOM = 0.7;
const MAX_ZOOM = 1.8;
const VIEW_ROWS = 48;
const VIEW_COLS = 20;
const REGION_ROW_STEP = 12;
const REGION_COL_STEP = 4;
const REGION_SCHEDULE_MS = 16;
const REPOSITORY_SEARCH_HISTORY_LIMIT = 5;
const REPOSITORY_SEARCH_HISTORY_PREFIX = "sheetproof:repository-search-history:";
const LEGACY_REPOSITORY_SEARCH_HISTORY_PREFIX = "ugxlsx:repository-search-history:";
const SHEET_LAYOUT_PREFIX = "sheetproof:sheet-layout:v1:";
const LEGACY_SHEET_LAYOUT_PREFIX = "ugxlsx.sheet-layout.v1:";
const DIFF_FILTER_TABS: DiffFilter[] = ["added", "deleted", "modified", "conflict"];
const { locale, preference, t, setLocalePreference } = useI18n();

const summary = ref<Summary | null>(null);
const repository = ref<RepositoryView | null>(null);
const region = ref<Region | null>(null);
const sheet = ref("");
const diffs = ref<CellDiff[]>([]);
const diffIndex = ref(-1);
const activePoint = ref<CellPoint | null>(null);
const selectionAnchor = ref<CellPoint | null>(null);
const selection = ref<SelectionRange | null>(null);
const busy = ref(false);
const error = ref("");
const externalNotice = ref("");
const diffFilter = ref<DiffFilter>("modified");
const selectedRowFilters = ref<DiffFilter[]>([]);
const leftScroll = ref<HTMLElement | null>(null);
const rightScroll = ref<HTMLElement | null>(null);
const viewportTop = ref(0);
const viewportLeft = ref(0);
const zoom = ref(1);
const columnWidths = ref<Record<number, number>>({});
const markerColumnWidths = ref<Record<number, number>>({});
const contextMenu = ref<{
  visible: boolean;
  x: number;
  y: number;
  row: number;
  col: number;
  side: "left" | "right";
  kind: "cell" | "row" | "column";
}>({ visible: false, x: 0, y: 0, row: 0, col: 0, side: "right", kind: "cell" });
const idDialog = ref<{
  visible: boolean;
  rows: number[];
  values: string[];
}>({ visible: false, rows: [], values: [] });
const repositorySwitchDialog = ref<{
  visible: boolean;
  repositories: RecentRepository[];
}>({ visible: false, repositories: [] });
const repositoryOpenDialog = ref(false);
const repositoryOpening = ref(false);
const repositoryOpenError = ref("");
const settingsDialog = ref(false);
const settingsConfirmation = ref<"cache" | "all" | null>(null);
const settingsNotice = ref<{ kind: "success" | "error" | "info"; message: string } | null>(null);
const externalChangeDialog = ref<{
  visible: boolean;
  side: "left" | "right";
  change: ExternalFileChange | null;
}>({ visible: false, side: "left", change: null });
const deferredExternalChange = ref<{ side: "left" | "right"; change: ExternalFileChange } | null>(null);
const startupLoading = ref(true);
const expandedDirectories = ref(new Set<string>());
const repositorySearch = ref("");
const repositorySearchHistory = ref<string[]>([]);
const repositorySearchFocused = ref(false);
const repositorySearchInput = ref<HTMLInputElement | null>(null);
const repositorySidebarTab = ref<"files" | "differences" | "sheets">("files");
const inlineEdit = ref<{ row: number; col: number; value: string; original: RegionCell } | null>(null);
const previewOriginal = ref(false);
const repoSidebar = ref<HTMLElement | null>(null);
let loadingTimer = 0;
let differenceIndexTimer = 0;
let externalCheckTimer = 0;
let externalFocusTimer = 0;
let wheelFrame = 0;
let wheelSource: HTMLElement | null = null;
let wheelDeltaTop = 0;
let wheelDeltaLeft = 0;
let regionRequest = 0;
let sheetRequest = 0;
let repositoryRequest = 0;
let pendingActions = 0;
let syncing = false;
let draggingSelection = false;
let draggingRows = false;
let dragAnchor: CellPoint | null = null;
let resizeState: { col: number; startX: number; startWidth: number } | null = null;
let repositoryResize: { startX: number; startWidth: number } | null = null;
let stopDropListener: (() => void) | undefined;
let previewFromKeyboard = false;
let previewFromPointer = false;
let checkingExternalChanges = false;
let externalCheckFailures = 0;

interface RepositoryTreeRow {
  kind: "directory" | "file";
  path: string;
  name: string;
  depth: number;
}

interface FilteredRowMapping {
  display: number;
  source: number;
  left: number;
  right: number;
}

interface RowViewportAnchor {
  sourceRow: number;
  viewportOffset: number;
}

const activeSheet = computed(() => summary.value?.diff.sheets.find((item) => item.name === sheet.value));
const activeRowAlignment = computed(() => summary.value?.rowAlignment.sheets?.[sheet.value]);
const leftReadonly = computed(() => Boolean(summary.value?.options.readonlyLeft));
const rowFilterActive = computed(() => selectedRowFilters.value.length > 0);
const selectedRowFilterSet = computed(() => new Set(selectedRowFilters.value));
const filteredRowMappings = computed<FilteredRowMapping[]>(() => {
  if (!rowFilterActive.value) return [];
  const mappings: FilteredRowMapping[] = [];
  const resolutions = summary.value?.resolutions ?? [];
  const appendTargets = new Set(
    resolutions
      .filter((item) => item.sheet === sheet.value && item.targetRow && isAppendResolution(item))
      .map((item) => item.targetSourceRow ?? item.targetRow)
  );
  const appendSources = new Map<number, number>();
  for (const resolution of resolutions) {
    if (
      resolution.sheet === sheet.value &&
      resolution.targetRow &&
      isAppendResolution(resolution)
    ) {
      appendSources.set(resolution.sourceRow, resolution.targetRow);
    }
  }
  for (const item of activeSheet.value?.rows ?? []) {
    if (appendTargets.has(item.row) || !selectedRowFilterSet.value.has(item.status as DiffFilter)) continue;
    const left = appendSources.get(item.row) ?? item.leftRow ?? 0;
    mappings.push({ display: mappings.length + 1, source: item.row, left, right: item.rightRow ?? 0 });
  }
  return mappings;
});
const totalRows = computed(() => rowFilterActive.value
  ? filteredRowMappings.value.length
  : Math.max(activeSheet.value?.maxRow ?? 0, 50) + 10
);
const totalCols = computed(() => Math.max(activeSheet.value?.maxCol ?? 0, 14) + 4);
const visibleColumns = computed(() => {
  if (!region.value) return [];
  const end = Math.min(totalCols.value, region.value.fromCol + VIEW_COLS - 1);
  return Array.from({ length: Math.max(0, end - region.value.fromCol + 1) }, (_, index) => region.value!.fromCol + index);
});
const visibleRows = computed(() => {
  if (!region.value) return [];
  const end = Math.min(totalRows.value, region.value.fromRow + VIEW_ROWS - 1);
  return Array.from({ length: Math.max(0, end - region.value.fromRow + 1) }, (_, index) => region.value!.fromRow + index);
});
const visibleCells = computed(() => region.value?.cells.filter((cell) => cell.row <= totalRows.value && cell.col <= totalCols.value) ?? []);

function sourcePathLabel(path: string, snapshot: boolean): string {
  if (!snapshot) return path;
  const filename = path.split(/[\\/]/).filter(Boolean).at(-1);
  if (filename === "missing-left.xlsx" || filename === "missing-right.xlsx") {
    return t("source.gitVersionMissing");
  }
  return filename ? t("source.gitSnapshotFile", { filename }) : t("source.gitSnapshot");
}
const rowHeight = computed(() => Math.round(BASE_ROW_HEIGHT * zoom.value));
const rowHeaderWidth = computed(() => Math.round(BASE_ROW_HEADER_WIDTH * zoom.value));
const scaledFontSize = computed(() => Math.max(9, Math.round(11 * zoom.value)));
const columnOffsets = computed(() => {
  const offsets = new Array<number>(totalCols.value + 2).fill(0);
  let left = 0;
  for (let col = 1; col <= totalCols.value; col++) {
    offsets[col] = left;
    left += columnWidth(col);
  }
  offsets[totalCols.value + 1] = left;
  return offsets;
});
const canvasWidth = computed(() => rowHeaderWidth.value + columnOffsets.value[totalCols.value + 1]);
const markerColumnOffsets = computed(() => {
  const offsets = new Array<number>(totalCols.value + 2).fill(0);
  let left = 0;
  for (let col = 1; col <= totalCols.value; col++) {
    offsets[col] = left;
    left += markerColumnWidth(col);
  }
  offsets[totalCols.value + 1] = left;
  return offsets;
});
const markerCanvasWidth = computed(() => rowHeaderWidth.value + markerColumnOffsets.value[totalCols.value + 1]);
const scrollbarMarkerStyle = computed<Record<string, string>>(() => {
  if (!diffs.value.length || totalRows.value <= 0 || markerCanvasWidth.value <= 0) {
    return {
      "--scrollbar-diff-vertical": "none",
      "--scrollbar-diff-horizontal": "none"
    };
  }
  const displayRows = rowFilterActive.value
    ? new Map(filteredRowMappings.value.map((item) => [item.source, item.display]))
    : null;
  const vertical: ScrollMarker[] = [];
  const horizontal: ScrollMarker[] = [];
  for (const item of diffs.value) {
    const status = (item.rowStatus || "modified") as RowStatus;
    const displayRow = displayRows ? displayRows.get(item.ref.row) : item.ref.row;
    if (!displayRow) continue;
    vertical.push({
      position: (displayRow + 0.5) / (totalRows.value + 1),
      status
    });
    const col = Math.min(Math.max(item.ref.col, 1), totalCols.value);
    horizontal.push({
      position: (rowHeaderWidth.value + markerColumnOffsets.value[col] + markerColumnWidth(col) / 2) / markerCanvasWidth.value,
      status
    });
  }
  return {
    "--scrollbar-diff-vertical": scrollMarkerGradient("bottom", vertical),
    "--scrollbar-diff-horizontal": scrollMarkerGradient("right", horizontal)
  };
});
const selectionSize = computed(() => rangeSize(selection.value));
const filteredDiffEntries = computed(() =>
  diffs.value
    .map((item, index) => ({ item, index }))
    .filter(({ item }) => (item.rowStatus || "modified") === diffFilter.value)
);
const diffFilterCounts = computed(() => {
  const counts: Record<DiffFilter, number> = {
    added: 0, deleted: 0, modified: 0, conflict: 0
  };
  for (const item of diffs.value) {
    const status = item.rowStatus || "modified";
    if (status !== "unchanged") counts[status]++;
  }
  return counts;
});
const resultMetrics = computed(() => {
  const current = activeSheet.value;
  return {
    added: current?.addedRowCount ?? 0,
    deleted: current?.deletedRowCount ?? 0,
    modified: current?.modifiedRowCount ?? 0,
    conflict: current?.conflictRowCount ?? 0
  };
});
const currentSheetDifferenceCount = computed(() => activeSheet.value?.differenceCount ?? 0);
const rowFilterSummary = computed(() => {
  if (!rowFilterActive.value) return t("diff.allData");
  if (selectedRowFilters.value.length === DIFF_FILTER_TABS.length) return t("diff.allDifferenceRows");
  return t("diff.filteredRows", { categories: selectedRowFilters.value.map(rowStatusLabel).join(locale.value === "en" ? ", " : "、") });
});
const currentTaskName = computed(() => {
  const selectedFile = repository.value?.selectedFile;
  if (selectedFile) return selectedFile.split("/").at(-1) ?? selectedFile;
  if (summary.value?.options.title) return summary.value.options.title;
  return t("app.title");
});
const filteredDiffPosition = computed(() =>
  filteredDiffEntries.value.findIndex(({ index }) => index === diffIndex.value)
);
const activeCell = computed(() => {
  const point = activePoint.value;
  return point ? region.value?.cells.find((cell) => cell.row === point.row && cell.col === point.col) ?? null : null;
});
const canPreviewOriginal = computed(() => Boolean(
  summary.value?.undoCount && region.value && !inlineEdit.value
));
const copyTargets = computed(() => {
  if (!selection.value) return [];
  if (rowFilterActive.value) {
    const sourceRows = new Set(
      filteredRowMappings.value
        .filter((item) => item.display >= selection.value!.startRow && item.display <= selection.value!.endRow)
        .map((item) => item.source)
    );
    return diffs.value
      .filter((item) => sourceRows.has(item.ref.row) && item.ref.col >= selection.value!.startCol && item.ref.col <= selection.value!.endCol)
      .map((item) => ({ row: item.ref.row, col: item.ref.col }));
  }
  return diffs.value
    .filter((item) => containsCell(selection.value, item.ref.row, item.ref.col))
    .map((item) => ({ row: item.ref.row, col: item.ref.col }));
});
function rowsForContext(row: number) {
  if (!selection.value || row < selection.value.startRow || row > selection.value.endRow) {
    return [sourceRowForDisplay(row)];
  }
  const displayRows = Array.from(
    { length: selection.value.endRow - selection.value.startRow + 1 },
    (_, index) => selection.value!.startRow + index
  );
  if (!rowFilterActive.value) return displayRows;
  return displayRows
    .map((display) => filteredRowMappings.value[display - 1]?.source)
    .filter((source): source is number => Boolean(source));
}

function isAppendResolution(item: RowResolution) {
  return item.kind === "append-row" || item.kind === "append-auto" || item.kind === "append-specified";
}
const contextRows = computed(() => {
  const row = contextMenu.value.row;
  if (!row) return [];
  return rowsForContext(row);
});
const contextActionableRows = computed(() =>
  contextRows.value.filter((row) => rowStatusForSource(row) !== "unchanged")
);
const contextAppendableRows = computed(() =>
  contextActionableRows.value.filter((row) => rowStatusForSource(row) !== "deleted")
);
const contextActionableStatuses = computed(() =>
  contextActionableRows.value.map((row) => rowStatusForSource(row))
);
const contextHasConflict = computed(() =>
  contextActionableStatuses.value.some((status) => status === "conflict")
);
const contextHasNonConflict = computed(() =>
  contextActionableStatuses.value.some((status) => status !== "conflict")
);
const contextHasMixedConflict = computed(() =>
  contextHasConflict.value && contextHasNonConflict.value
);
const contextIsConflict = computed(() =>
  contextHasConflict.value && !contextHasNonConflict.value
);
const contextIsActionable = computed(() =>
  contextActionableRows.value.length > 0 && !contextHasConflict.value
);
const contextStatusLabel = computed(() => {
  if (contextHasMixedConflict.value) return t("diff.includesConflict");
  const statuses = [...new Set(contextActionableStatuses.value)];
  if (statuses.length === 0) return t("common.noDifferences");
  if (statuses.length > 1) return t("diff.mixed");
  return rowStatusLabel(statuses[0]);
});
const contextActionDisabled = computed(() =>
  busy.value || Boolean(summary.value?.options.readonlyLeft) || !comparisonActive.value
);
const contextHasNumericIDColumn = computed(() =>
  Boolean(activeSheet.value?.idColumn && activeSheet.value.nextId > 0)
);
const automaticIDLabel = computed(() => {
  if (!contextHasNumericIDColumn.value) return "";
  const nextID = activeSheet.value?.nextId ?? 0;
  if (!nextID || !contextActionableRows.value.length) return "";
  const lastID = nextID + contextActionableRows.value.length - 1;
  return nextID === lastID ? `id:${nextID}` : `id:${nextID}~${lastID}`;
});
const comparisonActive = computed(() => !repository.value || repository.value.comparisonActive);
const repositoryRows = computed<RepositoryTreeRow[]>(() => {
  type Node = { directories: Map<string, Node>; files: string[] };
  const root: Node = { directories: new Map(), files: [] };
  const query = repositorySearch.value.trim().toLocaleLowerCase();
  const sourceFiles = repositorySidebarTab.value === "differences"
    ? (repository.value?.differenceFiles ?? [])
    : (repository.value?.files ?? []);
  const files = sourceFiles.filter((path) =>
    !query || path.toLocaleLowerCase().includes(query)
  );
  for (const path of files) {
    const parts = path.split("/");
    let node = root;
    for (const directory of parts.slice(0, -1)) {
      let child = node.directories.get(directory);
      if (!child) {
        child = { directories: new Map(), files: [] };
        node.directories.set(directory, child);
      }
      node = child;
    }
    node.files.push(parts.at(-1) ?? path);
  }
  const rows: RepositoryTreeRow[] = [];
  const visit = (node: Node, parent: string, depth: number) => {
    for (const [name, child] of [...node.directories.entries()].sort(([a], [b]) => a.localeCompare(b, undefined, { numeric: true }))) {
      const path = parent ? `${parent}/${name}` : name;
      rows.push({ kind: "directory", path, name, depth });
      if (query || expandedDirectories.value.has(path)) visit(child, path, depth + 1);
    }
    for (const name of node.files.sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))) {
      const path = parent ? `${parent}/${name}` : name;
      rows.push({ kind: "file", path, name, depth });
    }
  };
  visit(root, "", 0);
  return rows;
});
const statusText = computed(() => {
  if (!summary.value) return repository.value ? t("status.selectRepositoryWorkbook") : t("status.noWorkbook");
  const axis = activeCell.value ? cellAxis(activeCell.value, "left") : "—";
  const selectedText = selectionSize.value > 1 ? ` · ${t("diff.selectedCells", { count: selectionSize.value })}` : "";
  const filterText = rowFilterActive.value ? ` · ${rowFilterSummary.value}` : "";
  return `${axis}${selectedText}${filterText} · ${t("diff.count", { count: summary.value.diff.differenceCount })} · ${
    summary.value.dirty ? t("status.unsaved") : t("status.saved")
  }`;
});

async function guard<T>(action: () => Promise<T>): Promise<T | undefined> {
  pendingActions++;
  busy.value = true;
  error.value = "";
  try {
    return await action();
  } catch (reason) {
    error.value = localizeBackendError(reason);
  } finally {
    pendingActions = Math.max(0, pendingActions - 1);
    busy.value = pendingActions > 0;
  }
}

async function initialLoad() {
  startupLoading.value = true;
  try {
    let state = await guard(() => backend.bootstrap());
    while (state?.loading) {
      await new Promise((resolve) => window.setTimeout(resolve, 150));
      state = await guard(() => backend.bootstrap());
    }
    if (state?.error) {
      error.value = state.error;
    }
    if (state?.repository) {
      acceptRepositoryView(state.repository);
    }
    if (state?.hasSession) {
      const data = await guard(() => backend.summary());
      if (data) await acceptSummary(data, data.selectedSheet);
    }
  } finally {
    startupLoading.value = false;
  }
}

function acceptRepositoryView(view: RepositoryView) {
  const repositoryChanged = repository.value?.path !== view.path;
  repository.value = view;
  if (repositoryChanged) {
    repositorySearch.value = "";
    repositorySearchFocused.value = false;
    repositorySearchHistory.value = loadRepositorySearchHistory(view.path);
    const expanded = new Set<string>();
    for (const file of view.files) {
      const parts = file.split("/");
      for (let index = 1; index < parts.length; index++) {
        expanded.add(parts.slice(0, index).join("/"));
      }
    }
    expandedDirectories.value = expanded;
  }
  scheduleDifferenceIndexPoll();
}

function repositorySearchHistoryKey(path: string): string {
  return `${REPOSITORY_SEARCH_HISTORY_PREFIX}${encodeURIComponent(path)}`;
}

function legacyRepositorySearchHistoryKey(path: string): string {
  return `${LEGACY_REPOSITORY_SEARCH_HISTORY_PREFIX}${encodeURIComponent(path)}`;
}

function loadRepositorySearchHistory(path: string): string[] {
  try {
    const key = repositorySearchHistoryKey(path);
    let raw = window.localStorage.getItem(key);
    if (raw === null) {
      raw = window.localStorage.getItem(legacyRepositorySearchHistoryKey(path));
    }
    const saved = JSON.parse(raw ?? "[]");
    if (!Array.isArray(saved)) return [];
    const history = saved
      .filter((item): item is string => typeof item === "string" && Boolean(item.trim()))
      .slice(0, REPOSITORY_SEARCH_HISTORY_LIMIT);
    if (raw !== null && window.localStorage.getItem(key) === null) {
      window.localStorage.setItem(key, JSON.stringify(history));
    }
    return history;
  } catch {
    return [];
  }
}

function recordRepositorySearch(value = repositorySearch.value) {
  const path = repository.value?.path;
  const query = value.trim();
  if (!path || !query) return;
  const normalized = query.toLocaleLowerCase();
  const history = [
    query,
    ...repositorySearchHistory.value.filter((item) => item.toLocaleLowerCase() !== normalized)
  ].slice(0, REPOSITORY_SEARCH_HISTORY_LIMIT);
  repositorySearchHistory.value = history;
  try {
    window.localStorage.setItem(repositorySearchHistoryKey(path), JSON.stringify(history));
  } catch {
    // Search remains usable when WebView storage is unavailable.
  }
}

function finishRepositorySearch() {
  recordRepositorySearch();
  repositorySearchFocused.value = false;
}

function endRepositorySearch() {
  repositorySearchInput.value?.blur();
}

function clearRepositorySearch() {
  repositorySearch.value = "";
  repositorySearchFocused.value = true;
  repositorySearchInput.value?.focus();
}

function applyRepositorySearchHistory(query: string) {
  repositorySearch.value = query;
  recordRepositorySearch(query);
  repositorySearchFocused.value = false;
  nextTick(() => repositorySearchInput.value?.blur());
}

function scheduleDifferenceIndexPoll() {
  window.clearTimeout(differenceIndexTimer);
  const current = repository.value;
  if (!current?.differenceIndexing) return;
  const expectedPath = current.path;
  const expectedRef = current.selectedRef;
  differenceIndexTimer = window.setTimeout(async () => {
    try {
      const result = await backend.repository();
      if (
        repository.value?.path !== expectedPath ||
        repository.value?.selectedRef !== expectedRef
      ) return;
      acceptRepositoryView(result.repository);
    } catch (reason) {
      error.value = reason instanceof Error ? reason.message : String(reason);
    }
  }, 200);
}

async function acceptRepositoryResult(result: RepositoryResult) {
  acceptRepositoryView(result.repository);
  if (result.summary) {
    await acceptSummary(result.summary, result.summary.selectedSheet);
  } else {
    clearWorkbook();
  }
}

function clearWorkbook() {
  sheetRequest++;
  regionRequest++;
  summary.value = null;
  region.value = null;
  sheet.value = "";
  diffs.value = [];
  diffIndex.value = -1;
  activePoint.value = null;
  selection.value = null;
  inlineEdit.value = null;
  externalChangeDialog.value = { visible: false, side: "left", change: null };
  deferredExternalChange.value = null;
  externalNotice.value = "";
  stopOriginalPreview();
}

async function changeLanguage(event: Event) {
  const next = (event.target as HTMLSelectElement).value as LocalePreference;
  settingsNotice.value = null;
  setLocalePreference(next);
  try {
    await backend.setRuntimeLocale(locale.value);
    await backend.setLanguagePreference(next);
  } catch (reason) {
    const message = localizeBackendError(reason);
    if (settingsDialog.value) settingsNotice.value = { kind: "error", message };
    else error.value = message;
  }
}

async function initializeApplication() {
  try {
    initializeLocale(await backend.languagePreference());
    await backend.setRuntimeLocale(locale.value);
  } catch {
    initializeLocale("system");
    try { await backend.setRuntimeLocale(locale.value); } catch { /* older backend compatibility */ }
  }
  await initialLoad();
}

async function chooseRepository() {
  window.clearTimeout(differenceIndexTimer);
  beginRepositoryOpen();
  try {
    const result = await backend.selectRepository();
    await acceptRepositoryResult(result);
    repositoryOpenDialog.value = false;
  } catch (reason) {
    repositoryOpenError.value = reason instanceof Error ? reason.message : String(reason);
    scheduleDifferenceIndexPoll();
  } finally {
    finishRepositoryOpen();
  }
}

function showRepositoryOpenDialog() {
  repositoryOpenError.value = "";
  repositoryOpenDialog.value = true;
}

function closeRepositoryOpenDialog() {
  if (repositoryOpening.value) return;
  repositoryOpenDialog.value = false;
  repositoryOpenError.value = "";
}

function beginRepositoryOpen() {
  repositoryOpenDialog.value = true;
  repositoryOpenError.value = "";
  if (repositoryOpening.value) return;
  repositoryOpening.value = true;
  pendingActions++;
  busy.value = true;
}

function finishRepositoryOpen() {
  if (!repositoryOpening.value) return;
  repositoryOpening.value = false;
  pendingActions = Math.max(0, pendingActions - 1);
  busy.value = pendingActions > 0;
}

async function completeRepositoryDrop(result: RepositoryResult) {
  try {
    await acceptRepositoryResult(result);
    repositoryOpenDialog.value = false;
  } catch (reason) {
    repositoryOpenError.value = reason instanceof Error ? reason.message : String(reason);
  } finally {
    finishRepositoryOpen();
  }
}

async function showRepositorySwitcher() {
  const repositories = await guard(() => backend.recentRepositories());
  if (!repositories) return;
  repositorySwitchDialog.value = { visible: true, repositories };
}

async function switchToRecentRepository(path: string) {
  if (path === repository.value?.path) return;
  repositorySwitchDialog.value.visible = false;
  window.clearTimeout(differenceIndexTimer);
  const result = await guard(() => backend.openRepository(path));
  if (result) await acceptRepositoryResult(result);
  else scheduleDifferenceIndexPoll();
}

async function chooseOtherRepository() {
  repositorySwitchDialog.value.visible = false;
  await chooseRepository();
}

async function chooseFiles() {
  window.clearTimeout(differenceIndexTimer);
  const data = await guard(() => backend.selectAndOpen());
  if (data) {
    repository.value = null;
    await acceptSummary(data, data.selectedSheet);
  } else scheduleDifferenceIndexPoll();
}

function openSettings() {
  settingsNotice.value = null;
  settingsConfirmation.value = null;
  settingsDialog.value = true;
}

function closeSettings() {
  if (busy.value) return;
  settingsConfirmation.value = null;
  settingsDialog.value = false;
}

async function runSettingsAction<T>(action: () => Promise<T>): Promise<T | undefined> {
  pendingActions++;
  busy.value = true;
  settingsNotice.value = null;
  try {
    return await action();
  } catch (reason) {
    settingsNotice.value = { kind: "error", message: localizeBackendError(reason) };
  } finally {
    pendingActions = Math.max(0, pendingActions - 1);
    busy.value = pendingActions > 0;
  }
}

async function configureUGit() {
  const result = await runSettingsAction(() => backend.configureUGit());
  if (result) {
    settingsNotice.value = {
      kind: result.cancelled ? "info" : "success",
      message: result.message
    };
  }
}

function clearStoredClientData() {
  const prefixes = [
    REPOSITORY_SEARCH_HISTORY_PREFIX,
    LEGACY_REPOSITORY_SEARCH_HISTORY_PREFIX,
    SHEET_LAYOUT_PREFIX,
    LEGACY_SHEET_LAYOUT_PREFIX
  ];
  const keys: string[] = [];
  for (let index = 0; index < window.localStorage.length; index++) {
    const key = window.localStorage.key(index);
    if (key && prefixes.some((prefix) => key.startsWith(prefix))) keys.push(key);
  }
  for (const key of keys) window.localStorage.removeItem(key);
}

async function refreshRepositoryAfterDataCleanup() {
  window.clearTimeout(differenceIndexTimer);
  if (!repository.value) return;
  const result = await backend.repository();
  acceptRepositoryView(result.repository);
}

async function confirmSettingsCleanup() {
  const action = settingsConfirmation.value;
  if (!action) return;
  settingsConfirmation.value = null;
  const completed = await runSettingsAction(async () => {
    if (action === "cache") await backend.clearDifferenceIndexCache();
    else await backend.clearAllData();
    if (action === "all") clearStoredClientData();
    await refreshRepositoryAfterDataCleanup();
    return true;
  });
  if (completed) {
    repositorySearchHistory.value = action === "all" ? [] : repositorySearchHistory.value;
    settingsNotice.value = {
      kind: "success",
      message: action === "cache" ? t("settings.cacheCleared") : t("settings.allDataCleared")
    };
  }
}

async function selectRepositoryFile(path: string) {
  window.clearTimeout(differenceIndexTimer);
  const request = ++repositoryRequest;
  const previous = repository.value ? { ...repository.value } : null;
  if (repository.value) {
    repository.value = { ...repository.value, loading: true, leftState: "loading", rightState: "loading" };
  }
  window.setTimeout(() => {
    if (request === repositoryRequest && repository.value?.loading) {
      repository.value = { ...repository.value, rightState: "comparing" };
    }
  }, 120);
  const result = await guard(() => backend.selectRepositoryFile(path));
  if (!result || request !== repositoryRequest) {
    if (!result && request === repositoryRequest && previous) acceptRepositoryView(previous);
    return;
  }
  await acceptRepositoryResult(result);
}

async function selectRepositoryRef(value: string) {
  window.clearTimeout(differenceIndexTimer);
  const request = ++repositoryRequest;
  const previous = repository.value ? { ...repository.value } : null;
  if (repository.value) {
    repository.value = { ...repository.value, loading: true, rightState: "loading" };
  }
  window.setTimeout(() => {
    if (request === repositoryRequest && repository.value?.loading) {
      repository.value = { ...repository.value, rightState: "comparing" };
    }
  }, 120);
  const result = await guard(() => backend.selectRepositoryRef(value));
  if (!result || request !== repositoryRequest) {
    if (!result && request === repositoryRequest && previous) acceptRepositoryView(previous);
    return;
  }
  await acceptRepositoryResult(result);
}

async function refreshRepository() {
  window.clearTimeout(differenceIndexTimer);
  const result = await guard(() => backend.refreshRepository());
  if (result) await acceptRepositoryResult(result);
  else scheduleDifferenceIndexPoll();
}

function toggleDirectory(path: string) {
  const next = new Set(expandedDirectories.value);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  expandedDirectories.value = next;
}

function repositoryStateMessage(side: "left" | "right") {
  if (!repository.value) return "";
  const state = side === "left" ? repository.value.leftState : repository.value.rightState;
  if (state === "loading") return t("repository.stateLoading");
  if (state === "comparing") return t("repository.stateComparing");
  return side === "left" ? repository.value.leftMessage : repository.value.rightMessage;
}

async function acceptSummary(data: Summary, preferredSheet = sheet.value, resetSelection = true) {
  const sourceChanged = summary.value != null && (
    summary.value.diff.leftFile !== data.diff.leftFile ||
    summary.value.diff.rightFile !== data.diff.rightFile
  );
  if (sourceChanged) {
    externalChangeDialog.value = { visible: false, side: "left", change: null };
    deferredExternalChange.value = null;
    externalNotice.value = "";
  }
  summary.value = data;
  const fallback = data.diff.sheets[0]?.name ?? "";
  const preferred = data.diff.sheets.find((item) => item.name === preferredSheet);
  const firstDifferent = data.diff.sheets.find((item) => item.differenceCount > 0);
  const nextSheet = resetSelection && preferred?.differenceCount === 0
    ? (firstDifferent?.name ?? preferred.name)
    : (preferred?.name ?? firstDifferent?.name ?? fallback);
  if (nextSheet) await loadSheet(nextSheet, resetSelection);
}

function sameExternalChange(
  pending: { side: "left" | "right"; change: ExternalFileChange } | null,
  side: "left" | "right",
  change: ExternalFileChange
) {
  return pending?.side === side && pending.change.signature === change.signature;
}

async function reloadExternal(side: "left" | "right") {
  const result = await guard(() => backend.reloadExternal(side));
  if (!result) return;
  if (result.repository) acceptRepositoryView(result.repository);
  externalChangeDialog.value = { visible: false, side: "left", change: null };
  deferredExternalChange.value = null;
  externalNotice.value = result.notice;
  await acceptSummary(result.summary, sheet.value, false);
}

function deferExternalReload() {
  const dialog = externalChangeDialog.value;
  if (!dialog.change) return;
  deferredExternalChange.value = { side: dialog.side, change: dialog.change };
  externalNotice.value = t("externalChange.leftNotice");
  externalChangeDialog.value = { visible: false, side: "left", change: null };
}

async function checkExternalChanges() {
  if (
    document.visibilityState !== "visible" || checkingExternalChanges || startupLoading.value || busy.value || !summary.value ||
    externalChangeDialog.value.visible || deferredExternalChange.value || inlineEdit.value
  ) return;
  checkingExternalChanges = true;
  try {
    const changes = await backend.checkExternalChanges();
    externalCheckFailures = 0;
    if (changes.right?.changed) {
      await reloadExternal("right");
    }
    if (!changes.left?.changed) return;
    if (sameExternalChange(deferredExternalChange.value, "left", changes.left)) return;
    if (!changes.left.writable) {
      await reloadExternal("left");
      return;
    }
    externalChangeDialog.value = { visible: true, side: "left", change: changes.left };
  } catch (reason) {
    externalCheckFailures++;
    if (externalCheckFailures >= 2) {
      error.value = reason instanceof Error ? reason.message : String(reason);
    }
  } finally {
    checkingExternalChanges = false;
  }
}

function scheduleExternalCheck() {
  window.clearTimeout(externalFocusTimer);
  externalFocusTimer = window.setTimeout(() => void checkExternalChanges(), 250);
}

function onVisibilityChange() {
  if (document.visibilityState === "visible") scheduleExternalCheck();
}

async function loadSheet(name: string, resetSelection = true) {
  stopOriginalPreview();
  const request = ++sheetRequest;
  const changed = sheet.value !== name;
  sheet.value = name;
  if (changed || resetSelection) {
    loadLayout(name);
  }
  if (resetSelection) {
    activePoint.value = null;
    selectionAnchor.value = null;
    selection.value = null;
    inlineEdit.value = null;
  }
  diffIndex.value = -1;
  const list = await guard(() => backend.differences(name, 0, 10000));
  if (request !== sheetRequest || sheet.value !== name) return;
  diffs.value = list ?? [];
  const persistedNavigation = DIFF_FILTER_TABS.find((status) =>
    selectedRowFilterSet.value.has(status) && diffs.value.some((item) => item.rowStatus === status)
  ) ?? selectedRowFilters.value[0];
  setDiffNavigationFilter(persistedNavigation ?? preferredDiffFilter(diffs.value));
  if (changed || resetSelection) {
    const target = rowFilterActive.value
      ? diffs.value.find((item) => selectedRowFilterSet.value.has(item.rowStatus as DiffFilter))
      : filteredDiffEntries.value[0]?.item;
    if (target) {
      if (rowFilterActive.value && target.rowStatus !== "unchanged") {
        setDiffNavigationFilter(target.rowStatus);
      }
      await nextTick();
      if (request !== sheetRequest || sheet.value !== name) return;
      await scrollTo(target.ref.row, target.ref.col);
      return;
    }
  }
  const source = leftScroll.value ?? rightScroll.value;
  const fromRow = changed || resetSelection
    ? 1
    : Math.floor((source?.scrollTop ?? viewportTop.value) / rowHeight.value) + 1;
  const fromCol = changed || resetSelection
    ? 1
    : columnAtOffset(source?.scrollLeft ?? viewportLeft.value);
  const target = viewportRegionTarget(source, fromRow, fromCol);
  await loadRegion(target.row, target.col);
  if (request !== sheetRequest || sheet.value !== name) return;
  if (changed || resetSelection) {
    await nextTick();
    if (request !== sheetRequest || sheet.value !== name) return;
    for (const element of [leftScroll.value, rightScroll.value]) {
      if (element) {
        element.scrollTop = 0;
        element.scrollLeft = 0;
      }
    }
    updateViewportOffsets(0, 0);
  }
}

async function loadRegion(fromRow: number, fromCol: number): Promise<boolean> {
  if (!sheet.value) return false;
  const request = ++regionRequest;
  const requestedSheet = sheet.value;
  const requestedFilters = [...selectedRowFilters.value];
  try {
    const data = requestedFilters.length
      ? await backend.filteredRegion(
        requestedSheet,
        requestedFilters,
        Math.max(1, fromRow),
        VIEW_ROWS,
        Math.max(1, fromCol),
        VIEW_COLS
      )
      : await backend.region(
        requestedSheet,
        Math.max(1, fromRow),
        VIEW_ROWS,
        Math.max(1, fromCol),
        VIEW_COLS
      );
    if (
      request !== regionRequest ||
      sheet.value !== requestedSheet ||
      requestedFilters.join(",") !== selectedRowFilters.value.join(",")
    ) return false;
    region.value = data;
    return true;
  } catch (reason) {
    if (request === regionRequest && sheet.value === requestedSheet) {
      error.value = reason instanceof Error ? reason.message : String(reason);
    }
    return false;
  }
}

function viewportRegionTarget(source: HTMLElement | null, fallbackRow?: number, fallbackCol?: number) {
  const top = source?.scrollTop ?? viewportTop.value;
  const left = source?.scrollLeft ?? viewportLeft.value;
  const firstRow = fallbackRow ?? Math.floor(top / rowHeight.value) + 1;
  const lastRow = Math.floor((top + Math.max(0, source?.clientHeight ?? 0)) / rowHeight.value) + 1;
  const visibleRowCount = Math.max(1, lastRow - firstRow + 1);
  const rowPadding = Math.max(4, Math.floor((VIEW_ROWS - visibleRowCount) / 2));
  const rawRow = Math.max(1, firstRow - rowPadding);
  const row = 1 + Math.floor((rawRow - 1) / REGION_ROW_STEP) * REGION_ROW_STEP;

  const firstCol = fallbackCol ?? columnAtOffset(left);
  const lastCol = columnAtOffset(left + Math.max(0, source?.clientWidth ?? 0));
  const visibleColCount = Math.max(1, lastCol - firstCol + 1);
  const colPadding = Math.max(2, Math.floor((VIEW_COLS - visibleColCount) / 2));
  const rawCol = Math.max(1, firstCol - colPadding);
  const col = 1 + Math.floor((rawCol - 1) / REGION_COL_STEP) * REGION_COL_STEP;
  return { row, col };
}

function requestViewportRegion(source: HTMLElement | null) {
  const target = viewportRegionTarget(source);
  if (region.value?.fromRow === target.row && region.value.fromCol === target.col) return;
  void loadRegion(target.row, target.col);
}

function scheduleViewportRegion(source: HTMLElement) {
  if (loadingTimer) return;
  loadingTimer = window.setTimeout(() => {
    loadingTimer = 0;
    requestViewportRegion(source);
  }, REGION_SCHEDULE_MS);
}

function applySyncedScroll(source: HTMLElement, top: number, left: number) {
  const target = source === leftScroll.value ? rightScroll.value : leftScroll.value;
  syncing = true;
  source.scrollTop = Math.max(0, top);
  source.scrollLeft = Math.max(0, left);
  const actualTop = source.scrollTop;
  const actualLeft = source.scrollLeft;
  if (target) {
    target.scrollTop = actualTop;
    target.scrollLeft = actualLeft;
  }
  syncing = false;
  updateViewportOffsets(actualTop, actualLeft);
  scheduleViewportRegion(source);
}

function onScroll(side: "left" | "right") {
  if (syncing) return;
  const source = side === "left" ? leftScroll.value : rightScroll.value;
  if (!source) return;
  const target = side === "left" ? rightScroll.value : leftScroll.value;
  if (
    target &&
    target.scrollTop === source.scrollTop &&
    target.scrollLeft === source.scrollLeft &&
    viewportTop.value === source.scrollTop &&
    viewportLeft.value === source.scrollLeft
  ) return;
  applySyncedScroll(source, source.scrollTop, source.scrollLeft);
}

function updateViewportOffsets(top: number, left: number) {
  viewportTop.value = top;
  viewportLeft.value = left;
}

function setSingleSelection(cell: RegionCell) {
  const point = { row: cell.row, col: cell.col };
  activePoint.value = point;
  selectionAnchor.value = point;
  selection.value = makeRange(point, point);
  const sourceRow = cellSourceRow(cell);
  const index = diffs.value.findIndex((item) => item.ref.row === sourceRow && item.ref.col === cell.col);
  if (index >= 0) {
    const status = diffs.value[index].rowStatus || "modified";
    if (status !== "unchanged") diffFilter.value = status;
    diffIndex.value = index;
  }
}

function beginCellSelection(cell: RegionCell, event: PointerEvent) {
  if (event.button !== 0) return;
  contextMenu.value.visible = false;
  const point = { row: cell.row, col: cell.col };
  if (event.shiftKey && selectionAnchor.value) {
    dragAnchor = selectionAnchor.value;
  } else {
    selectionAnchor.value = point;
    dragAnchor = point;
  }
  activePoint.value = point;
  selection.value = makeRange(dragAnchor, point);
  draggingSelection = true;
  draggingRows = false;
}

function extendCellSelection(cell: RegionCell) {
  if (!draggingSelection || !dragAnchor) return;
  const point = { row: cell.row, col: cell.col };
  activePoint.value = point;
  selection.value = makeRange(dragAnchor, point);
}

function beginRowSelection(row: number, event: PointerEvent | MouseEvent) {
  if ("button" in event && event.button !== 0) return;
  contextMenu.value.visible = false;
  const lastCol = Math.max(activeSheet.value?.maxCol ?? 1, 1);
  const rowPoint = { row, col: region.value?.fromCol ?? 1 };
  const anchorRow = event.shiftKey && selectionAnchor.value ? selectionAnchor.value.row : row;
  if (!event.shiftKey || !selectionAnchor.value) {
    selectionAnchor.value = { row, col: 1 };
  }
  selection.value = {
    startRow: Math.min(anchorRow, row),
    endRow: Math.max(anchorRow, row),
    startCol: 1,
    endCol: lastCol
  };
  activePoint.value = rowPoint;
  dragAnchor = { row: anchorRow, col: 1 };
  draggingRows = event.type.startsWith("pointer");
  draggingSelection = false;
}

function extendRowSelection(row: number) {
  if (!draggingRows || !dragAnchor) return;
  const lastCol = Math.max(activeSheet.value?.maxCol ?? 1, 1);
  selection.value = {
    startRow: Math.min(dragAnchor.row, row),
    endRow: Math.max(dragAnchor.row, row),
    startCol: 1,
    endCol: lastCol
  };
  activePoint.value = { row, col: region.value?.fromCol ?? 1 };
}

function finishPointerAction() {
  draggingSelection = false;
  draggingRows = false;
  dragAnchor = null;
  if (resizeState) {
    resizeState = null;
    markerColumnWidths.value = { ...columnWidths.value };
    persistLayout();
  }
  if (repositoryResize) {
    void finishRepositoryResize();
  }
}

async function navigate(direction: 1 | -1) {
  if (!comparisonActive.value) return;
  const entries = filteredDiffEntries.value;
  const current = entries.findIndex(({ index }) => index === diffIndex.value);
  const filteredIndex = nextDiffIndex(current, entries.length, direction);
  if (filteredIndex < 0) return;
  const targetEntry = entries[filteredIndex];
  diffIndex.value = targetEntry.index;
  const target = targetEntry.item;
  await scrollTo(target.ref.row, target.ref.col);
}

function setDiffNavigationFilter(status: DiffFilter) {
  diffFilter.value = status;
  diffIndex.value = diffs.value.findIndex((item) =>
    (item.rowStatus || "modified") === status
  );
}

function captureRowViewportAnchor(): RowViewportAnchor {
  const source = leftScroll.value ?? rightScroll.value;
  const top = source?.scrollTop ?? viewportTop.value;
  const firstVisible = Math.max(1, Math.floor(top / rowHeight.value) + 1);
  const lastVisible = Math.max(
    firstVisible,
    Math.floor((top + Math.max(0, source?.clientHeight ?? 0)) / rowHeight.value) + 1
  );
  const displayRow = activePoint.value?.row &&
    activePoint.value.row >= firstVisible && activePoint.value.row <= lastVisible
    ? activePoint.value.row
    : firstVisible;
  return {
    sourceRow: sourceRowForDisplay(displayRow),
    viewportOffset: top - (displayRow - 1) * rowHeight.value
  };
}

function displayRowForAnchor(sourceRow: number): number {
  if (!rowFilterActive.value) return Math.max(1, sourceRow);
  const exact = filteredRowMappings.value.find((item) => item.source === sourceRow);
  if (exact) return exact.display;
  let nearest: FilteredRowMapping | undefined;
  for (const item of filteredRowMappings.value) {
    if (!nearest || Math.abs(item.source - sourceRow) < Math.abs(nearest.source - sourceRow)) {
      nearest = item;
    }
  }
  return nearest?.display ?? 1;
}

async function applyRowFilters(focusStatus?: DiffFilter, anchor?: RowViewportAnchor) {
  regionRequest++;
  activePoint.value = null;
  selectionAnchor.value = null;
  selection.value = null;
  inlineEdit.value = null;
  if (focusStatus) setDiffNavigationFilter(focusStatus);
  else if (rowFilterActive.value && !selectedRowFilterSet.value.has(diffFilter.value)) {
    setDiffNavigationFilter(selectedRowFilters.value[0]);
  }
  await nextTick();
  const displayRow = anchor ? displayRowForAnchor(anchor.sourceRow) : 1;
  const desiredTop = Math.max(
    0,
    (displayRow - 1) * rowHeight.value + (anchor?.viewportOffset ?? 0)
  );
  for (const element of [leftScroll.value, rightScroll.value]) {
    if (element) element.scrollTop = desiredTop;
  }
  const source = leftScroll.value ?? rightScroll.value;
  const actualTop = source?.scrollTop ?? desiredTop;
  for (const element of [leftScroll.value, rightScroll.value]) {
    if (element) element.scrollTop = actualTop;
  }
  updateViewportOffsets(actualTop, viewportLeft.value);
  const target = viewportRegionTarget(
    source,
    Math.floor(actualTop / rowHeight.value) + 1,
    columnAtOffset(viewportLeft.value)
  );
  await loadRegion(target.row, target.col);
}

async function toggleRowFilter(status: DiffFilter) {
  const anchor = captureRowViewportAnchor();
  const next = new Set(selectedRowFilters.value);
  if (next.has(status)) next.delete(status);
  else next.add(status);
  selectedRowFilters.value = DIFF_FILTER_TABS.filter((item) => next.has(item));
  await applyRowFilters(next.has(status) ? status : undefined, anchor);
}

async function toggleAllRowFilters() {
  const anchor = captureRowViewportAnchor();
  selectedRowFilters.value = selectedRowFilters.value.length === DIFF_FILTER_TABS.length
    ? []
    : [...DIFF_FILTER_TABS];
  await applyRowFilters(undefined, anchor);
}

async function selectDiffIndexFilter(status: DiffFilter) {
  if (selectedRowFilters.value.length !== 1 || selectedRowFilters.value[0] !== status) {
    selectedRowFilters.value = [status];
    await applyRowFilters(status);
    return;
  }
  setDiffNavigationFilter(status);
}

async function toggleRowAlignment() {
  const current = summary.value;
  if (!current || !activeRowAlignment.value?.available || current.options.gitMerge || current.undoCount > 0) return;
  const mode = current.rowAlignment.mode === "auto" ? "position" : "auto";
  const data = await guard(() => backend.setRowAlignment(mode));
  if (data) await acceptSummary(data, sheet.value, false);
}

async function setContextKeyColumn() {
  const current = summary.value;
  if (!current || contextMenu.value.kind !== "column" || current.undoCount > 0) return;
  const column = activeSheet.value?.idColumn === contextMenu.value.col ? 0 : contextMenu.value.col;
  const data = await guard(() => backend.setKeyColumn(sheet.value, column));
  contextMenu.value.visible = false;
  if (data) await acceptSummary(data, sheet.value, false);
}

function selectDiffEntry(index: number, item: CellDiff) {
  diffIndex.value = index;
  void scrollTo(item.ref.row, item.ref.col);
}

async function scrollTo(row: number, col: number) {
  const displayRow = rowFilterActive.value
    ? (filteredRowMappings.value.find((item) => item.source === row)?.display ?? 1)
    : row;
  const desiredTop = Math.max(0, (displayRow - 2) * rowHeight.value);
  const desiredLeft = columnOffsets.value[Math.max(1, col - 1)] ?? 0;
  for (const element of [leftScroll.value, rightScroll.value]) {
    if (element) {
      element.scrollTop = desiredTop;
      element.scrollLeft = desiredLeft;
    }
  }
  // scrollTop/scrollLeft are clamped synchronously by the browser near the
  // bottom and right edges. Header overlays and the virtual region must use
  // those actual values, not the requested values.
  const source = leftScroll.value ?? rightScroll.value;
  const actualTop = source?.scrollTop ?? desiredTop;
  const actualLeft = source?.scrollLeft ?? desiredLeft;
  for (const element of [leftScroll.value, rightScroll.value]) {
    if (element) {
      element.scrollTop = actualTop;
      element.scrollLeft = actualLeft;
    }
  }
  updateViewportOffsets(actualTop, actualLeft);
  const target = viewportRegionTarget(source);
  const loaded = await loadRegion(target.row, target.col);
  if (!loaded) return;
  await nextTick();
  const cell = region.value?.cells.find((item) => cellSourceRow(item) === row && item.col === col);
  if (cell) setSingleSelection(cell);
}

async function copySelection() {
  if (!copyTargets.value.length || !comparisonActive.value) return;
  const focus = activePoint.value ? { ...activePoint.value } : null;
  const savedSelection = selection.value ? { ...selection.value } : null;
  contextMenu.value.visible = false;
  const data = await guard(() => backend.copyMany(sheet.value, copyTargets.value));
  if (data) {
    await acceptSummary(data, sheet.value, false);
    selection.value = savedSelection;
    activePoint.value = focus;
  }
}

async function runContextAction(action: () => Promise<Summary>) {
  const focus = activePoint.value ? { ...activePoint.value } : null;
  const savedSelection = selection.value ? { ...selection.value } : null;
  contextMenu.value.visible = false;
  const data = await guard(action);
  if (!data) return;
  await acceptSummary(data, sheet.value, false);
  selection.value = savedSelection;
  activePoint.value = focus;
}

async function copyContextCells() {
  if (!copyTargets.value.length || copyTargets.value.length > 10000) return;
  await runContextAction(() =>
    backend.copyMany(sheet.value, copyTargets.value)
  );
}

async function copyContextRows() {
  if (!contextActionableRows.value.length) return;
  await runContextAction(() =>
    backend.copyRows(sheet.value, contextActionableRows.value)
  );
}

async function appendContextRowsAutomatically() {
  if (!contextActionableRows.value.length || !automaticIDLabel.value) return;
  await runContextAction(() =>
    backend.appendRows(sheet.value, contextActionableRows.value, [])
  );
}

async function appendContextRowsWithoutNumericID() {
  if (!contextAppendableRows.value.length || contextHasNumericIDColumn.value) return;
  await runContextAction(() =>
    backend.appendRows(sheet.value, contextAppendableRows.value, [])
  );
}

function openSpecifiedIDDialog() {
  idDialog.value = {
    visible: true,
    rows: [...contextActionableRows.value],
    values: contextActionableRows.value.map(() => "")
  };
  contextMenu.value.visible = false;
}

async function confirmSpecifiedIDs() {
  const current = {
    rows: [...idDialog.value.rows],
    values: [...idDialog.value.values]
  };
  if (!current.rows.length || current.values.some((value) => !value.trim())) return;
  idDialog.value.visible = false;
  await runContextAction(() =>
    backend.appendRows(sheet.value, current.rows, current.values)
  );
}

function startInlineEdit(cell: RegionCell) {
  if (busy.value || previewOriginal.value || summary.value?.options.readonlyLeft) return;
  setSingleSelection(cell);
  inlineEdit.value = {
    row: cell.row,
    col: cell.col,
    value: cell.left.formula ? `=${cell.left.formula}` : cell.left.raw,
    original: cell
  };
  void nextTick(() => {
    const input = document.querySelector<HTMLInputElement>(".inline-cell-editor");
    input?.focus();
    input?.select();
  });
}

function cancelInlineEdit() {
  inlineEdit.value = null;
}

function inlineEditType(value: string, original: RegionCell): string {
  if (value === "") return "clear";
  if (value.startsWith("=")) return "formula";
  if (original.right.present && value === original.right.raw) {
    return original.right.type === "number" ? "number" : "text";
  }
  if (/^[+-]?(?:\d+\.?\d*|\.\d+)(?:e[+-]?\d+)?$/i.test(value.trim())) {
    return "number";
  }
  return "text";
}

async function commitInlineEdit() {
  const current = inlineEdit.value;
  if (!current || busy.value) return;
  inlineEdit.value = null;
  const originalValue = current.original.left.formula
    ? `=${current.original.left.formula}`
    : current.original.left.raw;
  if (current.value === originalValue) return;
  const type = inlineEditType(current.value, current.original);
  const data = await guard(() =>
    backend.edit(sheet.value, cellLeftRow(current.original), current.col, current.value, type)
  );
  if (data) {
    await acceptSummary(data, sheet.value, false);
    const cell = region.value?.cells.find((item) =>
      item.row === current.row && item.col === current.col
    );
    if (cell) setSingleSelection(cell);
  }
}

function inlineEditing(cell: RegionCell) {
  return inlineEdit.value?.row === cell.row && inlineEdit.value?.col === cell.col;
}

function displayDifferenceValue(cell: RegionCell | null, side: "left" | "right") {
  if (!cell) return t("grid.noCell");
  const value = displayedCellValue(cell, side);
  if (!value.present) return t("grid.missingValue");
  if (value.formula) return `=${value.formula}`;
  if (value.raw === "") return t("grid.emptyString");
  return value.display || value.raw;
}

function displayedCellValue(cell: RegionCell, side: "left" | "right") {
  if (side === "left" && previewOriginal.value) {
    return cell.originalLeft ?? cell.left;
  }
  return cell[side];
}

function displayedCellText(cell: RegionCell, side: "left" | "right") {
  const value = displayedCellValue(cell, side);
  return value.formula ? `=${value.formula}` : value.display;
}

function leftChangedSinceOpen(cell: RegionCell) {
  const original = cell.originalLeft ?? cell.left;
  const current = cell.left;
  if (!original.present && !current.present) return false;
  return original.present !== current.present ||
    original.raw !== current.raw ||
    (original.formula ?? "") !== (current.formula ?? "") ||
    original.type !== current.type;
}

function syncOriginalPreview() {
  previewOriginal.value = canPreviewOriginal.value && (previewFromKeyboard || previewFromPointer);
}

function startOriginalPreviewFromPointer(event: PointerEvent) {
  if (!canPreviewOriginal.value) return;
  event.preventDefault();
  previewFromPointer = true;
  syncOriginalPreview();
}

function startOriginalPreviewFromKeyboardButton() {
  if (!canPreviewOriginal.value) return;
  previewFromPointer = true;
  syncOriginalPreview();
}

function stopOriginalPreviewFromPointer() {
  previewFromPointer = false;
  syncOriginalPreview();
}

function stopOriginalPreview() {
  previewFromKeyboard = false;
  previewFromPointer = false;
  previewOriginal.value = false;
}

function focusGrid(event: PointerEvent) {
  const target = event.target;
  if (target instanceof Element && target.matches("input, textarea, [contenteditable='true']")) return;
  (event.currentTarget as HTMLElement).focus({ preventScroll: true });
}

function gridOwnsKeyboardFocus(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest(".grid-scroll")) &&
    !target.matches("input, textarea, [contenteditable='true']");
}

function gridKeyboardSide(target: EventTarget | null): "left" | "right" | null {
  if (!(target instanceof Element)) return null;
  const grid = target.closest(".grid-scroll");
  if (grid === leftScroll.value) return "left";
  if (grid === rightScroll.value) return "right";
  return null;
}

function selectAllCells() {
  const current = activeSheet.value;
  if (!current) return;
  const endRow = rowFilterActive.value ? filteredRowMappings.value.length : current.maxRow;
  const endCol = current.maxCol;
  if (endRow < 1 || endCol < 1) return;
  const first = { row: 1, col: 1 };
  selectionAnchor.value = first;
  activePoint.value = first;
  selection.value = {
    startRow: 1,
    endRow,
    startCol: 1,
    endCol
  };
}

async function clearSelectedCells() {
  const current = selection.value;
  if (!current || busy.value || leftReadonly.value) return;
  const focus = activePoint.value ? { ...activePoint.value } : null;
  const savedSelection = { ...current };
  const rows = rowFilterActive.value
    ? Array.from(
        new Set(
          Array.from(
            { length: current.endRow - current.startRow + 1 },
            (_, index) => leftRowForDisplay(current.startRow + index)
          ).filter((row) => row > 0)
        )
      )
    : [];
  const data = await guard(() => backend.clearSelection(
    sheet.value,
    current.startRow,
    current.endRow,
    current.startCol,
    current.endCol,
    rows
  ));
  if (!data) return;
  await acceptSummary(data, sheet.value, false);
  selection.value = savedSelection;
  activePoint.value = focus;
}

async function undo() {
  const data = await guard(() => backend.undo());
  if (data) await acceptSummary(data, sheet.value, false);
}

function onWindowKeyDown(event: KeyboardEvent) {
  if (event.key === "Escape" && settingsConfirmation.value) {
    settingsConfirmation.value = null;
    return;
  }
  if (event.key === "Escape" && settingsDialog.value) {
    closeSettings();
    return;
  }
  if (event.key === "Escape" && externalChangeDialog.value.visible) {
    deferExternalReload();
    return;
  }
  if (event.key === "Escape" && repositoryOpenDialog.value) {
    closeRepositoryOpenDialog();
    return;
  }
  if (event.key === "Escape" && repositorySwitchDialog.value.visible) {
    repositorySwitchDialog.value.visible = false;
    return;
  }
  if (event.key === "Tab" && gridOwnsKeyboardFocus(event.target) && canPreviewOriginal.value) {
    event.preventDefault();
    previewFromKeyboard = true;
    syncOriginalPreview();
    return;
  }
  const target = event.target;
  const editing = target instanceof Element && target.matches("input, textarea, select, [contenteditable='true']");
  const gridSide = gridKeyboardSide(target);
  const key = event.key.toLowerCase();
  if (
    summary.value && gridSide && !editing && !event.altKey && !event.shiftKey &&
    (event.ctrlKey || event.metaKey) && key === "a"
  ) {
    event.preventDefault();
    selectAllCells();
    return;
  }
  if (
    summary.value && gridSide === "left" && !editing && !event.repeat &&
    !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey &&
    (event.key === "Backspace" || event.key === "Delete") &&
    selection.value && !leftReadonly.value && !busy.value
  ) {
    event.preventDefault();
    void clearSelectedCells();
    return;
  }
  if (
    summary.value && !busy.value && !editing && !event.repeat &&
    !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey &&
    /^[1-5]$/.test(event.key)
  ) {
    event.preventDefault();
    if (event.key === "5") void toggleAllRowFilters();
    else void toggleRowFilter(DIFF_FILTER_TABS[Number(event.key) - 1]);
    return;
  }
  if (!event.ctrlKey && !event.metaKey) return;
  if (key === "s") {
    if (!summary.value || summary.value.options.readonlyLeft || busy.value) return;
    event.preventDefault();
    if (event.shiftKey) {
      void saveFromShortcut(true);
    } else if (summary.value.dirty || inlineEdit.value) {
      void saveFromShortcut(false);
    }
    return;
  }
  if (event.shiftKey || key !== "z") return;
  if (target instanceof Element && target.matches("input, textarea, [contenteditable='true']")) return;
  if (!summary.value?.undoCount || busy.value) return;
  event.preventDefault();
  undo();
}

function onWindowKeyUp(event: KeyboardEvent) {
  if (event.key !== "Tab" || !previewFromKeyboard) return;
  previewFromKeyboard = false;
  syncOriginalPreview();
}

async function saveFromShortcut(as: boolean) {
  if (inlineEdit.value) await commitInlineEdit();
  if (!busy.value) await save(as);
}

async function save(as = false) {
  const data = await guard(() => (as ? backend.saveAs() : backend.save()));
  if (data) await acceptSummary(data, sheet.value, false);
}

function sourceRowForDisplay(row: number): number {
  return rowFilterActive.value ? (filteredRowMappings.value[row - 1]?.source ?? row) : row;
}

function leftRowForDisplay(row: number): number {
  return rowFilterActive.value ? (filteredRowMappings.value[row - 1]?.left ?? row) : row;
}

function rightRowForDisplay(row: number): number {
  return rowFilterActive.value ? (filteredRowMappings.value[row - 1]?.right ?? row) : row;
}

function cellSourceRow(cell: RegionCell): number {
  return cell.sourceRow ?? sourceRowForDisplay(cell.row);
}

function cellLeftRow(cell: RegionCell): number {
  return cell.leftRow ?? leftRowForDisplay(cell.row);
}

function cellRightRow(cell: RegionCell): number {
  return cell.rightRow ?? rightRowForDisplay(cell.row);
}

function rowStatusForSource(row: number): RowStatus {
  return activeSheet.value?.rows?.find((item) => item.row === row)?.status ?? "unchanged";
}

function rowStatus(row: number): RowStatus {
  const sourceRow = sourceRowForDisplay(row);
  const classified = rowStatusForSource(sourceRow);
  if (classified !== "unchanged") return classified;
  const cells = region.value?.cells.filter((item) => item.row === row) ?? [];
  const regionStatus = cells.find((item) => item.rowStatus !== "unchanged")?.rowStatus;
  if (regionStatus) return regionStatus;
  return cells.some((item) => item.status !== "unchanged") ? "modified" : "unchanged";
}

function rowStatusLabel(status: RowStatus | string) {
  switch (status) {
  case "added": return t("diff.added");
  case "deleted": return t("diff.deleted");
  case "modified": return t("diff.modified");
  case "conflict": return t("diff.conflict");
  default: return t("common.noDifferences");
  }
}

function rowResolution(
  row: number,
  side: "left" | "right"
): RowResolution | null {
  const sourceRow = sourceRowForDisplay(row);
  const leftRow = leftRowForDisplay(row);
  const items = summary.value?.resolutions ?? [];
  for (let index = items.length - 1; index >= 0; index--) {
    const item = items[index];
    if (item.sheet !== sheet.value) continue;
    if (side === "right" && item.sourceRow === sourceRow) return item;
    if (
      side === "left" &&
      item.targetRow === leftRow &&
      isAppendResolution(item)
    ) return item;
  }
  return null;
}

function resolutionLabel(item: RowResolution) {
  switch (item.kind) {
  case "overwrite-row":
    return t("resolution.rowOverwritten");
  case "overwrite-cells":
    return t("resolution.cellsOverwritten", { count: item.cellCount || 1 });
  case "append-auto":
    return t("resolution.appendedAuto", { id: item.targetId });
  case "append-specified":
    return t("resolution.appendedSpecified", { id: item.targetId });
  case "append-row":
    return t("resolution.appendedEnd");
  }
}

function cellClass(cell: RegionCell, side: "left" | "right") {
  if (side === "left" && previewOriginal.value) {
    return {
      cell: true,
      "original-preview-cell": true,
      "original-preview-changed": leftChangedSinceOpen(cell),
      selected: containsCell(selection.value, cell.row, cell.col),
      active: activePoint.value?.row === cell.row && activePoint.value?.col === cell.col
    };
  }
  const rowState = cell.rowStatus || rowStatus(cell.row);
  const appendedTarget = side === "left" ? rowResolution(cell.row, "left") : null;
  let changeState: Exclude<RowStatus, "unchanged" | "conflict"> | "" = "";
  if (appendedTarget && cell.left.present) {
    changeState = "added";
  } else {
    switch (cell.status) {
    case "right-added":
    case "left-missing":
      if (side === "right") changeState = "added";
      break;
    case "left-added":
    case "right-missing":
      if (side === "left") changeState = "deleted";
      break;
    case "modified":
      changeState = side === "left" ? "deleted" : "added";
      break;
    }
  }
  return {
    cell: true,
    difference: cell.status !== "unchanged",
    [`cell-${changeState}`]: Boolean(changeState) && (rowState !== "conflict" || Boolean(appendedTarget)),
    "row-conflict": rowState === "conflict" && !appendedTarget,
    "resolution-added": Boolean(appendedTarget),
    selected: containsCell(selection.value, cell.row, cell.col),
    active: activePoint.value?.row === cell.row && activePoint.value?.col === cell.col,
    missing: cell.status !== "unchanged" && !cell[side].present
  };
}

function rowClass(row: number, side: "left" | "right") {
  if (side === "left" && previewOriginal.value) {
    return {
      "row-header": true,
      selected: Boolean(selection.value && row >= selection.value.startRow && row <= selection.value.endRow)
    };
  }
  const status = rowStatus(row);
  const appendedTarget = side === "left" ? rowResolution(row, "left") : null;
  return {
    "row-header": true,
    [`row-${appendedTarget ? "added" : status}`]: Boolean(appendedTarget) || status !== "unchanged",
    resolved: side === "right" && Boolean(rowResolution(row, "right")),
    selected: Boolean(selection.value && row >= selection.value.startRow && row <= selection.value.endRow)
  };
}

function columnWidth(col: number) {
  return Math.round((columnWidths.value[col] ?? BASE_COL_WIDTH) * zoom.value);
}

function markerColumnWidth(col: number) {
  return Math.round((markerColumnWidths.value[col] ?? BASE_COL_WIDTH) * zoom.value);
}

function columnLeft(col: number) {
  return rowHeaderWidth.value + (columnOffsets.value[col] ?? 0);
}

function columnAtOffset(scrollLeft: number) {
  const target = Math.max(0, scrollLeft - rowHeaderWidth.value);
  const offsets = columnOffsets.value;
  let low = 1;
  let high = totalCols.value;
  while (low < high) {
    const middle = Math.ceil((low + high) / 2);
    if (offsets[middle] <= target) low = middle;
    else high = middle - 1;
  }
  return Math.max(1, low);
}

function layoutKey(name = sheet.value) {
  const file = summary.value?.diff.leftFile ?? "no-workbook";
  return `${SHEET_LAYOUT_PREFIX}${file}:${name}`;
}

function legacyLayoutKey(name = sheet.value) {
  const file = summary.value?.diff.leftFile ?? "no-workbook";
  return `${LEGACY_SHEET_LAYOUT_PREFIX}${file}:${name}`;
}

function loadLayout(name: string) {
  zoom.value = 1;
  columnWidths.value = {};
  markerColumnWidths.value = {};
  try {
    const key = layoutKey(name);
    let saved = window.localStorage.getItem(key);
    if (!saved) saved = window.localStorage.getItem(legacyLayoutKey(name));
    if (!saved) return;
    const value = JSON.parse(saved) as { zoom?: number; widths?: Record<number, number> };
    zoom.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, value.zoom ?? 1));
    columnWidths.value = value.widths ?? {};
    markerColumnWidths.value = { ...columnWidths.value };
    if (window.localStorage.getItem(key) === null) {
      window.localStorage.setItem(key, JSON.stringify(value));
    }
  } catch {
    // Invalid or unavailable local storage should not block workbook comparison.
  }
}

function persistLayout() {
  if (!sheet.value) return;
  try {
    window.localStorage.setItem(layoutKey(), JSON.stringify({
      zoom: zoom.value,
      widths: columnWidths.value
    }));
  } catch {
    // Layout caching is best effort.
  }
}

function rowLabel(row: number, side: "left" | "right") {
  return side === "left" ? leftRowForDisplay(row) : rightRowForDisplay(row);
}

function cellAxis(cell: RegionCell, side: "left" | "right") {
  const row = side === "left" ? cellLeftRow(cell) : cellRightRow(cell);
  return `${columnName(cell.col)}${row}`;
}

function queueSyncedWheel(source: HTMLElement, deltaTop: number, deltaLeft: number) {
  wheelSource = source;
  wheelDeltaTop += deltaTop;
  wheelDeltaLeft += deltaLeft;
  if (wheelFrame) return;
  wheelFrame = window.requestAnimationFrame(() => {
    wheelFrame = 0;
    const current = wheelSource;
    const top = wheelDeltaTop;
    const left = wheelDeltaLeft;
    wheelSource = null;
    wheelDeltaTop = 0;
    wheelDeltaLeft = 0;
    if (current) {
      applySyncedScroll(current, current.scrollTop + top, current.scrollLeft + left);
    }
  });
}

function onGridWheel(event: WheelEvent) {
  const source = event.currentTarget as HTMLElement;
  if (!event.ctrlKey && !event.metaKey) {
    event.preventDefault();
    const linePixels = rowHeight.value;
    const pageY = Math.max(linePixels, source.clientHeight);
    const pageX = Math.max(columnWidth(columnAtOffset(source.scrollLeft)), source.clientWidth);
    const multiplierY = event.deltaMode === 1 ? linePixels : event.deltaMode === 2 ? pageY : 1;
    const multiplierX = event.deltaMode === 1 ? linePixels : event.deltaMode === 2 ? pageX : 1;
    let deltaLeft = event.deltaX * multiplierX;
    let deltaTop = event.deltaY * multiplierY;
    if (event.shiftKey && deltaLeft === 0) {
      deltaLeft = deltaTop;
      deltaTop = 0;
    }
    queueSyncedWheel(source, deltaTop, deltaLeft);
    return;
  }
  event.preventDefault();
  const previous = zoom.value;
  const next = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Number((previous + (event.deltaY < 0 ? 0.1 : -0.1)).toFixed(1))));
  if (next === previous) return;
  const bounds = source.getBoundingClientRect();
  const pointerX = event.clientX - bounds.left;
  const pointerY = event.clientY - bounds.top;
  const ratio = next / previous;
  zoom.value = next;
  persistLayout();
  nextTick(() => {
    const nextLeft = Math.max(0, (source.scrollLeft + pointerX) * ratio - pointerX);
    const nextTop = Math.max(0, (source.scrollTop + pointerY) * ratio - pointerY);
    applySyncedScroll(source, nextTop, nextLeft);
  });
}

function resetZoom() {
  zoom.value = 1;
  persistLayout();
}

function beginColumnResize(col: number, event: PointerEvent) {
  event.preventDefault();
  event.stopPropagation();
  resizeState = {
    col,
    startX: event.clientX,
    startWidth: columnWidths.value[col] ?? BASE_COL_WIDTH
  };
}

function resizeColumn(event: PointerEvent) {
  if (!resizeState) return;
  const delta = (event.clientX - resizeState.startX) / zoom.value;
  const width = Math.round(Math.min(MAX_COL_WIDTH, Math.max(MIN_COL_WIDTH, resizeState.startWidth + delta)));
  columnWidths.value = { ...columnWidths.value, [resizeState.col]: width };
}

function beginRepositoryResize(event: PointerEvent) {
  if (!repository.value) return;
  event.preventDefault();
  repositoryResize = {
    startX: event.clientX,
    startWidth: repoSidebar.value?.getBoundingClientRect().width ?? repository.value.sidebarWidth
  };
}

function resizeRepositorySidebar(event: PointerEvent) {
  if (!repositoryResize || !repository.value) return;
  const width = Math.round(Math.min(520, Math.max(180, repositoryResize.startWidth + event.clientX - repositoryResize.startX)));
  repository.value = { ...repository.value, sidebarWidth: width };
}

function openCellMenu(event: MouseEvent, cell: RegionCell, side: "left" | "right") {
  event.preventDefault();
  const selected = containsCell(selection.value, cell.row, cell.col);
  const hasSelectedAction = selected && rowsForContext(cell.row).some((row) => rowStatusForSource(row) !== "unchanged");
  if ((cell.rowStatus || rowStatus(cell.row)) === "unchanged" && cell.status === "unchanged" && !hasSelectedAction) {
    contextMenu.value.visible = false;
    return;
  }
  if (!selected) {
    setSingleSelection(cell);
  }
  contextMenu.value = {
    visible: true,
    x: Math.max(8, Math.min(event.clientX, window.innerWidth - 368)),
    y: Math.max(8, Math.min(event.clientY, window.innerHeight - 300)),
    row: cell.row,
    col: cell.col,
    side,
    kind: "cell"
  };
}

function openRowMenu(event: MouseEvent, row: number, side: "left" | "right") {
  event.preventDefault();
  const selected = Boolean(selection.value && row >= selection.value.startRow && row <= selection.value.endRow);
  const hasSelectedAction = selected && rowsForContext(row).some((selectedRow) => rowStatusForSource(selectedRow) !== "unchanged");
  if (rowStatus(row) === "unchanged" && !hasSelectedAction) {
    contextMenu.value.visible = false;
    return;
  }
  if (!selected) {
    beginRowSelection(row, event);
  }
  contextMenu.value = {
    visible: true,
    x: Math.max(8, Math.min(event.clientX, window.innerWidth - 368)),
    y: Math.max(8, Math.min(event.clientY, window.innerHeight - 300)),
    row,
    col: 0,
    side,
    kind: "row"
  };
}

function openColumnMenu(event: MouseEvent, col: number, side: "left" | "right") {
  event.preventDefault();
  contextMenu.value = {
    visible: true,
    x: Math.max(8, Math.min(event.clientX, window.innerWidth - 260)),
    y: Math.max(8, Math.min(event.clientY, window.innerHeight - 160)),
    row: 0,
    col,
    side,
    kind: "column"
  };
}

function closeContextMenu(event: PointerEvent) {
  const target = event.target as HTMLElement | null;
  if (!target?.closest(".context-menu, .id-dialog")) contextMenu.value.visible = false;
}

async function finishRepositoryResize() {
  if (!repositoryResize || !repository.value) return;
  repositoryResize = null;
  await guard(() => backend.setRepositorySidebarWidth(repository.value!.sidebarWidth));
}

function columnName(col: number) {
  let value = col;
  let result = "";
  while (value > 0) {
    value--;
    result = String.fromCharCode(65 + (value % 26)) + result;
    value = Math.floor(value / 26);
  }
  return result;
}

onMounted(() => {
  window.addEventListener("pointermove", resizeColumn);
  window.addEventListener("pointermove", resizeRepositorySidebar);
  window.addEventListener("pointerup", finishPointerAction);
  window.addEventListener("pointerup", stopOriginalPreviewFromPointer);
  window.addEventListener("pointercancel", stopOriginalPreviewFromPointer);
  window.addEventListener("pointerdown", closeContextMenu);
  window.addEventListener("keydown", onWindowKeyDown);
  window.addEventListener("keyup", onWindowKeyUp);
  window.addEventListener("blur", stopOriginalPreview);
  window.addEventListener("focus", scheduleExternalCheck);
  document.addEventListener("visibilitychange", onVisibilityChange);
  externalCheckTimer = window.setInterval(() => void checkExternalChanges(), 2500);
  if ((window as Window & { runtime?: unknown }).runtime) {
    const stopDropStartedListener = EventsOn("repository-drop-started", () => {
      beginRepositoryOpen();
    });
    stopDropListener = EventsOn("repository-drop-result", (result: RepositoryResult | null, message: string) => {
      if (message) {
        repositoryOpenDialog.value = true;
        repositoryOpenError.value = message;
        finishRepositoryOpen();
      } else if (result) {
        void completeRepositoryDrop(result);
      }
    });
    const stopDropResultListener = stopDropListener;
    stopDropListener = () => {
      stopDropStartedListener();
      stopDropResultListener();
    };
  }
  void initializeApplication();
});

onBeforeUnmount(() => {
  window.clearTimeout(loadingTimer);
  window.clearTimeout(differenceIndexTimer);
  window.clearTimeout(externalFocusTimer);
  window.clearInterval(externalCheckTimer);
  window.cancelAnimationFrame(wheelFrame);
  window.removeEventListener("pointermove", resizeColumn);
  window.removeEventListener("pointermove", resizeRepositorySidebar);
  window.removeEventListener("pointerup", finishPointerAction);
  window.removeEventListener("pointerup", stopOriginalPreviewFromPointer);
  window.removeEventListener("pointercancel", stopOriginalPreviewFromPointer);
  window.removeEventListener("pointerdown", closeContextMenu);
  window.removeEventListener("keydown", onWindowKeyDown);
  window.removeEventListener("keyup", onWindowKeyUp);
  window.removeEventListener("blur", stopOriginalPreview);
  window.removeEventListener("focus", scheduleExternalCheck);
  document.removeEventListener("visibilitychange", onVisibilityChange);
  stopDropListener?.();
});
</script>

<template>
  <div class="app-shell" :aria-busy="startupLoading">
    <header class="toolbar">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true"><img src="/appicon.svg" alt="" /></span>
        <span class="brand-copy">
          <strong>SheetProof</strong>
          <small :title="currentTaskName">{{ currentTaskName }}</small>
        </span>
      </div>

      <div class="toolbar-group file-actions" :aria-label="t('toolbar.sources')">
        <button class="secondary" :disabled="busy" @click="showRepositoryOpenDialog">
          <AppIcon name="repository" />
          <span class="button-label full-label">{{ t("toolbar.openRepository") }}</span>
          <span class="button-label compact-label" aria-hidden="true">{{ t("toolbar.openRepositoryShort") }}</span>
        </button>
        <button class="ghost" :disabled="busy" @click="chooseFiles">
          <AppIcon name="files" />
          <span class="button-label full-label">{{ t("toolbar.openFiles") }}</span>
          <span class="button-label compact-label" aria-hidden="true">{{ t("toolbar.openFilesShort") }}</span>
        </button>
      </div>

      <div class="toolbar-group diff-navigation" :aria-label="t('toolbar.differenceNavigation')">
        <button
          class="icon-button"
          :disabled="!filteredDiffEntries.length || busy || !comparisonActive"
          :title="t('toolbar.previousDifference')"
          :aria-label="t('toolbar.previousDifference')"
          @click="navigate(-1)"
        ><AppIcon name="chevron-left" /><span class="button-label">{{ t("toolbar.previous") }}</span></button>
        <span class="counter" aria-live="polite">
          {{ filteredDiffPosition >= 0 ? filteredDiffPosition + 1 : 0 }} / {{ filteredDiffEntries.length }}
        </span>
        <button
          class="icon-button"
          :disabled="!filteredDiffEntries.length || busy || !comparisonActive"
          :title="t('toolbar.nextDifference')"
          :aria-label="t('toolbar.nextDifference')"
          @click="navigate(1)"
        ><span class="button-label">{{ t("toolbar.next") }}</span><AppIcon name="chevron-right" /></button>
      </div>

      <span class="grow"></span>

      <div class="toolbar-group edit-actions" :aria-label="t('toolbar.editAndMerge')">
        <button
          :disabled="!copyTargets.length || busy || summary?.options.readonlyLeft || !comparisonActive"
          class="primary merge-action"
          :title="t('toolbar.copyCellsToLeft', { count: copyTargets.length })"
          @click="copySelection"
        >
          <AppIcon name="merge" />
          <span class="full-label">{{ t("toolbar.copyCellsToLeft", { count: copyTargets.length }) }}</span>
          <span class="compact-label" aria-hidden="true">{{ t("toolbar.copyCellsToLeftShort", { count: copyTargets.length }) }}</span>
        </button>
        <button
          class="icon-button"
          :disabled="!summary?.undoCount || busy"
          :title="t('toolbar.undoShortcut')"
          :aria-label="t('toolbar.undo')"
          @click="undo"
        ><AppIcon name="undo" /><span class="button-label">{{ t("toolbar.undo") }}</span></button>
        <button
          class="zoom-button ghost"
          :title="t('toolbar.zoomHelp', { percent: Math.round(zoom * 100) })"
          @click="resetZoom"
        ><AppIcon name="zoom" /><span class="full-label">{{ t("toolbar.zoom", { percent: Math.round(zoom * 100) }) }}</span><span class="compact-label" aria-hidden="true">{{ t("toolbar.zoomShort", { percent: Math.round(zoom * 100) }) }}</span></button>
      </div>

      <div class="toolbar-group save-actions" :aria-label="t('toolbar.saveResults')">
        <button
          class="icon-button ghost"
          :disabled="!summary || busy || summary.options.readonlyLeft"
          :title="repository ? t('toolbar.exportCopyShortcut') : t('toolbar.saveAsShortcut')"
          :aria-label="t('toolbar.saveAs')"
          @click="save(true)"
        ><AppIcon name="save-as" /><span class="button-label">{{ repository ? t("toolbar.exportCopy") : t("toolbar.saveAs") }}</span></button>
        <button
          :disabled="!summary?.dirty || busy || summary.options.readonlyLeft"
          class="save"
          :title="repository || summary?.options.ugitWorktree ? t('toolbar.saveWorktree') : t('toolbar.saveLeft')"
          @click="save(false)"
        >
          <AppIcon name="save" />
          <span class="full-label">{{ repository || summary?.options.ugitWorktree ? t("toolbar.saveWorktree") : t("toolbar.saveLeft") }}</span>
          <span class="compact-label" aria-hidden="true">{{ repository || summary?.options.ugitWorktree ? t("toolbar.saveWorktreeShort") : t("toolbar.saveLeftShort") }}</span>
        </button>
      </div>
      <div class="toolbar-group settings-actions" :aria-label="t('toolbar.settings')">
        <button
          class="icon-button ghost"
          :disabled="busy"
          :title="t('toolbar.settings')"
          :aria-label="t('toolbar.settings')"
          @click="openSettings"
        >
          <AppIcon name="settings" />
          <span class="button-label">{{ t("toolbar.settings") }}</span>
        </button>
      </div>
      <div v-if="busy" class="toolbar-progress" aria-hidden="true"></div>
    </header>

    <div v-if="error" class="error-banner" role="alert">
      <AppIcon name="alert" />
      <span>{{ error }}</span>
      <button class="icon-button" :title="t('common.closeError')" :aria-label="t('common.closeError')" @click="error = ''">
        <AppIcon name="x" />
      </button>
    </div>
    <div
      v-if="externalNotice"
      class="external-change-banner"
      :class="{ warning: !!deferredExternalChange }"
      :role="deferredExternalChange ? 'alert' : 'status'"
    >
      <AppIcon :name="deferredExternalChange ? 'alert' : 'check'" />
      <span>{{ externalNotice }}</span>
      <button
        v-if="deferredExternalChange"
        class="compact-button"
        :disabled="busy"
        @click="reloadExternal(deferredExternalChange.side)"
      >{{ t("externalChange.reload") }}</button>
      <button
        v-else
        class="icon-button"
        :title="t('common.closeNotice')"
        :aria-label="t('common.closeNotice')"
        @click="externalNotice = ''"
      ><AppIcon name="x" /></button>
    </div>
    <div v-if="repository" class="repository-bar">
      <div class="repository-identity">
        <strong><AppIcon name="repository" />{{ repository.name }}</strong>
        <span :title="repository.path">{{ repository.path }}</span>
      </div>
      <span class="branch-status" :class="{ warning: repository.detached }">
        <AppIcon name="branch" :size="13" />
        {{ repository.detached ? t("repository.detachedHead", { branch: repository.currentBranch }) : t("repository.currentBranch", { branch: repository.currentBranch }) }}
      </span>
      <span v-if="repository.workspaceDirty" class="working-status"><span class="status-dot"></span>{{ t("repository.uncommittedChanges") }}</span>
      <span v-if="repository.operation" class="operation-status"><span class="status-dot"></span>{{ t("repository.operation", { operation: repository.operation }) }}</span>
      <span class="grow"></span>
      <button class="ghost compact-button" :disabled="busy" @click="showRepositorySwitcher">{{ t("repository.switch") }}</button>
    </div>
    <div v-if="repository?.notice" class="notice-banner"><AppIcon name="info" />{{ repository.notice }}</div>

    <main
      v-if="summary || repository"
      class="workspace"
      :class="{ 'repository-workspace': !!repository }"
      :style="repository ? { gridTemplateColumns: `${repository.sidebarWidth}px 1fr` } : undefined"
    >
      <aside v-if="repository" ref="repoSidebar" class="repository-sidebar">
        <div class="repository-sidebar-tabs" role="tablist" :aria-label="t('repository.sidebar')">
          <button
            role="tab"
            :aria-selected="repositorySidebarTab === 'files'"
            :class="{ active: repositorySidebarTab === 'files' }"
            @click="repositorySidebarTab = 'files'"
          ><AppIcon name="folder" :size="14" />{{ t("repository.files") }}</button>
          <button
            role="tab"
            :aria-selected="repositorySidebarTab === 'differences'"
            :class="{ active: repositorySidebarTab === 'differences' }"
            @click="repositorySidebarTab = 'differences'"
          >
            <AppIcon name="files" :size="14" />{{ t("repository.differingWorkbooks") }}
            <span class="sidebar-tab-count">
              {{ repository.differenceIndexing ? "…" : repository.differenceFiles.length }}
            </span>
          </button>
          <button
            role="tab"
            :aria-selected="repositorySidebarTab === 'sheets'"
            :class="{ active: repositorySidebarTab === 'sheets' }"
            @click="repositorySidebarTab = 'sheets'"
          >
            <AppIcon name="selection" :size="14" />{{ t("repository.sheetsAndDifferences") }}
            <span v-if="summary" class="sidebar-tab-count">{{ summary.diff.differenceCount }}</span>
          </button>
        </div>

        <div v-if="repositorySidebarTab !== 'sheets'" class="repository-files-pane" role="tabpanel">
          <div class="repository-tree-header">
            <strong>{{ repositorySidebarTab === "differences" ? t("repository.differingWorkbooks") : t("repository.files") }}</strong>
            <button
              class="icon-button ghost"
              :title="t('repository.refresh')"
              :aria-label="t('repository.refresh')"
              :disabled="busy"
              @click="refreshRepository"
            ><AppIcon name="refresh" /></button>
          </div>
          <div class="repository-search">
            <div class="repository-search-control">
              <AppIcon name="search" :size="14" />
              <input
                ref="repositorySearchInput"
                v-model="repositorySearch"
                type="text"
                autocomplete="off"
                :placeholder="repositorySidebarTab === 'differences' ? t('repository.searchDifferences') : t('repository.searchFiles')"
                :aria-label="repositorySidebarTab === 'differences' ? t('repository.searchDifferences') : t('repository.searchFilesAria')"
                @focus="repositorySearchFocused = true"
                @blur="finishRepositorySearch"
                @keydown.enter.prevent="endRepositorySearch"
                @keydown.esc.prevent="endRepositorySearch"
              />
              <button
                v-if="repositorySearch"
                type="button"
                class="repository-search-clear"
                :title="t('repository.clearSearch')"
                :aria-label="t('repository.clearSearch')"
                @mousedown.prevent
                @click="clearRepositorySearch"
              ><AppIcon name="x" :size="15" /></button>
            </div>
            <div
              v-if="repositorySearchFocused"
              class="repository-search-history"
              role="listbox"
              :aria-label="t('repository.recentSearches')"
            >
              <div class="repository-search-history-title">{{ t("repository.recentSearches") }}</div>
              <button
                v-for="query in repositorySearchHistory"
                :key="query"
                type="button"
                role="option"
                :aria-label="t('common.search', { query })"
                @mousedown.prevent
                @click="applyRepositorySearchHistory(query)"
              ><AppIcon name="search" :size="12" /><span>{{ query }}</span></button>
              <div v-if="!repositorySearchHistory.length" class="repository-search-history-empty">
                {{ t("repository.noRecentSearches") }}
              </div>
            </div>
          </div>
          <div
            v-if="repositorySidebarTab === 'differences' && !repository.differenceFiles.length"
            class="tree-empty"
          >
            {{
              repository.differenceIndexing
                ? t("repository.indexing")
                : repository.selectedRef
                  ? t("repository.noDifferentWorkbooks")
                  : t("repository.selectReference")
            }}
          </div>
          <div v-else-if="repositorySidebarTab === 'files' && !repository.files.length" class="tree-empty">
            {{ t("repository.noXlsx") }}
          </div>
          <div v-else-if="!repositoryRows.length" class="tree-empty">{{ t("repository.noMatchingFiles") }}</div>
          <div v-else class="repository-tree" role="tree">
            <button
              v-for="row in repositoryRows"
              :key="`${row.kind}:${row.path}`"
              class="tree-row"
              :class="{ selected: row.kind === 'file' && row.path === repository.selectedFile }"
              :style="{ paddingLeft: `${10 + row.depth * 17}px` }"
              :title="row.path"
              role="treeitem"
              :aria-expanded="row.kind === 'directory' ? expandedDirectories.has(row.path) : undefined"
              @click="row.kind === 'directory' ? toggleDirectory(row.path) : selectRepositoryFile(row.path)"
            >
              <span class="tree-icon">
                <template v-if="row.kind === 'directory'">
                  <AppIcon :name="expandedDirectories.has(row.path) ? 'folder-open' : 'folder'" :size="14" />
                </template>
                <template v-else><AppIcon name="file" :size="14" /></template>
              </span>
              <span class="tree-row-name">{{ row.name }}</span>
            </button>
          </div>
        </div>

        <div v-else class="repository-sheet-pane" role="tabpanel">
          <template v-if="summary">
            <div class="side-title">{{ t("repository.sheetsAndDifferences") }}</div>
            <button
              v-for="item in summary.diff.sheets"
              :key="item.name"
              class="sheet-item"
              :class="{ active: item.name === sheet }"
              @click="loadSheet(item.name)"
            >
              <span class="sheet-name">{{ item.name }}</span>
              <span class="badge" :class="item.status">{{ item.differenceCount }}</span>
              <small class="sheet-status-line">
                <span v-if="item.addedRowCount" class="row-status-chip added">{{ t("diff.rowCount", { category: t("diff.added"), count: item.addedRowCount }) }}</span>
                <span v-if="item.deletedRowCount" class="row-status-chip deleted">{{ t("diff.rowCount", { category: t("diff.deleted"), count: item.deletedRowCount }) }}</span>
                <span v-if="item.modifiedRowCount" class="row-status-chip modified">{{ t("diff.rowCount", { category: t("diff.modified"), count: item.modifiedRowCount }) }}</span>
                <span v-if="item.conflictRowCount" class="row-status-chip conflict">{{ t("diff.rowCount", { category: t("diff.conflict"), count: item.conflictRowCount }) }}</span>
                <span v-if="item.orderDifferent">{{ t("diff.orderDifferent") }}</span>
                <span v-if="!item.differenceCount">{{ t("common.noDifferences") }}</span>
              </small>
            </button>
            <div class="diff-index-title">{{ t("diff.index") }}</div>
            <div class="diff-filter-tabs" role="tablist" :aria-label="t('diff.indexCategories')">
              <button
                v-for="status in DIFF_FILTER_TABS"
                :key="status"
                role="tab"
                :aria-selected="diffFilter === status"
                :aria-pressed="selectedRowFilterSet.has(status)"
                :class="[status, { active: diffFilter === status, 'row-filter-active': selectedRowFilterSet.has(status) }]"
                :disabled="!diffFilterCounts[status]"
                @click="selectDiffIndexFilter(status)"
              >
                {{ rowStatusLabel(status) }}
                <span>{{ diffFilterCounts[status] }}</span>
              </button>
            </div>
            <div class="diff-list">
              <button
                v-for="{ item, index } in filteredDiffEntries"
                :key="`${item.ref.row}:${item.ref.col}`"
                :class="{ active: diffIndex === index }"
                @click="selectDiffEntry(index, item)"
              >
                <span class="diff-row-status" :class="item.rowStatus || 'modified'">
                  {{ rowStatusLabel(item.rowStatus || "modified") }}
                </span>
                {{ item.ref.row }}:{{ columnName(item.ref.col) }} · {{ rowStatusLabel(item.status) }}
              </button>
              <div v-if="!filteredDiffEntries.length" class="diff-list-empty">
                {{ t("diff.noneInCategory", { category: rowStatusLabel(diffFilter) }) }}
              </div>
            </div>
          </template>
          <div v-else class="tree-empty">{{ t("repository.selectWorkbookFirst") }}</div>
        </div>
        <div class="repository-resizer" @pointerdown="beginRepositoryResize"></div>
      </aside>

      <aside v-else class="sidebar">
        <div class="side-title">{{ t("repository.sheetsAndDifferences") }}</div>
        <button
          v-for="item in summary?.diff.sheets ?? []"
          :key="item.name"
          class="sheet-item"
          :class="{ active: item.name === sheet }"
          @click="loadSheet(item.name)"
        >
          <span class="sheet-name">{{ item.name }}</span>
          <span class="badge" :class="item.status">{{ item.differenceCount }}</span>
          <small class="sheet-status-line">
            <span v-if="item.addedRowCount" class="row-status-chip added">{{ t("diff.rowCount", { category: t("diff.added"), count: item.addedRowCount }) }}</span>
            <span v-if="item.deletedRowCount" class="row-status-chip deleted">{{ t("diff.rowCount", { category: t("diff.deleted"), count: item.deletedRowCount }) }}</span>
            <span v-if="item.modifiedRowCount" class="row-status-chip modified">{{ t("diff.rowCount", { category: t("diff.modified"), count: item.modifiedRowCount }) }}</span>
            <span v-if="item.conflictRowCount" class="row-status-chip conflict">{{ t("diff.rowCount", { category: t("diff.conflict"), count: item.conflictRowCount }) }}</span>
            <span v-if="item.orderDifferent">{{ t("diff.orderDifferent") }}</span>
            <span v-if="!item.differenceCount">{{ t("common.noDifferences") }}</span>
          </small>
        </button>
        <div class="diff-index-title">{{ t("diff.index") }}</div>
        <div class="diff-filter-tabs" role="tablist" :aria-label="t('diff.indexCategories')">
          <button
            v-for="status in DIFF_FILTER_TABS"
            :key="status"
            role="tab"
            :aria-selected="diffFilter === status"
            :aria-pressed="selectedRowFilterSet.has(status)"
            :class="[status, { active: diffFilter === status, 'row-filter-active': selectedRowFilterSet.has(status) }]"
            :disabled="!diffFilterCounts[status]"
            @click="selectDiffIndexFilter(status)"
          >
            {{ rowStatusLabel(status) }}
            <span>{{ diffFilterCounts[status] }}</span>
          </button>
        </div>
        <div class="diff-list">
          <button
            v-for="{ item, index } in filteredDiffEntries"
            :key="`${item.ref.row}:${item.ref.col}`"
            :class="{ active: diffIndex === index }"
            @click="selectDiffEntry(index, item)"
          >
            <span class="diff-row-status" :class="item.rowStatus || 'modified'">
              {{ rowStatusLabel(item.rowStatus || "modified") }}
            </span>
            {{ item.ref.row }}:{{ columnName(item.ref.col) }} · {{ rowStatusLabel(item.status) }}
          </button>
          <div v-if="!filteredDiffEntries.length" class="diff-list-empty">
            {{ t("diff.noneInCategory", { category: rowStatusLabel(diffFilter) }) }}
          </div>
        </div>
      </aside>

      <section class="content">
        <div v-if="repository" class="file-strip repository-file-strip">
          <div class="source-card editable-source">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon name="edit" /></span>
              <span>
                <small>{{ t("source.originalEditable") }}</small>
                <strong>{{ repository.detached ? t("repository.detachedHead", { branch: repository.currentBranch }) : repository.currentBranch }}</strong>
              </span>
              <span class="source-state" :class="{ warning: repository.fileModified || summary?.dirty }">
                <AppIcon
                  :name="!repository.selectedFile ? 'info' : repository.fileModified || summary?.dirty ? 'alert' : 'check'"
                  :size="12"
                />
                {{ !repository.selectedFile ? t("common.select") : repository.fileModified || summary?.dirty ? t("status.unsaved") : t("common.ready") }}
              </span>
            </div>
            <span class="source-path" :title="repository.selectedFile">{{ repository.selectedFile || t("repository.selectWorkbook") }}</span>
            <small v-if="repository.fileModified">{{ t("repository.gitFileModified") }}</small>
            <small v-if="summary?.dirty">{{ t("repository.sessionModified") }}</small>
          </div>
          <div class="source-card readonly-source reference-header">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon name="branch" /></span>
              <span>
                <small>{{ t("source.targetReadOnly") }}</small>
                <strong>{{ t("repository.comparisonWorkbook") }}</strong>
              </span>
              <span class="source-state">
                <AppIcon name="check" :size="12" />
                {{ comparisonActive ? t("common.ready") : t("common.select") }}
              </span>
            </div>
            <select
              :value="repository.selectedRef"
              :disabled="busy || !repository.branches.length"
              :aria-label="t('repository.chooseReference')"
              @change="selectRepositoryRef(($event.target as HTMLSelectElement).value)"
            >
              <option value="" disabled>{{ t("repository.selectReference") }}</option>
              <optgroup :label="t('repository.localBranches')">
                <option
                  v-for="branch in repository.branches.filter((item) => item.kind === 'local')"
                  :key="branch.fullName"
                  :value="branch.fullName"
                >{{ branch.name }}</option>
              </optgroup>
              <optgroup :label="t('repository.remoteBranches')">
                <option
                  v-for="branch in repository.branches.filter((item) => item.kind === 'remote')"
                  :key="branch.fullName"
                  :value="branch.fullName"
                >{{ branch.name }}</option>
              </optgroup>
            </select>
            <span class="source-path" :title="repository.selectedFile">{{ repository.selectedFile || t("repository.selectWorkbook") }}</span>
          </div>
        </div>
        <div v-else-if="summary" class="file-strip">
          <div class="source-card" :class="leftReadonly ? 'readonly-source' : 'editable-source'">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon :name="leftReadonly ? 'files' : 'edit'" /></span>
              <span>
                <small>{{ summary.options.ugitWorktree ? t("source.originalEditable") : summary.options.gitDiff ? t("source.gitDiffReadOnly") : summary.options.gitMerge ? t("source.gitMergeEditable") : leftReadonly ? t("source.originalReadOnly") : t("source.originalEditable") }}</small>
                <strong>{{ summary.options.leftLabel }}</strong>
              </span>
              <span class="source-state"><AppIcon name="check" :size="12" />{{ leftReadonly ? t("common.readOnly") : t("source.parsed") }}</span>
            </div>
            <span class="source-path" :title="summary.options.gitDiff ? t('source.gitTemporary') : summary.diff.leftFile">
              {{ sourcePathLabel(summary.diff.leftFile, summary.options.gitDiff) }}
            </span>
          </div>
          <div class="source-card readonly-source">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon name="files" /></span>
              <span><small>{{ summary.options.ugitWorktree ? t("source.gitVersionReadOnly") : summary.options.gitDiff ? t("source.gitDiffReadOnly") : summary.options.gitMerge ? t("source.gitMergeReadOnly") : t("source.targetReadOnly") }}</small><strong>{{ summary.options.rightLabel }}</strong></span>
              <span class="source-state"><AppIcon name="check" :size="12" />{{ t("common.readOnly") }}</span>
            </div>
            <span class="source-path" :title="summary.options.gitDiff || summary.options.ugitWorktree ? t('source.gitTemporary') : summary.diff.rightFile">
              {{ sourcePathLabel(summary.diff.rightFile, summary.options.gitDiff || summary.options.ugitWorktree) }}
            </span>
          </div>
        </div>

        <div v-if="summary" class="comparison-summary-stack">
          <div class="result-summary" :aria-label="t('diff.summary')">
            <div class="result-summary-title">
              <AppIcon :name="currentSheetDifferenceCount === 0 ? 'check' : 'selection'" />
              <span>
                <small>{{ t("diff.summary") }}</small>
                <strong>{{ currentSheetDifferenceCount === 0 ? t("diff.currentSheetEqual") : t("diff.count", { count: currentSheetDifferenceCount }) }}</strong>
              </span>
            </div>
            <button
              v-for="(status, index) in DIFF_FILTER_TABS"
              :key="status"
              type="button"
              class="summary-metric"
              :class="[status, { active: selectedRowFilterSet.has(status) }]"
              :aria-pressed="selectedRowFilterSet.has(status)"
              :title="t('diff.toggleFilter', { action: selectedRowFilterSet.has(status) ? t('diff.disable') : t('diff.enable'), category: rowStatusLabel(status), shortcut: index + 1 })"
              @click="toggleRowFilter(status)"
            ><i></i>{{ rowStatusLabel(status) }} <strong>{{ resultMetrics[status] }}</strong><kbd>{{ index + 1 }}</kbd></button>
            <span class="summary-context">
              {{ t("diff.rowFilter", { filter: rowFilterSummary }) }} <kbd>5</kbd>
              <template v-if="selectionSize > 0"> · {{ t("diff.selectedCells", { count: selectionSize }) }}</template>
            </span>
            <button
              v-if="activeRowAlignment?.available && !summary.options.gitMerge"
              type="button"
              class="alignment-toggle"
              :class="{ active: summary.rowAlignment.mode === 'auto' }"
              :aria-pressed="summary.rowAlignment.mode === 'auto'"
              :disabled="summary.undoCount > 0"
              :title="summary.undoCount > 0
                ? t('alignment.locked')
                : summary.rowAlignment.mode === 'auto'
                  ? t('alignment.usePhysical')
                  : t('alignment.useKey')"
              @click="toggleRowAlignment"
            >{{ summary.rowAlignment.mode === "auto" ? t("alignment.key", { count: activeRowAlignment.moved > 0 ? ` ${activeRowAlignment.moved}` : "" }) : t("alignment.physicalRows") }}</button>
          </div>

          <div
            v-if="summary.options.gitMerge && summary.mergeNotice"
            class="merge-semantic-notice"
            role="status"
            aria-live="polite"
          >
            <AppIcon name="alert" :size="15" />
            <span>{{ summary.mergeNotice }}</span>
          </div>
        </div>

        <div class="grids">
          <section class="grid-panel" :class="{ 'original-preview': previewOriginal }">
            <div class="panel-heading">
              <strong>
                <span class="panel-indicator" :class="leftReadonly ? 'readonly' : 'editable'"></span>
                {{ repository || summary?.options.ugitWorktree ? t("repository.worktreeWorkbook") : summary?.options.gitDiff ? t("source.originalSnapshot") : t("source.originalEditable") }}
              </strong>
              <span class="panel-permission" :class="leftReadonly ? 'readonly' : 'editable'">
                <AppIcon :name="leftReadonly ? 'files' : 'edit'" :size="12" />{{ leftReadonly ? t("common.readOnly") : t("common.editable") }}
              </span>
              <small v-if="previewOriginal" class="grid-edit-hint">{{ t("grid.previewing") }}</small>
              <small v-else-if="summary?.options.gitDiff" class="grid-edit-hint">{{ t("grid.gitReadOnlyHint") }}</small>
              <small v-else-if="summary && !summary.options.readonlyLeft" class="grid-edit-hint">{{ t("grid.editHint") }}</small>
              <button
                v-if="!leftReadonly"
                class="original-preview-button"
                :class="{ active: previewOriginal }"
                :disabled="!canPreviewOriginal"
                :aria-pressed="previewOriginal"
                :title="canPreviewOriginal ? t('grid.previewHelp') : t('grid.previewUnavailable')"
                @pointerdown="startOriginalPreviewFromPointer"
                @keydown.space.prevent="startOriginalPreviewFromKeyboardButton"
                @keyup.space.prevent="stopOriginalPreviewFromPointer"
                @keydown.enter.prevent="startOriginalPreviewFromKeyboardButton"
                @keyup.enter.prevent="stopOriginalPreviewFromPointer"
              ><AppIcon name="undo" :size="12" />{{ previewOriginal ? t("grid.previewLatest") : t("grid.previewBeforeAfter") }}<kbd>Tab</kbd></button>
            </div>
            <div v-if="repository && repository.leftState !== 'ready'" class="panel-empty">
              <template v-if="repository.leftState === 'loading' || repository.leftState === 'comparing'">
                <div class="loading-spinner small"></div>
                <strong class="panel-loading-label">{{ repositoryStateMessage("left") }}</strong>
              </template>
              <EmptyState
                v-else
                icon="file"
                :title="repositoryStateMessage('left')"
                :description="t('grid.selectWorkbookHelp')"
              />
            </div>
            <div v-else ref="leftScroll" class="grid-scroll" :style="scrollbarMarkerStyle" tabindex="-1" @pointerdown.capture="focusGrid" @scroll="onScroll('left')" @wheel="onGridWheel">
              <div class="canvas" :style="{ width: `${canvasWidth}px`, height: `${totalRows * rowHeight + rowHeight}px` }">
                <template v-if="region">
                  <div class="col-header-layer">
                    <div
                      v-for="col in visibleColumns"
                      :key="`lh${col}`"
                      class="col-header"
                      :class="{ 'key-column': activeSheet?.idColumn === col }"
                      :style="{
                        top: 0,
                        left: `${columnLeft(col)}px`,
                        width: `${columnWidth(col)}px`,
                        height: `${rowHeight}px`,
                        fontSize: `${scaledFontSize}px`
                      }"
                      @contextmenu="openColumnMenu($event, col, 'left')"
                    >
                      {{ columnName(col) }}<span v-if="activeSheet?.idColumn === col" class="key-column-badge">{{ t("alignment.keyColumn") }}</span>
                      <span class="col-resizer" @pointerdown="beginColumnResize(col, $event)"></span>
                    </div>
                  </div>
                  <div class="row-header-layer" :style="{ width: `${rowHeaderWidth}px` }">
                    <div
                      v-for="row in visibleRows"
                      :key="`lr${row}`"
                      :class="rowClass(row, 'left')"
                      :style="{
                        top: `${row * rowHeight}px`,
                        left: 0,
                        width: `${rowHeaderWidth}px`,
                        height: `${rowHeight}px`,
                        fontSize: `${scaledFontSize}px`
                      }"
                      @pointerdown="beginRowSelection(row, $event)"
                      @pointerenter="extendRowSelection(row)"
                      @contextmenu="openRowMenu($event, row, 'left')"
                    >
                      {{ rowLabel(row, "left") }}
                    </div>
                  </div>
                  <div
                    v-if="rowFilterActive && !totalRows"
                    class="filtered-grid-empty"
                  >{{ t("diff.emptyFilter") }}</div>
                  <div
                    v-for="cell in visibleCells"
                    :key="`l${cell.row}:${cell.col}`"
                    :class="cellClass(cell, 'left')"
                    :style="{
                      top: `${cell.row * rowHeight}px`,
                      left: `${columnLeft(cell.col)}px`,
                      width: `${columnWidth(cell.col)}px`,
                      height: `${rowHeight}px`,
                      fontSize: `${scaledFontSize}px`
                    }"
                    :title="displayedCellValue(cell, 'left').formula ? `=${displayedCellValue(cell, 'left').formula}` : displayedCellValue(cell, 'left').raw"
                    @pointerdown="beginCellSelection(cell, $event)"
                    @pointerenter="extendCellSelection(cell)"
                    @contextmenu="openCellMenu($event, cell, 'left')"
                    @dblclick.stop="startInlineEdit(cell)"
                  >
                    <input
                      v-if="inlineEditing(cell)"
                      v-model="inlineEdit!.value"
                      class="inline-cell-editor"
                      :aria-label="t('grid.editLeftCell')"
                      @pointerdown.stop
                      @dblclick.stop
                      @keydown.enter.prevent.stop="commitInlineEdit"
                      @keydown.esc.prevent.stop="cancelInlineEdit"
                      @blur="commitInlineEdit"
                    />
                    <template v-else>{{ displayedCellText(cell, "left") }}</template>
                  </div>
                </template>
              </div>
            </div>
          </section>

          <section class="grid-panel">
            <div class="panel-heading">
              <strong><span class="panel-indicator readonly"></span>{{ repository ? t("repository.comparisonWorkbook") : t("source.targetReadOnly") }}</strong>
              <span class="panel-permission readonly"><AppIcon name="branch" :size="12" />{{ t("common.readOnly") }}</span>
            </div>
            <div v-if="repository && repository.rightState !== 'ready'" class="panel-empty">
              <template v-if="repository.rightState === 'loading' || repository.rightState === 'comparing'">
                <div class="loading-spinner small"></div>
                <strong class="panel-loading-label">{{ repositoryStateMessage("right") }}</strong>
              </template>
              <EmptyState
                v-else
                :icon="repository.rightState === 'missing' ? 'alert' : 'branch'"
                :title="repositoryStateMessage('right')"
                :description="t('repository.loadReferenceHelp')"
              />
            </div>
            <div v-else ref="rightScroll" class="grid-scroll" :style="scrollbarMarkerStyle" tabindex="-1" @pointerdown.capture="focusGrid" @scroll="onScroll('right')" @wheel="onGridWheel">
              <div class="canvas" :style="{ width: `${canvasWidth}px`, height: `${totalRows * rowHeight + rowHeight}px` }">
                <template v-if="region">
                  <div class="col-header-layer">
                    <div
                      v-for="col in visibleColumns"
                      :key="`rh${col}`"
                      class="col-header"
                      :class="{ 'key-column': activeSheet?.idColumn === col }"
                      :style="{
                        top: 0,
                        left: `${columnLeft(col)}px`,
                        width: `${columnWidth(col)}px`,
                        height: `${rowHeight}px`,
                        fontSize: `${scaledFontSize}px`
                      }"
                      @contextmenu="openColumnMenu($event, col, 'right')"
                    >
                      {{ columnName(col) }}<span v-if="activeSheet?.idColumn === col" class="key-column-badge">{{ t("alignment.keyColumn") }}</span>
                      <span class="col-resizer" @pointerdown="beginColumnResize(col, $event)"></span>
                    </div>
                  </div>
                  <div class="row-header-layer" :style="{ width: `${rowHeaderWidth}px` }">
                    <div
                      v-for="row in visibleRows"
                      :key="`rr${row}`"
                      :class="rowClass(row, 'right')"
                      :style="{
                        top: `${row * rowHeight}px`,
                        left: 0,
                        width: `${rowHeaderWidth}px`,
                        height: `${rowHeight}px`,
                        fontSize: `${scaledFontSize}px`
                      }"
                      @pointerdown="beginRowSelection(row, $event)"
                      @pointerenter="extendRowSelection(row)"
                      @contextmenu="openRowMenu($event, row, 'right')"
                    >
                      {{ rowLabel(row, "right") }}
                      <span
                        v-if="rowResolution(row, 'right')"
                        class="resolution-marker"
                        :class="rowResolution(row, 'right')!.kind"
                        :title="resolutionLabel(rowResolution(row, 'right')!)"
                      >{{ resolutionLabel(rowResolution(row, "right")!) }}</span>
                    </div>
                  </div>
                  <div
                    v-if="rowFilterActive && !totalRows"
                    class="filtered-grid-empty"
                  >{{ t("diff.emptyFilter") }}</div>
                  <div
                    v-for="cell in visibleCells"
                    :key="`r${cell.row}:${cell.col}`"
                    :class="cellClass(cell, 'right')"
                    :style="{
                      top: `${cell.row * rowHeight}px`,
                      left: `${columnLeft(cell.col)}px`,
                      width: `${columnWidth(cell.col)}px`,
                      height: `${rowHeight}px`,
                      fontSize: `${scaledFontSize}px`
                    }"
                    :title="cell.right.formula ? `=${cell.right.formula}` : cell.right.raw"
                    @pointerdown="beginCellSelection(cell, $event)"
                    @pointerenter="extendCellSelection(cell)"
                    @contextmenu="openCellMenu($event, cell, 'right')"
                  >{{ cell.right.formula ? `=${cell.right.formula}` : cell.right.display }}</div>
                </template>
              </div>
            </div>
          </section>
        </div>

        <div v-if="summary && activeCell && selectionSize === 1" class="difference-inspector">
          <div class="difference-line left">
            <span class="difference-side">{{ previewOriginal ? t("grid.beforeState") : t("grid.currentWorktree") }}</span>
            <strong>{{ cellAxis(activeCell, "left") }}</strong>
            <span class="difference-value" :title="displayedCellValue(activeCell, 'left').raw">
              {{ displayDifferenceValue(activeCell, "left") }}
            </span>
            <span class="difference-type">{{ displayedCellValue(activeCell, "left").type || "unset" }}</span>
          </div>
          <div class="difference-line right">
            <span class="difference-side">{{ t("grid.comparisonSource") }}</span>
            <strong>{{ cellAxis(activeCell, "right") }}</strong>
            <span class="difference-value" :title="activeCell.right.raw">
              {{ displayDifferenceValue(activeCell, "right") }}
            </span>
            <span class="difference-type">{{ activeCell.right.type || "unset" }}</span>
            <span class="difference-status" :class="{ changed: activeCell.status !== 'unchanged' }">
              {{ rowStatusLabel(activeCell.status) }}
            </span>
          </div>
        </div>
        <div v-else class="difference-inspector empty">
          {{ selectionSize > 1 ? t("grid.multiSelectionHelp", { count: selectionSize }) : t("grid.selectionHelp") }}
        </div>
      </section>
    </main>

    <main v-else class="welcome">
      <div class="welcome-card">
        <div class="welcome-product">
          <div class="logo"><img src="/appicon.svg" alt="" /></div>
          <div>
            <span class="welcome-kicker">SHEETPROOF DESKTOP</span>
            <strong>{{ t("app.title") }}</strong>
          </div>
        </div>
        <h1>{{ t("welcome.title") }}</h1>
        <p>{{ t("welcome.description") }}</p>

        <div class="workflow-steps" :aria-label="t('welcome.workflow')">
          <span><i>1</i>{{ t("welcome.stepImport") }}</span>
          <span><i>2</i>{{ t("welcome.stepReview") }}</span>
          <span><i>3</i>{{ t("welcome.stepApply") }}</span>
          <span><i>4</i>{{ t("welcome.stepSave") }}</span>
        </div>

        <button class="primary large" :disabled="busy" @click="chooseRepository">
          <AppIcon name="repository" />{{ t("toolbar.openRepository") }}
        </button>
        <div class="drop-hint">
          <AppIcon name="folder" />
          <span><strong>{{ t("welcome.drop") }}</strong><small>{{ t("welcome.dropDetail") }}</small></span>
        </div>
        <div class="welcome-divider"><span>{{ t("welcome.directMode") }}</span></div>
        <button class="secondary-entry" :disabled="busy" @click="chooseFiles">
          <AppIcon name="files" />{{ t("welcome.compareFiles") }}
        </button>
        <small>{{ t("welcome.boundary") }}</small>
      </div>
    </main>
    <div
      v-if="contextMenu.visible"
      class="context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @pointerdown.stop
    >
      <template v-if="contextMenu.kind === 'column'">
        <button
          :disabled="busy || Boolean(summary?.options.gitMerge) || Boolean(summary?.undoCount)"
          @click="setContextKeyColumn"
        >{{ activeSheet?.idColumn === contextMenu.col ? t("alignment.clearKey") : t("alignment.setKey", { column: columnName(contextMenu.col) }) }}</button>
        <span v-if="summary?.undoCount" class="context-menu-warning">{{ t("alignment.historyLocked") }}</span>
        <span v-else>{{ t("alignment.help") }}</span>
      </template>
      <template v-else-if="contextHasMixedConflict">
        <span class="context-menu-warning">{{ t("context.mixedConflict") }}</span>
      </template>
      <template v-else-if="contextIsConflict">
        <button
          v-if="contextMenu.kind === 'cell'"
          :disabled="!copyTargets.length || copyTargets.length > 10000 || contextActionDisabled"
          @click="copyContextCells"
        >{{ t("context.overwriteCells") }}</button>
        <button :disabled="contextActionDisabled" @click="copyContextRows">{{ t("context.overwriteRows") }}</button>
        <button
          v-if="automaticIDLabel"
          :disabled="contextActionDisabled"
          @click="appendContextRowsAutomatically"
        >{{ t("context.appendAuto", { range: automaticIDLabel }) }}</button>
        <button
          v-if="!contextHasNumericIDColumn && contextAppendableRows.length"
          :disabled="contextActionDisabled"
          @click="appendContextRowsWithoutNumericID"
        >{{ t("context.appendRow") }}</button>
        <button
          v-if="contextHasNumericIDColumn"
          :disabled="contextActionDisabled"
          @click="openSpecifiedIDDialog"
        >{{ t("context.appendSpecified") }}</button>
      </template>
      <template v-else-if="contextIsActionable">
        <button
          v-if="contextMenu.kind === 'cell'"
          :disabled="!copyTargets.length || copyTargets.length > 10000 || contextActionDisabled"
          @click="copyContextCells"
        >{{ t("context.copyCells") }}</button>
        <button :disabled="contextActionDisabled" @click="copyContextRows">{{ t("context.copyRows") }}</button>
      </template>
      <span v-if="contextMenu.kind !== 'column'">
        {{ t("context.selectedRows", { count: contextRows.length, row: contextMenu.row }) }}
        · {{ contextStatusLabel }}
      </span>
    </div>
    <div v-if="idDialog.visible" class="id-dialog-overlay" @pointerdown.self="idDialog.visible = false">
      <form class="id-dialog" @submit.prevent="confirmSpecifiedIDs">
        <strong>{{ t("dialog.idsTitle") }}</strong>
        <span>{{ t("dialog.idsDescription") }}</span>
        <label v-for="(row, index) in idDialog.rows" :key="row">
          <span>{{ t("dialog.rightRow", { row }) }}</span>
          <input
            v-model="idDialog.values[index]"
            :aria-label="t('dialog.rightRowId', { row })"
            :placeholder="t('dialog.idPlaceholder')"
            autocomplete="off"
          />
        </label>
        <div class="id-dialog-actions">
          <button type="button" @click="idDialog.visible = false">{{ t("common.cancel") }}</button>
          <button
            type="submit"
            class="primary"
            :disabled="busy || idDialog.values.some((value) => !value.trim())"
          >{{ t("dialog.appendLeft") }}</button>
        </div>
      </form>
    </div>
    <div v-if="settingsDialog" class="settings-overlay" @pointerdown.self="closeSettings">
      <section class="settings-dialog" role="dialog" aria-modal="true" aria-labelledby="settings-title">
        <div class="repository-switch-header">
          <div>
            <strong id="settings-title">{{ t("settings.title") }}</strong>
            <span>{{ t("settings.description") }}</span>
          </div>
          <button
            class="icon-button"
            :title="t('common.close')"
            :aria-label="t('common.close')"
            :disabled="busy"
            @click="closeSettings"
          ><AppIcon name="x" /></button>
        </div>
        <div class="settings-content">
          <section class="settings-section">
            <div class="settings-section-heading">
              <strong>{{ t("settings.languageTitle") }}</strong>
              <span>{{ t("settings.languageDescription") }}</span>
            </div>
            <label class="settings-language-row">
              <span>{{ t("language.label") }}</span>
              <select :value="preference" :aria-label="t('language.label')" :disabled="busy" @change="changeLanguage">
                <option value="system">{{ t("language.followSystem") }}</option>
                <option value="en">English</option>
                <option value="zh-CN">简体中文</option>
                <option value="ja">日本語</option>
              </select>
            </label>
          </section>
          <section class="settings-section">
            <div class="settings-section-heading">
              <strong>{{ t("settings.integrationTitle") }}</strong>
              <span>{{ t("settings.integrationDescription") }}</span>
            </div>
            <div class="settings-action-row">
              <button
                class="secondary"
                :disabled="busy"
                :title="t('toolbar.configureUGitHelp')"
                :aria-label="t('toolbar.configureUGit')"
                @click="configureUGit"
              >
                <AppIcon name="settings" />{{ t("toolbar.configureUGit") }}
              </button>
            </div>
          </section>
          <div
            v-if="settingsNotice"
            class="settings-result"
            :class="settingsNotice.kind"
            :role="settingsNotice.kind === 'error' ? 'alert' : 'status'"
          >
            <AppIcon :name="settingsNotice.kind === 'error' ? 'alert' : settingsNotice.kind === 'success' ? 'check' : 'info'" />
            <span>{{ settingsNotice.message }}</span>
          </div>
          <section class="settings-section">
            <div class="settings-section-heading">
              <strong>{{ t("settings.storageTitle") }}</strong>
              <span>{{ t("settings.storageDescription") }}</span>
            </div>
            <div class="settings-storage-actions">
              <div>
                <span>{{ t("settings.clearCacheDescription") }}</span>
                <button :disabled="busy" @click="settingsConfirmation = 'cache'">{{ t("settings.clearCache") }}</button>
              </div>
              <div>
                <span>{{ t("settings.clearAllDescription") }}</span>
                <button class="danger" :disabled="busy" @click="settingsConfirmation = 'all'">{{ t("settings.clearAll") }}</button>
              </div>
            </div>
          </section>
        </div>
        <div class="repository-switch-actions">
          <button :disabled="busy" @click="closeSettings">{{ t("common.close") }}</button>
        </div>
      </section>
    </div>
    <div v-if="settingsConfirmation" class="settings-confirm-overlay" @pointerdown.self="settingsConfirmation = null">
      <section class="settings-confirm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="settings-confirm-title">
        <span class="external-change-icon"><AppIcon name="alert" :size="20" /></span>
        <div>
          <strong id="settings-confirm-title">{{ settingsConfirmation === "cache" ? t("settings.clearCacheConfirmTitle") : t("settings.clearAllConfirmTitle") }}</strong>
          <p>{{ settingsConfirmation === "cache" ? t("settings.clearCacheConfirmDescription") : t("settings.clearAllConfirmDescription") }}</p>
        </div>
        <div class="settings-confirm-actions">
          <button :disabled="busy" @click="settingsConfirmation = null">{{ t("common.cancel") }}</button>
          <button class="danger" :disabled="busy" @click="confirmSettingsCleanup">
            {{ settingsConfirmation === "cache" ? t("settings.confirmClearCache") : t("settings.confirmClearAll") }}
          </button>
        </div>
      </section>
    </div>
    <div
      v-if="repositorySwitchDialog.visible"
      class="repository-switch-overlay"
      @pointerdown.self="repositorySwitchDialog.visible = false"
    >
      <section class="repository-switch-dialog" role="dialog" aria-modal="true" aria-labelledby="repository-switch-title">
        <div class="repository-switch-header">
          <div>
            <strong id="repository-switch-title">{{ t("repository.switch") }}</strong>
            <span>{{ t("repository.recentLimit") }}</span>
          </div>
          <button
            class="icon-button"
            :title="t('common.close')"
            :aria-label="t('common.close')"
            @click="repositorySwitchDialog.visible = false"
          ><AppIcon name="x" /></button>
        </div>
        <div v-if="repositorySwitchDialog.repositories.length" class="recent-repository-list">
          <button
            v-for="item in repositorySwitchDialog.repositories"
            :key="item.path"
            class="recent-repository-item"
            :disabled="busy || !item.available || item.path === repository?.path"
            :title="item.path"
            @click="switchToRecentRepository(item.path)"
          >
            <AppIcon name="repository" />
            <span><strong>{{ item.name }}</strong><small>{{ item.path }}</small></span>
            <em v-if="item.path === repository?.path">{{ t("repository.currentRepository") }}</em>
            <em v-else-if="!item.available" class="unavailable">{{ t("repository.pathUnavailable") }}</em>
          </button>
        </div>
        <EmptyState
          v-else
          icon="repository"
          :title="t('repository.noRecentRepositories')"
          :description="t('repository.chooseAnother')"
          compact
        />
        <div class="repository-switch-actions">
          <button @click="repositorySwitchDialog.visible = false">{{ t("common.cancel") }}</button>
          <button class="primary" :disabled="busy" @click="chooseOtherRepository">
            <AppIcon name="folder" />{{ t("repository.chooseAnother") }}
          </button>
        </div>
      </section>
    </div>
    <div
      v-if="repositoryOpenDialog"
      class="repository-open-overlay"
      @pointerdown.self="closeRepositoryOpenDialog"
    >
      <section class="repository-open-dialog" role="dialog" aria-modal="true" aria-labelledby="repository-open-title">
        <div class="repository-switch-header">
          <div>
            <strong id="repository-open-title">{{ t("repository.openTitle") }}</strong>
            <span>{{ t("repository.openDescription") }}</span>
          </div>
          <button
            class="icon-button"
            :title="t('common.close')"
            :aria-label="t('common.close')"
            :disabled="repositoryOpening"
            @click="closeRepositoryOpenDialog"
          ><AppIcon name="x" /></button>
        </div>
        <div v-if="repositoryOpenError" class="repository-open-error" role="alert">
          <AppIcon name="alert" />
          <span>{{ repositoryOpenError }}</span>
        </div>
        <div class="repository-open-dropzone" :class="{ loading: repositoryOpening }">
          <template v-if="repositoryOpening">
            <span class="loading-spinner small" aria-hidden="true"></span>
            <strong>{{ t("repository.opening") }}</strong>
            <span>{{ t("repository.openingDetail") }}</span>
          </template>
          <template v-else>
            <AppIcon name="folder" :size="30" />
            <strong>{{ t("repository.dropHere") }}</strong>
            <span>{{ t("repository.dropDetail") }}</span>
          </template>
        </div>
        <div class="repository-open-divider"><span>{{ t("repository.or") }}</span></div>
        <div class="repository-switch-actions">
          <button :disabled="repositoryOpening" @click="closeRepositoryOpenDialog">{{ t("common.cancel") }}</button>
          <button class="primary" :disabled="busy || repositoryOpening" @click="chooseRepository">
            <span v-if="repositoryOpening" class="mini-spinner" aria-hidden="true"></span>
            <AppIcon v-else name="folder" />
            {{ repositoryOpening ? t("repository.openingShort") : t("repository.chooseFolder") }}
          </button>
        </div>
      </section>
    </div>
    <div v-if="externalChangeDialog.visible" class="external-change-overlay">
      <section class="external-change-dialog" role="alertdialog" aria-modal="true" aria-labelledby="external-change-title">
        <div class="external-change-heading">
          <span class="external-change-icon"><AppIcon name="alert" :size="20" /></span>
          <div>
            <strong id="external-change-title">{{ t("externalChange.title") }}</strong>
            <span :title="externalChangeDialog.change?.path">{{ externalChangeDialog.change?.path }}</span>
          </div>
        </div>
        <p v-if="summary?.dirty">
          {{ t("externalChange.dirty") }}
        </p>
        <p v-else>
          {{ t("externalChange.clean") }}
        </p>
        <div class="external-change-actions">
          <button :disabled="busy" @click="deferExternalReload">{{ t("externalChange.defer") }}</button>
          <button class="primary" :disabled="busy" @click="reloadExternal('left')">{{ t("externalChange.reload") }}</button>
        </div>
      </section>
    </div>
    <div v-if="startupLoading" class="loading-overlay" role="status" aria-live="polite">
      <div class="loading-dialog">
        <div class="loading-spinner" aria-hidden="true"></div>
        <strong>{{ repository ? t("startup.repository") : t("startup.workbooks") }}</strong>
        <span>{{ t("startup.detail") }}</span>
        <div class="loading-track" aria-hidden="true"><i></i></div>
      </div>
    </div>
    <footer class="statusbar">
      <span class="status-primary"><span class="status-dot" :class="{ busy }"></span>{{ statusText }}</span>
      <span v-if="busy" class="status-busy"><span class="mini-spinner"></span>{{ t("common.loading") }}</span>
      <span v-if="summary?.warnings?.length" class="status-warning"><AppIcon name="alert" :size="13" />{{ summary.warnings.at(-1) }}</span>
    </footer>
  </div>
</template>
