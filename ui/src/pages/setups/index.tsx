// SPDX-License-Identifier: MIT
// Setups (`/setups`): every setup with its state, holder and cost, acquire (HOLD-1) and release (HOLD-2).
// The row leads with cost; weight, source, the holder's balance and every label are in the details panel (UI-15 to UI-18).
// Every number shown is an API field (UI-1); the one count the page makes is of the rows it draws. Labels are filtered
// through the API by the selector, which the box completes from `label_index` (UI-21); the State filter and the
// exact-match tokens of label values that have no selector term (UI-22) are applied in the browser over the fetched list.
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { ApiError, api } from "../../api/client";
import { getMe, getPressure } from "../../api/endpoints";
import type { Me } from "../../api/types";
import { Alert, Badge, Button, Clip, ConfirmDialog, Field, Modal, Select, Table, useToast, type Column } from "../../components";
import { useApi } from "../../hooks/useApi";
import { POLL_INTERVAL_MS } from "../../hooks/usePolling";
import { formatMultiplier, formatPerHour, formatRate, formatTokens } from "../../lib/format";
import { isAdmin, useSession } from "../../shell/session";
import { elapsedSince, isPendingRelease, releaseHold, type Hold, type Setup, type SetupList } from "../usage/holdApi";
import { attributesLine, PRICE_CHANGE_SENTENCE, PriceSourceTag } from "../usage/weight";
import { SelectorBox, type SelectorBoxHandle } from "./SelectorBox";
import { SetupPanel, snapshotOf, type SetupSnapshot } from "./SetupPanel";
import styles from "./setups.module.css";

const getSetups = (selector: string) =>
  api.get<SetupList>(`/api/v1/setups${selector ? `?selector=${encodeURIComponent(selector)}` : ""}`);
const acquireSetup = (id: number) => api.post<Hold>(`/api/v1/setups/${id}/acquire`);

type StateFilter = "" | "free" | "deployed" | "acquired" | "disabled";

/** An exact-match filter of a label value that no selector can express (UI-22). */
interface ExactToken {
  key: string;
  value: string;
}

/** The row count in words, for the status line. */
const shownText = (n: number) => (n === 0 ? "No setups match the filters." : n === 1 ? "1 setup shown." : `${n} setups shown.`);

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
  // Exact-match tokens: one value per key. A token stays in force until it is removed, also when its value has left the index.
  const [tokens, setTokens] = useState<ExactToken[]>([]);
  // The page's one status line (polite). The 60-second refresh announces nothing.
  const [status, setStatus] = useState("");
  // A row count to announce: at once (`after` unset) or when a response newer than `after` is in.
  const countDue = useRef<{ prefix: string; after?: SetupList | null }>();
  const boxRef = useRef<SelectorBoxHandle>(null);
  const exactLeadId = useId();
  const unappliedId = useId();
  const [listOpen, setListOpen] = useState(false);
  // The panel holds the setup's id, not a copy, so it follows each refresh.
  const [detailsId, setDetailsId] = useState<number>();
  const lastSnapshot = useRef<SetupSnapshot>();
  const headingRef = useRef<HTMLHeadingElement>(null);
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
  const block = localAdmin ? undefined : acquireBlock(me.data);

  const labelIndex = data?.label_index ?? [];
  const rows = (data?.setups ?? []).filter((s) => {
    if (stateFilter && stateOf(s) !== stateFilter) return false;
    return tokens.every((t) => s.labels?.[t.key] === t.value);
  });
  const selectorError = setups.error?.code === "invalid_selector" ? setups.error.message : undefined;
  const failed = !!setups.error;

  useEffect(() => {
    if (selectorError) setStatus(`Selector not accepted. ${selectorError}`);
    if (failed) countDue.current = undefined;
  }, [selectorError, failed]);
  // After a filter change the row count is announced: at once for a filter applied in the browser, and for a
  // selector when its response is in. A refresh on its own leaves `countDue` unset and announces nothing.
  useEffect(() => {
    const due = countDue.current;
    if (!due || !data || due.after === data) return;
    countDue.current = undefined;
    setStatus(due.prefix + shownText(rows.length));
  });

  /** Applies selector text. The count is announced only when a request follows. */
  const applyText = (text: string) => {
    const next = text.trim();
    if (next !== selector) countDue.current = { prefix: "", after: data ?? null };
    setSelector(next);
  };

  // Another exact value of the same key replaces its token; the same value again changes nothing.
  const addExact = (key: string, value: string) => {
    if (tokens.some((t) => t.key === key && t.value === value)) return;
    const token = { key, value };
    setTokens(tokens.some((t) => t.key === key) ? tokens.map((t) => (t.key === key ? token : t)) : [...tokens, token]);
    countDue.current = { prefix: `Exact-match filter ${key}=${value} added. ` };
  };

  // The removed control is gone, so focus goes to the selector box.
  const removeExact = (token: ExactToken) => {
    setTokens(tokens.filter((t) => t !== token));
    countDue.current = { prefix: `Exact-match filter ${token.key}=${token.value} removed. ` };
    boxRef.current?.focus();
  };

  const clearFilters = () => {
    countDue.current = selector === "" ? undefined : { prefix: "", after: data ?? null };
    setStateFilter("");
    setSelectorDraft("");
    setSelector("");
    setTokens([]);
    setStatus("Filters cleared.");
    boxRef.current?.focus();
  };

  const changeState = (next: StateFilter) => {
    setStateFilter(next);
    countDue.current = { prefix: "" };
  };

  const inForce = stateFilter !== "" || selector !== "" || tokens.length > 0;
  // The box and the applied selector can differ (typing, or an exact-match pick that took a requirement out of
  // the box): the filters-in-force line then names the selector that is in force, and Filter is the way to apply
  // the box. Filter is the filled button only while the list is closed, so it never competes with a suggestion.
  const unapplied = selectorDraft.trim() !== selector;

  // The open panel: the setup as the newest response has it, or, when that response no longer lists it, the
  // last snapshot, whose every value is from one response.
  const liveSnapshot = detailsId === undefined ? undefined : snapshotOf(data, detailsId);
  if (liveSnapshot) lastSnapshot.current = liveSnapshot;
  const kept = lastSnapshot.current;
  const panel = detailsId === undefined ? undefined : (liveSnapshot ?? (kept?.setup.id === detailsId ? kept : undefined));
  const closeDetails = () => setDetailsId(undefined);

  const applySelector = (e: FormEvent) => {
    e.preventDefault();
    applyText(selectorDraft);
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

  // Line 1: the cost now and at the highest band, as a button that opens the panel, and beside it the source tag.
  // The cost is a number first: text colour, the cost now in bold, a dotted underline as the sign that it opens
  // something. The two API costs are compared, never combined: one value when `now` is not below `at_peak` (one
  // band, weight 0, a band edit before the next poll). Line 2, only for a hold that is not metering: the policy's reason.
  const costCell = (s: Setup) => {
    const { now, at_peak: atPeak } = s.cost_per_hour_tokens;
    const range = now < atPeak;
    const reason = s.state === "held" && s.metering && !s.metering.active ? s.metering.reason : null;
    const tagged = s.weight_source === "reported" || s.weight_source === "override";
    return (
      <>
        <button type="button" className={styles.costButton} data-cost="" onClick={() => setDetailsId(s.id)}>
          <span aria-hidden="true">
            <span className={styles.costNow}>{formatRate(now)}</span>
            {range ? (
              <>
                {" now "}
                <span className={styles.costMax}>· up to {formatRate(atPeak)}</span>
              </>
            ) : null}
          </span>
          <span className="sr-only">
            {range ? `${formatRate(now)} tokens per hour now, up to ${formatRate(atPeak)}. ` : `${formatRate(now)} tokens per hour. `}
            Price details for {s.name}
          </span>
        </button>
        {tagged ? (
          <span className={styles.costTag}>
            <PriceSourceTag source={s.weight_source} />
          </span>
        ) : null}
        {reason ? <span className={styles.costNote}>{reason}</span> : null}
      </>
    );
  };

  const holderCell = (s: Setup) =>
    s.holder ? (
      <>
        <span className={styles.holderLine} data-held="">
          {s.holder.is_you ? <span className={styles.you}>You</span> : <Clip text={s.holder.name} max={24} />} · {elapsedSince(s.held_since)}
        </span>
        {s.holder.overdrawn ? <span className={`${styles.overdrawn} ${styles.overdrawnNote}`}>overdrawn</span> : null}
      </>
    ) : (
      ""
    );

  // Every column is one line (`nowrap`); the cost cell's reason line has its own width and wraps (UI-20).
  const columns: Column<Setup>[] = [
    { key: "name", header: "Setup", nowrap: true, render: (s) => <strong><Clip text={s.name} max={32} /></strong> },
    {
      key: "state",
      header: "State",
      nowrap: true,
      render: (s) =>
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
    { key: "holder", header: "Held by", nowrap: true, render: holderCell },
    { key: "cost", header: "Cost (tokens / h)", nowrap: true, render: costCell },
    {
      key: "action",
      header: "",
      nowrap: true,
      render: (s) => (
        <span className={styles.actions}>
          <button type="button" className={styles.linkButton} onClick={() => setDetailsId(s.id)} aria-label={`Details for ${s.name}`}>
            Details
          </button>
          {s.holder?.is_you ? (
            s.holder.release_requested_at ? (
              <Badge tone="amber">Releasing</Badge>
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

  const otherError = setups.error && !selectorError ? setups.error : undefined;
  const cost = acquiring?.cost_per_hour_tokens;

  return (
    <div className="page">
      <div className={styles.region}>
        <div className={styles.titleRow}>
          <h1 ref={headingRef} tabIndex={-1}>Setups</h1>
          {data ? (
            <div className={styles.summary}>
              {data.summary.deployed} of {data.summary.enabled} setups deployed · {data.summary.free} free
            </div>
          ) : null}
        </div>

        <div className={styles.filters}>
          <Field label="State">
            {(p) => (
              <Select {...p} className={styles.stateSel} value={stateFilter} onChange={(e) => changeState(e.target.value as StateFilter)}>
                <option value="">All states</option>
                <option value="free">Free</option>
                <option value="deployed">Deployed</option>
                <option value="acquired">Acquired</option>
                {admin ? <option value="disabled">Disabled</option> : null}
              </Select>
            )}
          </Field>
          <form onSubmit={applySelector}>
            <SelectorBox
              ref={boxRef}
              value={selectorDraft}
              onChange={setSelectorDraft}
              onApply={applyText}
              onExact={addExact}
              onAnnounce={setStatus}
              onListOpen={setListOpen}
              labelIndex={labelIndex}
              error={selectorError}
            />
            <Button
              type="submit"
              variant={unapplied && !listOpen ? "primary" : undefined}
              className={styles.filterBtn}
              aria-describedby={unapplied ? unappliedId : undefined}
            >
              Filter
            </Button>
            {unapplied ? (
              <span id={unappliedId} className="sr-only">
                The typed selector is not applied yet.
              </span>
            ) : null}
          </form>
        </div>

        {inForce ? (
          <div className={styles.inForce}>
            {tokens.length > 0 ? (
              <>
                <span id={exactLeadId} className={styles.exactLead}>
                  Exact match:
                </span>
                <ul aria-labelledby={exactLeadId}>
                  {tokens.map((t) => (
                    <li key={t.key} className={styles.token}>
                      <span className={styles.tokenText} title={`${t.key}=${t.value}`}>
                        {t.key}={t.value}
                      </span>
                      <button
                        type="button"
                        className={styles.tokenRemove}
                        aria-label={`Remove exact-match filter ${t.key}=${t.value}`}
                        onClick={() => removeExact(t)}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
              </>
            ) : null}
            {selector !== "" && unapplied ? (
              <span className={styles.applied}>
                Selector in force: <code>{selector}</code>
              </span>
            ) : null}
            {data && rows.length > 0 ? <span className={styles.count}>{rows.length === 1 ? "1 setup shown" : `${rows.length} setups shown`}</span> : null}
            <button type="button" className={styles.linkButton} onClick={clearFilters}>
              Clear filters
            </button>
          </div>
        ) : null}
        {data ? (
          <div role="status" className="sr-only">
            {status}
          </div>
        ) : null}

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
            emptyMessage={inForce ? "No setups match the filters." : "No setups to show."}
          />
        ) : null}

        <Modal
          open={!!panel}
          title={panel ? `Details: ${panel.setup.name}` : "Details"}
          onClose={closeDetails}
          footer={<Button onClick={closeDetails}>Close</Button>}
          fallbackFocus={headingRef}
        >
          {panel ? <SetupPanel snapshot={panel} listed={!!liveSnapshot} /> : null}
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
                Weight {formatTokens(acquiring.weight)}
                {acquiring.weight_source === "reported" || acquiring.weight_source === "override" ? ` (${acquiring.weight_source})` : ""}
                . Costs {formatPerHour(cost.now)}
                {pressure.data ? ` now (${formatMultiplier(pressure.data.multiplier)})` : " now"}, up to{" "}
                {formatPerHour(cost.at_peak)} at the highest band.
              </p>
              {attributesLine(acquiring.weight_attributes) ? <p>{attributesLine(acquiring.weight_attributes)}</p> : null}
              {data?.reported_weights_enabled ? (
                <p>{PRICE_CHANGE_SENTENCE}</p>
              ) : null}
              {data?.metering_description ? <p>Charging starts: {data.metering_description}.</p> : null}
            </>
          ) : null}
        </ConfirmDialog>
      </div>
    </div>
  );
}
