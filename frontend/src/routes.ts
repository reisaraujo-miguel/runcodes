import { createBrowserRouter, redirect } from "react-router";

import App from "./App.tsx";
import { ProtectedRoute } from "./routes/protected.tsx";

export const router = createBrowserRouter([
  {
    Component: App,

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
            lazy: async () => {
              const { AdminTools } = await import("./routes/admin/layout.tsx");
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
          {
            path: "/professor",
            lazy: async () => {
              const { ProfessorTools } =
                await import("./routes/professor/layout.tsx");
              return { Component: ProfessorTools };
            },
            children: [
              {
                path: "newclass",
                lazy: async () => {
                  const { NewClassModal } =
                    await import("./components/professor/NewClassModal.tsx");
                  return { Component: NewClassModal };
                },
              },
            ],
          },
        ],
      },
    ],
  },
]);
