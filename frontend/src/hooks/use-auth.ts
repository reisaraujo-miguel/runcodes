import { createContext, use } from "react";

import type { AuthUser } from "@/lib/api/auth";

export interface AuthContextValue {
  /** The authenticated user, or null when there is no active session. */
  user: AuthUser | null;
  isAuthenticated: boolean;
  /** Re-fetches the session from the API and updates the auth state. */
  refreshAuth: () => Promise<void>;
}

export const AuthContext = createContext<AuthContextValue | undefined>(
  undefined,
);

export const useAuth = (): AuthContextValue => {
  const context = use(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};
