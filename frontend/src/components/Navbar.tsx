import { NavLink } from "react-router";
import { useState } from "react";

import { CircleUserRound } from "lucide-react";

import { buttonVariants } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { NewClassModal } from "@/components/professor/NewClassModal";
import { ThemeToggle } from "@/components/ThemeToggle";

import Logo from "@/assets/runcodes-logo/logo.png";

const USER_ROLES = {
  Student: "student",
  Teacher: "teacher",
  Admin: "admin",
} as const;

type UserRole = (typeof USER_ROLES)[keyof typeof USER_ROLES];

export function Navbar() {
  const role: UserRole = USER_ROLES.Admin; // This should be dynamically set based on the logged-in user
  const [isNewClassModalOpen, setIsNewClassModalOpen] = useState(false);

  return (
    <div>
      <nav className="sticky top-0 bg-primary">
        <div className="mx-auto flex items-center justify-between h-16">
          <div className="flex items-center">
            <NavLink to="/" className="ml-4">
              <img src={Logo} alt="RunCodes Logo" className="h-10" />
            </NavLink>
            <div className="text-white">
              <ThemeToggle />
            </div>
          </div>
          <div className="flex h-full">
            <div className="flex items-center px-2">
              <DropdownMenu>
                <DropdownMenuTrigger
                  className={buttonVariants({ variant: "ghost", size: "lg" })}
                >
                  <div className="flex items-center h-full gap-2 text-white">
                    <CircleUserRound />
                    test@usp.br
                  </div>
                </DropdownMenuTrigger>
                <DropdownMenuContent className="w-56">
                  <DropDownMenu
                    role={role}
                    onCreateNewClass={() => {
                      setIsNewClassModalOpen(true);
                    }}
                  />
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </div>
      </nav>
      {isNewClassModalOpen && (
        <NewClassModal
          onClose={() => {
            setIsNewClassModalOpen(false);
          }}
        />
      )}
    </div>
  );
}

function DropDownMenu(props: { role: UserRole; onCreateNewClass: () => void }) {
  return (
    <div>
      <div>
        <DropdownMenuItem
          style={{
            cursor: "pointer",
            color: "inherit",
            textDecoration: "none",
          }}
        >
          Perfil
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {(props.role === USER_ROLES.Teacher ||
          props.role === USER_ROLES.Admin) && (
          <>
            <DropdownMenuItem
              onClick={props.onCreateNewClass}
              style={{
                cursor: "pointer",
                color: "inherit",
                textDecoration: "none",
              }}
            >
              Criar Nova Turma
            </DropdownMenuItem>
            <DropdownMenuItem
              style={{
                cursor: "pointer",
                color: "inherit",
                textDecoration: "none",
              }}
            >
              Gerenciar Turmas
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </>
        )}
        {props.role === USER_ROLES.Admin && (
          <>
            <DropdownMenuItem>
              <NavLink
                to="/admin"
                style={{
                  cursor: "pointer",
                  textDecoration: "none",
                  color: "inherit",
                }}
              >
                Ferramentas de Admin
              </NavLink>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
          </>
        )}
        <DropdownMenuItem
          variant="destructive"
          style={{
            cursor: "pointer",
            textDecoration: "none",
          }}
        >
          Sair
        </DropdownMenuItem>
      </div>
    </div>
  );
}
