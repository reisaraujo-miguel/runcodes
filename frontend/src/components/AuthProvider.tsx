import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { AuthContext } from "@/hooks/use-auth";
import {
  checkAuth,
  logout,
  refreshSession,
  type AuthUser,
} from "@/lib/api/auth";

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

  /**
   * Ends the session. The session cookie is HttpOnly, so only the server can drop
   * it; the local user is cleared even when that request fails, since a sign-out
   * the user asked for must not leave the UI looking signed in. The route guard
   * then sends them to the login page.
   */
  const signOut = useCallback(async () => {
    try {
      await logout();
    } catch {
      // Best effort: a reload may still find the old cookie.
    }
    setUser(null);
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
    () => ({ user, isAuthenticated: user !== null, refreshAuth, signOut }),
    [user, refreshAuth, signOut],
  );

  // Show a loader while checking auth to prevent "flickering" or redirects
  if (loading) return null;

  return <AuthContext value={auth}>{children}</AuthContext>;
};
