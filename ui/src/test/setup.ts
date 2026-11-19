// SPDX-License-Identifier: MIT
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";
import { setCsrfToken, setUnauthorizedHandler } from "../api/client";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
  setCsrfToken(undefined);
  setUnauthorizedHandler(undefined);
});
