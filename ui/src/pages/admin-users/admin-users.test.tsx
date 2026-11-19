// SPDX-License-Identifier: MIT
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { App } from "../../App";
import { setCsrfToken } from "../../api/client";
import { adminSession, barRoutes, developerSession, errorResponse, mockApi } from "../../test/mockApi";
import { dana, localAdmin, morgan, priya, sam, userList } from "../admin-shared/fixtures";
import Page from "./index";

const everyone = userList([priya, sam, localAdmin, morgan]);
const pendingOnly = userList([morgan]);

function routes(extra: Record<string, unknown> = {}) {
  return {
    "GET /api/v1/admin/users": ({ url }: { url: string }) => (url.includes("status=pending") ? pendingOnly : everyone),
    ...extra,
  };
}
const renderPage = () => render(<Page />);

beforeEach(() => setCsrfToken("csrf-admin"));

describe("ADM-1 users page states", () => {
  it("shows loading, then the populated page", async () => {
    mockApi(routes());
    renderPage();
    expect(screen.getByRole("status")).toHaveTextContent("Loading users");
    expect(await screen.findByRole("table", { name: "Users" })).toBeInTheDocument();
    const rows = within(screen.getByRole("table", { name: "Users" })).getAllByRole("row");
    // header + Priya (overdrawn first, API order) + Sam + local admin; pending users are not in the table
    expect(rows).toHaveLength(4);
    expect(rows[1]).toHaveTextContent("Priya N.");
    expect(rows[1]).toHaveTextContent("−12");
    expect(rows[1]).toHaveTextContent("312");
    expect(rows[1]).toHaveTextContent("setup-02");
  });

  it("shows an empty state when there are no users and no pending requests", async () => {
    mockApi({ "GET /api/v1/admin/users": userList([]) });
    renderPage();
    expect(await screen.findByText("No pending access requests.")).toBeInTheDocument();
    expect(screen.getByText("No users yet.")).toBeInTheDocument();
  });

  it("shows the API error message", async () => {
    mockApi({ "GET /api/v1/admin/users": () => errorResponse(500, "internal", "database is down") });
    renderPage();
    expect(await screen.findByText("database is down")).toBeInTheDocument();
  });

  it.each([
    ["null figures", dana],
    // what a server before this change sent: the stale cached figures
    ["stale figures", { ...dana, balance_tokens: 200, month_to_date_burn_tokens: 12 }],
  ])("ADM-1 LED-3 a user with no role shows a dash for balance and burn, and no Grant (%s)", async (_, user) => {
    mockApi({ "GET /api/v1/admin/users": userList([priya, user]) });
    renderPage();
    const table = await screen.findByRole("table", { name: "Users" });
    const row = within(table).getByText("Dana R.").closest("tr")!;
    const cells = within(row).getAllByRole("cell");
    const headers = within(table).getAllByRole("columnheader").map((h) => h.textContent);
    expect(cells[headers.indexOf("Tokens left")]).toHaveTextContent(/^—$/);
    expect(cells[headers.indexOf("Burned this month")]).toHaveTextContent(/^—$/);
    expect(within(row).queryByRole("button", { name: "Grant to Dana R." })).toBeNull();
  });

  it("ADM-1 local admin row has no role or balance actions", async () => {
    mockApi(routes());
    renderPage();
    await screen.findByRole("table", { name: "Users" });
    expect(screen.getByText("Admin · fixed")).toBeInTheDocument();
    expect(screen.getByText("No balance")).toBeInTheDocument();
    expect(screen.getByText("Cannot hold setups")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Grant to admin" })).toBeNull();
    expect(screen.queryByRole("group", { name: "Roles for admin" })).toBeNull();
  });
});

describe("ADM-5 pending requests", () => {
  it("lists pending users and approves with the chosen roles, with the CSRF header", async () => {
    const { calls } = mockApi(routes({ "PUT /api/v1/admin/users/20/roles": { ...morgan, roles: ["developer"] } }));
    renderPage();
    expect(await screen.findByText("morgan@example.com")).toBeInTheDocument();
    const approve = screen.getByRole("button", { name: "Approve Morgan Q." });
    expect(approve).toBeDisabled();
    await userEvent.click(within(screen.getByRole("group", { name: "Roles for Morgan Q." })).getByLabelText("Developer"));
    await userEvent.click(approve);
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.url).toBe("/api/v1/admin/users/20/roles");
    expect(put.body).toEqual({ roles: ["developer"] });
    expect(put.headers.get("X-CSRF-Token")).toBe("csrf-admin");
    // refetched immediately after success
    await vi.waitFor(() => expect(calls.filter((c) => c.method === "GET").length).toBeGreaterThan(2));
  });

  it("rejects a pending user", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/admin/users/20/reject": morgan }));
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: "Reject Morgan Q." }));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/admin/users/20/reject");
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-admin");
  });
});

describe("ADM-1 role changes", () => {
  it("saves changed roles with PUT", async () => {
    const { calls } = mockApi(routes({ "PUT /api/v1/admin/users/11/roles": sam }));
    renderPage();
    await screen.findByRole("table", { name: "Users" });
    await userEvent.click(within(screen.getByRole("group", { name: "Roles for Sam Lee" })).getByLabelText("Admin"));
    await userEvent.click(screen.getByRole("button", { name: "Save roles for Sam Lee" }));
    const put = calls.find((c) => c.method === "PUT")!;
    expect(put.body).toEqual({ roles: ["developer", "admin"] });
  });

  it.each([
    ["self_demote", "You cannot remove your own admin role."],
    ["has_active_hold", "User holds a setup."],
  ])("shows the %s refusal as a message", async (code, message) => {
    mockApi(routes({ "PUT /api/v1/admin/users/11/roles": () => errorResponse(409, code, message) }));
    renderPage();
    await screen.findByRole("table", { name: "Users" });
    await userEvent.click(within(screen.getByRole("group", { name: "Roles for Sam Lee" })).getByLabelText("Admin"));
    await userEvent.click(screen.getByRole("button", { name: "Save roles for Sam Lee" }));
    expect(await screen.findByText(message)).toBeInTheDocument();
  });

  it("shows local_admin_protected on approve", async () => {
    mockApi(routes({ "PUT /api/v1/admin/users/20/roles": () => errorResponse(409, "local_admin_protected", "The local admin cannot be changed.") }));
    renderPage();
    await userEvent.click(within(await screen.findByRole("group", { name: "Roles for Morgan Q." })).getByLabelText("Admin"));
    await userEvent.click(screen.getByRole("button", { name: "Approve Morgan Q." }));
    expect(await screen.findByText("The local admin cannot be changed.")).toBeInTheDocument();
  });
});

describe("LED-4 grant and LED-5 reset", () => {
  async function openGrant() {
    await screen.findByRole("table", { name: "Users" });
    await userEvent.click(screen.getByRole("button", { name: "Grant to Priya N." }));
    return screen.getByRole("complementary", { name: "Grant tokens" });
  }

  it("moves focus into the Grant panel, traps Tab, and Escape closes it and returns focus to the button", async () => {
    mockApi(routes());
    renderPage();
    await screen.findByRole("table", { name: "Users" });
    const opener = screen.getByRole("button", { name: "Grant to Priya N." });
    opener.focus();
    await userEvent.click(opener);
    const panel = screen.getByRole("complementary", { name: "Grant tokens" });
    expect(panel).toContainElement(document.activeElement as HTMLElement);
    for (let i = 0; i < 12; i++) {
      await userEvent.tab();
      expect(panel).toContainElement(document.activeElement as HTMLElement);
    }
    await userEvent.tab({ shift: true });
    expect(panel).toContainElement(document.activeElement as HTMLElement);
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("complementary", { name: "Grant tokens" })).toBeNull();
    expect(opener).toHaveFocus();
  });

  it("requires a reason and a non-zero amount before calling the API", async () => {
    const { calls } = mockApi(routes());
    renderPage();
    const panel = await openGrant();
    await userEvent.click(within(panel).getByRole("button", { name: "Grant tokens" }));
    expect(within(panel).getByText("Enter a non-zero number.")).toBeInTheDocument();
    expect(within(panel).getByText("A reason is required.")).toBeInTheDocument();
    expect(calls.some((c) => c.method === "POST")).toBe(false);
  });

  it("grants a negative amount with a reason", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/admin/users/10/grants": {} }));
    renderPage();
    const panel = await openGrant();
    await userEvent.type(within(panel).getByLabelText("Amount"), "-25");
    await userEvent.type(within(panel).getByLabelText("Reason (required)"), "fix mistake");
    await userEvent.click(within(panel).getByRole("button", { name: "Grant tokens" }));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/admin/users/10/grants");
    expect(post.body).toEqual({ amount_tokens: -25, reason: "fix mistake" });
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-admin");
    expect(await screen.findByText(/Granted −25 tokens to Priya N\./)).toBeInTheDocument();
  });

  it("shows an API error and re-enables the button", async () => {
    mockApi(routes({ "POST /api/v1/admin/users/10/grants": () => errorResponse(400, "validation", "reason is too long", "reason") }));
    renderPage();
    const panel = await openGrant();
    await userEvent.type(within(panel).getByLabelText("Amount"), "5");
    await userEvent.type(within(panel).getByLabelText("Reason (required)"), "x");
    await userEvent.click(within(panel).getByRole("button", { name: "Grant tokens" }));
    expect(await within(panel).findByText("reason is too long (reason)")).toBeInTheDocument();
    expect(within(panel).getByRole("button", { name: "Grant tokens" })).toBeEnabled();
  });

  it("resets the balance after confirmation, with the default allowance from the API", async () => {
    const { calls } = mockApi(routes({ "POST /api/v1/admin/users/10/reset": {} }));
    renderPage();
    const panel = await openGrant();
    await userEvent.click(within(panel).getByRole("button", { name: "Reset to 300" }));
    expect(within(panel).getByText("A reason is required.")).toBeInTheDocument();
    await userEvent.type(within(panel).getByLabelText("Reason (required)"), "new month");
    await userEvent.click(within(panel).getByRole("button", { name: "Reset to 300" }));
    const dialog = screen.getByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Reset balance" }));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/admin/users/10/reset");
    expect(post.body).toEqual({ reason: "new month" });
  });
});

describe("UI-5 role gating", () => {
  it("a developer sees not-found on /admin/users and no admin call is made", async () => {
    const { calls } = mockApi({ "GET /api/v1/session": developerSession, "GET /api/v1/me": { balance_tokens: 1 }, "GET /api/v1/pressure": barRoutes["GET /api/v1/pressure"] });
    render(
      <MemoryRouter initialEntries={["/admin/users"]}>
        <App />
      </MemoryRouter>,
    );
    await vi.waitFor(() => expect(calls.some((c) => c.url.includes("/api/v1/session"))).toBe(true));
    expect(await screen.findByText(/not found/i)).toBeInTheDocument();
    expect(calls.some((c) => c.url.includes("/admin/users?status=pending") || c.url === "/api/v1/admin/users")).toBe(false);
  });

  it("an admin reaches the page through the router", async () => {
    mockApi({ "GET /api/v1/session": adminSession, ...barRoutes, ...routes() });
    render(
      <MemoryRouter initialEntries={["/admin/users"]}>
        <App />
      </MemoryRouter>,
    );
    expect(await screen.findByRole("table", { name: "Users" })).toBeInTheDocument();
  });
});
