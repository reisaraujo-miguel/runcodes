import { API_BASE_URL, isRecord, readApiBody, readApiError } from "./client";

/** Lifecycle of a commit, mirroring the backend's commit status enum. */
export type CommitStatus =
  | "queued"
  | "compiling"
  | "running"
  | "completed"
  | "uncompleted"
  | "compilation_error"
  | "server_error"
  | "timeout";

/** Terminal statuses reported by the `finished` event. */
export type FinishedStatus = Extract<
  CommitStatus,
  "completed" | "uncompleted" | "compilation_error" | "server_error" | "timeout"
>;

/** Per-test-case result, mirroring the backend's case status enum. */
export type CaseResultStatus =
  "correct" | "bad_formatted_output" | "killed_with_signal";

/** Response returned by `POST /api/v1/submissions`. */
export interface QueuedSubmission {
  commit_id: number;
  status: "queued";
  events_url: string;
}

/** The commit state carried by the first `snapshot` frame. */
export interface SubmissionCommit {
  id: number;
  status: CommitStatus;
  num_correct_cases: number;
  score: number;
  compiled: boolean;
  compilation_message: string;
  compilation_error: string;
  compilation_started: string | null;
  compilation_finished: string | null;
  created_at: string;
  s3_key: string;
}

/** A per-case result embedded in the `snapshot` frame. */
export interface SubmissionSnapshotResult {
  exercise_test_case_id: number;
  cpu_time: number;
  mem_usage: number;
  user_output: string;
  user_output_type: string;
  status: CaseResultStatus;
  status_message: string;
  error_message: string;
}

export interface SubmissionSnapshotEvent {
  type: "snapshot";
  commit: SubmissionCommit;
  results: SubmissionSnapshotResult[];
}

export interface SubmissionStatusEvent {
  type: "status";
  commit_id: number;
  seq: number;
  status: Extract<CommitStatus, "compiling" | "running">;
  at: string;
}

export interface SubmissionCompilationEvent {
  type: "compilation";
  commit_id: number;
  seq: number;
  compiled: boolean;
  message: string;
  error: string;
  at: string;
}

export interface SubmissionCaseResultEvent {
  type: "case_result";
  commit_id: number;
  seq: number;
  test_case_id: number;
  cpu_time: number;
  mem_usage: number;
  status: CaseResultStatus;
  status_message: string;
  user_output: string;
  user_output_type: string;
  error_message: string;
}

export interface SubmissionArtifactEvent {
  type: "artifact";
  commit_id: number;
  seq: number;
  kind: string;
  url: string;
}

export interface SubmissionFinishedEvent {
  type: "finished";
  commit_id: number;
  seq: number;
  status: FinishedStatus;
  num_correct_cases: number;
  score: number;
  compilation_message: string;
  compilation_error: string;
  started_at: string;
  finished_at: string;
}

export interface SubmissionStreamErrorEvent {
  type: "error";
  commit_id: number;
  seq: number;
  message: string;
}

/** Every frame the submission stream can carry. */
export type SubmissionEvent =
  | SubmissionSnapshotEvent
  | SubmissionStatusEvent
  | SubmissionCompilationEvent
  | SubmissionCaseResultEvent
  | SubmissionArtifactEvent
  | SubmissionFinishedEvent
  | SubmissionStreamErrorEvent;

export interface SubmissionEventHandlers {
  onSnapshot?: (event: SubmissionSnapshotEvent) => void;
  onStatus?: (event: SubmissionStatusEvent) => void;
  onCompilation?: (event: SubmissionCompilationEvent) => void;
  onCaseResult?: (event: SubmissionCaseResultEvent) => void;
  onArtifact?: (event: SubmissionArtifactEvent) => void;
  onFinished?: (event: SubmissionFinishedEvent) => void;
  /** A server-sent `error` frame; the stream ends after it. */
  onError?: (event: SubmissionStreamErrorEvent) => void;
  /** The SSE connection dropped and EventSource gave up reconnecting. */
  onConnectionError?: () => void;
}

const SSE_EVENT_NAMES = [
  "snapshot",
  "status",
  "compilation",
  "case_result",
  "artifact",
  "finished",
  "error",
] as const;

/**
 * Uploads a single source file for `exerciseId`. Uses `fetch` directly
 * (rather than `apiRequest`) and deliberately omits `Content-Type` so the
 * browser sets the multipart boundary.
 */
export async function createSubmission(
  exerciseId: number,
  file: File,
): Promise<QueuedSubmission> {
  const form = new FormData();
  form.append("exercise_id", String(exerciseId));
  form.append("file", file);

  const response = await fetch(`${API_BASE_URL}/api/v1/submissions`, {
    method: "POST",
    credentials: "include",
    body: form,
  });

  if (!response.ok) {
    throw new Error(await readApiError(response));
  }

  return readApiBody<QueuedSubmission>(response);
}

/** Validates a parsed frame and narrows it to a known event. */
function parseSubmissionEvent(value: unknown): SubmissionEvent | null {
  if (!isRecord(value) || typeof value.type !== "string") {
    return null;
  }
  switch (value.type) {
    case "snapshot":
    case "status":
    case "compilation":
    case "case_result":
    case "artifact":
    case "finished":
    case "error":
      return value as unknown as SubmissionEvent;
    default:
      return null;
  }
}

/**
 * Subscribes to a commit's live judging stream. The backend replays the
 * current state in a `snapshot` frame first, then forwards judge events, so a
 * reloaded page can simply re-subscribe. Returns an unsubscribe function.
 */
export function subscribeSubmissionEvents(
  commitId: number,
  handlers: SubmissionEventHandlers,
): () => void {
  const source = new EventSource(
    `${API_BASE_URL}/api/v1/submissions/${String(commitId)}/events`,
    { withCredentials: true },
  );
  let closed = false;

  const close = () => {
    if (!closed) {
      closed = true;
      source.close();
    }
  };

  const handleFrame = (event: Event) => {
    if (!(event instanceof MessageEvent)) return;
    const data: unknown = event.data;
    if (typeof data !== "string") return;

    let parsed: unknown;
    try {
      parsed = JSON.parse(data);
    } catch {
      return;
    }

    const submissionEvent = parseSubmissionEvent(parsed);
    if (!submissionEvent) return;

    switch (submissionEvent.type) {
      case "snapshot":
        handlers.onSnapshot?.(submissionEvent);
        break;
      case "status":
        handlers.onStatus?.(submissionEvent);
        break;
      case "compilation":
        handlers.onCompilation?.(submissionEvent);
        break;
      case "case_result":
        handlers.onCaseResult?.(submissionEvent);
        break;
      case "artifact":
        handlers.onArtifact?.(submissionEvent);
        break;
      case "finished":
        handlers.onFinished?.(submissionEvent);
        close();
        break;
      case "error":
        handlers.onError?.(submissionEvent);
        close();
        break;
    }
  };

  for (const name of SSE_EVENT_NAMES) {
    source.addEventListener(name, handleFrame);
  }
  // Fallback for servers/proxies that deliver frames without an event name.
  source.addEventListener("message", handleFrame);

  source.addEventListener("error", () => {
    if (!closed && source.readyState === EventSource.CLOSED) {
      closed = true;
      handlers.onConnectionError?.();
    }
  });

  return close;
}
