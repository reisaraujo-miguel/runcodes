import { useCallback, useEffect, useRef, useState } from "react";

import { errorMessage } from "@/lib/errors";

export interface AsyncState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
}

/** The resolved value of one request, tagged with the request it belongs to. */
interface Settled<T> {
  token: string;
  data: T | null;
  error: string | null;
}

/**
 * Loads a value from the API and tracks its loading / error / data state.
 *
 * `load` is read from a ref, so a caller does not need to memoize it; re-run the
 * request by passing a `key` that changes when its inputs change, or by calling
 * the returned `reload`.
 *
 * The effect never sets state synchronously: it only stores the result of the
 * request it started, tagged with that request's token. `loading` is derived by
 * comparing the stored token with the current one, which keeps the render free
 * of cascading updates.
 */
export function useAsync<T>(
  load: () => Promise<T>,
  key: string | number = "default",
  fallbackError = "Não foi possível carregar os dados.",
): AsyncState<T> & { reload: () => void } {
  const [nonce, setNonce] = useState(0);
  const [settled, setSettled] = useState<Settled<T>>({
    token: "",
    data: null,
    error: null,
  });
  const loadRef = useRef(load);

  useEffect(() => {
    loadRef.current = load;
  });

  const token = `${String(key)}#${String(nonce)}`;

  useEffect(() => {
    let cancelled = false;

    loadRef
      .current()
      .then((data) => {
        if (!cancelled) setSettled({ token, data, error: null });
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setSettled({
            token,
            data: null,
            error: errorMessage(error, fallbackError),
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [token, fallbackError]);

  const reload = useCallback(() => {
    setNonce((n) => n + 1);
  }, []);

  const isCurrent = settled.token === token;
  return {
    // Keep the previous value visible while a new request is in flight so the
    // view does not flash empty; `loading` tells the caller to show a spinner.
    data: settled.data,
    loading: !isCurrent,
    error: isCurrent ? settled.error : null,
    reload,
  };
}
