// SPDX-License-Identifier: MIT
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { App } from "../App";
import {
  adminSession,
  barRoutes,
  developerSession,
  errorResponse,
  localAdminSession,
  mockApi,
  noRoleSession,
} from "../test/mockApi";

function Where() {
  return <div data-testid="where">{useLocation().pathname}</div>;
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
      <Where />
    </MemoryRouter>,
  );
}

const where = () => screen.getByTestId("where").textContent;

describe("UI-5 shell routing by session state", () => {
  it("no session goes to /login", async () => {
    mockApi({
      "GET /api/v1/session": () => errorResponse(401, "unauthenticated", "sign in required"),
      "GET /api/v1/auth/methods": { oidc: true, local: true },
    });
    renderAt("/setups");
    expect(await screen.findByRole("heading", { name: "Hourglass" })).toBeInTheDocument();
    expect(where()).toBe("/login");
  });

  it("AUTH-5 no roles shows only /unauthorized whatever the URL, with the exact sentence and email", async () => {
    mockApi({ "GET /api/v1/session": noRoleSession });
    renderAt("/admin/users");
    expect(
      await screen.findByText("You are not authorized to use Hourglass. Reach out to your admin."),
    ).toBeInTheDocument();
    expect(where()).toBe("/unauthorized");
    expect(screen.getByText("Signed in as morgan@example.com")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign out" })).toBeInTheDocument();
    expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
  });

  it("must_change_password shows the change-password screen and nothing else", async () => {
    mockApi({ "GET /api/v1/session": { ...localAdminSession, must_change_password: true } });
    renderAt("/");
    expect(await screen.findByRole("heading", { name: "Change your password" })).toBeInTheDocument();
    expect(where()).toBe("/change-password");
    expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
  });

  it("a developer gets the layout, the balance, the pressure pill and no admin links", async () => {
    mockApi({ "GET /api/v1/session": developerSession, ...barRoutes });
    renderAt("/");
    expect(await screen.findByRole("heading", { name: "My usage" })).toBeInTheDocument();
    const nav = screen.getByRole("navigation", { name: "Main" });
    expect(nav).toHaveTextContent("My usage");
    expect(nav).toHaveTextContent("Setups");
    expect(screen.queryByText("Admin")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Users/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Audit log" })).not.toBeInTheDocument();
    expect(await screen.findByText("142.2 tokens")).toBeInTheDocument();
    expect(await screen.findByText(/Lab 82%/)).toHaveTextContent("Lab 82% · 2×");
    expect(screen.getByText("Dana Dev")).toBeInTheDocument();
  });

  it("a developer on an admin URL gets not-found, not the admin page", async () => {
    mockApi({ "GET /api/v1/session": developerSession, ...barRoutes });
    renderAt("/admin/settings");
    expect(await screen.findByRole("heading", { name: "Page not found" })).toBeInTheDocument();
  });

  it("an admin sees the admin links, with the pending count on Users", async () => {
    mockApi({ "GET /api/v1/session": adminSession, ...barRoutes });
    renderAt("/admin/users");
    expect(await screen.findByRole("heading", { name: "Users" })).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: "Users · 2 pending" })).toHaveAttribute("aria-current", "page");
    for (const name of ["Setup inventory", "Settings", "Audit log"]) {
      expect(screen.getByRole("link", { name })).toBeInTheDocument();
    }
  });

  it("the local admin sees no balance and /me is never called", async () => {
    const { calls } = mockApi({ "GET /api/v1/session": localAdminSession, ...barRoutes });
    renderAt("/");
    expect(await screen.findByText(/Lab 82%/)).toBeInTheDocument();
    expect(screen.queryByText(/tokens$/)).not.toBeInTheDocument();
    expect(calls.some((c) => c.url === "/api/v1/me")).toBe(false);
  });

  it("a signed-in user visiting /login is sent to the app", async () => {
    mockApi({ "GET /api/v1/session": developerSession, ...barRoutes });
    renderAt("/login");
    expect(await screen.findByRole("heading", { name: "My usage" })).toBeInTheDocument();
    expect(where()).toBe("/");
  });

  it("a 401 from any call sends the user to /login", async () => {
    let meCalls = 0;
    mockApi({
      "GET /api/v1/session": developerSession,
      "GET /api/v1/pressure": barRoutes["GET /api/v1/pressure"],
      "GET /api/v1/me": () => {
        meCalls++;
        return errorResponse(401, "unauthenticated", "sign in required");
      },
      "GET /api/v1/auth/methods": { oidc: false, local: true },
    });
    renderAt("/setups");
    await waitFor(() => expect(where()).toBe("/login"));
    expect(meCalls).toBeGreaterThan(0);
  });

  it("Sign out posts to /auth/logout with the CSRF token and returns to /login", async () => {
    const { calls } = mockApi({
      "GET /api/v1/session": developerSession,
      ...barRoutes,
      "POST /auth/logout": new Response(null, { status: 204 }),
      "GET /api/v1/auth/methods": { oidc: true, local: true },
    });
    renderAt("/");
    await userEvent.click(await screen.findByRole("button", { name: "Sign out" }));
    await waitFor(() => expect(where()).toBe("/login"));
    const post = calls.find((c) => c.method === "POST");
    expect(post?.headers.get("X-CSRF-Token")).toBe("csrf-dana");
  });

  it("a network failure on the first GET /session shows Cannot reach the server with Retry, not sign-in", async () => {
    let fail = true;
    mockApi({
      "GET /api/v1/session": () => {
        if (fail) throw new TypeError("Failed to fetch");
        return developerSession;
      },
      ...barRoutes,
    });
    renderAt("/");
    expect(await screen.findByText("Cannot reach the server.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Sign in as admin" })).toBeNull();
    fail = false;
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("navigation", { name: "Main" })).toBeInTheDocument();
  });

  it("a render error in a page shows a message with Reload instead of a blank page", async () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    mockApi({ "GET /api/v1/session": developerSession, ...barRoutes, "GET /api/v1/me/usage": () => ({ days: null }) });
    renderAt("/");
    expect(await screen.findByRole("button", { name: "Reload" })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Main" })).toBeInTheDocument();
    spy.mockRestore();
  });
});
