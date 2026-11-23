// SPDX-License-Identifier: MIT
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import type { Me, Pressure } from "../../api/types";
import { ToastProvider } from "../../components";
import { SessionProvider, useSessionState } from "../../shell/session";
import { adminSession, developerSession, errorResponse, localAdminSession, mockApi } from "../../test/mockApi";
import type { Hold, LedgerResponse, UsageResponse } from "./holdApi";
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
  burning: [{ hold_id: 7, setup_id: 4, setup_name: "setup-04", weight: 1, multiplier: 2, tokens_per_hour: 2 }],
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
      expect(calls.some((c) => c.url === "/api/v1/me/ledger?period=2026-10")).toBe(true);
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
