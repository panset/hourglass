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

  it("INT-7 shows when the last pressure and deployment reports arrived, and who sent them", async () => {
    mockApi(
      routes({
        "GET /api/v1/admin/collector": {
          ...collector,
          last_pressure_report: { observed_at: "2026-11-18T17:20:00Z", received_at: "2026-11-18T17:21:00Z", service_account: "lab-agent" },
          last_deployments_report: { observed_at: "2026-11-18T17:10:00Z", received_at: "2026-11-18T17:11:00Z", service_account: "lab-sync" },
        },
      }),
    );
    render(<Page />);
    const pressure = (await screen.findByText("Last pressure report")).closest("div")!;
    expect(pressure).toHaveTextContent("received from lab-agent");
    expect(pressure.querySelector("time[datetime='2026-11-18T17:21:00Z']")).not.toBeNull();
    const deployments = screen.getByText("Last deployment report").closest("div")!;
    expect(deployments).toHaveTextContent("received from lab-sync");
    expect(deployments.querySelector("time[datetime='2026-11-18T17:11:00Z']")).not.toBeNull();
  });

  it("INT-7 shows None received when a report is null (none yet, or the extension is not reported)", async () => {
    mockApi(routes({ "GET /api/v1/admin/collector": emptyCollector }));
    render(<Page />);
    const pressure = (await screen.findByText("Last pressure report")).closest("div")!;
    expect(pressure).toHaveTextContent("None received");
    expect(screen.getByText("Last deployment report").closest("div")).toHaveTextContent("None received");
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

describe("UI-14 Reported weights section", () => {
  const rw = (over: Partial<NonNullable<SettingsResponse["settings"]["reported_weights"]>> = {}) => ({
    ...settings,
    settings: { ...settings.settings, reported_weights: { enabled: false, min: 0, max: null, max_age: "10m", ...over } },
  });
  const section = async () => within(await screen.findByRole("region", { name: "Reported weights" }));

  it("UI-14 shows the switch, minimum, maximum and maximum age, and the advice about the poll interval", async () => {
    mockApi(routes({ "GET /api/v1/admin/settings": rw({ enabled: true, min: 0.5, max: 20, max_age: "15m" }) }));
    render(<Page />);
    const s = await section();
    expect(s.getByRole("checkbox", { name: /Accept weights reported/ })).toBeChecked();
    expect(s.getByLabelText("Minimum weight")).toHaveValue("0.5");
    expect(s.getByLabelText("Maximum weight")).toHaveValue("20");
    expect(s.getByLabelText("Maximum age")).toHaveValue("15m");
    expect(s.getByText(/maximum age should be at least twice the poll interval/)).toBeInTheDocument();
  });

  it("UI-14 shows the defaults when the server sends no key", async () => {
    mockApi(routes());
    render(<Page />);
    const s = await section();
    expect(s.getByLabelText("Maximum weight")).toHaveValue("");
    expect(s.getByLabelText("Maximum age")).toHaveValue("10m");
  });

  it("UI-14 saving a change sends all four fields; blank maximum is null", async () => {
    const { calls } = mockApi(routes({ "GET /api/v1/admin/settings": rw({ max: 20 }), "PUT /api/v1/admin/settings": rw({ max: 20 }) }));
    render(<Page />);
    const s = await section();
    await userEvent.click(s.getByRole("checkbox", { name: /Accept weights reported/ }));
    await userEvent.clear(s.getByLabelText("Maximum age"));
    await userEvent.type(s.getByLabelText("Maximum age"), "20m");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.body).toEqual({ ...settings.settings, reported_weights: { enabled: true, min: 0, max: 20, max_age: "20m" } });
    // A blank maximum is sent as null, with the switch off, so the server accepts it.
    await userEvent.clear(await s.findByLabelText("Maximum weight"));
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    const puts = calls.filter((c) => c.method === "PUT");
    expect(puts).toHaveLength(2);
    expect((puts[1]!.body as { reported_weights: unknown }).reported_weights).toEqual({ enabled: false, min: 0, max: null, max_age: "10m" });
  });

  it("UI-14 a save that changes only other settings does not send reported_weights (the key stays as stored)", async () => {
    const { calls } = mockApi(routes({ "GET /api/v1/admin/settings": rw({ enabled: true, max: 20 }), "PUT /api/v1/admin/settings": rw({ enabled: true, max: 20 }) }));
    render(<Page />);
    await userEvent.clear(await screen.findByLabelText("Max setups per developer"));
    await userEvent.type(screen.getByLabelText("Max setups per developer"), "3");
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    const body = calls.find((c) => c.method === "PUT")!.body as Record<string, unknown>;
    expect(body).not.toHaveProperty("reported_weights");
    expect(body.max_holds_per_user).toBe(3);
  });

  it("UI-14 turning the switch on without a maximum shows the server's validation message", async () => {
    const msg = "reported_weights.max is required when reported weights are enabled";
    mockApi(routes({ "PUT /api/v1/admin/settings": () => errorResponse(400, "validation", msg, "reported_weights.max") }));
    render(<Page />);
    const s = await section();
    await userEvent.click(s.getByRole("checkbox", { name: /Accept weights reported/ }));
    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(await s.findByText(msg)).toBeInTheDocument();
  });

  it("UI-14 shows when the last weights report arrived and from which account, or that none did", async () => {
    mockApi(
      routes({
        "GET /api/v1/admin/settings": rw({ enabled: true, max: 20 }),
        "GET /api/v1/admin/collector": {
          ...collector,
          last_weights_report: { observed_at: "2026-11-18T17:20:00Z", received_at: "2026-11-18T17:21:00Z", service_account: "lab-weights", entries: 9, out_of_bounds: 0 },
        },
      }),
    );
    render(<Page />);
    const stat = (await (await section()).findByText("Last weights report")).closest("div")!;
    expect(stat).toHaveTextContent("received from lab-weights");
    expect(stat).toHaveTextContent("9 weights");
    expect(stat.querySelector("time[datetime='2026-11-18T17:21:00Z']")).not.toBeNull();
  });

  it("UI-14 says None received, with no warnings, when nothing has arrived", async () => {
    mockApi(routes());
    render(<Page />);
    const s = await section();
    expect((await s.findByText("Last weights report")).closest("div")).toHaveTextContent("None received");
    expect(s.queryByText(/outside the minimum and maximum and not applied/)).toBeNull();
    expect(s.queryByText(/was refused/)).toBeNull();
  });

  it("UI-14 warns with the count when stored weights are outside the limits", async () => {
    mockApi(
      routes({
        "GET /api/v1/admin/collector": {
          ...collector,
          last_weights_report: { observed_at: "2026-11-18T17:20:00Z", received_at: "2026-11-18T17:21:00Z", service_account: "lab-weights", entries: 9, out_of_bounds: 3 },
        },
      }),
    );
    render(<Page />);
    expect(await (await section()).findByText(/3 stored weights are outside the minimum and maximum/)).toBeInTheDocument();
  });

  it("UI-14 warns with the time, account and message of a refusal after the last accepted report", async () => {
    mockApi(
      routes({
        "GET /api/v1/admin/collector": {
          ...collector,
          last_weights_report: { observed_at: "2026-11-18T17:20:00Z", received_at: "2026-11-18T17:21:00Z", service_account: "lab-weights", entries: 9, out_of_bounds: 0 },
          last_weights_refusal: { at: "2026-11-18T17:30:00Z", service_account: "lab-weights", message: 'weights[3] (setup "lab-03"): weight 25 is above the maximum 20' },
        },
      }),
    );
    render(<Page />);
    const alert = (await (await section()).findByText(/was refused after the last accepted one/)).closest("div")!;
    expect(alert).toHaveTextContent("lab-weights");
    expect(alert).toHaveTextContent('weights[3] (setup "lab-03"): weight 25 is above the maximum 20');
    expect(alert.querySelector("time[datetime='2026-11-18T17:30:00Z']")).not.toBeNull();
  });

  it("UI-14 does not show a refusal that is older than the last accepted report", async () => {
    mockApi(
      routes({
        "GET /api/v1/admin/collector": {
          ...collector,
          last_weights_report: { observed_at: "2026-11-18T17:20:00Z", received_at: "2026-11-18T17:21:00Z", service_account: "lab-weights", entries: 9, out_of_bounds: 0 },
          last_weights_refusal: { at: "2026-11-18T17:10:00Z", service_account: "lab-weights", message: "too old to matter" },
        },
      }),
    );
    render(<Page />);
    await (await section()).findByText("Last weights report");
    expect(screen.queryByText(/too old to matter/)).toBeNull();
  });
});
