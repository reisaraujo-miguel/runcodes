import { Navigate, Outlet } from "react-router";
import { useAuth } from "../hooks/use-auth";

export function ProtectedRoute() {
  const isAuthenticated = useAuth();

  console.log("auth is %s", isAuthenticated);

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
