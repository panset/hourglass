// SPDX-License-Identifier: MIT
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { setCsrfToken } from "../../api/client";
import { adminSession, errorResponse, mockApi } from "../../test/mockApi";
import { localAdmin, userList } from "../admin-shared/fixtures";
import type { Preapproval } from "./api";
import Page from "./index";
import { splitEmails } from "./preapprovals";


// These tests render the page without the session provider; the signed-in admin is the module-level `viewer`.
const viewer = vi.hoisted(() => ({ session: undefined as unknown }));
vi.mock("../../shell/session", async (orig) => {
  const actual = await orig<typeof import("../../shell/session")>();
  return {
    ...actual,
    useSession: () => {
      try {
        return actual.useSession();
      } catch {
        return viewer.session as ReturnType<typeof actual.useSession>;
      }
    },
  };
});
beforeEach(() => {
  viewer.session = adminSession;
});

const riley = { id: 1, name: "Riley Admin" };
const dbRow: Preapproval = {
  id: 7,
  email: "alice@example.com",
  roles: ["developer"],
  source: "database",
  created_at: "2026-11-10T09:00:00.000Z",
  created_by: riley,
  used_at: null,
  used_by: null,
};
const usedRow: Preapproval = {
  ...dbRow,
  id: 8,
  email: "bob@example.com",
  roles: ["admin"],
  used_at: "2026-11-12T10:30:00.000Z",
  used_by: { id: 30, name: "Bob B." },
};
const fileRow: Preapproval = {
  id: null,
  email: "carol@example.com",
  roles: ["developer", "admin"],
  source: "config",
  created_at: null,
  created_by: null,
  used_at: null,
  used_by: null,
};

function routes(preapprovals: Preapproval[], extra: Record<string, unknown> = {}) {
  return {
    "GET /api/v1/admin/users": userList([localAdmin]),
    "GET /api/v1/admin/preapprovals": () => ({ preapprovals }),
    ...extra,
  };
}
async function table() {
  return within(await screen.findByRole("table", { name: "Pre-approved users" }));
}
const textarea = () => screen.getByRole("textbox", { name: "Emails to pre-approve" });

beforeEach(() => setCsrfToken("csrf-admin"));

describe("UI-11 pre-approved users table", () => {
  it("UI-11 shows the columns, the explanation and both sources", async () => {
    mockApi(routes([dbRow, usedRow, fileRow]));
    render(<Page />);
    const t = await table();
    expect(t.getAllByRole("columnheader").map((h) => h.textContent)).toEqual([
      "Email",
      "Roles",
      "Source",
      "Added by",
      "Signed in",
      "Actions",
    ]);
    expect(screen.getByText(/get the chosen role the first time they sign in with SSO/)).toBeInTheDocument();
    const rows = t.getAllByRole("row");
    expect(rows).toHaveLength(4);
    expect(rows[1]).toHaveTextContent("alice@example.com");
    expect(rows[1]).toHaveTextContent("Developer");
    expect(rows[1]).toHaveTextContent("Added here");
    expect(rows[1]).toHaveTextContent("Riley Admin");
    expect(rows[3]).toHaveTextContent("carol@example.com");
    expect(rows[3]).toHaveTextContent("Developer, Admin");
    expect(rows[3]).toHaveTextContent("Config file");
  });

  it("UI-11 shows Not yet until used, then the date and who signed in", async () => {
    mockApi(routes([dbRow, usedRow]));
    render(<Page />);
    const t = await table();
    const [, unused, used] = t.getAllByRole("row");
    expect(unused).toHaveTextContent("Not yet");
    expect(used).not.toHaveTextContent("Not yet");
    expect(used).toHaveTextContent("Nov 12");
    expect(used).toHaveTextContent("Bob B.");
  });

  it("UI-11 a file row has no Remove button, a database row has one", async () => {
    mockApi(routes([dbRow, fileRow]));
    render(<Page />);
    const t = await table();
    expect(t.getAllByRole("button", { name: /^Remove/ })).toHaveLength(1);
    expect(t.getByRole("button", { name: "Remove alice@example.com" })).toBeInTheDocument();
    expect(t.queryByRole("button", { name: "Remove carol@example.com" })).toBeNull();
  });

  it("UI-11 an email in both sources appears as two rows", async () => {
    mockApi(routes([dbRow, { ...fileRow, email: dbRow.email }]));
    render(<Page />);
    const t = await table();
    const rows = t.getAllByRole("row").slice(1);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Added here");
    expect(rows[1]).toHaveTextContent("Config file");
  });

  it("UI-11 shows an empty message for an empty list", async () => {
    mockApi(routes([]));
    render(<Page />);
    expect(await screen.findByText("No pre-approved users.")).toBeInTheDocument();
  });
});

describe("UI-11 removing a database entry", () => {
  it("UI-11 asks for confirmation, then calls DELETE with the id and refetches", async () => {
    let rows = [dbRow, fileRow];
    const { calls } = mockApi(
      routes([], {
        "GET /api/v1/admin/preapprovals": () => ({ preapprovals: rows }),
        "DELETE /api/v1/admin/preapprovals/7": () => {
          rows = [fileRow];
          return new Response(null, { status: 204 });
        },
      }),
    );
    render(<Page />);
    const t = await table();
    await userEvent.click(t.getByRole("button", { name: "Remove alice@example.com" }));
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    const dialog = screen.getByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Remove" }));
    const del = calls.find((c) => c.method === "DELETE")!;
    expect(del.url).toBe("/api/v1/admin/preapprovals/7");
    expect(del.headers.get("X-CSRF-Token")).toBe("csrf-admin");
    await vi.waitFor(() => expect(screen.queryByText("alice@example.com")).toBeNull());
    expect(screen.getByText("carol@example.com")).toBeInTheDocument();
  });

  it("UI-11 Cancel sends nothing", async () => {
    const { calls } = mockApi(routes([dbRow]));
    render(<Page />);
    const t = await table();
    await userEvent.click(t.getByRole("button", { name: "Remove alice@example.com" }));
    await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Cancel" }));
    expect(calls.some((c) => c.method === "DELETE")).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("UI-11 adding emails", () => {
  it.each([
    ["commas", "a@example.test,b@example.test", ["a@example.test", "b@example.test"]],
    ["spaces", "a@example.test b@example.test", ["a@example.test", "b@example.test"]],
    ["new lines", "a@example.test\nb@example.test\r\nc@example.test", ["a@example.test", "b@example.test", "c@example.test"]],
    ["mixed with empty pieces", " a@example.test ,, b@example.test;\n\n ,c@example.test, ", ["a@example.test", "b@example.test;", "c@example.test"]],
    ["only separators", " , \n ,, ", []],
  ])("UI-11 splitEmails on %s", (_, text, want) => {
    expect(splitEmails(text)).toEqual(want);
  });

  it("UI-11 posts the split list with the default role developer, clears the text and refetches", async () => {
    let rows: Preapproval[] = [];
    const { calls } = mockApi(
      routes([], {
        "GET /api/v1/admin/preapprovals": () => ({ preapprovals: rows }),
        "POST /api/v1/admin/preapprovals": () => {
          rows = [dbRow];
          return { added: 3, updated: 0 };
        },
      }),
    );
    render(<Page />);
    await table();
    expect(screen.getByRole("combobox", { name: "Role" })).toHaveValue("developer");
    await userEvent.click(textarea());
    await userEvent.paste("a@example.test, b@example.test\n\n c@example.test ,");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    await vi.waitFor(() => expect(textarea()).toHaveValue(""));
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.url).toBe("/api/v1/admin/preapprovals");
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-admin");
    expect(post.body).toEqual({ emails: ["a@example.test", "b@example.test", "c@example.test"], roles: ["developer"] });
    expect(await screen.findByText("alice@example.com")).toBeInTheDocument();
  });

  it("UI-11 sends the chosen role", async () => {
    const { calls } = mockApi(routes([], { "POST /api/v1/admin/preapprovals": { added: 1, updated: 0 } }));
    render(<Page />);
    await table();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Role" }), "admin");
    await userEvent.type(textarea(), "a@example.test");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    await vi.waitFor(() => expect(calls.some((c) => c.method === "POST")).toBe(true));
    expect(calls.find((c) => c.method === "POST")!.body).toEqual({ emails: ["a@example.test"], roles: ["admin"] });
  });

  it("UI-11 Add is disabled with no email in the text", async () => {
    mockApi(routes([]));
    render(<Page />);
    await table();
    const add = screen.getByRole("button", { name: "Add" });
    expect(add).toBeDisabled();
    await userEvent.type(textarea(), " , \n");
    expect(add).toBeDisabled();
  });

  it("UI-11 a 400 shows the server's message next to the text area and keeps the text", async () => {
    const message = 'invalid email "nope": missing @';
    const { calls } = mockApi(
      routes([], { "POST /api/v1/admin/preapprovals": () => errorResponse(400, "validation", message, "emails") }),
    );
    render(<Page />);
    await table();
    await userEvent.click(textarea());
    await userEvent.paste("a@example.test nope");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    const msg = await screen.findByText(message);
    expect(textarea()).toHaveValue("a@example.test nope");
    expect(textarea()).toHaveAccessibleDescription(expect.stringContaining(message));
    expect(msg).toBeInTheDocument();
    // a failed add does not refetch the list
    expect(calls.filter((c) => c.method === "GET" && c.url.endsWith("/preapprovals"))).toHaveLength(1);
  });
});
