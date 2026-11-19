// SPDX-License-Identifier: MIT
import { api, ApiError, setCsrfToken, setUnauthorizedHandler } from "./client";
import { errorResponse, mockApi } from "../test/mockApi";

describe("api client (UI-2, UI-4, AUTH-3)", () => {
  it("sends X-CSRF-Token on state-changing calls only", async () => {
    const { calls } = mockApi({ "GET /api/v1/x": { ok: true }, "POST /api/v1/x": { ok: true }, "DELETE /api/v1/x": {} });
    setCsrfToken("tok-1");
    await api.get("/api/v1/x");
    await api.post("/api/v1/x", { a: 1 });
    await api.delete("/api/v1/x");
    expect(calls[0]!.headers.get("X-CSRF-Token")).toBeNull();
    expect(calls[1]!.headers.get("X-CSRF-Token")).toBe("tok-1");
    expect(calls[1]!.headers.get("Content-Type")).toBe("application/json");
    expect(calls[1]!.body).toEqual({ a: 1 });
    expect(calls[2]!.headers.get("X-CSRF-Token")).toBe("tok-1");
  });

  it("uses same-origin credentials", async () => {
    const { fetchMock } = mockApi({ "GET /api/v1/x": {} });
    await api.get("/api/v1/x");
    expect(fetchMock.mock.calls[0]![1]!.credentials).toBe("same-origin");
  });

  it("turns an error body into a typed ApiError {code, message}", async () => {
    mockApi({ "POST /api/v1/x": () => errorResponse(409, "hold_limit", "you already hold 2 setups") });
    const err = await api.post("/api/v1/x").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ code: "hold_limit", message: "you already hold 2 setups", status: 409 });
  });

  it("calls the unauthorized handler on 401, unless the call opts out", async () => {
    mockApi({ "GET /api/v1/x": () => errorResponse(401, "unauthenticated", "sign in required") });
    const handler = vi.fn();
    setUnauthorizedHandler(handler);
    await expect(api.get("/api/v1/x")).rejects.toMatchObject({ status: 401 });
    expect(handler).toHaveBeenCalledTimes(1);
    await expect(api.get("/api/v1/x", { skipUnauthorizedHandler: true })).rejects.toBeInstanceOf(ApiError);
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("reports an unreachable server as a network error", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("failed")));
    await expect(api.get("/api/v1/x")).rejects.toMatchObject({ code: "network" });
  });

  it("returns undefined for 204", async () => {
    mockApi({ "POST /auth/logout": new Response(null, { status: 204 }) });
    await expect(api.post("/auth/logout")).resolves.toBeUndefined();
  });
});
