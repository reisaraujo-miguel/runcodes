import { useEffect, useState } from "react";

import {
  subscribeSubmissionEvents,
  type CommitStatus,
  type SubmissionArtifact,
  type SubmissionStatusEvent,
} from "@/lib/api";
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
    setFinal(null);
    setStreamError(null);
  }

  return {
    status,
    history,
    compilation,
    results,
    artifacts,
    final,
    streamError,
    resetRunState,
  };
}
