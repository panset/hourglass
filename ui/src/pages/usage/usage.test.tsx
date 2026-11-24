// SPDX-License-Identifier: MIT
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import type { Me, Pressure } from "../../api/types";
import { ToastProvider } from "../../components";
import { SessionProvider, useSessionState } from "../../shell/session";
import { adminSession, developerSession, errorResponse, localAdminSession, mockApi } from "../../test/mockApi";
import type { Hold, LedgerEntry, LedgerResponse, UsageResponse } from "./holdApi";
import UsagePage from "./index";

function Ready({ children }: { children: ReactNode }) {
  return useSessionState().state.status === "ready" ? <>{children}</> : null;
}
function mount() {
  return render(
    <SessionProvider>
      <ToastProvider>
        <MemoryRouter>
          <Ready>
            <UsagePage />
          </Ready>
        </MemoryRouter>
      </ToastProvider>
    </SessionProvider>,
  );
}

const me: Me = {
  period: "2026-11",
  period_ends_at: "2026-12-01T00:00:00Z",
  balance_tokens: 142.2,
  allowance_tokens: 300,
  granted_tokens: 40,
  carryover_tokens: 0,
  debited_tokens: 197.8,
  burn_per_hour_tokens: 2,
  burning: [{ hold_id: 7, setup_id: 4, setup_name: "setup-04", weight: 1, weight_source: "default", multiplier: 2, tokens_per_hour: 2 }],
  projection: "date",
  projected_runout_at: "2026-11-21T12:00:00Z",
  hold_count: 1,
  hold_limit: 2,
  enforcement: true,
  overdrawn: false,
  can_acquire: true,
  cannot_acquire_reason: null,
};

const pressure: Pressure = {
  raw: 0.8,
  smoothed: 0.82,
  multiplier: 2,
  bands: {
    base: 0,
    steps: [
      { from: 0.5, multiplier: 1 },
      { from: 0.75, multiplier: 2 },
      { from: 0.9, multiplier: 3 },
    ],
  },
  source: "inventory",
  updated_at: "2026-11-18T17:30:00Z",
  setups_enabled: 11,
  setups_deployed: 9,
  history: [],
};

const hold: Hold = {
  id: 7,
  setup_id: 4,
  setup_name: "setup-04",
  setup_labels: { size: "medium", site: "sjc" },
  user_id: 2,
  active: true,
  weight: 1,
  weight_source: "default",
  weight_attributes: {},
  acquired_at: "2026-11-16T10:00:00Z",
  metering_started_at: "2026-11-16T12:00:00Z",
  deployed_at: "2026-11-16T11:00:00Z",
  release_requested_at: null,
  released_at: null,
  deployed: true,
  metering: { active: true, started_at: "2026-11-16T12:00:00Z", expected_start_at: null, reason: null },
  cost_per_hour_tokens: { now: 2, at_peak: 3 },
  charged_tokens: 10,
  released_by: null,
  release_reason: null,
  forced: false,
};

const usage: UsageResponse = {
  period: "2026-11",
  available_periods: ["2026-11", "2026-10"],
  total_debited_tokens: 6,
  days: [
    { date: "2026-11-01", debited_tokens: 0 },
    { date: "2026-11-02", debited_tokens: 4 },
    { date: "2026-11-03", debited_tokens: 2 },
  ],
};

const ledger: LedgerResponse = {
  period: "2026-11",
  available_periods: ["2026-11", "2026-10"],
  entries: [
    { id: 3, created_at: "2026-11-18T17:30:00Z", period: "2026-11", type: "debit", amount_tokens: -0.17, balance_after_tokens: 142, multiplier: 2, setup_name: "setup-04", hold_id: 7, actor_name: null, reason: null, entry_count: 1 },
    { id: 2, created_at: "2026-11-12T10:02:00Z", period: "2026-11", type: "grant", amount_tokens: 40, balance_after_tokens: 213.5, multiplier: null, setup_name: null, hold_id: null, actor_name: "Riley Admin", reason: "release week", entry_count: 1 },
  ],
  next_cursor: null,
};

const okRoutes = () => ({
  "GET /api/v1/session": developerSession,
  "GET /api/v1/me": me,
  "GET /api/v1/pressure": pressure,
  "GET /api/v1/me/holds": { holds: [hold] },
  "GET /api/v1/me/usage": usage,
  "GET /api/v1/me/ledger": ledger,
});

describe("My usage page", () => {
  it("shows a loading state first", async () => {
    mockApi({ ...okRoutes(), "GET /api/v1/me": () => new Promise(() => {}) as never });
    mount();
    expect((await screen.findAllByRole("status")).length).toBeGreaterThan(0);
  });

  it("UI-1 populated: stat cards, holds, chart and ledger come from API fields", async () => {
    mockApi(okRoutes());
    mount();
    expect(await screen.findByText("142.2")).toBeInTheDocument();
    expect(screen.getByText("of 300 allowance + 40 granted")).toBeInTheDocument();
    expect(screen.getByText("2.0 / h")).toBeInTheDocument();
    expect(screen.getByText("setup-04 · weight 1 × 2×")).toBeInTheDocument();
    expect(screen.getByText("Nov 21")).toBeInTheDocument();
    expect(screen.getByText("82% · 2×")).toBeInTheDocument();
    expect(screen.getByText("9 of 11 setups deployed")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /Pressure bands: free below 50, 1x to 75, 2x to 90, 3x above 90; currently 82 percent/ })).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Current holds · 1 of 2 allowed" })).toBeInTheDocument();
    const holds = await screen.findByRole("table", { name: "Current holds" });
    expect(within(holds).getByText("size=medium")).toBeInTheDocument();
    expect(within(holds).getByText("Deployed")).toBeInTheDocument();
    expect(within(holds).getByText("2.0 tokens / hour")).toBeInTheDocument();
    const bars = await screen.findAllByTestId("usage-bar");
    expect(bars).toHaveLength(3);
    expect(bars[2]).toHaveAttribute("data-today", "true");
    expect(bars[0]).toHaveAttribute("height", "2");
    const table = screen.getByRole("table", { name: "Ledger" });
    expect(within(table).getByText("Debit · 2×")).toBeInTheDocument();
    expect(within(table).getByText("Riley Admin: release week")).toBeInTheDocument();
    expect(within(table).getByText("+40")).toBeInTheDocument();
  });

  it("projected run-out shows 'not this month' when the API says so", async () => {
    mockApi({ ...okRoutes(), "GET /api/v1/me": { ...me, projection: "not_this_month", projected_runout_at: null } });
    mount();
    expect(await screen.findByText("not this month")).toBeInTheDocument();
  });

  it("empty: no holds and no ledger entries", async () => {
    mockApi({
      ...okRoutes(),
      "GET /api/v1/me/holds": { holds: [] },
      "GET /api/v1/me/ledger": { ...ledger, entries: [] },
    });
    mount();
    expect(await screen.findByText("You hold no setups.")).toBeInTheDocument();
    expect(await screen.findByText("No ledger entries for this period.")).toBeInTheDocument();
  });

  it("error: shows the API message", async () => {
    mockApi({
      ...okRoutes(),
      "GET /api/v1/me": () => errorResponse(500, "internal", "database is busy"),
    });
    mount();
    expect(await screen.findByText("database is busy")).toBeInTheDocument();
  });

  it("month picker reloads usage and ledger for the chosen period", async () => {
    const { calls } = mockApi(okRoutes());
    mount();
    const select = await screen.findByRole("combobox", { name: "Month" });
    expect(within(select).getAllByRole("option").map((o) => o.textContent)).toEqual(["November 2026", "October 2026"]);
    await userEvent.selectOptions(select, "2026-10");
    await waitFor(() => {
      expect(calls.some((c) => c.url === "/api/v1/me/usage?period=2026-10")).toBe(true);
      expect(calls.some((c) => c.url === "/api/v1/me/ledger?period=2026-10&rollup=day&limit=20")).toBe(true);
    });
  });

  it("HOLD-2 Release posts to the hold with the CSRF header and refetches", async () => {
    const { calls } = mockApi({ ...okRoutes(), "POST /api/v1/holds/7/release": { ...hold, active: false } });
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Release" }));
    await waitFor(() => expect(calls.some((c) => c.method === "POST")).toBe(true));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/holds/7/release");
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-dana");
    expect(post.body).toBeUndefined();
    await waitFor(() => expect(calls.filter((c) => c.url === "/api/v1/me/holds?state=active").length).toBe(2));
  });

  it("HOLD-6 a 202 shows the hold as Releasing without an error or a Released toast", async () => {
    let held: Hold = hold;
    mockApi({
      ...okRoutes(),
      "GET /api/v1/me/holds": () => ({ holds: [held] }),
      "POST /api/v1/holds/7/release": () => {
        held = { ...hold, release_requested_at: "2026-11-18T12:00:00Z" };
        return new Response(JSON.stringify(held), { status: 202, headers: { "Content-Type": "application/json" } });
      },
    });
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Release" }));
    expect(await screen.findByText("Releasing")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Release" })).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText("Released setup-04.")).not.toBeInTheDocument();
  });

  it("HOLD-6 a hold with release_requested_at renders Releasing and no Release button", async () => {
    mockApi({
      ...okRoutes(),
      "GET /api/v1/me/holds": { holds: [{ ...hold, release_requested_at: "2026-11-18T12:00:00Z" }] },
    });
    mount();
    expect(await screen.findByText("Releasing")).toBeInTheDocument();
    expect(screen.getByText(/requested/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Release" })).not.toBeInTheDocument();
  });

  it("the local admin sees no balance, no holds section and no personal calls", async () => {
    const { calls } = mockApi({ ...okRoutes(), "GET /api/v1/session": localAdminSession });
    mount();
    expect(await screen.findByText("82% · 2×")).toBeInTheDocument();
    expect(screen.queryByText("Tokens left")).not.toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Current holds" })).not.toBeInTheDocument();
    expect(calls.some((c) => c.url.startsWith("/api/v1/me"))).toBe(false);
  });

  it("an admin with a balance still sees holds", async () => {
    mockApi({ ...okRoutes(), "GET /api/v1/session": adminSession });
    mount();
    expect(await screen.findByRole("table", { name: "Current holds" })).toBeInTheDocument();
  });
});

// UI-8: the Ledger pages 20 rows at a time, by day or every poll, with Newer and Older.
const ledgerUrls = (calls: { url: string }[]) => calls.filter((c) => c.url.startsWith("/api/v1/me/ledger")).map((c) => c.url);
const lastLedgerUrl = (calls: { url: string }[]) => ledgerUrls(calls).at(-1);

const entry = (id: number, over: Partial<LedgerEntry> = {}): LedgerEntry => ({
  id,
  created_at: "2026-11-18T17:30:00Z",
  period: "2026-11",
  type: "debit",
  amount_tokens: -0.17,
  balance_after_tokens: 100,
  multiplier: 2,
  setup_name: "setup-04",
  hold_id: 7,
  actor_name: null,
  reason: null,
  entry_count: 1,
  ...over,
});
const rowsOf = (n: number, from: number): LedgerEntry[] => Array.from({ length: n }, (_, i) => entry(from - i));

/** Three pages by cursor: first (ids 60..41) -> "c2" -> (40..21) -> "c3" -> (20..11, last). */
const pages: Record<string, LedgerResponse> = {
  "": { ...ledger, entries: rowsOf(20, 60), next_cursor: "c2" },
  c2: { ...ledger, entries: rowsOf(20, 40), next_cursor: "c3" },
  c3: { ...ledger, entries: rowsOf(10, 20), next_cursor: null },
};
const pagedRoutes = () => ({
  ...okRoutes(),
  "GET /api/v1/me/ledger": ({ url }: { url: string }) => pages[new URL(url, "http://x").searchParams.get("cursor") ?? ""],
});
const ledgerTable = () => screen.getByRole("table", { name: "Ledger" });
const ledgerRows = () => within(ledgerTable()).getAllByRole("row").slice(1);

describe("UI-8 Ledger paging", () => {
  afterEach(() => vi.useRealTimers());

  it("UI-8 shows at most 20 rows even if the API returned more", async () => {
    mockApi({ ...okRoutes(), "GET /api/v1/me/ledger": { ...ledger, entries: rowsOf(25, 60), next_cursor: "c2" } });
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    expect(ledgerRows()).toHaveLength(20);
  });

  it("UI-8 By day is the default: rollup=day&limit=20; Every poll sends limit=20 with no rollup", async () => {
    const { calls } = mockApi(pagedRoutes());
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    expect(ledgerUrls(calls)).toEqual(["/api/v1/me/ledger?rollup=day&limit=20"]);
    expect(screen.getByRole("button", { name: "By day" })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: "Every poll" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?limit=20"));
    expect(screen.getByRole("button", { name: "Every poll" })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: "By day" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20"));
  });

  it("UI-8 Older follows next_cursor and Newer returns to the page before", async () => {
    const { calls } = mockApi(pagedRoutes());
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20&cursor=c2"));
    await waitFor(() => expect(ledgerRows()).toHaveLength(20));
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20&cursor=c3"));
    await waitFor(() => expect(ledgerRows()).toHaveLength(10));
    const before = ledgerUrls(calls).length;
    await userEvent.click(screen.getByRole("button", { name: "Newer" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20&cursor=c2"));
    await waitFor(() => expect(ledgerRows()).toHaveLength(20));
    await userEvent.click(screen.getByRole("button", { name: "Newer" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20"));
    expect(ledgerUrls(calls).length).toBe(before + 2);
  });

  it("UI-8 Newer is disabled on the first page and Older is disabled when next_cursor is null", async () => {
    mockApi(pagedRoutes());
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await waitFor(() => expect(screen.getByRole("button", { name: "Older" })).toBeEnabled());
    expect(screen.getByRole("button", { name: "Newer" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Newer" })).toBeEnabled());
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(ledgerRows()).toHaveLength(10));
    expect(screen.getByRole("button", { name: "Older" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Newer" })).toBeEnabled();
  });

  it("UI-8 a single page has both Newer and Older disabled", async () => {
    mockApi(okRoutes());
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    expect(screen.getByRole("button", { name: "Older" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Newer" })).toBeDisabled();
  });

  it("UI-8 changing the month returns to the first page and drops the visited cursors", async () => {
    const { calls } = mockApi(pagedRoutes());
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toContain("cursor=c2"));
    await userEvent.selectOptions(await screen.findByRole("combobox", { name: "Month" }), "2026-10");
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?period=2026-10&rollup=day&limit=20"));
    expect(screen.getByRole("button", { name: "Newer" })).toBeDisabled();
    // Older then Newer in the new month must not bring back a cursor of the old one.
    await waitFor(() => expect(screen.getByRole("button", { name: "Older" })).toBeEnabled());
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await userEvent.click(await screen.findByRole("button", { name: "Newer" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?period=2026-10&rollup=day&limit=20"));
  });

  it("UI-8 changing the view returns to the first page and drops the visited cursors", async () => {
    const { calls } = mockApi(pagedRoutes());
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toContain("cursor=c2"));
    await userEvent.click(screen.getByRole("button", { name: "Every poll" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?limit=20"));
    expect(ledgerUrls(calls).filter((u) => u.includes("rollup=") === false && u.includes("cursor="))).toEqual([]);
    expect(screen.getByRole("button", { name: "Newer" })).toBeDisabled();
  });

  it("UI-8 the 60-second poll refetches the page being shown, not the first", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi(pagedRoutes());
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toContain("cursor=c2"));
    const before = ledgerUrls(calls).length;
    await act(async () => void vi.advanceTimersByTime(60_000));
    await waitFor(() => expect(ledgerUrls(calls).length).toBeGreaterThan(before));
    expect(ledgerUrls(calls).slice(before).every((u) => u === "/api/v1/me/ledger?rollup=day&limit=20&cursor=c2")).toBe(true);
  });

  it("UI-8 the reload after a release refetches the page being shown", async () => {
    const { calls } = mockApi({ ...pagedRoutes(), "POST /api/v1/holds/7/release": { ...hold, active: false } });
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toContain("cursor=c2"));
    const before = ledgerUrls(calls).length;
    await userEvent.click(await screen.findByRole("button", { name: "Release" }));
    await waitFor(() => expect(ledgerUrls(calls).length).toBeGreaterThan(before));
    expect(ledgerUrls(calls).slice(before).every((u) => u.endsWith("&cursor=c2"))).toBe(true);
  });

  it("UI-8 a failed request for another page shows the error, not the previous page, and the controls still work", async () => {
    let failC2 = true;
    const { calls } = mockApi({
      ...pagedRoutes(),
      "GET /api/v1/me/ledger": ({ url }: { url: string }) => {
        const c = new URL(url, "http://x").searchParams.get("cursor") ?? "";
        return c === "c2" && failC2 ? errorResponse(500, "internal", "ledger is busy") : pages[c];
      },
    });
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    expect(await screen.findByText("ledger is busy")).toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Ledger" })).not.toBeInTheDocument();
    // Nothing is offered that would push the failed cursor again; Newer goes back to the page before.
    expect(screen.getByRole("button", { name: "Older" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Newer" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20"));
    await waitFor(() => expect(ledgerRows()).toHaveLength(20));
    expect(screen.queryByText("ledger is busy")).not.toBeInTheDocument();
    failC2 = false;
    await waitFor(() => expect(screen.getByRole("button", { name: "Older" })).toBeEnabled());
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toContain("cursor=c2"));
    await waitFor(() => expect(ledgerRows()).toHaveLength(20));
  });

  it("UI-8 a month rollover while paged on the default month drops the old cursors and shows the first page", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let nowMe: Me = me;
    const { calls } = mockApi({ ...pagedRoutes(), "GET /api/v1/me": () => nowMe });
    mount();
    await screen.findByRole("table", { name: "Ledger" });
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toContain("cursor=c2"));
    await waitFor(() => expect(screen.getByRole("button", { name: "Newer" })).toBeEnabled());
    nowMe = { ...me, period: "2026-12" };
    await act(async () => void vi.advanceTimersByTime(60_000));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20"));
    await waitFor(() => expect(screen.getByRole("button", { name: "Newer" })).toBeDisabled());
    // Older then Newer must not bring back a cursor of the old month.
    await waitFor(() => expect(screen.getByRole("button", { name: "Older" })).toBeEnabled());
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await userEvent.click(await screen.findByRole("button", { name: "Newer" }));
    await waitFor(() => expect(lastLedgerUrl(calls)).toBe("/api/v1/me/ledger?rollup=day&limit=20"));
  });

  it("UI-8 Detail shows the poll count only when entry_count is above 1; Type shows the multiplier only when not null", async () => {
    mockApi({
      ...okRoutes(),
      "GET /api/v1/me/ledger": {
        ...ledger,
        entries: [
          entry(9, { entry_count: 288, multiplier: null, amount_tokens: -48 }),
          entry(8, { entry_count: 1 }),
        ],
      },
    });
    mount();
    const table = await screen.findByRole("table", { name: "Ledger" });
    expect(within(table).getByText("setup-04 · 288 polls")).toBeInTheDocument();
    expect(within(table).getAllByText("setup-04")).toHaveLength(1);
    const rows = within(table).getAllByRole("row").slice(1);
    expect(within(rows[0]!).getByText("Debit")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("Debit · 2×")).toBeInTheDocument();
  });
});
