import { Fragment } from "react";
import { NavLink } from "react-router";

import { buttonVariants } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Sidebar, SidebarContent } from "@/components/ui/sidebar";

import { cn } from "@/lib/utils";

interface AdminNavItem {
  /** Route the entry navigates to. */
  to: string;
  label: string;
  /** Match the path exactly, so the dashboard is not active on the other pages. */
  end?: boolean;
  /** Draws a separator above the entry. */
  separated?: boolean;
}

const NAV_ITEMS: AdminNavItem[] = [
  { to: "/admin", label: "Dashboard", end: true },
  { to: "/admin/courses", label: "Turmas Cadastradas" },
  { to: "/admin/users", label: "Usuários", separated: true },
  { to: "/admin/settings", label: "Configurações", separated: true },
];

/** Sidebar with the admin panel's sections, highlighting the current route. */
export function AdminSidebar() {
  return (
    <Sidebar className="top-16">
      <SidebarContent className="bg-background">
        <div className="flex flex-col p-2 space-y-2">
          {NAV_ITEMS.map((item) => (
            <Fragment key={item.to}>
              {item.separated === true && <Separator />}
              <NavLink
                to={item.to}
                end={item.end === true}
                className={({ isActive }) =>
                  cn(
                    buttonVariants({
                      variant: isActive ? "secondary" : "ghost",
                      size: "lg",
                    }),
                    "justify-start",
                  )
                }
                style={{
                  cursor: "pointer",
                  color: "inherit",
                  textDecoration: "none",
                }}
              >
                {item.label}
              </NavLink>
            </Fragment>
          ))}
        </div>
      </SidebarContent>
    </Sidebar>
  );
}
