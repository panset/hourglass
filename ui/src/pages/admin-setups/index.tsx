// SPDX-License-Identifier: MIT
// Setup inventory (SET-1..SET-6, SET-5). Every value shown is an API field (UI-1).
import { useState } from "react";
import { Alert, Badge, Button, Chip, Clip, Table, Time, zoneLabel } from "../../components";
import { useApi } from "../../hooks/useApi";
import { errorText, useAction } from "../admin-shared/useAction";
import shared from "../admin-shared/admin.module.css";
import { attributesLine, notAppliedText, WeightTag } from "../usage/weight";
import { getCollector, listSetups, updateSetup, type Orphan, type Setup } from "./api";
import { SetupForm, toInput } from "./SetupForm";

const POLL_MS = 60_000;

function DisableButton({ setup, onDone }: { setup: Setup; onDone: () => void }) {
  const act = useAction();
  const enabled = setup.admin?.enabled ?? false;
  const toggle = async () => {
    if (await act.run(() => updateSetup(setup.id, toInput(setup, { enabled: !enabled })))) onDone();
  };
  return (
    <>
      <Button disabled={act.busy} onClick={toggle} aria-label={`${enabled ? "Disable" : "Enable"} ${setup.name}`}>
        {enabled ? "Disable" : "Enable"}
      </Button>
      {act.error ? <Alert tone="error">{errorText(act.error)}</Alert> : null}
    </>
  );
}

export default function Page() {
  const setups = useApi(listSetups, [], { pollMs: POLL_MS });
  const collector = useApi(getCollector, [], { pollMs: POLL_MS });
  const [editing, setEditing] = useState<number | "new">();
  const [notice, setNotice] = useState<string>();

  const refetch = () => {
    setups.reload();
    collector.reload();
  };

  const header = (
    <div className={shared.titleRow}>
      <h1>Setup inventory</h1>
      <Button
        variant="primary"
        onClick={() => {
          setNotice(undefined);
          setEditing("new");
        }}
      >
        Add setup
      </Button>
    </div>
  );

  if (setups.loading)
    return (
      <div className="page">
        {header}
        <p role="status">Loading setups…</p>
      </div>
    );
  if (setups.error || !setups.data)
    return (
      <div className="page">
        {header}
        <Alert tone="error">{setups.error?.message ?? "Could not load setups."}</Alert>
        <div>
          <Button onClick={setups.reload}>Retry</Button>
        </div>
      </div>
    );

  const rows = setups.data.setups;
  const current = typeof editing === "number" ? rows.find((s) => s.id === editing) : undefined;

  return (
    <div className="page">
      {header}
      {notice ? <Alert tone="success">{notice}</Alert> : null}
      <div className={shared.columns}>
        <div className={shared.main}>
          <Table<Setup>
            label="Setups"
            rows={rows}
            rowKey={(s) => s.id}
            emptyMessage="No setups yet. Add one to start."
            columns={[
              { key: "name", header: "Setup", nowrap: true, render: (s) => <span className={shared.name}><Clip text={s.name} max={32} /></span> },
              {
                key: "labels",
                header: "Labels",
                render: (s) => (
                  <div className={shared.chips}>
                    {Object.entries(s.labels).map(([k, v]) => (
                      <Chip key={k}>{`${k}=${v}`}</Chip>
                    ))}
                  </div>
                ),
              },
              {
                key: "weight",
                header: "Weight",
                render: (s) => {
                  const attrs = attributesLine(s.weight_attributes);
                  const reported = s.admin?.reported_weight;
                  const why = notAppliedText(s.admin?.reported_weight_not_applied);
                  return (
                    <>
                      {s.weight}
                      <WeightTag source={s.weight_source} />
                      {reported !== null && reported !== undefined ? (
                        <span className={shared.sub}>
                          <br />
                          Reported weight {reported}
                          {why ? `. ${why}` : ""}
                        </span>
                      ) : null}
                      {attrs ? (
                        <span className={shared.sub}>
                          <br />
                          {attrs}
                        </span>
                      ) : null}
                    </>
                  );
                },
              },
              {
                key: "seen",
                header: `Last seen by provider (${zoneLabel()})`,
                nowrap: true,
                render: (s) => (s.admin?.last_seen_at ? <Time iso={s.admin.last_seen_at} /> : "Not deployed"),
              },
              { key: "held", header: "Held by", render: (s) => s.holder?.name ?? "" },
              {
                key: "status",
                header: "Status",
                render: (s) =>
                  s.admin?.enabled ? <Badge tone="green">Enabled</Badge> : <Badge tone="grey">Disabled</Badge>,
              },
              {
                key: "act",
                header: "",
                render: (s) => (
                  <div className={shared.row}>
                    <Button
                      onClick={() => {
                        setNotice(undefined);
                        setEditing(s.id);
                      }}
                      aria-label={`Edit ${s.name}`}
                    >
                      Edit
                    </Button>
                    <DisableButton setup={s} onDone={refetch} />
                  </div>
                ),
              },
            ]}
          />

          <section className={shared.region} aria-labelledby="orphans-h">
            <h2 id="orphans-h">Orphans</h2>
            {collector.error ? (
              <Alert tone="error">{collector.error.message}</Alert>
            ) : (
              <Table<Orphan>
                label="Orphans"
                rows={collector.data?.orphans ?? []}
                rowKey={(o) => o.resource_name}
                emptyMessage="No orphans."
                columns={[
                  { key: "res", header: "Resource name reported", render: (o) => o.resource_name },
                  { key: "first", header: `First seen (${zoneLabel()})`, nowrap: true, render: (o) => <Time iso={o.first_seen_at} /> },
                  {
                    key: "match",
                    header: "Matches",
                    render: (o) => (o.setup ? `${o.setup.name} exists but nobody holds it` : "No setup matches"),
                  },
                ]}
              />
            )}
            <p className={shared.note}>Something is deployed that nobody holds. Orphans are charged to nobody.</p>
          </section>
        </div>

        {editing === "new" || current ? (
          <SetupForm
            key={current?.id ?? "new"}
            setup={current}
            onClose={() => setEditing(undefined)}
            onSaved={(message) => {
              setEditing(undefined);
              setNotice(message);
              refetch();
            }}
          />
        ) : null}
      </div>
    </div>
  );
}
