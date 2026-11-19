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
    holder: { user_id: 9, hold_id: 91, name: "Sam Lee", balance_tokens: 61, overdrawn: false, is_you: false },
    metering: { active: true, started_at: "2026-11-10T00:00:00Z", expected_start_at: null, reason: null },
    cost_per_hour_tokens: { now: 8, at_peak: 12 },
  },
  {
    ...base, id: 2, name: "setup-02", labels: { size: "large" }, weight: 2, state: "held", deployed: true,
    held_since: "2026-11-09T00:00:00Z",
    holder: { user_id: 10, hold_id: 92, name: "Priya N.", balance_tokens: -12, overdrawn: true, is_you: false },
    metering: { active: true, started_at: "2026-11-09T00:00:00Z", expected_start_at: null, reason: null },
    cost_per_hour_tokens: { now: 4, at_peak: 6 },
  },
  {
    ...base, id: 4, name: "setup-04", labels: { size: "medium" }, weight: 1, state: "held", deployed: true,
    held_since: "2026-11-16T10:00:00Z",
    holder: { user_id: 2, hold_id: 94, name: "Dana Dev", balance_tokens: 142, overdrawn: false, is_you: true },
    metering: { active: true, started_at: "2026-11-16T10:00:00Z", expected_start_at: null, reason: null },
    cost_per_hour_tokens: { now: 2, at_peak: 3 },
  },
  { ...base, id: 5, name: "setup-05", labels: { size: "medium" }, weight: 1, state: "free", cost_per_hour_tokens: { now: 2, at_peak: 3 } },
  {
    ...base, id: 6, name: "setup-06", labels: { size: "large" }, weight: 2, state: "disabled",
    cost_per_hour_tokens: { now: 4, at_peak: 6 },
    admin: { enabled: false, resource_name: "setup-06_RP", resource_name_override: null, last_seen_at: null, matched_rule_selector: null },
  },
];

const list = (over: Partial<SetupList> = {}, items: Setup[] = setups): SetupList => ({
  setups: items,
  display_label_keys: ["size"],
  summary: { enabled: 11, deployed: 9, free: 2 },
  metering_description: "deployed or 2 h after acquire",
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

  it("UI-1 not-yet-charging cost shows the API reason text, and nothing when it is null", async () => {
    const waiting = (id: number, reason: string | null): Setup => ({
      ...base, id, name: `wait-${id}`, labels: { size: "small" }, weight: 1, state: "held", deployed: false, held_since: "2026-11-16T10:00:00Z",
      holder: { user_id: 3, hold_id: 90 + id, name: "Sam Lee", balance_tokens: 5, overdrawn: false, is_you: false },
      metering: { active: false, started_at: null, expected_start_at: "2099-01-01T00:00:00Z", reason },
      cost_per_hour_tokens: { now: 2, at_peak: 3 },
    });
    mockApi(routes({ "GET /api/v1/setups": list({}, [waiting(7, "Charging starts when deployed"), waiting(8, null)]) }));
    mount();
    const table = await screen.findByRole("table", { name: "All setups" });
    expect(within(table).getByText("Charging starts when deployed")).toBeInTheDocument();
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

  it("HOLD-2 Release on own hold posts to its hold id; a refusal shows the API message", async () => {
    const { calls } = mockApi(
      routes({
        "POST /api/v1/holds/94/release": () =>
          errorResponse(409, "still_deployed", "This setup is still deployed. Tear it down, then release."),
      }),
    );
    mount();
    const row = (await screen.findByText("setup-04")).closest("tr")!;
    expect(within(screen.getByText("setup-01").closest("tr")!).queryByRole("button", { name: "Release" })).not.toBeInTheDocument();
    await userEvent.click(within(row).getByRole("button", { name: "Release" }));
    expect(await screen.findByText("This setup is still deployed. Tear it down, then release.")).toBeInTheDocument();
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/holds/94/release");
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-dana");
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
