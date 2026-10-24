import { useEffect, useState } from "react";
import { NavLink, Outlet, useMatches } from "react-router";

import { MenuIcon, PanelLeftCloseIcon, PanelLeftOpenIcon } from "lucide-react";

import { Brand } from "@/components/app/Brand";
import { PreviewBanner } from "@/components/app/PreviewBanner";
import { UserMenu } from "@/components/app/UserMenu";
import { ThemeToggle } from "@/components/ThemeToggle";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useAuth } from "@/hooks/use-auth";
import type { NavSection } from "@/lib/nav";
import { navSectionsFor } from "@/lib/nav";
import { cn } from "@/lib/utils";
import { GitHubLink } from "./GitHubLink";

const SIDEBAR_COLLAPSED_KEY = "sidebar-collapsed";

/**
 * A route asks for the workspace layout by declaring
 * `handle: { layout: "workspace" }`. Workspace pages drop the standard column
 * width and, on wide screens, fill the remaining viewport height so their
 * panels can scroll internally instead of the page.
 */
interface WorkspaceHandle {
  layout?: "workspace";
}

function isWorkspaceHandle(handle: unknown): handle is WorkspaceHandle {
  return (
    typeof handle === "object" &&
    handle !== null &&
    "layout" in handle &&
    (handle as { layout?: unknown }).layout === "workspace"
  );
}

/** The persisted compact-rail preference, defaulting to expanded. */
function readSidebarCollapsed(): boolean {
  try {
    return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "true";
  } catch {
    // localStorage may be unavailable (e.g., in private browsing)
    return false;
  }
}

/** The sidebar links, shared by the desktop rail and the mobile drawer. */
function SidebarNav({
  sections,
  collapsed = false,
  onNavigate,
}: {
  sections: NavSection[];
  /** Icon-only rail: labels are hidden and shown as a title on hover. */
  collapsed?: boolean;
  onNavigate?: () => void;
}) {
  return (
    <nav
      aria-label="Navegação principal"
      className={cn(
        "flex-1 space-y-6 overflow-y-auto py-4",
        collapsed ? "px-2" : "px-3",
      )}
    >
      {sections.map((section, index) => (
        <div key={section.label ?? `section-${String(index)}`}>
          {section.label && !collapsed ? (
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
                  // The label is the only accessible name once it is hidden.
                  aria-label={collapsed ? item.label : undefined}
                  title={collapsed ? item.label : undefined}
                  className={({ isActive }) =>
                    cn(
                      "flex items-center gap-3 rounded-lg text-sm font-medium transition-colors",
                      collapsed ? "justify-center px-2 py-2" : "px-3 py-2",
                      isActive
                        ? "bg-primary/10 text-primary"
                        : "text-muted-foreground hover:bg-accent hover:text-foreground",
                    )
                  }
                >
                  <item.icon className="size-4.5 shrink-0" aria-hidden />
                  {collapsed ? null : item.label}
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
  collapsed = false,
  onToggleCollapsed,
  onNavigate,
}: {
  sections: NavSection[];
  collapsed?: boolean;
  /** Provided only for the desktop rail; the mobile drawer cannot collapse. */
  onToggleCollapsed?: () => void;
  onNavigate?: () => void;
}) {
  return (
    <div className="bg-sidebar flex h-full flex-col">
      <div
        className={cn(
          "flex h-16 items-center border-b",
          collapsed ? "justify-center px-2" : "justify-between px-5 gap-4",
        )}
      >
        {collapsed ? null : <Brand />}
        {onToggleCollapsed ? (
          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground shrink-0"
                  aria-label={collapsed ? "Expandir menu" : "Recolher menu"}
                  onClick={onToggleCollapsed}
                />
              }
            >
              {collapsed ? <PanelLeftOpenIcon /> : <PanelLeftCloseIcon />}
            </TooltipTrigger>
            <TooltipContent side="right">
              {collapsed ? "Expandir menu" : "Recolher menu"}
            </TooltipContent>
          </Tooltip>
        ) : null}
      </div>
      <SidebarNav
        sections={sections}
        collapsed={collapsed}
        onNavigate={onNavigate}
      />
      <div className={cn("border-t", collapsed ? "p-2" : "p-3")}>
        {collapsed ? (
          <div className="flex flex-col items-center gap-1">
            <GitHubLink />
            <ThemeToggle />
            <UserMenu collapsed />
          </div>
        ) : (
          <div className="flex items-center gap-2">
            <UserMenu className="min-w-0 flex-1" />
            <ThemeToggle />
            <GitHubLink />
          </div>
        )}
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
  const [collapsed, setCollapsed] = useState(readSidebarCollapsed);
  const sections = navSectionsFor(user?.role ?? "student");
  const workspaceLayout = useMatches().some((match) =>
    isWorkspaceHandle(match.handle),
  );

  useEffect(() => {
    try {
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(collapsed));
    } catch {
      // localStorage may be unavailable (e.g., in private browsing)
    }
  }, [collapsed]);

  return (
    <div className="bg-muted/30 min-h-svh">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:bg-background focus:ring-ring focus:absolute focus:top-4 focus:left-4 focus:z-50 focus:rounded-md focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:ring-2"
      >
        Pular para o conteúdo
      </a>

      <aside
        aria-label="Barra lateral"
        className={cn(
          "fixed inset-y-0 left-0 z-40 hidden border-r transition-[width] duration-200 ease-out lg:block",
          collapsed ? "w-16" : "w-64",
        )}
      >
        <SidebarBody
          sections={sections}
          collapsed={collapsed}
          onToggleCollapsed={() => {
            setCollapsed((value) => !value);
          }}
        />
      </aside>

      <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
        <SheetContent
          id="mobile-navigation"
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

      <div
        className={cn(
          "transition-[padding] duration-200 ease-out",
          collapsed ? "lg:pl-16" : "lg:pl-64",
        )}
      >
        <header className="bg-background/80 sticky top-0 z-30 flex h-16 items-center gap-2 border-b px-4 backdrop-blur lg:hidden">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Abrir menu"
            aria-expanded={menuOpen}
            aria-controls="mobile-navigation"
            aria-haspopup="dialog"
            onClick={() => {
              setMenuOpen(true);
            }}
          >
            <MenuIcon />
          </Button>
          <Brand />
          <div className="ml-auto flex gap-2 items-center">
            <ThemeToggle />
            <GitHubLink />
          </div>
        </header>

        <main
          id="main-content"
          tabIndex={-1}
          className={cn(
            "w-full",
            workspaceLayout
              ? "px-4 py-4 xl:flex xl:h-svh xl:flex-col xl:overflow-hidden xl:px-6"
              : "mx-auto max-w-6xl px-4 py-6 sm:px-6 lg:px-8 lg:py-8",
          )}
        >
          <PreviewBanner />
          <div className={cn(workspaceLayout && "xl:min-h-0 xl:flex-1")}>
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
