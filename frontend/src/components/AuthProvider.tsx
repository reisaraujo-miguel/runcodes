import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { AuthContext } from "@/hooks/use-auth";
import { checkAuth, refreshSession, type AuthUser } from "@/lib/api/auth";

/** Refresh the session this long before it expires. */
const REFRESH_MARGIN_MS = 5 * 60 * 1000;
/** How often to check whether the session is close to expiring. */
const REFRESH_CHECK_INTERVAL_MS = 60 * 1000;

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);

  const refreshAuth = useCallback(async () => {
    try {
      setUser(await checkAuth());
    } catch {
      setUser(null);
    }
  }, []);

  const refreshUser = useCallback(async () => {
    try {
      setUser(await refreshSession());
    } catch {
      // The session can no longer be renewed — log the user out.
      setUser(null);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;

    async function initAuth() {
      await refreshAuth();
      if (!cancelled) setLoading(false);
    }

    void initAuth();

    return () => {
      cancelled = true;
    };
  }, [refreshAuth]);

  // Sliding session: while the app stays open, renew the token shortly before
  // it expires. If the renewal fails (e.g. the user was idle past the hard
  // expiry), the user is logged out.
  useEffect(() => {
    if (!user) return;

    const interval = setInterval(() => {
      const remaining = user.expires_at * 1000 - Date.now();
      if (remaining <= REFRESH_MARGIN_MS) {
        void refreshUser();
      }
    }, REFRESH_CHECK_INTERVAL_MS);

    return () => {
      clearInterval(interval);
    };
  }, [user, refreshUser]);

  const auth = useMemo(
    () => ({ user, isAuthenticated: user !== null, refreshAuth }),
    [user, refreshAuth],
  );

  // Show a loader while checking auth to prevent "flickering" or redirects
  if (loading) return null;

  return <AuthContext value={auth}>{children}</AuthContext>;
};
