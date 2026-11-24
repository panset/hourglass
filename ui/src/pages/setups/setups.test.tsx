// SPDX-License-Identifier: MIT
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import type { Me, Pressure } from "../../api/types";
import { ToastProvider } from "../../components";
import { SessionProvider, useSessionState } from "../../shell/session";
import { adminSession, developerSession, errorResponse, localAdminSession, mockApi } from "../../test/mockApi";
import type { Setup, SetupList } from "../usage/holdApi";
import SetupsPage from "./index";

function Ready({ children }: { children: ReactNode }) {
  return useSessionState().state.status === "ready" ? <>{children}</> : null;
}
function mount() {
  return render(
    <SessionProvider>
      <ToastProvider>
        <MemoryRouter>
          <Ready>
            <SetupsPage />
          </Ready>
        </MemoryRouter>
      </ToastProvider>
    </SessionProvider>,
  );
}

const base = {
  weight_source: "default" as const,
  weight_attributes: {},
  metering: null,
  held_since: null,
  holder: null,
  deployed: false,
  details: { note: "opaque", nested: { a: 1 } },
  admin: null,
};

const setups: Setup[] = [
  {
    ...base, id: 1, name: "setup-01", labels: { size: "extralarge" }, weight: 4, state: "held", deployed: true,
    held_since: "2026-11-10T00:00:00Z",
    holder: { user_id: 9, hold_id: 91, name: "Sam Lee", balance_tokens: 61, overdrawn: false, is_you: false, release_requested_at: null },
    metering: { active: true, started_at: "2026-11-10T00:00:00Z", expected_start_at: null, reason: null },
    cost_per_hour_tokens: { now: 8, at_peak: 12 },
  },
  {
    ...base, id: 2, name: "setup-02", labels: { size: "large" }, weight: 2, state: "held", deployed: true,
    held_since: "2026-11-09T00:00:00Z",
    holder: { user_id: 10, hold_id: 92, name: "Priya N.", balance_tokens: -12, overdrawn: true, is_you: false, release_requested_at: null },
    metering: { active: true, started_at: "2026-11-09T00:00:00Z", expected_start_at: null, reason: null },
    cost_per_hour_tokens: { now: 4, at_peak: 6 },
  },
  {
    ...base, id: 4, name: "setup-04", labels: { size: "medium" }, weight: 1, state: "held", deployed: true,
    held_since: "2026-11-16T10:00:00Z",
    holder: { user_id: 2, hold_id: 94, name: "Dana Dev", balance_tokens: 142, overdrawn: false, is_you: true, release_requested_at: null },
    metering: { active: true, started_at: "2026-11-16T10:00:00Z", expected_start_at: null, reason: null },
    cost_per_hour_tokens: { now: 2, at_peak: 3 },
  },
  { ...base, id: 5, name: "setup-05", labels: { size: "medium" }, weight: 1, state: "free", cost_per_hour_tokens: { now: 2, at_peak: 3 } },
  {
    ...base, id: 6, name: "setup-06", labels: { size: "large" }, weight: 2, state: "disabled",
    cost_per_hour_tokens: { now: 4, at_peak: 6 },
    admin: { enabled: false, last_seen_at: null, matched_rule_selector: null, reported_weight: null, reported_weight_not_applied: null, weight_override: null },
  },
];

const list = (over: Partial<SetupList> = {}, items: Setup[] = setups): SetupList => ({
  setups: items,
  display_label_keys: ["size"],
  label_index: [
    { key: "size", values: [{ value: "extralarge", selector: "size=extralarge" }, { value: "large", selector: "size=large" }, { value: "medium", selector: "size=medium" }] },
  ],
  summary: { enabled: 11, deployed: 9, free: 2 },
  metering_description: "deployed or 2 h after acquire",
  reported_weights_enabled: false,
  ...over,
});

const me: Me = {
  period: "2026-11",
  period_ends_at: "2026-12-01T00:00:00Z",
  balance_tokens: 142,
  allowance_tokens: 300,
  granted_tokens: 0,
  carryover_tokens: 0,
  debited_tokens: 158,
  burn_per_hour_tokens: 2,
  burning: [],
  projection: "not_burning",
  projected_runout_at: null,
  hold_count: 1,
  hold_limit: 2,
  enforcement: true,
  overdrawn: false,
  can_acquire: true,
  cannot_acquire_reason: null,
};
const pressure = { raw: 0.8, smoothed: 0.82, multiplier: 2, bands: { base: 0, steps: [] }, source: "x", updated_at: null, setups_enabled: 11, setups_deployed: 9, history: [] } as Pressure;

const routes = (over: Record<string, unknown> = {}) => ({
  "GET /api/v1/session": developerSession,
  "GET /api/v1/setups": list(),
  "GET /api/v1/me": me,
  "GET /api/v1/pressure": pressure,
  ...over,
});

describe("Setups page", () => {
  it("loading state", async () => {
    mockApi(routes({ "GET /api/v1/setups": () => new Promise(() => {}) }));
    mount();
    expect(await screen.findByRole("status")).toHaveTextContent("Loading");
  });

  it("SET-6 populated: label column from display_label_keys, holder, overdrawn flag, costs", async () => {
    mockApi(routes());
    mount();
    const table = await screen.findByRole("table", { name: "All setups" });
    const headers = within(table).getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers.slice(0, 9)).toEqual(["Setup", "size", "Weight", "State", "Held by", "Holder's tokens", "Held for", "Cost", "Actions"]);
    expect(screen.getByText("9 of 11 setups deployed · 2 free")).toBeInTheDocument();
    expect(within(table).getByText("extralarge")).toBeInTheDocument();
    expect(within(table).getByText("Sam Lee")).toBeInTheDocument();
    expect(within(table).getByText("−12 overdrawn")).toBeInTheDocument();
    expect(within(table).getByText("You")).toBeInTheDocument();
    expect(within(table).getByText("2.0 / h if acquired")).toBeInTheDocument();
    expect(within(table).getByText("8.0 / h")).toBeInTheDocument();
  });

  it("UI-1 not-yet-charging setups show their cost per hour, with the API reason text as secondary text when there is one", async () => {
    const waiting = (id: number, reason: string | null): Setup => ({
      ...base, id, name: `wait-${id}`, labels: { size: "small" }, weight: 1, state: "held", deployed: false, held_since: "2026-11-16T10:00:00Z",
      holder: { user_id: 3, hold_id: 90 + id, name: "Sam Lee", balance_tokens: 5, overdrawn: false, is_you: false, release_requested_at: null },
      metering: { active: false, started_at: null, expected_start_at: "2099-01-01T00:00:00Z", reason },
      cost_per_hour_tokens: { now: 2, at_peak: 3 },
    });
    mockApi(routes({ "GET /api/v1/setups": list({}, [waiting(7, "Charging starts when deployed"), waiting(8, null)]) }));
    mount();
    const table = await screen.findByRole("table", { name: "All setups" });
    expect(within(table).getByText("Charging starts when deployed")).toBeInTheDocument();
    for (const name of ["wait-7", "wait-8"]) {
      const row = within(table).getByText(name).closest("tr")!;
      expect(within(row).getByText(/^2\.0 \/ h/)).toBeInTheDocument(); // the cost cell is never empty
    }
    expect(within(table).queryByText(/Free for/)).toBeNull();
    expect(within(table).queryByText("Not charging yet")).toBeNull();
  });

  it("empty state", async () => {
    mockApi(routes({ "GET /api/v1/setups": list({}, []) }));
    mount();
    expect(await screen.findByText("No setups to show.")).toBeInTheDocument();
  });

  it("error state shows the API message", async () => {
    mockApi(routes({ "GET /api/v1/setups": () => errorResponse(500, "internal", "database is busy") }));
    mount();
    expect(await screen.findByText("database is busy")).toBeInTheDocument();
  });

  it("filters by state and by label on the fetched list", async () => {
    mockApi(routes());
    mount();
    const table = await screen.findByRole("table", { name: "All setups" });
    await userEvent.selectOptions(screen.getByLabelText("State"), "free");
    expect(within(table).queryByText("setup-01")).not.toBeInTheDocument();
    expect(within(table).getByText("setup-05")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("State"), "");
    await userEvent.selectOptions(screen.getByLabelText("size"), "large");
    expect(within(table).getByText("setup-02")).toBeInTheDocument();
    expect(within(table).queryByText("setup-05")).not.toBeInTheDocument();
  });

  it("RULE-2 the label selector goes to the API as selector= and a parse error is shown", async () => {
    const { calls } = mockApi(
      routes({
        "GET /api/v1/setups": ({ url }: { url: string }) =>
          url.includes("selector=")
            ? errorResponse(400, "invalid_selector", "unexpected token at 3", "selector")
            : list(),
      }),
    );
    mount();
    await screen.findByRole("table", { name: "All setups" });
    await userEvent.type(screen.getByLabelText("Label selector"), "a=(");
    await userEvent.click(screen.getByRole("button", { name: "Filter" }));
    expect(await screen.findByText("unexpected token at 3")).toBeInTheDocument();
    expect(calls.some((c) => c.url === "/api/v1/setups?selector=a%3D(")).toBe(true);
  });

  it("Details opens the pretty-printed JSON, read only", async () => {
    mockApi(routes());
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Details for setup-01" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByLabelText("Setup details JSON").textContent).toBe(JSON.stringify(setups[0]!.details, null, 2));
    await userEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("HOLD-1 Acquire confirms with cost now and at the highest band, then posts with the CSRF header", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/setups/5/acquire": { id: 1 } }));
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Acquire setup-05" }));
    const dialog = await screen.findByRole("dialog", { name: "Acquire setup-05?" });
    expect(dialog).toHaveTextContent("Costs 2.0 / h now (2×), up to 3.0 / h at the highest band.");
    expect(dialog).toHaveTextContent("deployed or 2 h after acquire");
    expect(calls.some((c) => c.method === "POST")).toBe(false);
    await userEvent.click(within(dialog).getByRole("button", { name: "Acquire" }));
    await waitFor(() => expect(calls.some((c) => c.method === "POST")).toBe(true));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/setups/5/acquire");
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-dana");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(calls.filter((c) => c.url === "/api/v1/setups").length).toBe(2));
  });

  it("HOLD-1 an acquire refusal shows the API message and keeps the dialog open", async () => {
    mockApi(routes({ "POST /api/v1/setups/5/acquire": () => errorResponse(409, "setup_held", "Someone just took it.") }));
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Acquire setup-05" }));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Acquire" }));
    expect(await screen.findByText("Someone just took it.")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("HOLD-1 Acquire is disabled with the reason at zero balance with enforcement on", async () => {
    mockApi(routes({ "GET /api/v1/me": { ...me, balance_tokens: 0, overdrawn: true, can_acquire: false, cannot_acquire_reason: "insufficient_tokens" } }));
    mount();
    expect(await screen.findByRole("button", { name: "Acquire setup-05" })).toBeDisabled();
    expect(await screen.findByText("Your balance is zero or below, so you cannot acquire setups.")).toBeInTheDocument();
  });

  it("HOLD-1 Acquire stays enabled at zero balance when enforcement is off (shadow mode)", async () => {
    mockApi(routes({ "GET /api/v1/me": { ...me, balance_tokens: 0, overdrawn: true, enforcement: false } }));
    mount();
    expect(await screen.findByRole("button", { name: "Acquire setup-05" })).toBeEnabled();
  });

  it("HOLD-1 Acquire is disabled with the reason at the hold limit", async () => {
    mockApi(routes({ "GET /api/v1/me": { ...me, hold_count: 2, hold_limit: 2, can_acquire: false, cannot_acquire_reason: "hold_limit" } }));
    mount();
    expect(await screen.findByRole("button", { name: "Acquire setup-05" })).toBeDisabled();
    expect(await screen.findByText("You already hold 2 of 2 allowed setups.")).toBeInTheDocument();
  });

  it("HOLD-2 Release on own hold posts to its hold id and says Released on a 200", async () => {
    const { calls } = mockApi(
      routes({ "POST /api/v1/holds/94/release": { id: 94, active: false, release_requested_at: null } }),
    );
    mount();
    const row = (await screen.findByText("setup-04")).closest("tr")!;
    expect(within(screen.getByText("setup-01").closest("tr")!).queryByRole("button", { name: "Release" })).not.toBeInTheDocument();
    await userEvent.click(within(row).getByRole("button", { name: "Release" }));
    expect(await screen.findByText("Released setup-04.")).toBeInTheDocument();
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/holds/94/release");
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-dana");
  });

  it("HOLD-6 a 202 moves the row to Releasing without an error or a Released toast", async () => {
    let requested: string | null = null;
    const mine = (s: Setup): Setup =>
      s.id === 4 ? { ...s, holder: { ...s.holder!, release_requested_at: requested } } : s;
    mockApi(
      routes({
        "GET /api/v1/setups": () => list({}, setups.map(mine)),
        "POST /api/v1/holds/94/release": () => {
          requested = "2026-11-18T12:00:00Z";
          return new Response(JSON.stringify({ id: 94, active: true, release_requested_at: requested }), {
            status: 202,
            headers: { "Content-Type": "application/json" },
          });
        },
      }),
    );
    mount();
    const row = (await screen.findByText("setup-04")).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", { name: "Release" }));
    const after = await waitFor(() => {
      const r = screen.getByText("setup-04").closest("tr")!;
      expect(within(r).getByText("Releasing")).toBeInTheDocument();
      return r;
    });
    expect(within(after).queryByRole("button", { name: "Release" })).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByText("Released setup-04.")).not.toBeInTheDocument();
    expect(screen.getByText(/Release requested for setup-04/)).toBeInTheDocument();
  });

  it("HOLD-6 a hold with release_requested_at shows Releasing with the request time and no Release button", async () => {
    const releasing = setups.map((s) =>
      s.id === 4 ? { ...s, holder: { ...s.holder!, release_requested_at: "2026-11-18T12:00:00Z" } } : s,
    );
    mockApi(routes({ "GET /api/v1/setups": list({}, releasing) }));
    mount();
    const row = (await screen.findByText("setup-04")).closest("tr")!;
    expect(within(row).getByText("Releasing")).toBeInTheDocument();
    expect(within(row).getByText(/requested/)).toBeInTheDocument();
    expect(row.querySelector("time")).toHaveAttribute("datetime", "2026-11-18T12:00:00Z");
    expect(within(row).queryByRole("button", { name: "Release" })).not.toBeInTheDocument();
  });

  it("the local admin sees no Acquire button and /me is not called", async () => {
    const { calls } = mockApi(routes({ "GET /api/v1/session": localAdminSession }));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(screen.queryByRole("button", { name: /^Acquire/ })).not.toBeInTheDocument();
    expect(calls.some((c) => c.url === "/api/v1/me")).toBe(false);
  });

  it("admins see disabled setups marked and can filter on Disabled; non-admins have no such filter option", async () => {
    mockApi(routes({ "GET /api/v1/session": adminSession }));
    const { unmount } = mount();
    const table = await screen.findByRole("table", { name: "All setups" });
    expect(within(table).getAllByText("Disabled").length).toBeGreaterThan(0);
    expect(within(screen.getByLabelText("State")).getByRole("option", { name: "Disabled" })).toBeInTheDocument();
    unmount();
    mockApi(routes());
    mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(within(screen.getByLabelText("State")).queryByRole("option", { name: "Disabled" })).not.toBeInTheDocument();
  });
});

type IndexKey = SetupList["label_index"][number];
const term = (key: string, value: string) => ({ value, selector: `${key}=${value}` });
const labelIndex: IndexKey[] = [
  { key: "site", values: [term("site", "ams"), term("site", "sjc")] },
  { key: "tier", values: [term("tier", "a"), term("tier", "b"), { value: "has space", selector: null }, { value: "", selector: "tier=" }] },
];
const withIndex = (label_index: IndexKey[]) => routes({ "GET /api/v1/setups": list({ label_index }) });
const selectorBox = () => screen.getByLabelText("Label selector") as HTMLInputElement;
const chip = (term: string) => screen.getByRole("button", { name: `Filter by ${term}` });
const lastUrl = (calls: { url: string }[]) => calls.filter((c) => c.url.startsWith("/api/v1/setups")).at(-1)?.url;

describe("UI-9 label chips on Setups", () => {
  it("UI-9 lists every key and every value of label_index as chips named 'Filter by key=value'", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(screen.getByRole("heading", { name: "Available labels" })).toBeInTheDocument();
    const site = screen.getByRole("group", { name: "Label site" });
    expect(within(site).getAllByRole("button").map((b) => b.getAttribute("aria-label"))).toEqual(["Filter by site=ams", "Filter by site=sjc"]);
    const tier = screen.getByRole("group", { name: "Label tier" });
    expect(within(tier).getAllByRole("button").map((b) => b.getAttribute("aria-label"))).toEqual([
      "Filter by tier=a", "Filter by tier=b", "tier=has space cannot be used in a selector", "Filter by tier=",
    ]);
  });

  it("UI-9 activating a chip appends its term and filters through the API", async () => {
    const { calls } = mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    await userEvent.click(chip("site=sjc"));
    expect(selectorBox().value).toBe("site=sjc");
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=site%3Dsjc"));
    await userEvent.click(chip("tier=a"));
    expect(selectorBox().value).toBe("site=sjc,tier=a");
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=site%3Dsjc%2Ctier%3Da"));
  });

  it("UI-9 a chip replaces another key=value term of the same key and keeps the other terms", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    await userEvent.type(selectorBox(), "site=ams, tier=a");
    await userEvent.click(chip("site=sjc"));
    expect(selectorBox().value).toBe("site=sjc,tier=a");
  });

  it("UI-9 a chip whose exact term is in the box removes it (toggle) and filters again", async () => {
    const { calls } = mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    await userEvent.click(chip("site=sjc"));
    await userEvent.click(chip("tier=a"));
    await userEvent.click(chip("site=sjc"));
    expect(selectorBox().value).toBe("tier=a");
    await userEvent.click(chip("tier=a"));
    expect(selectorBox().value).toBe("");
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups"));
  });

  it("UI-9 terms are compared whole: a comma inside parentheses does not split a term and an 'in' term is not replaced", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    await userEvent.type(selectorBox(), "site in (sjc,ams)");
    await userEvent.click(chip("site=sjc"));
    expect(selectorBox().value).toBe("site in (sjc,ams),site=sjc");
    // The other whole term can be toggled off without touching the 'in' term.
    await userEvent.click(chip("site=sjc"));
    expect(selectorBox().value).toBe("site in (sjc,ams)");
    // A longer value that merely starts with the chip's term is a different term.
    await userEvent.clear(selectorBox());
    await userEvent.type(selectorBox(), "tier=ab");
    await userEvent.click(chip("tier=a"));
    expect(selectorBox().value).toBe("tier=a");
  });

  it("UI-9 a chip whose term is in the applied selector is selected; typing without applying selects nothing", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    await userEvent.type(selectorBox(), "site=ams");
    expect(chip("site=ams")).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(screen.getByRole("button", { name: "Filter" }));
    await waitFor(() => expect(chip("site=ams")).toHaveAttribute("aria-pressed", "true"));
    expect(chip("site=sjc")).toHaveAttribute("aria-pressed", "false");
  });

  it("UI-9 a value with no term cannot be activated and says why", async () => {
    const { calls } = mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    const before = calls.length;
    const bad = screen.getByRole("button", { name: "tier=has space cannot be used in a selector" });
    expect(bad).toHaveAttribute("aria-disabled", "true");
    expect(bad).toHaveAttribute("title", expect.stringContaining("cannot be used in a selector"));
    await userEvent.click(bad);
    bad.focus();
    await userEvent.keyboard("{Enter}");
    expect(selectorBox().value).toBe("");
    expect(calls.length).toBe(before);
    expect(screen.getByText(/cannot be used in a selector\./)).toBeInTheDocument();
  });

  it("UI-9 the placeholder is the first non-null term of the index", async () => {
    mockApi(withIndex([{ key: "a", values: [{ value: "x y", selector: null }, term("a", "z")] }, { key: "b", values: [term("b", "1")] }]));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(selectorBox()).toHaveAttribute("placeholder", "e.g. a=z");
  });

  it("UI-9 with an empty index there is no placeholder and the block says no setup has labels", async () => {
    mockApi(withIndex([]));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(selectorBox()).not.toHaveAttribute("placeholder");
    expect(screen.getByText("No setup has labels.")).toBeInTheDocument();
  });

  it("UI-9 a key shows its first 12 values and '+N more' reveals the rest", async () => {
    const values = Array.from({ length: 15 }, (_, i) => term("rack", `r${String(i).padStart(2, "0")}`));
    mockApi(withIndex([{ key: "rack", values }]));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    const group = screen.getByRole("group", { name: "Label rack" });
    expect(within(group).getAllByRole("button", { name: /^Filter by/ })).toHaveLength(12);
    expect(within(group).queryByRole("button", { name: "Filter by rack=r12" })).not.toBeInTheDocument();
    await userEvent.click(within(group).getByRole("button", { name: "+3 more" }));
    expect(within(group).getAllByRole("button", { name: /^Filter by/ })).toHaveLength(15);
    expect(within(group).queryByRole("button", { name: /more/ })).not.toBeInTheDocument();
  });

  it("UI-9 exactly 12 values show no '+N more'", async () => {
    mockApi(withIndex([{ key: "rack", values: Array.from({ length: 12 }, (_, i) => term("rack", `r${i}`)) }]));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(screen.queryByRole("button", { name: /more/ })).not.toBeInTheDocument();
  });

  it("UI-9 with more than 8 keys the block starts collapsed behind Show labels; with 8 it is open", async () => {
    const keys = (n: number): IndexKey[] => Array.from({ length: n }, (_, i) => ({ key: `k${i}`, values: [term(`k${i}`, "v")] }));
    mockApi(withIndex(keys(9)));
    const first = mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(screen.queryByRole("button", { name: "Filter by k0=v" })).not.toBeInTheDocument();
    const toggle = screen.getByRole("button", { name: "Show labels" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(toggle);
    expect(screen.getByRole("button", { name: "Filter by k0=v" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hide labels" })).toHaveAttribute("aria-expanded", "true");
    first.unmount();

    mockApi(withIndex(keys(8)));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    expect(screen.queryByRole("button", { name: "Show labels" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Filter by k0=v" })).toBeInTheDocument();
  });

  it("UI-9 a chip is a button reachable with Tab and activated with Enter and Space", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    const c = chip("site=ams");
    expect(c.tagName).toBe("BUTTON");
    c.focus();
    await userEvent.keyboard("{Enter}");
    expect(selectorBox().value).toBe("site=ams");
    await userEvent.keyboard(" ");
    expect(selectorBox().value).toBe("");
  });

  it("UI-9 a chip for an empty value uses the term from the API (key=)", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await screen.findByRole("table", { name: "All setups" });
    await userEvent.click(chip("tier="));
    expect(selectorBox().value).toBe("tier=");
  });
});

describe("UI-12 weight source, attributes and the acquire warning", () => {
  const priced = (id: number, source: Setup["weight_source"], attrs: Record<string, string> = {}): Setup => ({
    ...base, id, name: `src-${id}`, labels: { size: "medium" }, weight: 1.15, weight_source: source, weight_attributes: attrs,
    state: "free", cost_per_hour_tokens: { now: 2.3, at_peak: 3.45 },
  });
  const mountWith = (items: Setup[], over: Partial<SetupList> = {}) => {
    mockApi(routes({ "GET /api/v1/setups": list(over, items) }));
    mount();
  };
  const rowOf = async (name: string) => within(await screen.findByRole("table", { name: "All setups" })).getByText(name).closest("tr")!;

  it("UI-12 the weight cell carries a tag for reported and override, and none for rule or default", async () => {
    mountWith([priced(1, "reported"), priced(2, "override"), priced(3, "rule"), priced(4, "default")]);
    expect(within(await rowOf("src-1")).getByText("reported")).toBeInTheDocument();
    expect(within(await rowOf("src-1")).getByText("1.15")).toBeInTheDocument();
    expect(within(await rowOf("src-2")).getByText("override")).toBeInTheDocument();
    for (const n of ["src-3", "src-4"]) {
      const row = await rowOf(n);
      expect(within(row).queryByText("reported")).toBeNull();
      expect(within(row).queryByText("override")).toBeNull();
      expect(within(row).queryByText("rule")).toBeNull();
      expect(within(row).queryByText("default")).toBeNull();
    }
  });

  it("UI-12 attributes are one secondary line, not columns, and absent when empty", async () => {
    mountWith([priced(1, "reported", { "memory-gb": "512", "storage-gb": "4000", tier: "gold" }), priced(2, "reported")]);
    const row = await rowOf("src-1");
    expect(within(row).getByText("memory-gb 512 · storage-gb 4000 · tier gold")).toBeInTheDocument();
    const table = screen.getByRole("table", { name: "All setups" });
    const headers = within(table).getAllByRole("columnheader").map((h) => h.textContent);
    expect(headers.some((h) => /memory|storage|tier/.test(h ?? ""))).toBe(false);
    expect(within(await rowOf("src-2")).queryByText(/memory-gb/)).toBeNull();
    expect(screen.queryByLabelText("memory-gb")).toBeNull(); // no filter either
  });

  const SENTENCE = "Weights are reported by automation, so the price can change while the setup is held.";
  it.each(["reported", "rule", "default", "override"] as const)(
    "UI-12 the acquire dialog shows the change warning when reported_weights_enabled is true, whatever the source (%s)",
    async (source) => {
      mountWith([priced(5, source, { tier: "gold" })], { reported_weights_enabled: true });
      await userEvent.click(await screen.findByRole("button", { name: "Acquire src-5" }));
      const dialog = await screen.findByRole("dialog", { name: "Acquire src-5?" });
      expect(dialog).toHaveTextContent("Weight 1.15");
      expect(dialog).toHaveTextContent("tier gold");
      expect(dialog).toHaveTextContent("Costs 2.3 / h now (2×), up to 3.45 / h at the highest band.");
      expect(within(dialog).getByText(SENTENCE)).toBeInTheDocument();
    },
  );

  it("UI-12 the acquire dialog has no change warning when reported_weights_enabled is false", async () => {
    mountWith([priced(5, "reported")], { reported_weights_enabled: false });
    await userEvent.click(await screen.findByRole("button", { name: "Acquire src-5" }));
    const dialog = await screen.findByRole("dialog", { name: "Acquire src-5?" });
    expect(dialog).toHaveTextContent("Weight 1.15");
    expect(within(dialog).queryByText(SENTENCE)).toBeNull();
  });
});
