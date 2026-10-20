import { useCallback, useState } from "react";

export interface ElementSize {
  width: number;
  height: number;
}

/**
 * Measures an element and keeps the measurement up to date while it resizes.
 *
 * The ref is a callback so the observer is attached as soon as the node
 * mounts, and disconnected by the cleanup React 19 runs on unmount. Updates
 * arrive through the observer rather than a synchronous effect, which keeps
 * the first render free of cascading state writes.
 */
export function useElementSize(): readonly [
  (node: HTMLElement | null) => (() => void) | undefined,
  ElementSize,
] {
  const [size, setSize] = useState<ElementSize>({ width: 0, height: 0 });

  const ref = useCallback((node: HTMLElement | null) => {
    if (!node) return;

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) return;
      const { width, height } = entry.contentRect;
      setSize((previous) =>
        previous.width === width && previous.height === height
          ? previous
          : { width, height },
      );
    });
    observer.observe(node);

    return () => {
      observer.disconnect();
    };
  }, []);

  return [ref, size] as const;
}
