import Logo from "@/assets/runcodes-logo/logo.png";
import LogoBlue from "@/assets/runcodes-logo/logoblue.png";

import { AboutSection } from "../../components/login/AboutSection";
import { LoginCard } from "../../components/login/LoginCard";
import { ThemeToggle } from "../../components/theme-toggle";
import { useTheme } from "../../lib/theme";

export function LoginPage() {
  const { theme } = useTheme();

  // Determine if we're in dark mode based on the current theme
  const isDarkTheme =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  return (
    <div className="min-h-screen bg-background">
      <div className="relative min-h-screen flex flex-col lg:grid lg:max-w-none lg:grid-cols-2 lg:px-0">
        {/* Desktop: Left Side (Logo, Theme Toggle, About Section) */}
        <div className="relative hidden h-full flex-col bg-muted p-10 lg:flex dark:border-r">
          <div className="relative z-20 flex items-center text-lg font-medium">
            <img
              src={isDarkTheme ? Logo : LogoBlue}
              alt="RunCodes Logo"
              className="h-10"
            />
            <div className="flex items-center">
              <ThemeToggle />
            </div>
          </div>
          <div className="relative z-20 mt-auto">
            <AboutSection />
          </div>
        </div>

        {/* Desktop: Right Side (Login Card) */}
        <div className="hidden lg:flex items-center justify-center p-8 lg:p-0">
          <div className="mx-auto flex w-full flex-col items-center justify-center space-y-6 sm:w-[350px]">
            <LoginCard />
          </div>
        </div>

        {/* Mobile: Logo and Theme Toggle */}
        <div className="lg:hidden flex items-center justify-between p-6">
          <img
            src={isDarkTheme ? Logo : LogoBlue}
            alt="RunCodes Logo"
            className="h-10"
          />
          <ThemeToggle />
        </div>

        {/* Mobile: Login Card */}
        <div className="flex flex-1 items-center justify-center p-8 lg:hidden">
          <div className="mx-auto flex w-full flex-col items-center justify-center space-y-6 sm:w-[350px]">
            <LoginCard />
          </div>
        </div>

        {/* Mobile: About Section */}
        <div className="lg:hidden bg-muted p-6 dark:border-t">
          <AboutSection />
        </div>
      </div>
    </div>
  );
}
