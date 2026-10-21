import {
  CloudCheckIcon,
  CloudOffIcon,
  Loader2Icon,
  RotateCcwIcon,
  Trash2Icon,
} from "lucide-react";

import { ConfirmButton } from "@/components/app/ConfirmDialog";
import type { CodeDraftSaveStatus } from "@/hooks/use-code-draft";
import { formatDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";

export interface CodeDraftIndicatorProps {
  status: CodeDraftSaveStatus;
  savedAt: number | null;
  error: string | null;
  restored: boolean;
  hasDraft: boolean;
  /** Hides the text label, leaving the status icon and its tooltip. */
  compact?: boolean;
  onDiscard: () => void;
}

/**
 * Compact autosave feedback for the code editor toolbar. It reports the state
 * itself, but does not put the whole indicator in a live region: the label
 * changes on every save while typing, which would be noisy for screen readers.
 * Only failures are announced, as an alert.
 */
export function CodeDraftIndicator({
  status,
  savedAt,
  error,
  restored,
  hasDraft,
  compact = false,
  onDiscard,
}: CodeDraftIndicatorProps) {
  if (status === "idle") return null;

  // A corrupt storage payload could carry a timestamp outside the range a Date
  // can represent; toISOString() throws for those, so validate before calling it.
  const savedDate = savedAt !== null && savedAt > 0 ? new Date(savedAt) : null;
  const timestamp =
    savedDate !== null && !Number.isNaN(savedDate.getTime())
      ? formatDateTime(savedDate.toISOString())
      : null;

  const fullLabel =
    status === "error"
      ? "Rascunho não salvo"
      : status === "saving"
        ? "Salvando…"
        : restored
          ? "Rascunho restaurado"
          : "Rascunho salvo";
  const shortLabel =
    status === "error"
      ? "Não salvo"
      : status === "saving"
        ? "Salvando…"
        : restored
          ? "Restaurado"
          : "Salvo";

  const title =
    status === "error" && error
      ? error
      : timestamp
        ? `${fullLabel}. Último salvamento em ${timestamp}.`
        : fullLabel;

  return (
    <div className="flex min-w-0 items-center gap-0.5">
      <span
        title={title}
        className={cn(
          "flex min-w-0 items-center gap-1.5 text-[11px] font-medium",
          status === "error" ? "text-destructive" : "text-muted-foreground",
        )}
      >
        {status === "saving" ? (
          <Loader2Icon className="size-3.5 shrink-0 animate-spin" aria-hidden />
        ) : status === "error" ? (
          <CloudOffIcon className="size-3.5 shrink-0" aria-hidden />
        ) : restored ? (
          <RotateCcwIcon className="size-3.5 shrink-0" aria-hidden />
        ) : (
          <CloudCheckIcon className="size-3.5 shrink-0" aria-hidden />
        )}
        <span className="truncate" aria-hidden={compact ? true : undefined}>
          {compact ? shortLabel : fullLabel}
        </span>
        {compact ? <span className="sr-only">{fullLabel}</span> : null}
      </span>
      {status === "error" && error ? (
        <span role="alert" className="sr-only">
          {error}
        </span>
      ) : null}
      {hasDraft ? (
        <ConfirmButton
          type="button"
          variant="ghost"
          size="icon-xs"
          className="text-muted-foreground hover:text-destructive"
          aria-label="Descartar rascunho salvo"
          title="Descartar rascunho?"
          description="O rascunho salvo neste navegador será removido. O código que está no editor não será apagado."
          confirmLabel="Descartar"
          cancelLabel="Cancelar"
          destructive
          onConfirm={onDiscard}
        >
          <Trash2Icon />
        </ConfirmButton>
      ) : null}
    </div>
  );
}
