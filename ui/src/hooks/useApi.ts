// SPDX-License-Identifier: MIT
import { useCallback, useEffect, useRef, useState, type DependencyList } from "react";
import { ApiError } from "../api/client";
import { usePolling } from "./usePolling";

export interface UseApiOptions {
  /** Refetch every this many milliseconds (pauses while the tab is hidden). */
  pollMs?: number;
  /** Skip fetching while false (e.g. data only admins may read). */
  enabled?: boolean;
}

export interface UseApiResult<T> {
  data: T | undefined;
  error: ApiError | undefined;
  /** True until the first response; refreshes keep showing the previous data. */
  loading: boolean;
  reload: () => void;
}

/** Runs `fetcher` on mount and whenever `deps` change. Stale responses are dropped. */
export function useApi<T>(
  fetcher: () => Promise<T>,
  deps: DependencyList = [],
  { pollMs, enabled = true }: UseApiOptions = {},
): UseApiResult<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<ApiError>();
  const [loading, setLoading] = useState(enabled);
  const seq = useRef(0);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const run = useCallback(() => {
    const mine = ++seq.current;
    fetcherRef
      .current()
      .then((value) => {
        if (mine !== seq.current) return;
        setData(value);
        setError(undefined);
        setLoading(false);
      })
      .catch((e: unknown) => {
        if (mine !== seq.current) return;
        setError(e instanceof ApiError ? e : new ApiError(0, "unknown", String(e)));
        setLoading(false);
      });
  }, []);

  // Dependency changes: reset to loading and refetch.
  useEffect(() => {
    if (!enabled) return;
    setLoading(true);
    run();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, ...deps]);

  usePolling(run, pollMs, pollMs !== undefined && enabled, false);

  useEffect(
    () => () => {
      seq.current++;
    },
    [],
  );

  return { data, error, loading, reload: run };
}
