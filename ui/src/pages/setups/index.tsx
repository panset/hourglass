// SPDX-License-Identifier: MIT
// Setups (`/setups`): every setup with admin-chosen label columns (SET-6), acquire (HOLD-1) and release (HOLD-2).
// Every number shown is an API field (UI-1); filtering by state and label is client-side over the fetched list.
import { useState, type FormEvent } from "react";
import { ApiError, api } from "../../api/client";
import { getMe, getPressure } from "../../api/endpoints";
import type { Me } from "../../api/types";
import { Alert, Badge, Button, Clip, ConfirmDialog, Field, Input, Modal, Select, Table, Time, useToast } from "../../components";
import { useApi } from "../../hooks/useApi";
import { POLL_INTERVAL_MS } from "../../hooks/usePolling";
import { formatMultiplier, formatPerHour, formatTokens } from "../../lib/format";
import { isAdmin, useSession } from "../../shell/session";
import { elapsedSince, isPendingRelease, releaseHold, type Hold, type Setup, type SetupList } from "../usage/holdApi";
import { LabelChips, placeholderFrom } from "./LabelChips";
import { applyChip, splitTerms } from "./labelTerms";
import styles from "./setups.module.css";

const getSetups = (selector: string) =>
  api.get<SetupList>(`/api/v1/setups${selector ? `?selector=${encodeURIComponent(selector)}` : ""}`);
const acquireSetup = (id: number) => api.post<Hold>(`/api/v1/setups/${id}/acquire`);

type StateFilter = "" | "free" | "deployed" | "acquired" | "disabled";

function stateOf(s: Setup): Exclude<StateFilter, ""> {
  if (s.state === "free") return "free";
  if (s.state === "disabled") return "disabled";
  return s.deployed ? "deployed" : "acquired";
}

/** Why the Acquire buttons are disabled: the API decides (`can_acquire`), the page only words the reason. */
function acquireBlock(me: Me | undefined): string | undefined {
  if (!me || me.can_acquire) return undefined;
  switch (me.cannot_acquire_reason) {
    case "insufficient_tokens":
      return "Your balance is zero or below, so you cannot acquire setups.";
    case "hold_limit":
      return `You already hold ${me.hold_count} of ${me.hold_limit} allowed setups.`;
    default:
      return "You cannot acquire setups.";
  }
}

export default function SetupsPage() {
  const session = useSession();
  const toast = useToast();
  const admin = isAdmin(session);
  const localAdmin = session.is_local_admin;

  const [selectorDraft, setSelectorDraft] = useState("");
  const [selector, setSelector] = useState("");
  const [stateFilter, setStateFilter] = useState<StateFilter>("");
  const [labelFilters, setLabelFilters] = useState<Record<string, string>>({});
  const [details, setDetails] = useState<Setup>();
  const [acquiring, setAcquiring] = useState<Setup>();
  const [acquireBusy, setAcquireBusy] = useState(false);
  const [acquireError, setAcquireError] = useState<string>();
  const [releasing, setReleasing] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string>();

  const poll = { pollMs: POLL_INTERVAL_MS };
  const setups = useApi(() => getSetups(selector), [selector], poll);
  const me = useApi(getMe, [], { ...poll, enabled: !localAdmin });
  const pressure = useApi(getPressure, [], poll);

  const data = setups.data;
  const keys = data?.display_label_keys ?? [];
  const block = localAdmin ? undefined : acquireBlock(me.data);

  const rows = (data?.setups ?? []).filter((s) => {
    if (stateFilter && stateOf(s) !== stateFilter) return false;
    return keys.every((k) => !labelFilters[k] || s.labels[k] === labelFilters[k]);
  });

  const labelIndex = data?.label_index ?? [];
  const placeholder = placeholderFrom(labelIndex);

  // A chip puts its API-given term into the box and applies the filter (UI-9).
  const activateChip = (key: string, term: string) => {
    const next = applyChip(selectorDraft, key, term);
    setSelectorDraft(next);
    setSelector(next);
  };

  const applySelector = (e: FormEvent) => {
    e.preventDefault();
    setSelector(selectorDraft.trim());
  };

  const reloadAll = () => {
    setups.reload();
    me.reload();
  };

  const confirmAcquire = async () => {
    if (!acquiring) return;
    setAcquireBusy(true);
    setAcquireError(undefined);
    try {
      await acquireSetup(acquiring.id);
      toast.push(`Acquired ${acquiring.name}.`, "success");
      setAcquiring(undefined);
      reloadAll();
    } catch (e) {
      setAcquireError(e instanceof ApiError ? e.message : "Acquire failed.");
    } finally {
      setAcquireBusy(false);
    }
  };

  const release = async (s: Setup) => {
    if (!s.holder) return;
    setReleasing(s.holder.hold_id);
    setActionError(undefined);
    try {
      const done = await releaseHold(s.holder.hold_id);
      toast.push(
        isPendingRelease(done)
          ? `Release requested for ${s.name}. It completes when the setup is torn down.`
          : `Released ${s.name}.`,
        "success",
      );
      reloadAll();
    } catch (e) {
      setActionError(e instanceof ApiError ? e.message : "Release failed.");
    } finally {
      setReleasing(null);
    }
  };

  // Every setup shows its cost per hour (the API sends it for all of them). A held setup that is not
  // metering yet adds the policy's own reason text from the API as secondary text, and nothing when it sends none.
  const costCell = (s: Setup) => {
    const perHour = formatPerHour(s.cost_per_hour_tokens.now);
    if (s.state === "free") return `${perHour} if acquired`;
    if (s.state === "held" && s.metering && !s.metering.active) {
      return (
        <>
          {perHour}
          {s.metering.reason ? <span className={styles.reason}>{s.metering.reason}</span> : null}
        </>
      );
    }
    return perHour;
  };

  const columns = [
    {
      key: "name",
      header: "Setup",
      nowrap: true,
      render: (s: Setup) => (
        <>
          <strong><Clip text={s.name} max={32} /></strong>
          {admin && s.state === "disabled" ? <> <Badge tone="grey">Disabled</Badge></> : null}
        </>
      ),
    },
    ...keys.map((k) => ({ key: `label:${k}`, header: k, render: (s: Setup) => s.labels[k] ?? "" })),
    { key: "weight", header: "Weight", render: (s: Setup) => formatTokens(s.weight) },
    {
      key: "state",
      header: "State",
      render: (s: Setup) =>
        s.state === "free" ? (
          <Badge tone="green">Free</Badge>
        ) : s.state === "disabled" ? (
          <Badge tone="grey">Disabled</Badge>
        ) : s.deployed ? (
          <Badge tone="blue">Deployed</Badge>
        ) : (
          <Badge tone="grey">Acquired · deploying</Badge>
        ),
    },
    { key: "holder", header: "Held by", render: (s: Setup) => (s.holder ? (s.holder.is_you ? "You" : s.holder.name) : "") },
    {
      key: "balance",
      header: "Holder's tokens",
      render: (s: Setup) =>
        s.holder ? (
          s.holder.overdrawn ? (
            <span className={styles.overdrawn}>{formatTokens(s.holder.balance_tokens)} overdrawn</span>
          ) : (
            formatTokens(s.holder.balance_tokens)
          )
        ) : (
          ""
        ),
    },
    { key: "held", header: "Held for", render: (s: Setup) => (s.held_since ? elapsedSince(s.held_since) : "") },
    { key: "cost", header: "Cost", render: costCell },
    {
      key: "action",
      header: "",
      render: (s: Setup) => (
        <span className={styles.actions}>
          <button type="button" className={styles.linkButton} onClick={() => setDetails(s)} aria-label={`Details for ${s.name}`}>
            Details
          </button>
          {s.holder?.is_you ? (
            s.holder.release_requested_at ? (
              <span className={styles.releasing}>
                <Badge tone="amber">Releasing</Badge>
                <span className={styles.reason}>
                  requested <Time iso={s.holder.release_requested_at} />
                </span>
              </span>
            ) : (
              <Button disabled={releasing === s.holder.hold_id} onClick={() => void release(s)}>
                Release
              </Button>
            )
          ) : null}
          {s.state === "free" && !localAdmin ? (
            <Button
              variant="primary"
              disabled={!!block}
              title={block}
              aria-label={`Acquire ${s.name}`}
              onClick={() => {
                setAcquireError(undefined);
                setAcquiring(s);
              }}
            >
              Acquire
            </Button>
          ) : null}
        </span>
      ),
    },
  ];

  const valuesFor = (k: string) =>
    Array.from(new Set((data?.setups ?? []).map((s) => s.labels[k]).filter((v): v is string => v !== undefined))).sort();

  const selectorError = setups.error?.code === "invalid_selector" ? setups.error.message : undefined;
  const otherError = setups.error && !selectorError ? setups.error : undefined;
  const cost = acquiring?.cost_per_hour_tokens;

  return (
    <div className="page">
      <div className={styles.region}>
        <div className={styles.titleRow}>
          <h1>Setups</h1>
          {data ? (
            <div className={styles.summary}>
              {data.summary.deployed} of {data.summary.enabled} setups deployed · {data.summary.free} free
            </div>
          ) : null}
        </div>

        <div className={styles.filters}>
          <Field label="State">
            {(p) => (
              <Select {...p} className={styles.stateSel} value={stateFilter} onChange={(e) => setStateFilter(e.target.value as StateFilter)}>
                <option value="">All states</option>
                <option value="free">Free</option>
                <option value="deployed">Deployed</option>
                <option value="acquired">Acquired</option>
                {admin ? <option value="disabled">Disabled</option> : null}
              </Select>
            )}
          </Field>
          {keys.map((k) => (
            <Field key={k} label={k}>
              {(p) => (
                <Select
                  {...p}
                  className={styles.labelSel}
                  value={labelFilters[k] ?? ""}
                  onChange={(e) => setLabelFilters({ ...labelFilters, [k]: e.target.value })}
                >
                  <option value="">Any</option>
                  {valuesFor(k).map((v) => (
                    <option key={v} value={v}>
                      {v}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          ))}
          <form onSubmit={applySelector}>
            <Field label="Label selector" error={selectorError}>
              {(p) => (
                <Input
                  {...p}
                  className={styles.selectorInput}
                  placeholder={placeholder}
                  value={selectorDraft}
                  onChange={(e) => setSelectorDraft(e.target.value)}
                />
              )}
            </Field>
            <Button type="submit">Filter</Button>
          </form>
        </div>

        {data ? <LabelChips index={labelIndex} applied={splitTerms(selector)} onToggle={activateChip} /> : null}

        {block ? <Alert tone="warning">{block}</Alert> : null}
        {actionError ? <Alert tone="error">{actionError}</Alert> : null}
        {otherError ? <Alert tone="error">{otherError.message}</Alert> : null}
        {setups.loading && !data ? <p role="status">Loading…</p> : null}
        {data ? (
          <Table<Setup>
            label="All setups"
            columns={columns}
            rows={rows}
            rowKey={(s) => s.id}
            emptyMessage={data.setups.length === 0 ? "No setups to show." : "No setups match the filters."}
          />
        ) : null}

        <Modal
          open={!!details}
          title={details ? `Details: ${details.name}` : "Details"}
          onClose={() => setDetails(undefined)}
          footer={<Button onClick={() => setDetails(undefined)}>Close</Button>}
        >
          <pre className={styles.details} aria-label="Setup details JSON">
            {details ? JSON.stringify(details.details, null, 2) : ""}
          </pre>
        </Modal>

        <ConfirmDialog
          open={!!acquiring}
          title={acquiring ? `Acquire ${acquiring.name}?` : "Acquire"}
          confirmLabel="Acquire"
          busy={acquireBusy}
          error={acquireError}
          onConfirm={() => void confirmAcquire()}
          onCancel={() => setAcquiring(undefined)}
        >
          {acquiring && cost ? (
            <>
              <p>
                Weight {formatTokens(acquiring.weight)}. Costs {formatPerHour(cost.now)}
                {pressure.data ? ` now (${formatMultiplier(pressure.data.multiplier)})` : " now"}, up to{" "}
                {formatPerHour(cost.at_peak)} at the highest band.
              </p>
              {data?.metering_description ? <p>Charging starts: {data.metering_description}.</p> : null}
            </>
          ) : null}
        </ConfirmDialog>
      </div>
    </div>
  );
}
