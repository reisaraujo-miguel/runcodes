import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import { AuthContext } from "@/hooks/use-auth";
import { checkAuth } from "@/lib/api/auth";

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    checkAuth()
      .then(() => {
        setIsAuthenticated(true);
      })
      .catch(() => {
        setIsAuthenticated(false);
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

  const auth = useMemo(
    () => ({ isAuthenticated, setAuthenticated: setIsAuthenticated }),
    [isAuthenticated],
  );

  // Show a loader while checking auth to prevent "flickering" or redirects
  if (loading) return null;

  return <AuthContext value={auth}>{children}</AuthContext>;
};
