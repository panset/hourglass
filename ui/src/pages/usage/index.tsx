// SPDX-License-Identifier: MIT
// My usage (`/`): balance, burn, projection, pressure, holds, tokens per day, ledger. Every number is an API field (UI-1).
import { useState } from "react";
import { ApiError } from "../../api/client";
import { getMe, getPressure } from "../../api/endpoints";
import { Alert, Badge, Button, Chip, Select, Table, useToast } from "../../components";
import { useApi } from "../../hooks/useApi";
import { POLL_INTERVAL_MS } from "../../hooks/usePolling";
import {
  formatDate,
  formatDateTime,
  formatMultiplier,
  formatPercent,
  formatPeriod,
  formatPerHour,
  formatTokens,
} from "../../lib/format";
import { useSession } from "../../shell/session";
import { BarChart } from "./BarChart";
import {
  elapsedSince,
  getMyLedger,
  getMyUsage,
  listActiveHolds,
  releaseHold,
  type Hold,
  type LedgerEntry,
} from "./holdApi";
import { PressureBar } from "./PressureBar";
import styles from "./usage.module.css";

const typeLabel = (e: LedgerEntry) => {
  const base = e.type.charAt(0).toUpperCase() + e.type.slice(1);
  return e.multiplier !== null && e.type === "debit" ? `${base} · ${formatMultiplier(e.multiplier)}` : base;
};

const detail = (e: LedgerEntry) => {
  if (e.setup_name) return e.setup_name;
  const parts = [e.actor_name, e.reason].filter(Boolean);
  return parts.join(": ");
};

function Loading() {
  return <p role="status">Loading…</p>;
}

function Failed({ error }: { error: ApiError }) {
  return <Alert tone="error">{error.message}</Alert>;
}

export default function UsagePage() {
  const session = useSession();
  const toast = useToast();
  const personal = !session.is_local_admin;
  const [period, setPeriod] = useState<string | undefined>();
  const [releasing, setReleasing] = useState<number | null>(null);
  const [releaseError, setReleaseError] = useState<string>();

  const poll = { pollMs: POLL_INTERVAL_MS };
  const pressure = useApi(getPressure, [], poll);
  const me = useApi(getMe, [], { ...poll, enabled: personal });
  const holds = useApi(listActiveHolds, [], { ...poll, enabled: personal });
  const usage = useApi(() => getMyUsage(period), [period], { ...poll, enabled: personal });
  const ledger = useApi(() => getMyLedger(period), [period], { ...poll, enabled: personal });

  const currentPeriod = me.data?.period;
  const selected = period ?? currentPeriod ?? usage.data?.period ?? "";
  const options = Array.from(
    new Set([currentPeriod, ...(usage.data?.available_periods ?? []), ...(ledger.data?.available_periods ?? [])]),
  )
    .filter((p): p is string => !!p)
    .sort()
    .reverse();

  const release = async (h: Hold) => {
    setReleasing(h.id);
    setReleaseError(undefined);
    try {
      await releaseHold(h.id);
      toast.push(`Released ${h.setup_name}.`, "success");
      holds.reload();
      me.reload();
      ledger.reload();
    } catch (e) {
      setReleaseError(e instanceof ApiError ? e.message : "Release failed.");
    } finally {
      setReleasing(null);
    }
  };

  const m = me.data;
  const p = pressure.data;

  return (
    <div className="page">
      <div className={styles.region}>
        <div className={styles.titleRow}>
          <h1>My usage</h1>
          {personal && options.length > 0 ? (
            <label className={styles.month}>
              Month
              <Select value={selected} onChange={(e) => setPeriod(e.target.value)}>
                {options.map((o) => (
                  <option key={o} value={o}>
                    {formatPeriod(o)}
                  </option>
                ))}
              </Select>
            </label>
          ) : null}
        </div>

        <div className={styles.cards}>
          {personal && me.error && !m ? <Failed error={me.error} /> : null}
          {personal && me.loading && !m ? <Loading /> : null}
          {m && m.balance_tokens !== null ? (
            <>
              <section className={styles.card} aria-label="Tokens left">
                <div className={styles.caption}>Tokens left</div>
                <div className={styles.value}>
                  {formatTokens(m.balance_tokens)}
                  {m.overdrawn ? <> <Badge tone="amber">Overdrawn</Badge></> : null}
                </div>
                <div className={styles.sub}>
                  of {formatTokens(m.allowance_tokens)} allowance + {formatTokens(m.granted_tokens)} granted
                  {m.carryover_tokens !== 0 ? ` · ${formatTokens(m.carryover_tokens)} carried over` : ""}
                </div>
              </section>
              <section className={styles.card} aria-label="Burning now">
                <div className={styles.caption}>Burning now</div>
                <div className={styles.value}>{formatPerHour(m.burn_per_hour_tokens)}</div>
                <div className={styles.sub}>
                  {(m.burning ?? []).length === 0
                    ? "Nothing is metering"
                    : (m.burning ?? [])
                        .map((b) => `${b.setup_name} · weight ${formatTokens(b.weight)} × ${formatMultiplier(b.multiplier)}`)
                        .join("; ")}
                </div>
              </section>
              <section className={styles.card} aria-label="Projected run-out">
                <div className={styles.caption}>Projected run-out</div>
                <div className={styles.value}>
                  {m.projection === "date" && m.projected_runout_at
                    ? formatDate(m.projected_runout_at)
                    : m.projection === "not_this_month"
                      ? "not this month"
                      : "not burning"}
                </div>
                <div className={styles.sub}>at the last 24 hours' rate</div>
              </section>
            </>
          ) : null}
          <section className={styles.card} aria-label="Lab pressure">
            <div className={styles.caption}>Lab pressure</div>
            {pressure.error && !p ? <Failed error={pressure.error} /> : null}
            {pressure.loading && !p ? <Loading /> : null}
            {p ? (
              <>
                <div className={styles.value}>
                  {formatPercent(p.smoothed)} · {formatMultiplier(p.multiplier)}
                </div>
                <PressureBar pressure={p} />
                <div className={styles.sub}>
                  {p.setups_deployed} of {p.setups_enabled} setups deployed
                </div>
              </>
            ) : null}
          </section>
        </div>

        {!personal ? (
          <Alert tone="info">The built-in administrator has no balance, holds or ledger.</Alert>
        ) : (
          <>
            <section className={styles.section} aria-label="Holds">
              <h2>{m ? `Current holds · ${m.hold_count} of ${m.hold_limit} allowed` : "Current holds"}</h2>
              {releaseError ? <Alert tone="error">{releaseError}</Alert> : null}
              {holds.loading && !holds.data ? <Loading /> : null}
              {holds.error && !holds.data ? <Failed error={holds.error} /> : null}
              {holds.data ? (
                <Table<Hold>
                  label="Current holds"
                  rows={holds.data.holds}
                  rowKey={(h) => h.id}
                  emptyMessage="You hold no setups."
                  columns={[
                    { key: "setup", header: "Setup", render: (h) => <strong>{h.setup_name}</strong> },
                    {
                      key: "labels",
                      header: "Labels",
                      render: (h) => (
                        <span className={styles.chips}>
                          {Object.entries(h.setup_labels).map(([k, v]) => (
                            <Chip key={k}>{`${k}=${v}`}</Chip>
                          ))}
                        </span>
                      ),
                    },
                    {
                      key: "state",
                      header: "State",
                      render: (h) =>
                        h.deployed ? <Badge tone="blue">Deployed</Badge> : <Badge tone="grey">Acquired · deploying</Badge>,
                    },
                    { key: "weight", header: "Weight", render: (h) => formatTokens(h.weight) },
                    {
                      key: "cost",
                      header: "Cost now",
                      render: (h) => (h.cost_per_hour_tokens ? `${formatTokens(h.cost_per_hour_tokens.now, { fixed: 1 })} tokens / hour` : "—"),
                    },
                    { key: "held", header: "Held for", render: (h) => elapsedSince(h.acquired_at) },
                    {
                      key: "action",
                      header: "",
                      render: (h) => (
                        <Button disabled={releasing === h.id} onClick={() => void release(h)}>
                          Release
                        </Button>
                      ),
                    },
                  ]}
                />
              ) : null}
              <p className={styles.note}>
                Charging continues until you release. Tear the setup down first: release is refused while it is still
                deployed.
              </p>
            </section>

            <div className={styles.columns}>
              <section aria-label="Tokens per day">
                <h2 style={{ fontSize: 16, margin: "0 0 12px" }}>Tokens per day</h2>
                <div className={styles.chart}>
                  {usage.loading && !usage.data ? <Loading /> : null}
                  {usage.error && !usage.data ? <Failed error={usage.error} /> : null}
                  {usage.data ? (
                    usage.data.days.length === 0 ? (
                      <p className={styles.sub}>No usage this period.</p>
                    ) : (
                      <>
                        <p className={styles.total}>{formatTokens(usage.data.total_debited_tokens)} tokens debited</p>
                        <BarChart
                          days={usage.data.days}
                          highlightLast={!!currentPeriod && usage.data.period === currentPeriod}
                          label={`Bar chart of tokens debited per day in ${formatPeriod(usage.data.period)}${
                            usage.data.period === currentPeriod ? ", today highlighted" : ""
                          }`}
                        />
                        <div className={styles.axis}>
                          <span>{formatDate(`${usage.data.days[0]!.date}T00:00:00Z`, { timeZone: "UTC" })}</span>
                          <span>
                            {formatDate(`${usage.data.days[usage.data.days.length - 1]!.date}T00:00:00Z`, { timeZone: "UTC" })}
                          </span>
                        </div>
                      </>
                    )
                  ) : null}
                </div>
              </section>

              <section aria-label="Ledger section">
                <h2 style={{ fontSize: 16, margin: "0 0 12px" }}>Ledger</h2>
                {ledger.loading && !ledger.data ? <Loading /> : null}
                {ledger.error && !ledger.data ? <Failed error={ledger.error} /> : null}
                {ledger.data ? (
                  <Table<LedgerEntry>
                    label="Ledger"
                    rows={ledger.data.entries}
                    rowKey={(e) => e.id}
                    emptyMessage="No ledger entries for this period."
                    columns={[
                      { key: "when", header: "When", render: (e) => formatDateTime(e.created_at) },
                      { key: "type", header: "Type", render: typeLabel },
                      { key: "detail", header: "Detail", render: detail },
                      { key: "amount", header: "Amount", align: "right", render: (e) => formatTokens(e.amount_tokens, { signed: true }) },
                      { key: "balance", header: "Balance", align: "right", render: (e) => formatTokens(e.balance_after_tokens) },
                    ]}
                  />
                ) : null}
              </section>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
