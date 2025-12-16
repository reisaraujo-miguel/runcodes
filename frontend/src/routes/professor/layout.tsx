import { Outlet } from "react-router";

import { Footer } from "@/components/Footer";
import { Navbar } from "@/components/Navbar";

export function ProfessorTools() {
  return (
    <div>
      <Navbar />
      <main>
        <Outlet />
      </main>
      <Footer />
    </div>
  );
}
