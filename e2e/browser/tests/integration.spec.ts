// SPDX-License-Identifier: MIT
// Integration scenario (docs/design/integration-api.md, "How the pieces fit"): a real Hourglass stack configured
// with the `reported` provider and pressure source and a webhook, driven by a scripted customer service
// (e2e/browser/reporter/reporter.ts) that reports deployments and pressure with a service token and receives
// signed webhooks. Developers act in the browser. Run through `e2e/run.sh integration`.
//
// Steps: I1 service account and token through the UI, I2 first reports and a successful poll, I3 acquire and the
// hold.acquired webhook, I4 reported deployed starts metering, I5 release of a deployed setup is pending
// (Releasing, 202, one hold.releasing, still charged), I6 teardown reported completes the release, I7 the reporter
// goes silent past max_age, I8 service release, I9 negative checks from the reporter's side, I10 the service
// accounts page end to end for a throwaway account (disable, enable, revoke, delete). Reported weights
// (docs/design/reported-weights.md, Tests): W1 the admin gives the account the scope and turns the setting on with a
// maximum, W2 weights for every setup show on the Setups page and in the acquire dialog, W3 a hold is charged at the
// reported weight and the next debit follows a change, W4 the reporter keeps the weight through teardown and the last
// debit is at it, W5 a report over the maximum is refused and shown on Settings, W6 weights stop and the price returns
// to the rate rule after max_age while polls succeed. The Setups page that leads with cost (docs/design/setups-declutter.md):
// U20 the table has the five columns Setup, State, Held by, Cost and actions while display_label_keys holds five keys
// (UI-15) and does not scroll sideways at 1280 px with a 32-character name (UI-20, the only real-browser check of it),
// for a developer and an admin, and the details panel, the only place labels are shown, equals the response (UI-18);
// U22 the Label selector box's suggestions come from label_index, a pick of a key then a value filters through the
// API, a value no selector can express becomes an exact-match token applied in the browser, an invalid selector shows
// the API's message, all also by keyboard (UI-21, UI-22); S1 the lab at 0x and S2 a higher band, with the owner's
// screenshots of the page and the open panel (a Settings save keeps the stored display keys, UI-23); S3 the owner's
// screenshots of the open suggestion list and of an exact-match token.
//
// Driven outside the UI, as the outside world: the reporter and receiver (the customer's service), the e2e clock
// control (to move past max_age without waiting), one Keycloak admin call (token lifetimes, as in the acceptance
// run, D34), and the admin API for three things the UI does not need to prove again or no longer offers: reading the
// collector, audit log and settings, creating a second, scope-limited account for the negative checks (D63), and
// storing display label keys, which no page edits since D96 (U20).
import { expect, test, type APIResponse, type BrowserContext, type Locator, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { Receiver, Reporter, type ReportedWeight } from "../reporter/reporter";

const MODE = "integration";
const OUT = process.env.E2E_OUT ?? path.resolve(__dirname, "..", "..", "out", MODE);
const BASE = process.env.E2E_BASE_URL ?? "http://localhost:8080";
const CLOCK = process.env.E2E_CLOCK_URL ?? "http://localhost:8091";
const RECEIVER_PORT = 9100; // config.integration.yaml points the webhook at host.docker.internal:9100
const WEBHOOK_SECRET = "dev-only-webhook-secret"; // config.integration.yaml

const ADMIN_INITIAL = "dev-only-not-a-secret";
const ADMIN_NEW = "dev-only-e2e-admin-password";
const ALICE = { user: "alice", password: "dev-only-alice" };
const ACCOUNT = "lab-automation";
const SETUP_A = "lab-03"; // weight 2 (size=large)
const SETUP_B = "lab-04";
const MAX_AGE_MIN = 10; // config.integration.yaml
const POLL_WAIT = 9 * 60_000; // the first poll after startup sleeps 5 minutes (default interval)

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

/** A screenshot of the whole page, or with `viewport` of the window only (an open dialog's backdrop covers the window). */
async function shot(page: Page | undefined, label: string, o: { viewport?: boolean } = {}) {
  if (!page || page.isClosed()) return;
  const file = path.join(OUT, "screenshots", `${String(++shots.n).padStart(2, "0")}-${label}.png`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  await page.screenshot({ path: file, fullPage: !o.viewport }).catch(() => undefined);
}

/** Runs one step; a failure is recorded (with a screenshot) and the run goes on. */
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

const nav = (page: Page, name: string) =>
  page.getByRole("navigation", { name: "Main" }).getByRole("link", { name: new RegExp(`^${name}`) }).click();

async function signInSSO(page: Page, who: { user: string; password: string }) {
  await page.goto("/login");
  await page.getByRole("link", { name: "Sign in with SSO" }).click();
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

/** As in the acceptance run: the clock jump must not outlive the Keycloak tokens (D34). Outside world; Hourglass untouched. */
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

type J = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

async function json(res: APIResponse, want?: number): Promise<J> {
  if (want !== undefined) expect(res.status(), `${res.url()}: ${await res.text()}`).toBe(want);
  return (await res.json().catch(() => ({}))) as J;
}

/** Calls the API with a session (cookie plus CSRF token). */
async function api(page: Page, method: "GET" | "POST" | "PUT" | "DELETE", p: string, data?: unknown): Promise<APIResponse> {
  const headers: Record<string, string> = {};
  if (data !== undefined) headers["content-type"] = "application/json";
  if (method !== "GET") headers["X-CSRF-Token"] = (await (await page.request.get("/api/v1/session")).json()).csrf_token;
  return page.request.fetch(p, { method, headers, data: data === undefined ? undefined : JSON.stringify(data) });
}

/** Calls an integration route as the customer's service would: bearer token, no cookie. */
async function svc(token: string | undefined, method: string, p: string, body?: unknown): Promise<{ status: number; body: J }> {
  const headers: Record<string, string> = { "content-type": "application/json" };
  if (token) headers.authorization = `Bearer ${token}`;
  const r = await fetch(`${BASE}${p}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  return { status: r.status, body: (await r.json().catch(() => ({}))) as J };
}

interface Run {
  id: number;
  ok: boolean;
  error: string | null;
  finished_at: string | null;
}
const collector = async (admin: Page): Promise<J> => json(await admin.request.get("/api/v1/admin/collector"), 200);
const latestRunId = async (admin: Page) => Math.max(0, ...((await collector(admin)).recent_runs as Run[]).map((r) => r.id));

/** Waits for the next poll that starts after now and has finished (it read the reports sent before this call). */
async function nextPoll(admin: Page, timeout = POLL_WAIT): Promise<Run> {
  const base = await latestRunId(admin);
  let got: Run | undefined;
  await expect
    .poll(
      async () => {
        got = ((await collector(admin)).recent_runs as Run[]).find((r) => r.id > base && r.finished_at !== null);
        return !!got;
      },
      { message: `waiting for the next collector poll (after run ${base})`, timeout, intervals: [3_000] },
    )
    .toBeTruthy();
  return got!;
}
async function nextOkPoll(admin: Page, timeout = POLL_WAIT): Promise<Run> {
  const r = await nextPoll(admin, timeout);
  expect(r.ok, `poll ${r.id} failed: ${r.error}`).toBeTruthy();
  return r;
}

const myHolds = async (p: Page, state?: "active" | "released"): Promise<J[]> =>
  (await json(await p.request.get(`/api/v1/me/holds${state ? `?state=${state}` : ""}`), 200)).holds;
const myBalance = async (p: Page): Promise<number> => (await json(await p.request.get("/api/v1/me"), 200)).balance_tokens;
const debitsOf = async (p: Page): Promise<J[]> =>
  ((await json(await p.request.get("/api/v1/me/ledger"), 200)).entries as J[]).filter((e) => e.type === "debit");

/** Debits, newest first (by row id). */
const debitsNewestFirst = async (p: Page): Promise<J[]> => (await debitsOf(p)).sort((a, b) => b.id - a.id);
/** Waits until a debit newer than `afterId` exists and returns the newest one. */
async function debitAfter(p: Page, afterId: number): Promise<J> {
  let got: J | undefined;
  await expect
    .poll(async () => ((got = (await debitsNewestFirst(p))[0]), (got?.id ?? 0) > afterId), { message: `a debit after row ${afterId}`, timeout: 4 * 60_000, intervals: [3_000] })
    .toBeTruthy();
  return got!;
}
const newestDebitId = async (p: Page) => (await debitsNewestFirst(p))[0]?.id ?? 0;
const stateOf = async (token: string): Promise<J> => (await svc(token, "GET", "/api/v1/integration/state")).body;
const setupIn = (st: J, name: string): J => (st.setups as J[]).find((x) => x.name === name)!;

// ---------------------------------------------------------------- Setups page (docs/design/setups-declutter.md)

const esc = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const num = (s: string) => parseFloat(s.replace(/,/g, ""));
/** A shown number against its API value: the page prints at most 3 decimals. */
const near = (shown: number, api: number, what: string, tol = 0.0006) =>
  expect(Math.abs(shown - api), `${what}: shown ${shown}, API ${api}`).toBeLessThanOrEqual(tol);

/** The `dd` that follows the `dt` with exactly this text. */
const ddOf = (scope: Locator, dt: string) => scope.locator(`xpath=.//dt[normalize-space(.)="${dt}"]/following-sibling::dd[1]`);

/** Loads /setups and returns the setups response the page itself received, so what is shown and what is compared are one response. */
async function loadSetups(p: Page): Promise<J> {
  const [resp] = await Promise.all([
    p.waitForResponse((r) => new URL(r.url()).pathname === "/api/v1/setups" && r.request().method() === "GET" && r.ok()),
    p.goto("/setups"),
  ]);
  const list = (await resp.json()) as J;
  await expect(p.getByRole("table", { name: "All setups" }).locator("tbody tr")).toHaveCount((list.setups as J[]).length);
  return list;
}

const SOURCE_WORDS: Record<string, string> = {
  rule: "rule (set by a rate rule)",
  default: "default (the default weight)",
  reported: "reported (sent by automation)",
  override: "override (pinned by an admin)",
};

/**
 * Opens the details panel of `name` from the cost cell or from Details and compares its Price, Holder and Labels
 * sections with the setups response `list` the page shows (UI-18; labels are shown here only, R3-3). Optionally takes
 * a screenshot of the viewport with it open, so the backdrop covers the page. Closes it (Escape for the cost cell,
 * Close for Details) and checks that focus is back on the opener.
 */
async function checkPanel(p: Page, list: J, name: string, via: "cost" | "details", shotLabel?: string): Promise<Record<string, string>> {
  const opener =
    via === "cost"
      ? p.getByRole("button", { name: new RegExp(`Price details for ${esc(name)}$`) })
      : p.getByRole("button", { name: `Details for ${name}`, exact: true });
  await opener.click();
  const dlg = p.getByRole("dialog", { name: `Details: ${name}` });
  await expect(dlg).toBeVisible();
  await expect(dlg.getByRole("heading", { level: 3, name: "Price", exact: true })).toBeFocused();
  const s = (list.setups as J[]).find((x) => x.name === name)!;
  const where = `${name} panel (${via})`;
  const texts = (l: Locator) => l.evaluateAll((els) => els.map((e) => (e.textContent ?? "").trim()));
  expect(await texts(dlg.locator("h3")), `${where} sections`).toEqual(s.holder ? ["Price", "Holder", "Labels", "Raw details"] : ["Price", "Labels", "Raw details"]);
  const lines: Record<string, string> = await dlg.evaluate((d) =>
    Object.fromEntries(
      [...d.querySelectorAll("dt")].map((dt) => [dt.textContent!.trim(), (dt.nextElementSibling?.textContent ?? "").replace(/\s+/g, " ").trim()]),
    ),
  );
  const pricing = list.pricing as J;
  const cost = s.cost_per_hour_tokens as J;
  near(num(lines["Weight"]!), s.weight, `${where} weight`);
  expect(lines["Weight"], where).toContain(SOURCE_WORDS[s.weight_source] ?? s.weight_source);
  near(num(lines["Multiplier now"]!), pricing.multiplier, `${where} multiplier`, 0.006);
  if (pricing.pressure === null) expect(lines["Lab pressure"]).toBe("not measured yet");
  else expect(lines["Lab pressure"], `${where} pressure`).toBe(`${Math.round(pricing.pressure * 100)}%`);
  expect(lines["Cost now"], where).toMatch(/^[\d.,]+ tokens \/ h$/);
  near(num(lines["Cost now"]!), cost.now, `${where} cost now`);
  if (cost.now > cost.at_peak) {
    expect(lines["Up to"], where).toMatch(/^The multipliers were changed\./);
  } else {
    const m = /^([\d.,]+) tokens \/ h at the highest multiplier \(([\d.,]+)×\)$/.exec(lines["Up to"] ?? "");
    expect(m, `${where} Up to line: ${lines["Up to"]}`).not.toBeNull();
    near(num(m![1]!), cost.at_peak, `${where} cost at peak`);
    near(num(m![2]!), pricing.peak_multiplier, `${where} peak multiplier`, 0.006);
  }
  // Every label of the setup, and nothing else, in the Labels list, sorted by key.
  const labels = s.labels as Record<string, string>;
  const labelKeys = await texts(dlg.locator('xpath=.//h3[normalize-space(.)="Labels"]/following-sibling::dl[1]/dt'));
  expect(labelKeys, `${where} label keys`).toEqual(Object.keys(labels).sort());
  expect(labelKeys.length, `${where} has labels`).toBeGreaterThan(0);
  for (const [k, v] of Object.entries(labels)) expect(lines[k], `${where} label ${k}`).toBe(v === "" ? "(empty)" : v);
  if (list.reported_weights_enabled) await expect(dlg).toContainText("Weights are reported by automation, so the price can change while the setup is held.");
  if (s.holder) expect(lines["Held by"], where).toBe(s.holder.is_you ? "You" : s.holder.name);
  const noDetails = s.details === null || s.details === undefined || (typeof s.details === "object" && Object.keys(s.details).length === 0);
  if (noDetails) await expect(dlg.getByLabel("Setup details JSON")).toHaveCount(0);
  else await expect(dlg.getByLabel("Setup details JSON")).toBeVisible();
  if (shotLabel) await shot(p, shotLabel, { viewport: true });
  if (via === "cost") await p.keyboard.press("Escape");
  else await dlg.getByRole("button", { name: "Close", exact: true }).click();
  await expect(dlg).toHaveCount(0);
  await expect(opener).toBeFocused();
  return lines;
}

/** The five columns of the Setups table (UI-15): no label column, whatever `display_label_keys` holds. */
const SETUP_HEADERS = ["Setup", "State", "Held by", "Cost (tokens / h)", "Actions"];

/**
 * Measures the Setups table as the page lays it out and checks every row's cost text against `list` (UI-16).
 * Returns the measurements for the caller's UI-15 and UI-20 assertions.
 */
async function measureSetups(p: Page, list: J) {
  const table = p.getByRole("table", { name: "All setups" });
  const m = await table.evaluate((t: HTMLTableElement) => {
    const lines = (el: Element | null) => {
      if (!el) return -1;
      const cs = getComputedStyle(el);
      const lh = cs.lineHeight === "normal" ? parseFloat(cs.fontSize) * 1.2 : parseFloat(cs.lineHeight);
      return Math.round(el.getBoundingClientRect().height / lh);
    };
    // The most lines any one text node inside an element takes, by the distinct tops of its line boxes.
    const textLines = (el: Element | null) => {
      if (!el) return -1;
      const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
      let most = 0;
      for (let n = walk.nextNode(); n; n = walk.nextNode()) {
        if (!(n.textContent ?? "").trim()) continue;
        const range = document.createRange();
        range.selectNodeContents(n);
        most = Math.max(most, new Set([...range.getClientRects()].filter((r) => r.width > 0).map((r) => Math.round(r.top))).size);
      }
      return most;
    };
    const scroller = t.parentElement!;
    return {
      headers: [...t.tHead!.rows[0]!.cells].map((c) => (c.textContent ?? "").trim()),
      scroller: { scrollWidth: scroller.scrollWidth, clientWidth: scroller.clientWidth },
      page: { scrollWidth: document.documentElement.scrollWidth, clientWidth: document.documentElement.clientWidth },
      tableWidth: Math.round(t.getBoundingClientRect().width),
      rows: [...t.tBodies[0]!.rows].map((r) => {
        const cells = [...r.cells];
        const costBtn = r.querySelector("[data-cost]");
        const costText = costBtn?.querySelector('[aria-hidden="true"]') ?? null;
        const held = r.querySelector("[data-held]");
        const nameSpan = cells[0]?.querySelector("span[title]") ?? null;
        return {
          name: (cells[0]?.textContent ?? "").trim(),
          nameTitle: nameSpan?.getAttribute("title") ?? null,
          nameLines: textLines(nameSpan),
          stateLines: textLines(cells[1] ?? null),
          cellCount: cells.length,
          cost: (costText?.textContent ?? "").trim(),
          costTextLines: textLines(costText), // the cost text's nested spans give several client rects on one line
          costLines: lines(costBtn),
          heldLines: held ? lines(held) : 0,
          tagged: (cells[cells.length - 2]?.textContent ?? "").includes("The price comes from"),
        };
      }),
    };
  });
  for (const r of m.rows) {
    const s = (list.setups as J[]).find((x) => x.name === r.name);
    expect(s, `row ${r.name} is in the response`).toBeDefined();
    const { now, at_peak: peak } = s!.cost_per_hour_tokens as J;
    if (now < peak) {
      const g = /^([\d.,]+) now · up to ([\d.,]+)$/.exec(r.cost);
      expect(g, `${r.name} cost cell: "${r.cost}"`).not.toBeNull();
      near(num(g![1]!), now, `${r.name} cost now`);
      near(num(g![2]!), peak, `${r.name} cost at peak`);
    } else {
      expect(r.cost, `${r.name} cost cell (now ${now} >= at_peak ${peak})`).toMatch(/^[\d.,]+$/);
      near(num(r.cost), now, `${r.name} cost now`);
    }
    expect(r.tagged, `${r.name} source tag (weight_source ${s!.weight_source})`).toBe(s!.weight_source === "reported" || s!.weight_source === "override");
  }
  return m;
}

async function acquire(p: Page, setup: string) {
  await nav(p, "Setups");
  await p.getByRole("button", { name: `Acquire ${setup}` }).click();
  await p.getByRole("dialog").getByRole("button", { name: "Acquire", exact: true }).click();
  await expect(p.getByText(`Acquired ${setup}.`)).toBeVisible();
}

// ---------------------------------------------------------------- the test

test("integration: service account, reported deployments and pressure, webhooks, pending release, silence, service release", async ({ browser }) => {
  const t0 = Date.now();
  const receiver = new Receiver(WEBHOOK_SECRET, RECEIVER_PORT);
  await receiver.start();
  let reporter: Reporter | undefined;
  let token = "";
  let offsetS = 0; // the test clock's offset so far

  const aliceCtx: BrowserContext = await browser.newContext({ baseURL: BASE });
  const adminCtx: BrowserContext = await browser.newContext({ baseURL: BASE });
  await aliceCtx.tracing.start({ screenshots: true, snapshots: true });
  await adminCtx.tracing.start({ screenshots: true, snapshots: true });
  const alice = await aliceCtx.newPage();
  const admin = await adminCtx.newPage();
  const A = () => alice;
  const D = () => admin;
  // Every request the admin's browser makes; the dashboard must never call the integration routes (SVC-2, SVC-6).
  const adminRequests: string[] = [];
  admin.on("request", (r) => adminRequests.push(r.url()));
  const assertNoIntegrationRequests = () => {
    expect(adminRequests.length, "the admin browser made requests").toBeGreaterThan(20);
    expect(adminRequests.filter((u) => new URL(u).pathname.startsWith("/api/v1/integration/"))).toEqual([]);
  };

  try {
    // ---- set-up (the admin's first run and alice's approval; proven in detail by the acceptance run)
    await step("I0", "admin first run (poll interval 1m), alice signs in and is approved", D, async () => {
      await adminSignIn(admin);
      await nav(admin, "Settings");
      await admin.getByLabel("Poll interval").fill("1m");
      // The default bands charge nothing while the lab is idle; as in the acceptance run, 1x below 5% pressure.
      await admin.getByLabel("Base multiplier", { exact: true }).fill("1");
      await admin.getByLabel("From pressure 1 (0 to 1)", { exact: true }).fill("0.05");
      await admin.getByLabel("Multiplier 1", { exact: true }).fill("2");
      for (const i of [3, 2]) {
        const rm = admin.getByRole("button", { name: `Remove band ${i}` });
        if (await rm.count()) await rm.click();
      }
      await admin.getByRole("button", { name: "Save changes" }).click();
      await expect(admin.getByText("Settings saved. They apply from the next poll.")).toBeVisible();
      await stretchKeycloakTokenLifetime();
      await signInSSO(alice, ALICE);
      await expect(alice.getByRole("heading", { level: 1 })).toContainText("not authorized");
      await nav(admin, "Users");
      const item = admin.getByRole("listitem").filter({ hasText: "alice@example.com" });
      await expect(item).toBeVisible({ timeout: 90_000 });
      await item.getByRole("group").getByLabel("Developer").check();
      await item.getByRole("button", { name: /^Approve/ }).click();
      await expect(admin.getByRole("listitem").filter({ hasText: "alice@example.com" })).toHaveCount(0);
      await alice.goto("/");
      await expect(alice.getByRole("heading", { name: "My usage" })).toBeVisible();
      const ext = (await collector(admin)).extensions;
      expect(ext.provider).toBe("reported");
      expect(ext.pressure_source).toBe("reported");
      asserted("poll interval 1m saved; alice approved as developer and sees My usage");
      asserted("the collector reports provider and pressure source `reported`");
      observe("extensions", ext);
    });

    // ---- a
    await step("I1", "admin creates a service account with the four scopes and issues a token in the UI", D, async () => {
      await admin.goto("/admin/service-accounts");
      await expect(admin.getByRole("heading", { name: "Service accounts" })).toBeVisible();
      await admin.getByRole("button", { name: "Create service account" }).first().click();
      const dlg = admin.getByRole("dialog", { name: "Create service account" });
      await dlg.getByRole("textbox", { name: "Name" }).fill(ACCOUNT);
      await dlg.getByRole("textbox", { name: "Description" }).fill("scripted reporter of the integration scenario");
      for (const s of ["state:read", "pressure:write", "deployments:write", "holds:release"]) {
        await dlg.getByRole("checkbox", { name: new RegExp(`^${s}`) }).check();
      }
      await dlg.getByRole("button", { name: "Create account" }).click();
      await expect(admin.getByText(`Created ${ACCOUNT}.`)).toBeVisible();
      const item = admin.getByRole("listitem", { name: `Service account ${ACCOUNT}` });
      await expect(item).toBeVisible();
      await item.getByRole("button", { name: `Issue token for ${ACCOUNT}` }).click();
      await admin.getByRole("dialog", { name: `Issue token for ${ACCOUNT}` }).getByRole("button", { name: "Issue token" }).click();
      const shown = admin.getByRole("dialog", { name: `Token for ${ACCOUNT}` });
      await expect(shown.getByText(/only time the token is shown/)).toBeVisible();
      token = await shown.getByRole("textbox", { name: "Token" }).inputValue();
      expect(token).toMatch(/^hgsa_\d+_[0-9a-f]{64}$/);
      await shown.getByRole("button", { name: "Close" }).click();
      await expect(shown).toHaveCount(0);
      const acct = ((await json(await admin.request.get("/api/v1/admin/service-accounts"), 200)).service_accounts as J[]).find((a) => a.name === ACCOUNT)!;
      expect([...acct.scopes].sort()).toEqual(["deployments:write", "holds:release", "pressure:write", "state:read"]);
      expect(acct.tokens).toHaveLength(1);
      const secret = token.split("_")[2]!;
      expect(JSON.stringify(acct)).not.toContain(secret); // the secret is never shown again (SVC-4)
      // Nor is it kept anywhere in the browser: storage, URL, or the page after a reload (SVC-4).
      const stored = () => admin.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
      const gone = async (where: string) => {
        const text = `${await stored()}\n${admin.url()}\n${await admin.content()}\n${await admin.locator("body").innerText()}`;
        expect(text, `the token is absent from ${where}`).not.toContain(secret);
      };
      await gone("storage, URL and page after Close");
      await admin.reload();
      await expect(admin.getByRole("listitem", { name: `Service account ${ACCOUNT}` })).toBeVisible();
      await gone("storage, URL and page after a reload");
      expect(await admin.evaluate(() => document.cookie)).not.toContain(secret);
      asserted("account created with the four scopes; a token of the form hgsa_<id>_<64 hex> shown once in the UI");
      asserted("the account list holds one token and never contains the secret");
      asserted("after Close and after a reload the secret is in neither localStorage, sessionStorage, the URL, a cookie nor the page");
      observe("token_prefix", token.slice(0, 10) + "...");
      reporter = new Reporter(BASE, token);
    });
    if (!reporter) throw new Error("I1 did not produce a token; the rest of the scenario cannot run");
    const rep = reporter;

    // ---- b
    await step("I2", "reporter sends pressure and an empty deployment set; a poll succeeds", D, async () => {
      await rep.report([], 0.02);
      expect(rep.last).toEqual({ pressure: 200, deployments: 200 });
      rep.start();
      const run = await nextOkPoll(admin);
      const st = await svc(token, "GET", "/api/v1/integration/state");
      expect(st.status).toBe(200);
      expect(st.body.poll.last_ok_at).not.toBeNull();
      expect(st.body.pressure.source).toBe("reported");
      expect(st.body.pressure.reported.value).toBeCloseTo(0.02);
      expect(st.body.deployments.provider).toBe("reported");
      const names = (st.body.setups as J[]).map((s) => s.name);
      expect(names).toContain(SETUP_A);
      expect((st.body.setups as J[]).every((s) => s.reported_deployed === false && s.hold === null)).toBeTruthy();
      const c = await collector(admin);
      expect(c.last_pressure_report.service_account).toBe(ACCOUNT);
      expect(c.last_deployments_report.service_account).toBe(ACCOUNT);
      asserted("both report routes answered 200; the next poll succeeded; the state route shows both reports and no hold");
      asserted(`the admin collector shows the last reports as sent by ${ACCOUNT}`);
      // And the admin UI shows them in the collector health (Settings).
      await nav(admin, "Settings");
      for (const title of ["Last pressure report", "Last deployment report"]) {
        const tile = admin.getByText(title, { exact: true }).locator("..");
        await expect(tile, title).toContainText(`received from ${ACCOUNT}; observed`);
        await expect(tile, title).not.toContainText("None received");
      }
      asserted(`the Settings page's collector health shows both last report times, received from ${ACCOUNT}`);
      observe("poll", run);
    });

    // ---- c
    await step("I3", "a developer acquires a setup; the receiver gets hold.acquired", A, async () => {
      await acquire(alice, SETUP_A);
      const ev = await receiver.waitFor("hold.acquired", (e) => e.body.setup?.name === SETUP_A);
      expect(ev).toHaveLength(1);
      expect(ev[0]!.body.user?.email).toBe("alice@example.com");
      expect(ev[0]!.body.hold?.setup).toBe(SETUP_A);
      expect(receiver.badSignatures).toBe(0);
      const st = await svc(token, "GET", "/api/v1/integration/state");
      const s = (st.body.setups as J[]).find((x) => x.name === SETUP_A)!;
      expect(s.hold.user.email).toBe("alice@example.com");
      expect(s.hold.deployed_at).toBeNull();
      asserted("hold.acquired received once, signature verified, for lab-03 and alice; the state route shows the hold, not deployed");
      observe("event", ev[0]!.body);
    });

    // ---- d
    await step("I4", "reporter reports the setup deployed; the poll deploys the hold, metering starts, the balance falls", A, async () => {
      const before = await myBalance(alice);
      await rep.report([SETUP_A]);
      await nextOkPoll(admin);
      const h = (await myHolds(alice, "active")).find((x) => x.setup_name === SETUP_A)!;
      expect(h.deployed).toBe(true);
      expect(h.deployed_at).not.toBeNull();
      expect(h.metering_started_at).not.toBeNull();
      const st = await svc(token, "GET", "/api/v1/integration/state");
      const s = (st.body.setups as J[]).find((x) => x.name === SETUP_A)!;
      expect(s.reported_deployed).toBe(true);
      expect(s.hold.deployed_at).not.toBeNull();
      asserted("after the next poll the hold has deployed_at and metering_started_at; the state route says reported_deployed");
      await nextOkPoll(admin); // the first debit comes one poll after metering started (D19)
      await expect.poll(async () => (await debitsOf(alice)).length, { timeout: 3 * 60_000, intervals: [3_000] }).toBeGreaterThan(0);
      const after = await myBalance(alice);
      expect(after).toBeLessThan(before);
      const d = (await debitsOf(alice))[0]!;
      expect(d.setup_name).toBe(SETUP_A);
      expect(Math.abs(d.amount_tokens)).toBeGreaterThan(0);
      asserted("the balance went down on the poll after metering started, with a debit entry for lab-03");
      observe("balance", { before, after });
      observe("first_debit", d.amount_tokens);
    });

    // ---- e
    await step("I5", "release while deployed is pending: 202, Releasing, one hold.releasing, still charged", A, async () => {
      await alice.goto("/usage");
      const row = alice.getByRole("table", { name: "Current holds" }).getByRole("row", { name: new RegExp(SETUP_A) });
      const [resp] = await Promise.all([
        alice.waitForResponse((r) => /\/api\/v1\/holds\/\d+\/release$/.test(r.url()) && r.request().method() === "POST"),
        row.getByRole("button", { name: "Release" }).click(),
      ]);
      expect(resp.status()).toBe(202);
      await expect(alice.getByText(`Release requested for ${SETUP_A}.`)).toBeVisible();
      await expect(row.getByText("Releasing")).toBeVisible();
      await expect(row.getByRole("button", { name: "Release" })).toHaveCount(0);
      asserted("the Release click got 202; the UI shows Releasing in place of the Release button");
      const ev = await receiver.waitFor("hold.releasing", (e) => e.body.setup?.name === SETUP_A);
      expect(ev).toHaveLength(1);
      expect(ev[0]!.body.hold?.release_requested_at).not.toBeNull();
      // A repeated request is the same 202 and fires nothing (HOLD-6).
      const holdId = ev[0]!.body.hold!.id;
      const again = await api(alice, "POST", `/api/v1/holds/${holdId}/release`);
      expect(again.status()).toBe(202);
      expect(receiver.of("hold.releasing", (e) => e.body.setup?.name === SETUP_A)).toHaveLength(1);
      asserted("a repeated request answered 202; the receiver has one hold.releasing (the count is re-checked in I6, after a later event)");
      // The Setups page shows the same: Releasing in place of the Release button.
      await nav(alice, "Setups");
      const setupRow = alice.getByRole("table", { name: "All setups" }).getByRole("row", { name: new RegExp(SETUP_A) });
      await expect(setupRow.getByText("Releasing")).toBeVisible();
      await expect(setupRow.getByRole("button", { name: /^Release/ })).toHaveCount(0);
      asserted("the Setups page also shows Releasing and no Release button on the held setup");
      // Still active and still charged until the teardown is reported.
      const h0 = (await myHolds(alice, "active")).find((x) => x.id === holdId)!;
      expect(h0.release_requested_at).not.toBeNull();
      const charged0 = h0.charged_tokens as number;
      await nextOkPoll(admin);
      const h1 = (await myHolds(alice, "active")).find((x) => x.id === holdId)!;
      expect(h1.active).toBe(true);
      expect(h1.charged_tokens).toBeGreaterThan(charged0);
      asserted("while the setup is still reported, the hold stays active and its charges keep growing");
      observe("charged", { at_request: charged0, one_poll_later: h1.charged_tokens });
    });

    // ---- f
    await step("I6", "reporter reports the set without the setup; the next poll releases the hold", A, async () => {
      await rep.report([]);
      await nextOkPoll(admin);
      const ev = await receiver.waitFor("hold.released", (e) => e.body.setup?.name === SETUP_A);
      expect(ev).toHaveLength(1);
      // The queue delivers in order, so with hold.released received, a late duplicate hold.releasing would already be here.
      expect(receiver.of("hold.releasing", (e) => e.body.setup?.name === SETUP_A)).toHaveLength(1);
      asserted("with hold.released received (a later event, delivered in order), still exactly one hold.releasing for lab-03");
      expect(ev[0]!.body.data.forced).toBe(false);
      expect(ev[0]!.body.hold?.released_at).not.toBeNull();
      expect((await myHolds(alice, "active")).filter((x) => x.setup_name === SETUP_A)).toHaveLength(0);
      const rel = (await myHolds(alice, "released")).find((x) => x.setup_name === SETUP_A)!;
      expect(rel.released_by.name).toBeTruthy();
      await nav(alice, "Setups");
      await expect(alice.getByRole("table", { name: "All setups" }).getByRole("row", { name: new RegExp(SETUP_A) })).toContainText("Free");
      asserted("after the poll without the setup the hold is released, hold.released received once (forced false), lab-03 is Free");
      observe("hold_released_event", ev[0]!.body);
    });

    // ---- g
    await step("I7", "reporter goes silent past max_age: the poll fails, poll.failed arrives, no debit, admins see the outage", D, async () => {
      // A held, deployed setup is metering when the silence starts.
      await acquire(alice, SETUP_B);
      await rep.report([SETUP_B]);
      await nextOkPoll(admin); // deployed, metering starts
      await nextOkPoll(admin); // first debit
      const hold = (await myHolds(alice, "active")).find((x) => x.setup_name === SETUP_B)!;
      expect(hold.deployed).toBe(true);
      const debits0 = (await debitsOf(alice)).length;
      const charged0 = hold.charged_tokens as number;
      expect(charged0).toBeGreaterThan(0);
      const mark = receiver.mark();
      // Silence, then the clock moves past max_age (the test clock replaces waiting for real minutes).
      rep.goSilent();
      offsetS += (MAX_AGE_MIN + 1) * 60;
      expect((await fetch(`${CLOCK}/clock`, { method: "PUT", body: JSON.stringify({ offset: `${offsetS}s` }) })).ok).toBeTruthy();
      const failed = await nextPoll(admin);
      expect(failed.ok).toBe(false);
      expect(failed.error).toMatch(/older than max_age|max_age/);
      const ev = await receiver.waitFor("poll.failed", () => true, mark);
      expect(ev.length).toBeGreaterThanOrEqual(1);
      asserted("with no report for more than max_age the next poll failed with an error naming max_age; poll.failed was received");
      expect((await debitsOf(alice)).length).toBe(debits0);
      const held = (await myHolds(alice, "active")).find((x) => x.setup_name === SETUP_B)!;
      expect(held.charged_tokens).toBe(charged0);
      asserted("no debit was written and the hold's charges did not change during the silence");
      const c1 = await collector(admin);
      expect(c1.last_error.message).toMatch(/max_age/);
      asserted("the admin collector shows the failed poll and its error");
      observe("poll_failed_error", failed.error);

      // A fresh report makes polls succeed again; the gap becomes an uncharged outage (MET-4).
      await rep.resume();
      const ok = await nextOkPoll(admin);
      const c2 = await collector(admin);
      expect(c2.outages_this_month.count).toBeGreaterThanOrEqual(1);
      expect(c2.outages[0].duration_minutes).toBeGreaterThanOrEqual(MAX_AGE_MIN);
      expect(c2.outages_this_month.minutes_not_charged).toBeGreaterThanOrEqual(MAX_AGE_MIN);
      const worst = Math.max(0, ...(await debitsOf(alice)).slice(0, (await debitsOf(alice)).length - debits0).map((d) => Math.abs(d.amount_tokens)));
      expect(worst).toBeLessThan(0.1); // weight 2 x one minute is 0.033; the 11 minute gap would be 0.37
      asserted("after a fresh report the poll succeeds; the admin collector lists the outage; the gap was not charged");
      observe("recovery_poll", ok);
      observe("outage", c2.outages[0]);
    });

    // ---- h
    await step("I8", "service release of an active, deployed hold, audited under the account's name", A, async () => {
      const st = await svc(token, "GET", "/api/v1/integration/state");
      const s = (st.body.setups as J[]).find((x) => x.name === SETUP_B)!;
      const holdId = s.hold.id as number;
      const mark = receiver.mark();
      const reason = "idle for 48 hours, torn down by automation";
      const rel = await svc(token, "POST", `/api/v1/integration/holds/${holdId}/release`, { reason });
      expect(rel.status, JSON.stringify(rel.body)).toBe(200);
      expect(rel.body.active).toBe(false);
      const ev = await receiver.waitFor("hold.released", (e) => e.body.setup?.name === SETUP_B, mark);
      expect(ev).toHaveLength(1);
      expect(ev[0]!.body.data).toMatchObject({ forced: false, reason, service_account: ACCOUNT });
      const audit = await json(await admin.request.get("/api/v1/admin/audit?action=hold.service_release"), 200);
      const entry = (audit.entries as J[]).find((e) => e.target === `hold:${holdId}`);
      expect(entry, `an audit entry for hold:${holdId} among ${JSON.stringify(audit.entries)}`).toBeDefined();
      expect(entry!.actor.name).toBe(ACCOUNT);
      expect(entry!.after).toMatchObject({ hold_id: holdId, setup: SETUP_B, reason });
      expect((await myHolds(alice, "active")).filter((x) => x.setup_name === SETUP_B)).toHaveLength(0);
      const again = await svc(token, "POST", `/api/v1/integration/holds/${holdId}/release`, { reason });
      expect(again.status).toBe(409);
      asserted("POST integration/holds/{id}/release with a reason answered 200; the hold ended without a teardown check");
      asserted(`hold.released arrived with service_account ${ACCOUNT} and the reason; the audit log shows hold.service_release by ${ACCOUNT} for hold:${holdId}, with the hold id, the setup and the reason`);
      asserted("a second release of the same hold answers 409");
      observe("audit_entry", entry);
      await rep.report([]); // the service has torn it down
    });


    // ---- reported weights (design: docs/design/reported-weights.md, Tests, end-to-end)
    // Rate rules of the seed: size=large weighs 2, so lab-03 and lab-04 cost 2 per hour at 1x without a report.
    const WEIGHT_MAX = 3;
    const attrs03 = { "memory-gb": "512", "storage-gb": "4000", tier: "gold" };
    const allWeights = (w03: number, w04: number): ReportedWeight[] => [
      { setup: "lab-01", weight: 0.8, attributes: { "memory-gb": "128", tier: "silver" } },
      { setup: "lab-02", weight: 0.8, attributes: { "memory-gb": "128", tier: "silver" } },
      { setup: SETUP_A, weight: w03, attributes: attrs03 },
      { setup: SETUP_B, weight: w04, attributes: { "memory-gb": "256", tier: "gold" } },
      { setup: "lab-05", weight: 2.5, attributes: { "memory-gb": "1024", tier: "gold" } },
      { setup: "lab-06", weight: 2.5, attributes: { "memory-gb": "1024", tier: "gold" } },
    ];
    let w3HoldId = 0;

    await step("W1", "admin gives the account weights:write and turns reported weights on with a maximum, in the UI", D, async () => {
      // Without the scope the route is forbidden (D3: no existing account gains it); with the scope and the setting off it is not_configured.
      const noScope = await svc(token, "PUT", "/api/v1/integration/weights", { weights: [] });
      expect(noScope.status, JSON.stringify(noScope.body)).toBe(403);
      expect(noScope.body.error.code).toBe("forbidden");
      asserted("before the grant the four-scope account gets 403 forbidden on the weights route");

      await admin.goto("/admin/service-accounts");
      const item = admin.getByRole("listitem", { name: `Service account ${ACCOUNT}` });
      await item.getByRole("button", { name: `Edit ${ACCOUNT}` }).click();
      const ed = admin.getByRole("dialog", { name: `Edit ${ACCOUNT}` });
      await ed.getByRole("checkbox", { name: /^weights:write/ }).check();
      await ed.getByRole("button", { name: "Save changes" }).click();
      await expect(admin.getByText(`Saved ${ACCOUNT}.`)).toBeVisible();
      const acct = ((await json(await admin.request.get("/api/v1/admin/service-accounts"), 200)).service_accounts as J[]).find((a) => a.name === ACCOUNT)!;
      expect([...acct.scopes].sort()).toEqual(["deployments:write", "holds:release", "pressure:write", "state:read", "weights:write"]);
      asserted("the Service accounts page's scope list offers weights:write; after saving, the account holds exactly the five scopes");

      const off = await svc(token, "PUT", "/api/v1/integration/weights", { weights: [] });
      expect(off.status, JSON.stringify(off.body)).toBe(409);
      expect(off.body.error.code).toBe("not_configured");
      expect(off.body.error.message).toMatch(/reported_weights/);
      asserted("with the scope and the setting off the route answers 409 not_configured naming the setting");

      await nav(admin, "Settings");
      await admin.getByLabel("Accept weights reported by a service account").check();
      await admin.getByLabel("Maximum weight").fill(String(WEIGHT_MAX));
      await admin.getByRole("button", { name: "Save changes" }).click();
      await expect(admin.getByText("Settings saved. They apply from the next poll.")).toBeVisible();
      const rw = (await json(await admin.request.get("/api/v1/admin/settings"), 200)).settings.reported_weights;
      expect(rw).toMatchObject({ enabled: true, min: 0, max: WEIGHT_MAX, max_age: "10m" });
      asserted(`the Settings page saved reported weights on, minimum 0, maximum ${WEIGHT_MAX}, maximum age 10m`);
      const empty = await svc(token, "PUT", "/api/v1/integration/weights", { weights: [] });
      expect(empty.status, JSON.stringify(empty.body)).toBe(200);
      expect(empty.body.accepted).toBe(0);
      asserted("an empty weights report is accepted now");
      observe("reported_weights", rw);
    });

    await step("W2", "the reporter sends weights for every setup; Setups shows the tag on the row and weight and attributes in the details panel before acquire; the dialog warns", A, async () => {
      await rep.setWeights(allWeights(1.15, 0.9));
      expect(rep.last.weights).toBe(200);
      const st = await stateOf(token);
      expect(st.weights).toMatchObject({ enabled: true, min: 0, max: WEIGHT_MAX });
      expect(st.weights.observed_at).not.toBeNull();
      expect(setupIn(st, SETUP_A).weight).toMatchObject({ applied: 1.15, source: "reported", reported: 1.15, not_applied: null, attributes: attrs03 });
      expect((st.setups as J[]).every((s) => s.weight.source === "reported")).toBeTruthy();
      asserted("the state route shows the weights block and a reported weight on every setup, with lab-03's attributes");

      await nav(alice, "Setups");
      await alice.reload(); // the page was loaded before the report; a screen shows what it fetched
      const row = alice.getByRole("table", { name: "All setups" }).getByRole("row", { name: new RegExp(SETUP_A) });
      await expect(row.getByText("reported", { exact: true })).toBeVisible();
      await expect(row).toContainText("Free");
      // The weight and the attributes are in the details panel, opened from the cost cell (UI-12 as amended, WGT-14).
      await row.getByRole("button", { name: new RegExp(`Price details for ${SETUP_A}$`) }).click();
      const panel = alice.getByRole("dialog", { name: `Details: ${SETUP_A}` });
      await expect(ddOf(panel, "Weight")).toHaveText("1.15 · reported (sent by automation)");
      await expect(ddOf(panel, "Attributes")).toHaveText("memory-gb 512 · storage-gb 4000 · tier gold");
      await panel.getByRole("button", { name: "Close", exact: true }).click();
      await expect(panel).toHaveCount(0);
      asserted("before any acquire the Setups row shows lab-03's reported tag; its details panel shows the weight 1.15 with its source and the attributes line");

      await alice.getByRole("button", { name: `Acquire ${SETUP_A}` }).click();
      const dlg = alice.getByRole("dialog");
      await expect(dlg).toContainText("Weights are reported by automation, so the price can change while the setup is held.");
      await expect(dlg).toContainText("Weight 1.15 (reported)");
      await expect(dlg).toContainText("memory-gb 512 · storage-gb 4000 · tier gold");
      asserted("the acquire dialog shows the weight with its source, the attributes and the change warning");
      await shot(alice, "W2-dialog");
      await dlg.getByRole("button", { name: "Acquire", exact: true }).click();
      await expect(alice.getByText(`Acquired ${SETUP_A}.`)).toBeVisible();
      w3HoldId = (setupIn(await stateOf(token), SETUP_A).hold as J).id;
      observe("hold_id", w3HoldId);
    });

    await step("W3", "a hold is charged at the reported weight; the reporter changes it and the next debit follows", A, async () => {
      const mark = receiver.mark();
      const base = await newestDebitId(alice);
      await rep.report([SETUP_A]);
      await nextOkPoll(admin); // deployed, metering starts
      await nextOkPoll(admin); // first debit
      const d1 = await debitAfter(alice, base);
      expect(d1.hold_id).toBe(w3HoldId);
      expect(Math.abs(d1.amount_tokens)).toBeGreaterThan(0);
      expect(d1).toMatchObject({ weight_min: 1.15, weight_max: 1.15, weight_sources: ["reported"] });
      asserted("the first debit of the hold records weight 1.15 and the source reported (not the rule weight 2)");
      const hold = (await myHolds(alice, "active")).find((x) => x.id === w3HoldId)!;
      expect(hold).toMatchObject({ weight: 1.15, weight_source: "reported", weight_attributes: attrs03 });
      await nav(alice, "My usage");
      const hrow = alice.getByRole("table", { name: "Current holds" }).getByRole("row", { name: new RegExp(SETUP_A) });
      await expect(hrow).toContainText("1.15");
      await expect(hrow.getByText("reported", { exact: true })).toBeVisible();
      await expect(alice.getByText(/weight 1\.15 \(reported\)/).first()).toBeVisible();
      asserted("My usage shows the hold at 1.15 with the reported tag, and the ledger debit as 'weight 1.15 (reported)'");

      await rep.setWeights(allWeights(1.4, 0.9));
      const hold2 = (await myHolds(alice, "active")).find((x) => x.id === w3HoldId)!;
      expect(hold2.weight).toBe(1.4);
      asserted("a screen follows the new report at once: the hold shows 1.4 before any poll");
      await nextOkPoll(admin);
      const d2 = await debitAfter(alice, d1.id);
      expect(d2).toMatchObject({ hold_id: w3HoldId, weight_min: 1.4, weight_max: 1.4, weight_sources: ["reported"] });
      asserted("the next debit after the change records weight 1.4, source reported; the earlier debit still says 1.15");
      const earlier = (await debitsOf(alice)).find((e) => e.id === d1.id)!;
      expect(earlier.weight_min).toBe(1.15);
      await alice.reload();
      // By day (the default) one row stands for both debits: the range of their weights (WGT-15); every poll shows each one.
      await expect(alice.getByText(/weight 1\.15 to 1\.40 \(reported\)/).first()).toBeVisible();
      await alice.getByRole("button", { name: "Every poll" }).click();
      await expect(alice.getByText(/weight 1\.4 \(reported\)/).first()).toBeVisible();
      observe("debits", { first: { id: d1.id, amount: d1.amount_tokens, weight: d1.weight_min }, next: { id: d2.id, amount: d2.amount_tokens, weight: d2.weight_min } });
      observe("webhooks_since", receiver.of("hold.acquired", () => true, mark).length);
    });

    await step("W4", "the user releases; the reporter keeps the weight through teardown; the last debit is at the reported weight", A, async () => {
      const before = (await debitsOf(alice)).filter((e) => e.hold_id === w3HoldId).length;
      const mark = receiver.mark();
      await nav(alice, "My usage");
      const row = alice.getByRole("table", { name: "Current holds" }).getByRole("row", { name: new RegExp(SETUP_A) });
      const [resp] = await Promise.all([
        alice.waitForResponse((r) => /\/api\/v1\/holds\/\d+\/release$/.test(r.url()) && r.request().method() === "POST"),
        row.getByRole("button", { name: "Release" }).click(),
      ]);
      expect(resp.status()).toBe(202);
      await expect(row.getByText("Releasing")).toBeVisible();
      // The service tears the setup down; it still reports the weight (weights go first, the deployments follow).
      await rep.report([]);
      expect(rep.last.weights).toBe(200);
      expect(setupIn(await stateOf(token), SETUP_A).hold).not.toBeNull();
      await nextOkPoll(admin);
      await receiver.waitFor("hold.released", (e) => e.body.setup?.name === SETUP_A, mark);
      const mine = (await debitsNewestFirst(alice)).filter((e) => e.hold_id === w3HoldId);
      expect(mine.length, "the poll that completed the release charged its interval (D58)").toBeGreaterThan(before);
      expect(mine[0]).toMatchObject({ weight_min: 1.4, weight_max: 1.4, weight_sources: ["reported"] });
      asserted("the hold was released by the poll after the teardown report, and its last debit is at the reported weight 1.4");
      const rel = (await myHolds(alice, "released")).find((x) => x.id === w3HoldId)!;
      expect(rel).toMatchObject({ weight: 1.4, weight_source: "reported" });
      asserted("the released hold shows the weight and source of its last sample: 1.4, reported");
      expect(setupIn(await stateOf(token), SETUP_A).hold).toBeNull();
      observe("last_debit", mine[0]);
    });

    await step("W5", "a report over the maximum is refused, changes nothing, and appears on the Settings page", D, async () => {
      // A held, deployed setup is charged at its reported weight when the refusal comes.
      await acquire(alice, SETUP_B);
      const base = await newestDebitId(alice);
      await rep.report([SETUP_B]);
      await nextOkPoll(admin);
      await nextOkPoll(admin);
      const d0 = await debitAfter(alice, base);
      expect(d0).toMatchObject({ weight_min: 0.9, weight_max: 0.9, weight_sources: ["reported"] });
      asserted("lab-04 is held and deployed and its latest debit is at the reported weight 0.9");

      await rep.stopWeights(); // no accepted weights report after this point, so the refusal is the later event
      const before = (await stateOf(token)).weights;
      const bad = await svc(token, "PUT", "/api/v1/integration/weights", { weights: [{ setup: SETUP_A, weight: 3.5 }, { setup: SETUP_B, weight: 1 }] });
      expect(bad.status, JSON.stringify(bad.body)).toBe(400);
      expect(bad.body.error).toMatchObject({ code: "validation", field: "weights" });
      expect(bad.body.error.message).toMatch(/weights\[0\] \(setup "lab-03"\).*above the maximum 3/);
      asserted(`the over-maximum report got 400 validation, field weights, naming entry 0 and lab-03: ${bad.body.error.message}`);
      const st = await stateOf(token);
      expect(st.weights.observed_at).toBe(before.observed_at);
      expect(setupIn(st, SETUP_B).weight).toMatchObject({ applied: 0.9, source: "reported" });
      expect(setupIn(st, SETUP_A).weight.reported).toBe(1.4);
      asserted("nothing changed: the stored report's observed_at is the same, lab-04 still applies 0.9 and lab-03 still holds 1.4");

      const c = await collector(admin);
      expect(c.last_weights_refusal).toMatchObject({ service_account: ACCOUNT });
      expect(c.last_weights_refusal.message).toMatch(/above the maximum 3/);
      await nav(admin, "Settings");
      // W1 left the admin on Settings, so the link alone does not fetch the collector again: reload for the refusal.
      await admin.reload();
      await expect(admin.getByText("A weights report was refused after the last accepted one")).toBeVisible();
      await expect(admin.getByText(/above the maximum 3/).first()).toBeVisible();
      await expect(admin.getByText(`received from ${ACCOUNT}; observed`).first()).toBeVisible();
      await shot(admin, "W5-settings");
      asserted("the Settings page shows the refusal warning with the account and message, and the last accepted report");

      await nextOkPoll(admin);
      const d1 = await debitAfter(alice, d0.id);
      expect(d1).toMatchObject({ weight_min: 0.9, weight_max: 0.9, weight_sources: ["reported"] });
      asserted("the next poll succeeded and charged lab-04 at 0.9 again");
      observe("refusal", c.last_weights_refusal);
    });

    await step("W6", "weights stop while deployments and pressure go on; after max_age the price is the rate rule and polls succeed", D, async () => {
      // The weights reporter is still silent (W5). Deployments and pressure keep coming; the clock passes max_age.
      const d0 = (await debitsNewestFirst(alice))[0]!;
      offsetS += (MAX_AGE_MIN + 1) * 60;
      expect((await fetch(`${CLOCK}/clock`, { method: "PUT", body: JSON.stringify({ offset: `${offsetS}s` }) })).ok).toBeTruthy();
      await rep.send(); // the service is alive: fresh deployments and pressure, no weights
      const st = await stateOf(token);
      expect(setupIn(st, SETUP_B).weight).toMatchObject({ applied: 2, source: "rule", reported: 0.9, not_applied: "expired" });
      expect(setupIn(st, SETUP_B).weight.attributes).toEqual({});
      asserted("past max_age the state route shows lab-04 at the rule weight 2 (source rule); the stale 0.9 is listed with not_applied expired");
      const run = await nextOkPoll(admin);
      const d1 = await debitAfter(alice, d0.id);
      expect(d1).toMatchObject({ hold_id: (await myHolds(alice, "active")).find((x) => x.setup_name === SETUP_B)!.id, weight_min: 2, weight_max: 2, weight_sources: ["rule"] });
      asserted("the poll after expiry succeeded and the new debit is at weight 2, source rule");
      const c = await collector(admin);
      // last_error is the latest failed poll ever (I7's); polls succeeding now means a success after it.
      expect(Date.parse(c.last_successful_poll_at)).toBeGreaterThan(Date.parse(c.last_error.at));
      expect((c.recent_runs as Run[])[0]!.ok).toBe(true);
      await nav(alice, "My usage");
      await expect(alice.getByText(/weight 2(?!\.)/).first()).toBeVisible();
      await admin.goto("/admin/setups");
      await expect(admin.getByText("Not applied: the report is older than the maximum age.").first()).toBeVisible();
      asserted("Setup inventory says in words that the reported weight is older than the maximum age; the collector shows no error");
      observe("poll_after_expiry", run);

      // Clean up: the service releases the hold and reports an empty set.
      const hold = setupIn(await stateOf(token), SETUP_B).hold as J;
      const rel = await svc(token, "POST", `/api/v1/integration/holds/${hold.id}/release`, { reason: "end of the weights scenario" });
      expect(rel.status, JSON.stringify(rel.body)).toBe(200);
      await rep.report([]);
    });

    // ---- i
    await step("I9", "negative checks from the reporter's side", D, async () => {
      // The token is not a login: any other route treats it as anonymous (SVC-1).
      for (const p of ["/api/v1/me", "/api/v1/setups", "/api/v1/admin/service-accounts"]) {
        const r = await svc(token, "GET", p);
        expect(r.status, `${p} with a service token`).toBe(401);
      }
      expect((await svc(undefined, "GET", "/api/v1/integration/state")).status).toBe(401);
      // A session cookie is not accepted on integration routes (SVC-2).
      expect((await admin.request.get("/api/v1/integration/state")).status()).toBe(401);
      asserted("the token gets 401 on /me, /setups and an admin route; no token and an admin session cookie get 401 on the state route");
      // A token without the scope gets 403 (SVC-3). Created through the admin API (D63).
      const acct = await json(await api(admin, "POST", "/api/v1/admin/service-accounts", { name: "state-reader", description: "scope check", scopes: ["state:read"] }), 201);
      const issued = await json(await api(admin, "POST", `/api/v1/admin/service-accounts/${acct.id}/tokens`, {}), 201);
      const t = issued.token as string;
      expect((await svc(t, "GET", "/api/v1/integration/state")).status).toBe(200);
      for (const [m, p, b] of [
        ["PUT", "/api/v1/integration/pressure", { value: 0.5 }],
        ["PUT", "/api/v1/integration/deployments", { deployments: [] }],
        ["POST", "/api/v1/integration/holds/1/release", { reason: "no scope" }],
        ["PUT", "/api/v1/integration/weights", { weights: [] }],
      ] as const) {
        const r = await svc(t, m, p, b);
        expect(r.status, `${m} ${p}`).toBe(403);
        expect(r.body.error.code).toBe("forbidden");
      }
      asserted("a token with only state:read reads the state (200) and gets 403 forbidden on pressure, deployments, weights and release");
      // A disabled account is refused on the next request (SVC-5).
      await json(await api(admin, "PUT", `/api/v1/admin/service-accounts/${acct.id}`, { description: "scope check", scopes: ["state:read"], enabled: false }), 200);
      expect((await svc(t, "GET", "/api/v1/integration/state")).status).toBe(401);
      asserted("disabling the account makes its token fail with 401 on the next request");
      assertNoIntegrationRequests();
      asserted("none of the admin browser's requests so far went to /api/v1/integration/");
    });

    // ---- j
    await step("I10", "the admin UI disables, enables, revokes and deletes a throwaway account; its token follows", D, async () => {
      const NAME = "throwaway";
      await admin.goto("/admin/service-accounts");
      await admin.getByRole("button", { name: "Create service account" }).first().click();
      const dlg = admin.getByRole("dialog", { name: "Create service account" });
      await dlg.getByRole("textbox", { name: "Name" }).fill(NAME);
      await dlg.getByRole("checkbox", { name: /^state:read/ }).check();
      await dlg.getByRole("button", { name: "Create account" }).click();
      await expect(admin.getByText(`Created ${NAME}.`)).toBeVisible();
      const item = admin.getByRole("listitem", { name: `Service account ${NAME}` });
      await item.getByRole("button", { name: `Issue token for ${NAME}` }).click();
      await admin.getByRole("dialog", { name: `Issue token for ${NAME}` }).getByRole("button", { name: "Issue token" }).click();
      const shown = admin.getByRole("dialog", { name: `Token for ${NAME}` });
      const t = await shown.getByRole("textbox", { name: "Token" }).inputValue();
      await shown.getByRole("button", { name: "Close" }).click();
      const state = () => svc(t, "GET", "/api/v1/integration/state");
      expect((await state()).status).toBe(200);

      // Edit: disable, then enable again.
      const setEnabled = async (on: boolean) => {
        await item.getByRole("button", { name: `Edit ${NAME}` }).click();
        const ed = admin.getByRole("dialog", { name: `Edit ${NAME}` });
        const box = ed.getByRole("checkbox", { name: "Enabled", exact: true });
        if (on) await box.check();
        else await box.uncheck();
        await ed.getByRole("button", { name: "Save changes" }).click();
        await expect(admin.getByText(`Saved ${NAME}.`)).toBeVisible();
        await expect(item.getByText(on ? "Enabled" : "Disabled", { exact: true })).toBeVisible();
      };
      await setEnabled(false);
      expect((await state()).status).toBe(401);
      await setEnabled(true);
      expect((await state()).status).toBe(200);
      asserted("a token works (200); after the UI disables the account it gets 401; after the UI enables it again it gets 200");

      // Revoke the token.
      await item.getByRole("button", { name: new RegExp(`^Revoke token \\d+ of ${NAME}$`) }).click();
      await admin.getByRole("dialog").getByRole("button", { name: "Revoke token", exact: true }).click();
      await expect(admin.getByText(new RegExp(`^Revoked token \\d+ of ${NAME}\\.`))).toBeVisible();
      expect((await state()).status).toBe(401);
      asserted("after the UI revokes the token it gets 401, though the account is enabled");

      // Delete the account.
      await item.getByRole("button", { name: `Delete ${NAME}` }).click();
      await admin.getByRole("dialog").getByRole("button", { name: "Delete account", exact: true }).click();
      await expect(admin.getByText(`Deleted ${NAME}.`)).toBeVisible();
      await expect(item).toHaveCount(0);
      const left = ((await json(await admin.request.get("/api/v1/admin/service-accounts"), 200)).service_accounts as J[]).map((a) => a.name);
      expect(left).not.toContain(NAME);
      expect(left).toContain(ACCOUNT);
      asserted("the UI deletes the throwaway account; it is gone from the list and the main account remains");
      assertNoIntegrationRequests();
      asserted("through the whole scenario the admin browser made no request to /api/v1/integration/");
    });

    // ---- setups-declutter (docs/design/setups-declutter.md and its amendment; D93, D96, D97). Last, so no earlier
    // step sees the extra labels. Neutral keys (one of 28 characters) and values of exactly 24 characters, one set wide capitals.
    const KEYS = ["key-one", "key-two-longer", "k3", "fourth-key", "a-rather-long-label-key-name"];
    const BASES = ["abcdefghijklmnopqrstuv", "10.200.130.0/24-vlan-1", "host.lab.example.test-", "WWWWWWWWWWMMMMMMMMMMWW", "rack-17-row-c-slot-42-"];
    const LONG_NAME = "setup-name-of-32-characters-long"; // the Setup cell is cut at 32 ch, so the longest name it shows whole
    const HELD = "lab-02";
    const labelsFor = (i: number): Record<string, string> =>
      Object.fromEntries(KEYS.map((k, j) => [k, `${BASES[(i + j) % BASES.length]}${String(i).padStart(2, "0")}`]));
    // UI-22: a value no selector can express (a space and parentheses), on a free setup and on the held one.
    const EXACT_KEY = "group-name";
    const EXACT_VALUE = "Unit B (west)";
    const EXACT_ON = ["lab-01", HELD];

    const fillLabels = async (form: Locator, labels: Record<string, string>) => {
      let i = await form.getByRole("textbox", { name: /^Label key \d+$/ }).count();
      for (const [k, v] of Object.entries(labels)) {
        await form.getByRole("button", { name: "Add label", exact: true }).click();
        i++;
        await form.getByRole("textbox", { name: `Label key ${i}`, exact: true }).fill(k);
        await form.getByRole("textbox", { name: `Label value ${i}`, exact: true }).fill(v);
      }
    };
    /** The stored settings, as the admin API returns them. */
    const storedSettings = async (): Promise<J> => (await json(await admin.request.get("/api/v1/admin/settings"), 200)).settings;

    await step("U20", "five columns whatever display_label_keys holds: at 1280 px the Setups table does not scroll sideways, for a developer and an admin; the panel equals the response", A, async () => {
      expect(LONG_NAME).toHaveLength(32);
      for (let i = 1; i <= 7; i++) for (const v of Object.values(labelsFor(i))) expect(v).toHaveLength(24);
      // The stored display label keys can be set only through the API now (D97 (j)); five of them must not add a column.
      const before = await storedSettings();
      expect((await api(admin, "PUT", "/api/v1/admin/settings", { ...before, display_label_keys: KEYS })).status(), "PUT settings with five display keys").toBe(200);
      expect((await storedSettings()).display_label_keys).toEqual(KEYS);
      await nav(admin, "Settings");
      await expect(admin.getByRole("heading", { name: "Settings" })).toBeVisible();
      await expect(admin.getByLabel("Display label keys")).toHaveCount(0);
      await expect(admin.getByText(/display label keys/i)).toHaveCount(0);
      asserted(`display_label_keys set to ${KEYS.join(", ")} through the admin API; the Settings page has no Display label keys control or text`);
      // The labels, a weight override, a long name and a disabled setup, in Setup inventory.
      await admin.goto("/admin/setups");
      await expect(admin.getByRole("heading", { name: "Setup inventory" })).toBeVisible();
      for (let i = 1; i <= 6; i++) {
        const name = `lab-0${i}`;
        await admin.getByRole("button", { name: `Edit ${name}`, exact: true }).click();
        const form = admin.getByRole("complementary", { name: "Edit setup" });
        await fillLabels(form, labelsFor(i));
        if (name === "lab-05") await form.getByLabel("Weight override").fill("3"); // an `override` tag on one row
        await form.getByRole("button", { name: "Save", exact: true }).click();
        await expect(admin.getByText(`Saved ${name}.`, { exact: true })).toBeVisible();
      }
      await admin.getByRole("button", { name: "Add setup", exact: true }).click();
      const add = admin.getByRole("complementary", { name: "Add setup" });
      await add.getByLabel("Name", { exact: true }).fill(LONG_NAME);
      await fillLabels(add, labelsFor(7));
      await add.getByRole("button", { name: "Save", exact: true }).click();
      await expect(admin.getByText(`Added ${LONG_NAME}.`, { exact: true })).toBeVisible();
      await admin.getByRole("button", { name: "Disable lab-06", exact: true }).click();
      await expect(admin.getByRole("button", { name: "Enable lab-06", exact: true })).toBeVisible();
      asserted(`in Setup inventory the admin gave lab-01 to lab-06 five 24-character label values each and lab-05 a weight override of 3, added ${LONG_NAME} (32 characters) with five values, and disabled lab-06`);

      // Weights for every setup again (W6 stopped them); alice holds one setup, so there is a held row.
      await rep.setWeights(allWeights(1.4, 0.9));
      expect(rep.last.weights).toBe(200);
      await acquire(alice, HELD);

      for (const [who, p, rowsWanted] of [
        ["developer", alice, 6],
        ["admin", admin, 7],
      ] as const) {
        await p.setViewportSize({ width: 1280, height: 900 });
        const list = await loadSetups(p);
        expect(list.display_label_keys, `${who}: the API still returns the stored keys`).toEqual(KEYS);
        expect(list.reported_weights_enabled, who).toBe(true);
        const m = await measureSetups(p, list);
        // Not vacuous: the whole table is there, with its five columns and the rows of every kind.
        expect(m.headers, `${who} headers`).toEqual(SETUP_HEADERS);
        expect(m.rows.length, `${who} rows`).toBe(rowsWanted);
        expect(m.rows.length, `${who} rows equal the response`).toBe((list.setups as J[]).length);
        expect(m.rows.every((r) => r.cellCount === SETUP_HEADERS.length), `${who}: five cells in every row`).toBeTruthy();
        const long = m.rows.find((r) => r.name === LONG_NAME);
        expect(long, `${who}: the ${LONG_NAME.length}-character setup is listed`).toBeDefined();
        expect(long!.nameTitle, `${who}: its full name`).toBe(LONG_NAME);
        expect(m.rows.filter((r) => r.heldLines > 0).length, `${who}: a held row`).toBeGreaterThanOrEqual(1);
        expect(m.rows.some((r) => r.tagged), `${who}: a row with a source tag`).toBeTruthy();
        if (who === "admin") expect(m.rows.map((r) => r.name)).toContain("lab-06");
        else expect(m.rows.map((r) => r.name)).not.toContain("lab-06");
        // No label value is in any row (R3-3).
        const rowText = await p.getByRole("table", { name: "All setups" }).locator("tbody").innerText();
        for (const s of list.setups as J[]) for (const v of Object.values(s.labels as Record<string, string>)) if (v.length > 3) expect(rowText, `${who}: label value ${v} in the table`).not.toContain(v);
        // UI-20: no sideways scroll at 1280 px, neither the table's scroller nor the page.
        expect(m.tableWidth, `${who} table width`).toBeGreaterThan(1000);
        expect(m.scroller.scrollWidth, `${who} table scroller ${JSON.stringify(m.scroller)}`).toBeLessThanOrEqual(m.scroller.clientWidth);
        expect(m.page.scrollWidth, `${who} page ${JSON.stringify(m.page)}`).toBeLessThanOrEqual(m.page.clientWidth);
        for (const r of m.rows) {
          expect(r.nameLines, `${who} ${r.name} name is one line`).toBe(1);
          expect(r.stateLines, `${who} ${r.name} state is one line`).toBe(1);
          expect(r.costTextLines, `${who} ${r.name} cost text is one line`).toBe(1);
          expect(r.costLines, `${who} ${r.name} cost button is one line box`).toBe(1);
          if (r.heldLines > 0) expect(r.heldLines, `${who} ${r.name} Held by first line`).toBe(1);
        }
        asserted(`${who}: exactly the headers ${SETUP_HEADERS.join(", ")} while display_label_keys holds five keys; ${m.rows.length} rows of five cells, among them ${LONG_NAME}, a held row and a tagged row; no label value in the table; at 1280 px the table (${m.tableWidth} px) and the page do not scroll sideways`);
        asserted(`${who}: every name, state, cost button and Held by first line is one line box; each cost cell matches now and at_peak of the response and carries a tag exactly for reported and override`);
        observe(`${who}_widths`, { table: m.tableWidth, ...m.scroller, page: m.page });
        await shot(p, `U20-setups-${who}`);
        // The panel from either opener, compared with the response the page shows (UI-18): every label is there.
        await checkPanel(p, list, HELD, "cost");
        await checkPanel(p, list, "lab-05", "details");
        asserted(`${who}: the panel opens from the cost cell (${HELD}) and from Details (lab-05) with focus on Price; sections Price, Holder (held only), Labels, Raw details; weight, source, pressure, multiplier, both costs and every label equal GET /api/v1/setups as the page received it; focus returns to the opener`);
      }
    });

    await step("U22", "the Label selector box suggests from label_index; a value no selector can express becomes an exact-match token; keyboard only; an invalid selector shows the API's error", A, async () => {
      // The value with a space and parentheses, in Setup inventory.
      await admin.goto("/admin/setups");
      await expect(admin.getByRole("heading", { name: "Setup inventory" })).toBeVisible();
      for (const name of EXACT_ON) {
        await admin.getByRole("button", { name: `Edit ${name}`, exact: true }).click();
        const form = admin.getByRole("complementary", { name: "Edit setup" });
        await fillLabels(form, { [EXACT_KEY]: EXACT_VALUE });
        await form.getByRole("button", { name: "Save", exact: true }).click();
        await expect(admin.getByText(`Saved ${name}.`, { exact: true })).toBeVisible();
      }
      const p = alice;
      await p.setViewportSize({ width: 1280, height: 900 });
      const list = await loadSetups(p);
      const index = list.label_index as { key: string; values: { value: string; selector: string | null }[] }[];
      const exactEntry = index.find((k) => k.key === EXACT_KEY)?.values.find((v) => v.value === EXACT_VALUE);
      expect(exactEntry, `label_index holds ${EXACT_KEY}=${EXACT_VALUE}`).toBeDefined();
      expect(exactEntry!.selector, "the value has no selector term").toBeNull();
      const all = (list.setups as J[]).length;
      // Every request for the setups list from here on: none may carry the exact value.
      const sent: string[] = [];
      const onRequest = (r: { url(): string }) => {
        const u = new URL(r.url());
        if (u.pathname === "/api/v1/setups") sent.push(u.search);
      };
      p.on("request", onRequest);
      const answered = (sel: string) =>
        p.waitForResponse((r) => {
          const u = new URL(r.url());
          return u.pathname === "/api/v1/setups" && (u.searchParams.get("selector") ?? "") === sel;
        });
      try {
        const rows = p.getByRole("table", { name: "All setups" }).locator("tbody tr");
        const box = p.getByRole("combobox", { name: "Label selector" });
        const lb = p.getByRole("listbox", { name: "Label suggestions" });
        const tokens = p.getByRole("list", { name: "Exact match:" });
        /** What the open list shows: per group, each option's left text and accessible name. */
        const listed = () =>
          lb.evaluate((el) =>
            [...el.querySelectorAll('[role="group"]')].map((g) => ({
              caption: document.getElementById(g.getAttribute("aria-labelledby") ?? "")?.textContent ?? "",
              options: [...g.querySelectorAll('[role="option"]')].map((o) => ({ text: o.firstElementChild?.textContent ?? "", name: o.getAttribute("aria-label") ?? o.textContent ?? "" })),
            })),
          );

        // Suggestions on click: every key of label_index, in its order, with the first three values as the note.
        await box.click();
        await expect(box).toHaveAttribute("aria-expanded", "true");
        await expect(lb.getByRole("group", { name: "Label keys" })).toBeVisible();
        const keysShown = await listed();
        expect(keysShown.map((g) => g.caption)).toEqual(["Label keys"]);
        expect(keysShown[0]!.options.map((o) => o.text), "the keys listed are label_index's").toEqual(index.map((k) => k.key));
        for (const [i, k] of index.entries()) {
          const sample = k.values.slice(0, 3).map((v) => (v.value === "" ? "(empty)" : v.value)).join(", ") + (k.values.length > 3 ? ", …" : "");
          expect(keysShown[0]!.options[i]!.name, `key option ${k.key}`).toBe(`${k.key}, values: ${sample}`);
        }
        await shot(p, "U22-keys-on-click", { viewport: true });
        await box.press("Escape");
        await expect(lb).toHaveCount(0);
        asserted(`a click in the empty box lists the ${index.length} keys of label_index in its order, each with its first three values`);

        // Suggestions on typing; pick of a key, then of a value: the API filters.
        await box.pressSequentially("key-o");
        await expect(lb.getByRole("group", { name: "Label keys" }).getByRole("option")).toHaveCount(1);
        await lb.getByRole("option", { name: /^key-one, values: / }).click();
        await expect(box).toHaveValue("key-one=");
        await expect(lb.getByRole("group", { name: "Values of key-one" })).toBeVisible();
        const k1 = index.find((k) => k.key === "key-one")!;
        const valuesShown = await listed();
        expect(valuesShown.map((g) => g.caption)).toEqual(["Values of key-one"]);
        expect(valuesShown[0]!.options.map((o) => o.text), "the values listed are label_index's for key-one").toEqual(k1.values.map((v) => v.value));
        const target = (list.setups as J[]).find((s) => s.name === "lab-03")!;
        const v3 = (target.labels as Record<string, string>)["key-one"]!;
        expect(k1.values.find((v) => v.value === v3)?.selector, "lab-03's key-one value has a term").toBe(`key-one=${v3}`);
        let got = answered(`key-one=${v3}`);
        await lb.getByRole("option", { name: v3, exact: true }).click();
        expect((await got).ok()).toBeTruthy();
        await expect(box).toHaveValue(`key-one=${v3}`);
        await expect(rows).toHaveCount(1);
        await expect(rows.first()).toContainText("lab-03");
        await expect(p.getByText("1 setup shown", { exact: true })).toBeVisible();
        asserted(`typing key-o lists key-one alone; picking it gives key-one= and the list Values of key-one with exactly label_index's values; picking ${v3} asks the API with key-one=${v3} and lists lab-03 only`);
        await p.getByRole("button", { name: "Clear filters" }).click();
        await expect(rows).toHaveCount(all);
        await expect(box).toHaveValue("");
        await expect(box).toBeFocused();
        asserted("Clear filters empties the box, lists every setup again and leaves focus in the box");

        // The exact-match token.
        const before = sent.length;
        await box.pressSequentially(`${EXACT_KEY}=`);
        const exactOption = lb.getByRole("group", { name: `Values of ${EXACT_KEY}` }).getByRole("option", { name: `${EXACT_VALUE}, exact match`, exact: true });
        await expect(exactOption).toBeVisible();
        await exactOption.click();
        const token = tokens.getByRole("listitem");
        await expect(token).toHaveCount(1);
        await expect(token).toContainText(`${EXACT_KEY}=${EXACT_VALUE}`);
        await expect(box).toHaveValue("");
        await expect(box).toBeFocused();
        await expect(rows).toHaveCount(EXACT_ON.length);
        expect((await rows.allInnerTexts()).map((t) => t.split(/\s/)[0]).sort()).toEqual([...EXACT_ON].sort());
        await expect(p.getByText(`${EXACT_ON.length} setups shown`, { exact: true })).toBeVisible();
        await expect(p.getByRole("button", { name: "Clear filters" })).toBeVisible();
        asserted(`picking "${EXACT_VALUE}, exact match" after ${EXACT_KEY}= adds the token ${EXACT_KEY}=${EXACT_VALUE} after "Exact match:", empties the box (no term for that key) and lists ${EXACT_ON.join(" and ")} only`);
        // With State set to Free the row count follows: only the free one of the two.
        await p.getByLabel("State").selectOption("free");
        await expect(rows).toHaveCount(1);
        await expect(rows.first()).toContainText("lab-01");
        await expect(p.getByText("1 setup shown", { exact: true })).toBeVisible();
        await p.getByLabel("State").selectOption("");
        await expect(rows).toHaveCount(EXACT_ON.length);
        asserted("with the token and State Free only lab-01 is listed (1 setup shown); State back to All states lists both again");
        // With a contradicting selector nothing is left; the token is still there to remove.
        got = answered("size=large");
        await box.fill("size=large");
        await box.press("Enter");
        expect((await got).ok()).toBeTruthy();
        await expect(rows).toHaveCount(1);
        await expect(rows.first()).toHaveText("No setups match the filters.");
        await expect(token).toHaveCount(1);
        got = answered("");
        await box.fill("");
        await box.press("Enter");
        expect((await got).ok()).toBeTruthy();
        await expect(rows).toHaveCount(EXACT_ON.length);
        asserted("the token with the selector size=large leaves no row: \"No setups match the filters.\", the token still shown; an empty selector brings both rows back");
        const leaked = sent.slice(before).filter((q) => /unit|west/i.test(decodeURIComponent(q.replace(/\+/g, " "))));
        expect(leaked, `requests after the pick: ${JSON.stringify(sent.slice(before))}`).toEqual([]);
        asserted(`no request for the setups list carried the exact value (${sent.length - before} requests checked)`);
        // Removing the token brings every row back; focus goes to the box.
        await p.getByRole("button", { name: `Remove exact-match filter ${EXACT_KEY}=${EXACT_VALUE}`, exact: true }).click();
        await expect(tokens).toHaveCount(0);
        await expect(rows).toHaveCount(all);
        await expect(box).toBeFocused();
        await expect(p.getByRole("button", { name: "Clear filters" })).toHaveCount(0);
        asserted("the token's remove button takes it away: every setup is listed, focus is in the box, no filter is in force");

        // An invalid selector shows the API's own message under the box and keeps the last rows.
        const bad = "key-one in ()";
        const apiSays = await alice.request.get(`/api/v1/setups?selector=${encodeURIComponent(bad)}`);
        expect(apiSays.status()).toBe(400);
        const err = (await apiSays.json()).error;
        expect(err.code).toBe("invalid_selector");
        got = answered(bad);
        await box.fill(bad);
        await box.press("Enter");
        expect((await got).status()).toBe(400);
        await expect(p.getByText(err.message, { exact: true })).toBeVisible();
        await expect(p.getByText(/^Forms: key=value, key!=value, key in \(a,b\), key notin \(a,b\), key, !key\./)).toBeVisible();
        await expect(box).toHaveAttribute("aria-invalid", "true");
        await expect(rows).toHaveCount(all);
        observe("invalid_selector_message", err.message);
        got = answered("");
        await box.fill("");
        await box.press("Enter");
        expect((await got).ok()).toBeTruthy();
        await expect(p.getByText(err.message, { exact: true })).toHaveCount(0);
        await expect(box).not.toHaveAttribute("aria-invalid", "true");
        asserted(`"${bad}" with Enter: the API's message "${err.message}" and the forms line under the box, the box marked invalid, every row still listed; an empty selector clears the error`);

        // Keyboard only: Tab into the box (no list), Arrow Down and Enter pick a key and the exact value; Tab reaches
        // the token's button; Escape closes the list and keeps the text; Enter with no active option submits.
        await p.getByLabel("State").focus();
        await p.keyboard.press("Tab");
        await expect(box).toBeFocused();
        await expect(box).toHaveAttribute("aria-expanded", "false");
        await expect(lb).toHaveCount(0);
        await p.keyboard.press("ArrowDown");
        await expect(lb).toBeVisible();
        const firstId = await lb.getByRole("option").first().getAttribute("id");
        await expect(box).toHaveAttribute("aria-activedescendant", firstId!);
        await p.keyboard.type(EXACT_KEY.slice(0, 7));
        await expect(box).not.toHaveAttribute("aria-activedescendant", /.+/);
        await p.keyboard.press("ArrowDown");
        await p.keyboard.press("Enter");
        await expect(box).toHaveValue(`${EXACT_KEY}=`);
        await expect(lb.getByRole("group", { name: `Values of ${EXACT_KEY}` })).toBeVisible();
        await p.keyboard.press("ArrowDown");
        await expect(box).toHaveAttribute("aria-activedescendant", (await lb.getByRole("option", { name: `${EXACT_VALUE}, exact match`, exact: true }).getAttribute("id"))!);
        await p.keyboard.press("Enter");
        await expect(tokens.getByRole("listitem")).toHaveCount(1);
        await expect(box).toHaveValue("");
        await expect(rows).toHaveCount(EXACT_ON.length);
        await p.keyboard.press("Tab");
        await expect(p.getByRole("button", { name: "Filter", exact: true })).toBeFocused();
        await p.keyboard.press("Tab");
        const remove = p.getByRole("button", { name: `Remove exact-match filter ${EXACT_KEY}=${EXACT_VALUE}`, exact: true });
        await expect(remove).toBeFocused();
        await p.keyboard.press("Enter");
        await expect(tokens).toHaveCount(0);
        await expect(box).toBeFocused();
        await expect(rows).toHaveCount(all);
        await p.keyboard.type("size=medium");
        await expect(lb).toBeVisible();
        await p.keyboard.press("Escape");
        await expect(lb).toHaveCount(0);
        await expect(box).toHaveValue("size=medium");
        got = answered("size=medium");
        await p.keyboard.press("Enter");
        expect((await got).ok()).toBeTruthy();
        await expect(rows).toHaveCount(2);
        await p.getByRole("button", { name: "Clear filters" }).click();
        await expect(rows).toHaveCount(all);
        asserted("keyboard only: Tab into the box opens nothing; Arrow Down opens the keys with the first active; typing clears the active option; Arrow Down and Enter pick the key, then the exact value (token added); Tab reaches Filter, then the token's button, whose Enter removes it and returns focus to the box; Escape closes the list and keeps size=medium; Enter submits it (2 rows)");
      } finally {
        p.off("request", onRequest);
      }
    });

    await step("S1", "the lab at 0x: base multiplier 0 and a third band; after the next poll the rows read 0.0 now and the panel 0x", A, async () => {
      const keysBefore = (await storedSettings()).display_label_keys;
      await nav(admin, "Settings");
      await admin.getByLabel("Base multiplier", { exact: true }).fill("0");
      await admin.getByRole("button", { name: "Add band", exact: true }).click();
      await admin.getByLabel("From pressure 2 (0 to 1)", { exact: true }).fill("0.5");
      await admin.getByLabel("Multiplier 2", { exact: true }).fill("3");
      await admin.getByRole("button", { name: "Save changes" }).click();
      await expect(admin.getByText("Settings saved. They apply from the next poll.")).toBeVisible();
      expect(keysBefore).toEqual(KEYS);
      expect((await storedSettings()).display_label_keys, "display_label_keys after a save on the Settings page").toEqual(KEYS);
      asserted(`a save on the Settings page kept the stored display_label_keys (${KEYS.join(", ")}) unchanged (UI-23)`);
      await rep.report([HELD]); // the held setup is deployed and metering, for the screenshots
      await expect
        .poll(
          async () => {
            const l = await json(await alice.request.get("/api/v1/setups"), 200);
            return l.pricing.multiplier === 0 && (l.setups as J[]).find((s) => s.name === HELD)?.deployed === true;
          },
          { message: "a poll at 0x with the held setup deployed", timeout: POLL_WAIT, intervals: [5_000] },
        )
        .toBeTruthy();
      // 1280 wide and tall enough for the whole panel (UX acceptance P5).
      await alice.setViewportSize({ width: 1280, height: 1100 });
      const list = await loadSetups(alice);
      expect(list.pricing).toMatchObject({ multiplier: 0, peak_multiplier: 3 });
      expect((list.setups as J[]).every((s) => s.cost_per_hour_tokens.now === 0 && s.cost_per_hour_tokens.at_peak > 0)).toBeTruthy();
      const m = await measureSetups(alice, list);
      expect(m.headers).toEqual(SETUP_HEADERS);
      expect(m.rows.length).toBe((list.setups as J[]).length);
      expect(m.rows.every((r) => /^0\.0 now · up to [\d.,]+$/.test(r.cost)), JSON.stringify(m.rows.map((r) => r.cost))).toBeTruthy();
      expect(m.scroller.scrollWidth).toBeLessThanOrEqual(m.scroller.clientWidth);
      asserted("at 0x every cost cell reads `0.0 now · up to X`, X the response's at_peak at the 3x band; five headers; the table still does not scroll sideways");
      observe("costs_0x", m.rows.map((r) => `${r.name}: ${r.cost}`));
      observe("pricing_0x", list.pricing);
      await shot(alice, "S1-setups-0x");
      const lines = await checkPanel(alice, list, HELD, "cost", "S1-panel-0x");
      expect(lines["Multiplier now"]).toBe("0×");
      expect(lines["Cost now"]).toBe("0.0 tokens / h");
      expect(lines["Up to"]).toMatch(/\(3×\)$/);
      expect(lines["Charging"]).toMatch(/^Since /);
      asserted(`the panel of ${HELD} shows Multiplier now 0×, Cost now 0.0 tokens / h, Up to at the highest multiplier (3×), Charging Since, and every label`);
      observe("panel_0x", lines);
    });

    await step("S2", "a higher band: the reporter sends 0.95; after a poll the smoothed pressure is in the 2x band and the panel shows 2x", A, async () => {
      await rep.report([HELD], 0.95);
      let pricing: J = {};
      await expect
        .poll(
          async () => ((pricing = (await json(await alice.request.get("/api/v1/setups"), 200)).pricing), pricing.multiplier),
          { message: "a poll in the 2x band", timeout: 6 * 60_000, intervals: [5_000] },
        )
        .toBe(2);
      await alice.setViewportSize({ width: 1280, height: 1100 });
      const list = await loadSetups(alice);
      expect(list.pricing.multiplier, "still 2x when the page loaded").toBe(2);
      expect(list.pricing.pressure).toBeGreaterThanOrEqual(0.05);
      expect(list.pricing.pressure).toBeLessThan(0.5);
      const m = await measureSetups(alice, list);
      expect(m.headers).toEqual(SETUP_HEADERS);
      expect(m.rows.every((r) => / now · up to /.test(r.cost) && !r.cost.startsWith("0.0 ")), JSON.stringify(m.rows.map((r) => r.cost))).toBeTruthy();
      expect(m.scroller.scrollWidth).toBeLessThanOrEqual(m.scroller.clientWidth);
      asserted(`in the 2x band (smoothed pressure ${list.pricing.pressure}) every cost cell reads "N now · up to M" with the response's values; no sideways scroll`);
      observe("costs_2x", m.rows.map((r) => `${r.name}: ${r.cost}`));
      observe("pricing_2x", list.pricing);
      await shot(alice, "S2-setups-2x");
      const lines = await checkPanel(alice, list, HELD, "cost", "S2-panel-2x");
      expect(lines["Multiplier now"]).toBe("2×");
      asserted(`the panel of ${HELD} shows Multiplier now 2×, the lab pressure, both costs and the 3x highest multiplier of the same response`);
      observe("panel_2x", lines);
    });

    await step("S3", "the owner's shots of the selector box: the list open on Label keys, and an exact-match token with a selector, the count and Clear filters", A, async () => {
      const p = alice;
      await p.setViewportSize({ width: 1280, height: 900 });
      const list = await loadSetups(p);
      const box = p.getByRole("combobox", { name: "Label selector" });
      const lb = p.getByRole("listbox", { name: "Label suggestions" });
      await box.click();
      await expect(lb.getByRole("group", { name: "Label keys" })).toBeVisible();
      await shot(p, "S3-list-open", { viewport: true });
      await box.press("Escape");
      await box.pressSequentially(`${EXACT_KEY}=`);
      await lb.getByRole("option", { name: `${EXACT_VALUE}, exact match`, exact: true }).click();
      await expect(p.getByRole("list", { name: "Exact match:" }).getByRole("listitem")).toHaveCount(1);
      const rows = p.getByRole("table", { name: "All setups" }).locator("tbody tr");
      const answered = p.waitForResponse((r) => {
        const u = new URL(r.url());
        return u.pathname === "/api/v1/setups" && u.searchParams.get("selector") === "size=medium";
      });
      await box.pressSequentially("size=medium");
      await box.press("Escape");
      await box.press("Enter");
      expect((await answered).ok()).toBeTruthy();
      await expect(rows).toHaveCount(EXACT_ON.length);
      await expect(p.getByText(`${EXACT_ON.length} setups shown`, { exact: true })).toBeVisible();
      await expect(p.getByRole("button", { name: "Clear filters" })).toBeVisible();
      await p.getByRole("heading", { name: "Setups", level: 1 }).click(); // blur, so no list or focus ring covers the line
      await shot(p, "S4-token-and-selector", { viewport: true });
      asserted(`shots of the list open on Label keys in an empty box, and of the token ${EXACT_KEY}=${EXACT_VALUE} with size=medium in the box, ${EXACT_ON.length} setups shown and Clear filters (pricing multiplier ${list.pricing.multiplier}×)`);
      await p.getByRole("button", { name: "Clear filters" }).click();
    });
  } finally {
    reporter?.stop();
    await receiver.stop();
    const failed = results.filter((r) => r.status === "fail");
    fs.mkdirSync(OUT, { recursive: true });
    fs.writeFileSync(
      path.join(OUT, "steps.json"),
      JSON.stringify(
        { mode: MODE, seconds: Math.round((Date.now() - t0) / 1000), webhooks: receiver.events.map((e) => `${e.at} ${e.type}`), badSignatures: receiver.badSignatures, steps: results },
        null,
        2,
      ),
    );
    await aliceCtx.tracing.stop(failed.length ? { path: path.join(OUT, "trace-alice.zip") } : undefined);
    await adminCtx.tracing.stop(failed.length ? { path: path.join(OUT, "trace-admin.zip") } : undefined);
    await aliceCtx.close();
    await adminCtx.close();
    for (const r of results) console.log(`${r.status === "pass" ? "ok  " : "FAIL"} ${r.id} ${r.title} (${r.seconds}s)`);
  }
  expect(receiver.badSignatures, "webhooks with a bad signature").toBe(0);
  expect(results.filter((r) => r.status === "fail").map((r) => `${r.id}: ${r.error}`), "failing steps").toEqual([]);
});
