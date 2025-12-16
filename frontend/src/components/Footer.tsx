import Github from "@/assets/svg-icons/github.svg?react";

import { Button } from "./ui/button";

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
          <Button
            variant="ghost"
            size="sm"
            className="text-muted-foreground hover:text-foreground"
          >
            <a
              href="https://github.com/reisaraujo-miguel/runcodes-react"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2"
            >
              <Github className="h-7 w-7" />
              <span>GitHub</span>
            </a>
          </Button>
        </div>
      </div>
    </footer>
  );
}
