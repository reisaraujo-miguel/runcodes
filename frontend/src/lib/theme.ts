export type Theme = "dark" | "light";

const STORAGE_KEY = "theme-preference";

type Listener = (theme: Theme) => void;

let theme: Theme;
let storedTheme: Theme | null = null;
const listeners = new Set<Listener>();

function getStoredPreference(): Theme | null {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "dark" || stored === "light") return stored;
  } catch {
    // localStorage may be unavailable (e.g., in private browsing)
  }
  return null;
}

function getSystemPreference(): Theme {
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function applyTheme(next: Theme) {
  const root = document.documentElement;
  root.classList.remove("dark", "light");
  root.classList.add(next);
}

function update(next: Theme, persist: boolean) {
  if (next === theme) return;
  theme = next;
  if (persist) {
    storedTheme = next;
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // localStorage may be unavailable
    }
  }
  applyTheme(next);
  for (const listener of listeners) listener(next);
}

export function getTheme(): Theme {
  return theme;
}

export function setTheme(next: Theme) {
  update(next, true);
}

export function subscribeToTheme(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// Resolve the initial theme (stored preference falling back to the system
// preference) and apply it as soon as this module loads — before React renders
// anything, including the HydrateFallback. This keeps the theme independent of
// which components happen to be mounted.
// Note: index.html applies the theme in an inline script before first paint;
// keep that script and the resolution logic here in sync.
storedTheme = getStoredPreference();
theme = storedTheme ?? getSystemPreference();
applyTheme(theme);

// Follow system preference changes until the user picks an explicit theme.
window
  .matchMedia("(prefers-color-scheme: dark)")
  .addEventListener("change", (event) => {
    if (storedTheme !== null) return;
    update(event.matches ? "dark" : "light", false);
  });
