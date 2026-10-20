import { clamp } from "@/lib/utils";

/**
 * Thickness of a dock splitter's drag strip. The page reserves the same gap
 * between the docks so dragging never changes the overall layout size.
 */
export const DOCK_SPLITTER_SIZE = 12;

/** Width of the rail a collapsed statement dock leaves behind. */
export const STATEMENT_RAIL_WIDTH = 40;

/** Minimum dock sizes, in CSS pixels, kept usable on a 1366px laptop. */
export const MIN_STATEMENT_WIDTH = 280;
export const MIN_RIGHT_WIDTH = 380;
export const MIN_EDITOR_HEIGHT = 220;
export const MIN_RESULTS_HEIGHT = 240;

export interface DockBounds {
  min: number;
  max: number;
}

/**
 * Converts the minimum pixel sizes of both docks into ratios of the space they
 * share. Recomputing this from the measured container means a stored split can
 * never squeeze a dock below its usable size after a window resize.
 */
export function dockBounds(
  available: number,
  firstMin: number,
  secondMin: number,
): DockBounds {
  if (available <= 0) return { min: 0.2, max: 0.8 };

  const min = clamp(firstMin / available, 0.1, 0.9);
  const max = clamp(1 - secondMin / available, 0.1, 0.9);
  return min < max ? { min, max } : { min: 0.5, max: 0.5 };
}

export interface DockCollapsed {
  statement: boolean;
  editor: boolean;
  results: boolean;
}

export interface DockLayout {
  /** Share of the width taken by the exercise panel. */
  horizontal: number;
  /** Share of the height taken by the editor panel. */
  vertical: number;
  collapsed: DockCollapsed;
}

export const DEFAULT_DOCK_LAYOUT: DockLayout = {
  horizontal: 0.475,
  vertical: 0.575,
  collapsed: { statement: false, editor: false, results: false },
};

function readRatio(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value)
    ? clamp(value, 0.1, 0.9)
    : fallback;
}

function readCollapsed(value: unknown): DockCollapsed {
  const record =
    typeof value === "object" && value !== null
      ? (value as Record<string, unknown>)
      : {};
  return {
    statement: record.statement === true,
    editor: record.editor === true,
    results: record.results === true,
  };
}

/**
 * Turns a value read from storage into a complete layout. Unknown or corrupt
 * fields fall back independently so adding a new dock never invalidates the
 * positions a user already saved.
 */
export function normalizeDockLayout(value: unknown): DockLayout {
  const record =
    typeof value === "object" && value !== null
      ? (value as Record<string, unknown>)
      : {};
  return {
    horizontal: readRatio(record.horizontal, DEFAULT_DOCK_LAYOUT.horizontal),
    vertical: readRatio(record.vertical, DEFAULT_DOCK_LAYOUT.vertical),
    collapsed: readCollapsed(record.collapsed),
  };
}
