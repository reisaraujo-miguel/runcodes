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
            path: "/profile",
            lazy: async () => {
              const { ProfilePage } = await import("./routes/profile/page.tsx");
              return { Component: ProfilePage };
            },
          },
          {
            path: "/exercises/:exerciseId/submit",
            lazy: async () => {
              const { SubmitPage } =
                await import("./routes/exercises/submit/page.tsx");
              return { Component: SubmitPage };
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
                  {
                    path: "courses",
                    lazy: async () => {
                      const { AdminCoursesPage } =
                        await import("./routes/admin/courses/page.tsx");
                      return { Component: AdminCoursesPage };
                    },
                  },
                  {
                    path: "users",
                    lazy: async () => {
                      const { AdminUsersPage } =
                        await import("./routes/admin/users/page.tsx");
                      return { Component: AdminUsersPage };
                    },
                  },
                  {
                    path: "settings",
                    lazy: async () => {
                      const { AdminSettingsPage } =
                        await import("./routes/admin/settings/page.tsx");
                      return { Component: AdminSettingsPage };
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
                    index: true,
                    lazy: async () => {
                      const { ProfessorClassesPage } =
                        await import("./routes/professor/page.tsx");
                      return { Component: ProfessorClassesPage };
                    },
                  },
                  {
                    path: "class/:offeringId",
                    lazy: async () => {
                      const { ClassPage } =
                        await import("./routes/professor/class/page.tsx");
                      return { Component: ClassPage };
                    },
                  },
                  {
                    path: "exercise/:exerciseId",
                    lazy: async () => {
                      const { ExercisePage } =
                        await import("./routes/professor/exercise/page.tsx");
                      return { Component: ExercisePage };
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
