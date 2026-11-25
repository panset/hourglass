// SPDX-License-Identifier: MIT
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { setCsrfToken } from "../../api/client";
import { errorResponse, mockApi } from "../../test/mockApi";
import Page from "./index";
import type { ServiceAccount } from "./api";

const TOKEN = "hgsa_3_" + "ab".repeat(32);

const reporter: ServiceAccount = {
  id: 1,
  name: "lab-agent",
  description: "Reports pressure",
  scopes: ["pressure:write", "state:read"],
  enabled: true,
  created_at: "2026-11-10T09:00:00Z",
  tokens: [
    { id: 1, created_at: "2026-11-10T09:05:00Z", expires_at: null, last_used_at: "2026-11-18T17:00:00Z" },
    { id: 2, created_at: "2026-11-12T09:05:00Z", expires_at: "2027-01-01T00:00:00Z", last_used_at: null },
  ],
};
const idle: ServiceAccount = { ...reporter, id: 2, name: "idle-svc", description: "", scopes: [], enabled: false, tokens: [] };

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

function routes(extra: Record<string, unknown> = {}) {
  return { "GET /api/v1/admin/service-accounts": { service_accounts: [reporter, idle] }, ...extra };
}

beforeEach(() => setCsrfToken("csrf-admin"));

describe("SVC-6 service accounts list", () => {
  it("shows loading, then accounts with scopes, enabled flag and each token's times", async () => {
    mockApi(routes());
    render(<Page />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading service accounts");
    const card = await screen.findByRole("listitem", { name: "Service account lab-agent" });
    expect(within(card).getByText("Enabled")).toBeInTheDocument();
    expect(within(card).getByText("pressure:write")).toBeInTheDocument();
    expect(within(card).getByText("state:read")).toBeInTheDocument();
    const rows = within(within(card).getByRole("table", { name: "Tokens of lab-agent" })).getAllByRole("row");
    expect(rows).toHaveLength(3);
    expect(rows[1]).toHaveTextContent("Never"); // no expiry
    expect(rows[1]!.querySelector("time[datetime='2026-11-18T17:00:00Z']")).not.toBeNull();
    expect(rows[2]).toHaveTextContent("Never"); // not used yet
    expect(rows[2]!.querySelector("time[datetime='2027-01-01T00:00:00Z']")).not.toBeNull();
    const other = screen.getByRole("listitem", { name: "Service account idle-svc" });
    expect(within(other).getByText("Disabled")).toBeInTheDocument();
    expect(within(other).getByText("No tokens.")).toBeInTheDocument();
  });

  it("shows an empty state and the API error", async () => {
    mockApi({ "GET /api/v1/admin/service-accounts": { service_accounts: [] } });
    const { unmount } = render(<Page />);
    expect(await screen.findByText("No service accounts yet.")).toBeInTheDocument();
    unmount();
    mockApi({ "GET /api/v1/admin/service-accounts": () => errorResponse(500, "internal", "database is down") });
    render(<Page />);
    expect(await screen.findByText("database is down")).toBeInTheDocument();
  });
});

describe("SVC-6 create, edit, delete", () => {
  it("creates an account with the checked scopes and refetches", async () => {
    const { calls } = mockApi(
      routes({ "POST /api/v1/admin/service-accounts": () => json({ ...idle, id: 3, name: "new-svc", enabled: true }, 201) }),
    );
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Create service account" }));
    const dialog = screen.getByRole("dialog", { name: "Create service account" });
    expect(within(dialog).getAllByRole("checkbox")).toHaveLength(5); // UI-14: weights:write is the fifth scope (design D3)
    await userEvent.type(within(dialog).getByLabelText("Name"), "new-svc");
    await userEvent.type(within(dialog).getByLabelText("Description"), "for tests");
    await userEvent.click(within(dialog).getByRole("checkbox", { name: /holds:release/ }));
    await userEvent.click(within(dialog).getByRole("checkbox", { name: /state:read/ }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Create account" }));
    expect(await screen.findByText("Created new-svc.")).toBeInTheDocument();
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.body).toEqual({ name: "new-svc", description: "for tests", scopes: ["state:read", "holds:release"] });
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-admin");
    expect(calls.filter((c) => c.url === "/api/v1/admin/service-accounts" && c.method === "GET").length).toBe(2);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("shows the API's name error in the dialog and keeps it open", async () => {
    mockApi(routes({ "POST /api/v1/admin/service-accounts": () => errorResponse(409, "conflict", "name already in use", "name") }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Create service account" }));
    const dialog = screen.getByRole("dialog");
    await userEvent.type(within(dialog).getByLabelText("Name"), "lab-agent");
    await userEvent.click(within(dialog).getByRole("button", { name: "Create account" }));
    expect(await within(dialog).findByText("name already in use")).toBeInTheDocument();
  });

  it("edits description, scopes and enabled with a full replacement", async () => {
    const { calls } = mockApi(routes({ "PUT /api/v1/admin/service-accounts/1": () => json({ ...reporter, enabled: false }) }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit lab-agent" }));
    const dialog = screen.getByRole("dialog", { name: "Edit lab-agent" });
    expect(within(dialog).queryByLabelText("Name")).toBeNull();
    const desc = within(dialog).getByLabelText("Description");
    await userEvent.clear(desc);
    await userEvent.type(desc, "Reports and reads");
    await userEvent.click(within(dialog).getByRole("checkbox", { name: /deployments:write/ }));
    await userEvent.click(within(dialog).getByRole("checkbox", { name: "Enabled" }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Save changes" }));
    expect(await screen.findByText("Saved lab-agent.")).toBeInTheDocument();
    expect(calls.find((c) => c.method === "PUT")!.body).toEqual({
      description: "Reports and reads",
      scopes: ["state:read", "pressure:write", "deployments:write"],
      enabled: false,
    });
  });

  it("deletes only after confirmation", async () => {
    const { calls } = mockApi(routes({ "DELETE /api/v1/admin/service-accounts/2": () => new Response(null, { status: 204 }) }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Delete idle-svc" }));
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    await userEvent.click(within(screen.getByRole("dialog", { name: "Delete idle-svc?" })).getByRole("button", { name: "Cancel" }));
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    await userEvent.click(screen.getByRole("button", { name: "Delete idle-svc" }));
    await userEvent.click(screen.getByRole("button", { name: "Delete account" }));
    expect(await screen.findByText("Deleted idle-svc.")).toBeInTheDocument();
    expect(calls.find((c) => c.method === "DELETE")!.url).toBe("/api/v1/admin/service-accounts/2");
  });
});

describe("A2.7 dialogs ignore a backdrop click", () => {
  it("A2.7-9 Create service account keeps the typed name; Escape closes it", async () => {
    mockApi(routes());
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Create service account" }));
    const dialog = screen.getByRole("dialog", { name: "Create service account" });
    await userEvent.type(within(dialog).getByLabelText(/^Name/), "billing");
    await userEvent.click(dialog.parentElement!);
    expect(screen.getByRole("dialog", { name: "Create service account" })).toBeInTheDocument();
    expect(within(dialog).getByLabelText(/^Name/)).toHaveValue("billing");
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("Edit service account and Issue token ignore a backdrop click", async () => {
    mockApi(routes());
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit lab-agent" }));
    await userEvent.click(screen.getByRole("dialog", { name: "Edit lab-agent" }).parentElement!);
    expect(screen.getByRole("dialog", { name: "Edit lab-agent" })).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    await userEvent.click(screen.getByRole("button", { name: "Issue token for lab-agent" }));
    await userEvent.click(screen.getByRole("dialog", { name: "Issue token for lab-agent" }).parentElement!);
    expect(screen.getByRole("dialog", { name: "Issue token for lab-agent" })).toBeInTheDocument();
  });

  it("A2.7-10 the Token shown once dialog does not close on a backdrop click and keeps the token", async () => {
    mockApi(
      routes({
        "POST /api/v1/admin/service-accounts/1/tokens": () =>
          json({ id: 3, created_at: "2026-11-20T10:00:00Z", expires_at: null, last_used_at: null, token: TOKEN }, 201),
      }),
    );
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Issue token for lab-agent" }));
    await userEvent.click(screen.getByRole("button", { name: "Issue token" }));
    const dialog = await screen.findByRole("dialog", { name: "Token for lab-agent" });
    await userEvent.click(dialog.parentElement!);
    expect(screen.getByRole("dialog", { name: "Token for lab-agent" })).toBeInTheDocument();
    expect(within(dialog).getByLabelText("Token")).toHaveValue(TOKEN);
  });

  it("a delete confirmation (no field) closes on a backdrop click and deletes nothing", async () => {
    const { calls } = mockApi(routes());
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Delete idle-svc" }));
    await userEvent.click(screen.getByRole("dialog", { name: "Delete idle-svc?" }).parentElement!);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
  });
});

describe("SVC-4 SVC-6 tokens", () => {
  const issueRoute = () => ({
    "POST /api/v1/admin/service-accounts/1/tokens": () =>
      json({ id: 3, created_at: "2026-11-20T10:00:00Z", expires_at: null, last_used_at: null, token: TOKEN }, 201),
  });

  it("SVC-4 shows the issued token once with a copy button and a warning, and not after closing", async () => {
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    const { calls } = mockApi(routes(issueRoute()));
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Issue token for lab-agent" }));
    await userEvent.click(screen.getByRole("button", { name: "Issue token" }));

    const dialog = await screen.findByRole("dialog", { name: "Token for lab-agent" });
    expect(within(dialog).getByLabelText("Token")).toHaveValue(TOKEN);
    expect(within(dialog).getByText(/only time the token is shown/)).toBeInTheDocument();
    expect(calls.find((c) => c.method === "POST")!.body).toEqual({});
    await userEvent.click(within(dialog).getByRole("button", { name: "Copy token" }));
    expect(writeText).toHaveBeenCalledWith(TOKEN);
    expect(await within(dialog).findByText("Copied.")).toBeInTheDocument();

    await userEvent.click(within(dialog).getAllByRole("button", { name: "Close" }).at(-1)!);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.body.textContent).not.toContain(TOKEN);
    expect(document.body.innerHTML).not.toContain(TOKEN);
    expect(window.location.href).not.toContain(TOKEN);
    expect(setItem).not.toHaveBeenCalled();

    // Opening the dialog again starts at the form, not at the old token.
    await userEvent.click(screen.getByRole("button", { name: "Issue token for lab-agent" }));
    expect(screen.queryByLabelText("Token")).toBeNull();
    expect(document.body.innerHTML).not.toContain(TOKEN);
    setItem.mockRestore();
  });

  it("sends the optional expiry as a UTC time and refuses a past one locally", async () => {
    const { calls } = mockApi(routes(issueRoute()));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Issue token for lab-agent" }));
    const field = screen.getByLabelText("Expires (optional)");
    await userEvent.type(field, "2020-01-01T00:00");
    await userEvent.click(screen.getByRole("button", { name: "Issue token" }));
    expect(await screen.findByText("The expiry must be in the future.")).toBeInTheDocument();
    expect(calls.some((c) => c.method === "POST")).toBe(false);
    await userEvent.clear(field);
    await userEvent.type(field, "2099-06-01T12:30");
    await userEvent.click(screen.getByRole("button", { name: "Issue token" }));
    await screen.findByRole("dialog", { name: "Token for lab-agent" });
    const body = calls.find((c) => c.method === "POST")!.body as { expires_at: string };
    expect(new Date(body.expires_at).getTime()).toBe(new Date("2099-06-01T12:30").getTime());
  });

  it("shows the API error when issuing fails, with no token", async () => {
    mockApi(routes({ "POST /api/v1/admin/service-accounts/1/tokens": () => errorResponse(404, "not_found", "no such account") }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Issue token for lab-agent" }));
    await userEvent.click(screen.getByRole("button", { name: "Issue token" }));
    expect(await screen.findByText("no such account")).toBeInTheDocument();
    expect(screen.queryByLabelText("Token")).toBeNull();
  });

  it("revokes a token after confirmation", async () => {
    const { calls } = mockApi(routes({ "DELETE /api/v1/admin/service-accounts/1/tokens/2": () => new Response(null, { status: 204 }) }));
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Revoke token 2 of lab-agent" }));
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    await userEvent.click(screen.getByRole("button", { name: "Revoke token" }));
    expect(await screen.findByText("Revoked token 2 of lab-agent.")).toBeInTheDocument();
    expect(calls.find((c) => c.method === "DELETE")!.url).toBe("/api/v1/admin/service-accounts/1/tokens/2");
  });
});

describe("UI-14 the weights:write scope", () => {
  it("UI-14 the scope list offers weights:write and sends it with the others", async () => {
    const { calls } = mockApi(
      routes({ "POST /api/v1/admin/service-accounts": () => json({ ...idle, id: 3, name: "w-svc", enabled: true }, 201) }),
    );
    render(<Page />);
    await userEvent.click(await screen.findByRole("button", { name: "Create service account" }));
    const dialog = screen.getByRole("dialog", { name: "Create service account" });
    const boxes = within(dialog).getAllByRole("checkbox");
    expect(boxes).toHaveLength(5);
    await userEvent.type(within(dialog).getByLabelText("Name"), "w-svc");
    await userEvent.click(within(dialog).getByRole("checkbox", { name: /weights:write/ }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Create account" }));
    await screen.findByText("Created w-svc.");
    expect(calls.find((c) => c.method === "POST")!.body).toEqual({ name: "w-svc", description: "", scopes: ["weights:write"] });
  });
});
