import { Brand } from "@/components/app/Brand";
import { Spinner } from "@/components/ui/spinner";

/**
 * Shown on the first client render while React Router loads the lazy route
 * modules. Without a HydrateFallback the router logs a warning during
 * initial hydration and renders nothing until the lazy chunks are ready.
 */
export function RootHydrateFallback() {
  return (
    <div className="bg-background flex min-h-svh flex-col items-center justify-center gap-6">
      <Brand />
      <Spinner className="text-muted-foreground size-6" label="Carregando" />
    </div>
  );
}
