// SPDX-License-Identifier: MIT
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { setCsrfToken, setUnauthorizedHandler } from "../api/client";
import { getSession, logout } from "../api/endpoints";
import type { Session } from "../api/types";

export type SessionState =
  | { status: "loading"; session: undefined }
  | { status: "anonymous"; session: undefined }
  | { status: "ready"; session: Session };

interface SessionApi {
  state: SessionState;
  /** Store a fresh session (after sign-in or password change). */
  setSession: (s: Session) => void;
  /** Re-read GET /api/v1/session. */
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
}

const SessionContext = createContext<SessionApi | null>(null);

/** Loads GET /api/v1/session once (UI-5) and keeps the CSRF token and 401 handling wired. */
export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<SessionState>({ status: "loading", session: undefined });

  const apply = useCallback((s: Session | undefined) => {
    setCsrfToken(s?.csrf_token);
    setState(s ? { status: "ready", session: s } : { status: "anonymous", session: undefined });
  }, []);

  const refresh = useCallback(async () => {
    try {
      apply(await getSession());
    } catch {
      apply(undefined);
    }
  }, [apply]);

  useEffect(() => {
    setUnauthorizedHandler(() => apply(undefined));
    void refresh();
    return () => setUnauthorizedHandler(undefined);
  }, [apply, refresh]);

  const signOut = useCallback(async () => {
    try {
      await logout();
    } catch {
      // The session may already be gone; either way the user ends up signed out.
    }
    apply(undefined);
  }, [apply]);

  const value = useMemo<SessionApi>(
    () => ({ state, setSession: apply, refresh, signOut }),
    [state, apply, refresh, signOut],
  );
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSessionState(): SessionApi {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSessionState needs a <SessionProvider>");
  return ctx;
}

/** The signed-in session. Only call below the Gate, where a session is guaranteed. */
export function useSession(): Session {
  const { state } = useSessionState();
  if (state.status !== "ready") throw new Error("useSession called without a session");
  return state.session;
}

export function isAdmin(s: Session): boolean {
  return s.roles.includes("admin");
}
