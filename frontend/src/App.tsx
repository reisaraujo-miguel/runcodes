/*  ______    __  __   ___   __    ______   ______   ______   ______   ______
 * /_____/\  /_/\/_/\ /__/\ /__/\ /_____/\ /_____/\ /_____/\ /_____/\ /_____/\
 * \:::_ \ \ \:\ \:\ \\::\_\\  \ \\:::__\/ \:::_ \ \\:::_ \ \\::::_\/_\::::_\/_
 *  \:(_) ) )_\:\ \:\ \\:. `-\  \ \\:\ \  __\:\ \ \ \\:\ \ \ \\:\/___/\\:\/___/\
 *   \: __ `\ \\:\ \:\ \\:. _    \ \\:\ \/_/\\:\ \ \ \\:\ \ \ \\::___\/_\_::._\:\
 *    \ \ `\ \ \\:\_\:\ \\. \`-\  \ \\:\_\ \ \\:\_\ \ \\:\/.:| |\:\____/\ /____\:\
 *     \_\/ \_\/ \_____\/ \__\/ \__\/ \_____\/ \_____\/ \____/_/ \_____\/ \_____\/
 *
 * "programmer:
 *  noun [pro-gram-mer]
 *
 *  1. A person who turns coffee into buggy code.
 *  2. A person who fixes a problem no one knows the cause in a way no one understands."
 *
 *  - Leug
 */

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
