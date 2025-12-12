import { useState } from "react";
import { NavLink } from "react-router";

import { CircleUserRound } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { ThemeToggle } from "@/components/theme-toggle";

import Logo from "@/assets/runcodes-logo/logo.png";

import { NewClassModal } from "./professor/NewClassModal";

const USER_ROLES = {
  Student: "student",
  Teacher: "teacher",
  Admin: "admin",
} as const;

type UserRole = (typeof USER_ROLES)[keyof typeof USER_ROLES];

export function Navbar() {
  let role: UserRole = USER_ROLES.Admin; // This should be dynamically set based on the logged-in user

  const [isNewClassModalOpen, setNewClassModalOpen] = useState(false);

  const openNewClassModal = () => {
    setNewClassModalOpen(true);
  };

  const closeNewClassModal = () => {
    setNewClassModalOpen(false);
  };

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
                <DropdownMenuTrigger asChild>
                  <Button className="text-white" variant="ghost" size="lg">
                    <CircleUserRound />
                    test@usp.br rop{" "}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent className="w-56">
                  <DropDownMenu
                    role={role}
                    openNewClassModal={openNewClassModal}
                  />
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </div>
      </nav>
      <NewClassModal
        isOpen={isNewClassModalOpen}
        onClose={closeNewClassModal}
      />
    </div>
  );
}

interface DropDownMenuProps {
  role: UserRole;
  openNewClassModal: () => void;
}

function DropDownMenu(props: DropDownMenuProps) {
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
        {(props.role === USER_ROLES.Teacher || USER_ROLES.Admin) && (
          <>
            <DropdownMenuItem
              onSelect={props.openNewClassModal}
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
