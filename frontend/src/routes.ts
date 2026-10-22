import { createBrowserRouter, redirect } from "react-router";

import { RootErrorBoundary } from "@/components/RootErrorBoundary.tsx";
import App from "./App.tsx";
import { AppShell } from "./components/app/AppShell.tsx";
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
        path: "/verify-email",
        lazy: async () => {
          const { VerifyEmailPage } =
            await import("./routes/verify-email/page.tsx");
          return { Component: VerifyEmailPage };
        },
      },
      {
        path: "/forgot-password",
        lazy: async () => {
          const { ForgotPasswordPage } =
            await import("./routes/forgot-password/page.tsx");
          return { Component: ForgotPasswordPage };
        },
      },
      {
        path: "/reset-password",
        lazy: async () => {
          const { ResetPasswordPage } =
            await import("./routes/reset-password/page.tsx");
          return { Component: ResetPasswordPage };
        },
      },
      {
        Component: ProtectedRoute,
        children: [
          {
            // Everything signed in shares one shell (sidebar + top bar + footer).
            Component: AppShell,
            children: [
              {
                index: true,
                lazy: async () => {
                  const { Dashboard } = await import("./routes/home/page.tsx");
                  return { Component: Dashboard };
                },
              },
              {
                path: "exercises",
                lazy: async () => {
                  const { ExercisesPage } =
                    await import("./routes/exercises/page.tsx");
                  return { Component: ExercisesPage };
                },
              },
              {
                path: "exercises/:exerciseId/submit",
                // The submission page is a workspace: it uses the full width
                // and, on wide screens, the remaining viewport height instead
                // of the standard page column. See AppShell.
                handle: { layout: "workspace" },
                lazy: async () => {
                  const { SubmitPage } =
                    await import("./routes/exercises/submit/page.tsx");
                  return { Component: SubmitPage };
                },
              },
              {
                path: "classes",
                lazy: async () => {
                  const { ClassesPage } =
                    await import("./routes/classes/page.tsx");
                  return { Component: ClassesPage };
                },
              },
              {
                path: "classes/:offeringId",
                lazy: async () => {
                  const { OfferingPage } =
                    await import("./routes/classes/offering/page.tsx");
                  return { Component: OfferingPage };
                },
              },
              {
                path: "profile",
                lazy: async () => {
                  const { ProfilePage } =
                    await import("./routes/profile/page.tsx");
                  return { Component: ProfilePage };
                },
              },
              {
                path: "professor",
                Component: ProfessorRoute,
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
              {
                path: "admin",
                Component: AdminRoute,
                children: [
                  {
                    index: true,
                    lazy: async () => {
                      const { AdminDashboard } =
                        await import("./routes/admin/dashboard/page.tsx");
                      return { Component: AdminDashboard };
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
                    path: "courses",
                    lazy: async () => {
                      const { AdminCoursesPage } =
                        await import("./routes/admin/courses/page.tsx");
                      return { Component: AdminCoursesPage };
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
        ],
      },
    ],
  },
]);
