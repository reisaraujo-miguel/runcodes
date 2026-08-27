import Logo from "@/assets/runcodes-logo/logo.png";
import LogoBlue from "@/assets/runcodes-logo/logoblue.png";

import { useSearchParams } from "react-router";
import { AboutSection } from "../../components/login/AboutSection";
import { LoginCard } from "../../components/login/LoginCard";
import { SignInCard } from "../../components/login/SignInCard";
import { ThemeToggle } from "../../components/ThemeToggle";
import { useTheme } from "../../hooks/use-theme";

export function AuthPage() {
  const [searchParams] = useSearchParams();
  const isSignUp = searchParams.get("mode") === "signup";
  const isDark = useTheme() === "dark";

  return (
    <div className="min-h-screen bg-background">
      <div className="relative min-h-screen flex flex-col lg:grid lg:max-w-none lg:grid-cols-2 lg:px-0">
        {/* Desktop: Left Side (Logo, Theme Toggle, About Section) */}
        <div className="relative hidden h-full flex-col bg-muted p-10 lg:flex dark:border-r">
          <div className="relative z-20 flex items-center text-lg font-medium">
            <img
              src={isDark ? Logo : LogoBlue}
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

        {/* Desktop: Right Side (Card) */}
        <div className="hidden lg:flex items-center justify-center p-8 lg:p-0">
          <div className="mx-auto flex w-full flex-col items-center justify-center space-y-6 sm:w-87.5">
            {isSignUp ? <SignInCard /> : <LoginCard />}
          </div>
        </div>

        {/* Mobile: Logo and Theme Toggle */}
        <div className="lg:hidden flex items-center justify-between p-6">
          <img
            src={isDark ? Logo : LogoBlue}
            alt="RunCodes Logo"
            className="h-10"
          />
          <ThemeToggle />
        </div>

        {/* Mobile: Card */}
        <div className="flex flex-1 items-center justify-center p-8 lg:hidden">
          <div className="mx-auto flex w-full flex-col items-center justify-center space-y-6 sm:w-87.5">
            {isSignUp ? <SignInCard /> : <LoginCard />}
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
