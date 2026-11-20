// SPDX-License-Identifier: MIT
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { setCsrfToken } from "../../api/client";
import { errorResponse, mockApi } from "../../test/mockApi";
import { emptyCollector, type RulesResponse, type SettingsResponse } from "../admin-shared/fixtures";
import Page from "./index";

const settings: SettingsResponse = {
  settings: {
    default_weight: 1,
    display_label_keys: ["size"],
    bands: { base: 0, steps: [{ from: 0.5, multiplier: 1 }, { from: 0.75, multiplier: 2 }] },
    default_allowance: 300,
    max_holds_per_user: 2,
    release_requires_teardown: true,
    poll_interval: "5m",
    enforcement: false,
  },
  runtime: {
    provider: "vsphere",
    pressure_source: "vsphere",
    metering_policy: { name: "default", description: "deployed or 2 h after acquire" },
    hooks: ["notify"],
    oidc_issuer: "https://issuer.example",
    timezone: "UTC",
  },
};
const rules: RulesResponse = {
  rules: [
    { id: 1, position: 0, selector: "size=extralarge", weight: 4, match_count: 1 },
    { id: 2, position: 1, selector: "size=large", weight: 2, match_count: 3 },
  ],
  default_weight: 1,
  unmatched_setups: [{ id: 5, name: "setup-05", labels: {} }],
};
const collector = {
  ...emptyCollector,
  last_error: { message: "vcenter unreachable", at: "2026-11-18T17:00:00Z" },
  outages: [{ started_at: "2026-11-10T10:00:00Z", ended_at: "2026-11-10T10:35:00Z", duration_minutes: 35 }],
  failed_deliveries: [
    { id: 1, event_type: "hold.acquired", hook: "notify", error: "timeout", started_at: "2026-11-18T10:00:00Z", finished_at: null },
  ],
};

function routes(extra: Record<string, unknown> = {}) {
  return {
    "GET /api/v1/admin/settings": settings,
    "GET /api/v1/admin/rules": rules,
    "GET /api/v1/admin/collector": collector,
    ...extra,
  };
}

beforeEach(() => setCsrfToken("csrf-admin"));

describe("ADM-2 settings page states", () => {
  it("shows loading, then settings, rules with match counts and the default-weight list", async () => {
    mockApi(routes());
    render(<Page />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading settings");
    expect(await screen.findByLabelText("Monthly allowance (tokens)")).toHaveValue("300");
    expect(screen.getByLabelText("Selector 1")).toHaveValue("size=extralarge");
    const table = screen.getByRole("table", { name: "Rate rules" });
    expect(within(table).getByText("3 setups")).toBeInTheDocument();
    expect(within(table).getByText("1 setup")).toBeInTheDocument();
    expect(screen.getByRole("list", { name: "Setups using the default weight" })).toHaveTextContent("setup-05");
  });

  it("ADM-2 shows shadow mode when enforcement is off", async () => {
    mockApi(routes());
    render(<Page />);
    expect(await screen.findByText(/Shadow mode/)).toBeInTheDocument();
    await userEvent.click(screen.getByLabelText("Block acquiring at zero tokens"));
    expect(screen.queryByText(/Shadow mode/)).toBeNull();
  });

  it("shows the API error", async () => {
    mockApi(routes({ "GET /api/v1/admin/settings": () => errorResponse(500, "internal", "settings unavailable") }));
    render(<Page />);
    expect(await screen.findByText("settings unavailable")).toBeInTheDocument();
  });

  it("ADM-4 shows collector health and read-only runtime names", async () => {
    mockApi(routes());
    render(<Page />);
    expect(await screen.findByText(/vcenter unreachable/)).toBeInTheDocument();
    expect(screen.getByText("0 min, not charged")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Outage history" })).getByText("35")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Failed event deliveries" })).getByText("hold.acquired")).toBeInTheDocument();
    expect(screen.getByText(/Provider: vsphere/)).toHaveTextContent("OIDC issuer: https://issuer.example");
    expect(screen.getByText(/Provider: vsphere/)).toHaveTextContent("Hooks: notify");
  });

  it("shows an empty collector state", async () => {
    mockApi(routes({ "GET /api/v1/admin/collector": emptyCollector, "GET /api/v1/admin/rules": { ...rules, rules: [], unmatched_setups: [] } }));
    render(<Page />);
    expect(await screen.findByText("No outages.")).toBeInTheDocument();
    expect(screen.getByText("No rules: every setup uses the default weight.")).toBeInTheDocument();
  });
});

describe("unsaved changes, nothing saved implicitly", () => {
  it("is disabled until something changes, shows the indicator, and sends nothing before Save", async () => {
    const { calls } = mockApi(routes());
    render(<Page />);
    const save = await screen.findByRole("button", { name: "Save changes" });
    expect(save).toBeDisabled();
    expect(screen.queryByText("Unsaved changes")).toBeNull();
    await userEvent.clear(screen.getByLabelText("Max setups per developer"));
    await userEvent.type(screen.getByLabelText("Max setups per developer"), "3");
    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
    expect(save).toBeEnabled();
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
  });

  it("ADM-2 saves settings with PUT /admin/settings and the CSRF header, without calling rules", async () => {
    const { calls } = mockApi(routes({ "PUT /api/v1/admin/settings": settings }));
    render(<Page />);
    await userEvent.clear(await screen.findByLabelText("Max setups per developer"));
    await userEvent.type(screen.getByLabelText("Max setups per developer"), "3");
    await userEvent.click(screen.getByLabelText("Block acquiring at zero tokens"));
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    const puts = calls.filter((c) => c.method === "PUT");
    expect(puts).toHaveLength(1);
    expect(puts[0]!.url).toBe("/api/v1/admin/settings");
    expect(puts[0]!.headers.get("X-CSRF-Token")).toBe("csrf-admin");
    expect(puts[0]!.body).toEqual({ ...settings.settings, max_holds_per_user: 3, enforcement: true });
    expect(await screen.findByText(/Settings saved/)).toBeInTheDocument();
  });

  it("shows a settings validation error against its field", async () => {
    mockApi(routes({ "PUT /api/v1/admin/settings": () => errorResponse(400, "validation", "poll interval must be at least 1m", "poll_interval") }));
    render(<Page />);
    await userEvent.clear(await screen.findByLabelText("Poll interval"));
    await userEvent.type(screen.getByLabelText("Poll interval"), "10s");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("poll interval must be at least 1m")).toBeInTheDocument();
  });

  it("rejects a non-number locally without calling the API", async () => {
    const { calls } = mockApi(routes());
    render(<Page />);
    await userEvent.clear(await screen.findByLabelText("Monthly allowance (tokens)"));
    await userEvent.type(screen.getByLabelText("Monthly allowance (tokens)"), "abc");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("Enter a number.")).toBeInTheDocument();
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
  });
});

describe("RULE-3 rate rules", () => {
  it("adds, edits, reorders and deletes, then saves the ordered list through PUT /admin/rules", async () => {
    const { calls } = mockApi(routes({ "PUT /api/v1/admin/rules": rules }));
    render(<Page />);
    await screen.findByLabelText("Selector 1");
    await userEvent.click(screen.getByRole("button", { name: "Move rule 2 up" }));
    expect(screen.getByLabelText("Selector 1")).toHaveValue("size=large");
    await userEvent.click(screen.getByRole("button", { name: "Add rule" }));
    await userEvent.type(screen.getByLabelText("Selector 3"), "tier in (a,b)");
    await userEvent.clear(screen.getByLabelText("Weight 3"));
    await userEvent.type(screen.getByLabelText("Weight 3"), "5");
    await userEvent.click(screen.getByRole("button", { name: "Remove rule 2" }));
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    const puts = calls.filter((c) => c.method === "PUT");
    expect(puts).toHaveLength(1);
    expect(puts[0]!.url).toBe("/api/v1/admin/rules");
    expect(puts[0]!.body).toEqual({
      rules: [
        { selector: "size=large", weight: 2 },
        { selector: "tier in (a,b)", weight: 5 },
      ],
    });
    expect(puts[0]!.headers.get("X-CSRF-Token")).toBe("csrf-admin");
  });

  it("shows an invalid selector error against the offending rule and keeps the draft", async () => {
    mockApi(routes({ "PUT /api/v1/admin/rules": () => errorResponse(400, "invalid_selector", "unexpected token ')'", "rules[1].selector") }));
    render(<Page />);
    await userEvent.type(await screen.findByLabelText("Selector 2"), ")");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    const alert = await screen.findByText("unexpected token ')'");
    expect(alert).toBeInTheDocument();
    expect(screen.getByLabelText("Selector 2")).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Selector 1")).not.toHaveAttribute("aria-invalid");
    expect(screen.getByLabelText("Selector 2")).toHaveValue("size=large)");
  });

  it("disables Save while the request is in flight", async () => {
    let release: (r: Response) => void = () => {};
    mockApi(routes({ "PUT /api/v1/admin/rules": () => new Promise<Response>((r) => (release = r)) }));
    // the mock awaits plain values, so a pending promise keeps the request open
    render(<Page />);
    await userEvent.type(await screen.findByLabelText("Selector 1"), "x");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    release(new Response(JSON.stringify(rules), { status: 200 }));
  });
});
