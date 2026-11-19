// SPDX-License-Identifier: MIT
import { act, renderHook } from "@testing-library/react";
import { usePolling } from "./usePolling";

function setHidden(hidden: boolean) {
  Object.defineProperty(document, "hidden", { configurable: true, get: () => hidden });
  document.dispatchEvent(new Event("visibilitychange"));
}

describe("UI-6 usePolling", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    setHidden(false);
  });

  it("calls at once, then every interval", () => {
    const fn = vi.fn();
    renderHook(() => usePolling(fn, 60_000));
    expect(fn).toHaveBeenCalledTimes(1);
    act(() => void vi.advanceTimersByTime(60_000));
    expect(fn).toHaveBeenCalledTimes(2);
    act(() => void vi.advanceTimersByTime(120_000));
    expect(fn).toHaveBeenCalledTimes(4);
  });

  it("pauses while the tab is hidden and refetches when it is visible again", () => {
    const fn = vi.fn();
    renderHook(() => usePolling(fn, 60_000));
    act(() => setHidden(true));
    act(() => void vi.advanceTimersByTime(300_000));
    expect(fn).toHaveBeenCalledTimes(1);
    act(() => setHidden(false));
    expect(fn).toHaveBeenCalledTimes(2);
    act(() => void vi.advanceTimersByTime(60_000));
    expect(fn).toHaveBeenCalledTimes(3);
  });

  it("refetches on window focus", () => {
    const fn = vi.fn();
    renderHook(() => usePolling(fn, 60_000));
    act(() => void window.dispatchEvent(new Event("focus")));
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("stops on unmount and when disabled", () => {
    const fn = vi.fn();
    const { unmount } = renderHook(() => usePolling(fn, 60_000));
    unmount();
    act(() => void vi.advanceTimersByTime(180_000));
    expect(fn).toHaveBeenCalledTimes(1);
    const off = vi.fn();
    renderHook(() => usePolling(off, 60_000, false));
    act(() => void vi.advanceTimersByTime(180_000));
    expect(off).not.toHaveBeenCalled();
  });

  it("uses the latest callback without restarting the timer", () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = renderHook(({ fn }) => usePolling(fn, 60_000), { initialProps: { fn: first } });
    rerender({ fn: second });
    act(() => void vi.advanceTimersByTime(60_000));
    expect(first).toHaveBeenCalledTimes(1);
    expect(second).toHaveBeenCalledTimes(1);
  });
});
