// SPDX-License-Identifier: MIT
// Audit log (ADM-3). No wireframe: built from the spec row.
import { useState } from "react";
import { Alert, Button, Field, Input, Select, Table, Time, zoneLabel } from "../../components";
import { useApi } from "../../hooks/useApi";
import shared from "../admin-shared/admin.module.css";
import { listActors, listAudit, type AuditEntry } from "./api";

export const PAGE_SIZE = 50;
const POLL_MS = 60_000;

function Details({ entry }: { entry: AuditEntry }) {
  const [open, setOpen] = useState(false);
  return (
    <div className={shared.region}>
      <Button aria-expanded={open} aria-label={`${open ? "Hide" : "Show"} before and after for entry ${entry.id}`} onClick={() => setOpen(!open)}>
        {open ? "Hide" : "Show"}
      </Button>
      {open ? (
        <div className={shared.row} style={{ alignItems: "flex-start" }}>
          <div className={shared.wide}>
            <div className={shared.sub}>Before</div>
            <pre className={shared.pre}>{JSON.stringify(entry.before, null, 2)}</pre>
          </div>
          <div className={shared.wide}>
            <div className={shared.sub}>After</div>
            <pre className={shared.pre}>{JSON.stringify(entry.after, null, 2)}</pre>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default function Page() {
  const [actor, setActor] = useState("");
  const [action, setAction] = useState("");
  const [applied, setApplied] = useState<{ actor?: number; action?: string }>({});
  const [offset, setOffset] = useState(0);

  const actors = useApi(listActors);
  const audit = useApi(
    () => listAudit({ ...applied, limit: PAGE_SIZE, offset }),
    [applied.actor, applied.action, offset],
    { pollMs: POLL_MS },
  );

  const apply = () => {
    setOffset(0);
    setApplied({ actor: actor === "" ? undefined : Number(actor), action: action.trim() === "" ? undefined : action.trim() });
  };

  const filters = (
    <form
      className={shared.row}
      onSubmit={(e) => {
        e.preventDefault();
        apply();
      }}
      aria-label="Audit filters"
    >
      <Field label="Actor">
        {(p) => (
          <Select {...p} value={actor} onChange={(e) => setActor(e.target.value)}>
            <option value="">All actors</option>
            {(actors.data?.users ?? []).map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </Select>
        )}
      </Field>
      <Field label="Action" hint="Exact name, e.g. grant.create">
        {(p) => <Input {...p} value={action} onChange={(e) => setAction(e.target.value)} />}
      </Field>
      <Button type="submit" variant="primary">
        Apply filters
      </Button>
    </form>
  );

  let body;
  if (audit.loading) body = <p role="status">Loading audit log…</p>;
  else if (audit.error || !audit.data)
    body = (
      <>
        <Alert tone="error">{audit.error?.message ?? "Could not load the audit log."}</Alert>
        <div>
          <Button onClick={audit.reload}>Retry</Button>
        </div>
      </>
    );
  else {
    const d = audit.data;
    const shown = d.entries.length;
    body = (
      <>
        <Table<AuditEntry>
          label="Audit log"
          rows={d.entries}
          rowKey={(e) => e.id}
          emptyMessage="No audit entries match."
          columns={[
            { key: "time", header: `Time (${zoneLabel()})`, nowrap: true, render: (e) => <Time iso={e.created_at} /> },
            { key: "actor", header: "Actor", render: (e) => e.actor.name },
            { key: "action", header: "Action", render: (e) => <code>{e.action}</code> },
            { key: "target", header: "Target", render: (e) => <code>{e.target}</code> },
            { key: "detail", header: "Before / after", render: (e) => <Details entry={e} /> },
          ]}
        />
        <div className={shared.row}>
          <Button disabled={d.offset <= 0} onClick={() => setOffset(Math.max(0, d.offset - d.limit))}>
            Previous
          </Button>
          <span className={shared.note}>
            {shown === 0 ? "0" : `${d.offset + 1} to ${d.offset + shown}`} of {d.total}
          </span>
          <Button disabled={d.offset + shown >= d.total} onClick={() => setOffset(d.offset + d.limit)}>
            Next
          </Button>
        </div>
      </>
    );
  }

  return (
    <div className="page">
      <h1>Audit log</h1>
      {filters}
      {body}
    </div>
  );
}
