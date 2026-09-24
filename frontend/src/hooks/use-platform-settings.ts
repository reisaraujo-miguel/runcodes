import { useEffect, useState } from "react";

import {
  FALLBACK_SETTINGS,
  getPublicSettings,
  type PlatformSettings,
} from "@/lib/api/settings";

/**
 * The contact address and disclaimer rendered on the public pages, which an
 * admin edits in the database.
 *
 * The build-time fallback is used first and replaced once the API answers. The
 * login page must render even when the API is unreachable, so a failed request
 * silently keeps the fallback instead of surfacing an error.
 */
export function usePlatformSettings(): PlatformSettings {
  const [settings, setSettings] = useState<PlatformSettings>(FALLBACK_SETTINGS);

  useEffect(() => {
    let cancelled = false;

    async function loadSettings() {
      try {
        const data = await getPublicSettings();
        if (!cancelled) setSettings(data);
      } catch {
        // Keep the fallback: the configured values are unavailable.
      }
    }

    void loadSettings();

    return () => {
      cancelled = true;
    };
  }, []);

  return settings;
}
