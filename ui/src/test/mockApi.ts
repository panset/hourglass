// SPDX-License-Identifier: MIT
import { vi } from "vitest";
import type { AdminUserList, Me, Pressure, Session } from "../api/types";

export type Handler = (req: { url: string; method: string; headers: Headers; body: unknown }) => unknown | Response;

export interface RecordedCall {
  method: string;
  url: string;
  headers: Headers;
  body: unknown;
}

/** Stubs global fetch with a "METHOD /path" table. Values are JSON bodies, Responses, or functions. */
export function mockApi(routes: Record<string, Handler | unknown>) {
  const calls: RecordedCall[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = (init?.method ?? "GET").toUpperCase();
    const headers = new Headers(init?.headers);
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ method, url, headers, body });
    const key = `${method} ${url.split("?")[0]}`;
    const entry = routes[key];
    if (entry === undefined) {
      return new Response(JSON.stringify({ error: { code: "not_found", message: `no mock for ${key}` } }), { status: 404 });
    }
    const value = typeof entry === "function" ? (entry as Handler)({ url, method, headers, body }) : entry;
    if (value instanceof Response) return value;
    return new Response(JSON.stringify(value), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  vi.stubGlobal("fetch", fetchMock);
  return { calls, fetchMock };
}

export function errorResponse(status: number, code: string, message: string, field?: string): Response {
  return new Response(JSON.stringify({ error: { code, message, field } }), { status });
}

export const developerSession: Session = {
  user_id: 2,
  username: "dana",
  email: "dana@example.com",
  name: "Dana Dev",
  display_name: "Dana Dev",
  auth_source: "oidc",
  status: "active",
  roles: ["developer"],
  csrf_token: "csrf-dana",
  must_change_password: false,
  is_local_admin: false,
};

export const adminSession: Session = {
  ...developerSession,
  user_id: 3,
  username: "riley",
  email: "riley@example.com",
  name: "Riley Admin",
  display_name: "Riley Admin",
  roles: ["admin"],
  csrf_token: "csrf-riley",
};

export const noRoleSession: Session = {
  ...developerSession,
  email: "morgan@example.com",
  name: "Morgan Q.",
  display_name: "Morgan Q.",
  roles: [],
  status: "pending",
};

export const localAdminSession: Session = {
  ...adminSession,
  csrf_token: "csrf-local",
  user_id: 1,
  username: "admin",
  email: "",
  name: "admin",
  display_name: "admin",
  auth_source: "local",
  is_local_admin: true,
};

export const me = { balance_tokens: 142.2 } as unknown as Me;
export const pressure = { smoothed: 0.82, raw: 0.8, multiplier: 2 } as unknown as Pressure;
export const pendingUsers = { pending_count: 2, users: [] } as unknown as AdminUserList;

/** Routes for the top bar's polled calls. */
export const barRoutes = {
  "GET /api/v1/me": me,
  "GET /api/v1/pressure": pressure,
  "GET /api/v1/admin/users": pendingUsers,
};
