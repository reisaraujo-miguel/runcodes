import { describe, expect, test } from "bun:test";

import {
  connectOutputFilesChannel,
  MAX_OUTPUT_SOCKET_ATTEMPTS,
  type OutputFilesChannel,
  type OutputSocket,
} from "./output-files-channel";

class FakeSocket implements OutputSocket {
  onmessage: ((message: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  closed = false;

  close() {
    this.closed = true;
  }

  emit(data: unknown) {
    this.onmessage?.({ data });
  }

  drop() {
    this.onclose?.();
  }
}

function frame(overrides: Record<string, unknown> = {}): string {
  return JSON.stringify({
    type: "outputfiles",
    files: [{ path: "test_v2_1/a.txt", size: 4 }],
    artifacts: [{ kind: "output", test_case_id: 7, number: 1, state: "ready" }],
    complete: false,
    ...overrides,
  });
}

interface Harness {
  channel: OutputFilesChannel;
  sockets: FakeSocket[];
  timers: { callback: () => void; delay: number }[];
  cleared: number[];
  frames: unknown[];
  fallbacks: number;
  connects: number;
  lastSocket: () => FakeSocket;
}

function harness(
  overrides: {
    createSocket?: (url: string) => OutputSocket;
  } = {},
): Harness {
  const state: Harness = {
    channel: undefined as unknown as OutputFilesChannel,
    sockets: [],
    timers: [],
    cleared: [],
    frames: [],
    fallbacks: 0,
    connects: 0,
    lastSocket: () => {
      const socket = state.sockets[state.sockets.length - 1];
      if (!socket) throw new Error("no socket was created");
      return socket;
    },
  };

  state.channel = connectOutputFilesChannel({
    url: "ws://localhost/api/v1/submissions/42/output-events",
    onFrame: (snapshot) => {
      state.frames.push(snapshot);
    },
    onFallback: () => {
      state.fallbacks += 1;
    },
    onConnect: () => {
      state.connects += 1;
    },
    createSocket:
      overrides.createSocket ??
      (() => {
        const socket = new FakeSocket();
        state.sockets.push(socket);
        return socket;
      }),
    setTimer: (callback, delay) => {
      state.timers.push({ callback, delay });
      return state.timers.length;
    },
    clearTimer: (handle) => {
      state.cleared.push(handle);
    },
  });

  return state;
}

describe("connectOutputFilesChannel", () => {
  test("delivers parsed frames and reports each attempt", () => {
    const state = harness();
    expect(state.connects).toBe(1);
    expect(state.sockets).toHaveLength(1);

    state.lastSocket().emit(frame());
    expect(state.frames).toHaveLength(1);
    expect(state.frames[0]).toMatchObject({
      type: "outputfiles",
      files: [{ path: "test_v2_1/a.txt", size: 4 }],
      artifacts: [
        { kind: "output", test_case_id: 7, number: 1, state: "ready" },
      ],
      complete: false,
    });

    state.channel.close();
  });

  test("ignores malformed, unknown and non-text frames", () => {
    const state = harness();
    state.lastSocket().emit("not json");
    state.lastSocket().emit(JSON.stringify({ type: "status" }));
    state.lastSocket().emit(JSON.stringify(null));
    state.lastSocket().emit(new ArrayBuffer(4));

    expect(state.frames).toHaveLength(0);
    state.channel.close();
  });

  test("retries a dropped socket with backoff, then falls back", () => {
    const state = harness();

    for (let attempt = 1; attempt <= MAX_OUTPUT_SOCKET_ATTEMPTS; attempt += 1) {
      state.lastSocket().drop();
      expect(state.timers).toHaveLength(attempt);
      expect(state.timers[attempt - 1]?.delay).toBe(attempt * 1000);
      state.timers[attempt - 1]?.callback();
      expect(state.sockets).toHaveLength(attempt + 1);
    }

    // One more drop is past the retry budget: the caller must read instead.
    state.lastSocket().drop();
    expect(state.fallbacks).toBe(1);
    expect(state.timers).toHaveLength(MAX_OUTPUT_SOCKET_ATTEMPTS);

    state.channel.close();
  });

  test("does not reconnect after the terminal frame", () => {
    const state = harness();
    state.lastSocket().emit(frame({ complete: true }));
    expect(state.frames).toHaveLength(1);

    state.lastSocket().drop();
    expect(state.timers).toHaveLength(0);
    expect(state.fallbacks).toBe(0);

    state.channel.close();
  });

  test("close closes the open socket", () => {
    const state = harness();
    state.channel.close();

    expect(state.sockets[0]?.closed).toBe(true);
    expect(state.sockets).toHaveLength(1);
  });

  test("close cancels a pending retry", () => {
    const state = harness();
    state.lastSocket().drop();
    expect(state.timers).toHaveLength(1);

    state.channel.close();

    expect(state.cleared).toHaveLength(1);
    // The cancelled retry never opens another socket.
    expect(state.sockets).toHaveLength(1);
  });

  test("falls back when the socket cannot be constructed", () => {
    const state = harness({
      createSocket: () => {
        throw new Error("no websocket");
      },
    });

    expect(state.fallbacks).toBe(1);
    expect(state.timers).toHaveLength(0);
    state.channel.close();
  });
});
