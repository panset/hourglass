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
// to the rate rule after max_age while polls succeed.
//
// Driven outside the UI, as the outside world: the reporter and receiver (the customer's service), the e2e clock
// control (to move past max_age without waiting), one Keycloak admin call (token lifetimes, as in the acceptance
// run, D34), and the admin API for two things the UI does not need to prove again: reading the collector and audit
// log, and creating a second, scope-limited account for the negative checks (D63).
import { expect, test, type APIResponse, type BrowserContext, type Page } from "@playwright/test";
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

async function shot(page: Page | undefined, label: string) {
  if (!page || page.isClosed()) return;
  const file = path.join(OUT, "screenshots", `${String(++shots.n).padStart(2, "0")}-${label}.png`);
  fs.mkdirSync(path.dirname(file), { recursive: true });
  await page.screenshot({ path: file, fullPage: true }).catch(() => undefined);
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

    await step("W2", "the reporter sends weights for every setup; Setups shows weight, tag and attributes before acquire; the dialog warns", A, async () => {
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
      await expect(row).toContainText("1.15");
      await expect(row.getByText("reported", { exact: true })).toBeVisible();
      await expect(row).toContainText("memory-gb 512 · storage-gb 4000 · tier gold");
      await expect(row).toContainText("Free");
      asserted("before any acquire the Setups page shows lab-03's weight 1.15, the reported tag and the attributes line");

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
