// SPDX-License-Identifier: MIT
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "../../api/client";

export interface ActionState {
  /** True while the request is in flight; disable the triggering button with it. */
  busy: boolean;
  /** The API error of the last attempt (`{error:{code,message}}`), cleared on the next one. */
  error: ApiError | undefined;
  /** Runs the request. Resolves true on success, false on failure (the error is stored). */
  run: (fn: () => Promise<unknown>) => Promise<boolean>;
  clear: () => void;
}

/** One state-changing action: in-flight flag plus the API error to show (UI rules). */
export function useAction(): ActionState {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError>();
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  const run = useCallback(async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(undefined);
    try {
      await fn();
      if (alive.current) setBusy(false);
      return true;
    } catch (e) {
      if (alive.current) {
        setError(e instanceof ApiError ? e : new ApiError(0, "unknown", String(e)));
        setBusy(false);
      }
      return false;
    }
  }, []);

  const clear = useCallback(() => setError(undefined), []);
  return { busy, error, run, clear };
}

/** Message for an ApiError, with the offending request field when the API names one. */
export function errorText(e: ApiError): string {
  return e.field ? `${e.message} (${e.field})` : e.message;
}
