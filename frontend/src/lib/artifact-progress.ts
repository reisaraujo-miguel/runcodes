import { formatBytes } from "./format";

/**
 * The judge uploads each case's generated files after the case finishes, so the
 * download link appears a little later than the result. It announces the upload
 * (`pending`) and its outcome (`ready`/`failed`); the results panel shows a
 * spinner in place of the link until then, or the failure reason when there will
 * never be a link.
 */

/** Minimal shape of an artifact event needed to track upload progress. */
export interface ArtifactProgressEvent {
  test_case_id?: number;
  state?: string;
}

/**
 * Applies an artifact event to the list of cases whose files are still being
 * uploaded. Events without a case (the run-level artifact) leave the list alone.
 */
export function applyArtifactProgress(
  pending: readonly number[],
  event: ArtifactProgressEvent,
): number[] {
  const testCaseId = event.test_case_id;
  if (testCaseId === undefined || !Number.isSafeInteger(testCaseId)) {
    return [...pending];
  }

  switch (event.state) {
    case "pending":
      return pending.includes(testCaseId)
        ? [...pending]
        : [...pending, testCaseId];
    case "ready":
    case "failed":
      return pending.filter((id) => id !== testCaseId);
    default:
      // An older judge sends the run-level event only.
      return [...pending];
  }
}

/** Upload state of one test case's generated files. */
export interface ArtifactStatus {
  state: "pending" | "ready" | "failed";
  reason?: string;
  limitBytes?: number;
}

/** Minimal shape of an artifact event needed to update a status map. */
export interface ArtifactStatusEvent {
  test_case_id?: number;
  state?: string;
  reason?: string;
  limit_bytes?: number;
}

function isArtifactState(
  state: string | undefined,
): state is ArtifactStatus["state"] {
  return state === "pending" || state === "ready" || state === "failed";
}

/**
 * Applies one artifact event to the per-case status map. Run-level events and
 * events from an older judge without a state are ignored; the map is returned as
 * a fresh value so React sees the change.
 */
export function applyArtifactStatus(
  statuses: ReadonlyMap<number, ArtifactStatus>,
  event: ArtifactStatusEvent,
): Map<number, ArtifactStatus> {
  const testCaseId = event.test_case_id;
  const next = new Map(statuses);
  if (
    testCaseId === undefined ||
    !Number.isSafeInteger(testCaseId) ||
    !isArtifactState(event.state)
  ) {
    return next;
  }

  next.set(testCaseId, {
    state: event.state,
    reason: event.reason,
    limitBytes: event.limit_bytes,
  });

  return next;
}

/** True while at least one case's files are still being uploaded. */
export function pendingUploads(
  statuses: ReadonlyMap<number, ArtifactStatus>,
): number[] {
  const pending: number[] = [];
  for (const [testCaseId, status] of statuses) {
    if (status.state === "pending") pending.push(testCaseId);
  }

  return pending;
}

/**
 * Sentence shown where a failed case's download link would be. The reason codes
 * are the judge's stable wire values; an unrecognised reason still says that the
 * files are unavailable rather than showing nothing.
 */
export function artifactUnavailableMessage(
  status: ArtifactStatus | undefined,
): string | null {
  if (status?.state !== "failed") return null;

  switch (status.reason) {
    case "commit_artifact_budget_exceeded":
      return status.limitBytes && status.limitBytes > 0
        ? `Os arquivos gerados não estão disponíveis porque esta submissão atingiu o limite total de arquivos gerados (${formatBytes(status.limitBytes)}).`
        : "Os arquivos gerados não estão disponíveis porque esta submissão atingiu o limite total de arquivos gerados.";
    case "file_too_large":
      return "Os arquivos gerados não estão disponíveis porque excedem o limite de tamanho deste caso.";
    case "upload_failed":
    case "storage_unavailable":
      return "Os arquivos gerados não estão disponíveis porque falhou o envio para o armazenamento.";
    case "upload_interrupted":
      return "Os arquivos gerados não estão disponíveis porque o envio foi interrompido quando a correção terminou.";
    default:
      return "Os arquivos gerados não estão disponíveis para esta submissão.";
  }
}
