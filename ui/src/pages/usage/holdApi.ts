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

/** The two Ledger views (A5): `day` requests `rollup=day`, `poll` sends no `rollup`. */
export type LedgerView = "day" | "poll";
/** Rows per Ledger page (UI-8). */
export const LEDGER_PAGE_SIZE = 20;

/** One page of the ledger: `limit=20`, plus `rollup=day` in the By day view, plus the cursor of the page. */
export const getMyLedgerPage = (period: string | undefined, view: LedgerView, cursor?: string) => {
  const q = new URLSearchParams();
  if (period) q.set("period", period);
  if (view === "day") q.set("rollup", "day");
  q.set("limit", String(LEDGER_PAGE_SIZE));
  if (cursor) q.set("cursor", cursor);
  return api.get<LedgerResponse>(`/api/v1/me/ledger?${q.toString()}`);
};

/**
 * Normal release: empty body. The API answers 200 with the released hold, or 202 with the still-active
 * hold and `release_requested_at` set, when the setup is still deployed (HOLD-6).
 */
export const releaseHold = (holdId: number) => api.post<Hold>(`/api/v1/holds/${holdId}/release`);

/** True when the release answer is the pending one (202): the hold is still active and waiting for teardown. */
export const isPendingRelease = (h: Hold | undefined): boolean => !!h && h.active && !!h.release_requested_at;

/** Display only: elapsed time since an API timestamp, as "2 d 4 h". */
export function elapsedSince(iso: string | null | undefined, now: number = Date.now()): string {
  if (!iso) return "—";
  const t = Date.parse(iso);
  return Number.isNaN(t) ? "—" : formatDuration((now - t) / 1000);
}
