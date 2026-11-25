// SPDX-License-Identifier: MIT
// Settings (ADM-2, ADM-4, RULE-3). Nothing is saved implicitly (UI rules).
import { useState } from "react";
import { Alert, Button, Field, Input, Table, Time, zoneLabel } from "../../components";
import { useApi } from "../../hooks/useApi";
import shared from "../admin-shared/admin.module.css";
import { errorText, useAction } from "../admin-shared/useAction";
import { getCollector, getRules, getSettings, putRules, putSettings, type Collector as CollectorData } from "./api";
import { reportedFields, sameJson, toForm, toRuleInputs, toRuleRows, toSettings, type FieldErrors, type RuleRow, type SettingsForm } from "./form";

const POLL_MS = 60_000;
let newKey = 1;

type ReportTimes = NonNullable<CollectorData["last_pressure_report"]>;

/** One stored report: when it was observed and received, and which account sent it (D62). Null is "None received". */
function ReportStat({ title, report }: { title: string; report: ReportTimes | null }) {
  return (
    <div className={shared.stat}>
      <span className={shared.sub}>{title}</span>
      {report ? (
        <>
          <span className={shared.statValue}>
            <Time iso={report.received_at} />
          </span>
          <span className={shared.sub}>
            received from {report.service_account}; observed <Time iso={report.observed_at} />
          </span>
        </>
      ) : (
        <span className={shared.statValue}>None received</span>
      )}
    </div>
  );
}

type CollectorState = ReturnType<typeof useApi<CollectorData>>;

/** The "Reported weights" section (D1, C3): the four fields, the last accepted report and the two warnings. */
function ReportedWeights({
  form,
  edit,
  collector,
  pollInterval,
  fieldError,
}: {
  form: SettingsForm;
  edit: (patch: Partial<SettingsForm>) => void;
  collector: CollectorState;
  pollInterval: string;
  fieldError: (key: string) => string | undefined;
}) {
  const report = collector.data?.last_weights_report ?? null;
  const refusal = collector.data?.last_weights_refusal ?? null;
  return (
    <section className={shared.card} aria-labelledby="rw-h">
      <h2 id="rw-h">Reported weights</h2>
      <label className={shared.check}>
        <input type="checkbox" checked={form.rwEnabled} onChange={(e) => edit({ rwEnabled: e.target.checked })} />
        Accept weights reported by a service account
      </label>
      {fieldError("reported_weights.enabled") ?? fieldError("reported_weights") ? (
        <Alert tone="error">{fieldError("reported_weights.enabled") ?? fieldError("reported_weights")}</Alert>
      ) : null}
      <p className={shared.note}>
        On: a usable reported weight replaces the rate rules for that setup. Off: reports are refused and prices come from the rules.
      </p>
      <Field label="Minimum weight" error={fieldError("reported_weights.min")}>
        {(p) => <Input {...p} inputMode="decimal" value={form.rwMin} onChange={(e) => edit({ rwMin: e.target.value })} />}
      </Field>
      <Field label="Maximum weight" hint="Required before the switch can be on." error={fieldError("reported_weights.max")}>
        {(p) => <Input {...p} inputMode="decimal" value={form.rwMax} onChange={(e) => edit({ rwMax: e.target.value })} />}
      </Field>
      <Field label="Maximum age" hint="A duration from 1m to 24h, such as 10m." error={fieldError("reported_weights.max_age")}>
        {(p) => <Input {...p} value={form.rwMaxAge} onChange={(e) => edit({ rwMaxAge: e.target.value })} />}
      </Field>
      <p className={shared.note}>
        The maximum age should be at least twice the poll interval ({pollInterval}). With a shorter value, polls can alternate
        between the reported price and the rule price without any error.
      </p>
      {collector.error && !collector.data ? <Alert tone="error">{collector.error.message}</Alert> : null}
      {collector.data ? (
        <div className={shared.stat}>
          <span className={shared.sub}>Last weights report</span>
          {report ? (
            <>
              <span className={shared.statValue}>
                <Time iso={report.received_at} />
              </span>
              <span className={shared.sub}>
                received from {report.service_account}; observed <Time iso={report.observed_at} />; {report.entries} weight
                {report.entries === 1 ? "" : "s"}
              </span>
            </>
          ) : (
            <span className={shared.statValue}>None received</span>
          )}
        </div>
      ) : null}
      {report && report.out_of_bounds > 0 ? (
        <Alert tone="warning">
          {report.out_of_bounds} stored weight{report.out_of_bounds === 1 ? " is" : "s are"} outside the minimum and maximum
          and not applied. Those setups are charged at their rate rule price, and a report that still holds such a weight is refused.
        </Alert>
      ) : null}
      {refusal && (!report || Date.parse(refusal.at) > Date.parse(report.received_at)) ? (
        <Alert tone="warning">
          A weights report was refused after the last accepted one, at <Time iso={refusal.at} />, from {refusal.service_account}:{" "}
          {refusal.message}
        </Alert>
      ) : null}
    </section>
  );
}

function Collector({ c }: { c: CollectorState }) {
  if (c.loading) return <p role="status">Loading collector health…</p>;
  if (c.error || !c.data) return <Alert tone="error">{c.error?.message ?? "Could not load collector health."}</Alert>;
  const d = c.data;
  return (
    <div className={shared.region}>
      <div className={shared.cards}>
        <div className={shared.stat}>
          <span className={shared.sub}>Last successful poll</span>
          <span className={shared.statValue}>
            {d.last_successful_poll_at ? <Time iso={d.last_successful_poll_at} /> : "Never"}
          </span>
        </div>
        <ReportStat title="Last pressure report" report={d.last_pressure_report} />
        <ReportStat title="Last deployment report" report={d.last_deployments_report} />
        <div className={shared.stat}>
          <span className={shared.sub}>Setups matched</span>
          <span className={shared.statValue}>{d.matched_setups}</span>
          <span className={shared.sub}>{d.orphan_count} orphan(s)</span>
        </div>
        <div className={shared.stat}>
          <span className={shared.sub}>Outages this month</span>
          <span className={shared.statValue}>{d.outages_this_month.count}</span>
          <span className={shared.sub}>{d.outages_this_month.minutes_not_charged} min, not charged</span>
        </div>
        <div className={shared.stat}>
          <span className={shared.sub}>Event queue</span>
          <span className={shared.statValue}>{d.queue_length}</span>
          <span className={shared.sub}>waiting for delivery</span>
        </div>
      </div>
      {d.last_error ? (
        <Alert tone="warning">
          Last error (<Time iso={d.last_error.at} />): {d.last_error.message}
        </Alert>
      ) : (
        <p className={shared.note}>No collector error recorded.</p>
      )}
      <h3 className={shared.name}>Outage history</h3>
      <Table
        label="Outage history"
        rows={d.outages}
        rowKey={(o) => o.started_at}
        emptyMessage="No outages."
        columns={[
          { key: "s", header: `Started (${zoneLabel()})`, nowrap: true, render: (o) => <Time iso={o.started_at} /> },
          { key: "e", header: "Ended", nowrap: true, render: (o) => (o.ended_at ? <Time iso={o.ended_at} /> : "Ongoing") },
          { key: "d", header: "Duration (min)", render: (o) => o.duration_minutes },
        ]}
      />
      <h3 className={shared.name}>Recent failed event deliveries</h3>
      <Table
        label="Failed event deliveries"
        rows={d.failed_deliveries}
        rowKey={(f) => f.id}
        emptyMessage="No failed deliveries."
        columns={[
          { key: "t", header: "Event", render: (f) => f.event_type },
          { key: "h", header: "Hook", render: (f) => f.hook },
          { key: "w", header: `When (${zoneLabel()})`, nowrap: true, render: (f) => <Time iso={f.started_at} /> },
          { key: "e", header: "Error", render: (f) => f.error },
        ]}
      />
    </div>
  );
}

export default function Page() {
  const settings = useApi(getSettings, [], { pollMs: POLL_MS });
  const collector = useApi(getCollector, [], { pollMs: POLL_MS });
  const rules = useApi(getRules, [], { pollMs: POLL_MS });
  const [formEdits, setFormEdits] = useState<SettingsForm | null>(null);
  const [ruleEdits, setRuleEdits] = useState<RuleRow[] | null>(null);
  const [localErrors, setLocalErrors] = useState<FieldErrors>({});
  const [saved, setSaved] = useState(false);
  const save = useAction();

  if (settings.loading || rules.loading)
    return (
      <div className="page">
        <h1>Settings</h1>
        <p role="status">Loading settings…</p>
      </div>
    );
  if (settings.error || rules.error || !settings.data || !rules.data)
    return (
      <div className="page">
        <h1>Settings</h1>
        <Alert tone="error">{(settings.error ?? rules.error)?.message ?? "Could not load settings."}</Alert>
        <div>
          <Button
            onClick={() => {
              settings.reload();
              rules.reload();
            }}
          >
            Retry
          </Button>
        </div>
      </div>
    );

  const { settings: serverSettings, runtime } = settings.data;
  const baseForm = toForm(serverSettings);
  const baseRules = toRuleRows(rules.data);
  const form = formEdits ?? baseForm;
  const ruleRows = ruleEdits ?? baseRules;
  const formDirty = formEdits !== null && !sameJson(formEdits, baseForm);
  const rulesDirty = ruleEdits !== null && !sameJson(ruleEdits, baseRules);
  const dirty = formDirty || rulesDirty;
  const reportedDirty = formEdits !== null && !sameJson(reportedFields(formEdits), reportedFields(baseForm));

  const editForm = (patch: Partial<SettingsForm>) => {
    setSaved(false);
    setFormEdits({ ...form, ...patch });
  };
  const editRules = (rows: RuleRow[]) => {
    setSaved(false);
    setRuleEdits(rows);
  };
  const patchRule = (i: number, patch: Partial<RuleRow>) =>
    editRules(ruleRows.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const move = (i: number, d: -1 | 1) => {
    const rows = [...ruleRows];
    const [r] = rows.splice(i, 1);
    rows.splice(i + d, 0, r!);
    editRules(rows);
  };

  const submit = async () => {
    setSaved(false);
    const errs: FieldErrors = {};
    let ruleBody: ReturnType<typeof toRuleInputs> | undefined;
    let settingsBody: ReturnType<typeof toSettings> | undefined;
    if (rulesDirty) {
      ruleBody = toRuleInputs(ruleRows);
      if (!ruleBody.ok) Object.assign(errs, ruleBody.errors);
    }
    if (formDirty) {
      settingsBody = toSettings(form, reportedDirty);
      if (!settingsBody.ok) Object.assign(errs, settingsBody.errors);
    }
    setLocalErrors(errs);
    if (Object.keys(errs).length > 0) return;
    const ok = await save.run(async () => {
      if (ruleBody?.ok) await putRules(ruleBody.value);
      if (settingsBody?.ok) await putSettings(settingsBody.value);
    });
    if (ok) {
      setFormEdits(null);
      setRuleEdits(null);
      setSaved(true);
    }
    // Refetch either way: the rules may have saved even if the settings call failed.
    rules.reload();
    settings.reload();
    collector.reload();
  };

  const apiField = save.error?.field;
  const fieldError = (key: string): string | undefined =>
    localErrors[key] ?? (apiField === key ? save.error?.message : undefined);

  return (
    <div className="page">
      <div className={shared.titleRow}>
        <h1>Settings</h1>
        <div className={shared.titleRight}>
          <span className={shared.note}>Changes apply from the next poll, never retroactively</span>
          {dirty ? <span className={shared.dirty}>Unsaved changes</span> : null}
          <Button variant="primary" disabled={!dirty || save.busy} onClick={submit}>
            Save changes
          </Button>
        </div>
      </div>
      {saved ? <Alert tone="success">Settings saved. They apply from the next poll.</Alert> : null}
      {save.error ? <Alert tone="error">{errorText(save.error)}</Alert> : null}

      <section className={shared.region} aria-labelledby="collector-h">
        <h2 id="collector-h">Collector health</h2>
        <Collector c={collector} />
        <p className={shared.note}>
          Provider: {runtime.provider ?? "none"} · Pressure source: {runtime.pressure_source} · Metering policy:{" "}
          {runtime.metering_policy.name}
          {runtime.metering_policy.description ? ` (${runtime.metering_policy.description})` : ""} · Hooks:{" "}
          {runtime.hooks.length > 0 ? runtime.hooks.join(", ") : "none"} · OIDC issuer: {runtime.oidc_issuer ?? "none"}.
          Extensions, connection settings and secrets come from the config file and are read-only here.
        </p>
      </section>

      <div className={shared.columns}>
        <div className={shared.main}>
          <section className={shared.card} aria-labelledby="enf-h">
            <h2 id="enf-h">Enforcement and release</h2>
            <label className={shared.check}>
              <input
                type="checkbox"
                checked={form.enforcement}
                onChange={(e) => editForm({ enforcement: e.target.checked })}
              />
              Block acquiring at zero tokens
            </label>
            {form.enforcement ? (
              <p className={shared.note}>On: acquiring is refused when the balance is zero or below.</p>
            ) : (
              <Alert tone="warning">Shadow mode: usage is metered and balances fall, nobody is blocked.</Alert>
            )}
            <label className={shared.check}>
              <input type="checkbox" checked={form.teardown} onChange={(e) => editForm({ teardown: e.target.checked })} />
              Require teardown before release
            </label>
            <p className={shared.note}>
              A release while the provider still reports the setup is only requested; the hold is released when the setup is gone. No effect without a provider.
            </p>
          </section>

          <section className={shared.card} aria-labelledby="allow-h">
            <h2 id="allow-h">Allowance and timing</h2>
            <Field label="Monthly allowance (tokens)" error={fieldError("default_allowance")}>
              {(p) => <Input {...p} inputMode="decimal" value={form.allowance} onChange={(e) => editForm({ allowance: e.target.value })} />}
            </Field>
            <Field label="Max setups per developer" error={fieldError("max_holds_per_user")}>
              {(p) => <Input {...p} inputMode="numeric" value={form.maxHolds} onChange={(e) => editForm({ maxHolds: e.target.value })} />}
            </Field>
            <Field label="Poll interval" hint="A duration such as 5m; at least 1m." error={fieldError("poll_interval")}>
              {(p) => <Input {...p} value={form.poll} onChange={(e) => editForm({ poll: e.target.value })} />}
            </Field>
          </section>
        </div>

        <div className={shared.main}>
          <ReportedWeights
            form={form}
            edit={editForm}
            collector={collector}
            pollInterval={serverSettings.poll_interval}
            fieldError={fieldError}
          />

          <section className={shared.card} aria-labelledby="bands-h">
            <h2 id="bands-h">Pressure bands</h2>
            <div className={shared.row}>
              <Field label="Base multiplier" hint="Below the first band." error={fieldError("bands.base")}>
                {(p) => <Input {...p} className={shared.num} inputMode="decimal" value={form.baseMultiplier} onChange={(e) => editForm({ baseMultiplier: e.target.value })} />}
              </Field>
            </div>
            {form.bands.map((b, i) => (
              <div className={shared.rowBottom} key={i}>
                <Field label={`From pressure ${i + 1} (0 to 1)`} error={fieldError(`bands.steps[${i}].from`)}>
                  {(p) => (
                    <Input
                      {...p}
                      className={shared.num}
                      inputMode="decimal"
                      value={b.from}
                      onChange={(e) =>
                        editForm({ bands: form.bands.map((x, j) => (j === i ? { ...x, from: e.target.value } : x)) })
                      }
                    />
                  )}
                </Field>
                <Field label={`Multiplier ${i + 1}`} error={fieldError(`bands.steps[${i}].multiplier`)}>
                  {(p) => (
                    <Input
                      {...p}
                      className={shared.num}
                      inputMode="decimal"
                      value={b.multiplier}
                      onChange={(e) =>
                        editForm({ bands: form.bands.map((x, j) => (j === i ? { ...x, multiplier: e.target.value } : x)) })
                      }
                    />
                  )}
                </Field>
                <Button
                  onClick={() => editForm({ bands: form.bands.filter((_, j) => j !== i) })}
                  aria-label={`Remove band ${i + 1}`}
                >
                  Remove
                </Button>
              </div>
            ))}
            <div>
              <Button onClick={() => editForm({ bands: [...form.bands, { from: "", multiplier: "" }] })}>Add band</Button>
            </div>
          </section>

          <section className={shared.card} aria-labelledby="rules-h">
            <h2 id="rules-h">Rate rules</h2>
            <p className={shared.note}>First matching rule sets a setup's weight: tokens per hour at 1×.</p>
            <div className={shared.row}>
              <Field label="Default weight" hint="Used when no rule matches." error={fieldError("default_weight")}>
                {(p) => <Input {...p} className={shared.num} inputMode="decimal" value={form.defaultWeight} onChange={(e) => editForm({ defaultWeight: e.target.value })} />}
              </Field>
            </div>
            {ruleRows.length === 0 ? (
              <p className={shared.note}>No rules: every setup uses the default weight.</p>
            ) : (
              <div style={{ overflowX: "auto" }}>
                <table aria-label="Rate rules" style={{ width: "100%", borderCollapse: "collapse" }}>
                  <thead>
                    <tr>
                      <th scope="col">#</th>
                      <th scope="col">Label selector</th>
                      <th scope="col">Weight</th>
                      <th scope="col">Matches</th>
                      <th scope="col">
                        <span className="sr-only">Actions</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {ruleRows.map((r, i) => {
                      const selErr = fieldError(`rules[${i}].selector`);
                      const wErr = fieldError(`rules[${i}].weight`);
                      return (
                        <tr key={r.key}>
                          <td>{i + 1}</td>
                          <td>
                            <Input
                              aria-label={`Selector ${i + 1}`}
                              aria-invalid={selErr ? true : undefined}
                              value={r.selector}
                              onChange={(e) => patchRule(i, { selector: e.target.value })}
                            />
                            {selErr ? (
                              <div role="alert" className={shared.overdrawn}>
                                {selErr}
                              </div>
                            ) : null}
                          </td>
                          <td>
                            <Input
                              aria-label={`Weight ${i + 1}`}
                              aria-invalid={wErr ? true : undefined}
                              className={shared.num}
                              inputMode="decimal"
                              value={r.weight}
                              onChange={(e) => patchRule(i, { weight: e.target.value })}
                            />
                            {wErr ? <div className={shared.overdrawn}>{wErr}</div> : null}
                          </td>
                          <td>
                            {r.matchCount === undefined
                              ? "After saving"
                              : `${r.matchCount} setup${r.matchCount === 1 ? "" : "s"}`}
                          </td>
                          <td>
                            <div className={shared.row}>
                              <Button disabled={i === 0} onClick={() => move(i, -1)} aria-label={`Move rule ${i + 1} up`}>
                                Up
                              </Button>
                              <Button
                                disabled={i === ruleRows.length - 1}
                                onClick={() => move(i, 1)}
                                aria-label={`Move rule ${i + 1} down`}
                              >
                                Down
                              </Button>
                              <Button
                                onClick={() => editRules(ruleRows.filter((_, j) => j !== i))}
                                aria-label={`Remove rule ${i + 1}`}
                              >
                                Remove
                              </Button>
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
            <div>
              <Button onClick={() => editRules([...ruleRows, { key: `n${newKey++}`, selector: "", weight: "1" }])}>
                Add rule
              </Button>
            </div>
            {rulesDirty ? <p className={shared.note}>Match counts reflect the saved rules until you save.</p> : null}
            <h3 className={shared.name}>Setups using the default weight</h3>
            {rules.data.unmatched_setups.length === 0 ? (
              <p className={shared.note}>Every enabled setup matches a rule.</p>
            ) : (
              <ul aria-label="Setups using the default weight">
                {rules.data.unmatched_setups.map((s) => (
                  <li key={s.id}>{s.name}</li>
                ))}
              </ul>
            )}
            <p className={shared.note}>
              Selectors: key=value, key!=value, key in (a,b), key notin (a,b), key, !key; commas mean AND.
            </p>
          </section>
        </div>
      </div>
    </div>
  );
}
