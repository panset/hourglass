// SPDX-License-Identifier: MIT
// The scripted "customer service" of the integration scenario (e2e/browser/tests/integration.spec.ts).
//
// It plays both halves of docs/integrating.md without any Hourglass code:
//   - a webhook receiver that verifies X-Hourglass-Signature (HMAC-SHA256 of the raw body, hex, "sha256=" prefix)
//     and records every event it accepts;
//   - a reporter that sends the full set of deployed setups and the lab pressure with its service token, once on
//     command and then every few seconds like a real service would (a heartbeat), until it is told to go silent.
// Reports carry no observed_at, so Hourglass stamps them with its own (movable) clock.
import crypto from "node:crypto";
import http from "node:http";

export interface ReceivedEvent {
  type: string;
  at: number; // receipt order, 1-based
  headerType: string;
  body: {
    type: string;
    time: string;
    user: { username: string; email: string } | null;
    setup: { name: string } | null;
    hold: { id: number; setup: string; release_requested_at: string | null; released_at: string | null } | null;
    data: Record<string, unknown>;
  };
}

export class Receiver {
  readonly events: ReceivedEvent[] = [];
  /** Requests refused because the signature did not verify (must stay 0). */
  badSignatures = 0;
  private seq = 0;
  private server?: http.Server;

  constructor(private readonly secret: string, private readonly port: number) {}

  start(): Promise<void> {
    this.server = http.createServer((req, res) => {
      const chunks: Buffer[] = [];
      req.on("data", (c: Buffer) => chunks.push(c));
      req.on("end", () => {
        const raw = Buffer.concat(chunks);
        const want = "sha256=" + crypto.createHmac("sha256", this.secret).update(raw).digest("hex");
        const got = String(req.headers["x-hourglass-signature"] ?? "");
        const ok = got.length === want.length && crypto.timingSafeEqual(Buffer.from(got), Buffer.from(want));
        if (req.method !== "POST" || !ok) {
          this.badSignatures++;
          res.writeHead(401).end();
          return;
        }
        this.events.push({
          type: String(req.headers["x-hourglass-event"] ?? ""),
          headerType: String(req.headers["x-hourglass-event"] ?? ""),
          at: ++this.seq,
          body: JSON.parse(raw.toString("utf8")),
        });
        res.writeHead(200).end();
      });
    });
    return new Promise((resolve, reject) => {
      this.server!.once("error", reject);
      this.server!.listen(this.port, "0.0.0.0", () => resolve());
    });
  }

  stop(): Promise<void> {
    return new Promise((resolve) => (this.server ? this.server.close(() => resolve()) : resolve()));
  }

  /** Events of a type that arrived after `after` (a value of `mark()`), optionally filtered. */
  of(type: string, pred: (e: ReceivedEvent) => boolean = () => true, after = 0): ReceivedEvent[] {
    return this.events.filter((e) => e.type === type && e.at > after && pred(e));
  }

  /** A position in the event stream; later calls to `of` can ask for what came after it. */
  mark(): number {
    return this.seq;
  }

  /** Waits for at least one matching event; returns all matches so far. */
  async waitFor(type: string, pred: (e: ReceivedEvent) => boolean = () => true, after = 0, timeoutMs = 90_000): Promise<ReceivedEvent[]> {
    const end = Date.now() + timeoutMs;
    for (;;) {
      const m = this.of(type, pred, after);
      if (m.length > 0) return m;
      if (Date.now() > end) throw new Error(`no ${type} webhook within ${timeoutMs / 1000} s; received: ${this.events.map((e) => e.type).join(", ") || "nothing"}`);
      await new Promise((r) => setTimeout(r, 500));
    }
  }
}

export class Reporter {
  private deployed: string[] = [];
  private pressure = 0.02;
  private silent = false;
  private timer?: NodeJS.Timeout;
  private chain: Promise<unknown> = Promise.resolve();
  /** The last response status of each report route, for assertions. */
  readonly last: { pressure?: number; deployments?: number } = {};

  constructor(private readonly base: string, private token: string, private readonly heartbeatMs = 20_000) {}

  /** Starts the heartbeat. Nothing is sent until `send` or the first tick. */
  start() {
    this.timer = setInterval(() => void this.send(), this.heartbeatMs);
  }
  stop() {
    if (this.timer) clearInterval(this.timer);
  }

  /** The service goes quiet: no reports at all until `resume`. */
  goSilent() {
    this.silent = true;
  }
  async resume() {
    this.silent = false;
    await this.send();
  }

  async report(deployed: string[], pressure = this.pressure) {
    this.deployed = deployed;
    this.pressure = pressure;
    await this.send();
  }

  /** Sends both reports now (serialized, so a command and a heartbeat never overtake each other). */
  send(): Promise<void> {
    if (this.silent) return Promise.resolve();
    const run = this.chain.then(async () => {
      if (this.silent) return;
      const h = { authorization: `Bearer ${this.token}`, "content-type": "application/json" };
      const p = await fetch(`${this.base}/api/v1/integration/pressure`, { method: "PUT", headers: h, body: JSON.stringify({ value: this.pressure }) });
      this.last.pressure = p.status;
      const d = await fetch(`${this.base}/api/v1/integration/deployments`, {
        method: "PUT",
        headers: h,
        body: JSON.stringify({ deployments: this.deployed.map((setup) => ({ setup, labels: { reporter: "e2e" } })) }),
      });
      this.last.deployments = d.status;
    });
    this.chain = run.catch(() => undefined);
    return run;
  }

  setToken(t: string) {
    this.token = t;
  }
}
