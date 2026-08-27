import { useSyncExternalStore } from "react";

import { getTheme, subscribeToTheme, type Theme } from "@/lib/theme";

export function useTheme(): Theme {
  return useSyncExternalStore(subscribeToTheme, getTheme);
}
