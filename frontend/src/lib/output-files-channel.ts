import {
  parseSubmissionOutputSnapshot,
  type SubmissionOutputSnapshot,
} from "@/lib/api";

/**
 * The generated-file listing is pushed over a WebSocket instead of being
 * polled. A proxy that drops the upgrade, a browser without WebSocket, and a
 * flaky connection all have to end in the same place: the caller reads the
 * listing once as a fallback instead of waiting for frames that will never
 * arrive.
 *
 * This module owns that lifecycle — connect, retry, fall back, close — so it can
 * be tested with an injected socket and timers instead of a real browser.
 */

/** The subset of `WebSocket` the channel uses. */
export interface OutputSocket {
  onmessage: ((message: { data: unknown }) => void) | null;
  onclose: (() => void) | null;
  close: () => void;
}

export interface OutputFilesChannelOptions {
  url: string;
  /** Called for every valid frame, in arrival order. */
  onFrame: (snapshot: SubmissionOutputSnapshot) => void;
  /** Called when the push channel is unavailable and the read must be one-shot. */
  onFallback: () => void;
  /** Called before each connection attempt, including retries. */
  onConnect?: () => void;
  createSocket?: (url: string) => OutputSocket;
  setTimer?: (callback: () => void, delayMs: number) => number;
  clearTimer?: (handle: number) => void;
}

export interface OutputFilesChannel {
  close: () => void;
}

/** How many reconnects are attempted before the caller falls back to a read. */
export const MAX_OUTPUT_SOCKET_ATTEMPTS = 2;

function defaultCreateSocket(url: string): OutputSocket {
  return new WebSocket(url) as unknown as OutputSocket;
}

export function connectOutputFilesChannel(
  options: OutputFilesChannelOptions,
): OutputFilesChannel {
  const createSocket = options.createSocket ?? defaultCreateSocket;
  const setTimer =
    options.setTimer ??
    ((callback, delayMs) => window.setTimeout(callback, delayMs));
  const clearTimer =
    options.clearTimer ??
    ((handle) => {
      window.clearTimeout(handle);
    });

  let closed = false;
  let complete = false;
  let attempts = 0;
  let retry: number | undefined;
  let socket: OutputSocket | null = null;

  const fallBack = () => {
    if (!closed) options.onFallback();
  };

  const connect = () => {
    if (closed) return;
    if (!options.createSocket && typeof WebSocket === "undefined") {
      fallBack();
      return;
    }

    options.onConnect?.();

    let next: OutputSocket;
    try {
      next = createSocket(options.url);
    } catch {
      fallBack();
      return;
    }
    socket = next;

    next.onmessage = (message) => {
      if (closed || typeof message.data !== "string") return;

      let parsed: unknown;
      try {
        parsed = JSON.parse(message.data);
      } catch {
        return;
      }

      const snapshot = parseSubmissionOutputSnapshot(parsed);
      if (!snapshot) return;

      // The last frame of a run is terminal; the server closes after it, and a
      // reconnect would only replay the same state.
      if (snapshot.complete) complete = true;
      options.onFrame(snapshot);
    };

    next.onclose = () => {
      socket = null;
      if (closed || complete) return;

      // A dropped socket is not a reason to poll: retry the push channel a
      // couple of times, then fall back to one read per artifact event.
      if (attempts >= MAX_OUTPUT_SOCKET_ATTEMPTS) {
        fallBack();
        return;
      }
      attempts += 1;
      retry = setTimer(connect, attempts * 1000);
    };
  };

  connect();

  return {
    close() {
      closed = true;
      if (retry !== undefined) {
        clearTimer(retry);
        retry = undefined;
      }
      socket?.close();
      socket = null;
    },
  };
}
