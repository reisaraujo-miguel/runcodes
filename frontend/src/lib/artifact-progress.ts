/**
 * The judge uploads each case's generated files after the case finishes, so the
 * download link appears a little later than the result. It announces the upload
 * (`pending`) and its outcome (`ready`/`failed`); the results panel shows a
 * spinner in place of the link until then.
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
      return pending.includes(testCaseId) ? [...pending] : [...pending, testCaseId];
    case "ready":
    case "failed":
      return pending.filter((id) => id !== testCaseId);
    default:
      // An older judge sends the run-level event only.
      return [...pending];
  }
}
