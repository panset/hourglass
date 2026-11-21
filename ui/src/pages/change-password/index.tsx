// SPDX-License-Identifier: MIT
import { useState, type FormEvent } from "react";
import { ApiError } from "../../api/client";
import { changePassword } from "../../api/endpoints";
import { Alert, BrandMark, Button, Field, Input } from "../../components";
import { useSessionState } from "../../shell/session";

/** Minimum length of the new local admin password (AUTH-7). */
export const MIN_PASSWORD_LENGTH = 12;

type Errors = Partial<Record<"current_password" | "new_password" | "confirm" | "form", string>>;

/** Shown instead of everything else while must_change_password is true. */
export default function ChangePassword() {
  const { setSession, signOut } = useSessionState();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [errors, setErrors] = useState<Errors>({});
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    const found: Errors = {};
    if (!current) found.current_password = "Enter your current password.";
    if (next.length < MIN_PASSWORD_LENGTH) found.new_password = `Use at least ${MIN_PASSWORD_LENGTH} characters.`;
    else if (confirm !== next) found.confirm = "The two passwords do not match.";
    setErrors(found);
    if (Object.keys(found).length > 0) return;

    setBusy(true);
    try {
      setSession(await changePassword({ current_password: current, new_password: next }));
    } catch (err) {
      if (err instanceof ApiError && (err.field === "current_password" || err.field === "new_password")) {
        setErrors({ [err.field]: err.message });
      } else {
        setErrors({ form: err instanceof ApiError ? err.message : "Could not change the password." });
      }
      setBusy(false);
    }
  }

  return (
    <div className="public-column">
      <div style={{ fontSize: "var(--fs-18)", fontWeight: 600 }}>
        <BrandMark />
        Hourglass
      </div>
      <h1>Change your password</h1>
      <p className="muted">You must choose a new password before you can continue.</p>
      <form onSubmit={submit} noValidate style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <Field label="Current password" error={errors.current_password}>
          {(p) => <Input {...p} type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} />}
        </Field>
        <Field label="New password" hint={`At least ${MIN_PASSWORD_LENGTH} characters.`} error={errors.new_password}>
          {(p) => <Input {...p} type="password" autoComplete="new-password" value={next} onChange={(e) => setNext(e.target.value)} />}
        </Field>
        <Field label="Repeat new password" error={errors.confirm}>
          {(p) => <Input {...p} type="password" autoComplete="new-password" value={confirm} onChange={(e) => setConfirm(e.target.value)} />}
        </Field>
        {errors.form ? <Alert tone="error">{errors.form}</Alert> : null}
        <Button type="submit" variant="primary" disabled={busy}>
          Change password
        </Button>
        <Button onClick={() => void signOut()}>Sign out</Button>
      </form>
    </div>
  );
}
