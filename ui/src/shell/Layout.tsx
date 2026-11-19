// SPDX-License-Identifier: MIT
import { Suspense } from "react";
import { Route, Routes, useLocation } from "react-router-dom";
import { ErrorBoundary } from "../components";
import NotFound from "../pages/not-found";
import { pageRoutes } from "../routes";
import { isAdmin, useSession } from "./session";
import { TopBar } from "./TopBar";

/** The app layout for a user with a role: top bar plus the lazy-loaded role pages. */
export function Layout() {
  const admin = isAdmin(useSession());
  const { pathname } = useLocation();
  return (
    <>
      <TopBar />
      <main>
        {/* Keyed by path: navigating away from a failed page gives the next one a fresh start. */}
        <ErrorBoundary key={pathname}>
          <Suspense fallback={<p className="page muted">Loading…</p>}>
            <Routes>
              {pageRoutes
                .filter((r) => admin || !r.admin)
                .map(({ path, Component }) => (
                  <Route key={path} path={path} element={<Component />} />
                ))}
              <Route path="*" element={<NotFound />} />
            </Routes>
          </Suspense>
        </ErrorBoundary>
      </main>
    </>
  );
}
