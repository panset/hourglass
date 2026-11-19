// SPDX-License-Identifier: MIT
import { useEffect, useRef } from "react";

/** Live numbers refresh every 60 seconds (UI-6). */
export const POLL_INTERVAL_MS = 60_000;

/**
 * Calls `fn` immediately (unless `immediate` is false) and then every `intervalMs`. Pauses while the tab is
 * hidden and refetches (and restarts the timer) when it becomes visible or the
 * window regains focus. `fn` may change between renders without restarting.
 */
export function usePolling(fn: () => void, intervalMs: number = POLL_INTERVAL_MS, enabled = true, immediate = true): void {
  const latest = useRef(fn);
  latest.current = fn;

  useEffect(() => {
    if (!enabled) return;
    let timer: ReturnType<typeof setInterval> | undefined;

    const stop = () => {
      if (timer !== undefined) clearInterval(timer);
      timer = undefined;
    };
    const start = () => {
      stop();
      timer = setInterval(() => latest.current(), intervalMs);
    };
    const refresh = () => {
      if (document.hidden) return;
      latest.current();
      start();
    };
    const onVisibility = () => {
      if (document.hidden) stop();
      else refresh();
    };

    if (immediate) latest.current();
    if (!document.hidden) start();
    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("focus", refresh);
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("focus", refresh);
    };
  }, [intervalMs, enabled, immediate]);
}
