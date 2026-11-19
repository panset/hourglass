// SPDX-License-Identifier: MIT
// Users and grants (ADM-1, ADM-5, LED-4, LED-5). Every number shown is an API field (UI-1).
import { useState } from "react";
import { Alert, Badge, Button, ConfirmDialog, Field, Input, Table } from "../../components";
import { useApi } from "../../hooks/useApi";
import { formatDateTime, formatPerHour, formatTokens } from "../../lib/format";
import { errorText, useAction } from "../admin-shared/useAction";
import shared from "../admin-shared/admin.module.css";
import { grantTokens, listPending, listUsers, rejectUser, resetBalance, setRoles, type AdminUser, type Role } from "./api";

const ROLES: Role[] = ["developer", "admin"];
const POLL_MS = 60_000;
const LABEL: Record<Role, string> = { developer: "Developer", admin: "Admin" };

function RoleBoxes({ id, value, onChange }: { id: string; value: Role[]; onChange: (r: Role[]) => void }) {
  return (
    <div className={shared.row} role="group" aria-label={`Roles for ${id}`}>
      {ROLES.map((r) => (
        <label key={r} className={shared.check}>
          <input
            type="checkbox"
            checked={value.includes(r)}
            onChange={(e) => onChange(e.target.checked ? [...value, r] : value.filter((x) => x !== r))}
          />
          {LABEL[r]}
        </label>
      ))}
    </div>
  );
}

function PendingRow({ user, onDone }: { user: AdminUser; onDone: () => void }) {
  const [roles, setPicked] = useState<Role[]>([]);
  const act = useAction();
  const approve = async () => {
    if (await act.run(() => setRoles(user.id, roles))) onDone();
  };
  const reject = async () => {
    if (await act.run(() => rejectUser(user.id))) onDone();
  };
  return (
    <li className={shared.card}>
      <div className={shared.row}>
        <div className={shared.wide}>
          <div className={shared.name}>{user.name}</div>
          <div className={shared.sub}>{user.email}</div>
        </div>
        <div className={shared.sub}>First sign-in {formatDateTime(user.first_sign_in_at)}</div>
        <RoleBoxes id={user.name} value={roles} onChange={setPicked} />
        <Button
          variant="primary"
          disabled={act.busy || roles.length === 0}
          onClick={approve}
          aria-label={`Approve ${user.name}`}
        >
          Approve
        </Button>
        <Button disabled={act.busy} onClick={reject} aria-label={`Reject ${user.name}`}>
          Reject
        </Button>
      </div>
      {act.error ? <Alert tone="error">{errorText(act.error)}</Alert> : null}
    </li>
  );
}

function RoleCell({ user, onDone }: { user: AdminUser; onDone: () => void }) {
  const [roles, setPicked] = useState<Role[]>(user.roles);
  const act = useAction();
  const changed = roles.length !== user.roles.length || roles.some((r) => !user.roles.includes(r));
  const save = async () => {
    if (await act.run(() => setRoles(user.id, roles))) onDone();
  };
  return (
    <div className={shared.region}>
      <RoleBoxes id={user.name} value={roles} onChange={setPicked} />
      {user.status === "rejected" ? <Badge tone="grey">Rejected</Badge> : null}
      {changed ? (
        <Button disabled={act.busy} onClick={save} aria-label={`Save roles for ${user.name}`}>
          Save roles
        </Button>
      ) : null}
      {act.error ? <Alert tone="error">{errorText(act.error)}</Alert> : null}
    </div>
  );
}

function GrantPanel({
  user,
  list,
  onClose,
  onDone,
}: {
  user: AdminUser;
  list: { default_allowance_tokens: number; period_ends_at: string };
  onClose: () => void;
  onDone: (message: string) => void;
}) {
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [local, setLocal] = useState<{ amount?: string; reason?: string }>({});
  const [confirmReset, setConfirmReset] = useState(false);
  const grant = useAction();
  const reset = useAction();

  const submitGrant = async () => {
    const n = Number(amount);
    const errs: typeof local = {};
    if (amount.trim() === "" || !Number.isFinite(n) || n === 0) errs.amount = "Enter a non-zero number.";
    if (reason.trim() === "") errs.reason = "A reason is required.";
    setLocal(errs);
    if (errs.amount || errs.reason) return;
    if (await grant.run(() => grantTokens(user.id, { amount_tokens: n, reason: reason.trim() })))
      onDone(`Granted ${formatTokens(n, { signed: true })} tokens to ${user.name}.`);
  };
  const askReset = () => {
    if (reason.trim() === "") {
      setLocal({ reason: "A reason is required." });
      return;
    }
    setLocal({});
    reset.clear();
    setConfirmReset(true);
  };
  const doReset = async () => {
    if (await reset.run(() => resetBalance(user.id, reason.trim()))) {
      setConfirmReset(false);
      onDone(`Reset ${user.name} to ${formatTokens(list.default_allowance_tokens)} tokens.`);
    }
  };
  const holds = user.active_holds.map((h) => h.setup_name).join(", ");

  return (
    <aside className={`${shared.panel} ${shared.side}`} aria-label="Grant tokens">
      <h2>Grant tokens to {user.name}</h2>
      <p className={shared.sub}>
        Balance {formatTokens(user.balance_tokens)}
        {holds ? ` · burning ${formatPerHour(user.burn_per_hour_tokens)} on ${holds}` : ""}
      </p>
      <Field label="Amount" error={local.amount} hint="Negative amounts correct mistakes.">
        {(p) => <Input {...p} inputMode="decimal" value={amount} onChange={(e) => setAmount(e.target.value)} />}
      </Field>
      <Field label="Reason (required)" error={local.reason}>
        {(p) => <Input {...p} value={reason} onChange={(e) => setReason(e.target.value)} />}
      </Field>
      <p className={shared.sub}>
        Unused tokens expire {formatDateTime(list.period_ends_at)}; debt carries into the next period. Both actions
        are written to the audit log.
      </p>
      {grant.error ? <Alert tone="error">{errorText(grant.error)}</Alert> : null}
      <div className={shared.row}>
        <Button variant="primary" disabled={grant.busy} onClick={submitGrant}>
          Grant tokens
        </Button>
        <Button disabled={grant.busy || reset.busy} onClick={askReset}>
          Reset to {formatTokens(list.default_allowance_tokens)}
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </div>
      <ConfirmDialog
        open={confirmReset}
        title={`Reset ${user.name}'s balance?`}
        confirmLabel="Reset balance"
        busy={reset.busy}
        error={reset.error ? errorText(reset.error) : undefined}
        onConfirm={doReset}
        onCancel={() => setConfirmReset(false)}
      >
        Sets the balance to {formatTokens(list.default_allowance_tokens)} tokens and clears carried debt. Reason: {reason}
      </ConfirmDialog>
    </aside>
  );
}

export default function Page() {
  const users = useApi(listUsers, [], { pollMs: POLL_MS });
  const pending = useApi(listPending, [], { pollMs: POLL_MS });
  const [grantFor, setGrantFor] = useState<number>();
  const [notice, setNotice] = useState<string>();

  const refetch = () => {
    users.reload();
    pending.reload();
  };

  if (users.loading || pending.loading)
    return (
      <div className="page">
        <h1>Users</h1>
        <p role="status">Loading users…</p>
      </div>
    );
  const failure = users.error ?? pending.error;
  if (failure || !users.data || !pending.data)
    return (
      <div className="page">
        <h1>Users</h1>
        <Alert tone="error">{failure?.message ?? "Could not load users."}</Alert>
        <div>
          <Button onClick={refetch}>Retry</Button>
        </div>
      </div>
    );

  const list = users.data;
  const everyone = list.users.filter((u) => u.status !== "pending");
  const target = grantFor === undefined ? undefined : everyone.find((u) => u.id === grantFor);

  return (
    <div className="page">
      <h1>Users</h1>
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      <section className={shared.region} aria-labelledby="pending-h">
        <h2 id="pending-h">Pending access requests</h2>
        {pending.data.users.length === 0 ? (
          <p className={shared.note}>No pending access requests.</p>
        ) : (
          <ul style={{ listStyle: "none", margin: 0, padding: 0 }} className={shared.region}>
            {pending.data.users.map((u) => (
              <PendingRow
                key={u.id}
                user={u}
                onDone={() => {
                  setNotice(undefined);
                  refetch();
                }}
              />
            ))}
          </ul>
        )}
        <p className={shared.note}>
          New sign-ins have no role and see only the not-authorized page until approved here. Approving credits the
          monthly allowance. Rejected users drop off this list.
        </p>
      </section>

      <div className={shared.columns}>
        <section className={shared.main} aria-labelledby="all-h">
          <h2 id="all-h">All users</h2>
          <Table<AdminUser>
            label="Users"
            rows={everyone}
            rowKey={(u) => u.id}
            emptyMessage="No users yet."
            columns={[
              {
                key: "user",
                header: "User",
                render: (u) => (
                  <>
                    <div className={shared.name}>{u.name}</div>
                    <div className={shared.sub}>{u.is_local_admin ? "Local account · first user" : u.email}</div>
                  </>
                ),
              },
              {
                key: "role",
                header: "Role",
                render: (u) =>
                  u.is_local_admin ? <Badge tone="grey">Admin · fixed</Badge> : <RoleCell key={u.roles.join()} user={u} onDone={refetch} />,
              },
              {
                key: "tokens",
                header: "Tokens left",
                render: (u) =>
                  u.is_local_admin ? (
                    "No balance"
                  ) : (
                    <span className={u.overdrawn ? shared.overdrawn : undefined}>{formatTokens(u.balance_tokens)}</span>
                  ),
              },
              {
                key: "burn",
                header: "Burned this month",
                render: (u) => (u.is_local_admin ? "" : formatTokens(u.month_to_date_burn_tokens)),
              },
              {
                key: "holding",
                header: "Holding",
                render: (u) =>
                  u.is_local_admin ? "Cannot hold setups" : u.active_holds.map((h) => h.setup_name).join(", "),
              },
              {
                key: "act",
                header: "",
                render: (u) =>
                  u.is_local_admin || u.balance_tokens === null ? null : (
                    <Button
                      onClick={() => {
                        setNotice(undefined);
                        setGrantFor(u.id);
                      }}
                      aria-label={`Grant to ${u.name}`}
                    >
                      Grant
                    </Button>
                  ),
              },
            ]}
          />
          <p className={shared.note}>Overdrawn users sort first. An admin cannot remove their own admin role.</p>
        </section>

        {target ? (
          <GrantPanel
            key={target.id}
            user={target}
            list={list}
            onClose={() => setGrantFor(undefined)}
            onDone={(message) => {
              setGrantFor(undefined);
              setNotice(message);
              refetch();
            }}
          />
        ) : null}
      </div>
    </div>
  );
}
