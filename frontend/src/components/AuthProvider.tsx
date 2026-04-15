import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import { AuthContext } from "@/hooks/use-auth";

const API_BASE_URL = import.meta.env.VITE_API_ENDPOINT as string;

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);

  const checkAuth = async () => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/auth`, {
        credentials: "include",
      });

      if (res.ok) {
        setIsAuthenticated(true);
      } else {
        setIsAuthenticated(false);
      }
    } catch {
      setIsAuthenticated(false);
    } finally {
      setLoading(false);
      console.log("hello");
    }
  };

  useEffect(() => {
    void checkAuth();
  }, []);

  const auth = useMemo(() => isAuthenticated, [isAuthenticated]);

  // Show a loader while checking auth to prevent "flickering" or redirects
  if (loading) return null;

  return <AuthContext value={auth}>{children}</AuthContext>;
};
