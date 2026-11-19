// SPDX-License-Identifier: MIT
import { useState, type FormEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { ApiError } from "../../api/client";
import { getAuthMethods, localSignIn } from "../../api/endpoints";
import { Alert, Button, Field, Input, LinkButton } from "../../components";
import { useApi } from "../../hooks/useApi";
import { useSessionState } from "../../shell/session";
import styles from "./Login.module.css";

const OIDC_ERRORS: Record<string, string> = {
  unauthenticated: "Single sign-on did not complete. Try again.",
  forbidden: "Single sign-on was refused.",
  rate_limited: "Too many attempts. Wait a few minutes and try again.",
  internal: "Something went wrong on the server. Try again.",
};

/** AUTH-6: "Sign in with SSO" (only when OIDC is configured) and the local admin form. */
export default function Login() {
  const methods = useApi(getAuthMethods);
  const { setSession } = useSessionState();
  const [params] = useSearchParams();
  const urlError = params.get("auth_error");

  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    try {
      setSession(await localSignIn({ username, password }));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Sign-in failed.");
      setBusy(false);
    }
  }

  const showOidc = methods.data?.oidc === true;
  const showLocal = methods.data ? methods.data.local : methods.error !== undefined;

  return (
    <div className="public-column">
      <h1 className={styles.title}>Hourglass</h1>
      <p className="muted">Sign in to see your setups and tokens.</p>

      {urlError ? (
        <Alert tone="error">{OIDC_ERRORS[urlError] ?? `Sign-in failed (${urlError}).`}</Alert>
      ) : null}
      {methods.error ? <Alert tone="error">Could not load the sign-in options: {methods.error.message}</Alert> : null}
      {methods.loading ? <p className="muted">Loading sign-in options…</p> : null}

      {showOidc ? (
        <LinkButton variant="primary" block href="/auth/oidc/start">
          Sign in with SSO
        </LinkButton>
      ) : null}

      {showOidc && showLocal ? <div className={styles.divider}>Local administrator</div> : null}

      {showLocal ? (
        <form className={styles.form} onSubmit={submit}>
          <Field label="Username">
            {(p) => (
              <Input {...p} name="username" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} />
            )}
          </Field>
          <Field label="Password">
            {(p) => (
              <Input
                {...p}
                name="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            )}
          </Field>
          {error ? <Alert tone="error">{error}</Alert> : null}
          <Button type="submit" disabled={busy}>
            Sign in as admin
          </Button>
          <p className={styles.note}>
            The local form is only for the built-in admin account. Everyone else uses SSO. First local sign-in forces a
            password change.
          </p>
        </form>
      ) : null}

      {methods.data && !showOidc && !showLocal ? <Alert>No sign-in method is configured.</Alert> : null}
    </div>
  );
}
