// SPDX-License-Identifier: MIT
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { App } from "../App";
import { barRoutes, developerSession, localAdminSession, mockApi } from "../test/mockApi";

function renderApp() {
  return render(
    <MemoryRouter initialEntries={["/usage"]}>
      <App />
    </MemoryRouter>,
  );
}

describe("UI-10 greeting in the top bar", () => {
  it("greets an SSO user with their display name", async () => {
    mockApi({ "GET /api/v1/session": developerSession, ...barRoutes });
    renderApp();
    expect(await screen.findByText("Hello, Dana Dev")).toBeInTheDocument();
  });

  it("greets the local admin", async () => {
    mockApi({ "GET /api/v1/session": localAdminSession, ...barRoutes });
    renderApp();
    expect(await screen.findByText("Hello, admin")).toBeInTheDocument();
  });

  it("shows a user with an empty name by their email (display_name)", async () => {
    mockApi({ "GET /api/v1/session": { ...developerSession, name: "", display_name: "dana@example.com" }, ...barRoutes });
    renderApp();
    expect(await screen.findByText("Hello, dana@example.com")).toBeInTheDocument();
  });

  it("shows no greeting at all when display_name is empty", async () => {
    mockApi({ "GET /api/v1/session": { ...developerSession, name: "", email: "", username: "", display_name: "" }, ...barRoutes });
    renderApp();
    expect(await screen.findByText(/Lab 82%/)).toBeInTheDocument();
    expect(screen.queryByText(/Hello/)).not.toBeInTheDocument();
  });

  it("carries the full name in title so a long name can be cut with an ellipsis", async () => {
    const long = "Dana Developer-Of-A-Very-Long-Hyphenated-Surname The Third, Esquire";
    mockApi({ "GET /api/v1/session": { ...developerSession, display_name: long }, ...barRoutes });
    renderApp();
    const greeting = await screen.findByText(`Hello, ${long}`);
    expect(greeting).toHaveAttribute("title", long);
  });
});
