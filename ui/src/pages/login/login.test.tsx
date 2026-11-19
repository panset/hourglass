// SPDX-License-Identifier: MIT
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { App } from "../../App";
import { errorResponse, localAdminSession, mockApi } from "../../test/mockApi";

function renderLogin(url = "/login") {
  return render(
    <MemoryRouter initialEntries={[url]}>
      <App />
    </MemoryRouter>,
  );
}
const noSession = { "GET /api/v1/session": () => errorResponse(401, "unauthenticated", "sign in required") };

describe("AUTH-6 login page", () => {
  it("shows SSO and the local form when both are available", async () => {
    mockApi({ ...noSession, "GET /api/v1/auth/methods": { oidc: true, local: true } });
    renderLogin();
    expect(await screen.findByRole("link", { name: "Sign in with SSO" })).toHaveAttribute("href", "/auth/oidc/start");
    expect(screen.getByText("Local administrator")).toBeInTheDocument();
    expect(screen.getByLabelText("Username")).toHaveValue("admin");
    expect(screen.getByLabelText("Password")).toHaveValue("");
    expect(screen.getByRole("button", { name: "Sign in as admin" })).toBeInTheDocument();
    expect(screen.getByText(/First local sign-in forces a password change/)).toBeInTheDocument();
  });

  it("hides SSO when OIDC is not available", async () => {
    mockApi({ ...noSession, "GET /api/v1/auth/methods": { oidc: false, local: true } });
    renderLogin();
    expect(await screen.findByRole("button", { name: "Sign in as admin" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Sign in with SSO" })).not.toBeInTheDocument();
  });

  it("shows only SSO when the local form is off", async () => {
    mockApi({ ...noSession, "GET /api/v1/auth/methods": { oidc: true, local: false } });
    renderLogin();
    expect(await screen.findByRole("link", { name: "Sign in with SSO" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Username")).not.toBeInTheDocument();
  });

  it("shows the ?error= from a failed SSO callback", async () => {
    mockApi({ ...noSession, "GET /api/v1/auth/methods": { oidc: true, local: true } });
    renderLogin("/login?error=forbidden");
    expect(await screen.findByRole("alert")).toHaveTextContent("Single sign-on was refused.");
  });

  it("shows an unknown ?error= code as given", async () => {
    mockApi({ ...noSession, "GET /api/v1/auth/methods": { oidc: true, local: true } });
    renderLogin("/login?error=weird_code");
    expect(await screen.findByRole("alert")).toHaveTextContent("weird_code");
  });

  it("shows wrong-credential errors without leaving the page", async () => {
    mockApi({
      ...noSession,
      "GET /api/v1/auth/methods": { oidc: false, local: true },
      "POST /auth/local": () => errorResponse(401, "unauthenticated", "invalid username or password"),
    });
    renderLogin();
    await userEvent.type(await screen.findByLabelText("Password"), "nope");
    await userEvent.click(screen.getByRole("button", { name: "Sign in as admin" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("invalid username or password");
    expect(screen.getByLabelText("Username")).toBeInTheDocument();
  });

  it("posts the credentials and enters the app on success", async () => {
    const { calls } = mockApi({
      ...noSession,
      "GET /api/v1/auth/methods": { oidc: false, local: true },
      "POST /auth/local": localAdminSession,
      "GET /api/v1/pressure": { smoothed: 0.5, multiplier: 1 },
      "GET /api/v1/admin/users": { pending_count: 0 },
    });
    renderLogin();
    await userEvent.type(await screen.findByLabelText("Password"), "secret-pass");
    await userEvent.click(screen.getByRole("button", { name: "Sign in as admin" }));
    expect(await screen.findByRole("heading", { name: "My usage" })).toBeInTheDocument();
    expect(calls.find((c) => c.method === "POST")?.body).toEqual({ username: "admin", password: "secret-pass" });
  });
});
