import { Navigate, Outlet } from "react-router";

import { useAuth } from "../hooks/use-auth";
import type { UserRole } from "../lib/api/auth";

export function ProtectedRoute({
  allowedRoles,
}: {
  allowedRoles?: UserRole[];
}) {
  const { user } = useAuth();

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  if (allowedRoles && !allowedRoles.includes(user.role)) {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}

/** Route gate: professors and admins only. */
export function ProfessorRoute() {
  return <ProtectedRoute allowedRoles={["professor", "admin"]} />;
}

/** Route gate: admins only. */
export function AdminRoute() {
  return <ProtectedRoute allowedRoles={["admin"]} />;
}
