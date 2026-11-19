// SPDX-License-Identifier: MIT
import { useState } from "react";
import { Alert, Button, ConfirmDialog, Field, Input } from "../../components";
import { usePanelFocus } from "../../hooks/usePanelFocus";
import { errorText, useAction } from "../admin-shared/useAction";
import shared from "../admin-shared/admin.module.css";
import { createSetup, deleteSetup, forceRelease, updateSetup, type Setup, type SetupInput } from "./api";

interface LabelRow {
  key: string;
  value: string;
}

export function parseJson(text: string): { ok: true; value: unknown } | { ok: false; message: string } {
  try {
    return { ok: true, value: JSON.parse(text) as unknown };
  } catch (e) {
    return { ok: false, message: e instanceof Error ? e.message : "Invalid JSON" };
  }
}

/** The editable fields of a setup as the API wants them back (PUT replaces them all). */
export function toInput(s: Setup, over: Partial<SetupInput> = {}): SetupInput {
  return {
    name: s.name,
    labels: s.labels,
    details: s.details ?? null,
    enabled: s.admin?.enabled ?? s.state !== "disabled",
    resource_name_override: s.admin?.resource_name_override ?? null,
    ...over,
  };
}

interface Props {
  /** Undefined means "Add setup". */
  setup?: Setup;
  onSaved: (message: string) => void;
  onClose: () => void;
}

export function SetupForm({ setup, onSaved, onClose }: Props) {
  const [name, setName] = useState(setup?.name ?? "");
  const [labels, setLabels] = useState<LabelRow[]>(
    Object.entries(setup?.labels ?? {}).map(([key, value]) => ({ key, value })),
  );
  const [override, setOverride] = useState(setup?.admin?.resource_name_override ?? "");
  const [details, setDetails] = useState(setup ? JSON.stringify(setup.details ?? null, null, 2) : "{}");
  const [enabled, setEnabled] = useState(setup?.admin?.enabled ?? true);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [forcing, setForcing] = useState(false);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string>();
  const save = useAction();
  const del = useAction();
  const force = useAction();
  const panelRef = usePanelFocus<HTMLElement>(onClose);

  const parsed = parseJson(details);

  const setLabel = (i: number, patch: Partial<LabelRow>) =>
    setLabels((rows) => rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));

  const submit = async () => {
    if (!parsed.ok) return;
    const map: Record<string, string> = {};
    for (const r of labels) if (r.key !== "" || r.value !== "") map[r.key] = r.value;
    const body: SetupInput = {
      name: name.trim(),
      labels: map,
      details: parsed.value,
      enabled,
      resource_name_override: override.trim() === "" ? null : override.trim(),
    };
    const ok = await save.run(() => (setup ? updateSetup(setup.id, body) : createSetup(body)));
    if (ok) onSaved(setup ? `Saved ${body.name}.` : `Added ${body.name}.`);
  };

  const doDelete = async () => {
    if (setup && (await del.run(() => deleteSetup(setup.id)))) onSaved(`Deleted ${setup.name}.`);
  };

  const doForce = async () => {
    if (!setup?.holder) return;
    if (reason.trim() === "") {
      setReasonError("A reason is required.");
      return;
    }
    const holdId = setup.holder.hold_id;
    if (await force.run(() => forceRelease(holdId, reason.trim()))) {
      setForcing(false);
      onSaved(`Force-released ${setup.name}.`);
    }
  };

  const title = setup ? `Edit ${setup.name}` : "Add setup";
  return (
    <aside ref={panelRef} className={`${shared.panel} ${shared.side}`} aria-label={setup ? "Edit setup" : "Add setup"}>
      <h2>{title}</h2>
      <Field label="Name">{(p) => <Input {...p} value={name} onChange={(e) => setName(e.target.value)} />}</Field>

      <fieldset className={shared.fieldset}>
        <legend className={shared.legend}>Labels</legend>
        {labels.map((r, i) => (
          <div className={shared.row} key={i}>
            <Input
              aria-label={`Label key ${i + 1}`}
              value={r.key}
              onChange={(e) => setLabel(i, { key: e.target.value })}
              style={{ flex: "1 1 120px", width: "auto" }}
            />
            <Input
              aria-label={`Label value ${i + 1}`}
              value={r.value}
              onChange={(e) => setLabel(i, { value: e.target.value })}
              style={{ flex: "1 1 120px", width: "auto" }}
            />
            <Button onClick={() => setLabels((rows) => rows.filter((_, j) => j !== i))} aria-label={`Remove label ${i + 1}`}>
              Remove
            </Button>
          </div>
        ))}
        <div>
          <Button onClick={() => setLabels((rows) => [...rows, { key: "", value: "" }])}>Add label</Button>
        </div>
        <p className={shared.note}>Free-form key and value; the server checks the limits on save.</p>
      </fieldset>

      <Field
        label="Resource name override"
        hint={setup?.admin ? `Blank uses the template: ${setup.admin.resource_name}` : "Blank uses the template."}
      >
        {(p) => <Input {...p} value={override} onChange={(e) => setOverride(e.target.value)} />}
      </Field>

      <Field
        label="Details (JSON, stored as-is, passed to hooks)"
        error={parsed.ok ? undefined : `Invalid JSON: ${parsed.message}`}
        hint={parsed.ok ? "Valid JSON" : undefined}
      >
        {(p) => (
          <>
            <textarea
              {...p}
              className={shared.textarea}
              rows={7}
              value={details}
              onChange={(e) => setDetails(e.target.value)}
            />
            <div>
              <Button
                disabled={!parsed.ok}
                onClick={() => parsed.ok && setDetails(JSON.stringify(parsed.value, null, 2))}
              >
                Pretty-print
              </Button>
            </div>
          </>
        )}
      </Field>

      <label className={shared.check}>
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
        Enabled (counts toward lab pressure, can be acquired)
      </label>

      {save.error ? <Alert tone="error">{errorText(save.error)}</Alert> : null}
      <div className={shared.row}>
        <Button variant="primary" disabled={save.busy || !parsed.ok} onClick={submit}>
          Save
        </Button>
        {setup?.holder ? (
          <Button
            onClick={() => {
              setReason("");
              setReasonError(undefined);
              force.clear();
              setForcing(true);
            }}
          >
            Force-release
          </Button>
        ) : null}
        {setup ? (
          <Button
            variant="destructive"
            onClick={() => {
              del.clear();
              setConfirmDelete(true);
            }}
          >
            Delete
          </Button>
        ) : null}
        <Button onClick={onClose}>Cancel</Button>
      </div>
      {setup ? <p className={shared.note}>Delete is blocked while the setup is held.</p> : null}

      <ConfirmDialog
        open={confirmDelete}
        title={`Delete ${setup?.name ?? ""}?`}
        confirmLabel="Delete setup"
        destructive
        busy={del.busy}
        error={del.error ? errorText(del.error) : undefined}
        onConfirm={doDelete}
        onCancel={() => setConfirmDelete(false)}
      >
        This removes the setup. It cannot be undone.
      </ConfirmDialog>

      <ConfirmDialog
        open={forcing}
        title={`Force-release ${setup?.name ?? ""}?`}
        confirmLabel="Force-release"
        destructive
        busy={force.busy}
        error={force.error ? errorText(force.error) : undefined}
        onConfirm={doForce}
        onCancel={() => setForcing(false)}
      >
        <Field label="Reason (required)" error={reasonError}>
          {(p) => <Input {...p} value={reason} onChange={(e) => setReason(e.target.value)} />}
        </Field>
      </ConfirmDialog>
    </aside>
  );
}
