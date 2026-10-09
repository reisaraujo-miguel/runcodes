import { LogOutIcon, UserRoundIcon } from "lucide-react";
import { NavLink } from "react-router";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAuth } from "@/hooks/use-auth";
import { platformRoleLabel } from "@/lib/roles";
import { cn } from "@/lib/utils";

/** The first letters of a name, for the avatar fallback. */
function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  const first = parts[0]?.[0] ?? "";
  const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? "") : "";
  return (first + last).toUpperCase();
}

/**
 * The account control pinned to the bottom of the sidebar: the signed-in user,
 * a shortcut to their profile and the way out.
 */
export function UserMenu({
  className,
  collapsed = false,
}: {
  className?: string;
  /** Compact rail: the avatar alone, with the name as its accessible label. */
  collapsed?: boolean;
}) {
  const { user, signOut } = useAuth();
  if (!user) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={collapsed ? user.name : undefined}
        title={collapsed ? user.name : undefined}
        className={cn(
          "hover:bg-accent data-open:bg-accent flex w-full items-center rounded-lg text-left outline-none transition-colors",
          collapsed ? "justify-center py-2" : "gap-3 px-2 py-2",
          className,
        )}
      >
        <Avatar className="size-9">
          <AvatarFallback>{initials(user.name)}</AvatarFallback>
        </Avatar>
        {collapsed ? null : (
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{user.name}</p>
            <p className="text-muted-foreground truncate text-xs">
              {user.email}
            </p>
          </div>
        )}
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="start" className="w-60">
        {/* Base UI's group label only works inside a group, so the role header
            is wrapped in one. */}
        <DropdownMenuGroup>
          <DropdownMenuLabel>{platformRoleLabel(user.role)}</DropdownMenuLabel>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem render={<NavLink to="/profile" />}>
          <UserRoundIcon />
          Meu perfil
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          onClick={() => {
            void signOut();
          }}
        >
          <LogOutIcon />
          Sair
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
