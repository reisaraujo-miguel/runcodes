import { describe, expect, test } from "bun:test";

import {
  subscribeSubmissionEvents,
  type SubmissionEventHandlers,
} from "./submissions";

/**
 * Minimal stand-in for the platform's EventSource: it records listener
 * registration and lets a test push a frame, so the stream lifecycle can be
 * exercised without a server.
 */
class FakeEventSource {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;

  readonly url: string;
  readyState: number = FakeEventSource.CONNECTING;
  closeCalls = 0;

  private readonly listeners = new Map<string, Set<(event: Event) => void>>();

  constructor(url: string) {
    this.url = url;
  }

  addEventListener(name: string, listener: (event: Event) => void): void {
    const existing = this.listeners.get(name) ?? new Set();
    existing.add(listener);
    this.listeners.set(name, existing);
  }

  close(): void {
    this.closeCalls += 1;
    this.readyState = FakeEventSource.CLOSED;
  }

  /** Delivers one frame, as the browser would for a named SSE event. */
  emit(name: string, payload: unknown): void {
    this.emitRaw(name, JSON.stringify(payload));
  }

  /** Delivers a frame with raw (possibly invalid) data. */
  emitRaw(name: string, data: string): void {
    this.dispatch(name, new MessageEvent(name, { data }));
  }

  /**
   * Delivers a connection-level error, which the browser raises as a plain Event
   * (not a MessageEvent) — distinct from a server-sent `error` frame.
   */
  emitConnectionError(): void {
    this.dispatch("error", new Event("error"));
  }

  private dispatch(name: string, event: Event): void {
    for (const listener of this.listeners.get(name) ?? []) {
      listener(event);
    }
  }
}

function subscribe(handlers: SubmissionEventHandlers) {
  const sources: FakeEventSource[] = [];
  const unsubscribe = subscribeSubmissionEvents(1, handlers, {
    createEventSource: (url) => {
      const source = new FakeEventSource(url);
      sources.push(source);
      return source as unknown as EventSource;
    },
  });

  const source = sources[0];
  if (!source) throw new Error("no EventSource was created");

  return { source, unsubscribe };
}

const commit = (status: string) => ({
  id: 1,
  status,
  num_correct_cases: 0,
  score: 0,
  compiled: false,
  compilation_message: "",
  compilation_error: "",
  compilation_started: null,
  compilation_finished: null,
  created_at: "2025-01-01T00:00:00Z",
  s3_key: "uuid/main.c",
});

describe("subscribeSubmissionEvents", () => {
  test("closes the stream when the replayed snapshot is terminal", () => {
    const { source } = subscribe({});

    source.emit("snapshot", {
      type: "snapshot",
      commit: commit("completed"),
      results: [],
    });

    // The backend ends the response after a terminal snapshot, and EventSource
    // treats that as "reconnect": without an explicit close the client would
    // retry forever against a stream that keeps ending.
    expect(source.closeCalls).toBe(1);
  });

  test("keeps the stream open while the run is still in progress", () => {
    const { source } = subscribe({});

    for (const status of ["pending", "queued", "compiling", "running"]) {
      source.emit("snapshot", {
        type: "snapshot",
        commit: commit(status),
        results: [],
      });
    }

    expect(source.closeCalls).toBe(0);
  });

  test("closes on a terminal snapshot for the legacy-only statuses too", () => {
    const { source } = subscribe({});

    source.emit("snapshot", {
      type: "snapshot",
      commit: commit("plagiarism"),
      results: [],
    });

    expect(source.closeCalls).toBe(1);
  });

  test("closes once on the finished frame and ignores later frames", () => {
    const finished: string[] = [];
    const statuses: string[] = [];
    const { source } = subscribe({
      onFinished: (event) => finished.push(event.status),
      onStatus: (event) => statuses.push(event.status),
    });

    source.emit("finished", {
      type: "finished",
      commit_id: 1,
      seq: 4,
      status: "completed",
      num_correct_cases: 1,
      score: 100,
      compilation_message: "",
      compilation_error: "",
      started_at: "",
      finished_at: "",
    });
    source.emit("status", {
      type: "status",
      commit_id: 1,
      seq: 5,
      status: "running",
      at: "",
    });

    expect(finished).toEqual(["completed"]);
    expect(statuses).toEqual([]);
    expect(source.closeCalls).toBe(1);
  });

  test("closes on a server-sent error frame", () => {
    const errors: string[] = [];
    const { source } = subscribe({
      onError: (event) => errors.push(event.message),
    });

    source.emit("error", {
      type: "error",
      commit_id: 1,
      seq: 2,
      message: "boom",
    });

    expect(errors).toEqual(["boom"]);
    expect(source.closeCalls).toBe(1);
  });

  test("ignores malformed and unknown frames", () => {
    const snapshots: unknown[] = [];
    const { source } = subscribe({
      onSnapshot: (event) => snapshots.push(event),
    });

    source.emitRaw("snapshot", "not json");
    source.emit("snapshot", { type: "who_knows" });
    source.emit("snapshot", {});
    source.emitConnectionError();

    // None of them reached a handler or closed the stream.
    expect(snapshots).toHaveLength(0);
    expect(source.closeCalls).toBe(0);

    // A well-formed frame still gets through afterwards.
    source.emit("snapshot", {
      type: "snapshot",
      commit: commit("queued"),
      results: [],
    });
    expect(snapshots).toHaveLength(1);
  });

  test("reports a dropped connection only when the transport gave up", () => {
    const connectionErrors: number[] = [];
    const { source } = subscribe({
      onConnectionError: () => connectionErrors.push(1),
    });

    // Still reconnecting: EventSource reports connection errors while it retries.
    source.readyState = FakeEventSource.CONNECTING;
    source.emitConnectionError();
    expect(connectionErrors).toHaveLength(0);

    source.readyState = FakeEventSource.CLOSED;
    source.emitConnectionError();
    expect(connectionErrors).toHaveLength(1);
  });
});
