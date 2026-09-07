// Keep logical worksheet coordinates; only scroll positions omit pinned items.
// This avoids allocating an index for every worksheet row.
export function bodyIndex(index: number, frozen: readonly number[]): number {
  return index - frozen.filter((item) => item < index).length;
}

export function logicalIndex(index: number, frozen: readonly number[]): number {
  let logical = Math.max(1, index);
  for (const item of frozen) {
    if (item > logical) break;
    logical++;
  }
  return logical;
}

export function bodyWindow(from: number, count: number, total: number, frozen: readonly number[]): number[] {
  const excluded = new Set(frozen);
  const indexes: number[] = [];
  for (let index = Math.max(1, from); index <= total && indexes.length < count; index++) {
    if (!excluded.has(index)) indexes.push(index);
  }
  return indexes;
}
