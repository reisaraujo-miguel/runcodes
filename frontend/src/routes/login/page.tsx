import { Brand } from "@/components/app/Brand";
import { AboutSection } from "@/components/login/AboutSection";
import { AuthCard } from "@/components/login/AuthCard";
import { ThemeToggle } from "@/components/ThemeToggle";

export function AuthPage() {
  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      {/* Brand / marketing panel (desktop). */}
      <aside className="bg-muted/40 relative hidden flex-col border-r p-10 lg:flex">
        <div className="flex items-center justify-between">
          <Brand />
          <ThemeToggle />
        </div>
        <div className="mt-auto max-w-md">
          <AboutSection />
        </div>
      </aside>

      {/* Sign-in / sign-up column. */}
      <div className="flex flex-col">
        <header className="flex items-center justify-between p-6 lg:hidden">
          <Brand />
          <ThemeToggle />
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
