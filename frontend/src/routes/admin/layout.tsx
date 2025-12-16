import { Outlet } from "react-router";

import { Footer } from "@/components/Footer";
import { Navbar } from "@/components/Navbar";
import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";

import { AdminSidebar } from "@/components/admin/AdminSidebar";

export function AdminTools() {
  return (
    <div>
      <Navbar />
      <main>
        <SidebarProvider>
          <AdminSidebar />
          <div className="flex flex-col flex-1">
            <SidebarTrigger className="p-4 m-2" />
            <Outlet />
          </div>
        </SidebarProvider>
      </main>
      <Footer />
    </div>
  );
}
