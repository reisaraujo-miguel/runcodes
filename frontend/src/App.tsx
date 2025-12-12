import "@/App.css";

import { Outlet } from "react-router";

import { Footer } from "@/components/Footer";
import { ThemeProvider } from "@/components/theme-provider";

import { Navbar } from "./components/Navbar";

export default function App() {
  return (
    <ThemeProvider>
      <Navbar />
      <main>
        <Outlet />
      </main>
      <Footer />
    </ThemeProvider>
  );
}
