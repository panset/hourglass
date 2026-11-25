// SPDX-License-Identifier: MIT
// The body of the Setups details panel (UI-18): Price, Holder, Labels, Raw details. Price gives the answer first
// (the two costs), then the caveat, then the working. Every number is an API field and is only formatted (UI-1);
// the two costs are compared, never combined. This is the only place a setup's labels are shown. The body holds
// no button, link or field, so the dialog puts focus on the Price heading (`data-autofocus`).
import { Fragment } from "react";
import { Time } from "../../components";
import { formatMultiplier, formatPercent, formatRate, formatTokens, formatWeight } from "../../lib/format";
import { elapsedSince, type Setup, type SetupList } from "../usage/holdApi";
import { attributesLine, PRICE_CHANGE_SENTENCE, sourceWords } from "../usage/weight";
import styles from "./setups.module.css";

/**
 * A setup together with the list-level values of the same response. The panel shows one snapshot, so the
 * multiplier it prints is the one its costs were computed with, also after the setup has left the list.
 */
export interface SetupSnapshot {
  setup: Setup;
  /** Absent when the API is older than the UI: the pressure and multiplier lines are then left out. */
  pricing: SetupList["pricing"] | undefined;
  reportedWeightsEnabled: boolean;
  meteringDescription: string | null;
}

/** The snapshot of setup `id` in a response, or undefined when the response does not hold it. */
export function snapshotOf(list: SetupList | undefined, id: number): SetupSnapshot | undefined {
  const setup = list?.setups.find((s) => s.id === id);
  if (!list || !setup) return undefined;
  return {
    setup,
    pricing: list.pricing,
    reportedWeightsEnabled: !!list.reported_weights_enabled,
    meteringDescription: list.metering_description ?? null,
  };
}

function Charging({ setup, description }: { setup: Setup; description: string | null }) {
  const m = setup.state === "held" ? setup.metering : null;
  if (m?.active) return m.started_at ? <>Since <Time iso={m.started_at} /></> : <>Yes</>;
  if (m) return <>{m.reason ? `Not yet: ${m.reason}` : "Not yet"}</>;
  return description ? <>Starts: {description}</> : null;
}

export function SetupPanel({ snapshot, listed }: { snapshot: SetupSnapshot; listed: boolean }) {
  const { setup: s, pricing } = snapshot;
  const cost = s.cost_per_hour_tokens;
  const attrs = attributesLine(s.weight_attributes);
  const charging = Charging({ setup: s, description: snapshot.meteringDescription });
  // By key, in code-point order: the order of the suggestion list's keys.
  const labels = Object.entries(s.labels ?? {}).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
  const noDetails = s.details === null || s.details === undefined || (typeof s.details === "object" && !Array.isArray(s.details) && Object.keys(s.details).length === 0);
  const holder = s.holder;

  return (
    <div className={styles.panel}>
      {listed ? null : <p role="status">This setup is no longer listed. These are the last values loaded.</p>}

      <h3 tabIndex={-1} data-autofocus="">Price</h3>
      <dl className={`${styles.panelList} ${styles.panelLead}`}>
        <dt>Cost now</dt>
        <dd>
          <strong>{formatRate(cost.now)}</strong> tokens / h
        </dd>
        <dt>Up to</dt>
        <dd>
          {cost.now > cost.at_peak ? (
            "The multipliers were changed. This value shows after the next poll."
          ) : (
            <>
              {formatRate(cost.at_peak)} tokens / h{" "}
              <span className={styles.upToNote}>
                at the highest multiplier{pricing ? ` (${formatMultiplier(pricing.peak_multiplier)})` : ""}
              </span>
            </>
          )}
        </dd>
      </dl>
      {charging ? (
        <dl className={styles.panelList}>
          <dt>Charging</dt>
          <dd>{charging}</dd>
        </dl>
      ) : null}
      {snapshot.reportedWeightsEnabled ? <p>{PRICE_CHANGE_SENTENCE}</p> : null}
      <p className={styles.panelNote}>
        Cost per hour is the setup's weight times the lab's multiplier, rounded to a thousandth of a token. The multiplier
        follows lab pressure.
      </p>
      <dl className={styles.panelList}>
        <dt>Weight</dt>
        <dd>
          <span>{formatWeight(s.weight)}</span>
          {s.weight_source ? <> · <span>{sourceWords(s.weight_source)}</span></> : null}
        </dd>
        {attrs ? (
          <>
            <dt>Attributes</dt>
            <dd>{attrs}</dd>
          </>
        ) : null}
        {pricing ? (
          <>
            <dt>Lab pressure</dt>
            <dd>{pricing.pressure === null || pricing.pressure === undefined ? "not measured yet" : formatPercent(pricing.pressure)}</dd>
            <dt>Multiplier now</dt>
            <dd>{formatMultiplier(pricing.multiplier)}</dd>
          </>
        ) : null}
      </dl>

      {holder ? (
        <>
          <h3>Holder</h3>
          <dl className={styles.panelList}>
            <dt>Held by</dt>
            <dd>{holder.is_you ? "You" : holder.name}</dd>
            {s.held_since ? (
              <>
                <dt>Held since</dt>
                <dd>
                  <Time iso={s.held_since} /> ({elapsedSince(s.held_since)})
                </dd>
              </>
            ) : null}
            <dt>Balance</dt>
            <dd>
              {formatTokens(holder.balance_tokens)} tokens{holder.overdrawn ? ", overdrawn" : ""}
            </dd>
            {holder.release_requested_at ? (
              <>
                <dt>Release</dt>
                <dd>
                  Release requested <Time iso={holder.release_requested_at} />. It completes when the setup is torn down.
                </dd>
              </>
            ) : null}
          </dl>
        </>
      ) : null}

      <h3>Labels</h3>
      {labels.length === 0 ? (
        <p>No labels.</p>
      ) : (
        <>
          <dl className={`${styles.panelList} ${styles.panelLabels}`}>
            {labels.map(([k, v]) => (
              <Fragment key={k}>
                <dt>{k}</dt>
                <dd>{v === "" ? <span className={styles.emptyValue}>(empty)</span> : v}</dd>
              </Fragment>
            ))}
          </dl>
          <p className={styles.panelNote}>Use a label in the Label selector box to filter the list.</p>
        </>
      )}

      <h3>Raw details</h3>
      {noDetails ? <p>None.</p> : <pre className={styles.details} aria-label="Setup details JSON">{JSON.stringify(s.details, null, 2)}</pre>}
    </div>
  );
}
