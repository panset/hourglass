// SPDX-License-Identifier: MIT
import { NavLink } from "react-router-dom";
import { getMe, getPendingUsers, getPressure } from "../api/endpoints";
import { useApi } from "../hooks/useApi";
import { POLL_INTERVAL_MS } from "../hooks/usePolling";
import { formatMultiplier, formatPercent, formatTokens } from "../lib/format";
import { pageRoutes } from "../routes";
import { isAdmin, useSession, useSessionState } from "./session";
import styles from "./TopBar.module.css";

/** Product name, navigation, user, balance and the lab pressure pill (docs/ui-notes.md "Shared"). */
export function TopBar() {
  const session = useSession();
  const { signOut } = useSessionState();
  const admin = isAdmin(session);

  // Live numbers refresh every 60 s (UI-6). The local admin has no balance, so /me is skipped for it.
  const me = useApi(getMe, [], { pollMs: POLL_INTERVAL_MS, enabled: !session.is_local_admin });
  const pressure = useApi(getPressure, [], { pollMs: POLL_INTERVAL_MS });
  const pending = useApi(getPendingUsers, [], { pollMs: POLL_INTERVAL_MS, enabled: admin });

  const balance = me.data?.balance_tokens;
  const pendingCount = pending.data?.pending_count ?? 0;

  const link = (path: string, title: string) => {
    const label = path === "/admin/users" && pendingCount > 0 ? `${title} · ${pendingCount} pending` : title;
    return (
      <NavLink key={path} to={path} className={styles.link}>
        {label}
      </NavLink>
    );
  };
  const general = pageRoutes.filter((r) => !r.admin);
  const adminRoutes = pageRoutes.filter((r) => r.admin);

  return (
    <header className={styles.bar}>
      <span className={styles.brand}>Hourglass</span>
      <nav className={styles.nav} aria-label="Main">
        {general.map((r) => link(r.path, r.title))}
        {admin ? (
          <div className={styles.admin}>
            <span className={styles.adminLabel}>Admin</span>
            {adminRoutes.map((r) => link(r.path, r.title))}
          </div>
        ) : null}
      </nav>
      <div className={styles.right}>
        {pressure.data ? (
          <span className={styles.pill}>
            Lab {formatPercent(pressure.data.smoothed)} {"·"} {formatMultiplier(pressure.data.multiplier)}
          </span>
        ) : null}
        {!session.is_local_admin && typeof balance === "number" ? (
          <span className={styles.balance}>{formatTokens(balance)} tokens</span>
        ) : null}
        <span>{session.name}</span>
        <button type="button" className={styles.signOut} onClick={() => void signOut()}>
          Sign out
        </button>
      </div>
    </header>
  );
}
