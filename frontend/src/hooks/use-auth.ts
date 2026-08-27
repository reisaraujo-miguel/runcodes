import { createContext, use } from "react";

export interface AuthContextValue {
  isAuthenticated: boolean;
  /** Update the auth state after a successful login/logout. */
  setAuthenticated: (value: boolean) => void;
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
