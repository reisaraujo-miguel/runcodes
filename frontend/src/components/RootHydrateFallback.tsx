import Logo from "@/assets/runcodes-logo/logo.png";
import LogoBlue from "@/assets/runcodes-logo/logoblue.png";

import { useTheme } from "@/hooks/use-theme";

/**
 * Shown on the first client render while React Router loads the lazy route
 * modules. Without a HydrateFallback the router logs a warning during
 * initial hydration and renders nothing until the lazy chunks are ready.
 */
export function RootHydrateFallback() {
  const isDark = useTheme() === "dark";

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-6">
        <img
          src={isDark ? Logo : LogoBlue}
          alt="RunCodes Logo"
          className="h-10"
        />
        <div
          aria-label="Loading"
          className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-muted-foreground"
          role="status"
        />
      </div>
    </div>
  );
}
