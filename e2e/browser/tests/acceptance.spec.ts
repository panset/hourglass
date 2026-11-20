// SPDX-License-Identifier: MIT
// Acceptance path (docs/SPEC.md, "Acceptance path"), driven through the real UI in headless Chromium.
// The same file runs against the bundled and the split stack (E2E_MODE). Test ids cite the steps:
//   A1 not-authorized (AUTH-5), A2 admin approves (ADM-1), A3 acquire (HOLD-1), A4 pool + metering (MET-1, MET-4),
//   A5 pressure band (MET-2, MET-3), A6 hold limit (HOLD-3), A7 release of a deployed setup is pending until teardown (HOLD-2, HOLD-6, HOLD-7),
//   A8 month rollover with debt (LED-3).
// Only two things are driven outside the UI, as they stand in for the outside world: the simulator control API
// (pool and pressure) and the e2e clock control (month jump).
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

const MODE = process.env.E2E_MODE ?? "bundled";
const OUT = process.env.E2E_OUT ?? path.resolve(__dirname, "..", "..", "out", MODE);
const BASE = process.env.E2E_BASE_URL ?? "http://localhost:8080";
const SIM = process.env.E2E_SIM_URL ?? "http://localhost:8990";
const CLOCK = process.env.E2E_CLOCK_URL ?? "http://localhost:8091";

// Fake development credentials of the local stack (deploy/README.md); nothing real.
const ADMIN_INITIAL = "dev-only-not-a-secret";
const ADMIN_NEW = "dev-only-e2e-admin-password";
const ALICE = { user: "alice", password: "dev-only-alice" };

const SETUP_A = "lab-03"; // size=large, weight 2 (rate rule size=large)
const SETUP_B = "lab-04"; // size=large
const SETUP_FREE = "lab-01"; // size=medium, stays free
const POOL_A = `${SETUP_A}_RP`; // the provider strips the suffix (pool_name_suffix in deploy/config.sample.yaml, D40)
const POLL_WAIT = 9 * 60_000; // first poll after startup sleeps up to 5 minutes (default interval)

// ---------------------------------------------------------------- reporting

interface StepResult {
  id: string;
  title: string;
  status: "pass" | "fail";
  seconds: number;
  asserted: string[];
  observed: Record<string, unknown>;
  error?: string;
}
const results: StepResult[] = [];
let current: StepResult | undefined;
const shots = { n: 0 };

const asserted = (s: string) => current?.asserted.push(s);
const observe = (k: string, v: unknown) => {
  if (current) current.observed[k] = v;
};

async function shot(page: Page | undefined, label: string) {
  if (!page || page.isClosed()) return;
  const file = path.join(OUT, "screenshots", `${String(++shots.n).padStart(2, "0")}-${label}.png`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  await page.screenshot({ path: file, fullPage: true }).catch(() => undefined);
}

/** Runs one acceptance step; a failure is recorded (with a screenshot) and the run continues. */
async function step(id: string, title: string, page: () => Page | undefined, fn: () => Promise<void>) {
  const r: StepResult = { id, title, status: "pass", seconds: 0, asserted: [], observed: {} };
  current = r;
  const t0 = Date.now();
  console.log(`== ${id} ${title}`);
  try {
    await fn();
  } catch (e) {
    r.status = "fail";
    r.error = String((e as Error).message ?? e).split("\n").slice(0, 12).join("\n");
    console.log(`!! ${id} FAILED: ${r.error}`);
  }
  r.seconds = Math.round((Date.now() - t0) / 1000);
  await shot(page(), `${id}-${r.status}`);
  results.push(r);
  current = undefined;
}

// ---------------------------------------------------------------- helpers

const num = (s: string) => Number(s.replace(/−/g, "-").replace(/[+,]/g, "").replace(/[^\d.\-]/g, ""));

async function simPut(p: string, body: unknown) {
  const res = await fetch(`${SIM}${p}`, { method: "PUT", body: JSON.stringify(body) });
  expect(res.ok, `simulator PUT ${p}: ${res.status}`).toBeTruthy();
}

async function signInSSO(page: Page, who: { user: string; password: string }) {
  await page.goto("/login");
  await page.getByRole("link", { name: "Sign in with SSO" }).click();
  await finishKeycloak(page, who);
}

/** Fills Keycloak's form when it is shown; with a live Keycloak session it just redirects back. */
async function finishKeycloak(page: Page, who: { user: string; password: string }) {
  await page.waitForURL(/keycloak\.localhost:8081|localhost:8080\/(?!auth\/)/, { timeout: 60_000 });
  if (page.url().includes("keycloak.localhost")) {
    await page.locator("#username").fill(who.user);
    await page.locator("#password").fill(who.password);
    await page.locator("#kc-login").click();
  }
  await page.waitForURL((u) => u.origin === BASE && !u.pathname.startsWith("/auth/"), { timeout: 60_000 });
}

async function adminSignIn(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Username").fill("admin");
  await page.getByLabel("Password").fill(ADMIN_INITIAL);
  await page.getByRole("button", { name: "Sign in as admin" }).click();
  await expect(page.getByRole("heading", { name: "Change your password" })).toBeVisible();
  await page.getByLabel("Current password").fill(ADMIN_INITIAL);
  await page.getByLabel("New password", { exact: true }).fill(ADMIN_NEW);
  await page.getByLabel("Repeat new password").fill(ADMIN_NEW);
  await page.getByRole("button", { name: "Change password" }).click();
  await expect(page.getByRole("navigation", { name: "Main" })).toBeVisible();
}

/**
 * The month jump moves Hourglass's clock about 12 days ahead, while Keycloak runs on the real clock. An ID token
 * issued after the jump would be fine, but the sign-in that follows the jump (the old session has expired under the
 * new clock) must not be rejected for a token Keycloak signed with its default 5 minute lifetime. So, as part of
 * standing up the identity provider (outside world, like the simulator), the realm's token lifetimes are raised to
 * 40 days through Keycloak's admin REST API. Hourglass itself is untouched.
 */
async function stretchKeycloakTokenLifetime() {
  const kc = process.env.E2E_KEYCLOAK_URL ?? "http://localhost:8081";
  const tok = await fetch(`${kc}/realms/master/protocol/openid-connect/token`, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ grant_type: "password", client_id: "admin-cli", username: "admin", password: "dev-only-not-a-secret" }),
  });
  expect(tok.ok, `keycloak admin token: ${tok.status}`).toBeTruthy();
  const auth = { authorization: `Bearer ${(await tok.json()).access_token}`, "content-type": "application/json" };
  const realm = await (await fetch(`${kc}/admin/realms/hourglass`, { headers: auth })).json();
  const days40 = 40 * 24 * 3600;
  Object.assign(realm, { accessTokenLifespan: days40, ssoSessionIdleTimeout: days40, ssoSessionMaxLifespan: days40 });
  const put = await fetch(`${kc}/admin/realms/hourglass`, { method: "PUT", headers: auth, body: JSON.stringify(realm) });
  expect(put.ok, `keycloak realm update: ${put.status}`).toBeTruthy();
}

const nav = (page: Page, name: string) =>
  page.getByRole("navigation", { name: "Main" }).getByRole("link", { name: new RegExp(`^${name}`) }).click();

interface LedgerRow {
  when: string;
  type: string;
  detail: string;
  amount: number;
  balance: number;
  mult?: number;
}

/** My usage > Ledger, oldest first. */
async function readLedger(page: Page): Promise<LedgerRow[]> {
  const rows = page.getByRole("table", { name: "Ledger" }).locator("tbody tr");
  const n = await rows.count();
  const out: LedgerRow[] = [];
  for (let i = 0; i < n; i++) {
    const c = await rows.nth(i).locator("td").allInnerTexts();
    if (c.length < 5) continue;
    const m = /(\d+(?:\.\d+)?)×/.exec(c[1]!);
    out.push({ when: c[0]!, type: c[1]!, detail: c[2]!, amount: num(c[3]!), balance: num(c[4]!), mult: m ? Number(m[1]) : undefined });
  }
  // The table must be newest first: each row's balance is the next (older) row's balance plus its own
  // amount (within display rounding). Assert that for every pair, then return the rows oldest first.
  for (let i = 0; i + 1 < out.length; i++) {
    const a = out[i]!, b = out[i + 1]!;
    expect(
      Math.abs(a.balance - (b.balance + a.amount)),
      `ledger not newest first at row ${i + 1}: ${a.when} ${a.type} ${a.amount} -> ${a.balance}, then ${b.when} ${b.type} ${b.amount} -> ${b.balance}`,
    ).toBeLessThanOrEqual(0.002);
  }
  return out.reverse();
}
const debits = (l: LedgerRow[], mult?: number) =>
  l.filter((r) => r.type.startsWith("Debit") && (mult === undefined || r.mult === mult));

async function reloadHome(page: Page) {
  await page.goto("/usage");
  await expect(page.getByRole("heading", { name: "My usage" })).toBeVisible();
  await expect(page.getByRole("table", { name: "Ledger" })).toBeVisible();
}

/** Reloads My usage every 10 s until `check` returns a truthy value; waits for collector polls, not fixed sleeps. */
async function untilHome<T>(page: Page, what: string, check: () => Promise<T | false | undefined>, timeout = POLL_WAIT): Promise<T> {
  let got: T | false | undefined;
  await expect
    .poll(
      async () => {
        await reloadHome(page);
        got = await check();
        return !!got;
      },
      { message: `waiting for ${what}`, timeout, intervals: [10_000] },
    )
    .toBeTruthy();
  return got as T;
}

/** The big number in the "Tokens left" card of My usage. */
async function balance(page: Page): Promise<number> {
  const card = page.locator('section[aria-label="Tokens left"]');
  const t = await card.locator("div").nth(1).innerText();
  return num(t.replace("Overdrawn", ""));
}

const pill = async (page: Page) => (await page.locator("header").getByText(/^Lab \d+%/).innerText()).replace(/\s+/g, " ");

// ---------------------------------------------------------------- the test

test("acceptance path: sign in, approve, acquire, meter, pressure, hold limit, release, rollover", async ({ browser }) => {
  let alice: Page | undefined;
  let admin: Page | undefined;
  let aliceCtx: BrowserContext | undefined;
  let adminCtx: BrowserContext | undefined;
  const origins = new Set<string>();
  const t0 = Date.now();

  aliceCtx = await browser.newContext({ baseURL: BASE });
  adminCtx = await browser.newContext({ baseURL: BASE });
  await aliceCtx.tracing.start({ screenshots: true, snapshots: true });
  await adminCtx.tracing.start({ screenshots: true, snapshots: true });
  alice = await aliceCtx.newPage();
  admin = await adminCtx.newPage();
  alice.on("request", (r) => origins.add(new URL(r.url()).origin));
  const A = () => alice;
  const D = () => admin;

  // Admin first run (not a numbered step of the checklist, but the spec's first-run checklist): through Settings.
  await step("A0", "local admin signs in, changes the password, configures Settings", D, async () => {
    await adminSignIn(admin!);
    await expect(admin!).toHaveURL(/\/setups$/);
    await expect(admin!.getByRole("navigation", { name: "Main" }).getByRole("link", { name: "Setups" })).toHaveAttribute("aria-current", "page");
    asserted("admin form sign-in, forced password change completed, navigation shown, admin lands on Setups (/setups)");
    await nav(admin!, "Settings");
    await expect(admin!.getByRole("heading", { name: "Settings" })).toBeVisible();
    const p = admin!;
    await p.getByLabel("Display label keys").fill("size");
    await p.getByLabel("Poll interval").fill("1m");
    const enforce = p.getByRole("checkbox", { name: "Block acquiring at zero tokens" });
    if (!(await enforce.isChecked())) await enforce.check();
    // Bands: base 1x below 5% pressure, 2x from 5%. The simulator's baseline (about 1%) is in the first band and
    // 100% CPU, even averaged with up to 14 baseline polls in the 15-minute window (at least 1/15 = 6.7%), is in the second.
    await p.getByLabel("Base multiplier", { exact: true }).fill("1");
    await p.getByLabel("From pressure 1 (0 to 1)", { exact: true }).fill("0.05");
    await p.getByLabel("Multiplier 1", { exact: true }).fill("2");
    for (const i of [3, 2]) {
      const rm = p.getByRole("button", { name: `Remove band ${i}` });
      if (await rm.count()) await rm.click();
    }
    await p.getByRole("button", { name: "Save changes" }).click();
    await expect(p.getByText("Settings saved. They apply from the next poll.")).toBeVisible();
    await p.reload();
    await expect(p.getByLabel("Display label keys")).toHaveValue("size");
    await expect(p.getByLabel("Poll interval")).toHaveValue("1m");
    await expect(p.getByLabel("From pressure 1 (0 to 1)", { exact: true })).toHaveValue("0.05");
    await expect(p.getByRole("checkbox", { name: "Block acquiring at zero tokens" })).toBeChecked();
    asserted("label key size, poll 1m, enforcement ON, bands 1x / 2x from 0.05 saved and reloaded");
    observe("bands", "base 1, from 0.05 -> 2");
    await stretchKeycloakTokenLifetime();
    observe("keycloak", "realm token and SSO session lifetimes set to 40 days (see A8)");
  });

  // ---- 1
  await step("A1", "new user signs in and sees the not-authorized page", A, async () => {
    const p = alice!;
    await signInSSO(p, ALICE);
    await expect(p.getByRole("heading", { level: 1 })).toHaveText("You are not authorized to use Hourglass. Reach out to your admin.");
    await expect(p.getByText("Signed in as alice@example.com")).toBeVisible();
    await expect(p.getByRole("navigation")).toHaveCount(0);
    await expect(p.locator("header")).toHaveCount(0);
    const text = await p.locator("body").innerText();
    expect(text).not.toMatch(/lab-0\d|My usage|Setups|tokens/i);
    const res = await p.request.get("/api/v1/setups");
    expect(res.status()).toBe(403);
    expect((await res.json()).error.code).toBe("no_role");
    asserted("exact sentence; signed-in email; no navigation or header; no setup names or balances in the page text");
    asserted("GET /api/v1/setups from this session: 403 no_role");
    observe("url", p.url());
  });

  // ---- 2
  await step("A2", "the local admin approves the user", D, async () => {
    const p = admin!;
    await nav(p, "Users");
    const item = p.getByRole("listitem").filter({ hasText: "alice@example.com" });
    await expect(item).toBeVisible({ timeout: 90_000 });
    asserted("alice appears under Pending access requests with her email alice@example.com");
    await p.reload(); // the nav's pending count refreshes every 60 s; a reload shows it now
    await expect(p.getByRole("link", { name: /Users · \d+ pending/ })).toBeVisible();
    await item.getByRole("group").getByLabel("Developer").check();
    await item.getByRole("button", { name: /^Approve/ }).click();
    await expect(p.getByRole("listitem").filter({ hasText: "alice@example.com" })).toHaveCount(0);
    const q = alice!;
    await q.goto("/");
    await expect(q).toHaveURL(/\/usage$/);
    await expect(q.getByRole("heading", { name: "My usage" })).toBeVisible();
    await expect(q.locator('section[aria-label="Tokens left"]')).toContainText("300");
    await expect(q.locator("header")).toContainText("300 tokens");
    asserted("after approval alice lands on My usage; Tokens left card and top bar show 300");
    observe("tokens_left", await balance(q));
  });

  // ---- 3
  await step("A3", "acquire a size=large setup", A, async () => {
    const p = alice!;
    await nav(p, "Setups");
    await expect(p.getByRole("heading", { name: "Setups" })).toBeVisible();
    const row = p.getByRole("table", { name: "All setups" }).getByRole("row", { name: new RegExp(SETUP_A) });
    await expect(row).toContainText("large");
    await p.getByRole("button", { name: `Acquire ${SETUP_A}` }).click();
    const dlg = p.getByRole("dialog");
    await expect(dlg).toContainText(new RegExp(`Acquire ${SETUP_A}\\?`));
    await expect(dlg).toContainText(/Weight 2\./);
    await expect(dlg).toContainText(/Costs [\d.,]+ \/ h/);
    observe("acquire_dialog", (await dlg.innerText()).replace(/\s+/g, " "));
    asserted("acquire dialog shows weight 2 and a cost per hour");
    await dlg.getByRole("button", { name: "Acquire", exact: true }).click();
    await expect(p.getByText(`Acquired ${SETUP_A}.`)).toBeVisible();
    await expect(row).toContainText("Acquired · deploying");
    await expect(row).toContainText("You");
    await expect(row).not.toContainText("Deployed");
    await reloadHome(p);
    await expect(p.getByText("Current holds · 1 of 2 allowed")).toBeVisible();
    await expect(p.locator('section[aria-label="Burning now"]')).toContainText("Nothing is metering");
    // Held by alice, from the admin's view.
    await admin!.goto("/admin/users");
    await expect(admin!.getByRole("table", { name: "Users" }).getByRole("row", { name: /alice@example\.com/ })).toContainText(SETUP_A);
    asserted(`${SETUP_A} is held by "You" (alice; admin Users page lists it under Holding), state Acquired · deploying, nothing metering`);
  });

  // ---- 4
  await step("A4", "pool created in the simulator, metering starts at weight 2", A, async () => {
    const p = alice!;
    const res = await fetch(`${SIM}/pools`, { method: "POST", body: JSON.stringify({ name: POOL_A }) });
    expect(res.status).toBe(201);
      await untilHome(p, `${SETUP_A} deployed`, async () => {
      const row = p.getByRole("table", { name: "Current holds" }).getByRole("row", { name: new RegExp(SETUP_A) });
      return (await row.getByText("Deployed", { exact: true }).count()) > 0;
    });
    const row = p.getByRole("table", { name: "Current holds" }).getByRole("row", { name: new RegExp(SETUP_A) });
    await expect(row).toContainText("Deployed");
    const cells = await row.locator("td").allInnerTexts();
    expect(num(cells[3]!)).toBe(2);
    asserted("hold row shows Deployed and weight 2");
    observe("hold_row", cells.join(" | "));

    // The balance must fall: first debit comes one poll after metering started (D19).
    const l = await untilHome(p, "two 1x debits", async () => {
      const lg = await readLedger(p);
      return debits(lg, 1).length >= 2 ? lg : false;
    });
    const burning = await p.locator('section[aria-label="Burning now"]').innerText();
    expect(burning.replace(/\s+/g, " ")).toMatch(new RegExp(`${SETUP_A} · weight 2 × 1×`));
    observe("burning_now", burning.replace(/\s+/g, " "));
    const d = debits(l, 1);
    const expected = (2 * 1 * 60) / 3600; // weight x multiplier x one 60 s poll, in tokens
    for (const x of d.slice(0, 2)) {
      expect(Math.abs(x.amount)).toBeGreaterThan(expected * 0.75);
      expect(Math.abs(x.amount)).toBeLessThan(expected * 1.25);
    }
    const bal = await balance(p);
    const sum = d.reduce((s, x) => s + x.amount, 0);
    expect(Math.abs(bal - (300 + sum))).toBeLessThan(0.002 * (d.length + 1));
    await expect(p.locator("header")).toContainText(/Lab \d+% · 1×/);
    asserted(`weight 2 x multiplier 1x x one poll = ${expected.toFixed(4)} tokens; each of the first two debits within 25 percent`);
    asserted("Tokens left = 300 + sum of the debits listed in the ledger (within rounding)");
    asserted("Burning now shows lab-03 · weight 2 × 1×; top bar pill shows 1×");
    observe("debits_1x", d.map((x) => x.amount));
    observe("expected_per_poll", expected);
    observe("balance", bal);
    observe("pill", await pill(p));
  });

  // ---- 5
  await step("A5", "pressure driven across a band, the debit rate changes", A, async () => {
    const p = alice!;
    await reloadHome(p);
    const before = await pill(p);
    const beforePct = num(/Lab (\d+)%/.exec(before)![1]!) / 100;
    expect(beforePct).toBeLessThan(0.05);
    await simPut("/pressure", { cpu: 1.0 });
    // Smoothed pressure is the mean of the polls in the last 15 minutes (D-smoothing): after one poll at ~1.0 and
    // at most 14 baseline polls it is at least 1/15. Wait for the pill to show the new multiplier.
    const pillAfter = await untilHome(p, "top bar multiplier 2x", async () => {
      const t = await pill(p);
      return t.includes("2×") ? t : false;
    }, 6 * 60_000);
    const afterPct = num(/Lab (\d+)%/.exec(pillAfter)![1]!) / 100;
    expect(afterPct).toBeGreaterThanOrEqual(0.05);
    asserted(`top bar pill moved from "${before}" to "${pillAfter}"; smoothed pressure is on the right side of the 5 percent threshold`);
    observe("pill_before", before);
    observe("pill_after", pillAfter);
    const l = await untilHome(p, "two 2x debits", async () => {
      const lg = await readLedger(p);
      return debits(lg, 2).length >= 2 ? lg : false;
    });
    const d1 = debits(l, 1).slice(-2).map((x) => Math.abs(x.amount));
    const d2 = debits(l, 2).slice(0, 2).map((x) => Math.abs(x.amount));
    const mean = (a: number[]) => a.reduce((s, x) => s + x, 0) / a.length;
    const ratio = mean(d2) / mean(d1);
    expect(ratio).toBeGreaterThan(1.75);
    expect(ratio).toBeLessThan(2.25);
    asserted("two consecutive debits at 2x are twice the two last debits at 1x (ratio 1.75 to 2.25, absorbing rounding to 0.001 and poll jitter)");
    await expect(p.locator('section[aria-label="Burning now"]')).toContainText("weight 2 × 2×");
    observe("debits_1x_last2", d1);
    observe("debits_2x_first2", d2);
    observe("ratio", Number(ratio.toFixed(3)));
  });

  // ---- 6
  await step("A6", "a third acquire is refused by the hold limit", A, async () => {
    const p = alice!;
    await nav(p, "Setups");
    await p.getByRole("button", { name: `Acquire ${SETUP_B}` }).click();
    await p.getByRole("dialog").getByRole("button", { name: "Acquire", exact: true }).click();
    await expect(p.getByText(`Acquired ${SETUP_B}.`)).toBeVisible();
    await p.reload();
    // Through the UI: with two holds the hold-limit message is shown and the Acquire control of a third
    // setup is visible but disabled, so the browser cannot even attempt the third acquire.
    await expect(p.getByText("You already hold 2 of 2 allowed setups.")).toBeVisible();
    const third = p.getByRole("button", { name: `Acquire ${SETUP_FREE}` });
    await expect(third).toBeVisible();
    await expect(third).toBeDisabled();
    asserted("Setups page says \"You already hold 2 of 2 allowed setups.\" and the Acquire control of a third setup is disabled");
    // The direct API call below is the proof that the server refuses too: the disabled control
    // means the UI never lets that click through, so this is the only way to observe the 409 itself.
    const sess = await (await p.request.get("/api/v1/session")).json();
    const list = await (await p.request.get("/api/v1/setups")).json();
    const free = list.setups.find((s: { name: string }) => s.name === SETUP_FREE);
    const res = await p.request.post(`/api/v1/setups/${free.id}/acquire`, { headers: { "X-CSRF-Token": sess.csrf_token } });
    const body = await res.json();
    observe("api_status", res.status());
    observe("api_error", body.error);
    expect(res.status()).toBeGreaterThanOrEqual(400);
    expect(body.error.code).toBe("hold_limit");
    expect(body.error.message).toBe("You already hold the maximum of 2 setups.");
    asserted("POST acquire on a free setup: refused with code hold_limit and the hold-limit message");
  });

  // ---- 7
  await step("A7", "release of a deployed setup is pending until the pool is destroyed, then completes", A, async () => {
    const p = alice!;
    await reloadHome(p);
    const holds = p.getByRole("table", { name: "Current holds" });
    const row = holds.getByRole("row", { name: new RegExp(SETUP_A) });
    await row.getByRole("button", { name: "Release" }).click();
    // D43: the release of a deployed setup is not refused any more; it is accepted (202) and stays pending.
    await expect(p.getByText(`Release requested for ${SETUP_A}.`)).toBeVisible();
    await expect(p.getByText("This setup is still deployed")).toHaveCount(0);
    await expect(row.getByText("Releasing")).toBeVisible();
    await expect(row.getByRole("button", { name: "Release" })).toHaveCount(0);
    await p.reload();
    await expect(holds.getByRole("row", { name: new RegExp(SETUP_A) }).getByText("Releasing")).toBeVisible();
    asserted("Release on a deployed setup is accepted: \"Release requested for lab-03.\", the row shows Releasing, the Release button is gone and the hold stays after a reload");
    const del = await fetch(`${SIM}/pools/${POOL_A}`, { method: "DELETE" });
    expect(del.status).toBe(204);
    await untilHome(p, `${SETUP_A} released by the poll`, async () => {
      return (await holds.getByRole("row", { name: new RegExp(SETUP_A) }).count()) === 0;
    });
    await expect(p.getByText("Current holds · 1 of 2 allowed")).toBeVisible();
    await nav(p, "Setups");
    const setupRow = p.getByRole("table", { name: "All setups" }).getByRole("row", { name: new RegExp(SETUP_A) });
    await expect(setupRow).toContainText("Free");
    asserted(`after the pool is destroyed and one poll the hold is released without a second click; ${SETUP_A} is Free on Setups; holds 1 of 2`);
  });

  // ---- 8
  await step("A8", "month rollover with a negative balance, the debt carries", A, async () => {
    const p = alice!;
    const a = admin!;
    await reloadHome(p);
    const bal0 = await balance(p);
    observe("balance_before_grant", bal0);
    // Negative grant through the Users page: leave the balance at about -100.
    const amount = -(Math.round(bal0) + 100);
    await a.goto("/admin/users");
    await a.getByRole("button", { name: /^Grant to Alice/ }).click();
    const panel = a.getByRole("complementary", { name: "Grant tokens" });
    await panel.getByLabel("Amount").fill(String(amount));
    await panel.getByLabel("Reason (required)").fill("e2e: make the balance negative");
    await panel.getByRole("button", { name: "Grant tokens", exact: true }).click();
    await expect(a.getByText(/^Granted/)).toBeVisible();
    await reloadHome(p);
    await expect(p.locator('section[aria-label="Tokens left"]')).toContainText("Overdrawn");
    const debt = await balance(p);
    expect(debt).toBeLessThan(-90);
    expect(debt).toBeGreaterThan(-110);
    asserted(`admin grant of ${amount} with a reason leaves alice overdrawn (balance ${debt})`);
    observe("balance_overdrawn", debt);
    const grantLedger = await readLedger(p);
    expect(grantLedger.some((r) => r.type === "Grant" && r.amount === amount)).toBeTruthy();

    // Clock jump: to 03:00 UTC on the 1st of next month. It is an uncharged outage and triggers the rollover.
    const now = new Date((await (await fetch(`${CLOCK}/clock`)).json()).now as string);
    const target = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 1, 3, 0, 0));
    const offsetS = Math.round((target.getTime() - now.getTime()) / 1000);
    const put = await fetch(`${CLOCK}/clock`, { method: "PUT", body: JSON.stringify({ offset: `${offsetS}s` }) });
    expect(put.ok).toBeTruthy();
    observe("clock_jump", `${now.toISOString()} -> ${target.toISOString()}`);
    const periodLabel = new Intl.DateTimeFormat("en-US", { month: "long", year: "numeric", timeZone: "UTC" }).format(target);

    // Sessions are checked against the same clock and may have expired: sign in again if asked.
    const l = await expect
      .poll(
        async () => {
          await p.goto("/usage");
          if (new URL(p.url()).pathname === "/login" || (await p.getByRole("link", { name: "Sign in with SSO" }).count())) {
            await p.getByRole("link", { name: "Sign in with SSO" }).click();
            await finishKeycloak(p, ALICE);
          }
          if (!(await p.getByRole("heading", { name: "My usage" }).isVisible().catch(() => false))) return false;
          await expect(p.getByRole("table", { name: "Ledger" })).toBeVisible();
          const sel = await p.getByLabel("Month").inputValue().catch(() => "");
          const opt = await p.getByLabel("Month").locator("option:checked").innerText().catch(() => "");
          void sel;
          return opt === periodLabel && (await readLedger(p)).some((r) => r.type === "Carryover");
        },
        { message: "waiting for the poll that opens the new month", timeout: 4 * 60_000, intervals: [10_000] },
      )
      .toBeTruthy()
      .then(() => readLedger(p));
    const carry = l.find((r) => r.type === "Carryover")!;
    const allow = l.find((r) => r.type === "Allowance")!;
    expect(allow.amount).toBe(300);
    expect(Math.abs(carry.amount - debt)).toBeLessThan(0.5); // debits between reading the debt and the jump
    const rest = l.filter((r) => r !== carry && r !== allow).reduce((s, r) => s + r.amount, 0);
    const bal = await balance(p);
    expect(Math.abs(bal - (300 + carry.amount + rest))).toBeLessThan(0.005);
    expect(bal).toBeGreaterThan(300 + debt - 5);
    expect(bal).toBeLessThan(300 + debt + 1);
    await expect(p.locator('section[aria-label="Tokens left"]')).toContainText("carried over");
    asserted(`new month ${periodLabel}: ledger has Allowance +300 and Carryover ${carry.amount} (the debt of ${debt}); balance = 300 + carry-over + debits since`);
    asserted("the jump itself is not charged (no debit entry covers the 30-day gap)");
    expect(l.every((r) => Math.abs(r.amount) < 5 || r === allow || r === carry)).toBeTruthy();
    observe("ledger_new_month", l.map((r) => `${r.type} ${r.amount} -> ${r.balance}`));
    observe("balance_after_rollover", bal);
  });

  // ---- mode specific
  await step("A9", MODE === "split" ? "split mode: page and API from different containers, one origin" : "bundled mode: one container serves page and API, one origin", A, async () => {
    const p = alice!;
    const ui = await p.request.get("/ui-healthz");
    const api = await p.request.get("/healthz");
    expect(api.status()).toBe(200);
    observe("healthz", await api.json());
    if (MODE === "split") {
      expect(ui.status()).toBe(200);
      expect(await ui.json()).toEqual({ status: "ok" }); // only the UI container answers /ui-healthz
      expect((await api.json()).status).toBeDefined(); // /healthz is proxied to the API container
      asserted("/ui-healthz is answered by the UI container, /healthz is proxied to the API container, both on the one origin");
      // The API container is not reachable on the UI's port except through the proxy: the UI container has no API of its own.
    } else {
      expect(ui.headers()["content-type"] ?? "").not.toContain("application/json");
      asserted("/ui-healthz does not exist: there is no separate UI container; /healthz and the page share the origin");
    }
    const hosts = [...origins];
    observe("browser_origins", hosts);
    for (const o of hosts) expect([BASE, "http://keycloak.localhost:8081"]).toContain(o);
    asserted("every browser request went to http://localhost:8080 (or Keycloak's login) - the page never called the API on another origin");
  });

  // ---- finish
  const failed = results.filter((r) => r.status === "fail");
  fs.mkdirSync(OUT, { recursive: true });
  fs.writeFileSync(
    path.join(OUT, "steps.json"),
    JSON.stringify({ mode: MODE, seconds: Math.round((Date.now() - t0) / 1000), steps: results }, null, 2),
  );
  if (failed.length > 0) {
    await aliceCtx.tracing.stop({ path: path.join(OUT, "trace-alice.zip") });
    await adminCtx.tracing.stop({ path: path.join(OUT, "trace-admin.zip") });
  } else {
    await aliceCtx.tracing.stop();
    await adminCtx.tracing.stop();
  }
  await aliceCtx.close();
  await adminCtx.close();
  for (const r of results) console.log(`${r.status === "pass" ? "ok  " : "FAIL"} ${r.id} ${r.title} (${r.seconds}s)`);
  expect(failed.map((r) => `${r.id}: ${r.error}`), "failing steps").toEqual([]);
});
