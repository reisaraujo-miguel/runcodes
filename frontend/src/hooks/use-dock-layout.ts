import { useCallback, useEffect, useState } from "react";

import {
  DEFAULT_DOCK_LAYOUT,
  normalizeDockLayout,
  type DockLayout,
} from "@/lib/dock-layout";
import { clamp } from "@/lib/utils";

const STORAGE_KEY = "runcodes:submission-docks:v1";

function readStoredLayout(): DockLayout {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    return normalizeDockLayout(stored ? JSON.parse(stored) : null);
  } catch {
    return normalizeDockLayout(null);
  }
}

/**
 * Persists the split positions and collapsed panes of the submission
 * workspace. Writes are debounced so a pointer drag only touches storage once
 * it settles.
 */
export function useDockLayout() {
  const [layout, setLayout] = useState<DockLayout>(readStoredLayout);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(layout));
      } catch {
        // Storage unavailable (private mode): the layout still works for the
        // current visit, it just does not survive a reload.
      }
    }, 250);
    return () => {
      window.clearTimeout(timer);
    };
  }, [layout]);

  const setHorizontal = useCallback((value: number) => {
    setLayout((previous) =>
      Math.abs(previous.horizontal - value) < 0.0005
        ? previous
        : { ...previous, horizontal: clamp(value, 0, 1) },
    );
  }, []);

  const setVertical = useCallback((value: number) => {
    setLayout((previous) =>
      Math.abs(previous.vertical - value) < 0.0005
        ? previous
        : { ...previous, vertical: clamp(value, 0, 1) },
    );
  }, []);

  const resetHorizontal = useCallback(() => {
    setLayout((previous) => ({
      ...previous,
      horizontal: DEFAULT_DOCK_LAYOUT.horizontal,
    }));
  }, []);

  const resetVertical = useCallback(() => {
    setLayout((previous) => ({
      ...previous,
      vertical: DEFAULT_DOCK_LAYOUT.vertical,
    }));
  }, []);

  const toggleStatement = useCallback(() => {
    setLayout((previous) => ({
      ...previous,
      collapsed: {
        ...previous.collapsed,
        statement: !previous.collapsed.statement,
      },
    }));
  }, []);

  const toggleResults = useCallback(() => {
    setLayout((previous) => ({
      ...previous,
      collapsed: {
        ...previous.collapsed,
        results: !previous.collapsed.results,
      },
    }));
  }, []);

  return {
    layout,
    setHorizontal,
    setVertical,
    resetHorizontal,
    resetVertical,
    toggleStatement,
    toggleResults,
  };
}
