// SPDX-License-Identifier: MIT
import { Navigate, useLocation } from "react-router-dom";
import ChangePassword from "../pages/change-password";
import Login from "../pages/login";
import Unauthorized from "../pages/unauthorized";
import { Layout } from "./Layout";
import { useSessionState } from "./session";

const PUBLIC_ONLY = new Set(["/login", "/unauthorized", "/change-password"]);

/**
 * Decides what to render from the session alone (UI-5, AUTH-5):
 * no session -> /login; must_change_password -> only the change-password screen;
 * no roles -> only /unauthorized, whatever URL was requested; otherwise the app layout.
 */
export function Gate() {
  const { state } = useSessionState();
  const { pathname } = useLocation();

  if (state.status === "loading") return <p className="public-column muted">Loading…</p>;

  if (state.status === "anonymous") {
    return pathname === "/login" ? <Login /> : <Navigate to="/login" replace />;
  }

  const s = state.session;
  if (s.must_change_password) {
    return pathname === "/change-password" ? <ChangePassword /> : <Navigate to="/change-password" replace />;
  }
  if (s.roles.length === 0) {
    return pathname === "/unauthorized" ? <Unauthorized /> : <Navigate to="/unauthorized" replace />;
  }
  if (PUBLIC_ONLY.has(pathname)) return <Navigate to="/" replace />;
  return <Layout />;
}
