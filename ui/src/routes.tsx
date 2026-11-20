// SPDX-License-Identifier: MIT
// Manifest of the six role pages. "/" is not a page: the Gate sends admins to /setups and everyone else to /usage (D41). Each page lives alone in pages/<name>/index.tsx (default export) and is
// lazy-loaded. A page task edits only its own directory; adding a seventh page is one line here.
import { lazy, type ComponentType, type LazyExoticComponent } from "react";

export interface PageRoute {
  path: string;
  /** Navigation label (and page title). */
  title: string;
  /** Rendered and routable for admins only (the API enforces; the UI reflects). */
  admin?: boolean;
  Component: LazyExoticComponent<ComponentType>;
}

export const pageRoutes: PageRoute[] = [
  { path: "/usage", title: "My usage", Component: lazy(() => import("./pages/usage")) },
  { path: "/setups", title: "Setups", Component: lazy(() => import("./pages/setups")) },
  { path: "/admin/users", title: "Users", admin: true, Component: lazy(() => import("./pages/admin-users")) },
  { path: "/admin/setups", title: "Setup inventory", admin: true, Component: lazy(() => import("./pages/admin-setups")) },
  { path: "/admin/settings", title: "Settings", admin: true, Component: lazy(() => import("./pages/admin-settings")) },
  { path: "/admin/audit", title: "Audit log", admin: true, Component: lazy(() => import("./pages/admin-audit")) },
];
