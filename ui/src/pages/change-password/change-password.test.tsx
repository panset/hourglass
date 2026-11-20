// SPDX-License-Identifier: MIT
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { App } from "../../App";
import { barRoutes, errorResponse, localAdminSession, mockApi } from "../../test/mockApi";

const forced = { ...localAdminSession, must_change_password: true };

function setup(extra: Record<string, unknown> = {}) {
  const m = mockApi({ "GET /api/v1/session": forced, ...extra });
  render(
    <MemoryRouter initialEntries={["/"]}>
      <App />
    </MemoryRouter>,
  );
  return m;
}

describe("AUTH-7 change-password screen", () => {
  it("rejects a password shorter than 12 characters without calling the API", async () => {
    const { calls } = setup();
    await userEvent.type(await screen.findByLabelText("Current password"), "old");
    await userEvent.type(screen.getByLabelText("New password"), "short");
    await userEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("Use at least 12 characters.")).toBeInTheDocument();
    expect(calls.some((c) => c.method === "POST")).toBe(false);
  });

  it("requires the two entries to match", async () => {
    setup();
    await userEvent.type(await screen.findByLabelText("Current password"), "old");
    await userEvent.type(screen.getByLabelText("New password"), "a-long-enough-pw");
    await userEvent.type(screen.getByLabelText("Repeat new password"), "different-long-pw");
    await userEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("The two passwords do not match.")).toBeInTheDocument();
  });

  it("shows a server validation error on the right field", async () => {
    setup({ "POST /auth/local/password": () => errorResponse(400, "validation", "current password is wrong", "current_password") });
    await userEvent.type(await screen.findByLabelText("Current password"), "old");
    await userEvent.type(screen.getByLabelText("New password"), "a-long-enough-pw");
    await userEvent.type(screen.getByLabelText("Repeat new password"), "a-long-enough-pw");
    await userEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByText("current password is wrong")).toBeInTheDocument();
  });

  it("sends the CSRF header and continues into the app on success", async () => {
    const { calls } = setup({
      "POST /auth/local/password": { ...localAdminSession, csrf_token: "csrf-new" },
      ...barRoutes,
      "GET /api/v1/setups": { setups: [], display_label_keys: [], summary: { enabled: 0, deployed: 0, free: 0 }, metering_description: "" },
    });
    await userEvent.type(await screen.findByLabelText("Current password"), "old");
    await userEvent.type(screen.getByLabelText("New password"), "a-long-enough-pw");
    await userEvent.type(screen.getByLabelText("Repeat new password"), "a-long-enough-pw");
    await userEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByRole("heading", { name: "Setups" })).toBeInTheDocument();
    const post = calls.find((c) => c.method === "POST")!;
    expect(post.headers.get("X-CSRF-Token")).toBe("csrf-local");
    expect(post.body).toEqual({ current_password: "old", new_password: "a-long-enough-pw" });
  });
});
