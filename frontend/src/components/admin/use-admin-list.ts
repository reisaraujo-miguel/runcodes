import { useCallback, useRef, useState } from "react";

import { useAsync } from "@/hooks/use-async";
import { ADMIN_PAGE_LIMIT } from "@/lib/api";
import { errorMessage } from "@/lib/errors";

/** A row an admin list can address by id (every admin listing has one). */
interface Identified {
  id: number;
}

export interface AdminListResult<T> {
  /** The rows loaded so far: the first page plus any appended pages. */
  items: T[];
  /** True while the first page of the current query is loading. */
  loading: boolean;
  /** True while an additional page is loading. */
  loadingMore: boolean;
  /** The load error, if the last request failed. */
  error: string | null;
  /** The term currently applied to the API (not the draft in the input). */
  query: string;
  /** Applies a search term, restarting from the first page. */
  search: (query: string) => void;
  /** Re-fetches from the first page, e.g. after a mutation. */
  reload: () => void;
  /** Retries the request that just failed without resetting pagination. */
  retry: () => void;
  /** Appends the next page. Only meaningful while `hasMore` is true. */
  loadMore: () => void;
  /** Whether the last page was full, so more rows may exist. */
  hasMore: boolean;
  /** Replaces a row in place, matched by id. */
  update: (id: number, item: T) => void;
  /** Drops a row from the list, matched by id. */
  remove: (id: number) => void;
}

function mergeById<T extends Identified>(previous: T[], incoming: T[]): T[] {
  const seen = new Set(previous.map((item) => item.id));
  return [...previous, ...incoming.filter((item) => !seen.has(item.id))];
}

/**
 * Loads a paginated admin listing on top of `useAsync`.
 *
 * The API caps a page at `ADMIN_PAGE_LIMIT` and reports no total, so the list
 * keeps the pages it has accumulated and offers "Carregar mais" while the last
 * page came back full. A search resets the accumulation; a mutation can patch
 * the loaded rows in place without dropping the pages already fetched.
 *
 * Responses are applied from the async loader (not a state-syncing effect), and
 * a request id discards a reply that arrives after the inputs have moved on.
 */
export function useAdminList<T extends Identified>(
  load: (query: string, offset: number) => Promise<T[]>,
  keyPrefix: string,
  fallbackError: string,
): AdminListResult<T> {
  const [params, setParams] = useState({ query: "", offset: 0, nonce: 0 });
  const [items, setItems] = useState<T[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const requestId = useRef(0);

  const page = useAsync(
    async () => {
      const id = requestId.current + 1;
      requestId.current = id;
      try {
        const chunk = await load(params.query, params.offset);
        if (id !== requestId.current) return;
        setItems((previous) =>
          params.offset === 0 ? chunk : mergeById(previous, chunk),
        );
        setHasMore(chunk.length >= ADMIN_PAGE_LIMIT);
        setError(null);
      } catch (loadError) {
        if (id !== requestId.current) return;
        setError(errorMessage(loadError, fallbackError));
      }
    },
    `${keyPrefix}:${params.query}:${String(params.offset)}:${String(params.nonce)}`,
    fallbackError,
  );

  const search = useCallback((query: string) => {
    requestId.current += 1;
    setItems([]);
    setHasMore(false);
    setError(null);
    setParams((previous) => ({
      query: query.trim(),
      offset: 0,
      nonce: previous.nonce + 1,
    }));
  }, []);

  const reload = useCallback(() => {
    requestId.current += 1;
    setItems([]);
    setHasMore(false);
    setError(null);
    setParams((previous) => ({
      ...previous,
      offset: 0,
      nonce: previous.nonce + 1,
    }));
  }, []);

  const retry = useCallback(() => {
    requestId.current += 1;
    setError(null);
    setParams((previous) => ({ ...previous, nonce: previous.nonce + 1 }));
  }, []);

  const loadMore = useCallback(() => {
    setParams((previous) => ({
      ...previous,
      offset: previous.offset + ADMIN_PAGE_LIMIT,
    }));
  }, []);

  const update = useCallback((id: number, item: T) => {
    setItems((previous) => previous.map((row) => (row.id === id ? item : row)));
  }, []);

  const remove = useCallback((id: number) => {
    setItems((previous) => previous.filter((row) => row.id !== id));
  }, []);

  return {
    items,
    loading: page.loading && params.offset === 0,
    loadingMore: page.loading && params.offset > 0,
    error,
    query: params.query,
    search,
    reload,
    retry,
    loadMore,
    hasMore,
    update,
    remove,
  };
}
