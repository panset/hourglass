// SPDX-License-Identifier: MIT
// My usage (`/usage`): balance, burn, projection, pressure, holds, tokens per day, ledger. Every number is an API field (UI-1).
import { useEffect, useState } from "react";
import { ApiError } from "../../api/client";
import { getMe, getPressure } from "../../api/endpoints";
import { Alert, Badge, Button, Chip, Select, Table, Time, useToast, zoneLabel } from "../../components";
import { useApi } from "../../hooks/useApi";
import { POLL_INTERVAL_MS } from "../../hooks/usePolling";
import {
  formatDate,
  formatDay,
  formatMultiplier,
  formatPercent,
  formatPeriod,
  formatRate,
  formatTokens,
} from "../../lib/format";
import { useSession } from "../../shell/session";
import { BarChart } from "./BarChart";
import {
  elapsedSince,
  getMyLedgerPage,
  LEDGER_PAGE_SIZE,
  isPendingRelease,
  getMyUsage,
  listActiveHolds,
  releaseHold,
  type Hold,
  type LedgerEntry,
  type LedgerView,
} from "./holdApi";
import { PressureBar } from "./PressureBar";
import { ledgerWeight, WeightTag } from "./weight";
import styles from "./usage.module.css";

const typeLabel = (e: LedgerEntry) => {
  const base = e.type.charAt(0).toUpperCase() + e.type.slice(1);
  if (e.type !== "debit") return base;
  // UI-13: the weight follows the multiplier; nothing is added when the API says the weight is unknown.
  const parts = [base];
  if (e.multiplier !== null) parts.push(formatMultiplier(e.multiplier));
  const w = ledgerWeight(e.weight_min, e.weight_max, e.weight_sources);
  if (w) parts.push(w);
  return parts.join(" · ");
};

const detail = (e: LedgerEntry) => {
  if (e.setup_name) return (e.entry_count ?? 1) > 1 ? `${e.setup_name} · ${e.entry_count} polls` : e.setup_name;
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
  const [view, setView] = useState<LedgerView>("day");
  // Cursors of the pages visited after the first: the page shown is cursors[cursors.length - 1], or the first page when empty.
  // A cursor is meant for the period and view it came from, so changing either drops them all.
  const [cursors, setCursors] = useState<string[]>([]);
  const [releasing, setReleasing] = useState<number | null>(null);
  const [releaseError, setReleaseError] = useState<string>();

  const poll = { pollMs: POLL_INTERVAL_MS };
  const pressure = useApi(getPressure, [], poll);
  const me = useApi(getMe, [], { ...poll, enabled: personal });
  const holds = useApi(listActiveHolds, [], { ...poll, enabled: personal });
  const usage = useApi(() => getMyUsage(period), [period], { ...poll, enabled: personal });
  const cursor = cursors[cursors.length - 1];
  // The page is returned with the request it answers, so a failed request for another page is never shown as the stale one.
  const ledgerKey = `${period ?? ""}|${view}|${cursor ?? ""}`;
  const ledger = useApi(
    async () => ({ key: ledgerKey, page: await getMyLedgerPage(period, view, cursor) }),
    [period, view, cursor],
    { ...poll, enabled: personal },
  );
  const ledgerPage = ledger.data?.key === ledgerKey ? ledger.data.page : undefined;
  const changePeriod = (p: string) => {
    setPeriod(p);
    setCursors([]);
  };
  const changeView = (v: LedgerView) => {
    setView(v);
    setCursors([]);
  };
  const nextCursor = ledgerPage?.next_cursor ?? null;

  const currentPeriod = me.data?.period;
  // On the default month the period is implicit, so a rollover changes it without the picker: the cursors of the old month are dropped.
  useEffect(() => {
    if (period === undefined) setCursors([]);
  }, [currentPeriod]); // eslint-disable-line react-hooks/exhaustive-deps
  const selected = period ?? currentPeriod ?? usage.data?.period ?? "";
  const options = Array.from(
    new Set([currentPeriod, ...(usage.data?.available_periods ?? []), ...(ledger.data?.page.available_periods ?? [])]),
  )
    .filter((p): p is string => !!p)
    .sort()
    .reverse();

  const release = async (h: Hold) => {
    setReleasing(h.id);
    setReleaseError(undefined);
    try {
      const done = await releaseHold(h.id);
      toast.push(
        isPendingRelease(done)
          ? `Release requested for ${h.setup_name}. It completes when the setup is torn down.`
          : `Released ${h.setup_name}.`,
        "success",
      );
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
              <Select value={selected} onChange={(e) => changePeriod(e.target.value)}>
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
                <div className={styles.value}>{formatRate(m.burn_per_hour_tokens)} tokens / h</div>
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
                    {
                      key: "weight",
                      header: "Weight",
                      render: (h) => (
                        <>
                          {formatTokens(h.weight)}
                          <WeightTag source={h.weight_source} />
                        </>
                      ),
                    },
                    {
                      key: "cost",
                      header: "Cost now",
                      render: (h) => (h.cost_per_hour_tokens ? `${formatTokens(h.cost_per_hour_tokens.now, { fixed: 1 })} tokens / h` : "—"),
                    },
                    { key: "held", header: "Held for", render: (h) => elapsedSince(h.acquired_at) },
                    {
                      key: "action",
                      header: "",
                      render: (h) =>
                        h.release_requested_at ? (
                          <span className={styles.releasing}>
                            <Badge tone="amber">Releasing</Badge>
                            <span className={styles.sub}>
                              requested <Time iso={h.release_requested_at} />
                            </span>
                          </span>
                        ) : (
                          <Button disabled={releasing === h.id} onClick={() => void release(h)}>
                            Release
                          </Button>
                        ),
                    },
                  ]}
                />
              ) : null}
              <p className={styles.note}>
                Charging continues until you release. If the setup is still deployed, Release only requests it: the hold shows
                Releasing, keeps charging, and is released when the setup is torn down.
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
                          <span>{formatDay(usage.data.days[0]!.date)}</span>
                          <span>
                            {formatDay(usage.data.days[usage.data.days.length - 1]!.date)}
                          </span>
                        </div>
                      </>
                    )
                  ) : null}
                </div>
              </section>

              <section aria-label="Ledger section">
                <h2 style={{ fontSize: 16, margin: "0 0 12px" }}>Ledger</h2>
                {ledger.loading && !ledgerPage ? <Loading /> : null}
                {ledger.error && !ledger.loading && !ledgerPage ? <Failed error={ledger.error} /> : null}
                <div className={styles.viewSwitch} role="group" aria-label="Ledger view">
                  <Button aria-pressed={view === "day"} onClick={() => changeView("day")}>
                    By day
                  </Button>
                  <Button aria-pressed={view === "poll"} onClick={() => changeView("poll")}>
                    Every poll
                  </Button>
                </div>
                {ledgerPage ? (
                  <Table<LedgerEntry>
                    label="Ledger"
                    rows={ledgerPage.entries.slice(0, LEDGER_PAGE_SIZE)}
                    rowKey={(e) => e.id}
                    emptyMessage="No ledger entries for this period."
                    columns={[
                      { key: "when", header: `When (${zoneLabel()})`, nowrap: true, render: (e) => <Time iso={e.created_at} /> },
                      { key: "type", header: "Type", render: typeLabel },
                      { key: "detail", header: "Detail", render: detail },
                      { key: "amount", header: "Amount", align: "right", render: (e) => formatTokens(e.amount_tokens, { signed: true }) },
                      { key: "balance", header: "Balance", align: "right", render: (e) => formatTokens(e.balance_after_tokens) },
                    ]}
                  />
                ) : null}
                <div className={styles.pager}>
                  <Button disabled={cursors.length === 0 || ledger.loading} onClick={() => setCursors((c) => c.slice(0, -1))}>
                    Newer
                  </Button>
                  <Button
                    disabled={nextCursor === null || ledger.loading}
                    onClick={() => nextCursor !== null && setCursors((c) => [...c, nextCursor])}
                  >
                    Older
                  </Button>
                </div>
              </section>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
