<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { backend } from "./backend";
import { nextDiffIndex } from "./diffNav";
import { containsCell, makeRange, rangeSize, type CellPoint, type SelectionRange } from "./gridSelection";
import type { CellDiff, Region, RegionCell, Summary } from "./types";

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
const region = ref<Region | null>(null);
const sheet = ref("");
const diffs = ref<CellDiff[]>([]);
const diffIndex = ref(-1);
const activePoint = ref<CellPoint | null>(null);
const selectionAnchor = ref<CellPoint | null>(null);
const selection = ref<SelectionRange | null>(null);
const busy = ref(false);
const error = ref("");
const editValue = ref("");
const editType = ref("text");
const onlyDiffs = ref(false);
const leftScroll = ref<HTMLElement | null>(null);
const rightScroll = ref<HTMLElement | null>(null);
const viewportTop = ref(0);
const viewportLeft = ref(0);
const zoom = ref(1);
const columnWidths = ref<Record<number, number>>({});
const contextMenu = ref({ visible: false, x: 0, y: 0 });
const startupLoading = ref(true);
let loadingTimer = 0;
let regionRequest = 0;
let pendingActions = 0;
let syncing = false;
let draggingSelection = false;
let draggingRows = false;
let dragAnchor: CellPoint | null = null;
let resizeState: { col: number; startX: number; startWidth: number } | null = null;

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
const statusText = computed(() => {
  if (!summary.value) return "尚未打开工作簿";
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
    if (state?.hasSession) {
      const data = await guard(() => backend.summary());
      if (data) await acceptSummary(data, data.selectedSheet);
    }
  } finally {
    startupLoading.value = false;
  }
}

async function chooseFiles() {
  const data = await guard(() => backend.selectAndOpen());
  if (data) await acceptSummary(data, data.selectedSheet);
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

function updateEditor(cell: RegionCell | null) {
  if (!cell) return;
  editValue.value = cell.left.formula ? `=${cell.left.formula}` : cell.left.raw;
  editType.value = cell.left.formula ? "formula" : cell.left.type === "number" ? "number" : "text";
}

function setSingleSelection(cell: RegionCell) {
  const point = { row: cell.row, col: cell.col };
  activePoint.value = point;
  selectionAnchor.value = point;
  selection.value = makeRange(point, point);
  updateEditor(cell);
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
  updateEditor(cell);
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
  updateEditor(region.value?.cells.find((cell) => cell.row === row && cell.col === rowPoint.col) ?? null);
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
}

async function navigate(direction: 1 | -1) {
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
  if (!copyTargets.value.length) return;
  const focus = activePoint.value ? { ...activePoint.value } : null;
  const savedSelection = selection.value ? { ...selection.value } : null;
  contextMenu.value.visible = false;
  const data = await guard(() => backend.copyMany(sheet.value, copyTargets.value));
  if (data) {
    await acceptSummary(data, sheet.value, false);
    selection.value = savedSelection;
    activePoint.value = focus;
    updateEditor(activeCell.value);
  }
}

async function editSelected() {
  if (!activePoint.value || selectionSize.value !== 1) return;
  const row = activePoint.value.row;
  const col = activePoint.value.col;
  const data = await guard(() => backend.edit(sheet.value, row, col, editValue.value, editType.value));
  if (data) {
    await acceptSummary(data, sheet.value, false);
    const cell = region.value?.cells.find((item) => item.row === row && item.col === col);
    if (cell) setSingleSelection(cell);
  }
}

async function undo() {
  const data = await guard(() => backend.undo());
  if (data) await acceptSummary(data, sheet.value, false);
}

function onWindowKeyDown(event: KeyboardEvent) {
  if (!event.ctrlKey && !event.metaKey) return;
  const key = event.key.toLowerCase();
  if (key === "s" && event.shiftKey) {
    if (!summary.value || summary.value.options.readonlyLeft || busy.value) return;
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
  window.addEventListener("pointerup", finishPointerAction);
  window.addEventListener("pointerdown", closeContextMenu);
  window.addEventListener("keydown", onWindowKeyDown);
  initialLoad();
});

onBeforeUnmount(() => {
  window.removeEventListener("pointermove", resizeColumn);
  window.removeEventListener("pointerup", finishPointerAction);
  window.removeEventListener("pointerdown", closeContextMenu);
  window.removeEventListener("keydown", onWindowKeyDown);
});
</script>

<template>
  <div class="app-shell" :aria-busy="startupLoading">
    <header class="toolbar">
      <div class="brand">ugxlsx</div>
      <button :disabled="busy" @click="chooseFiles">打开左右文件</button>
      <span class="separator"></span>
      <button :disabled="!diffs.length || busy" @click="navigate(-1)">上一处</button>
      <button :disabled="!diffs.length || busy" @click="navigate(1)">下一处</button>
      <span class="counter">{{ diffIndex >= 0 ? diffIndex + 1 : 0 }} / {{ diffs.length }}</span>
      <button
        :disabled="!copyTargets.length || busy || summary?.options.readonlyLeft"
        class="primary"
        @click="copySelection"
      >复制{{ copyTargets.length > 1 ? ` ${copyTargets.length} 格` : "" }}到左侧</button>
      <button :disabled="!summary?.undoCount || busy" title="Ctrl/Command + Z" @click="undo">撤销</button>
      <button class="zoom-button" title="按住 Ctrl 并滚动鼠标滚轮缩放" @click="resetZoom">{{ Math.round(zoom * 100) }}%</button>
      <span class="grow"></span>
      <button
        :disabled="!summary || busy || summary.options.readonlyLeft"
        title="Ctrl/Command + Shift + S"
        @click="save(true)"
      >另存为</button>
      <button :disabled="!summary?.dirty || busy || summary.options.readonlyLeft" class="save" @click="save(false)">保存左侧</button>
    </header>

    <div v-if="error" class="error-banner">{{ error }}</div>
    <main v-if="summary" class="workspace">
      <aside class="sidebar">
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
          <button v-for="(item, index) in diffs" :key="`${item.ref.row}:${item.ref.col}`" @click="diffIndex = index; scrollTo(item.ref.row, item.ref.col)">
            {{ item.ref.row }}:{{ columnName(item.ref.col) }} · {{ item.status }}
          </button>
        </div>
      </aside>

      <section class="content">
        <div class="file-strip">
          <div><strong>{{ summary.options.leftLabel }}</strong><span>{{ summary.diff.leftFile }}</span></div>
          <div><strong>{{ summary.options.rightLabel }}</strong><span>{{ summary.diff.rightFile }}</span></div>
        </div>

        <div class="grids">
          <section class="grid-panel">
            <h2>左侧 · 可编辑</h2>
            <div ref="leftScroll" class="grid-scroll" @scroll="onScroll('left')" @wheel="onGridWheel">
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
                  >{{ cell.left.formula ? `=${cell.left.formula}` : cell.left.display }}</div>
                </template>
              </div>
            </div>
          </section>

          <section class="grid-panel">
            <h2>右侧 · 只读</h2>
            <div ref="rightScroll" class="grid-scroll" @scroll="onScroll('right')" @wheel="onGridWheel">
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

        <div class="editor">
          <strong>{{ activePoint ? `${columnName(activePoint.col)}${activePoint.row}` : "选择一个单元格" }}</strong>
          <span v-if="selectionSize > 1" class="selection-note">已选 {{ selectionSize }} 个单元格；编辑仅支持单格</span>
          <select v-model="editType" :disabled="!activePoint || selectionSize !== 1">
            <option value="text">文本</option>
            <option value="number">数字</option>
            <option value="formula">公式</option>
            <option value="clear">清空</option>
          </select>
          <input v-model="editValue" :disabled="!activePoint || selectionSize !== 1 || editType === 'clear'" placeholder="左侧单元格新值" @keyup.enter="editSelected" />
          <button :disabled="!activePoint || selectionSize !== 1 || busy || summary.options.readonlyLeft" @click="editSelected">应用编辑</button>
        </div>
      </section>
    </main>

    <main v-else class="welcome">
      <div class="welcome-card">
        <div class="logo">UG</div>
        <h1>比较并安全合并 Excel</h1>
        <p>左侧是本地工作文件，可编辑并保存；右侧始终作为只读对比来源。</p>
        <button class="primary large" :disabled="busy" @click="chooseFiles">选择左右两个 .xlsx 文件</button>
      </div>
    </main>
    <div
      v-if="contextMenu.visible"
      class="context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @pointerdown.stop
    >
      <button
        :disabled="!copyTargets.length || busy || summary?.options.readonlyLeft"
        @click="copySelection"
      >复制{{ copyTargets.length > 1 ? ` ${copyTargets.length} 个差异` : "" }}到左侧</button>
      <span v-if="selectionSize > 1">当前选区：{{ selectionSize }} 格</span>
    </div>
    <div v-if="startupLoading" class="loading-overlay" role="status" aria-live="polite">
      <div class="loading-dialog">
        <div class="loading-spinner" aria-hidden="true"></div>
        <strong>正在加载并比较工作簿</strong>
        <span>正在读取 Excel 数据并建立差异索引，请稍候…</span>
      </div>
    </div>
    <footer class="statusbar"><span>{{ statusText }}</span><span v-if="busy">处理中…</span><span v-if="summary?.warnings?.length">⚠ {{ summary.warnings.at(-1) }}</span></footer>
  </div>
</template>
