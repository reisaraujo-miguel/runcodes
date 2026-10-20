import { describe, expect, test } from "bun:test";

import {
  DEFAULT_DOCK_LAYOUT,
  dockBounds,
  normalizeDockLayout,
} from "./dock-layout";

describe("dockBounds", () => {
  test("turns both pixel minima into limits for the first dock", () => {
    const { min, max } = dockBounds(1242, 280, 380);
    expect(min).toBeCloseTo(280 / 1242);
    expect(max).toBeCloseTo(1 - 380 / 1242);
    // Both docks keep at least their configured width at the limits.
    expect(min * 1242).toBeCloseTo(280);
    expect((1 - max) * 1242).toBeCloseTo(380);
  });

  test("keeps the limits ordered when the container is tight", () => {
    const { min, max } = dockBounds(1000, 280, 380);
    expect(min).toBeLessThan(max);
    expect(min * 1000).toBeGreaterThanOrEqual(280);
    expect((1 - max) * 1000).toBeGreaterThanOrEqual(380);
  });

  test("falls back to an even split when both minima do not fit", () => {
    expect(dockBounds(500, 280, 380)).toEqual({ min: 0.5, max: 0.5 });
  });

  test("returns a permissive range before the container is measured", () => {
    expect(dockBounds(0, 280, 380)).toEqual({ min: 0.2, max: 0.8 });
  });

  test("keeps very large containers within the 10-90% range", () => {
    const { min, max } = dockBounds(10_000, 280, 380);
    expect(min).toBe(0.1);
    expect(max).toBe(0.9);
  });
});

describe("normalizeDockLayout", () => {
  test("fills collapsed flags for a layout saved before panes were collapsible", () => {
    expect(normalizeDockLayout({ horizontal: 0.6, vertical: 0.4 })).toEqual({
      horizontal: 0.6,
      vertical: 0.4,
      collapsed: { statement: false, editor: false, results: false },
    });
  });

  test("keeps valid collapsed flags and ignores invalid ones", () => {
    const layout = normalizeDockLayout({
      horizontal: 0.4,
      vertical: 0.6,
      collapsed: { statement: true, editor: "yes", results: true },
    });
    expect(layout.collapsed).toEqual({
      statement: true,
      editor: false,
      results: true,
    });
  });

  test("falls back independently for corrupt storage", () => {
    expect(normalizeDockLayout(null)).toEqual(DEFAULT_DOCK_LAYOUT);
    expect(normalizeDockLayout("not a layout")).toEqual(DEFAULT_DOCK_LAYOUT);
    expect(normalizeDockLayout({ horizontal: 4 })).toEqual({
      ...DEFAULT_DOCK_LAYOUT,
      horizontal: 0.9,
    });
  });
});
