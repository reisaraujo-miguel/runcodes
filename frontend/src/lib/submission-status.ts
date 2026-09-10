import type {
  CaseResultStatus,
  CommitStatus,
  FinishedStatus,
} from "@/lib/api/submissions";

/** Tone used to color status badges consistently across pages. */
export type StatusTone = "neutral" | "info" | "success" | "warning" | "danger";

export interface StatusToneValue {
  label: string;
  tone: StatusTone;
}

const COMMIT_STATUS: Record<CommitStatus, StatusToneValue> = {
  queued: { label: "Na fila", tone: "neutral" },
  compiling: { label: "Compilando", tone: "info" },
  running: { label: "Executando", tone: "info" },
  completed: { label: "Concluído", tone: "success" },
  uncompleted: { label: "Não concluído", tone: "warning" },
  compilation_error: { label: "Erro de compilação", tone: "danger" },
  server_error: { label: "Erro no servidor", tone: "danger" },
  timeout: { label: "Tempo esgotado", tone: "danger" },
};

const CASE_STATUS: Record<CaseResultStatus, StatusToneValue> = {
  correct: { label: "Correto", tone: "success" },
  bad_formatted_output: { label: "Saída incorreta", tone: "warning" },
  killed_with_signal: { label: "Interrompido", tone: "danger" },
};

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
  return status === "queued" || status === "compiling" || status === "running";
}
