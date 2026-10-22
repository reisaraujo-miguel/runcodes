import type { ReactNode } from "react";

import { Brand } from "@/components/app/Brand";
import { ThemeToggle } from "@/components/ThemeToggle";

/**
 * The frame shared by the signed-out email flows (verification and password
 * recovery): brand in the header, one card in the middle of the page.
 */
export function AuthShell({ children }: { children: ReactNode }) {
  return (
    <div className="bg-muted/40 flex min-h-svh flex-col">
      <header className="flex items-center justify-between p-6">
        <Brand />
        <ThemeToggle />
      </header>
      <main className="flex flex-1 items-center justify-center p-6">
        {children}
      </main>
    </div>
  );
}
