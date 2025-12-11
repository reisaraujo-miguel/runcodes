import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Sidebar, SidebarContent } from "@/components/ui/sidebar";

const ItemType = {
  Dashboard: "dashboard",
  Turmas: "turmas",
  SystemLogs: "system-logs",
  Settings: "settings",
} as const;

type ItemKey = (typeof ItemType)[keyof typeof ItemType];

export function AdminSidebar() {
  const [selectedItem, setSelectedItem] = useState<ItemKey>("dashboard");

  const handleItemClick = (item: ItemKey) => {
    setSelectedItem(item);
  };

  return (
    <Sidebar className="top-16">
      <SidebarContent className="bg-background">
        <div className="flex flex-col p-2 space-y-2">
          <Button
            variant={
              selectedItem === ItemType.Dashboard ? "secondary" : "ghost"
            }
            size="lg"
            className="justify-start"
            style={{
              cursor: "pointer",
              color: "inherit",
              textDecoration: "none",
            }}
            onClick={() => handleItemClick(ItemType.Dashboard)}
          >
            Dashboard
          </Button>
          <Button
            variant={selectedItem === ItemType.Turmas ? "secondary" : "ghost"}
            size="lg"
            className="justify-start"
            style={{
              cursor: "pointer",
              color: "inherit",
              textDecoration: "none",
            }}
            onClick={() => handleItemClick(ItemType.Turmas)}
          >
            Turmas Cadastradas
          </Button>
          <Separator />
          <Button
            variant={
              selectedItem === ItemType.SystemLogs ? "secondary" : "ghost"
            }
            size="lg"
            className="justify-start"
            style={{
              cursor: "pointer",
              color: "inherit",
              textDecoration: "none",
            }}
            onClick={() => handleItemClick(ItemType.SystemLogs)}
          >
            System Logs
          </Button>
          <Separator />
          <Button
            variant={selectedItem === ItemType.Settings ? "secondary" : "ghost"}
            size="lg"
            className="justify-start"
            style={{
              cursor: "pointer",
              color: "inherit",
              textDecoration: "none",
            }}
            onClick={() => handleItemClick(ItemType.Settings)}
          >
            Settings
          </Button>
        </div>
      </SidebarContent>
    </Sidebar>
  );
}
