// SPDX-License-Identifier: MIT
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter } from "react-router-dom";
import type { Me, Pressure } from "../../api/types";
import { ToastProvider } from "../../components";
import buttonStyles from "../../components/Button.module.css";
import tableStyles from "../../components/Table.module.css";
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
  pricing: { multiplier: 2, peak_multiplier: 3, pressure: 0.8 },
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

const tableEl = () => screen.findByRole("table", { name: "All setups" });
const rowOf = async (name: string) => within(await tableEl()).getByText(name).closest("tr")!;
const headersOf = (table: HTMLElement) => within(table).getAllByRole("columnheader").map((h) => h.textContent);
const costButton = (name: string) => screen.getByRole("button", { name: new RegExp(`Price details for ${name}$`) });
/** The cost as a sighted reader sees it. */
const costText = (name: string) => costButton(name).querySelector("[aria-hidden]")!.textContent;
const detailsLink = (name: string) => screen.getByRole("button", { name: `Details for ${name}` });
/** Opens the panel of a setup from its Details link and returns the dialog. */
const openPanel = async (name: string) => {
  await userEvent.click(detailsLink(name));
  return screen.findByRole("dialog", { name: `Details: ${name}` });
};
const closePanel = async () => {
  await userEvent.click(within(screen.getByRole("dialog")).getAllByRole("button", { name: "Close" }).at(-1)!);
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
};
/** The value of a named line of the panel: the `dd` after the `dt` with that text, or null when the line is absent. */
const line = (dialog: HTMLElement, name: string) =>
  (within(dialog).queryByText(name, { selector: "dt" })?.nextElementSibling as HTMLElement | null | undefined) ?? null;
/** A window `focus` event makes the page refetch at once (usePolling). */
const refresh = () => act(() => void window.dispatchEvent(new Event("focus")));

describe("Setups page", () => {
  it("loading state", async () => {
    mockApi(routes({ "GET /api/v1/setups": () => new Promise(() => {}) }));
    mount();
    expect(await screen.findByRole("status")).toHaveTextContent("Loading");
  });

  it("the populated table shows holder, overdrawn word and costs, and no label column", async () => {
    mockApi(routes());
    mount();
    const table = await tableEl();
    expect(headersOf(table)).toEqual(["Setup", "State", "Held by", "Cost (tokens / h)", "Actions"]);
    expect(screen.getByText("9 of 11 setups deployed · 2 free")).toBeInTheDocument();
    expect(within(table).queryByText("extralarge")).toBeNull(); // labels are in the panel only
    expect(within(table).getByText("Sam Lee")).toBeInTheDocument();
    expect(within(await rowOf("setup-02")).getByText("overdrawn")).toBeInTheDocument();
    expect(within(table).queryByText(/−12/)).toBeNull(); // the balance is in the panel
    expect(within(await rowOf("setup-04")).getByText("You")).toBeInTheDocument();
    expect(costText("setup-05")).toBe("2.0 now · up to 3.0");
    expect(costText("setup-01")).toBe("8.0 now · up to 12.0");
    expect(within(table).queryByText(/if acquired/)).toBeNull();
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
    const table = await tableEl();
    expect(within(table).getByText("Charging starts when deployed")).toBeInTheDocument();
    for (const name of ["wait-7", "wait-8"]) {
      expect(costText(name)).toBe("2.0 now · up to 3.0"); // the cost cell is never empty
    }
    // The reason is the cost cell's second line; a hold without one has no second line.
    expect(costButton("wait-7").closest("td")!.textContent).toContain("Charging starts when deployed");
    expect(costButton("wait-8").closest("td")!.querySelectorAll(":scope > span")).toHaveLength(0);
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

  it("UI-19 the page has no per-label dropdown and no label chips; the State filter and the selector box work", async () => {
    const { calls } = mockApi(routes());
    mount();
    const table = await tableEl();
    expect(screen.queryByRole("heading", { name: "Available labels" })).toBeNull();
    expect(screen.queryByRole("button", { name: /^Filter by / })).toBeNull();
    // One dropdown, State. The selector box is a text input with the role combobox.
    expect(document.querySelectorAll("select")).toHaveLength(1);
    expect(document.querySelector("select")).toBe(screen.getByLabelText("State"));
    expect(screen.queryByLabelText("size")).toBeNull();
    expect(screen.getByLabelText("Label selector").tagName).toBe("INPUT");
    await userEvent.selectOptions(screen.getByLabelText("State"), "free");
    expect(within(table).queryByText("setup-01")).not.toBeInTheDocument();
    expect(within(table).getByText("setup-05")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("State"), "");
    expect(within(table).getByText("setup-01")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Label selector"), "size=medium");
    await userEvent.click(screen.getByRole("button", { name: "Filter" }));
    await waitFor(() => expect(calls.at(-1)?.url).toBe("/api/v1/setups?selector=size%3Dmedium"));
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
    expect(within(dialog).queryByRole("textbox")).toBeNull();
    await userEvent.click(within(dialog).getAllByRole("button", { name: "Close" }).at(-1)!);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("A2.7-3 A2.7-5 Details closes on a backdrop click and on the x, and focus returns to Details", async () => {
    mockApi(routes());
    mount();
    const opener = await screen.findByRole("button", { name: "Details for setup-01" });
    await userEvent.click(opener);
    await userEvent.click((await screen.findByRole("dialog")).parentElement!);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
    await userEvent.click(opener);
    await userEvent.click(within(await screen.findByRole("dialog")).getAllByRole("button", { name: "Close" })[0]!);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });

  it("A2.7-11 Acquire closes on a backdrop click like Cancel and focus returns to the Acquire button", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/setups/5/acquire": { id: 1 } }));
    mount();
    const opener = await screen.findByRole("button", { name: "Acquire setup-05" });
    await userEvent.click(opener);
    await userEvent.click((await screen.findByRole("dialog", { name: "Acquire setup-05?" })).parentElement!);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
    expect(calls.some((c) => c.method === "POST")).toBe(false);
  });

  it("HOLD-1 Acquire confirms with cost now and at the highest multiplier, then posts with the CSRF header", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/setups/5/acquire": { id: 1 } }));
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Acquire setup-05" }));
    const dialog = await screen.findByRole("dialog", { name: "Acquire setup-05?" });
    expect(dialog).toHaveTextContent("Cost now 2.0 tokens / h, up to 3.0 tokens / h at the highest multiplier (3×).");
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

  it("HOLD-6 a hold with release_requested_at shows Releasing and no Release button; the request time is in the panel", async () => {
    const releasing = setups.map((s) =>
      s.id === 4 ? { ...s, holder: { ...s.holder!, release_requested_at: "2026-11-18T12:00:00Z" } } : s,
    );
    mockApi(routes({ "GET /api/v1/setups": list({}, releasing) }));
    mount();
    const row = (await screen.findByText("setup-04")).closest("tr")!;
    expect(within(row).getByText("Releasing")).toBeInTheDocument();
    expect(within(row).queryByText(/requested/)).toBeNull();
    expect(row.querySelector("time")).toBeNull();
    expect(within(row).queryByRole("button", { name: "Release" })).not.toBeInTheDocument();
    const release = line(await openPanel("setup-04"), "Release")!;
    expect(release).toHaveTextContent(/^Release requested .+\. It completes when the setup is torn down\.$/);
    expect(release.querySelector("time")).toHaveAttribute("datetime", "2026-11-18T12:00:00Z");
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
    expect(within(table).getAllByText("Disabled")).toHaveLength(1); // the State badge only
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
const AWKWARD = "Team A (infra)";
const labelIndex: IndexKey[] = [
  { key: "owner", values: [{ value: AWKWARD, selector: null }, term("owner", "plain")] },
  { key: "site", values: [term("site", "ams"), term("site", "sjc")] },
  { key: "size", values: [term("size", "large"), term("size", "medium")] },
  { key: "tier", values: [{ value: "", selector: "tier=" }, term("tier", "a"), term("tier", "b"), { value: "has space", selector: null }] },
];
const withIndex = (label_index: IndexKey[]) => routes({ "GET /api/v1/setups": list({ label_index }) });
const selectorBox = () => screen.getByLabelText("Label selector") as HTMLInputElement;
const lastUrl = (calls: { url: string }[]) => calls.filter((c) => c.url.startsWith("/api/v1/setups")).at(-1)?.url;
const setupsCalls = (calls: { url: string }[]) => calls.filter((c) => c.url.startsWith("/api/v1/setups")).length;
const listbox = () => screen.getByRole("listbox", { name: "Label suggestions" });
/** The left text of every option, in order. */
const optionTexts = () => within(listbox()).getAllByRole("option").map((o) => o.firstElementChild!.textContent);
const option = (name: string | RegExp) => within(listbox()).getByRole("option", { name });
const statusLine = () => screen.getByRole("status");
const expanded = () => selectorBox().getAttribute("aria-expanded");

describe("UI-21 the Label selector box suggests keys, operators and values", () => {
  it("UI-21 the selector box is a combobox: typing opens the list, Arrow Down and Enter pick, and a picked value filters through the API", async () => {
    const { calls } = mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    const box = selectorBox();
    expect(box).toHaveAttribute("role", "combobox");
    expect(box).toHaveAttribute("aria-autocomplete", "list");
    expect(box).toHaveAttribute("autocomplete", "off");
    expect(box).toHaveAccessibleName("Label selector");
    expect(expanded()).toBe("false");
    expect(screen.queryByRole("listbox")).toBeNull();

    await userEvent.type(box, "si");
    expect(expanded()).toBe("true");
    expect(box).toHaveAttribute("aria-controls", listbox().id);
    expect(within(listbox()).getByRole("group", { name: "Label keys" })).toBeInTheDocument();
    expect(optionTexts()).toEqual(["site", "size"]);
    await userEvent.keyboard("{ArrowDown}{ArrowDown}{Enter}");
    // A key: `=` is added, nothing is applied, and the values of the key are listed at once.
    expect(box.value).toBe("size=");
    expect(lastUrl(calls)).toBe("/api/v1/setups");
    expect(within(listbox()).getByRole("group", { name: "Values of size" })).toBeInTheDocument();
    expect(optionTexts()).toEqual(["large", "medium"]);
    await userEvent.keyboard("{ArrowDown}{Enter}");
    // A value: applied at once, the list closes, focus and caret stay in the box.
    expect(box.value).toBe("size=large");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(box).toHaveFocus();
    expect(box.selectionStart).toBe("size=large".length);
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=size%3Dlarge"));
    await waitFor(() => expect(statusLine()).toHaveTextContent("5 setups shown."));

    // The caret back in the value: another pick replaces that token only and applies again.
    await userEvent.type(box, ",tier=a");
    box.setSelectionRange(5, 5);
    fireEvent.keyUp(box, { key: "ArrowLeft" }); // the list follows the caret
    expect(optionTexts()).toEqual(["large", "medium"]);
    await userEvent.click(option("medium"));
    expect(box.value).toBe("size=medium,tier=a");
    expect(box.selectionStart).toBe("size=medium".length);
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=size%3Dmedium%2Ctier%3Da"));
  });

  it("UI-21 a typed value is listed under Values as its whole term, and picking it filters in one action", async () => {
    const { calls } = mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    await userEvent.type(selectorBox(), "ar");
    expect(within(listbox()).queryByRole("group", { name: "Label keys" })).toBeNull();
    expect(within(within(listbox()).getByRole("group", { name: "Values" })).getAllByRole("option").map((o) => o.textContent)).toEqual(["size=large"]);
    expect(option("size=large")).not.toHaveAttribute("aria-label"); // no note: the term is not said twice
    await userEvent.click(option("size=large"));
    expect(selectorBox().value).toBe("size=large");
    expect(selectorBox()).toHaveFocus();
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=size%3Dlarge"));
    // After a comma the same holds, and the first requirement is untouched.
    await userEvent.type(selectorBox(), ",sj");
    await userEvent.click(option("site=sjc"));
    expect(selectorBox().value).toBe("size=large,site=sjc");
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=size%3Dlarge%2Csite%3Dsjc"));
  });

  it("UI-21 Enter with no active option submits the typed text; Escape closes the list and keeps the text; Tab picks nothing", async () => {
    const { calls } = mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    const box = selectorBox();
    await userEvent.type(box, "site=s");
    expect(optionTexts()).toEqual(["sjc", "ams"]); // starts with, then contains
    expect(box).not.toHaveAttribute("aria-activedescendant");
    await userEvent.keyboard("{Enter}");
    expect(box.value).toBe("site=s"); // as typed: nothing was picked
    expect(screen.queryByRole("listbox")).toBeNull();
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=site%3Ds"));

    await userEvent.keyboard("j");
    expect(optionTexts()).toEqual(["sjc"]);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(box.value).toBe("site=sj");
    await userEvent.keyboard("{Escape}"); // closed: nothing happens, the text is never cleared
    expect(box.value).toBe("site=sj");
    expect(box).toHaveFocus();
    expect(lastUrl(calls)).toBe("/api/v1/setups?selector=site%3Ds");

    await userEvent.keyboard("{ArrowDown}");
    expect(box).toHaveAttribute("aria-activedescendant", option("sjc").id);
    await userEvent.tab();
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(box.value).toBe("site=sj");
    expect(screen.getByRole("button", { name: "Filter" })).toHaveFocus();
    expect(lastUrl(calls)).toBe("/api/v1/setups?selector=site%3Ds");
  });

  it("UI-21 the list opens on click and on Arrow Down in an empty box, and not on focus alone", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    const box = selectorBox();
    act(() => box.focus());
    expect(expanded()).toBe("false");
    await userEvent.tab({ shift: true });
    await userEvent.tab();
    expect(box).toHaveFocus();
    expect(expanded()).toBe("false");

    await userEvent.keyboard("{ArrowDown}");
    expect(optionTexts()).toEqual(["owner", "site", "size", "tier"]);
    expect(box).toHaveAttribute("aria-activedescendant", option(/^owner/).id);
    // Each key carries a sample of its values.
    expect(option(/^owner/)).toHaveAccessibleName("owner, values: Team A (infra), plain");
    expect(option(/^tier/)).toHaveAccessibleName("tier, values: (empty), a, b, …");
    await userEvent.keyboard("{Escape}");
    expect(expanded()).toBe("false");

    await userEvent.click(box);
    expect(optionTexts()).toEqual(["owner", "site", "size", "tier"]);
    expect(box).not.toHaveAttribute("aria-activedescendant");
    await userEvent.keyboard("{Escape}");

    // The arrow button opens and closes it too; it is not a Tab stop and focus stays in the box.
    const arrow = screen.getByRole("button", { name: "Show label suggestions" });
    expect(arrow).toHaveAttribute("tabindex", "-1");
    await userEvent.click(arrow);
    expect(arrow).toHaveAttribute("aria-expanded", "true");
    expect(optionTexts()).toHaveLength(4);
    expect(box).toHaveFocus();
    await userEvent.click(arrow);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(box).toHaveFocus();
  });

  it("UI-21 aria-activedescendant follows the active option and the status line announces the count", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    const box = selectorBox();
    await userEvent.click(box);
    expect(statusLine()).toHaveTextContent("4 suggestions.");
    expect(statusLine()).toHaveClass("sr-only");
    expect(within(listbox()).queryByRole("option", { selected: true })).toBeNull();
    await userEvent.keyboard("{ArrowDown}");
    expect(box).toHaveAttribute("aria-activedescendant", option(/^owner/).id);
    expect(option(/^owner/)).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{ArrowUp}"); // wraps
    expect(box).toHaveAttribute("aria-activedescendant", option(/^tier/).id);
    expect(option(/^owner/)).toHaveAttribute("aria-selected", "false");
    await userEvent.keyboard("{Home}"); // a caret key: it does not move to the first option
    expect(box).not.toHaveAttribute("aria-activedescendant", option(/^owner/).id);

    // An edit: no option is active, the count follows.
    await userEvent.keyboard("ow");
    expect(box).not.toHaveAttribute("aria-activedescendant");
    expect(statusLine()).toHaveTextContent("1 suggestion.");
    await userEvent.keyboard("ner ");
    expect(within(listbox()).getByRole("group", { name: "Match owner" })).toBeInTheDocument();
    expect(within(listbox()).getAllByRole("option").map((o) => o.getAttribute("aria-label"))).toEqual(["=, is", "!=, is not", "in (, is one of", "notin (, is none of"]);
    expect(statusLine()).toHaveTextContent("4 suggestions.");

    // Nothing matches: one line that is not an option, and the box still submits as typed.
    await userEvent.clear(box);
    await userEvent.keyboard("zz");
    expect(expanded()).toBe("true");
    expect(within(listbox()).queryByRole("option")).toBeNull();
    expect(within(listbox()).getByText('No label matches "zz".')).toBeInTheDocument();
    expect(statusLine()).toHaveTextContent("No suggestions.");
    await userEvent.clear(box);
    await userEvent.keyboard("size=zz");
    expect(within(listbox()).getByText('No value of size matches "zz".')).toBeInTheDocument();
  });

  it("UI-21 more than 50 matches: 50 options and a line saying how many more", async () => {
    mockApi(withIndex([{ key: "rack", values: Array.from({ length: 64 }, (_, i) => term("rack", `r${String(i).padStart(2, "0")}`)) }]));
    mount();
    await tableEl();
    await userEvent.type(selectorBox(), "rack=");
    expect(within(listbox()).getAllByRole("option")).toHaveLength(50);
    const more = within(listbox()).getByText("14 more. Keep typing to narrow.");
    expect(more).not.toHaveAttribute("role", "option");
    expect(more.parentElement).toBe(listbox()); // outside the scrolling part: in view without scrolling
    expect(statusLine()).toHaveTextContent("64 suggestions.");
  });

  it("UI-21 with an empty index or no label_index the box is a plain text box with no placeholder", async () => {
    const { label_index: _l, ...older } = list();
    for (const response of [list({ label_index: [] }), older]) {
      const { calls } = mockApi(routes({ "GET /api/v1/setups": response }));
      const { unmount } = mount();
      await tableEl();
      const box = selectorBox();
      expect(box).not.toHaveAttribute("role");
      expect(box).not.toHaveAttribute("placeholder");
      expect(box).not.toHaveAttribute("aria-expanded");
      expect(screen.queryByRole("button", { name: "Show label suggestions" })).toBeNull();
      await userEvent.type(box, "size ");
      await userEvent.keyboard("{ArrowDown}");
      expect(screen.queryByRole("listbox")).toBeNull();
      await userEvent.keyboard("in (a,b){Enter}");
      await waitFor(() => expect(lastUrl(calls)).toBe(`/api/v1/setups?selector=${encodeURIComponent("size in (a,b)")}`));
      unmount();
    }
  });

  it("UI-21 the placeholder names a term of the index that is short enough to read", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    expect(selectorBox()).toHaveAttribute("placeholder", "Type or pick a label, e.g. owner=plain");
  });

  it("UI-21 keys pressed during composition are left to the input method", async () => {
    const { calls } = mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    const box = selectorBox();
    act(() => box.focus());
    const before = setupsCalls(calls);
    fireEvent.compositionStart(box);
    fireEvent.change(box, { target: { value: "si" } });
    expect(screen.queryByRole("listbox")).toBeNull(); // not computed while composing
    await userEvent.keyboard("{ArrowDown}{Enter}");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(box.value).toBe("si"); // nothing picked
    expect(setupsCalls(calls)).toBe(before); // nothing submitted
    fireEvent.compositionEnd(box);
    expect(optionTexts()).toEqual(["site", "size"]);
    await userEvent.keyboard("{Enter}");
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=si"));
  });

  it("UI-21 a refused selector shows the API's message and the forms line under the box, and is announced; no control moves into the message", async () => {
    mockApi(
      routes({
        "GET /api/v1/setups": ({ url }: { url: string }) =>
          url.includes("selector=") ? errorResponse(400, "invalid_selector", "selector: expected a value at position 9", "selector") : list({ label_index: labelIndex }),
      }),
    );
    mount();
    await tableEl();
    const box = selectorBox();
    await userEvent.type(box, "size in ({Enter}");
    const message = await screen.findByText("selector: expected a value at position 9");
    const help = screen.getByText("Forms: key=value, key!=value, key in (a,b), key notin (a,b), key, !key. Commas mean AND. The table still shows the last filter that worked.");
    expect(box).toHaveAttribute("aria-invalid", "true");
    expect(box.getAttribute("aria-describedby")!.split(" ")).toEqual([message.id, help.id]);
    expect(statusLine()).toHaveTextContent("Selector not accepted. selector: expected a value at position 9");
    expect(within(await tableEl()).getByText("setup-01")).toBeInTheDocument(); // the rows of the last filter that worked
    expect(box.value).toBe("size in (");
    // Both lines go when a request succeeds.
    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    await waitFor(() => expect(screen.queryByText("selector: expected a value at position 9")).toBeNull());
    expect(box).not.toHaveAttribute("aria-invalid");
  });

  it("UI-21 a selector that matches no setup says the filters match nothing, in the table as in the status line", async () => {
    mockApi(routes({ "GET /api/v1/setups": ({ url }: { url: string }) => list({ label_index: labelIndex }, url.includes("selector=") ? [] : setups) }));
    mount();
    await tableEl();
    await userEvent.type(selectorBox(), "site=ams,size=large{Enter}");
    expect(await screen.findByText("No setups match the filters.", { selector: "td" })).toBeInTheDocument();
    expect(screen.queryByText("No setups to show.")).toBeNull();
    await waitFor(() => expect(statusLine()).toHaveTextContent("No setups match the filters."));
    expect(screen.getByRole("button", { name: "Clear filters" })).toBeInTheDocument();
  });

  it("UI-21 the Filter button is the filled button only while typed text is unapplied and the list is closed", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    const filter = screen.getByRole("button", { name: "Filter" });
    expect(filter).not.toHaveClass(buttonStyles.primary!);
    await userEvent.type(selectorBox(), "lar");
    expect(optionTexts()).toEqual(["size=large"]);
    // The list is open: the suggestion is the action on offer, and Filter stays quiet.
    expect(filter).not.toHaveClass(buttonStyles.primary!);
    expect(filter).toHaveAccessibleDescription("The typed selector is not applied yet.");
    await userEvent.keyboard("{Escape}");
    expect(filter).toHaveClass(buttonStyles.primary!);
    await userEvent.keyboard("g");
    expect(filter).not.toHaveClass(buttonStyles.primary!);
    await userEvent.click(option("size=large"));
    expect(filter).not.toHaveClass(buttonStyles.primary!); // applied
    expect(filter).not.toHaveAccessibleDescription();
  });

  it("UI-9 in a narrow window the placeholder has no example", async () => {
    vi.stubGlobal("matchMedia", (query: string) => ({ matches: query === "(max-width: 480px)", addEventListener: () => {}, removeEventListener: () => {} }));
    mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    expect(selectorBox()).toHaveAttribute("placeholder", "Type or pick a label");
  });

  it("UI-21 the Filter button says when the typed selector is not applied yet", async () => {
    mockApi(withIndex(labelIndex));
    mount();
    await tableEl();
    const filter = screen.getByRole("button", { name: "Filter" });
    expect(filter).not.toHaveAccessibleDescription();
    await userEvent.type(selectorBox(), "tier=a");
    expect(filter).toHaveAccessibleDescription("The typed selector is not applied yet.");
    expect(screen.queryByRole("button", { name: "Clear filters" })).toBeNull(); // typed, not applied: no filter is in force
    await userEvent.click(filter);
    expect(filter).toHaveAccessibleName("Filter");
    expect(filter).not.toHaveAccessibleDescription();
    expect(screen.getByRole("button", { name: "Clear filters" })).toBeInTheDocument();
  });
});

const priced = (id: number, source: Setup["weight_source"], attrs: Record<string, string> = {}): Setup => ({
  ...base, id, name: `src-${id}`, labels: { size: "medium" }, weight: 1.15, weight_source: source, weight_attributes: attrs,
  state: "free", cost_per_hour_tokens: { now: 2.3, at_peak: 3.45 },
});
const mountWith = (items: Setup[], over: Partial<SetupList> = {}, more: Record<string, unknown> = {}) => {
  const api = mockApi(routes({ "GET /api/v1/setups": list(over, items), ...more }));
  return { ...api, ...mount() };
};
const SENTENCE = "Weights are reported by automation, so the price can change while the setup is held.";
const SOURCE_WORDS = [
  ["rule", "rule (set by a rate rule)"],
  ["default", "default (the default weight)"],
  ["reported", "reported (sent by automation)"],
  ["override", "override (pinned by an admin)"],
] as const;

describe("UI-12 weight source, attributes and the acquire warning", () => {
  it("UI-12 the cost cell carries a tag for reported and override, and none for rule or default", async () => {
    mountWith([priced(1, "reported"), priced(2, "override"), priced(3, "rule"), priced(4, "default")]);
    expect(within(await rowOf("src-1")).getByText("reported")).toBeInTheDocument();
    expect(within(await rowOf("src-2")).getByText("override")).toBeInTheDocument();
    for (const n of ["src-1", "src-2"]) {
      const cell = costButton(n).closest("td")!;
      expect(within(cell).getByText(/^(reported|override)$/)).toBeInTheDocument(); // the tag is in the cost cell
    }
    for (const n of ["src-3", "src-4"]) {
      const row = await rowOf(n);
      for (const word of ["reported", "override", "rule", "default"]) expect(within(row).queryByText(word)).toBeNull();
    }
  });

  it("UI-12 the panel shows the weight, its source and the attributes; the row shows none of them", async () => {
    mountWith([priced(1, "reported", { "memory-gb": "512", "storage-gb": "4000", tier: "gold" }), priced(2, "rule")]);
    const row = await rowOf("src-1");
    expect(within(row).queryByText("1.15")).toBeNull();
    expect(row).not.toHaveTextContent("1.15");
    expect(row).not.toHaveTextContent("memory-gb");
    const table = screen.getByRole("table", { name: "All setups" });
    expect(headersOf(table).some((h) => /Weight|memory|storage|tier/.test(h ?? ""))).toBe(false);
    expect(screen.queryByLabelText("memory-gb")).toBeNull(); // no filter either

    // WGT-14: the weight and its source are read from the panel.
    await userEvent.click(costButton("src-1"));
    let dialog = await screen.findByRole("dialog", { name: "Details: src-1" });
    expect(line(dialog, "Weight")).toHaveTextContent("1.15 · reported (sent by automation)");
    expect(within(dialog).getByText("1.15")).toBeInTheDocument();
    expect(line(dialog, "Attributes")).toHaveTextContent("memory-gb 512 · storage-gb 4000 · tier gold");
    await closePanel();
    dialog = await openPanel("src-2");
    expect(line(dialog, "Weight")).toHaveTextContent("1.15 · rule (set by a rate rule)");
    expect(line(dialog, "Attributes")).toBeNull();
  });

  it.each(["reported", "rule", "default", "override"] as const)(
    "UI-12 the acquire dialog shows the change warning when reported_weights_enabled is true, whatever the source (%s)",
    async (source) => {
      mountWith([priced(5, source, { tier: "gold" })], { reported_weights_enabled: true });
      await userEvent.click(await screen.findByRole("button", { name: "Acquire src-5" }));
      const dialog = await screen.findByRole("dialog", { name: "Acquire src-5?" });
      expect(dialog).toHaveTextContent("Weight 1.15");
      expect(dialog).toHaveTextContent("tier gold");
      expect(dialog).toHaveTextContent("Cost now 2.3 tokens / h, up to 3.45 tokens / h at the highest multiplier (3×).");
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

describe("UI-15 to UI-17, UI-20 the Setups table", () => {
  it("UI-15 the table's columns are Setup, State, Held by, Cost and actions, whatever display_label_keys holds", async () => {
    for (const keys of [["size"], ["label-a", "label-b", "label-c", "label-d", "label-e"], []]) {
      const { unmount } = mountWith(setups, { display_label_keys: keys });
      const headers = headersOf(await tableEl());
      expect(headers).toEqual(["Setup", "State", "Held by", "Cost (tokens / h)", "Actions"]);
      for (const s of setups) expect(await rowOf(s.name)).not.toHaveTextContent(s.labels.size!);
      unmount();
    }
  });

  it("UI-16 the cost cell shows now and at_peak with the unit, and one value when now is not below at_peak", async () => {
    const costing = (id: number, now: number, at_peak: number): Setup => ({ ...priced(id, "rule"), name: `cost-${id}`, cost_per_hour_tokens: { now, at_peak } });
    mountWith([costing(1, 8, 12), costing(2, 2, 2), costing(3, 4, 3), costing(4, 0, 6), costing(5, 0, 0)]);
    const table = await tableEl();
    expect(within(table).getByRole("columnheader", { name: "Cost (tokens / h)" })).toBeInTheDocument();
    const shown = (name: string) => costButton(name).querySelector("[aria-hidden]")!.textContent;
    expect(shown("cost-1")).toBe("8.0 now · up to 12.0");
    expect(shown("cost-2")).toBe("2.0"); // equal: one value
    expect(shown("cost-3")).toBe("4.0"); // a band edit before the next poll: never "4.0 now · up to 3.0"
    expect(shown("cost-4")).toBe("0.0 now · up to 6.0"); // the lab at 0×
    expect(shown("cost-5")).toBe("0.0");
    expect(table).not.toHaveTextContent("up to 3.0");
    expect(table).not.toHaveTextContent("if acquired");
    // The accessible name says the unit and the values in words.
    expect(costButton("cost-1")).toHaveAccessibleName("8.0 tokens per hour now, up to 12.0. Price details for cost-1");
    expect(costButton("cost-2")).toHaveAccessibleName("2.0 tokens per hour. Price details for cost-2");
    expect(costButton("cost-3")).toHaveAccessibleName("4.0 tokens per hour. Price details for cost-3");
    expect(costButton("cost-4")).toHaveAccessibleName("0.0 tokens per hour now, up to 6.0. Price details for cost-4");
  });

  it("UI-16 the cost cell carries the source tag for reported and override, and none for rule or default", async () => {
    mountWith([priced(1, "reported"), priced(2, "override"), priced(3, "rule"), priced(4, "default")]);
    await tableEl();
    const cell = (name: string) => costButton(name).closest("td")!;
    expect(within(cell("src-1")).getByText("reported")).not.toHaveAttribute("title");
    expect(within(cell("src-1")).getByText("The price comes from a reported weight.")).toHaveClass("sr-only");
    expect(within(cell("src-2")).getByText("override")).not.toHaveAttribute("title");
    expect(within(cell("src-2")).getByText("The price comes from a weight pinned by an admin.")).toHaveClass("sr-only");
    // The tag is beside the button, not inside it: the button's name is the price alone.
    expect(costButton("src-1")).toHaveAccessibleName("2.3 tokens per hour now, up to 3.45. Price details for src-1");
    for (const n of ["src-3", "src-4"]) {
      expect(cell(n)).not.toHaveTextContent(/reported|override|rule|default|The price comes from/);
    }
  });

  it('UI-17 the Held by cell shows the holder or You, how long, and "overdrawn" in words', async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-11-16T13:00:00Z"));
    mountWith(setups);
    const cell = async (name: string) => (await rowOf(name)).querySelectorAll("td")[2]!;
    expect(within(await tableEl()).getAllByRole("columnheader")[2]).toHaveTextContent("Held by");
    expect((await cell("setup-04")).textContent).toBe("You · 3 h");
    expect(within(await cell("setup-04")).getByText("You").tagName).toBe("SPAN"); // set apart, for the bold face
    expect((await cell("setup-01")).textContent).toBe("Sam Lee · 6 d 13 h");
    expect(within(await cell("setup-01")).getByText("Sam Lee")).toHaveAttribute("title", "Sam Lee");
    expect(within(await cell("setup-01")).getByText("Sam Lee")).toHaveStyle({ maxWidth: "24ch" });
    expect((await cell("setup-02")).textContent).toBe("Priya N. · 7 d 13 hoverdrawn");
    expect(within(await cell("setup-02")).getByText("overdrawn")).toBeInTheDocument();
    expect(within(await cell("setup-01")).queryByText("overdrawn")).toBeNull();
    expect((await cell("setup-05")).textContent).toBe(""); // free
    expect(await cell("setup-02")).not.toHaveTextContent("12"); // the balance is not on the row
  });

  it("UI-20 every column of the Setups table is one line", async () => {
    mountWith(setups, { display_label_keys: ["size", "label-b"] });
    const table = await tableEl();
    expect(within(table).getAllByRole("columnheader")).toHaveLength(5);
    for (const s of setups) {
      const cells = Array.from((await rowOf(s.name)).querySelectorAll("td"));
      expect(cells).toHaveLength(5);
      for (const td of cells) expect(td).toHaveClass(tableStyles.nowrap!);
    }
  });

  it("UI-16 the tag is beside the cost and the second line holds only a not-charging reason", async () => {
    const waiting: Setup = {
      ...priced(1, "reported"), state: "held", held_since: "2026-11-16T10:00:00Z",
      holder: { user_id: 3, hold_id: 97, name: "Sam Lee", balance_tokens: 5, overdrawn: false, is_you: false, release_requested_at: null },
      metering: { active: false, started_at: null, expected_start_at: null, reason: "a-reason-of-one-long-word" },
    };
    mountWith([waiting, priced(2, "override")]);
    await tableEl();
    const parts = (name: string) => Array.from(costButton(name).closest("td")!.children);
    const [button, tag, note, ...more] = parts("src-1");
    expect(button!.tagName).toBe("BUTTON");
    expect(within(tag as HTMLElement).getByText("reported")).toBeInTheDocument();
    expect(note!.textContent).toBe("a-reason-of-one-long-word");
    expect(more).toHaveLength(0);
    expect(parts("src-2")).toHaveLength(2); // the cost and its tag: one line
    // The cost now is marked up apart from the rest, and the button is not the blue link.
    expect(costButton("src-1").querySelector("[aria-hidden] > span")!.textContent).toBe("2.3");
    expect(costButton("src-1").className).not.toBe(detailsLink("src-1").className);
  });
});

describe("UI-18 the details panel", () => {
  const held = (id: number, over: Partial<Setup> = {}): Setup => ({
    ...base, id, name: `held-${id}`, labels: { size: "small", "not-shown": "elsewhere", blank: "" }, weight: 1.15, weight_source: "rule",
    state: "held", deployed: true, held_since: "2026-11-10T00:00:00Z",
    holder: { user_id: 9, hold_id: 900 + id, name: "A holder with a long name", balance_tokens: 61, overdrawn: false, is_you: false, release_requested_at: null },
    metering: { active: true, started_at: "2026-11-10T01:00:00Z", expected_start_at: null, reason: null },
    cost_per_hour_tokens: { now: 2.3, at_peak: 3.45 },
    ...over,
  });

  it("UI-18 the panel opens from the cost cell and from Details, with focus on Price, and returns focus on close", async () => {
    mountWith(setups);
    await tableEl();
    for (const opener of [() => costButton("setup-01"), () => detailsLink("setup-01")]) {
      for (const close of ["Escape", "Close"] as const) {
        opener().focus();
        await userEvent.keyboard("{Enter}"); // both are buttons: the keyboard opens them
        const dialog = await screen.findByRole("dialog", { name: "Details: setup-01" });
        expect(within(dialog).getByRole("heading", { level: 3, name: "Price" })).toHaveFocus();
        expect(within(dialog).getAllByRole("heading", { level: 3 }).map((h) => h.textContent)).toEqual(["Price", "Holder", "Labels", "Raw details"]);
        if (close === "Escape") await userEvent.keyboard("{Escape}");
        else await userEvent.click(within(dialog).getAllByRole("button", { name: "Close" }).at(-1)!);
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(opener()).toHaveFocus();
      }
    }
    // Space opens it too, and a free setup has no Holder section.
    costButton("setup-05").focus();
    await userEvent.keyboard(" ");
    const free = await screen.findByRole("dialog", { name: "Details: setup-05" });
    expect(within(free).queryByRole("heading", { name: "Holder" })).toBeNull();
  });

  it("UI-18 the panel shows weight, source, attributes, pressure and multiplier, both costs, charging, the holder, every label and the details JSON", async () => {
    const items: Setup[] = [
      ...SOURCE_WORDS.map(([source], i) => priced(i + 1, source, i === 2 ? { "memory-gb": "512" } : {})),
      held(10),
      held(11, { metering: { active: false, started_at: null, expected_start_at: null, reason: "Charging starts when deployed" } }),
      held(12, { metering: { active: false, started_at: null, expected_start_at: null, reason: null } }),
      held(13, { holder: { user_id: 2, hold_id: 913, name: "Dana Dev", balance_tokens: -12, overdrawn: true, is_you: true, release_requested_at: "2026-11-18T12:00:00Z" } }),
      { ...priced(14, "default"), labels: {}, details: null },
      { ...priced(15, "rule"), weight_source: "something-new" as Setup["weight_source"] },
    ];
    mountWith(items);
    await tableEl();

    // One case per source word.
    for (const [i, [, words]] of SOURCE_WORDS.entries()) {
      const dialog = await openPanel(`src-${i + 1}`);
      expect(line(dialog, "Weight")).toHaveTextContent(`1.15 · ${words}`);
      await closePanel();
    }
    let dialog = await openPanel("src-15");
    expect(line(dialog, "Weight")).toHaveTextContent("1.15 · something-new"); // an unknown source: the word as sent
    await closePanel();

    // A free setup: price lines, no holder, the "Charging starts" line from metering_description.
    dialog = await openPanel("src-3");
    expect(dialog).toHaveTextContent("Cost per hour is the setup's weight times the lab's multiplier, rounded to a thousandth of a token. The multiplier follows lab pressure.");
    expect(dialog).not.toHaveTextContent("=");
    expect(line(dialog, "Attributes")).toHaveTextContent("memory-gb 512");
    expect(line(dialog, "Lab pressure")).toHaveTextContent("80%");
    expect(line(dialog, "Multiplier now")).toHaveTextContent("2×");
    expect(line(dialog, "Cost now")!.textContent).toBe("2.3 tokens / h");
    expect(line(dialog, "Up to")!.textContent).toBe("3.45 tokens / h at the highest multiplier (3×)");
    expect(line(dialog, "Charging")!.textContent).toBe("Starts: deployed or 2 h after acquire");
    // The answer first, then the working; the row's words, not the Settings page's.
    expect(Array.from(dialog.querySelectorAll("dt")).map((d) => d.textContent)).toEqual(["Cost now", "Up to", "Charging", "Weight", "Attributes", "Lab pressure", "Multiplier now", "size"]);
    expect(dialog).not.toHaveTextContent(/band/i);
    expect(within(dialog).queryByRole("heading", { name: "Holder" })).toBeNull();
    expect(line(dialog, "Held by")).toBeNull();
    expect(line(dialog, "Balance")).toBeNull();
    await closePanel();

    // Held and metering: "Charging since", the holder's full name, since, balance; every label; the JSON.
    dialog = await openPanel("held-10");
    expect(line(dialog, "Charging")).toHaveTextContent(/^Since /);
    expect(line(dialog, "Charging")!.querySelector("time")).toHaveAttribute("datetime", "2026-11-10T01:00:00Z");
    expect(line(dialog, "Held by")!.textContent).toBe("A holder with a long name");
    expect(line(dialog, "Held since")!.querySelector("time")).toHaveAttribute("datetime", "2026-11-10T00:00:00Z");
    expect(line(dialog, "Held since")).toHaveTextContent(/\(\d+ d( \d+ h)?\)$/);
    expect(line(dialog, "Balance")!.textContent).toBe("61 tokens");
    expect(line(dialog, "Release")).toBeNull();
    expect(line(dialog, "size")!.textContent).toBe("small");
    expect(line(dialog, "not-shown")!.textContent).toBe("elsewhere"); // not a display key, still listed
    expect(line(dialog, "blank")!.textContent).toBe("(empty)");
    // Sorted by key, each once; the hint on how to use them follows.
    expect(Array.from(within(dialog).getByRole("heading", { name: "Labels" }).nextElementSibling!.querySelectorAll("dt")).map((d) => d.textContent)).toEqual(["blank", "not-shown", "size"]);
    expect(within(dialog).getByText("Use a label in the Label selector box to filter the list.")).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Setup details JSON").textContent).toBe(JSON.stringify(base.details, null, 2));
    expect(dialog).not.toHaveTextContent("last_seen_at"); // the admin block is not shown
    await closePanel();

    // Held, not metering: with and without a reason.
    dialog = await openPanel("held-11");
    expect(line(dialog, "Charging")!.textContent).toBe("Not yet: Charging starts when deployed");
    await closePanel();
    dialog = await openPanel("held-12");
    expect(line(dialog, "Charging")!.textContent).toBe("Not yet");
    await closePanel();

    // Your own overdrawn hold with a pending release.
    dialog = await openPanel("held-13");
    expect(line(dialog, "Held by")!.textContent).toBe("You");
    expect(line(dialog, "Balance")!.textContent).toBe("−12 tokens, overdrawn");
    expect(line(dialog, "Release")).toHaveTextContent(/^Release requested .+\. It completes when the setup is torn down\.$/);
    expect(line(dialog, "Release")!.querySelector("time")).toHaveAttribute("datetime", "2026-11-18T12:00:00Z");
    await closePanel();

    // No labels, no details.
    dialog = await openPanel("src-14");
    expect(within(dialog).getByText("No labels.")).toBeInTheDocument();
    expect(within(dialog).queryByText("Use a label in the Label selector box to filter the list.")).toBeNull();
    expect(within(dialog).queryByLabelText("Setup details JSON")).toBeNull();
    expect(within(dialog).getByRole("heading", { name: "Raw details" }).nextElementSibling!.textContent).toBe("None.");
  });

  it("UI-18 a free setup has no Charging line when metering_description is null, and pressure reads not measured yet before the first poll", async () => {
    mountWith([priced(1, "rule")], { metering_description: null, pricing: { multiplier: 0, peak_multiplier: 3, pressure: null } });
    await tableEl();
    const dialog = await openPanel("src-1");
    expect(line(dialog, "Charging")).toBeNull();
    expect(line(dialog, "Lab pressure")!.textContent).toBe("not measured yet");
    expect(line(dialog, "Multiplier now")!.textContent).toBe("0×");
  });

  it("UI-18 the panel shows the price-change sentence whenever reported_weights_enabled is true", async () => {
    const items = SOURCE_WORDS.map(([source], i) => priced(i + 1, source));
    for (const enabled of [true, false]) {
      const { unmount } = mountWith(items, { reported_weights_enabled: enabled });
      await tableEl();
      for (const s of items) {
        const dialog = await openPanel(s.name);
        expect(within(dialog).queryAllByText(SENTENCE)).toHaveLength(enabled ? 1 : 0);
        await closePanel();
      }
      unmount();
    }
  });

  it("UI-18 the multiplier and pressure in the panel are those of the setups response", async () => {
    const other = { ...pressure, raw: 0.1, smoothed: 0.11, multiplier: 7 };
    for (const answer of [other, () => errorResponse(500, "internal", "pressure is down")]) {
      const { unmount } = mountWith(setups, {}, { "GET /api/v1/pressure": answer });
      await tableEl();
      const dialog = await openPanel("setup-02");
      expect(line(dialog, "Lab pressure")!.textContent).toBe("80%");
      expect(line(dialog, "Multiplier now")!.textContent).toBe("2×");
      expect(line(dialog, "Cost now")!.textContent).toBe("4.0 tokens / h");
      expect(line(dialog, "Up to")!.textContent).toBe("6.0 tokens / h at the highest multiplier (3×)");
      expect(dialog).not.toHaveTextContent("7×");
      expect(dialog).not.toHaveTextContent("11%");
      unmount();
    }
  });

  it("UI-18 the panel opens without pricing in the response", async () => {
    // An API older than the UI: no `pricing`, and no `label_index` either.
    const { pricing: _p, label_index: _l, ...older } = list();
    mockApi(routes({ "GET /api/v1/setups": older }));
    mount();
    await tableEl();
    await userEvent.click(costButton("setup-02"));
    const dialog = await screen.findByRole("dialog", { name: "Details: setup-02" });
    expect(line(dialog, "Weight")).toHaveTextContent("2 · default (the default weight)");
    expect(line(dialog, "Cost now")!.textContent).toBe("4.0 tokens / h");
    expect(line(dialog, "Up to")!.textContent).toBe("6.0 tokens / h at the highest multiplier");
    expect(line(dialog, "Lab pressure")).toBeNull();
    expect(line(dialog, "Multiplier now")).toBeNull();
    expect(dialog.children[1]).not.toHaveTextContent("×"); // the body; the header holds the close x
    expect(line(dialog, "Balance")!.textContent).toBe("−12 tokens, overdrawn");
  });

  it("UI-18 a weight that three decimals would show as 0 is printed as the API sent it", async () => {
    mountWith([{ ...priced(1, "rule"), weight: 0.0004, cost_per_hour_tokens: { now: 0.001, at_peak: 0.001 } }]);
    await tableEl();
    const dialog = await openPanel("src-1");
    expect(line(dialog, "Weight")).toHaveTextContent("0.0004 · rule (set by a rate rule)");
    expect(line(dialog, "Cost now")!.textContent).toBe("0.001 tokens / h");
  });

  it("UI-18 the panel does not show a highest-band cost below the cost now", async () => {
    mountWith([{ ...priced(1, "rule"), cost_per_hour_tokens: { now: 4, at_peak: 3 } }, { ...priced(2, "rule"), cost_per_hour_tokens: { now: 3, at_peak: 3 } }], {
      pricing: { multiplier: 4, peak_multiplier: 3, pressure: 0.9 },
    });
    await tableEl();
    let dialog = await openPanel("src-1");
    expect(line(dialog, "Cost now")!.textContent).toBe("4.0 tokens / h");
    expect(line(dialog, "Up to")!.textContent).toBe("The multipliers were changed. This value shows after the next poll.");
    expect(dialog).not.toHaveTextContent("3.0 tokens / h");
    await closePanel();
    // Equal costs: both show.
    dialog = await openPanel("src-2");
    expect(line(dialog, "Cost now")!.textContent).toBe("3.0 tokens / h");
    expect(line(dialog, "Up to")!.textContent).toBe("3.0 tokens / h at the highest multiplier (3×)");
  });

  it("UI-18 an open panel follows a refresh; when its setup is no longer listed it keeps one response's values and focus returns to the heading", async () => {
    const NOTICE = "This setup is no longer listed. These are the last values loaded.";
    let response = list();
    mockApi(routes({ "GET /api/v1/setups": () => response }));
    mount();
    await tableEl();
    const dialog = await openPanel("setup-05");
    const price = within(dialog).getByRole("heading", { name: "Price" });
    expect(price).toHaveFocus();
    expect(line(dialog, "Multiplier now")!.textContent).toBe("2×");
    expect(line(dialog, "Cost now")!.textContent).toBe("2.0 tokens / h");
    expect(within(dialog).queryByRole("heading", { name: "Holder" })).toBeNull();
    expect(within(dialog).queryByText(NOTICE)).toBeNull();

    // A refresh: the setup was acquired and the lab moved. The panel shows the new values; focus does not move.
    const acquired: Setup = {
      ...setups[3]!, state: "held", held_since: "2026-11-18T00:00:00Z", cost_per_hour_tokens: { now: 2.5, at_peak: 3 },
      holder: { user_id: 9, hold_id: 95, name: "Sam Lee", balance_tokens: 61, overdrawn: false, is_you: false, release_requested_at: null },
      metering: { active: false, started_at: null, expected_start_at: null, reason: null },
    };
    response = list({ pricing: { multiplier: 2.5, peak_multiplier: 3, pressure: 0.9 } }, setups.map((s) => (s.id === 5 ? acquired : s)));
    await refresh();
    await waitFor(() => expect(line(dialog, "Multiplier now")!.textContent).toBe("2.5×"));
    expect(line(dialog, "Cost now")!.textContent).toBe("2.5 tokens / h");
    expect(line(dialog, "Lab pressure")!.textContent).toBe("90%");
    expect(line(dialog, "Held by")!.textContent).toBe("Sam Lee");
    expect(line(dialog, "Charging")!.textContent).toBe("Not yet");
    expect(price).toHaveFocus();

    // The setup leaves the list and the lab moves again: the panel keeps the values of the last response that held it.
    response = list({ pricing: { multiplier: 5, peak_multiplier: 6, pressure: 0.99 }, reported_weights_enabled: true }, setups.filter((s) => s.id !== 5));
    await refresh();
    expect(await within(dialog).findByText(NOTICE)).toHaveAttribute("role", "status");
    expect(screen.queryByRole("button", { name: "Details for setup-05" })).toBeNull();
    expect(screen.getByRole("dialog", { name: "Details: setup-05" })).toBe(dialog);
    expect(line(dialog, "Multiplier now")!.textContent).toBe("2.5×");
    expect(line(dialog, "Lab pressure")!.textContent).toBe("90%");
    expect(line(dialog, "Cost now")!.textContent).toBe("2.5 tokens / h");
    expect(line(dialog, "Up to")!.textContent).toBe("3.0 tokens / h at the highest multiplier (3×)");
    expect(within(dialog).queryByText(SENTENCE)).toBeNull(); // not mixed in from the newer response

    // The control that opened it is gone: focus goes to the page heading.
    await userEvent.click(within(dialog).getAllByRole("button", { name: "Close" }).at(-1)!);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Setups" })).toHaveFocus();
  });

  it("UI-18 when the State filter hides the row of an open panel, the panel keeps following and focus returns to the heading", async () => {
    let response = list();
    mockApi(routes({ "GET /api/v1/setups": () => response }));
    mount();
    await tableEl();
    await userEvent.selectOptions(screen.getByLabelText("State"), "free");
    const dialog = await openPanel("setup-05");
    response = list({}, setups.map((s) => (s.id === 5 ? { ...s, state: "held" as const, cost_per_hour_tokens: { now: 2.5, at_peak: 3 } } : s)));
    await refresh();
    await waitFor(() => expect(line(dialog, "Cost now")!.textContent).toBe("2.5 tokens / h"));
    expect(within(dialog).queryByRole("status")).toBeNull(); // still listed, only filtered from the table
    await userEvent.keyboard("{Escape}");
    expect(screen.getByRole("heading", { level: 1, name: "Setups" })).toHaveFocus();
  });
});

describe("UI-22 exact-match tokens", () => {
  const OTHER = "Team B, core";
  const item = (id: number, state: Setup["state"], labels: Record<string, string>): Setup => ({ ...priced(id, "rule"), name: `lab-${id}`, state, labels });
  const items = [
    item(1, "free", { owner: AWKWARD, site: "ams" }),
    item(2, "disabled", { owner: AWKWARD, site: "sjc" }),
    item(3, "free", { owner: OTHER, site: "ams" }),
    item(4, "free", { owner: "plain", site: "ams" }),
    item(5, "free", { site: "sjc" }),
  ];
  const index: IndexKey[] = [
    { key: "owner", values: [{ value: AWKWARD, selector: null }, { value: OTHER, selector: null }, term("owner", "plain")] },
    { key: "site", values: [term("site", "ams"), term("site", "sjc")] },
  ];
  const names = async () => Array.from((await tableEl()).querySelectorAll("tbody tr strong")).map((n) => n.textContent);
  const ALL = ["lab-1", "lab-2", "lab-3", "lab-4", "lab-5"];
  // The mock applies a `site=` selector the way the API would, so the test sees both filters combine.
  const serve = ({ url }: { url: string }) => {
    const site = /selector=site%3D(\w+)/.exec(url)?.[1];
    return list({ label_index: index }, site ? items.filter((s) => s.labels.site === site) : items);
  };
  const start = async (setups: unknown = serve) => {
    const api = mockApi(routes({ "GET /api/v1/session": adminSession, "GET /api/v1/setups": setups }));
    mount();
    expect(await names()).toEqual(ALL);
    return api;
  };
  const tokenList = () => screen.queryByRole("list", { name: "Exact match:" });
  const tokenTexts = () => (tokenList() ? within(tokenList()!).getAllByRole("listitem").map((li) => li.firstElementChild!.textContent) : []);
  const removeButton = (value: string) => screen.getByRole("button", { name: `Remove exact-match filter owner=${value}` });
  /** Types `owner=` and picks the value marked "exact match". */
  const pickExact = async (value: string) => {
    await userEvent.type(selectorBox(), "owner=");
    await userEvent.click(option(`${value}, exact match`));
  };

  it("UI-22 picking a value with no selector term adds an exact-match token, filters the rows in the browser and sends nothing to the API", async () => {
    const { calls } = await start();
    const fetched = setupsCalls(calls);
    expect(tokenList()).toBeNull();
    expect(screen.queryByRole("button", { name: "Clear filters" })).toBeNull(); // no filter in force: no line

    await userEvent.type(selectorBox(), "site=ams,owner=");
    expect(optionTexts()).toEqual([AWKWARD, OTHER, "plain"]);
    expect(option(/^plain/)).toHaveAccessibleName("plain"); // a value with a term carries no note
    await userEvent.click(option(`${AWKWARD}, exact match`));
    // The requirement leaves the box with its comma; the rest of the box is neither changed nor applied.
    expect(selectorBox().value).toBe("site=ams");
    expect(selectorBox()).toHaveFocus();
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(tokenTexts()).toEqual([`owner=${AWKWARD}`]);
    expect(within(tokenList()!).getByText(`owner=${AWKWARD}`)).toHaveAttribute("title", `owner=${AWKWARD}`);
    expect(removeButton(AWKWARD)).toBeInTheDocument();
    expect(await names()).toEqual(["lab-1", "lab-2"]);
    expect(screen.getByText("2 setups shown")).toBeInTheDocument();
    expect(statusLine()).toHaveTextContent(`Exact-match filter owner=${AWKWARD} added. 2 setups shown.`);
    expect(setupsCalls(calls)).toBe(fetched);
    expect(calls.some((c) => decodeURIComponent(c.url).includes("Team"))).toBe(false);
  });

  it("UI-22 a value with no term typed at the start of a requirement is offered as key=value and becomes a token", async () => {
    const { calls } = await start();
    const fetched = setupsCalls(calls);
    await userEvent.type(selectorBox(), "core");
    await userEvent.click(option(`owner=${OTHER}, exact match`));
    expect(selectorBox().value).toBe("");
    expect(tokenTexts()).toEqual([`owner=${OTHER}`]);
    expect(await names()).toEqual(["lab-3"]);
    expect(screen.getByText("1 setup shown")).toBeInTheDocument();
    expect(setupsCalls(calls)).toBe(fetched);
  });

  it("UI-22 a token combines with the selector and the State filter; another value of the same key replaces it; the same value again changes nothing", async () => {
    await start();
    await pickExact(AWKWARD);
    expect(await names()).toEqual(["lab-1", "lab-2"]);

    // With the State filter.
    await userEvent.selectOptions(screen.getByLabelText("State"), "free");
    expect(await names()).toEqual(["lab-1"]);
    expect(statusLine()).toHaveTextContent("1 setup shown.");
    await userEvent.selectOptions(screen.getByLabelText("State"), "disabled");
    expect(await names()).toEqual(["lab-2"]);
    await userEvent.selectOptions(screen.getByLabelText("State"), "");

    // With a picked selector value: the selector goes to the API, the token stays in force over its answer.
    await userEvent.type(selectorBox(), "site=");
    await userEvent.click(option("sjc"));
    await waitFor(async () => expect(await names()).toEqual(["lab-2"]));
    expect(selectorBox().value).toBe("site=sjc");
    expect(tokenTexts()).toEqual([`owner=${AWKWARD}`]);
    await waitFor(() => expect(statusLine()).toHaveTextContent("1 setup shown."));

    // With typed selector text.
    await userEvent.clear(selectorBox());
    await userEvent.type(selectorBox(), "site=ams");
    await userEvent.click(screen.getByRole("button", { name: "Filter" }));
    await waitFor(async () => expect(await names()).toEqual(["lab-1"]));

    // The same value again: nothing changes, and the box is not left with the requirement.
    await userEvent.type(selectorBox(), ",owner=");
    await userEvent.click(option(`${AWKWARD}, exact match`));
    expect(tokenTexts()).toEqual([`owner=${AWKWARD}`]);
    expect(selectorBox().value).toBe("site=ams");

    // Another value of the same key replaces the token.
    await userEvent.type(selectorBox(), ",owner=");
    await userEvent.click(option(`${OTHER}, exact match`));
    expect(tokenTexts()).toEqual([`owner=${OTHER}`]);
    expect(await names()).toEqual(["lab-3"]);

    // No row left: the table says so, the count is left out, and the way out is still on screen.
    await userEvent.selectOptions(screen.getByLabelText("State"), "disabled");
    expect(screen.getByText("No setups match the filters.", { selector: "td" })).toBeInTheDocument();
    expect(screen.queryByText(/setups? shown$/)).toBeNull();
    expect(statusLine()).toHaveTextContent("No setups match the filters.");
    expect(screen.getByRole("button", { name: "Clear filters" })).toBeInTheDocument();
  });

  it("UI-22 removing a token restores the rows and returns focus to the selector box", async () => {
    const { calls } = await start();
    await pickExact(AWKWARD);
    expect(await names()).toEqual(["lab-1", "lab-2"]);
    const fetched = setupsCalls(calls);
    await userEvent.click(removeButton(AWKWARD));
    expect(tokenList()).toBeNull();
    expect(await names()).toEqual(ALL);
    expect(selectorBox()).toHaveFocus();
    expect(expanded()).toBe("false"); // focus alone does not open the list
    expect(statusLine()).toHaveTextContent(`Exact-match filter owner=${AWKWARD} removed. 5 setups shown.`);
    expect(screen.queryByRole("button", { name: "Clear filters" })).toBeNull();
    expect(setupsCalls(calls)).toBe(fetched);
  });

  it("UI-22 a token whose value is no longer in label_index stays shown and removable", async () => {
    let response = list({ label_index: index }, items);
    await start(() => response);
    await pickExact(AWKWARD);
    expect(await names()).toEqual(["lab-1", "lab-2"]);
    response = list({ label_index: [index[1]!] }, items.slice(2));
    await refresh();
    await waitFor(() => expect(screen.getByText("No setups match the filters.", { selector: "td" })).toBeInTheDocument());
    expect(tokenTexts()).toEqual([`owner=${AWKWARD}`]); // still in force, with its control
    await userEvent.click(removeButton(AWKWARD));
    expect(await names()).toEqual(["lab-3", "lab-4", "lab-5"]);
  });

  it("UI-22 Clear filters resets the State filter, the box, the applied selector and the tokens, and moves focus to the box", async () => {
    const { calls } = await start();
    await pickExact(AWKWARD);
    await userEvent.selectOptions(screen.getByLabelText("State"), "free");
    await userEvent.type(selectorBox(), "site=");
    await userEvent.click(option("ams"));
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=site%3Dams"));
    expect(await names()).toEqual(["lab-1"]);
    // Tab order of the filter area: State, the box, Filter, the token's remove button, Clear filters.
    act(() => screen.getByLabelText("State").focus());
    const stops = [];
    for (let i = 0; i < 4; i++) {
      await userEvent.tab();
      stops.push(document.activeElement);
    }
    expect(stops).toEqual([selectorBox(), screen.getByRole("button", { name: "Filter" }), removeButton(AWKWARD), screen.getByRole("button", { name: "Clear filters" })]);

    await userEvent.keyboard("{Enter}");
    expect(screen.getByLabelText("State")).toHaveValue("");
    expect(selectorBox().value).toBe("");
    expect(tokenList()).toBeNull();
    expect(screen.queryByRole("button", { name: "Clear filters" })).toBeNull();
    expect(selectorBox()).toHaveFocus();
    expect(screen.queryByRole("listbox")).toBeNull();
    // "Filters cleared." is announced first (next test); the count follows when the response is in.
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups"));
    await waitFor(async () => expect(await names()).toEqual(ALL));
    await waitFor(() => expect(statusLine()).toHaveTextContent("5 setups shown."));
  });

  it("UI-22 when an exact-match pick leaves the box different from the applied selector, the line names the selector in force", async () => {
    const { calls } = await start();
    await userEvent.type(selectorBox(), "site=sjc,owner=plain{Enter}");
    await waitFor(() => expect(lastUrl(calls)).toBe(`/api/v1/setups?selector=${encodeURIComponent("site=sjc,owner=plain")}`));
    expect(screen.queryByText(/Selector in force:/)).toBeNull(); // the box says it
    // The owner requirement is rewritten and an exact value picked: it leaves the box, and nothing is applied.
    await userEvent.keyboard("{Backspace}{Backspace}{Backspace}{Backspace}{Backspace}");
    await userEvent.click(option(`${AWKWARD}, exact match`));
    expect(selectorBox().value).toBe("site=sjc");
    expect(tokenTexts()).toEqual([`owner=${AWKWARD}`]);
    expect(lastUrl(calls)).toBe(`/api/v1/setups?selector=${encodeURIComponent("site=sjc,owner=plain")}`);
    const inForce = screen.getByText(/Selector in force:/);
    expect(inForce).toHaveTextContent("Selector in force: site=sjc,owner=plain");
    expect(inForce.querySelector("code")!.textContent).toBe("site=sjc,owner=plain");
    // Filter applies the box, and the two agree again.
    await userEvent.click(screen.getByRole("button", { name: "Filter" }));
    await waitFor(() => expect(lastUrl(calls)).toBe("/api/v1/setups?selector=site%3Dsjc"));
    expect(screen.queryByText(/Selector in force:/)).toBeNull();
    expect(await names()).toEqual(["lab-2"]);
  });

  it("UI-22 a setup that comes without a labels object does not match a token and does not break the page", async () => {
    const bare = { ...item(9, "free", {}), labels: undefined } as unknown as Setup;
    mockApi(routes({ "GET /api/v1/setups": list({ label_index: index }, [...items, bare]) }));
    mount();
    expect(await names()).toEqual([...ALL, "lab-9"]);
    await pickExact(AWKWARD);
    expect(await names()).toEqual(["lab-1", "lab-2"]);
  });

  it("UI-22 Clear filters is announced, and the 60-second refresh announces nothing", async () => {
    await start();
    await userEvent.selectOptions(screen.getByLabelText("State"), "free");
    expect(statusLine()).toHaveTextContent("4 setups shown.");
    expect(screen.getByText("4 setups shown")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(statusLine()).toHaveTextContent("Filters cleared.");
    expect(selectorBox()).toHaveFocus();
    await userEvent.selectOptions(screen.getByLabelText("State"), "free");
    expect(statusLine()).toHaveTextContent("4 setups shown.");
    await userEvent.click(selectorBox());
    expect(statusLine()).toHaveTextContent("2 suggestions.");
    await refresh();
    await act(async () => {});
    expect(statusLine()).toHaveTextContent("2 suggestions.");
  });
});

describe("A1 how a person is shown (names)", () => {
  const holderOf = (name: string, over: Record<string, unknown> = {}) =>
    setups.map((x) =>
      x.name === "setup-01" ? { ...x, holder: { ...x.holder!, name, ...over } } : x,
    ) as Setup[];

  it("A1-1 P2 P3 a named holder shows the full name in the table and the panel, and no email anywhere", async () => {
    mockApi(routes({ "GET /api/v1/setups": list({}, holderOf("Ada Lovelace")) }));
    mount();
    const cell = (await rowOf("setup-01")).querySelectorAll("td")[2]!;
    expect(within(cell).getByText("Ada Lovelace")).toHaveAttribute("title", "Ada Lovelace");
    const dialog = await openPanel("setup-01");
    expect(line(dialog, "Held by")).toHaveTextContent("Ada Lovelace");
    expect(document.body.innerHTML).not.toContain("@");
  });

  it("A1-2 P2 P3 a holder with no name shows what the API sent in its place (username or email), never blank", async () => {
    mockApi(routes({ "GET /api/v1/setups": list({}, holderOf("sam")) }));
    mount();
    const cell = (await rowOf("setup-01")).querySelectorAll("td")[2]!;
    expect(cell.textContent).toMatch(/^sam · /);
    expect(line(await openPanel("setup-01"), "Held by")).toHaveTextContent("sam");
  });

  it("A1-6 P2 P3 a long name is cut at 24 characters in the table with the full name in title, and in full in the panel", async () => {
    const long = "A very long full name that goes past forty characters";
    mockApi(routes({ "GET /api/v1/setups": list({}, holderOf(long)) }));
    mount();
    const cell = (await rowOf("setup-01")).querySelectorAll("td")[2]!;
    const clip = within(cell).getByText(long);
    expect(clip).toHaveAttribute("title", long);
    expect(clip).toHaveStyle({ maxWidth: "24ch" });
    expect(line(await openPanel("setup-01"), "Held by")).toHaveTextContent(long);
  });

  it("A1-5 the viewer's own hold reads You in the table and in the panel", async () => {
    mockApi(routes());
    mount();
    expect((await rowOf("setup-04")).querySelectorAll("td")[2]!.textContent).toMatch(/^You · /);
    expect(line(await openPanel("setup-04"), "Held by")).toHaveTextContent("You");
  });

  it("B2.3 the Acquire dialog says tokens / h and the highest multiplier, never band", async () => {
    mockApi(routes());
    mount();
    await userEvent.click(await screen.findByRole("button", { name: "Acquire setup-05" }));
    const dialog = await screen.findByRole("dialog", { name: "Acquire setup-05?" });
    expect(dialog).toHaveTextContent(/Cost now [\d.]+ tokens \/ h, up to [\d.]+ tokens \/ h at the highest multiplier \(3×\)\./);
    expect(dialog.textContent).not.toMatch(/band/i);
  });
});
