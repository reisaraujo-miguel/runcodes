import { Outlet } from "react-router";

import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";

import { AdminSidebar } from "@/components/admin/AdminSidebar";

export function AdminTools() {
  return (
    <SidebarProvider>
      <AdminSidebar />
      <div className="flex flex-col flex-1">
        <SidebarTrigger className="p-4 m-2" />
        <Outlet />
      </div>
    </SidebarProvider>
  );
}
