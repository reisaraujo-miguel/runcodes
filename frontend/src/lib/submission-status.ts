import type {
  CaseResultStatus,
  CommitStatus,
  FinishedStatus,
} from "@/lib/api/submissions";

/**
 * Tone used to color status badges consistently across pages.
 *
 * Covers every value of the backend's `commit_status_t` enum. `pending` and
 * `plagiarism` only reach the client through a snapshot of a commit migrated
 * from the legacy system, but they must be handled: the maps below are indexed by
 * these keys, so a missing entry would render an undefined badge instead of a
 * status.
 */
export type StatusTone = "neutral" | "info" | "success" | "warning" | "danger";

export interface StatusToneValue {
  label: string;
  tone: StatusTone;
}

const COMMIT_STATUS: Record<CommitStatus, StatusToneValue> = {
  pending: { label: "Aguardando", tone: "neutral" },
  queued: { label: "Na fila", tone: "neutral" },
  compiling: { label: "Compilando", tone: "info" },
  running: { label: "Executando", tone: "info" },
  completed: { label: "Concluído", tone: "success" },
  uncompleted: { label: "Não concluído", tone: "warning" },
  compilation_error: { label: "Erro de compilação", tone: "danger" },
  server_error: { label: "Erro no servidor", tone: "danger" },
  plagiarism: { label: "Plágio", tone: "danger" },
  timeout: { label: "Tempo esgotado", tone: "danger" },
};

const CASE_REASON: Record<string, string> = {
  timeout: "Tempo limite excedido",
  cpu_limit: "Limite de CPU excedido",
  memory_limit: "Limite de memória excedido",
  stack_limit: "Limite de pilha excedido",
  file_size_limit: "Limite de tamanho de arquivo excedido",
  signal: "Encerrado por um sinal",
  monitor_error: "Falha do monitor de execução",
  output_error: "Não foi possível ler a saída do caso",
  server_error: "Falha do servidor de execução",
};

/**
 * The reason an interrupted case was interrupted, in the student's language, or
 * null when the judge could not attribute the failure (a plain non-zero exit).
 */
export function caseReasonLabel(
  reason: string | null | undefined,
): string | null {
  if (!reason) return null;
  return CASE_REASON[reason] ?? null;
}

const CASE_STATUS: Record<CaseResultStatus, StatusToneValue> = {
  correct: { label: "Correto", tone: "success" },
  bad_formatted_output: { label: "Saída incorreta", tone: "warning" },
  killed_with_signal: { label: "Interrompido", tone: "danger" },
};

/**
 * Maps a commit status to its badge.
 *
 * The record is exhaustive over `CommitStatus`, so a status added to the enum
 * without a badge is a compile error rather than an undefined badge at runtime.
 */
export function commitStatus(status: CommitStatus): StatusToneValue {
  return COMMIT_STATUS[status];
}

export function caseStatus(status: CaseResultStatus): StatusToneValue {
  return CASE_STATUS[status];
}

export function finishedStatus(status: FinishedStatus): StatusToneValue {
  return COMMIT_STATUS[status];
}

/** Status values that mean the run is still in progress. */
export function isPendingStatus(status: CommitStatus): boolean {
  return (
    status === "pending" ||
    status === "queued" ||
    status === "compiling" ||
    status === "running"
  );
}

/**
 * Reports whether a commit has settled.
 *
 * This is the positive form of `isPendingStatus`, and a type predicate: every
 * `CommitStatus` that is not in progress is one of the terminal statuses. An
 * unrecognised status (a value added to the enum later) therefore ends the stream
 * instead of leaving the client subscribed to a server that has already closed it.
 */
export function isTerminalStatus(
  status: CommitStatus,
): status is FinishedStatus {
  return !isPendingStatus(status);
}
