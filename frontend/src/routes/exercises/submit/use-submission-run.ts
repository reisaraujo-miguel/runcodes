import { useEffect, useState } from "react";

import {
  subscribeSubmissionEvents,
  type CommitStatus,
  type SubmissionArtifact,
  type SubmissionStatusEvent,
} from "@/lib/api";
import { applyArtifactProgress } from "@/lib/artifact-progress";
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
  history: SubmissionStatusEvent[];
  compilation: CompilationInfo | null;
  results: CaseResultView[];
  artifacts: SubmissionArtifact[];
  /** Increments on every artifact event, so the file listing can be re-read. */
  artifactVersion: number;
  /** Cases whose generated files are still being uploaded. */
  pendingUploads: number[];
  final: FinalSummary | null;
  streamError: string | null;
  resetRunState: () => void;
}

/** Subscribes to one commit and exposes its live grading state. */
export function useSubmissionRun(commitId: number | null): SubmissionRunState {
  const [status, setStatus] = useState<CommitStatus>(INITIAL_STATUS);
  const [history, setHistory] = useState<SubmissionStatusEvent[]>([]);
  const [compilation, setCompilation] = useState<CompilationInfo | null>(null);
  const [results, setResults] = useState<CaseResultView[]>([]);
  const [artifacts, setArtifacts] = useState<SubmissionArtifact[]>([]);
  // artifactVersion changes on every artifact event, including repeats of a kind
  // the panel already knows: each one means new files are downloadable, so the
  // listing must be read again.
  const [artifactVersion, setArtifactVersion] = useState(0);
  const [pendingUploads, setPendingUploads] = useState<number[]>([]);
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
        setArtifacts((previous) => {
          const fromSnapshot = event.artifacts ?? [];
          if (fromSnapshot.length === 0) return previous;
          const merged = new Map(
            previous.map((artifact) => [artifact.kind, artifact]),
          );
          for (const artifact of fromSnapshot) {
            merged.set(artifact.kind, artifact);
          }
          return [...merged.values()];
        });
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
        setArtifacts((previous) =>
          previous.some((artifact) => artifact.kind === event.kind)
            ? previous
            : [...previous, { kind: event.kind }],
        );
        setPendingUploads((previous) => applyArtifactProgress(previous, event));
        // A ready event means the listing grew; a pending one mounts the panel so
        // the placeholder is visible. Both justify re-reading the listing.
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
      },
      onError: (event) => {
        setStreamError(event.message);
      },
      onConnectionError: () => {
        setStreamError(
          "Não foi possível obter os resultados em tempo real. Recarregue a página para tentar novamente.",
        );
      },
    });

    return unsubscribe;
  }, [commitId]);

  function resetRunState() {
    setStatus(INITIAL_STATUS);
    setHistory([]);
    setCompilation(null);
    setResults([]);
    setArtifacts([]);
    setArtifactVersion(0);
    setPendingUploads([]);
    setFinal(null);
    setStreamError(null);
  }

  return {
    status,
    history,
    compilation,
    results,
    artifacts,
    artifactVersion,
    pendingUploads,
    final,
    streamError,
    resetRunState,
  };
}
