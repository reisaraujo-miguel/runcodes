import { Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";

import { setGlobalIsDark } from "@/lib/theme";
import { useEffect, useState } from "react";

const STORAGE_KEY = "theme-preference";

type ThemePreference = "dark" | "light" | null;

function getStoredPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "dark" || stored === "light") return stored;
  } catch {
    // localStorage may be unavailable (e.g., in SSR or private browsing)
  }
  return null;
}

function getInitialIsDark(): boolean {
  const stored = getStoredPreference();
  if (stored) {
    return stored === "dark";
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

export function ThemeToggle() {
  const [isDark, setIsDark] = useState(getInitialIsDark);

  useEffect(() => {
    const root = window.document.documentElement;
    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");

    // Only listen for system preference changes when no manual preference is stored
    const handleChange = (e: MediaQueryListEvent) => {
      if (!getStoredPreference()) {
        setIsDark(e.matches);
      }
    };

    mediaQuery.addEventListener("change", handleChange);

    root.classList.remove("dark", "light");
    root.classList.add(isDark ? "dark" : "light");
    setGlobalIsDark(isDark);

    return () => {
      mediaQuery.removeEventListener("change", handleChange);
    };
  }, [isDark]);

  const handleToggle = () => {
    const newIsDark = !isDark;
    setIsDark(newIsDark);
    try {
      localStorage.setItem(STORAGE_KEY, newIsDark ? "dark" : "light");
    } catch {
      // localStorage may be unavailable
    }
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
