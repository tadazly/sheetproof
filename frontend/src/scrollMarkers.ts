import type { RowStatus } from "./types";

export interface ScrollMarker {
  position: number;
  status: RowStatus;
}

const STATUS_PRIORITY: Record<RowStatus, number> = {
  unchanged: 0,
  added: 1,
  deleted: 2,
  modified: 3,
  conflict: 4
};

const STATUS_COLOR: Record<Exclude<RowStatus, "unchanged">, string> = {
  added: "var(--diff-added)",
  deleted: "var(--diff-deleted)",
  modified: "var(--diff-modified)",
  conflict: "var(--diff-conflict)"
};

/**
 * Builds a bounded-complexity scrollbar track gradient. Many cell differences
 * may land in the same visual pixel, so markers are aggregated into bins and
 * the highest-priority semantic color wins. This keeps CSS and paint cost
 * stable even when a sheet contains thousands of differences.
 */
export function scrollMarkerGradient(
  direction: "right" | "bottom",
  markers: readonly ScrollMarker[],
  maxBins = 180
): string {
  const binCount = Math.max(1, Math.floor(maxBins));
  const bins = new Map<number, Exclude<RowStatus, "unchanged">>();
  for (const marker of markers) {
    if (marker.status === "unchanged" || !Number.isFinite(marker.position)) continue;
    const position = Math.min(1, Math.max(0, marker.position));
    const bin = Math.min(binCount - 1, Math.floor(position * binCount));
    const current = bins.get(bin);
    if (!current || STATUS_PRIORITY[marker.status] > STATUS_PRIORITY[current]) {
      bins.set(bin, marker.status);
    }
  }
  if (!bins.size) return "none";

  const stops: string[] = [];
  let cursor = 0;
  for (const [bin, status] of [...bins.entries()].sort(([left], [right]) => left - right)) {
    const start = (bin / binCount) * 100;
    const end = ((bin + 1) / binCount) * 100;
    if (start > cursor) {
      stops.push(`transparent ${formatPercent(cursor)}`, `transparent ${formatPercent(start)}`);
    }
    stops.push(`${STATUS_COLOR[status]} ${formatPercent(start)}`, `${STATUS_COLOR[status]} ${formatPercent(end)}`);
    cursor = end;
  }
  if (cursor < 100) {
    stops.push(`transparent ${formatPercent(cursor)}`, "transparent 100%");
  }
  return `linear-gradient(to ${direction}, ${stops.join(", ")})`;
}

function formatPercent(value: number): string {
  return `${Number(value.toFixed(3))}%`;
}
