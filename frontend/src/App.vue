<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { EventsOn } from "../wailsjs/runtime/runtime";
import AppIcon from "./components/AppIcon.vue";
import EmptyState from "./components/EmptyState.vue";
import { backend } from "./backend";
import { nextDiffIndex, preferredDiffFilter, type DiffFilter } from "./diffNav";
import { containsCell, makeRange, rangeSize, type CellPoint, type SelectionRange } from "./gridSelection";
import type {
  CellDiff,
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
const REPOSITORY_SEARCH_HISTORY_PREFIX = "ugxlsx:repository-search-history:";
const DIFF_FILTER_TABS: DiffFilter[] = ["added", "deleted", "modified", "conflict"];

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
const integrationNotice = ref("");
const diffFilter = ref<DiffFilter>("modified");
const leftScroll = ref<HTMLElement | null>(null);
const rightScroll = ref<HTMLElement | null>(null);
const viewportTop = ref(0);
const viewportLeft = ref(0);
const zoom = ref(1);
const columnWidths = ref<Record<number, number>>({});
const contextMenu = ref<{
  visible: boolean;
  x: number;
  y: number;
  row: number;
  side: "left" | "right";
  kind: "cell" | "row";
}>({ visible: false, x: 0, y: 0, row: 0, side: "right", kind: "cell" });
const idDialog = ref<{
  visible: boolean;
  rows: number[];
  values: string[];
}>({ visible: false, rows: [], values: [] });
const repositorySwitchDialog = ref<{
  visible: boolean;
  repositories: RecentRepository[];
}>({ visible: false, repositories: [] });
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

interface RepositoryTreeRow {
  kind: "directory" | "file";
  path: string;
  name: string;
  depth: number;
}

const activeSheet = computed(() => summary.value?.diff.sheets.find((item) => item.name === sheet.value));
const leftReadonly = computed(() => Boolean(summary.value?.options.readonlyLeft));
const totalRows = computed(() => Math.max(activeSheet.value?.maxRow ?? 0, 50) + 10);
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

function sourcePathLabel(path: string): string {
  if (!summary.value?.options.gitDiff) return path;
  const filename = path.split(/[\\/]/).filter(Boolean).at(-1);
  if (filename === "missing-left.xlsx" || filename === "missing-right.xlsx") {
    return "Git 快照 · 该版本不存在";
  }
  return filename ? `Git 快照 · ${filename}` : "Git 差异快照";
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
  const metrics = { added: 0, deleted: 0, modified: 0, conflict: 0 };
  for (const item of summary.value?.diff.sheets ?? []) {
    metrics.added += item.addedRowCount;
    metrics.deleted += item.deletedRowCount;
    metrics.modified += item.modifiedRowCount;
    metrics.conflict += item.conflictRowCount;
  }
  return metrics;
});
const currentTaskName = computed(() => {
  const selectedFile = repository.value?.selectedFile;
  if (selectedFile) return selectedFile.split("/").at(-1) ?? selectedFile;
  if (summary.value?.options.title) return summary.value.options.title;
  return "表格对比与合并";
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
  const result = diffs.value
    .filter((item) => containsCell(selection.value, item.ref.row, item.ref.col))
    .map((item) => ({ row: item.ref.row, col: item.ref.col }));
  if (result.length === 0 && selectionSize.value === 1 && activePoint.value) {
    result.push({ ...activePoint.value });
  }
  return result;
});
function rowsForContext(row: number) {
  if (!selection.value || row < selection.value.startRow || row > selection.value.endRow) {
    return [row];
  }
  return Array.from(
    { length: selection.value.endRow - selection.value.startRow + 1 },
    (_, index) => selection.value!.startRow + index
  );
}
const contextRows = computed(() => {
  const row = contextMenu.value.row;
  if (!row) return [];
  return rowsForContext(row);
});
const contextActionableRows = computed(() =>
  contextRows.value.filter((row) => rowStatus(row) !== "unchanged")
);
const contextActionableStatuses = computed(() =>
  contextActionableRows.value.map((row) => rowStatus(row))
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
  if (contextHasMixedConflict.value) return "包含冲突";
  const statuses = [...new Set(contextActionableStatuses.value)];
  if (statuses.length === 0) return "无差异";
  if (statuses.length > 1) return "混合差异";
  return rowStatusLabel(statuses[0]);
});
const contextActionDisabled = computed(() =>
  busy.value || Boolean(summary.value?.options.readonlyLeft) || !comparisonActive.value
);
const automaticIDLabel = computed(() => {
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
  if (!summary.value) return repository.value ? "请选择仓库中的 XLSX 表格" : "尚未打开工作簿";
  const axis = activePoint.value ? `${columnName(activePoint.value.col)}${activePoint.value.row}` : "—";
  const selectedText = selectionSize.value > 1 ? ` · 已选 ${selectionSize.value} 格` : "";
  return `${axis}${selectedText} · ${summary.value.diff.differenceCount} 处差异 · ${
    summary.value.dirty ? "有未保存修改" : "已保存"
  }`;
});

async function guard<T>(action: () => Promise<T>): Promise<T | undefined> {
  pendingActions++;
  busy.value = true;
  error.value = "";
  try {
    return await action();
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : String(reason);
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

function loadRepositorySearchHistory(path: string): string[] {
  try {
    const saved = JSON.parse(window.localStorage.getItem(repositorySearchHistoryKey(path)) ?? "[]");
    if (!Array.isArray(saved)) return [];
    return saved
      .filter((item): item is string => typeof item === "string" && Boolean(item.trim()))
      .slice(0, REPOSITORY_SEARCH_HISTORY_LIMIT);
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
  stopOriginalPreview();
}

async function chooseRepository() {
  window.clearTimeout(differenceIndexTimer);
  const result = await guard(() => backend.selectRepository());
  if (result) await acceptRepositoryResult(result);
  else scheduleDifferenceIndexPoll();
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

async function configureUGit() {
  integrationNotice.value = "";
  const result = await guard(() => backend.configureUGit());
  if (result && !result.cancelled) {
    integrationNotice.value = result.message;
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
  if (state === "loading") return "正在加载表格……";
  if (state === "comparing") return "正在对比表格……";
  return side === "left" ? repository.value.leftMessage : repository.value.rightMessage;
}

async function acceptSummary(data: Summary, preferredSheet = sheet.value, resetSelection = true) {
  summary.value = data;
  const fallback = data.diff.sheets[0]?.name ?? "";
  const preferred = data.diff.sheets.find((item) => item.name === preferredSheet);
  const firstDifferent = data.diff.sheets.find((item) => item.differenceCount > 0);
  const nextSheet = resetSelection && preferred?.differenceCount === 0
    ? (firstDifferent?.name ?? preferred.name)
    : (preferred?.name ?? firstDifferent?.name ?? fallback);
  if (nextSheet) await loadSheet(nextSheet, resetSelection);
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
  selectDiffFilter(preferredDiffFilter(diffs.value));
  if (changed || resetSelection) {
    const target = filteredDiffEntries.value[0]?.item;
    if (target) {
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
  try {
    const data = await backend.region(
      requestedSheet,
      Math.max(1, fromRow),
      VIEW_ROWS,
      Math.max(1, fromCol),
      VIEW_COLS
    );
    if (request !== regionRequest || sheet.value !== requestedSheet) return false;
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
  const index = diffs.value.findIndex((item) => item.ref.row === cell.row && item.ref.col === cell.col);
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

function selectDiffFilter(status: DiffFilter) {
  diffFilter.value = status;
  diffIndex.value = diffs.value.findIndex((item) =>
    (item.rowStatus || "modified") === status
  );
}

function selectDiffEntry(index: number, item: CellDiff) {
  diffIndex.value = index;
  void scrollTo(item.ref.row, item.ref.col);
}

async function scrollTo(row: number, col: number) {
  const desiredTop = Math.max(0, (row - 2) * rowHeight.value);
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
  const cell = region.value?.cells.find((item) => item.row === row && item.col === col);
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
  const type = inlineEditType(current.value, current.original);
  const data = await guard(() =>
    backend.edit(sheet.value, current.row, current.col, current.value, type)
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
  if (!cell) return "请选择一个单元格";
  const value = displayedCellValue(cell, side);
  if (!value.present) return "（不存在）";
  if (value.formula) return `=${value.formula}`;
  if (value.raw === "") return '""（空字符串）';
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

async function undo() {
  const data = await guard(() => backend.undo());
  if (data) await acceptSummary(data, sheet.value, false);
}

function onWindowKeyDown(event: KeyboardEvent) {
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
  if (!event.ctrlKey && !event.metaKey) return;
  const key = event.key.toLowerCase();
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
  const target = event.target;
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

function rowStatus(row: number): RowStatus {
  const classified = activeSheet.value?.rows?.find((item) => item.row === row)?.status;
  if (classified) return classified;
  const cells = region.value?.cells.filter((item) => item.row === row) ?? [];
  const regionStatus = cells.find((item) => item.rowStatus !== "unchanged")?.rowStatus;
  if (regionStatus) return regionStatus;
  return cells.some((item) => item.status !== "unchanged") ? "modified" : "unchanged";
}

function rowStatusLabel(status: RowStatus) {
  switch (status) {
  case "added": return "增加";
  case "deleted": return "删除";
  case "modified": return "修改";
  case "conflict": return "冲突";
  default: return "无差异";
  }
}

function rowResolution(
  row: number,
  side: "left" | "right"
): RowResolution | null {
  const items = summary.value?.resolutions ?? [];
  for (let index = items.length - 1; index >= 0; index--) {
    const item = items[index];
    if (item.sheet !== sheet.value) continue;
    if (side === "right" && item.sourceRow === row) return item;
    if (
      side === "left" &&
      item.targetRow === row &&
      (item.kind === "append-auto" || item.kind === "append-specified")
    ) return item;
  }
  return null;
}

function resolutionLabel(item: RowResolution) {
  switch (item.kind) {
  case "overwrite-row":
    return "已覆盖整行到左侧";
  case "overwrite-cells":
    return `已覆盖 ${item.cellCount || 1} 个单元格`;
  case "append-auto":
    return `已自动新增为 ID ${item.targetId}`;
  case "append-specified":
    return `已新增为指定 ID ${item.targetId}`;
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
  return `ugxlsx.sheet-layout.v1:${file}:${name}`;
}

function loadLayout(name: string) {
  zoom.value = 1;
  columnWidths.value = {};
  try {
    const saved = window.localStorage.getItem(layoutKey(name));
    if (!saved) return;
    const value = JSON.parse(saved) as { zoom?: number; widths?: Record<number, number> };
    zoom.value = Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, value.zoom ?? 1));
    columnWidths.value = value.widths ?? {};
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
  const hasSelectedAction = selected && rowsForContext(cell.row).some((row) => rowStatus(row) !== "unchanged");
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
    side,
    kind: "cell"
  };
}

function openRowMenu(event: MouseEvent, row: number, side: "left" | "right") {
  event.preventDefault();
  const selected = Boolean(selection.value && row >= selection.value.startRow && row <= selection.value.endRow);
  const hasSelectedAction = selected && rowsForContext(row).some((selectedRow) => rowStatus(selectedRow) !== "unchanged");
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
    side,
    kind: "row"
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
  if ((window as Window & { runtime?: unknown }).runtime) {
    stopDropListener = EventsOn("repository-drop-result", (result: RepositoryResult | null, message: string) => {
      if (message) {
        error.value = message;
      } else if (result) {
        void acceptRepositoryResult(result);
      }
    });
  }
  void initialLoad();
});

onBeforeUnmount(() => {
  window.clearTimeout(loadingTimer);
  window.clearTimeout(differenceIndexTimer);
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
  stopDropListener?.();
});
</script>

<template>
  <div class="app-shell" :aria-busy="startupLoading">
    <header class="toolbar">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true"><img src="/appicon.svg" alt="" /></span>
        <span class="brand-copy">
          <strong>SheetProof <span class="product-name-zh">表鉴</span></strong>
          <small :title="currentTaskName">{{ currentTaskName }}</small>
        </span>
      </div>

      <div class="toolbar-group file-actions" aria-label="打开来源">
        <button class="secondary" :disabled="busy" @click="chooseRepository">
          <AppIcon name="repository" />
          <span class="button-label">打开本地仓库</span>
        </button>
        <button class="ghost" :disabled="busy" @click="chooseFiles">
          <AppIcon name="files" />
          <span class="button-label">打开左右文件</span>
        </button>
      </div>

      <div class="toolbar-group diff-navigation" aria-label="差异导航">
        <button
          class="icon-button"
          :disabled="!filteredDiffEntries.length || busy || !comparisonActive"
          title="上一处差异"
          aria-label="上一处差异"
          @click="navigate(-1)"
        ><AppIcon name="chevron-left" /><span class="button-label">上一处</span></button>
        <span class="counter" aria-live="polite">
          {{ filteredDiffPosition >= 0 ? filteredDiffPosition + 1 : 0 }} / {{ filteredDiffEntries.length }}
        </span>
        <button
          class="icon-button"
          :disabled="!filteredDiffEntries.length || busy || !comparisonActive"
          title="下一处差异"
          aria-label="下一处差异"
          @click="navigate(1)"
        ><span class="button-label">下一处</span><AppIcon name="chevron-right" /></button>
      </div>

      <span class="grow"></span>

      <div class="toolbar-group edit-actions" aria-label="编辑与合并">
        <button
          :disabled="!copyTargets.length || busy || summary?.options.readonlyLeft || !comparisonActive"
          class="primary merge-action"
          @click="copySelection"
        >
          <AppIcon name="merge" />
          <span>复制{{ copyTargets.length > 1 ? ` ${copyTargets.length} 格` : "" }}到左侧</span>
        </button>
        <button
          class="icon-button"
          :disabled="!summary?.undoCount || busy"
          title="撤销（Ctrl/Command + Z）"
          aria-label="撤销"
          @click="undo"
        ><AppIcon name="undo" /><span class="button-label">撤销</span></button>
        <button
          class="zoom-button ghost"
          :title="`当前表格缩放为 ${Math.round(zoom * 100)}%；按住 Ctrl/Command 并滚动鼠标滚轮调整，点击恢复为 100%`"
          @click="resetZoom"
        ><AppIcon name="zoom" />缩放 {{ Math.round(zoom * 100) }}%</button>
      </div>

      <div class="toolbar-group save-actions" aria-label="保存结果">
        <button
          class="icon-button ghost"
          :disabled="!summary || busy || summary.options.readonlyLeft"
          :title="repository ? '导出副本（Ctrl/Command + Shift + S）' : '另存为（Ctrl/Command + Shift + S）'"
          aria-label="另存为"
          @click="save(true)"
        ><AppIcon name="save-as" /><span class="button-label">{{ repository ? "导出副本" : "另存为" }}</span></button>
        <button
          :disabled="!summary?.dirty || busy || summary.options.readonlyLeft"
          class="save"
          @click="save(false)"
        >
          <AppIcon name="save" />
          <span>{{ repository ? "保存到当前工作区" : "保存左侧" }}</span>
        </button>
      </div>
      <div class="toolbar-group integration-actions" aria-label="外部工具配置">
        <button
          class="icon-button ghost"
          :disabled="busy"
          title="将当前应用配置为 UGit 的 *.xlsx 差异与合并工具"
          aria-label="配置 UGit"
          @click="configureUGit"
        >
          <AppIcon name="settings" />
          <span class="button-label">配置 UGit</span>
        </button>
      </div>
      <div v-if="busy" class="toolbar-progress" aria-hidden="true"></div>
    </header>

    <div v-if="error" class="error-banner" role="alert">
      <AppIcon name="alert" />
      <span>{{ error }}</span>
      <button class="icon-button" title="关闭错误提示" aria-label="关闭错误提示" @click="error = ''">
        <AppIcon name="x" />
      </button>
    </div>
    <div v-if="integrationNotice" class="success-banner" role="status">
      <AppIcon name="check" />
      <span>{{ integrationNotice }}</span>
      <button
        class="icon-button"
        title="关闭配置提示"
        aria-label="关闭配置提示"
        @click="integrationNotice = ''"
      >
        <AppIcon name="x" />
      </button>
    </div>
    <div v-if="repository" class="repository-bar">
      <div class="repository-identity">
        <strong><AppIcon name="repository" />{{ repository.name }}</strong>
        <span :title="repository.path">{{ repository.path }}</span>
      </div>
      <span class="branch-status" :class="{ warning: repository.detached }">
        <AppIcon name="branch" :size="13" />
        {{ repository.detached ? `Detached HEAD · ${repository.currentBranch}` : `当前分支 · ${repository.currentBranch}` }}
      </span>
      <span v-if="repository.workspaceDirty" class="working-status"><span class="status-dot"></span>工作区有未提交修改</span>
      <span v-if="repository.operation" class="operation-status"><span class="status-dot"></span>正在进行 {{ repository.operation }}</span>
      <span class="grow"></span>
      <button class="ghost compact-button" :disabled="busy" @click="showRepositorySwitcher">切换仓库</button>
    </div>
    <div v-if="repository?.notice" class="notice-banner"><AppIcon name="info" />{{ repository.notice }}</div>

    <main
      v-if="summary || repository"
      class="workspace"
      :class="{ 'repository-workspace': !!repository }"
      :style="repository ? { gridTemplateColumns: `${repository.sidebarWidth}px 1fr` } : undefined"
    >
      <aside v-if="repository" ref="repoSidebar" class="repository-sidebar">
        <div class="repository-sidebar-tabs" role="tablist" aria-label="仓库侧边栏">
          <button
            role="tab"
            :aria-selected="repositorySidebarTab === 'files'"
            :class="{ active: repositorySidebarTab === 'files' }"
            @click="repositorySidebarTab = 'files'"
          ><AppIcon name="folder" :size="14" />仓库文件</button>
          <button
            role="tab"
            :aria-selected="repositorySidebarTab === 'differences'"
            :class="{ active: repositorySidebarTab === 'differences' }"
            @click="repositorySidebarTab = 'differences'"
          >
            <AppIcon name="files" :size="14" />差异表
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
            <AppIcon name="selection" :size="14" />工作表/差异
            <span v-if="summary" class="sidebar-tab-count">{{ summary.diff.differenceCount }}</span>
          </button>
        </div>

        <div v-if="repositorySidebarTab !== 'sheets'" class="repository-files-pane" role="tabpanel">
          <div class="repository-tree-header">
            <strong>{{ repositorySidebarTab === "differences" ? "差异表" : "仓库文件" }}</strong>
            <button
              class="icon-button ghost"
              title="刷新目录树与分支"
              aria-label="刷新目录树与分支"
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
                :placeholder="repositorySidebarTab === 'differences' ? '筛选差异表或目录' : '筛选文件或目录'"
                :aria-label="repositorySidebarTab === 'differences' ? '筛选差异表或目录' : '筛选仓库文件或目录'"
                @focus="repositorySearchFocused = true"
                @blur="finishRepositorySearch"
                @keydown.enter.prevent="endRepositorySearch"
                @keydown.esc.prevent="endRepositorySearch"
              />
              <button
                v-if="repositorySearch"
                type="button"
                class="repository-search-clear"
                title="清空搜索"
                aria-label="清空搜索"
                @mousedown.prevent
                @click="clearRepositorySearch"
              ><AppIcon name="x" :size="15" /></button>
            </div>
            <div
              v-if="repositorySearchFocused"
              class="repository-search-history"
              role="listbox"
              aria-label="最近搜索"
            >
              <div class="repository-search-history-title">最近搜索</div>
              <button
                v-for="query in repositorySearchHistory"
                :key="query"
                type="button"
                role="option"
                :aria-label="`搜索 ${query}`"
                @mousedown.prevent
                @click="applyRepositorySearchHistory(query)"
              ><AppIcon name="search" :size="12" /><span>{{ query }}</span></button>
              <div v-if="!repositorySearchHistory.length" class="repository-search-history-empty">
                暂无搜索记录
              </div>
            </div>
          </div>
          <div
            v-if="repositorySidebarTab === 'differences' && !repository.differenceFiles.length"
            class="tree-empty"
          >
            {{
              repository.differenceIndexing
                ? "正在后台建立差异表索引……"
                : repository.selectedRef
                  ? "当前工作区与所选分支没有差异表格"
                  : "请选择一个用于对比的分支"
            }}
          </div>
          <div v-else-if="repositorySidebarTab === 'files' && !repository.files.length" class="tree-empty">
            仓库中没有 XLSX 文件
          </div>
          <div v-else-if="!repositoryRows.length" class="tree-empty">没有匹配的文件或目录</div>
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
            <div class="side-title">工作表</div>
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
                <span v-if="item.addedRowCount" class="row-status-chip added">增加 {{ item.addedRowCount }}</span>
                <span v-if="item.deletedRowCount" class="row-status-chip deleted">删除 {{ item.deletedRowCount }}</span>
                <span v-if="item.modifiedRowCount" class="row-status-chip modified">修改 {{ item.modifiedRowCount }}</span>
                <span v-if="item.conflictRowCount" class="row-status-chip conflict">冲突 {{ item.conflictRowCount }}</span>
                <span v-if="item.orderDifferent">顺序不同</span>
                <span v-if="!item.differenceCount">无差异</span>
              </small>
            </button>
            <div class="diff-index-title">差异索引</div>
            <div class="diff-filter-tabs" role="tablist" aria-label="差异索引分类">
              <button
                v-for="status in DIFF_FILTER_TABS"
                :key="status"
                role="tab"
                :aria-selected="diffFilter === status"
                :class="[status, { active: diffFilter === status }]"
                :disabled="!diffFilterCounts[status]"
                @click="selectDiffFilter(status)"
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
                {{ item.ref.row }}:{{ columnName(item.ref.col) }} · {{ item.status }}
              </button>
              <div v-if="!filteredDiffEntries.length" class="diff-list-empty">
                当前工作表没有“{{ rowStatusLabel(diffFilter) }}”差异
              </div>
            </div>
          </template>
          <div v-else class="tree-empty">请先在“仓库文件”中选择一个 XLSX 表格</div>
        </div>
        <div class="repository-resizer" @pointerdown="beginRepositoryResize"></div>
      </aside>

      <aside v-else class="sidebar">
        <div class="side-title">工作表</div>
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
            <span v-if="item.addedRowCount" class="row-status-chip added">增加 {{ item.addedRowCount }}</span>
            <span v-if="item.deletedRowCount" class="row-status-chip deleted">删除 {{ item.deletedRowCount }}</span>
            <span v-if="item.modifiedRowCount" class="row-status-chip modified">修改 {{ item.modifiedRowCount }}</span>
            <span v-if="item.conflictRowCount" class="row-status-chip conflict">冲突 {{ item.conflictRowCount }}</span>
            <span v-if="item.orderDifferent">顺序不同</span>
            <span v-if="!item.differenceCount">无差异</span>
          </small>
        </button>
        <div class="diff-index-title">差异索引</div>
        <div class="diff-filter-tabs" role="tablist" aria-label="差异索引分类">
          <button
            v-for="status in DIFF_FILTER_TABS"
            :key="status"
            role="tab"
            :aria-selected="diffFilter === status"
            :class="[status, { active: diffFilter === status }]"
            :disabled="!diffFilterCounts[status]"
            @click="selectDiffFilter(status)"
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
            {{ item.ref.row }}:{{ columnName(item.ref.col) }} · {{ item.status }}
          </button>
          <div v-if="!filteredDiffEntries.length" class="diff-list-empty">
            当前工作表没有“{{ rowStatusLabel(diffFilter) }}”差异
          </div>
        </div>
      </aside>

      <section class="content">
        <div v-if="repository" class="file-strip repository-file-strip">
          <div class="source-card editable-source">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon name="edit" /></span>
              <span>
                <small>原始表格 · 可编辑</small>
                <strong>{{ repository.detached ? "当前工作区 · Detached HEAD" : repository.currentBranch }}</strong>
              </span>
              <span class="source-state" :class="{ warning: repository.fileModified || summary?.dirty }">
                <AppIcon
                  :name="!repository.selectedFile ? 'info' : repository.fileModified || summary?.dirty ? 'alert' : 'check'"
                  :size="12"
                />
                {{ !repository.selectedFile ? "待选择" : repository.fileModified || summary?.dirty ? "有修改" : "已就绪" }}
              </span>
            </div>
            <span class="source-path" :title="repository.selectedFile">{{ repository.selectedFile || "尚未选择表格" }}</span>
            <small v-if="repository.fileModified">Git：该文件有未提交修改</small>
            <small v-if="summary?.dirty">工具内：有尚未保存的编辑</small>
          </div>
          <div class="source-card readonly-source reference-header">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon name="branch" /></span>
              <span>
                <small>目标表格 · 只读</small>
                <strong>对比分支</strong>
              </span>
              <span class="source-state">
                <AppIcon name="check" :size="12" />
                {{ comparisonActive ? "已就绪" : "待选择" }}
              </span>
            </div>
            <select
              :value="repository.selectedRef"
              :disabled="busy || !repository.branches.length"
              aria-label="选择对比分支"
              @change="selectRepositoryRef(($event.target as HTMLSelectElement).value)"
            >
              <option value="" disabled>请选择一个用于对比的分支</option>
              <optgroup label="本地分支">
                <option
                  v-for="branch in repository.branches.filter((item) => item.kind === 'local')"
                  :key="branch.fullName"
                  :value="branch.fullName"
                >{{ branch.name }}</option>
              </optgroup>
              <optgroup label="远端分支">
                <option
                  v-for="branch in repository.branches.filter((item) => item.kind === 'remote')"
                  :key="branch.fullName"
                  :value="branch.fullName"
                >{{ branch.name }}</option>
              </optgroup>
            </select>
            <span class="source-path" :title="repository.selectedFile">{{ repository.selectedFile || "尚未选择表格" }}</span>
          </div>
        </div>
        <div v-else-if="summary" class="file-strip">
          <div class="source-card" :class="leftReadonly ? 'readonly-source' : 'editable-source'">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon :name="leftReadonly ? 'files' : 'edit'" /></span>
              <span>
                <small>{{ summary.options.gitDiff ? "Git 差异快照 · 只读" : summary.options.gitMerge ? "Git 合并来源 · 可编辑" : leftReadonly ? "原始表格 · 只读" : "原始表格 · 可编辑" }}</small>
                <strong>{{ summary.options.leftLabel }}</strong>
              </span>
              <span class="source-state"><AppIcon name="check" :size="12" />{{ leftReadonly ? "只读" : "已解析" }}</span>
            </div>
            <span class="source-path" :title="summary.options.gitDiff ? '由 Git 提供的临时只读快照' : summary.diff.leftFile">
              {{ sourcePathLabel(summary.diff.leftFile) }}
            </span>
          </div>
          <div class="source-card readonly-source">
            <div class="source-card-heading">
              <span class="source-icon"><AppIcon name="files" /></span>
              <span><small>{{ summary.options.gitDiff ? "Git 差异快照 · 只读" : summary.options.gitMerge ? "Git 合并来源 · 只读" : "目标表格 · 只读" }}</small><strong>{{ summary.options.rightLabel }}</strong></span>
              <span class="source-state"><AppIcon name="check" :size="12" />只读</span>
            </div>
            <span class="source-path" :title="summary.options.gitDiff ? '由 Git 提供的临时只读快照' : summary.diff.rightFile">
              {{ sourcePathLabel(summary.diff.rightFile) }}
            </span>
          </div>
        </div>

        <div v-if="summary" class="comparison-summary-stack">
          <div class="result-summary" aria-label="对比结果摘要">
            <div class="result-summary-title">
              <AppIcon :name="summary.diff.equal ? 'check' : 'selection'" />
              <span>
                <small>对比结果</small>
                <strong>{{ summary.diff.equal ? "两侧内容一致" : `${summary.diff.differenceCount} 处差异` }}</strong>
              </span>
            </div>
            <span class="summary-metric added"><i></i>新增行 <strong>{{ resultMetrics.added }}</strong></span>
            <span class="summary-metric deleted"><i></i>删除行 <strong>{{ resultMetrics.deleted }}</strong></span>
            <span class="summary-metric modified"><i></i>修改行 <strong>{{ resultMetrics.modified }}</strong></span>
            <span class="summary-metric conflict"><i></i>冲突行 <strong>{{ resultMetrics.conflict }}</strong></span>
            <span class="summary-context">
              当前筛选：{{ rowStatusLabel(diffFilter) }}
              <template v-if="selectionSize > 0"> · 已选择 {{ selectionSize }} 格</template>
            </span>
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
                {{ repository ? "当前分支中的表格" : summary?.options.gitDiff ? "原始快照" : "原始表格" }}
              </strong>
              <span class="panel-permission" :class="leftReadonly ? 'readonly' : 'editable'">
                <AppIcon :name="leftReadonly ? 'files' : 'edit'" :size="12" />{{ leftReadonly ? "只读" : "可编辑" }}
              </span>
              <small v-if="previewOriginal" class="grid-edit-hint">正在看打开时的状态</small>
              <small v-else-if="summary?.options.gitDiff" class="grid-edit-hint">由 Git 提供，不能编辑或保存</small>
              <small v-else-if="summary && !summary.options.readonlyLeft" class="grid-edit-hint">双击单元格编辑</small>
              <button
                v-if="!leftReadonly"
                class="original-preview-button"
                :class="{ active: previewOriginal }"
                :disabled="!canPreviewOriginal"
                :aria-pressed="previewOriginal"
                :title="canPreviewOriginal ? '按住查看打开表格时的状态；表格聚焦时也可按住 Tab' : '修改左侧表格后即可回看打开时的状态'"
                @pointerdown="startOriginalPreviewFromPointer"
                @keydown.space.prevent="startOriginalPreviewFromKeyboardButton"
                @keyup.space.prevent="stopOriginalPreviewFromPointer"
                @keydown.enter.prevent="startOriginalPreviewFromKeyboardButton"
                @keyup.enter.prevent="stopOriginalPreviewFromPointer"
              ><AppIcon name="undo" :size="12" />{{ previewOriginal ? "松开看最新" : "前后对比 · 按住" }}<kbd>Tab</kbd></button>
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
                description="从左侧目录树选择一个 XLSX 表格开始对比。"
              />
            </div>
            <div v-else ref="leftScroll" class="grid-scroll" tabindex="-1" @pointerdown.capture="focusGrid" @scroll="onScroll('left')" @wheel="onGridWheel">
              <div class="canvas" :style="{ width: `${canvasWidth}px`, height: `${totalRows * rowHeight + rowHeight}px` }">
                <template v-if="region">
                  <div class="col-header-layer">
                    <div
                      v-for="col in visibleColumns"
                      :key="`lh${col}`"
                      class="col-header"
                      :style="{
                        top: 0,
                        left: `${columnLeft(col)}px`,
                        width: `${columnWidth(col)}px`,
                        height: `${rowHeight}px`,
                        fontSize: `${scaledFontSize}px`
                      }"
                    >
                      {{ columnName(col) }}
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
                      {{ row }}
                    </div>
                  </div>
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
                      aria-label="编辑左侧单元格"
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
              <strong><span class="panel-indicator readonly"></span>{{ repository ? "对比分支中的表格" : "目标表格" }}</strong>
              <span class="panel-permission readonly"><AppIcon name="branch" :size="12" />只读</span>
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
                description="选择可用分支后，将通过 Git 对象只读加载同路径表格。"
              />
            </div>
            <div v-else ref="rightScroll" class="grid-scroll" tabindex="-1" @pointerdown.capture="focusGrid" @scroll="onScroll('right')" @wheel="onGridWheel">
              <div class="canvas" :style="{ width: `${canvasWidth}px`, height: `${totalRows * rowHeight + rowHeight}px` }">
                <template v-if="region">
                  <div class="col-header-layer">
                    <div
                      v-for="col in visibleColumns"
                      :key="`rh${col}`"
                      class="col-header"
                      :style="{
                        top: 0,
                        left: `${columnLeft(col)}px`,
                        width: `${columnWidth(col)}px`,
                        height: `${rowHeight}px`,
                        fontSize: `${scaledFontSize}px`
                      }"
                    >
                      {{ columnName(col) }}
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
                      {{ row }}
                      <span
                        v-if="rowResolution(row, 'right')"
                        class="resolution-marker"
                        :class="rowResolution(row, 'right')!.kind"
                        :title="resolutionLabel(rowResolution(row, 'right')!)"
                      >{{ resolutionLabel(rowResolution(row, "right")!) }}</span>
                    </div>
                  </div>
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
            <span class="difference-side">{{ previewOriginal ? "打开时状态" : "当前工作区" }}</span>
            <strong>{{ activeCell.axis }}</strong>
            <span class="difference-value" :title="displayedCellValue(activeCell, 'left').raw">
              {{ displayDifferenceValue(activeCell, "left") }}
            </span>
            <span class="difference-type">{{ displayedCellValue(activeCell, "left").type || "unset" }}</span>
          </div>
          <div class="difference-line right">
            <span class="difference-side">对比来源</span>
            <strong>{{ activeCell.axis }}</strong>
            <span class="difference-value" :title="activeCell.right.raw">
              {{ displayDifferenceValue(activeCell, "right") }}
            </span>
            <span class="difference-type">{{ activeCell.right.type || "unset" }}</span>
            <span class="difference-status" :class="{ changed: activeCell.status !== 'unchanged' }">
              {{ activeCell.status }}
            </span>
          </div>
        </div>
        <div v-else class="difference-inspector empty">
          {{ selectionSize > 1 ? `已选 ${selectionSize} 个单元格；请选择单格查看左右差异` : "选择一个单元格查看两侧内容；双击左侧单元格可直接编辑" }}
        </div>
      </section>
    </main>

    <main v-else class="welcome">
      <div class="welcome-card">
        <div class="welcome-product">
          <div class="logo"><img src="/appicon.svg" alt="" /></div>
          <div>
            <span class="welcome-kicker">SHEETPROOF DESKTOP</span>
            <strong>表格对比与合并</strong>
          </div>
        </div>
        <h1>打开本地 Git 仓库</h1>
        <p>从工作区选择原始表格，与任一本地或远端跟踪分支进行安全对比。</p>

        <div class="workflow-steps" aria-label="工作流程">
          <span><i>1</i>导入文件</span>
          <span><i>2</i>检查差异</span>
          <span><i>3</i>应用修改</span>
          <span><i>4</i>保存结果</span>
        </div>

        <button class="primary large" :disabled="busy" @click="chooseRepository">
          <AppIcon name="repository" />打开本地仓库
        </button>
        <div class="drop-hint">
          <AppIcon name="folder" />
          <span><strong>也可以拖入仓库目录</strong><small>支持子目录，将自动定位 Git 根目录</small></span>
        </div>
        <div class="welcome-divider"><span>直接文件模式</span></div>
        <button class="secondary-entry" :disabled="busy" @click="chooseFiles">
          <AppIcon name="files" />选择两个表格进行对比
        </button>
        <small>左侧为可编辑原始表格，右侧为只读目标表格；仅支持 .xlsx。</small>
      </div>
    </main>
    <div
      v-if="contextMenu.visible"
      class="context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @pointerdown.stop
    >
      <template v-if="contextHasMixedConflict">
        <span class="context-menu-warning">选中的内容中包含冲突，请单独处理冲突部分</span>
      </template>
      <template v-else-if="contextIsConflict">
        <button
          v-if="contextMenu.kind === 'cell'"
          :disabled="!copyTargets.length || copyTargets.length > 10000 || contextActionDisabled"
          @click="copyContextCells"
        >覆盖单元格到左侧</button>
        <button :disabled="contextActionDisabled" @click="copyContextRows">覆盖整行到左侧</button>
        <button
          v-if="automaticIDLabel"
          :disabled="contextActionDisabled"
          @click="appendContextRowsAutomatically"
        >将整行新增为 {{ automaticIDLabel }} 到左侧</button>
        <button
          v-if="activeSheet?.idColumn"
          :disabled="contextActionDisabled"
          @click="openSpecifiedIDDialog"
        >将整行新增到指定 id 到左侧</button>
      </template>
      <template v-else-if="contextIsActionable">
        <button
          v-if="contextMenu.kind === 'cell'"
          :disabled="!copyTargets.length || copyTargets.length > 10000 || contextActionDisabled"
          @click="copyContextCells"
        >复制单元格到左侧</button>
        <button :disabled="contextActionDisabled" @click="copyContextRows">复制整行到左侧</button>
      </template>
      <span>
        {{ contextRows.length > 1 ? `已选 ${contextRows.length} 行` : `第 ${contextMenu.row} 行` }}
        · {{ contextStatusLabel }}
      </span>
    </div>
    <div v-if="idDialog.visible" class="id-dialog-overlay" @pointerdown.self="idDialog.visible = false">
      <form class="id-dialog" @submit.prevent="confirmSpecifiedIDs">
        <strong>为新增行指定 ID</strong>
        <span>每个来源行需要一个在左侧尚未使用的 ID。</span>
        <label v-for="(row, index) in idDialog.rows" :key="row">
          <span>右侧第 {{ row }} 行</span>
          <input
            v-model="idDialog.values[index]"
            :aria-label="`右侧第 ${row} 行的指定 ID`"
            placeholder="输入 ID"
            autocomplete="off"
          />
        </label>
        <div class="id-dialog-actions">
          <button type="button" @click="idDialog.visible = false">取消</button>
          <button
            type="submit"
            class="primary"
            :disabled="busy || idDialog.values.some((value) => !value.trim())"
          >新增到左侧</button>
        </div>
      </form>
    </div>
    <div
      v-if="repositorySwitchDialog.visible"
      class="repository-switch-overlay"
      @pointerdown.self="repositorySwitchDialog.visible = false"
    >
      <section class="repository-switch-dialog" role="dialog" aria-modal="true" aria-labelledby="repository-switch-title">
        <div class="repository-switch-header">
          <div>
            <strong id="repository-switch-title">切换仓库</strong>
            <span>最近打开的仓库，最多显示 10 个。</span>
          </div>
          <button
            class="icon-button"
            title="关闭"
            aria-label="关闭切换仓库弹窗"
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
            <em v-if="item.path === repository?.path">当前仓库</em>
            <em v-else-if="!item.available" class="unavailable">路径不可用</em>
          </button>
        </div>
        <EmptyState
          v-else
          icon="repository"
          title="暂无最近仓库"
          description="可选择其他本地 Git 仓库。"
          compact
        />
        <div class="repository-switch-actions">
          <button @click="repositorySwitchDialog.visible = false">取消</button>
          <button class="primary" :disabled="busy" @click="chooseOtherRepository">
            <AppIcon name="folder" />选择其他仓库
          </button>
        </div>
      </section>
    </div>
    <div v-if="startupLoading" class="loading-overlay" role="status" aria-live="polite">
      <div class="loading-dialog">
        <div class="loading-spinner" aria-hidden="true"></div>
        <strong>{{ repository ? "正在加载仓库与表格" : "正在加载并比较工作簿" }}</strong>
        <span>正在读取数据并建立差异索引，请稍候…</span>
        <div class="loading-track" aria-hidden="true"><i></i></div>
      </div>
    </div>
    <footer class="statusbar">
      <span class="status-primary"><span class="status-dot" :class="{ busy }"></span>{{ statusText }}</span>
      <span v-if="busy" class="status-busy"><span class="mini-spinner"></span>处理中…</span>
      <span v-if="summary?.warnings?.length" class="status-warning"><AppIcon name="alert" :size="13" />{{ summary.warnings.at(-1) }}</span>
    </footer>
  </div>
</template>
