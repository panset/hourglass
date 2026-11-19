// SPDX-License-Identifier: MIT
// Acceptance test for the bundled and split stacks (docs/SPEC.md, "Testing without a vCenter" item 4).
// Run through e2e/run.sh; the stack must already be up on localhost:8080.
import { defineConfig } from "@playwright/test";
import path from "node:path";

const mode = process.env.E2E_MODE ?? "bundled";
const out = process.env.E2E_OUT ?? path.resolve(__dirname, "..", "out", mode);

export default defineConfig({
  testDir: "./tests",
  outputDir: path.join(out, "test-results"),
  timeout: 40 * 60_000,
  workers: 1,
  fullyParallel: false,
  retries: 0,
  reporter: [
    ["list"],
    ["html", { outputFolder: path.join(out, "playwright-report"), open: "never" }],
    ["json", { outputFile: path.join(out, "report.json") }],
  ],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:8080",
    headless: true,
    // e2e/run.sh sets E2E_BROWSER_CHANNEL=chrome when only system Chrome launches.
    channel: process.env.E2E_BROWSER_CHANNEL || undefined,
    actionTimeout: 30_000,
    navigationTimeout: 60_000,
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
