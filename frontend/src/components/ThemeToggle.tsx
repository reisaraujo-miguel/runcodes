import { Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";

import { setGlobalIsDark } from "@/lib/theme";
import { useEffect, useState } from "react";

export function ThemeToggle() {
  const [isDark, setIsDark] = useState(
    () => window.matchMedia("(prefers-color-scheme: dark)").matches,
  );

  useEffect(() => {
    const root = window.document.documentElement;
    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");

    // Listen for changes
    const handleChange = (e: MediaQueryListEvent) => {
      setIsDark(e.matches);
    };

    mediaQuery.addEventListener("change", handleChange);

    root.classList.remove("dark", "light");
    root.classList.add(isDark ? "dark" : "light");
    setGlobalIsDark(isDark);
  });

  const handleToggle = () => {
    setIsDark(!isDark);
  };

  return (
    <Button
      variant="ghost"
      size="icon"
      className="hover:text-muted-foreground"
      onClick={handleToggle}
      title={`Switch to ${isDark ? "dark" : "light"} mode`}
    >
      <Sun className="h-[1.2rem] w-[1.2rem] rotate-0 scale-100 transition-all dark:-rotate-90 dark:scale-0" />
      <Moon className="absolute h-[1.2rem] w-[1.2rem] rotate-90 scale-0 transition-all dark:rotate-0 dark:scale-100" />
    </Button>
  );
}
