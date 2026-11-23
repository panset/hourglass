// SPDX-License-Identifier: MIT
// Admin > Users, "Pre-approved users" (UI-11, design D5). Every field shown is an API field (UI-1).
import { useState } from "react";
import { Alert, Button, ConfirmDialog, Field, Select, Table, Time } from "../../components";
import { useApi } from "../../hooks/useApi";
import { errorText, useAction } from "../admin-shared/useAction";
import shared from "../admin-shared/admin.module.css";
import { addPreapprovals, deletePreapproval, listPreapprovals, type Preapproval, type Role } from "./api";
import styles from "./preapprovals.module.css";

const LABEL: Record<Role, string> = { developer: "Developer", admin: "Admin" };

/** Splits pasted text on commas, whitespace and new lines; empty pieces are dropped. */
export function splitEmails(text: string): string[] {
  return text.split(/[\s,]+/).filter((s) => s !== "");
}

const rowKey = (p: Preapproval) => (p.id === null ? `config:${p.email}` : `database:${p.id}`);

export function Preapprovals() {
  const list = useApi(listPreapprovals);
  const [text, setText] = useState("");
  const [role, setRole] = useState<Role>("developer");
  const [removing, setRemoving] = useState<Preapproval>();
  const add = useAction();
  const remove = useAction();

  const emails = splitEmails(text);
  const submit = async () => {
    if (await add.run(() => addPreapprovals(emails, [role]))) {
      setText("");
      list.reload();
    }
  };
  const confirmRemove = async () => {
    if (removing?.id == null) return;
    const id = removing.id;
    if (await remove.run(() => deletePreapproval(id))) {
      setRemoving(undefined);
      list.reload();
    }
  };

  return (
    <section className={shared.region} aria-labelledby="preapproved-h">
      <h2 id="preapproved-h">Pre-approved users</h2>
      <p className={shared.note}>Listed people get the chosen role the first time they sign in with SSO.</p>

      {list.loading ? (
        <p role="status">Loading pre-approved users…</p>
      ) : list.error || !list.data ? (
        <>
          <Alert tone="error">{list.error?.message ?? "Could not load pre-approved users."}</Alert>
          <div>
            <Button onClick={list.reload}>Retry</Button>
          </div>
        </>
      ) : (
        <Table<Preapproval>
          label="Pre-approved users"
          rows={list.data.preapprovals}
          rowKey={rowKey}
          emptyMessage="No pre-approved users."
          columns={[
            { key: "email", header: "Email", render: (p) => p.email },
            { key: "roles", header: "Roles", render: (p) => p.roles.map((r) => LABEL[r]).join(", ") },
            { key: "source", header: "Source", render: (p) => (p.source === "database" ? "Added here" : "Config file") },
            { key: "by", header: "Added by", render: (p) => p.created_by?.name ?? "—" },
            {
              key: "used",
              header: "Signed in",
              render: (p) =>
                p.used_at === null ? (
                  "Not yet"
                ) : (
                  <>
                    <Time iso={p.used_at} />
                    {p.used_by ? <div className={shared.sub}>{p.used_by.name}</div> : null}
                  </>
                ),
            },
            {
              key: "act",
              header: "",
              render: (p) =>
                p.id === null ? null : (
                  <Button
                    onClick={() => {
                      remove.clear();
                      setRemoving(p);
                    }}
                    aria-label={`Remove ${p.email}`}
                  >
                    Remove
                  </Button>
                ),
            },
          ]}
        />
      )}

      <form
        className={shared.region}
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <Field
          label="Emails to pre-approve"
          hint="Separate with commas, spaces or new lines."
          error={add.error ? add.error.message : undefined}
        >
          {(p) => (
            <textarea {...p} className={styles.text} rows={4} value={text} onChange={(e) => setText(e.target.value)} />
          )}
        </Field>
        <div className={shared.rowBottom}>
          <div className={styles.roleField}>
            <Field label="Role">
              {(p) => (
                <Select {...p} value={role} onChange={(e) => setRole(e.target.value as Role)}>
                  <option value="developer">Developer</option>
                  <option value="admin">Admin</option>
                </Select>
              )}
            </Field>
          </div>
          <Button type="submit" variant="primary" disabled={add.busy || emails.length === 0}>
            Add
          </Button>
        </div>
      </form>

      <ConfirmDialog
        open={removing !== undefined}
        title={`Remove ${removing?.email ?? ""}?`}
        confirmLabel="Remove"
        destructive
        busy={remove.busy}
        error={remove.error ? errorText(remove.error) : undefined}
        onConfirm={confirmRemove}
        onCancel={() => setRemoving(undefined)}
      >
        They will no longer be approved at their first SSO sign-in. Anyone who already signed in keeps their access.
      </ConfirmDialog>
    </section>
  );
}
