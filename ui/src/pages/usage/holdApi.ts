// SPDX-License-Identifier: MIT
// API wrappers and display helpers shared by the My usage and Setups pages (HOLD-2).
import { api } from "../../api/client";
import type { components } from "../../api/schema.gen";
import { formatDuration } from "../../lib/format";

type S = components["schemas"];
export type Hold = S["Hold"];
export type LedgerResponse = S["LedgerResponse"];
export type LedgerEntry = S["LedgerEntry"];
export type UsageResponse = S["UsageResponse"];
export type SetupList = S["SetupList"];
export type Setup = S["Setup"];

export const listActiveHolds = () => api.get<{ holds: Hold[] }>("/api/v1/me/holds?state=active");

const periodQuery = (period?: string) => (period ? `?period=${encodeURIComponent(period)}` : "");
export const getMyUsage = (period?: string) => api.get<UsageResponse>(`/api/v1/me/usage${periodQuery(period)}`);
export const getMyLedger = (period?: string) => api.get<LedgerResponse>(`/api/v1/me/ledger${periodQuery(period)}`);

/** Normal release: empty body, so the API applies the teardown check (HOLD-2). */
export const releaseHold = (holdId: number) => api.post<Hold>(`/api/v1/holds/${holdId}/release`);

/** Display only: elapsed time since an API timestamp, as "2 d 4 h". */
export function elapsedSince(iso: string | null | undefined, now: number = Date.now()): string {
  if (!iso) return "—";
  const t = Date.parse(iso);
  return Number.isNaN(t) ? "—" : formatDuration((now - t) / 1000);
}
