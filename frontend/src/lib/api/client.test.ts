import { afterEach, expect, test } from "bun:test";

import { ApiRequestError, UNAUTHORIZED_EVENT, apiRequest } from "./client";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function respondWith(status: number): void {
  globalThis.fetch = (() =>
    Promise.resolve(
      new Response(JSON.stringify({ error_msg: "falhou" }), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    )) as unknown as typeof fetch;
}

test("a 401 asks the auth provider to clear the session", async () => {
  respondWith(401);

  let notifications = 0;
  const listener = () => {
    notifications += 1;
  };
  globalThis.addEventListener(UNAUTHORIZED_EVENT, listener);

  try {
    const error = await apiRequest("/anything").catch(
      (caught: unknown) => caught,
    );
    if (!(error instanceof ApiRequestError)) {
      throw new Error("expected an ApiRequestError");
    }
    expect(error.status).toBe(401);
    expect(notifications).toBe(1);
  } finally {
    globalThis.removeEventListener(UNAUTHORIZED_EVENT, listener);
  }
});

test("other failures do not clear the session", async () => {
  respondWith(403);

  let notifications = 0;
  const listener = () => {
    notifications += 1;
  };
  globalThis.addEventListener(UNAUTHORIZED_EVENT, listener);

  try {
    const error = await apiRequest("/anything").catch(
      (caught: unknown) => caught,
    );
    if (!(error instanceof ApiRequestError)) {
      throw new Error("expected an ApiRequestError");
    }
    expect(error.status).toBe(403);
    expect(notifications).toBe(0);
  } finally {
    globalThis.removeEventListener(UNAUTHORIZED_EVENT, listener);
  }
});
