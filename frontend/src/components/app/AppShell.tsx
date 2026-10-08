import { useState } from "react";
import { NavLink, Outlet } from "react-router";

import { MenuIcon } from "lucide-react";

import { Brand } from "@/components/app/Brand";
import { UserMenu } from "@/components/app/UserMenu";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { useAuth } from "@/hooks/use-auth";
import type { NavSection } from "@/lib/nav";
import { navSectionsFor } from "@/lib/nav";
import { cn } from "@/lib/utils";

/** The sidebar links, shared by the desktop rail and the mobile drawer. */
function SidebarNav({
  sections,
  onNavigate,
}: {
  sections: NavSection[];
  onNavigate?: () => void;
}) {
  return (
    <nav className="flex-1 space-y-6 overflow-y-auto px-3 py-4">
      {sections.map((section, index) => (
        <div key={section.label ?? `section-${String(index)}`}>
          {section.label ? (
            <p className="text-muted-foreground px-3 pb-2 text-xs font-semibold tracking-wider uppercase">
              {section.label}
            </p>
          ) : null}
          <ul className="space-y-1">
            {section.items.map((item) => (
              <li key={item.to}>
                <NavLink
                  to={item.to}
                  end={item.end}
                  onClick={onNavigate}
                  className={({ isActive }) =>
                    cn(
                      "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                      isActive
                        ? "bg-primary/10 text-primary"
                        : "text-muted-foreground hover:bg-accent hover:text-foreground",
                    )
                  }
                >
                  <item.icon className="size-4.5 shrink-0" aria-hidden />
                  {item.label}
                </NavLink>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </nav>
  );
}

/** The full sidebar body: brand, navigation and the account control. */
function SidebarBody({
  sections,
  onNavigate,
}: {
  sections: NavSection[];
  onNavigate?: () => void;
}) {
  return (
    <div className="bg-sidebar flex h-full flex-col">
      <div className="flex h-16 items-center border-b px-5">
        <Brand />
      </div>
      <SidebarNav sections={sections} onNavigate={onNavigate} />
      <div className="space-y-1 border-t p-3">
        <UserMenu />
      </div>
    </div>
  );
}

/**
 * The authenticated application shell: a fixed sidebar on desktop, a slide-in
 * drawer on mobile, and a content column that pages render into. Keeping the
 * chrome in one layout means pages never re-declare the navbar or footer.
 */
export function AppShell() {
  const { user } = useAuth();
  const [menuOpen, setMenuOpen] = useState(false);
  const sections = navSectionsFor(user?.role ?? "student");

  return (
    <div className="bg-muted/30 min-h-svh">
      <aside className="fixed inset-y-0 left-0 z-40 hidden w-64 border-r lg:block">
        <SidebarBody sections={sections} />
      </aside>

      <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
        <SheetContent
          side="left"
          showCloseButton={false}
          className="bg-sidebar w-72 gap-0 p-0"
        >
          <SheetTitle className="sr-only">Menu de navegação</SheetTitle>
          <SidebarBody
            sections={sections}
            onNavigate={() => {
              setMenuOpen(false);
            }}
          />
        </SheetContent>
      </Sheet>

      <div className="lg:pl-64">
        <header className="bg-background/80 sticky top-0 z-30 flex h-16 items-center gap-2 border-b px-4 backdrop-blur lg:hidden">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Abrir menu"
            onClick={() => {
              setMenuOpen(true);
            }}
          >
            <MenuIcon />
          </Button>
          <Brand />
          <div className="ml-auto">
            <ThemeToggle />
          </div>
        </header>

        <main className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
