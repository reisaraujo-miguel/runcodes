import "./index.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createBrowserRouter, RouterProvider } from "react-router";

import App from "./App.tsx";

const rootElement = document.getElementById("root");

const router = createBrowserRouter([
  {
    Component: App,
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
    ],
  },
]);

if (rootElement) {
  createRoot(rootElement).render(
    <StrictMode>
      <RouterProvider router={router} />
    </StrictMode>,
  );
}
