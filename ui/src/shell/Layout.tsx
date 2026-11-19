// SPDX-License-Identifier: MIT
import { Suspense } from "react";
import { Route, Routes } from "react-router-dom";
import NotFound from "../pages/not-found";
import { pageRoutes } from "../routes";
import { isAdmin, useSession } from "./session";
import { TopBar } from "./TopBar";

/** The app layout for a user with a role: top bar plus the lazy-loaded role pages. */
export function Layout() {
  const admin = isAdmin(useSession());
  return (
    <>
      <TopBar />
      <main>
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
      </main>
    </>
  );
}
