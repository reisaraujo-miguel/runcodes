import { DownloadIcon, HistoryIcon, RotateCcwIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { StatusBadge } from "@/components/app/StatusBadge";
import { EmptyState, ErrorState, LoadingState } from "@/components/app/states";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { useAsync } from "@/hooks/use-async";
import {
  getSubmissionSourceBlob,
  getSubmissionSourceText,
  listExerciseSubmissions,
  type SubmissionSummary,
} from "@/lib/api";
import { ApiRequestError } from "@/lib/api/client";
import { downloadBlob } from "@/lib/download";
import {
  editorLanguageForFilename,
  type EditorLanguage,
} from "@/lib/editor/languages";
import { errorMessage } from "@/lib/errors";
import { formatDateTime } from "@/lib/format";
import { commitStatus, isTerminalStatus } from "@/lib/submission-status";

export interface SubmissionHistoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  exerciseId: number;
  /** Languages the editor can currently offer, used to enable restoring. */
  languages: readonly EditorLanguage[];
  /**
   * Called with the fetched source once the user asks to restore it. The parent
   * decides whether a confirmation is needed and applies the content.
   */
  onRestore: (submission: SubmissionSummary, source: string) => void;
}

type HistoryAction = "download" | "restore";

/** Turns a missing source into student-facing copy instead of the API's message. */
function actionErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiRequestError && error.status === 404) {
    return "Esta submissão não está mais disponível.";
  }
  return errorMessage(error, fallback);
}

/**
 * Lists the user's past submissions for one exercise. Every entry can be
 * downloaded as its original file; submissions written in a language the editor
 * still offers can also be restored into the editor.
 */
export function SubmissionHistoryDialog({
  open,
  onOpenChange,
  exerciseId,
  languages,
  onRestore,
}: SubmissionHistoryDialogProps) {
  const submissions = useAsync(
    () => listExerciseSubmissions(exerciseId),
    `submissions-${String(exerciseId)}`,
    "Não foi possível carregar as submissões anteriores.",
  );
  const [busy, setBusy] = useState<{
    id: number;
    action: HistoryAction;
  } | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // A fetch can outlive the dialog if the user closes it while the request is
  // in flight. In particular, closing must not restore code after the fact.
  const activeRef = useRef(true);
  useEffect(() => {
    activeRef.current = open;
    return () => {
      activeRef.current = false;
    };
  }, [open]);

  // Guard against a double click starting two requests before React applies the
  // disabled state to the action buttons.
  const busyRef = useRef(false);

  const list = submissions.data ?? [];

  async function handleDownload(submission: SubmissionSummary) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy({ id: submission.id, action: "download" });
    setActionError(null);
    try {
      // Fetch the blob, not text: upload-mode submissions may be binary.
      const source = await getSubmissionSourceBlob(submission.id);
      if (!activeRef.current) return;
      downloadBlob(submission.filename, source);
    } catch (error) {
      if (!activeRef.current) return;
      setActionError(
        actionErrorMessage(error, "Não foi possível baixar a submissão."),
      );
    } finally {
      busyRef.current = false;
      if (activeRef.current) setBusy(null);
    }
  }

  async function handleRestore(submission: SubmissionSummary) {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy({ id: submission.id, action: "restore" });
    setActionError(null);
    try {
      const source = await getSubmissionSourceText(submission.id);
      if (!activeRef.current) return;
      onRestore(submission, source);
      onOpenChange(false);
    } catch (error) {
      if (!activeRef.current) return;
      setActionError(
        actionErrorMessage(error, "Não foi possível carregar a submissão."),
      );
    } finally {
      busyRef.current = false;
      if (activeRef.current) setBusy(null);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Submissões anteriores</DialogTitle>
          <DialogDescription>
            Baixe o arquivo original ou restaure uma solução no editor.
          </DialogDescription>
        </DialogHeader>

        {actionError ? (
          <p role="alert" className="text-destructive text-xs">
            {actionError}
          </p>
        ) : null}

        {submissions.loading && submissions.data === null ? (
          <LoadingState className="py-8" />
        ) : submissions.error ? (
          <ErrorState
            description={submissions.error}
            onRetry={submissions.reload}
          />
        ) : list.length === 0 ? (
          <EmptyState
            icon={HistoryIcon}
            title="Nenhuma submissão anterior"
            description="As soluções que você enviar para este exercício aparecem aqui."
            className="py-8"
          />
        ) : (
          <ul className="divide-border max-h-[55vh] divide-y overflow-y-auto rounded-lg border">
            {list.map((submission) => {
              const language = editorLanguageForFilename(
                languages,
                submission.filename,
              );
              const active = busy?.id === submission.id;
              const finished = isTerminalStatus(submission.status);

              return (
                <li key={submission.id} className="space-y-2 px-3 py-2.5">
                  <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <span className="font-mono text-xs font-medium">
                      {submission.filename}
                    </span>
                    <StatusBadge value={commitStatus(submission.status)} />
                    <span className="text-muted-foreground text-[11px] tabular-nums">
                      Submissão #{submission.id} ·{" "}
                      {formatDateTime(submission.created_at)}
                    </span>
                    <span className="ml-auto text-xs">
                      {finished ? (
                        <>
                          <span className="text-muted-foreground">Nota </span>
                          <span className="font-semibold tabular-nums">
                            {submission.score.toFixed(2)}
                          </span>
                        </>
                      ) : (
                        <span className="text-muted-foreground">
                          Ainda em correção
                        </span>
                      )}
                    </span>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={busy !== null}
                      onClick={() => void handleDownload(submission)}
                    >
                      {active && busy.action === "download" ? (
                        <Spinner className="size-3.5" />
                      ) : (
                        <DownloadIcon />
                      )}
                      Baixar
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      disabled={busy !== null || !language}
                      title={
                        language
                          ? undefined
                          : "Este arquivo não pode ser restaurado no editor."
                      }
                      onClick={() => void handleRestore(submission)}
                    >
                      {active && busy.action === "restore" ? (
                        <Spinner className="size-3.5" />
                      ) : (
                        <RotateCcwIcon />
                      )}
                      Restaurar no editor
                    </Button>
                    {!language ? (
                      <span className="text-muted-foreground text-[11px]">
                        Restauração indisponível para este arquivo.
                      </span>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ul>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              onOpenChange(false);
            }}
          >
            Fechar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
