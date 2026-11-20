// SPDX-License-Identifier: MIT
import { Navigate, useLocation } from "react-router-dom";
import { Alert, Button } from "../components";
import ChangePassword from "../pages/change-password";
import Login from "../pages/login";
import Unauthorized from "../pages/unauthorized";
import { Layout } from "./Layout";
import { useState } from "react";
import { isAdmin, useSessionState } from "./session";

const PUBLIC_ONLY = new Set(["/login", "/unauthorized", "/change-password"]);

/**
 * Decides what to render from the session alone (UI-5, AUTH-5):
 * no session -> /login; must_change_password -> only the change-password screen;
 * no roles -> only /unauthorized, whatever URL was requested; otherwise the app layout
 * ("/" redirects by role, D41).
 */
export function Gate() {
  const { state, refresh } = useSessionState();
  const { pathname, search } = useLocation();

  if (state.status === "loading") return <p className="public-column muted">Loading…</p>;

  if (state.status === "error") return <SessionError message={state.message} retry={refresh} />;

  if (state.status === "anonymous") {
    // UI-7: the backend redirects to "/?auth_error=<code>"; forward the code to the sign-in page.
    const authError = new URLSearchParams(search).get("auth_error");
    const to = authError ? `/login?auth_error=${encodeURIComponent(authError)}` : "/login";
    return pathname === "/login" ? <Login /> : <Navigate to={to} replace />;
  }

  const s = state.session;
  if (s.must_change_password) {
    return pathname === "/change-password" ? <ChangePassword /> : <Navigate to="/change-password" replace />;
  }
  if (s.roles.length === 0) {
    return pathname === "/unauthorized" ? <Unauthorized /> : <Navigate to="/unauthorized" replace />;
  }
  // "/" has no page of its own (D41): admins land on Setups, everyone else on My usage. Other URLs stay as requested.
  if (pathname === "/" || PUBLIC_ONLY.has(pathname)) {
    return <Navigate to={isAdmin(s) ? "/setups" : "/usage"} replace />;
  }
  return <Layout />;
}

/** The first GET /session failed for a reason other than 401, so we do not know whether the user is signed in. */
function SessionError({ message, retry }: { message: string; retry: () => Promise<void> }) {
  const [busy, setBusy] = useState(false);
  return (
    <div className="public-column">
      <h1>Hourglass</h1>
      <Alert tone="error">{message}</Alert>
      <div>
        <Button
          variant="primary"
          disabled={busy}
          onClick={() => {
            setBusy(true);
            void retry().finally(() => setBusy(false));
          }}
        >
          Retry
        </Button>
      </div>
    </div>
  );
}
