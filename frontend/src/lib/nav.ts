import {
  BookOpenIcon,
  GraduationCapIcon,
  HomeIcon,
  LayoutDashboardIcon,
  SettingsIcon,
  SquarePenIcon,
  UserRoundIcon,
  UsersIcon,
  type LucideIcon,
} from "lucide-react";

import type { UserRole } from "@/lib/api/auth";

export interface NavItem {
  /** Route path, passed to `NavLink`. */
  to: string;
  label: string;
  icon: LucideIcon;
  /** Match the route exactly (used for the index route). */
  end?: boolean;
}

export interface NavSection {
  label?: string;
  items: NavItem[];
}

/**
 * The sidebar navigation for a role.
 *
 * Every role shares the student workspace (dashboard, exercises, classes). Staff
 * add the teaching area, and admins add the platform panel, so the menu grows
 * with privilege instead of switching to a separate layout.
 */
export function navSectionsFor(role: UserRole): NavSection[] {
  const sections: NavSection[] = [
    {
      items: [
        { to: "/", label: "Início", icon: HomeIcon, end: true },
        { to: "/exercises", label: "Exercícios", icon: BookOpenIcon },
        { to: "/classes", label: "Minhas Turmas", icon: GraduationCapIcon },
      ],
    },
  ];

  if (role === "professor" || role === "admin") {
    sections.push({
      label: "Ensino",
      items: [
        {
          to: "/professor",
          label: "Gerenciar Turmas",
          icon: SquarePenIcon,
        },
      ],
    });
  }

  if (role === "admin") {
    sections.push({
      label: "Administração",
      items: [
        {
          to: "/admin",
          label: "Painel",
          icon: LayoutDashboardIcon,
          end: true,
        },
        { to: "/admin/users", label: "Usuários", icon: UsersIcon },
        {
          to: "/admin/courses",
          label: "Todas as Turmas",
          icon: GraduationCapIcon,
        },
        {
          to: "/admin/settings",
          label: "Configurações",
          icon: SettingsIcon,
        },
      ],
    });
  }

  sections.push({
    items: [{ to: "/profile", label: "Meu Perfil", icon: UserRoundIcon }],
  });

  return sections;
}
