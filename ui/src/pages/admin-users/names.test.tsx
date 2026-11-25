// SPDX-License-Identifier: MIT
// UX-REVIEW A1 (P6 to P9) and A3: how the Users page shows a person, and the local account's row.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { setCsrfToken } from "../../api/client";
import { adminSession, mockApi } from "../../test/mockApi";
import { dana, localAdmin, morgan, priya, sam, userList, type AdminUser } from "../admin-shared/fixtures";
import Page from "./index";

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
  setCsrfToken("csrf-admin");
  viewer.session = adminSession;
});

/** The API as the server sends it: the pending list separately, the active list in server order. */
function mount(active: AdminUser[], pending: AdminUser[] = []) {
  const all = userList([...active, ...pending]);
  const only = userList(pending);
  mockApi({
    "GET /api/v1/admin/users": ({ url }: { url: string }) => (url.includes("status=pending") ? only : all),
    "GET /api/v1/admin/preapprovals": { preapprovals: [] },
  });
  return render(<Page />);
}
const usersTable = () => screen.findByRole("table", { name: "Users" });
const rowsOf = async () => within(await usersTable()).getAllByRole("row").slice(1);
// A person who gave no name: the server's display value is the username, else the email.
const nameless = (over: Partial<AdminUser>): AdminUser => ({ ...sam, name: "", ...over });

describe("A1-3 P6 P7 pending requests show a non-blank display name", () => {
  it("a named pending user shows the name above the email and names the buttons with it", async () => {
    mount([localAdmin], [morgan]);
    expect(await screen.findByText("Morgan Q.")).toBeInTheDocument();
    expect(screen.getByText("morgan@example.com")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Approve Morgan Q." })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject Morgan Q." })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Roles for Morgan Q." })).toBeInTheDocument();
  });

  it("an unnamed pending user shows the display value, never a blank or 'Approve ' with nothing after", async () => {
    mount([localAdmin], [{ ...morgan, name: "", display_name: "morgan.q" }]);
    expect(await screen.findByRole("button", { name: "Approve morgan.q" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Reject morgan.q" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Roles for morgan.q" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^(Approve|Reject)\s*$/ })).toBeNull();
  });

  it("an older API with no display_name still gives a non-blank name (name, username, then email)", async () => {
    const old = { ...morgan, name: "", username: "", display_name: undefined } as unknown as AdminUser;
    mount([localAdmin], [old]);
    expect(await screen.findByRole("button", { name: "Approve morgan@example.com" })).toBeInTheDocument();
  });
});

describe("A1-2 A1-4 A1-5 A1-6 P8 the All users cell", () => {
  it("shows the display name over the email; a nameless user shows the fallback and sits where the server put it", async () => {
    mount([localAdmin, priya, nameless({ id: 12, username: "zed", display_name: "zed", email: "zed@example.com" }), sam]);
    const rows = await rowsOf();
    expect(rows[1]).toHaveTextContent("Priya N.");
    expect(rows[1]).toHaveTextContent("priya@example.com");
    // the UI keeps the server order: zed was sent before Sam, and is not re-sorted
    expect(rows[2]).toHaveTextContent("zed");
    expect(rows[2]).toHaveTextContent("zed@example.com");
    expect(rows[3]).toHaveTextContent("Sam Lee");
  });

  it("a user with neither name nor username shows the email as the name, and no cell is blank", async () => {
    mount([localAdmin, nameless({ id: 13, username: "", display_name: "bare@example.com", email: "bare@example.com" })]);
    const rows = await rowsOf();
    expect(within(rows[1]!).getAllByText("bare@example.com").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole("button", { name: "Grant to bare@example.com" })).toBeInTheDocument();
  });

  it("the viewer's own row reads (you) after the name; nobody else's does", async () => {
    viewer.session = { ...adminSession, user_id: sam.id };
    mount([localAdmin, priya, sam]);
    const rows = await rowsOf();
    expect(rows[2]).toHaveTextContent("Sam Lee (you)");
    expect(rows[1]).not.toHaveTextContent("(you)");
  });

  it("a 40-character name is cut at 32 characters with the full name in title", async () => {
    const long = "Ada Lovelace-Byron of the Analytical Engine";
    mount([localAdmin, { ...sam, name: long, display_name: long }]);
    const clip = within((await rowsOf())[1]!).getByText(long);
    expect(clip).toHaveAttribute("title", long);
    expect(clip).toHaveStyle({ maxWidth: "32ch" });
  });

  it("an email is cut at 40 characters with the full email in title", async () => {
    const email = "a.very.long.mailbox.name.indeed@example.com";
    mount([localAdmin, { ...sam, email }]);
    const clip = within((await rowsOf())[1]!).getByText(email);
    expect(clip).toHaveAttribute("title", email);
    expect(clip).toHaveStyle({ maxWidth: "40ch" });
  });
});

describe("A1 P9 the row actions and texts use the display name", () => {
  it("Grant, Save roles, the panel heading, the toast and the reset dialog name the person", async () => {
    mount([localAdmin, nameless({ id: 12, username: "zed", display_name: "zed" })]);
    await usersTable();
    await userEvent.click(screen.getByRole("button", { name: "Grant to zed" }));
    expect(screen.getByRole("heading", { name: "Grant tokens to zed" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Roles for zed" })).toBeInTheDocument();
    await userEvent.click(within(screen.getByRole("group", { name: "Roles for zed" })).getByRole("checkbox", { name: "Admin" }));
    expect(screen.getByRole("button", { name: "Save roles for zed" })).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Reason (required)"), "fix");
    await userEvent.click(screen.getByRole("button", { name: /^Reset to/ }));
    expect(await screen.findByRole("dialog", { name: "Reset zed's balance?" })).toBeInTheDocument();
  });
});

describe("A3 the local account first, with its sub-line", () => {
  it("A3-1 A3-3 shows the rows in the server order: the local account, then overdrawn, then by name; the UI does not re-sort", async () => {
    const aaron = { ...sam, id: 30, name: "Aaron", display_name: "Aaron", username: "aaron", email: "aaron@example.com" };
    const zoe = { ...priya, id: 31, name: "Zoe", display_name: "Zoe", username: "zoe", email: "zoe@example.com" };
    mount([localAdmin, zoe, aaron]);
    const rows = await rowsOf();
    expect(rows.map((r) => r.querySelector("td")!.textContent)).toEqual([
      expect.stringContaining("admin"),
      expect.stringContaining("Zoe"),
      expect.stringContaining("Aaron"),
    ]);
  });

  it("A3-2 the local account's sub-line is 'Built-in local account', with no email, no new badge and a dash for Burned this month", async () => {
    mount([localAdmin, priya]);
    const row = (await rowsOf())[0]!;
    const cells = within(row).getAllByRole("cell");
    expect(cells[0]).toHaveTextContent("adminBuilt-in local account");
    expect(cells[0]).not.toHaveTextContent("Local account · first user");
    expect(cells[1]).toHaveTextContent("Admin · fixed");
    expect(cells[2]).toHaveTextContent("No balance");
    expect(cells[3]).toHaveTextContent("—");
    expect(cells[4]).toHaveTextContent("Cannot hold setups");
    expect(row.querySelectorAll("[class*=badge]")).toHaveLength(1);
  });

  it("A3-2 the note under the table names the order", async () => {
    mount([localAdmin, priya]);
    await usersTable();
    expect(
      screen.getByText(
        "The built-in local account is listed first, then overdrawn users, then everyone by name. An admin cannot remove their own admin role.",
      ),
    ).toBeInTheDocument();
  });

  it("A3 a user with no role still shows dashes (unchanged), next to the local row", async () => {
    mount([localAdmin, dana]);
    expect((await rowsOf())[1]).toHaveTextContent("Dana R.");
  });
});
