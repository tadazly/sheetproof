import { describe, expect, it } from "vitest";
import { scrollMarkerGradient } from "./scrollMarkers";

describe("scrollMarkerGradient", () => {
  it("uses the existing semantic colors on both scrollbar axes", () => {
    const markers = [
      { position: 0.1, status: "added" as const },
      { position: 0.3, status: "deleted" as const },
      { position: 0.6, status: "modified" as const },
      { position: 0.9, status: "conflict" as const }
    ];

    const vertical = scrollMarkerGradient("bottom", markers, 100);
    const horizontal = scrollMarkerGradient("right", markers, 100);

    expect(vertical).toContain("linear-gradient(to bottom");
    expect(horizontal).toContain("linear-gradient(to right");
    expect(vertical).toContain("var(--diff-added)");
    expect(vertical).toContain("var(--diff-deleted)");
    expect(vertical).toContain("var(--diff-modified)");
    expect(vertical).toContain("var(--diff-conflict)");
  });

  it("bounds gradient complexity and keeps the strongest status in a crowded bin", () => {
    const markers = Array.from({ length: 10_000 }, (_, index) => ({
      position: index / 9_999,
      status: index === 5_000 ? "conflict" as const : "added" as const
    }));

    const gradient = scrollMarkerGradient("bottom", markers, 64);

    expect(gradient).toContain("var(--diff-conflict)");
    expect(gradient.split(",")).toHaveLength(129);
    expect(gradient.length).toBeLessThan(8_000);
  });

  it("omits unchanged and invalid positions", () => {
    expect(scrollMarkerGradient("bottom", [
      { position: Number.NaN, status: "modified" },
      { position: 0.5, status: "unchanged" }
    ])).toBe("none");
  });
});
