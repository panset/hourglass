// SPDX-License-Identifier: MIT
// Thin same-origin fetch client (UI-2, UI-4). Types come from schema.gen.ts.
import type { ErrorCode } from "./types";

/** Error shape of every non-2xx API response: {"error": {"code", "message"}}. */
export class ApiError extends Error {
  readonly code: ErrorCode | "network" | "unknown";
  readonly status: number;
  readonly field?: string;

  constructor(status: number, code: ApiError["code"], message: string, field?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.field = field;
  }
}

/** Base URL for development from another origin (UI-4). Empty means same origin. */
export const API_BASE: string = (import.meta.env?.VITE_API_BASE as string | undefined) ?? "";

let csrfToken: string | undefined;
let unauthorizedHandler: (() => void) | undefined;

/** The session response carries the CSRF token; the shell stores it here (AUTH-3). */
export function setCsrfToken(token: string | undefined): void {
  csrfToken = token;
}

/** Called on any 401 (except requests that opt out); the shell sends the user to /login. */
export function setUnauthorizedHandler(handler: (() => void) | undefined): void {
  unauthorizedHandler = handler;
}

export interface RequestOptions {
  body?: unknown;
  /** Set for calls where a 401 is an expected answer (wrong password). */
  skipUnauthorizedHandler?: boolean;
  signal?: AbortSignal;
}

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

export async function request<T>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (!SAFE_METHODS.has(method) && csrfToken) headers["X-CSRF-Token"] = csrfToken;

  let res: Response;
  try {
    res = await fetch(API_BASE + path, {
      method,
      headers,
      credentials: API_BASE ? "include" : "same-origin",
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      signal: opts.signal,
    });
  } catch (e) {
    if (e instanceof DOMException && e.name === "AbortError") throw e;
    throw new ApiError(0, "network", "Cannot reach the server.");
  }

  if (res.ok) {
    if (res.status === 204) return undefined as T;
    const text = await res.text();
    return (text ? JSON.parse(text) : undefined) as T;
  }

  let code: ApiError["code"] = "unknown";
  let message = res.statusText || `Request failed (${res.status})`;
  let field: string | undefined;
  try {
    const parsed = (await res.json()) as { error?: { code?: ErrorCode; message?: string; field?: string } };
    if (parsed.error) {
      code = parsed.error.code ?? "unknown";
      message = parsed.error.message ?? message;
      field = parsed.error.field;
    }
  } catch {
    // Not JSON: keep the generic message.
  }
  if (res.status === 401 && !opts.skipUnauthorizedHandler) unauthorizedHandler?.();
  throw new ApiError(res.status, code, message, field);
}

export const api = {
  get: <T>(path: string, opts?: Omit<RequestOptions, "body">) => request<T>("GET", path, opts),
  post: <T>(path: string, body?: unknown, opts?: Omit<RequestOptions, "body">) =>
    request<T>("POST", path, { ...opts, body }),
  put: <T>(path: string, body?: unknown, opts?: Omit<RequestOptions, "body">) =>
    request<T>("PUT", path, { ...opts, body }),
  patch: <T>(path: string, body?: unknown, opts?: Omit<RequestOptions, "body">) =>
    request<T>("PATCH", path, { ...opts, body }),
  delete: <T>(path: string, opts?: Omit<RequestOptions, "body">) => request<T>("DELETE", path, opts),
};
