import Github from "@/assets/svg-icons/github.svg?react";

import { buttonVariants } from "./ui/button";

const REPOSITORY_URL = "https://github.com/reisaraujo-miguel/runcodes-react";

export function Footer() {
  return (
    <footer className="w-full p-4 border-t bg-background">
      <div className="container flex flex-col items-center justify-center gap-4 md:flex-row md:justify-between">
        <div className="text-center md:text-left">
          <p className="text-sm text-muted-foreground">
            Este projeto é software livre ❤️
          </p>
        </div>
        <div>
          {/* A link styled as a button, not a button wrapping a link: nesting
              one interactive element inside another is invalid HTML and gives
              the control two tab stops. */}
          <a
            href={REPOSITORY_URL}
            target="_blank"
            rel="noopener noreferrer"
            className={buttonVariants({ variant: "ghost", size: "sm" })}
          >
            <Github className="h-7 w-7" aria-hidden="true" />
            <span>GitHub</span>
          </a>
        </div>
      </div>
    </footer>
  );
}
