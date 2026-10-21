import { Navigate } from "react-router";

import { Brand } from "@/components/app/Brand";
import { GitHubLink } from "@/components/app/GitHubLink";
import { AboutSection } from "@/components/login/AboutSection";
import { AuthCard } from "@/components/login/AuthCard";
import { ThemeToggle } from "@/components/ThemeToggle";
import { useAuth } from "@/hooks/use-auth";

export function AuthPage() {
  const { user } = useAuth();

  // The sign-in page is only for signed-out visitors. A session restored from
  // the cookie must land on the app instead of showing a form it no longer
  // needs; `replace` keeps the login entry out of the history.
  if (user) return <Navigate to="/" replace />;

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      {/* Brand / marketing panel (desktop). */}
      <aside className="bg-muted/40 relative hidden flex-col border-r p-10 lg:flex">
        <div className="flex items-center justify-between">
          <Brand />
          <div className="flex gap-4 items-center">
            <ThemeToggle />
            <GitHubLink />
          </div>
        </div>
        <div className="mt-auto max-w-md">
          <AboutSection />
        </div>
      </aside>

      {/* Sign-in / sign-up column. */}
      <div className="flex flex-col">
        <header className="flex items-center justify-between p-6 lg:hidden">
          <Brand />
          <div className="flex gap-4 items-center">
            <ThemeToggle />
            <GitHubLink />
          </div>
        </header>
        <main className="flex flex-1 items-center justify-center p-6">
          <AuthCard />
        </main>
        <footer className="bg-muted/40 border-t p-6 lg:hidden">
          <AboutSection />
        </footer>
      </div>
    </div>
  );
}
