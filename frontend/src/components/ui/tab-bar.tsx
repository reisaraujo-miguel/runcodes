import { useRef, type KeyboardEvent, type ReactNode } from "react";

import { cn } from "@/lib/utils";

export interface TabBarItem<T extends string> {
  value: T;
  label: ReactNode;
  /** Small count or state marker rendered next to the label. */
  badge?: ReactNode;
}

/**
 * A dense, underline-style tab strip for split workspaces.
 *
 * It owns the WAI-ARIA tab semantics (roving tabindex and arrow-key
 * navigation) so each panel only has to render the matching tabpanel with the
 * ids derived from `idPrefix`.
 */
export function TabBar<T extends string>({
  items,
  value,
  onChange,
  label,
  trailing,
  idPrefix,
  className,
}: {
  items: readonly TabBarItem<T>[];
  value: T;
  onChange: (value: T) => void;
  /** Accessible name of the tab list. */
  label: string;
  /** Optional controls rendered at the far end of the strip. */
  trailing?: ReactNode;
  idPrefix: string;
  className?: string;
}) {
  const buttonRefs = useRef(new Map<T, HTMLButtonElement>());

  function focusTab(next: T) {
    onChange(next);
    buttonRefs.current.get(next)?.focus();
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const currentIndex = items.findIndex((item) => item.value === value);
    if (currentIndex === -1) return;

    let nextIndex: number;
    switch (event.key) {
      case "ArrowRight":
        nextIndex = (currentIndex + 1) % items.length;
        break;
      case "ArrowLeft":
        nextIndex = (currentIndex - 1 + items.length) % items.length;
        break;
      case "Home":
        nextIndex = 0;
        break;
      case "End":
        nextIndex = items.length - 1;
        break;
      default:
        return;
    }

    const next = items[nextIndex];
    if (!next) return;
    event.preventDefault();
    focusTab(next.value);
  }

  return (
    <div className={cn("flex h-10 shrink-0 items-center border-b", className)}>
      <div
        role="tablist"
        aria-label={label}
        className="flex h-full min-w-0 items-stretch pl-1"
        onKeyDown={handleKeyDown}
      >
        {items.map((item) => {
          const active = item.value === value;
          return (
            <button
              key={item.value}
              ref={(node) => {
                if (node) buttonRefs.current.set(item.value, node);
                else buttonRefs.current.delete(item.value);
              }}
              type="button"
              role="tab"
              id={`${idPrefix}-tab-${item.value}`}
              aria-selected={active}
              aria-controls={`${idPrefix}-panel-${item.value}`}
              tabIndex={active ? 0 : -1}
              onClick={() => {
                onChange(item.value);
              }}
              className={cn(
                "focus-visible:ring-ring/50 relative inline-flex items-center gap-1.5 px-3 text-xs font-medium whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-inset",
                active
                  ? "text-foreground after:bg-primary after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:rounded-full after:content-['']"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {item.label}
              {item.badge !== undefined && item.badge !== null ? (
                <span
                  className={cn(
                    "rounded-full px-1.5 py-px text-[10px] font-medium tabular-nums",
                    active
                      ? "bg-primary/12 text-primary"
                      : "bg-muted text-muted-foreground",
                  )}
                >
                  {item.badge}
                </span>
              ) : null}
            </button>
          );
        })}
      </div>
      {trailing ? (
        <div className="ml-auto flex min-w-0 items-center gap-2 pr-2">
          {trailing}
        </div>
      ) : null}
    </div>
  );
}
