// SPDX-License-Identifier: MIT
import { renderHook, waitFor } from "@testing-library/react";
import { ApiError } from "../api/client";
import { useApi } from "./useApi";

describe("useApi", () => {
  it("goes from loading to data", async () => {
    const { result } = renderHook(() => useApi(() => Promise.resolve(42)));
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.data).toBe(42));
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeUndefined();
  });

  it("exposes a typed error", async () => {
    const { result } = renderHook(() => useApi(() => Promise.reject(new ApiError(403, "forbidden", "no"))));
    await waitFor(() => expect(result.current.error?.code).toBe("forbidden"));
    expect(result.current.loading).toBe(false);
  });

  it("refetches when deps change and reload() refetches", async () => {
    const fetcher = vi.fn((n: number) => Promise.resolve(n));
    const { result, rerender } = renderHook(({ n }) => useApi(() => fetcher(n), [n]), { initialProps: { n: 1 } });
    await waitFor(() => expect(result.current.data).toBe(1));
    rerender({ n: 2 });
    await waitFor(() => expect(result.current.data).toBe(2));
    result.current.reload();
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(3));
  });

  it("does not fetch while disabled", () => {
    const fetcher = vi.fn(() => Promise.resolve(1));
    renderHook(() => useApi(fetcher, [], { enabled: false }));
    expect(fetcher).not.toHaveBeenCalled();
  });
});
