import { useEffect, useState } from "react";

const DEFAULT_INTERVAL_MS = 60_000;

/**
 * A coarse current timestamp that refreshes periodically.
 *
 * Deadline-aware UI reads this instead of calling `Date.now()` during render, so
 * components stay pure while a page left open still flips to "Encerrado" when a
 * deadline passes.
 */
export function useNow(intervalMs = DEFAULT_INTERVAL_MS): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = window.setInterval(() => {
      setNow(Date.now());
    }, intervalMs);

    return () => {
      window.clearInterval(id);
    };
  }, [intervalMs]);

  return now;
}
