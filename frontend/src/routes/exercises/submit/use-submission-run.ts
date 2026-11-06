import { useEffect, useMemo, useRef, useState } from "react";

import {
  getSubmissionOutputFiles,
  parseSubmissionOutputSnapshot,
  submissionOutputEventsUrl,
  subscribeSubmissionEvents,
  type CommitStatus,
  type SubmissionArtifact,
  type SubmissionOutputFile,
  type SubmissionStatusEvent,
} from "@/lib/api";
import {
  applyArtifactStatus,
  pendingUploads as pendingUploadsFrom,
  type ArtifactStatus,
} from "@/lib/artifact-progress";
import { isTerminalStatus } from "@/lib/submission-status";
import {
  upsertCaseResult,
  viewFromEvent,
  viewFromSnapshot,
  type CaseResultView,
  type CompilationInfo,
  type FinalSummary,
} from "@/lib/submission-view";

import { INITIAL_STATUS } from "./submit-page-helpers";

export interface SubmissionRunState {
  status: CommitStatus;
  /** 1-based submission position for this exercise, once the snapshot arrives. */
  submissionNumber: number | null;
  history: SubmissionStatusEvent[];
  compilation: CompilationInfo | null;
  results: CaseResultView[];
  artifacts: SubmissionArtifact[];
  /** Upload state of each case that generated files. */
  artifactStatuses: ReadonlyMap<number, ArtifactStatus>;
  /** Generated files, pushed by the output-files WebSocket. */
  outputFiles: SubmissionOutputFile[];
  /** Cases whose generated files are still being uploaded. */
  pendingUploads: number[];
  final: FinalSummary | null;
  streamError: string | null;
  resetRunState: () => void;
}

/** One artifact entry per case, or one per run-level kind. */
function artifactKey(artifact: SubmissionArtifact): string {
  return artifact.test_case_id === undefined
    ? `kind:${artifact.kind}`
    : `case:${String(artifact.test_case_id)}`;
}

function mergeArtifacts(
  current: SubmissionArtifact[],
  incoming: readonly SubmissionArtifact[],
): SubmissionArtifact[] {
  if (incoming.length === 0) return current;

  const merged = new Map(
    current.map((artifact) => [artifactKey(artifact), artifact]),
  );
  for (const artifact of incoming) {
    merged.set(artifactKey(artifact), artifact);
  }

  return [...merged.values()];
}

function mergeStatuses(
  current: ReadonlyMap<number, ArtifactStatus>,
  incoming: readonly SubmissionArtifact[],
): Map<number, ArtifactStatus> {
  const next = new Map(current);
  for (const artifact of incoming) {
    if (
      artifact.test_case_id === undefined ||
      (artifact.state !== "pending" &&
        artifact.state !== "ready" &&
        artifact.state !== "failed")
    ) {
      continue;
    }

    next.set(artifact.test_case_id, {
      state: artifact.state,
      reason: artifact.reason,
      limitBytes: artifact.limit_bytes,
    });
  }

  return next;
}

function settlePendingStatuses(
  current: ReadonlyMap<number, ArtifactStatus>,
): Map<number, ArtifactStatus> {
  let changed = false;
  const next = new Map(current);
  for (const [testCaseId, status] of current) {
    if (status.state !== "pending") continue;
    changed = true;
    next.set(testCaseId, {
      state: "failed",
      reason: "upload_interrupted",
    });
  }

  return changed ? next : new Map(current);
}

/** Subscribes to one commit and exposes its live grading state. */
export function useSubmissionRun(commitId: number | null): SubmissionRunState {
  const [status, setStatus] = useState<CommitStatus>(INITIAL_STATUS);
  const [submissionNumber, setSubmissionNumber] = useState<number | null>(null);
  const [history, setHistory] = useState<SubmissionStatusEvent[]>([]);
  const [compilation, setCompilation] = useState<CompilationInfo | null>(null);
  const [results, setResults] = useState<CaseResultView[]>([]);
  const [artifacts, setArtifacts] = useState<SubmissionArtifact[]>([]);
  const [artifactStatuses, setArtifactStatuses] = useState<
    Map<number, ArtifactStatus>
  >(() => new Map());
  const [outputFiles, setOutputFiles] = useState<SubmissionOutputFile[]>([]);
  // artifactVersion changes on every artifact event. It only drives the
  // one-shot listing fallback when the WebSocket is unavailable; a healthy
  // socket pushes the listing itself.
  const [artifactVersion, setArtifactVersion] = useState(0);
  const [outputFallback, setOutputFallback] = useState(false);
  const receivedOutputSocket = useRef(false);
  const [final, setFinal] = useState<FinalSummary | null>(null);
  const [streamError, setStreamError] = useState<string | null>(null);

  useEffect(() => {
    if (commitId === null) return;

    const unsubscribe = subscribeSubmissionEvents(commitId, {
      onSnapshot: (event) => {
        // A frame only arrives on a live connection, so any earlier stream
        // warning is stale.
        setStreamError(null);
        setStatus(event.commit.status);
        setSubmissionNumber(
          typeof event.commit.number === "number" && event.commit.number > 0
            ? event.commit.number
            : null,
        );
        setHistory([]);
        setCompilation({
          compiled: event.commit.compiled,
          message: event.commit.compilation_message,
          error: event.commit.compilation_error,
        });
        setResults(event.results.map(viewFromSnapshot));
        // Artifact frames are transient; the snapshot rehydrates availability
        // after a reload for a settled submission. Merge rather than replace so
        // a reconnect cannot drop an artifact seen earlier in this run.
        setArtifacts((previous) =>
          mergeArtifacts(previous, event.artifacts ?? []),
        );
        setArtifactStatuses((previous) =>
          mergeStatuses(previous, event.artifacts ?? []),
        );
        setArtifactVersion((version) => version + 1);
        if (isTerminalStatus(event.commit.status)) {
          setFinal({
            status: event.commit.status,
            numCorrectCases: event.commit.num_correct_cases,
            score: event.commit.score,
            compilationMessage: event.commit.compilation_message,
            compilationError: event.commit.compilation_error,
          });
        } else {
          setFinal(null);
        }
      },
      onStatus: (event) => {
        setStreamError(null);
        setStatus(event.status);
        setHistory((previous) => [...previous, event]);
      },
      onCompilation: (event) => {
        setStreamError(null);
        setCompilation({
          compiled: event.compiled,
          message: event.message,
          error: event.error,
        });
      },
      onCaseResult: (event) => {
        setStreamError(null);
        setResults((previous) =>
          upsertCaseResult(previous, viewFromEvent(event)),
        );
      },
      onArtifact: (event) => {
        setStreamError(null);
        setArtifacts((previous) => mergeArtifacts(previous, [event]));
        setArtifactStatuses((previous) => applyArtifactStatus(previous, event));
        setArtifactVersion((version) => version + 1);
      },
      onFinished: (event) => {
        setStreamError(null);
        setStatus(event.status);
        setFinal({
          status: event.status,
          numCorrectCases: event.num_correct_cases,
          score: event.score,
          compilationMessage: event.compilation_message,
          compilationError: event.compilation_error,
        });
        setCompilation({
          compiled: event.compilation_error === "",
          message: event.compilation_message,
          error: event.compilation_error,
        });
        // The judge now drains uploads before this event, so a pending entry
        // here can only come from an older judge whose late events are lost.
        // It will never settle, so show the interrupted message instead of a
        // spinner that never stops.
        setArtifactStatuses(settlePendingStatuses);
      },
      onError: (event) => {
        setStreamError(event.message);
        setArtifactStatuses(settlePendingStatuses);
      },
      onConnectionError: () => {
        setStreamError(
          "Não foi possível obter os resultados em tempo real. Recarregue a página para tentar novamente.",
        );
        // The output socket can still report a real outcome; until it does, do
        // not leave a spinner that no event can settle.
        setArtifactStatuses(settlePendingStatuses);
      },
    });

    return unsubscribe;
  }, [commitId]);

  // The generated-file listing is pushed over a WebSocket instead of being
  // polled. It carries the per-case upload states too, so the panel can replace
  // a pending placeholder with a link or the reason there is none.
  useEffect(() => {
    if (commitId === null) return;

    let disposed = false;
    let complete = false;
    let attempts = 0;
    let retry: number | undefined;
    let socket: WebSocket | null = null;
    receivedOutputSocket.current = false;

    const fallBackToFetch = () => {
      if (!disposed) setOutputFallback(true);
    };

    const connect = () => {
      if (disposed) return;
      if (typeof WebSocket === "undefined") {
        fallBackToFetch();
        return;
      }

      // A new attempt has not delivered a snapshot yet, so a failed reconnect
      // must fall back to the one-shot listing rather than trust the previous
      // socket's state.
      receivedOutputSocket.current = false;
      try {
        socket = new WebSocket(submissionOutputEventsUrl(commitId));
      } catch {
        fallBackToFetch();
        return;
      }

      socket.onmessage = (message) => {
        if (disposed || typeof message.data !== "string") return;

        let parsed: unknown;
        try {
          parsed = JSON.parse(message.data);
        } catch {
          return;
        }

        const snapshot = parseSubmissionOutputSnapshot(parsed);
        if (!snapshot) return;

        receivedOutputSocket.current = true;
        setOutputFallback(false);
        setOutputFiles(snapshot.files);
        setArtifacts((previous) =>
          mergeArtifacts(previous, snapshot.artifacts),
        );
        setArtifactStatuses((previous) =>
          mergeStatuses(previous, snapshot.artifacts),
        );
        if (snapshot.complete) complete = true;
      };

      socket.onclose = () => {
        socket = null;
        if (disposed || complete) return;

        // A dropped socket is not a reason to poll: retry the push channel a
        // couple of times, then fall back to one read per artifact event.
        if (attempts >= 2) {
          fallBackToFetch();
          return;
        }
        attempts += 1;
        retry = window.setTimeout(connect, attempts * 1000);
      };
    };

    connect();

    return () => {
      disposed = true;
      if (retry !== undefined) window.clearTimeout(retry);
      socket?.close();
    };
  }, [commitId]);

  // Fallback for a browser or proxy that cannot keep the WebSocket open: read
  // the listing once, and once more whenever an artifact event says it grew.
  // There is deliberately no interval here.
  useEffect(() => {
    if (commitId === null || !outputFallback) return;

    let active = true;
    getSubmissionOutputFiles(commitId)
      .then((files) => {
        // A socket frame that arrived while this read was in flight is newer.
        if (active && !receivedOutputSocket.current) setOutputFiles(files);
      })
      .catch(() => {
        // The results themselves are unaffected by a listing failure.
      });

    return () => {
      active = false;
    };
  }, [artifactVersion, commitId, outputFallback]);

  const pendingUploads = useMemo(
    () => pendingUploadsFrom(artifactStatuses),
    [artifactStatuses],
  );

  function resetRunState() {
    setStatus(INITIAL_STATUS);
    setSubmissionNumber(null);
    setHistory([]);
    setCompilation(null);
    setResults([]);
    setArtifacts([]);
    setArtifactStatuses(new Map());
    setOutputFiles([]);
    setArtifactVersion(0);
    setOutputFallback(false);
    receivedOutputSocket.current = false;
    setFinal(null);
    setStreamError(null);
  }

  return {
    status,
    submissionNumber,
    history,
    compilation,
    results,
    artifacts,
    artifactStatuses,
    outputFiles,
    pendingUploads,
    final,
    streamError,
    resetRunState,
  };
}
