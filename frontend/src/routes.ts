import { createBrowserRouter, redirect } from "react-router";

import { RootErrorBoundary } from "@/components/RootErrorBoundary.tsx";
import App from "./App.tsx";
import { RootHydrateFallback } from "./components/RootHydrateFallback.tsx";
import {
  AdminRoute,
  ProfessorRoute,
  ProtectedRoute,
} from "./routes/protected.tsx";

export const router = createBrowserRouter([
  {
    Component: App,
    ErrorBoundary: RootErrorBoundary,
    HydrateFallback: RootHydrateFallback,

    children: [
      {
        path: "/login",
        lazy: async () => {
          const { AuthPage } = await import("./routes/login/page.tsx");
          return { Component: AuthPage };
        },
      },
      {
        path: "/signup",
        loader: () => redirect("/login?mode=signup"),
      },
      {
        Component: ProtectedRoute,
        children: [
          {
            index: true,
            lazy: async () => {
              const { Home } = await import("./routes/home/page.tsx");
              return { Component: Home };
            },
          },
          {
            path: "/admin",
            Component: AdminRoute,
            children: [
              {
                lazy: async () => {
                  const { AdminTools } =
                    await import("./routes/admin/layout.tsx");
                  return { Component: AdminTools };
                },
                children: [
                  {
                    index: true,
                    lazy: async () => {
                      const { Dashboard } =
                        await import("./routes/admin/dashboard/page.tsx");
                      return { Component: Dashboard };
                    },
                  },
                ],
              },
            ],
          },
          {
            path: "/professor",
            Component: ProfessorRoute,
            children: [
              {
                lazy: async () => {
                  const { ProfessorTools } =
                    await import("./routes/professor/layout.tsx");
                  return { Component: ProfessorTools };
                },
                children: [
                  {
                    path: "class/:offeringId",
                    lazy: async () => {
                      const { ClassPage } =
                        await import("./routes/professor/class/page.tsx");
                      return { Component: ClassPage };
                    },
                  },
                ],
              },
            ],
          },
        ],
      },
    ],
  },
]);
