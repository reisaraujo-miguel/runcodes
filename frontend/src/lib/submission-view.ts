import type {
  CaseResultStatus,
  FinishedStatus,
  SubmissionCaseResultEvent,
  SubmissionSnapshotResult,
} from "@/lib/api/submissions";

/** Normalized per-case result used by the UI, from snapshot or live event. */
export interface CaseResultView {
  testCaseId: number;
  cpuTime: number;
  memUsage: number;
  status: CaseResultStatus;
  statusMessage: string;
  errorMessage: string;
  userOutput: string;
  userOutputType: string;
}

export interface CompilationInfo {
  compiled: boolean;
  message: string;
  error: string;
}

export interface FinalSummary {
  status: FinishedStatus;
  numCorrectCases: number;
  score: number;
  compilationMessage: string;
  compilationError: string;
}

export function viewFromSnapshot(
  result: SubmissionSnapshotResult,
): CaseResultView {
  return {
    testCaseId: result.exercise_test_case_id,
    cpuTime: result.cpu_time,
    memUsage: result.mem_usage,
    status: result.status,
    statusMessage: result.status_message,
    errorMessage: result.error_message,
    userOutput: result.user_output,
    userOutputType: result.user_output_type,
  };
}

export function viewFromEvent(
  event: SubmissionCaseResultEvent,
): CaseResultView {
  return {
    testCaseId: event.test_case_id,
    cpuTime: event.cpu_time,
    memUsage: event.mem_usage,
    status: event.status,
    statusMessage: event.status_message,
    errorMessage: event.error_message,
    userOutput: event.user_output,
    userOutputType: event.user_output_type,
  };
}

/** Inserts or replaces a case result, preserving first-seen order. */
export function upsertCaseResult(
  results: CaseResultView[],
  view: CaseResultView,
): CaseResultView[] {
  const index = results.findIndex(
    (item) => item.testCaseId === view.testCaseId,
  );
  if (index === -1) return [...results, view];
  const next = [...results];
  next[index] = view;
  return next;
}
