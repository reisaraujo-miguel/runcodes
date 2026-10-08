import type { LucideIcon } from "lucide-react";
import { AlertTriangleIcon, RotateCwIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";

/** Centered spinner for a page or panel that is still loading. */
export function LoadingState({
  label = "Carregando…",
  className,
}: {
  label?: string;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "text-muted-foreground flex flex-col items-center justify-center gap-3 py-16 text-sm",
        className,
      )}
    >
      <Spinner className="size-5" label={label} />
      <p>{label}</p>
    </div>
  );
}

/** An inline error with an optional retry affordance. */
export function ErrorState({
  title = "Algo deu errado",
  description,
  onRetry,
  retryLabel = "Tentar novamente",
  className,
}: {
  title?: string;
  description: ReactNode;
  onRetry?: () => void;
  retryLabel?: string;
  className?: string;
}) {
  return (
    <Alert variant="destructive" className={className}>
      <AlertTriangleIcon aria-hidden />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        <div>{description}</div>
        {onRetry ? (
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RotateCwIcon />
            {retryLabel}
          </Button>
        ) : null}
      </AlertDescription>
    </Alert>
  );
}

/** A dashed placeholder for a collection with no items yet. */
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: {
  icon?: LucideIcon;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "border-border flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed px-6 py-12 text-center",
        className,
      )}
    >
      {Icon ? (
        <div className="bg-muted text-muted-foreground flex size-11 items-center justify-center rounded-full">
          <Icon className="size-5" aria-hidden />
        </div>
      ) : null}
      <div className="space-y-1">
        <p className="font-medium">{title}</p>
        {description ? (
          <p className="text-muted-foreground mx-auto max-w-sm text-sm">
            {description}
          </p>
        ) : null}
      </div>
      {action}
    </div>
  );
}
