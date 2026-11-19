// SPDX-License-Identifier: MIT
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { setCsrfToken } from "../../api/client";
import { errorResponse, mockApi } from "../../test/mockApi";
import { emptyCollector, setup04, setup07, setupList } from "../admin-shared/fixtures";
import Page from "./index";

const collector = {
  ...emptyCollector,
  orphan_count: 1,
  orphans: [
    {
      resource_name: "setup-09_RP",
      first_seen_at: "2026-11-16T08:10:00Z",
      last_seen_at: "2026-11-18T08:10:00Z",
      setup: { id: 9, name: "setup-09" },
      labels: {},
    },
  ],
};

function routes(extra: Record<string, unknown> = {}) {
  return {
    "GET /api/v1/setups": setupList([setup04, setup07]),
    "GET /api/v1/admin/collector": collector,
    ...extra,
  };
}

beforeEach(() => setCsrfToken("csrf-admin"));

describe("SET-1 setup inventory states", () => {
  it("shows loading then rows, status pills, override suffix and orphans", async () => {
    mockApi(routes());
    render(<Page />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading setups");
    const table = await screen.findByRole("table", { name: "Setups" });
    const rows = within(table).getAllByRole("row");
    expect(rows[1]).toHaveTextContent("size=medium");
    expect(rows[1]).toHaveTextContent("setup-04_RP");
    expect(rows[1]).toHaveTextContent("Sam Lee");
    expect(rows[1]).toHaveTextContent("Enabled");
    expect(rows[2]).toHaveTextContent("lab7-pool (override)");
    expect(rows[2]).toHaveTextContent("Not deployed");
    expect(rows[2]).toHaveTextContent("Disabled");
    expect(await screen.findByText("setup-09 exists but nobody holds it")).toBeInTheDocument();
  });

  it("shows the empty state", async () => {
    mockApi(routes({ "GET /api/v1/setups": setupList([]), "GET /api/v1/admin/collector": emptyCollector }));
    render(<Page />);
    expect(await screen.findByText("No setups yet. Add one to start.")).toBeInTheDocument();
    expect(await screen.findByText("No orphans.")).toBeInTheDocument();
  });

  it("shows the API error", async () => {
    mockApi(routes({ "GET /api/v1/setups": () => errorResponse(500, "internal", "boom") }));
    render(<Page />);
    expect(await screen.findByText("boom")).toBeInTheDocument();
  });
});

describe("SET-2, SET-3 add and edit", () => {
  it("adds a setup with labels and parsed JSON details", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/admin/setups": setup04 }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Add setup" }));
    const panel = screen.getByRole("complementary", { name: "Add setup" });
    await userEvent.type(within(panel).getByLabelText("Name"), "lab-02");
    await userEvent.click(within(panel).getByRole("button", { name: "Add label" }));
    await userEvent.type(within(panel).getByLabelText("Label key 1"), "tier");
    await userEvent.type(within(panel).getByLabelText("Label value 1"), "a");
    const details = within(panel).getByLabelText(/Details/);
    await userEvent.clear(details);
    await userEvent.click(details);
    await userEvent.paste('{"rack":"r2"}');
    await userEvent.click(within(panel).getByRole("button", { name: "Save" }));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/admin/setups");
    expect(post.body).toEqual({
      name: "lab-02",
      labels: { tier: "a" },
      details: { rack: "r2" },
      enabled: true,
      resource_name_override: null,
    });
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-admin");
  });

  it("blocks saving invalid JSON and pretty-prints valid JSON", async () => {
    const { calls } = mockApi(routes());
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit setup-04" }));
    const panel = screen.getByRole("complementary", { name: "Edit setup" });
    const details = within(panel).getByLabelText(/Details/);
    await userEvent.clear(details);
    await userEvent.click(details);
    await userEvent.paste("{bad");
    expect(within(panel).getByText(/Invalid JSON/)).toBeInTheDocument();
    expect(within(panel).getByRole("button", { name: "Save" })).toBeDisabled();
    await userEvent.clear(details);
    await userEvent.click(details);
    await userEvent.paste('{"a":1}');
    expect(within(panel).getByText("Valid JSON")).toBeInTheDocument();
    await userEvent.click(within(panel).getByRole("button", { name: "Pretty-print" }));
    expect(details).toHaveValue('{\n  "a": 1\n}');
    expect(calls.some((c) => c.method === "PUT")).toBe(false);
  });

  it("surfaces the SET-3 label limit error from the API", async () => {
    mockApi(routes({ "PUT /api/v1/admin/setups/4": () => errorResponse(400, "validation", "label value longer than 63 characters", "labels.size") }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit setup-04" }));
    await userEvent.click(within(screen.getByRole("complementary", { name: "Edit setup" })).getByRole("button", { name: "Save" }));
    expect(await screen.findByText("label value longer than 63 characters (labels.size)")).toBeInTheDocument();
  });

  it("edit sends the full replacement with PUT", async () => {
    const { calls } = mockApi(routes({ "PUT /api/v1/admin/setups/4": setup04 }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit setup-04" }));
    const panel = screen.getByRole("complementary", { name: "Edit setup" });
    await userEvent.type(within(panel).getByLabelText("Resource name override"), "pool-4");
    await userEvent.click(within(panel).getByRole("button", { name: "Save" }));
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.body).toEqual({
      name: "setup-04",
      labels: { size: "medium", site: "sjc" },
      details: { rack: "r1" },
      enabled: true,
      resource_name_override: "pool-4",
    });
  });
});

describe("SET-5 disable, delete, force-release", () => {
  it("disables an enabled setup through PUT", async () => {
    const { calls } = mockApi(routes({ "PUT /api/v1/admin/setups/4": setup04 }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Disable setup-04" }));
    expect(calls.find((c) => c.method === "PUT")!.body).toMatchObject({ enabled: false, name: "setup-04" });
  });

  it("shows has_active_hold when disabling a held setup is refused", async () => {
    mockApi(routes({ "PUT /api/v1/admin/setups/4": () => errorResponse(409, "has_active_hold", "Setup is held.") }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Disable setup-04" }));
    expect(await screen.findByText("Setup is held.")).toBeInTheDocument();
  });

  it("deletes after confirmation", async () => {
    const { calls } = mockApi(routes({ "DELETE /api/v1/admin/setups/7": new Response(null, { status: 204 }) }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit setup-07" }));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete setup" }));
    const del = calls.find((c) => c.method === "DELETE")!;
    expect(del.url).toBe("/api/v1/admin/setups/7");
    expect(del.headers.get("X-CSRF-Token")).toBe("csrf-admin");
  });

  it("shows the API refusal when deleting a held setup", async () => {
    mockApi(routes({ "DELETE /api/v1/admin/setups/4": () => errorResponse(409, "has_active_hold", "Cannot delete a held setup.") }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit setup-04" }));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete setup" }));
    expect(await screen.findByText("Cannot delete a held setup.")).toBeInTheDocument();
  });

  it("force-releases a held setup with a required reason", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/holds/77/release": {} }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit setup-04" }));
    await userEvent.click(screen.getByRole("button", { name: "Force-release" }));
    const dialog = screen.getByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Force-release" }));
    expect(within(dialog).getByText("A reason is required.")).toBeInTheDocument();
    expect(calls.some((c) => c.method === "POST")).toBe(false);
    await userEvent.type(within(dialog).getByLabelText("Reason (required)"), "owner left");
    await userEvent.click(within(dialog).getByRole("button", { name: "Force-release" }));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/holds/77/release");
    expect(post.body).toEqual({ force: true, reason: "owner left" });
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-admin");
  });

  it("offers no force-release for a free setup", async () => {
    mockApi(routes());
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit setup-07" }));
    expect(screen.queryByRole("button", { name: "Force-release" })).toBeNull();
  });
});
