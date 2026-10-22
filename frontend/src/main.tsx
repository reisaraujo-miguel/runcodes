import "./index.css";
// Applies the stored/system theme to <html> before React renders anything.
import "@/lib/theme";

import { AuthProvider } from "@/components/AuthProvider.tsx";
import { browserTelemetryConfig } from "@/lib/telemetry/config";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router";
import { router } from "./routes";

async function bootstrap() {
  // The OpenTelemetry browser SDK is a sizeable dependency. Import it only in
  // builds that enable tracing, and await it before mounting React so the auth
  // provider's first API call is already part of a trace.
  if (browserTelemetryConfig(import.meta.env)) {
    try {
      const { initBrowserTelemetry } = await import("@/lib/telemetry");
      initBrowserTelemetry();
    } catch (error) {
      console.warn("OpenTelemetry browser tracing disabled:", error);
    }
  }

  const rootElement = document.getElementById("root");

  if (rootElement) {
    createRoot(rootElement).render(
      <StrictMode>
        <AuthProvider>
          <RouterProvider router={router} />
        </AuthProvider>
      </StrictMode>,
    );
  }
}

void bootstrap();
