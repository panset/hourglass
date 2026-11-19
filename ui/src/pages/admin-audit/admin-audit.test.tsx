// SPDX-License-Identifier: MIT
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { errorResponse, mockApi } from "../../test/mockApi";
import { priya, sam, userList, type AuditEntry, type AuditList } from "../admin-shared/fixtures";
import Page from "./index";

const entry = (id: number, over: Partial<AuditEntry> = {}): AuditEntry => ({
  id,
  actor: { id: 3, name: "Riley Admin" },
  action: "grant.create",
  target: "user:10",
  before: { balance: -12 },
  after: { balance: 88 },
  created_at: "2026-11-18T09:14:00Z",
  ...over,
});
const list = (entries: AuditEntry[], total = entries.length, offset = 0): AuditList => ({ entries, total, limit: 50, offset });

function routes(audit: unknown) {
  return { "GET /api/v1/admin/users": userList([priya, sam]), "GET /api/v1/admin/audit": audit };
}

describe("ADM-3 audit log page", () => {
  it("shows loading then entries with time, actor, action and target", async () => {
    mockApi(routes(list([entry(1), entry(2, { action: "setup.delete", target: "setup:4" })])));
    render(<Page />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading audit log");
    const rows = within(await screen.findByRole("table", { name: "Audit log" })).getAllByRole("row");
    expect(rows).toHaveLength(3);
    expect(rows[1]).toHaveTextContent("Riley Admin");
    expect(rows[1]).toHaveTextContent("grant.create");
    expect(rows[1]).toHaveTextContent("user:10");
    expect(rows[2]).toHaveTextContent("setup:4");
  });

  it("shows the empty state", async () => {
    mockApi(routes(list([])));
    render(<Page />);
    expect(await screen.findByText("No audit entries match.")).toBeInTheDocument();
  });

  it("shows the API error", async () => {
    mockApi(routes(() => errorResponse(500, "internal", "audit store failed")));
    render(<Page />);
    expect(await screen.findByText("audit store failed")).toBeInTheDocument();
  });

  it("expands the before and after JSON", async () => {
    mockApi(routes(list([entry(1)])));
    render(<Page />);
    const btn = await screen.findByRole("button", { name: "Show before and after for entry 1" });
    expect(btn).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(btn);
    expect(screen.getByText(/"balance": -12/)).toBeInTheDocument();
    expect(screen.getByText(/"balance": 88/)).toBeInTheDocument();
  });

  it("filters by actor and action through query parameters", async () => {
    const { calls } = mockApi(routes(list([entry(1)])));
    render(<Page />);
    await screen.findByRole("table", { name: "Audit log" });
    await userEvent.selectOptions(await screen.findByLabelText("Actor"), "Priya N.");
    await userEvent.type(screen.getByLabelText(/^Action/), "grant.create");
    await userEvent.click(screen.getByRole("button", { name: "Apply filters" }));
    await vi.waitFor(() => {
      const last = calls.filter((c) => c.url.includes("/admin/audit")).at(-1)!;
      expect(last.url).toContain("actor=10");
      expect(last.url).toContain("action=grant.create");
      expect(last.url).toContain("offset=0");
    });
  });

  it("pages with limit and offset", async () => {
    const { calls } = mockApi(routes(({ url }: { url: string }) => (url.includes("offset=50") ? list([entry(51)], 51, 50) : list([entry(1)], 51, 0))));
    render(<Page />);
    await screen.findByRole("table", { name: "Audit log" });
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Next" }));
    await vi.waitFor(() => expect(calls.filter((c) => c.url.includes("/admin/audit")).at(-1)!.url).toContain("limit=50&offset=50"));
    expect(await screen.findByText(/51 to 51 of 51/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
  });
});
