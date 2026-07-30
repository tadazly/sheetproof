<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { backend } from "./backend";
import { nextDiffIndex } from "./diffNav";
import { containsCell, makeRange, rangeSize, type CellPoint, type SelectionRange } from "./gridSelection";
import type { CellDiff, Region, RegionCell, RepositoryResult, RepositoryView, Summary } from "./types";

const BASE_ROW_HEIGHT = 23;
const BASE_COL_WIDTH = 96;
const BASE_ROW_HEADER_WIDTH = 42;
const MIN_COL_WIDTH = 48;
const MAX_COL_WIDTH = 420;
const MIN_ZOOM = 0.7;
const MAX_ZOOM = 1.8;
const VIEW_ROWS = 48;
const VIEW_COLS = 20;

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
const onlyDiffs = ref(false);
const leftScroll = ref<HTMLElement | null>(null);
const rightScroll = ref<HTMLElement | null>(null);
const viewportTop = ref(0);
const viewportLeft = ref(0);
const zoom = ref(1);
const columnWidths = ref<Record<number, number>>({});
const contextMenu = ref({ visible: false, x: 0, y: 0 });
const startupLoading = ref(true);
const expandedDirectories = ref(new Set<string>());
const repositorySearch = ref("");
const repositorySidebarTab = ref<"files" | "sheets">("files");
const inlineEdit = ref<{ row: number; col: number; value: string; original: RegionCell } | null>(null);
const repoSidebar = ref<HTMLElement | null>(null);
let loadingTimer = 0;
let regionRequest = 0;
let repositoryRequest = 0;
let pendingActions = 0;
let syncing = false;
let draggingSelection = false;
let draggingRows = false;
let dragAnchor: CellPoint | null = null;
let resizeState: { col: number; startX: number; startWidth: number } | null = null;
let repositoryResize: { startX: number; startWidth: number } | null = null;
let stopDropListener: (() => void) | undefined;

interface RepositoryTreeRow {
  kind: "directory" | "file";
  path: string;
  name: string;
  depth: number;
}

const activeSheet = computed(() => summary.value?.diff.sheets.find((item) => item.name === sheet.value));
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
const activeCell = computed(() => {
  const point = activePoint.value;
  return point ? region.value?.cells.find((cell) => cell.row === point.row && cell.col === point.col) ?? null : null;
});
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
const comparisonActive = computed(() => !repository.value || repository.value.comparisonActive);
const repositoryRows = computed<RepositoryTreeRow[]>(() => {
  type Node = { directories: Map<string, Node>; files: string[] };
  const root: Node = { directories: new Map(), files: [] };
  const query = repositorySearch.value.trim().toLocaleLowerCase();
  const files = (repository.value?.files ?? []).filter((path) =>
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
  repository.value = view;
  const expanded = new Set(expandedDirectories.value);
  for (const file of view.files) {
    const parts = file.split("/");
    for (let index = 1; index < parts.length; index++) {
      expanded.add(parts.slice(0, index).join("/"));
    }
  }
  expandedDirectories.value = expanded;
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
  summary.value = null;
  region.value = null;
  sheet.value = "";
  diffs.value = [];
  diffIndex.value = -1;
  activePoint.value = null;
  selection.value = null;
  inlineEdit.value = null;
}

async function chooseRepository() {
  const result = await guard(() => backend.selectRepository());
  if (result) await acceptRepositoryResult(result);
}

async function chooseFiles() {
  const data = await guard(() => backend.selectAndOpen());
  if (data) {
    repository.value = null;
    await acceptSummary(data, data.selectedSheet);
  }
}

async function selectRepositoryFile(path: string) {
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
    if (!result && request === repositoryRequest && previous) repository.value = previous;
    return;
  }
  await acceptRepositoryResult(result);
  diffIndex.value = -1;
}

async function selectRepositoryRef(value: string) {
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
    if (!result && request === repositoryRequest && previous) repository.value = previous;
    return;
  }
  await acceptRepositoryResult(result);
  diffIndex.value = -1;
}

async function refreshRepository() {
  const result = await guard(() => backend.refreshRepository());
  if (result) await acceptRepositoryResult(result);
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
  const nextSheet = data.diff.sheets.some((item) => item.name === preferredSheet) ? preferredSheet : fallback;
  if (nextSheet) await loadSheet(nextSheet, resetSelection);
}

async function loadSheet(name: string, resetSelection = true) {
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
  diffs.value = list ?? [];
  const source = leftScroll.value ?? rightScroll.value;
  const fromRow = changed || resetSelection
    ? 1
    : Math.floor((source?.scrollTop ?? viewportTop.value) / rowHeight.value) + 1;
  const fromCol = changed || resetSelection
    ? 1
    : columnAtOffset(source?.scrollLeft ?? viewportLeft.value);
  await loadRegion(fromRow, fromCol);
  if (changed || resetSelection) {
    await nextTick();
    for (const element of [leftScroll.value, rightScroll.value]) {
      if (element) {
        element.scrollTop = 0;
        element.scrollLeft = 0;
      }
    }
    viewportTop.value = 0;
    viewportLeft.value = 0;
  }
}

async function loadRegion(fromRow: number, fromCol: number): Promise<boolean> {
  if (!sheet.value) return false;
  const request = ++regionRequest;
  const data = await guard(() => backend.region(sheet.value, Math.max(1, fromRow), VIEW_ROWS, Math.max(1, fromCol), VIEW_COLS));
  if (!data || request !== regionRequest) return false;
  region.value = data;
  return true;
}

function onScroll(side: "left" | "right") {
  if (syncing) return;
  const source = side === "left" ? leftScroll.value : rightScroll.value;
  const target = side === "left" ? rightScroll.value : leftScroll.value;
  if (!source || !target) return;
  syncing = true;
  target.scrollTop = source.scrollTop;
  target.scrollLeft = source.scrollLeft;
  viewportTop.value = source.scrollTop;
  viewportLeft.value = source.scrollLeft;
  syncing = false;
  window.clearTimeout(loadingTimer);
  loadingTimer = window.setTimeout(() => {
    loadRegion(Math.floor(source.scrollTop / rowHeight.value) + 1, columnAtOffset(source.scrollLeft));
  }, 60);
}

function setSingleSelection(cell: RegionCell) {
  const point = { row: cell.row, col: cell.col };
  activePoint.value = point;
  selectionAnchor.value = point;
  selection.value = makeRange(point, point);
  const index = diffs.value.findIndex((item) => item.ref.row === cell.row && item.ref.col === cell.col);
  if (index >= 0) diffIndex.value = index;
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
  const index = nextDiffIndex(diffIndex.value, diffs.value.length, direction);
  if (index < 0) return;
  diffIndex.value = index;
  const target = diffs.value[index];
  await scrollTo(target.ref.row, target.ref.col);
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
  viewportTop.value = actualTop;
  viewportLeft.value = actualLeft;
  const loaded = await loadRegion(
    Math.floor(actualTop / rowHeight.value) + 1,
    columnAtOffset(actualLeft)
  );
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

function startInlineEdit(cell: RegionCell) {
  if (busy.value || summary.value?.options.readonlyLeft) return;
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
  const value = cell[side];
  if (!value.present) return "（不存在）";
  if (value.formula) return `=${value.formula}`;
  if (value.raw === "") return '""（空字符串）';
  return value.display || value.raw;
}

async function undo() {
  const data = await guard(() => backend.undo());
  if (data) await acceptSummary(data, sheet.value, false);
}

function onWindowKeyDown(event: KeyboardEvent) {
  if (!event.ctrlKey && !event.metaKey) return;
  const key = event.key.toLowerCase();
  if (key === "s" && event.shiftKey) {
    if (!summary.value || repository.value || summary.value.options.readonlyLeft || busy.value) return;
    event.preventDefault();
    void save(true);
    return;
  }
  if (event.shiftKey || key !== "z") return;
  const target = event.target;
  if (target instanceof Element && target.matches("input, textarea, [contenteditable='true']")) return;
  if (!summary.value?.undoCount || busy.value) return;
  event.preventDefault();
  undo();
}

async function save(as = false) {
  const data = await guard(() => (as ? backend.saveAs() : backend.save()));
  if (data) await acceptSummary(data, sheet.value, false);
}

function cellClass(cell: RegionCell) {
  return {
    cell: true,
    difference: cell.status !== "unchanged",
    selected: containsCell(selection.value, cell.row, cell.col),
    active: activePoint.value?.row === cell.row && activePoint.value?.col === cell.col,
    missing: cell.status.includes("missing")
  };
}

function rowClass(row: number) {
  return {
    "row-header": true,
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

function onGridWheel(event: WheelEvent) {
  if (!event.ctrlKey && !event.metaKey) return;
  event.preventDefault();
  const source = event.currentTarget as HTMLElement;
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
    for (const element of [leftScroll.value, rightScroll.value]) {
      if (element) {
        element.scrollLeft = nextLeft;
        element.scrollTop = nextTop;
      }
    }
    viewportLeft.value = nextLeft;
    viewportTop.value = nextTop;
    loadRegion(Math.floor(nextTop / rowHeight.value) + 1, columnAtOffset(nextLeft));
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

function openCellMenu(event: MouseEvent, cell: RegionCell) {
  event.preventDefault();
  if (!containsCell(selection.value, cell.row, cell.col)) {
    setSingleSelection(cell);
  }
  contextMenu.value = {
    visible: true,
    x: Math.min(event.clientX, window.innerWidth - 210),
    y: Math.min(event.clientY, window.innerHeight - 100)
  };
}

function openRowMenu(event: MouseEvent, row: number) {
  event.preventDefault();
  if (!selection.value || row < selection.value.startRow || row > selection.value.endRow) {
    beginRowSelection(row, event);
  }
  contextMenu.value = {
    visible: true,
    x: Math.min(event.clientX, window.innerWidth - 210),
    y: Math.min(event.clientY, window.innerHeight - 100)
  };
}

function closeContextMenu(event: PointerEvent) {
  const target = event.target as HTMLElement | null;
  if (!target?.closest(".context-menu")) contextMenu.value.visible = false;
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
  window.addEventListener("pointerdown", closeContextMenu);
  window.addEventListener("keydown", onWindowKeyDown);
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
  window.removeEventListener("pointermove", resizeColumn);
  window.removeEventListener("pointermove", resizeRepositorySidebar);
  window.removeEventListener("pointerup", finishPointerAction);
  window.removeEventListener("pointerdown", closeContextMenu);
  window.removeEventListener("keydown", onWindowKeyDown);
  stopDropListener?.();
});
</script>

<template>
  <div class="app-shell" :aria-busy="startupLoading">
    <header class="toolbar">
      <div class="brand">ugxlsx</div>
      <button class="primary" :disabled="busy" @click="chooseRepository">打开本地仓库</button>
      <button :disabled="busy" @click="chooseFiles">打开左右文件</button>
      <span class="separator"></span>
      <button :disabled="!diffs.length || busy || !comparisonActive" @click="navigate(-1)">上一处</button>
      <button :disabled="!diffs.length || busy || !comparisonActive" @click="navigate(1)">下一处</button>
      <span class="counter">{{ diffIndex >= 0 ? diffIndex + 1 : 0 }} / {{ diffs.length }}</span>
      <button
        :disabled="!copyTargets.length || busy || summary?.options.readonlyLeft || !comparisonActive"
        class="primary"
        @click="copySelection"
      >复制{{ copyTargets.length > 1 ? ` ${copyTargets.length} 格` : "" }}到左侧</button>
      <button :disabled="!summary?.undoCount || busy" title="Ctrl/Command + Z" @click="undo">撤销</button>
      <button class="zoom-button" title="按住 Ctrl 并滚动鼠标滚轮缩放" @click="resetZoom">{{ Math.round(zoom * 100) }}%</button>
      <span class="grow"></span>
      <button
        :disabled="!summary || busy || summary.options.readonlyLeft || !!repository"
        title="Ctrl/Command + Shift + S"
        @click="save(true)"
      >另存为</button>
      <button :disabled="!summary?.dirty || busy || summary.options.readonlyLeft" class="save" @click="save(false)">
        {{ repository ? "保存到当前工作区" : "保存左侧" }}
      </button>
    </header>

    <div v-if="error" class="error-banner">{{ error }}</div>
    <div v-if="repository" class="repository-bar">
      <div class="repository-identity">
        <strong>{{ repository.name }}</strong>
        <span :title="repository.path">{{ repository.path }}</span>
      </div>
      <span class="branch-status" :class="{ warning: repository.detached }">
        {{ repository.detached ? `Detached HEAD · ${repository.currentBranch}` : `当前分支 · ${repository.currentBranch}` }}
      </span>
      <span v-if="repository.workspaceDirty" class="working-status">工作区有未提交修改</span>
      <span v-if="repository.operation" class="operation-status">正在进行 {{ repository.operation }}</span>
      <span class="grow"></span>
      <button :disabled="busy" @click="chooseRepository">切换仓库</button>
    </div>
    <div v-if="repository?.notice" class="notice-banner">{{ repository.notice }}</div>

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
          >仓库文件</button>
          <button
            role="tab"
            :aria-selected="repositorySidebarTab === 'sheets'"
            :class="{ active: repositorySidebarTab === 'sheets' }"
            @click="repositorySidebarTab = 'sheets'"
          >
            工作表/差异
            <span v-if="summary" class="sidebar-tab-count">{{ summary.diff.differenceCount }}</span>
          </button>
        </div>

        <div v-if="repositorySidebarTab === 'files'" class="repository-files-pane" role="tabpanel">
          <div class="repository-tree-header">
            <strong>仓库文件</strong>
            <button title="刷新目录树与分支" :disabled="busy" @click="refreshRepository">↻</button>
          </div>
          <label class="repository-search">
            <span aria-hidden="true">⌕</span>
            <input
              v-model="repositorySearch"
              type="search"
              placeholder="筛选文件或目录"
              aria-label="筛选仓库文件或目录"
            />
          </label>
          <div v-if="!repository.files.length" class="tree-empty">仓库中没有 XLSX 文件</div>
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
              @click="row.kind === 'directory' ? toggleDirectory(row.path) : selectRepositoryFile(row.path)"
            >
              <span class="tree-icon">
                <template v-if="row.kind === 'directory'">{{ expandedDirectories.has(row.path) ? "▾" : "▸" }}</template>
                <template v-else>▧</template>
              </span>
              <span>{{ row.name }}</span>
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
              <small>{{ item.status }}<template v-if="item.orderDifferent"> · 顺序不同</template></small>
            </button>
            <label class="toggle"><input v-model="onlyDiffs" type="checkbox" /> 差异索引</label>
            <div v-if="onlyDiffs" class="diff-list">
              <button
                v-for="(item, index) in diffs"
                :key="`${item.ref.row}:${item.ref.col}`"
                @click="diffIndex = index; scrollTo(item.ref.row, item.ref.col)"
              >
                {{ item.ref.row }}:{{ columnName(item.ref.col) }} · {{ item.status }}
              </button>
              <div v-if="!diffs.length" class="diff-list-empty">当前工作表没有单元格差异</div>
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
          <small>{{ item.status }}<template v-if="item.orderDifferent"> · 顺序不同</template></small>
        </button>
        <label class="toggle"><input v-model="onlyDiffs" type="checkbox" /> 差异索引</label>
        <div v-if="onlyDiffs" class="diff-list">
          <button v-for="(item, index) in diffs" :key="`${item.ref.row}:${item.ref.col}`" @click="diffIndex = index; scrollTo(item.ref.row, item.ref.col)">
            {{ item.ref.row }}:{{ columnName(item.ref.col) }} · {{ item.status }}
          </button>
        </div>
      </aside>

      <section class="content">
        <div v-if="repository" class="file-strip repository-file-strip">
          <div>
            <strong>{{ repository.detached ? "当前工作区 · Detached HEAD" : `当前工作区 · ${repository.currentBranch}` }}</strong>
            <span>{{ repository.selectedFile || "尚未选择表格" }}</span>
            <small v-if="repository.fileModified">Git：该文件有未提交修改</small>
            <small v-if="summary?.dirty">工具内：有尚未保存的编辑</small>
          </div>
          <div class="reference-header">
            <strong>对比分支 · 只读</strong>
            <select
              :value="repository.selectedRef"
              :disabled="busy || !repository.branches.length"
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
            <span>{{ repository.selectedFile || "尚未选择表格" }}</span>
          </div>
        </div>
        <div v-else-if="summary" class="file-strip">
          <div><strong>{{ summary.options.leftLabel }}</strong><span>{{ summary.diff.leftFile }}</span></div>
          <div><strong>{{ summary.options.rightLabel }}</strong><span>{{ summary.diff.rightFile }}</span></div>
        </div>

        <div class="grids">
          <section class="grid-panel">
            <div class="panel-heading">
              <strong>{{ repository ? "当前分支中的表格 · 可编辑" : "左侧 · 可编辑" }}</strong>
              <small v-if="summary && !summary.options.readonlyLeft">双击单元格编辑</small>
            </div>
            <div v-if="repository && repository.leftState !== 'ready'" class="panel-empty">
              <div v-if="repository.leftState === 'loading' || repository.leftState === 'comparing'" class="loading-spinner small"></div>
              <strong>{{ repositoryStateMessage("left") }}</strong>
            </div>
            <div v-else ref="leftScroll" class="grid-scroll" @scroll="onScroll('left')" @wheel="onGridWheel">
              <div class="canvas" :style="{ width: `${canvasWidth}px`, height: `${totalRows * rowHeight + rowHeight}px` }">
                <template v-if="region">
                  <div
                    v-for="col in visibleColumns"
                    :key="`lh${col}`"
                    class="col-header"
                    :style="{
                      top: `${viewportTop}px`,
                      left: `${columnLeft(col)}px`,
                      width: `${columnWidth(col)}px`,
                      height: `${rowHeight}px`,
                      fontSize: `${scaledFontSize}px`
                    }"
                  >
                    {{ columnName(col) }}
                    <span class="col-resizer" @pointerdown="beginColumnResize(col, $event)"></span>
                  </div>
                  <div
                    v-for="row in visibleRows"
                    :key="`lr${row}`"
                    :class="rowClass(row)"
                    :style="{
                      top: `${row * rowHeight}px`,
                      left: `${viewportLeft}px`,
                      width: `${rowHeaderWidth}px`,
                      height: `${rowHeight}px`,
                      fontSize: `${scaledFontSize}px`
                    }"
                    @pointerdown="beginRowSelection(row, $event)"
                    @pointerenter="extendRowSelection(row)"
                    @contextmenu="openRowMenu($event, row)"
                  >
                    {{ row }}
                  </div>
                  <div
                    v-for="cell in visibleCells"
                    :key="`l${cell.row}:${cell.col}`"
                    :class="cellClass(cell)"
                    :style="{
                      top: `${cell.row * rowHeight}px`,
                      left: `${columnLeft(cell.col)}px`,
                      width: `${columnWidth(cell.col)}px`,
                      height: `${rowHeight}px`,
                      fontSize: `${scaledFontSize}px`
                    }"
                    :title="cell.left.formula ? `=${cell.left.formula}` : cell.left.raw"
                    @pointerdown="beginCellSelection(cell, $event)"
                    @pointerenter="extendCellSelection(cell)"
                    @contextmenu="openCellMenu($event, cell)"
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
                    <template v-else>{{ cell.left.formula ? `=${cell.left.formula}` : cell.left.display }}</template>
                  </div>
                </template>
              </div>
            </div>
          </section>

          <section class="grid-panel">
            <div class="panel-heading"><strong>{{ repository ? "对比分支中的表格 · 只读" : "右侧 · 只读" }}</strong></div>
            <div v-if="repository && repository.rightState !== 'ready'" class="panel-empty">
              <div v-if="repository.rightState === 'loading' || repository.rightState === 'comparing'" class="loading-spinner small"></div>
              <strong>{{ repositoryStateMessage("right") }}</strong>
            </div>
            <div v-else ref="rightScroll" class="grid-scroll" @scroll="onScroll('right')" @wheel="onGridWheel">
              <div class="canvas" :style="{ width: `${canvasWidth}px`, height: `${totalRows * rowHeight + rowHeight}px` }">
                <template v-if="region">
                  <div
                    v-for="col in visibleColumns"
                    :key="`rh${col}`"
                    class="col-header"
                    :style="{
                      top: `${viewportTop}px`,
                      left: `${columnLeft(col)}px`,
                      width: `${columnWidth(col)}px`,
                      height: `${rowHeight}px`,
                      fontSize: `${scaledFontSize}px`
                    }"
                  >
                    {{ columnName(col) }}
                    <span class="col-resizer" @pointerdown="beginColumnResize(col, $event)"></span>
                  </div>
                  <div
                    v-for="row in visibleRows"
                    :key="`rr${row}`"
                    :class="rowClass(row)"
                    :style="{
                      top: `${row * rowHeight}px`,
                      left: `${viewportLeft}px`,
                      width: `${rowHeaderWidth}px`,
                      height: `${rowHeight}px`,
                      fontSize: `${scaledFontSize}px`
                    }"
                    @pointerdown="beginRowSelection(row, $event)"
                    @pointerenter="extendRowSelection(row)"
                    @contextmenu="openRowMenu($event, row)"
                  >
                    {{ row }}
                  </div>
                  <div
                    v-for="cell in visibleCells"
                    :key="`r${cell.row}:${cell.col}`"
                    :class="cellClass(cell)"
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
                    @contextmenu="openCellMenu($event, cell)"
                  >{{ cell.right.formula ? `=${cell.right.formula}` : cell.right.display }}</div>
                </template>
              </div>
            </div>
          </section>
        </div>

        <div v-if="summary && activeCell && selectionSize === 1" class="difference-inspector">
          <div class="difference-line left">
            <span class="difference-side">当前工作区</span>
            <strong>{{ activeCell.axis }}</strong>
            <span class="difference-value" :title="activeCell.left.raw">
              {{ displayDifferenceValue(activeCell, "left") }}
            </span>
            <span class="difference-type">{{ activeCell.left.type || "unset" }}</span>
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
        <div class="logo">UG</div>
        <h1>打开本地 Git 仓库</h1>
        <p>选择一个本地 Git 仓库，或将仓库目录拖入此窗口。</p>
        <button class="primary large" :disabled="busy" @click="chooseRepository">打开本地仓库</button>
        <div class="welcome-divider"><span>也可以</span></div>
        <button class="secondary-entry" :disabled="busy" @click="chooseFiles">选择两个表格进行对比</button>
        <small>支持直接拖入仓库目录；拖入子目录时会自动定位仓库根目录。</small>
      </div>
    </main>
    <div
      v-if="contextMenu.visible"
      class="context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @pointerdown.stop
    >
      <button
        :disabled="!copyTargets.length || busy || summary?.options.readonlyLeft || !comparisonActive"
        @click="copySelection"
      >复制{{ copyTargets.length > 1 ? ` ${copyTargets.length} 个差异` : "" }}到左侧</button>
      <span v-if="selectionSize > 1">当前选区：{{ selectionSize }} 格</span>
    </div>
    <div v-if="startupLoading" class="loading-overlay" role="status" aria-live="polite">
      <div class="loading-dialog">
        <div class="loading-spinner" aria-hidden="true"></div>
        <strong>{{ repository ? "正在加载仓库与表格" : "正在加载并比较工作簿" }}</strong>
        <span>正在读取数据并建立差异索引，请稍候…</span>
      </div>
    </div>
    <footer class="statusbar"><span>{{ statusText }}</span><span v-if="busy">处理中…</span><span v-if="summary?.warnings?.length">⚠ {{ summary.warnings.at(-1) }}</span></footer>
  </div>
</template>
