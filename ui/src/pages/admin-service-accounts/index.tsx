// SPDX-License-Identifier: MIT
// Service accounts (SVC-6): accounts, scopes, tokens. A token is shown once, in the dialog that issued it, and is
// held only in that dialog's state: never in storage, the URL or a log (SVC-4).
import { useState } from "react";
import { Alert, Badge, Button, Chip, ConfirmDialog, Field, Input, Modal, Table, Time } from "../../components";
import { useApi } from "../../hooks/useApi";
import shared from "../admin-shared/admin.module.css";
import { errorText, useAction } from "../admin-shared/useAction";
import {
  createServiceAccount,
  deleteServiceAccount,
  issueServiceToken,
  listServiceAccounts,
  revokeServiceToken,
  SCOPES,
  updateServiceAccount,
  type IssuedServiceToken,
  type ServiceAccount,
  type ServiceScope,
  type ServiceToken,
} from "./api";

const POLL_MS = 60_000;

function ScopeBoxes({ value, onChange }: { value: ServiceScope[]; onChange: (s: ServiceScope[]) => void }) {
  return (
    <fieldset className={shared.fieldset}>
      <legend className={shared.legend}>Scopes</legend>
      {SCOPES.map(({ scope, hint }) => (
        <label key={scope} className={shared.check}>
          <input
            type="checkbox"
            checked={value.includes(scope)}
            onChange={(e) => onChange(e.target.checked ? [...value, scope] : value.filter((x) => x !== scope))}
          />
          <span>
            {scope} <span className={shared.sub}>{hint}</span>
          </span>
        </label>
      ))}
    </fieldset>
  );
}

const sorted = (s: ServiceScope[]) => SCOPES.map((x) => x.scope).filter((x) => s.includes(x));

/** Create (no `account`) or edit (an account) in one dialog. */
function AccountDialog({ account, onClose, onDone }: { account?: ServiceAccount; onClose: () => void; onDone: (msg: string) => void }) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState(account?.description ?? "");
  const [scopes, setScopes] = useState<ServiceScope[]>(account?.scopes ?? []);
  const [enabled, setEnabled] = useState(account?.enabled ?? true);
  const act = useAction();

  const save = async () => {
    const ok = await act.run(() =>
      account
        ? updateServiceAccount(account.id, { description: description.trim(), scopes: sorted(scopes), enabled })
        : createServiceAccount({ name: name.trim(), description: description.trim(), scopes: sorted(scopes) }),
    );
    if (ok) onDone(account ? `Saved ${account.name}.` : `Created ${name.trim()}.`);
  };
  const err = act.error;
  return (
    <Modal
      open
      title={account ? `Edit ${account.name}` : "Create service account"}
      onClose={onClose}
      footer={
        <>
          <Button variant="primary" disabled={act.busy || (!account && name.trim() === "")} onClick={() => void save()}>
            {account ? "Save changes" : "Create account"}
          </Button>
          <Button disabled={act.busy} onClick={onClose}>
            Cancel
          </Button>
        </>
      }
    >
      <div className={shared.region}>
        {account ? null : (
          <Field
            label="Name"
            hint="Letters, digits, dot, underscore and dash. It cannot be changed later."
            error={err?.field === "name" ? err.message : undefined}
          >
            {(p) => <Input {...p} value={name} onChange={(e) => setName(e.target.value)} />}
          </Field>
        )}
        <Field label="Description" error={err?.field === "description" ? err.message : undefined}>
          {(p) => <Input {...p} value={description} onChange={(e) => setDescription(e.target.value)} />}
        </Field>
        <ScopeBoxes value={scopes} onChange={setScopes} />
        {account ? (
          <label className={shared.check}>
            <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
            Enabled
          </label>
        ) : null}
        {err && err.field !== "name" && err.field !== "description" ? <Alert tone="error">{errorText(err)}</Alert> : null}
      </div>
    </Modal>
  );
}

/** Issue a token, then show it once. The token lives in this component's state only and goes with it on close. */
function IssueDialog({ account, onClose, onDone }: { account: ServiceAccount; onClose: () => void; onDone: () => void }) {
  const [expires, setExpires] = useState("");
  const [local, setLocal] = useState<string>();
  const [issued, setIssued] = useState<IssuedServiceToken>();
  const [copied, setCopied] = useState<"yes" | "no">();
  const act = useAction();

  const issue = async () => {
    let body: { expires_at?: string } = {};
    if (expires) {
      const t = new Date(expires);
      if (Number.isNaN(t.getTime()) || t.getTime() <= Date.now()) {
        setLocal("The expiry must be in the future.");
        return;
      }
      body = { expires_at: t.toISOString() };
    }
    setLocal(undefined);
    let got: IssuedServiceToken | undefined;
    if (await act.run(async () => (got = await issueServiceToken(account.id, body)))) {
      setIssued(got);
      onDone();
    }
  };
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(issued!.token);
      setCopied("yes");
    } catch {
      setCopied("no");
    }
  };
  const close = () => {
    setIssued(undefined); // the token does not outlive the dialog
    onClose();
  };

  if (issued)
    return (
      <Modal
        open
        title={`Token for ${account.name}`}
        onClose={close}
        footer={
          <>
            <Button onClick={() => void copy()}>Copy token</Button>
            <Button variant="primary" onClick={close}>
              Close
            </Button>
          </>
        }
      >
        <div className={shared.region}>
          <Alert tone="warning">
            This is the only time the token is shown. Hourglass keeps only a hash and cannot show it again. Copy it now
            and store it where the service reads its secrets.
          </Alert>
          <Field label="Token">
            {(p) => <Input {...p} readOnly value={issued.token} onFocus={(e) => e.currentTarget.select()} />}
          </Field>
          {copied === "yes" ? <p role="status" className={shared.sub}>Copied.</p> : null}
          {copied === "no" ? <p role="status" className={shared.sub}>Could not copy. Select the token and copy it by hand.</p> : null}
        </div>
      </Modal>
    );

  return (
    <Modal
      open
      title={`Issue token for ${account.name}`}
      onClose={close}
      footer={
        <>
          <Button variant="primary" disabled={act.busy} onClick={() => void issue()}>
            Issue token
          </Button>
          <Button disabled={act.busy} onClick={close}>
            Cancel
          </Button>
        </>
      }
    >
      <div className={shared.region}>
        <Field label="Expires (optional)" hint="Leave empty for a token that does not expire." error={local ?? (act.error?.field === "expires_at" ? act.error.message : undefined)}>
          {(p) => <Input {...p} type="datetime-local" value={expires} onChange={(e) => setExpires(e.target.value)} />}
        </Field>
        {act.error && act.error.field !== "expires_at" ? <Alert tone="error">{errorText(act.error)}</Alert> : null}
      </div>
    </Modal>
  );
}

type Dialog =
  | { kind: "create" }
  | { kind: "edit"; account: ServiceAccount }
  | { kind: "issue"; account: ServiceAccount }
  | { kind: "delete"; account: ServiceAccount }
  | { kind: "revoke"; account: ServiceAccount; token: ServiceToken };

export default function Page() {
  const list = useApi(listServiceAccounts, [], { pollMs: POLL_MS });
  const [dialog, setDialog] = useState<Dialog>();
  const [notice, setNotice] = useState<string>();
  const remove = useAction();
  const revoke = useAction();

  const close = () => {
    setDialog(undefined);
    remove.clear();
    revoke.clear();
  };
  const done = (msg: string) => {
    setDialog(undefined);
    setNotice(msg);
    list.reload();
  };

  const header = (
    <div className={shared.titleRow}>
      <h1>Service accounts</h1>
      <Button
        variant="primary"
        onClick={() => {
          setNotice(undefined);
          setDialog({ kind: "create" });
        }}
      >
        Create service account
      </Button>
    </div>
  );

  if (list.loading)
    return (
      <div className="page">
        {header}
        <p role="status">Loading service accounts…</p>
      </div>
    );
  if (list.error || !list.data)
    return (
      <div className="page">
        {header}
        <Alert tone="error">{list.error?.message ?? "Could not load service accounts."}</Alert>
        <div>
          <Button onClick={list.reload}>Retry</Button>
        </div>
      </div>
    );

  const accounts = list.data.service_accounts;
  const open = (d: Dialog) => {
    setNotice(undefined);
    setDialog(d);
  };

  return (
    <div className="page">
      {header}
      {notice ? <Alert tone="success">{notice}</Alert> : null}
      <p className={shared.note}>
        A service account is a program, such as a lab inventory system, that reports to Hourglass or reads its state. Its
        tokens work only on the integration routes, never in this dashboard.
      </p>

      {accounts.length === 0 ? <p className={shared.note}>No service accounts yet.</p> : null}
      <ul style={{ listStyle: "none", margin: 0, padding: 0 }} className={shared.region}>
        {accounts.map((a) => (
          <li key={a.id} className={shared.card} aria-label={`Service account ${a.name}`}>
            <div className={shared.row}>
              <div className={shared.wide}>
                <div className={shared.name}>
                  {a.name} {a.enabled ? <Badge tone="green">Enabled</Badge> : <Badge tone="grey">Disabled</Badge>}
                </div>
                {a.description ? <div className={shared.sub}>{a.description}</div> : null}
              </div>
              <div className={shared.chips} aria-label={`Scopes of ${a.name}`}>
                {a.scopes.length === 0 ? <span className={shared.sub}>No scopes</span> : a.scopes.map((s) => <Chip key={s}>{s}</Chip>)}
              </div>
              <Button onClick={() => open({ kind: "issue", account: a })} aria-label={`Issue token for ${a.name}`}>
                Issue token
              </Button>
              <Button onClick={() => open({ kind: "edit", account: a })} aria-label={`Edit ${a.name}`}>
                Edit
              </Button>
              <Button variant="destructive" onClick={() => open({ kind: "delete", account: a })} aria-label={`Delete ${a.name}`}>
                Delete
              </Button>
            </div>
            <Table<ServiceToken>
              label={`Tokens of ${a.name}`}
              rows={a.tokens}
              rowKey={(t) => t.id}
              emptyMessage="No tokens."
              columns={[
                { key: "id", header: "Token id", render: (t) => t.id },
                { key: "created", header: "Created", nowrap: true, render: (t) => <Time iso={t.created_at} /> },
                { key: "expires", header: "Expires", nowrap: true, render: (t) => (t.expires_at ? <Time iso={t.expires_at} /> : "Never") },
                { key: "used", header: "Last used", nowrap: true, render: (t) => (t.last_used_at ? <Time iso={t.last_used_at} /> : "Never") },
                {
                  key: "act",
                  header: "",
                  render: (t) => (
                    <Button onClick={() => open({ kind: "revoke", account: a, token: t })} aria-label={`Revoke token ${t.id} of ${a.name}`}>
                      Revoke
                    </Button>
                  ),
                },
              ]}
            />
          </li>
        ))}
      </ul>

      {dialog?.kind === "create" ? <AccountDialog onClose={close} onDone={done} /> : null}
      {dialog?.kind === "edit" ? <AccountDialog key={dialog.account.id} account={dialog.account} onClose={close} onDone={done} /> : null}
      {dialog?.kind === "issue" ? (
        <IssueDialog key={dialog.account.id} account={dialog.account} onClose={close} onDone={() => list.reload()} />
      ) : null}
      <ConfirmDialog
        open={dialog?.kind === "delete"}
        title={dialog?.kind === "delete" ? `Delete ${dialog.account.name}?` : "Delete"}
        confirmLabel="Delete account"
        destructive
        busy={remove.busy}
        error={remove.error ? errorText(remove.error) : undefined}
        onConfirm={async () => {
          if (dialog?.kind !== "delete") return;
          const a = dialog.account;
          if (await remove.run(() => deleteServiceAccount(a.id))) done(`Deleted ${a.name}.`);
        }}
        onCancel={close}
      >
        Its tokens stop working at the next request. Holds and audit entries keep the account's name.
      </ConfirmDialog>
      <ConfirmDialog
        open={dialog?.kind === "revoke"}
        title={dialog?.kind === "revoke" ? `Revoke token ${dialog.token.id}?` : "Revoke"}
        confirmLabel="Revoke token"
        destructive
        busy={revoke.busy}
        error={revoke.error ? errorText(revoke.error) : undefined}
        onConfirm={async () => {
          if (dialog?.kind !== "revoke") return;
          const { account: a, token: t } = dialog;
          if (await revoke.run(() => revokeServiceToken(a.id, t.id))) done(`Revoked token ${t.id} of ${a.name}.`);
        }}
        onCancel={close}
      >
        {dialog?.kind === "revoke"
          ? `A service using token ${dialog.token.id} of ${dialog.account.name} is refused from its next request.`
          : null}
      </ConfirmDialog>
    </div>
  );
}
