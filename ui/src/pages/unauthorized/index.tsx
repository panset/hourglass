// SPDX-License-Identifier: MIT
import { Button } from "../../components";
import { useSession, useSessionState } from "../../shell/session";

/** AUTH-5: the fixed message, the signed-in email and a sign-out. No navigation, no data. */
export default function Unauthorized() {
  const session = useSession();
  const { signOut } = useSessionState();
  return (
    <div className="public-column">
      <div style={{ fontSize: "var(--fs-18)", fontWeight: 600 }}>Hourglass</div>
      <h1>You are not authorized to use Hourglass. Reach out to your admin.</h1>
      <p className="muted">Signed in as {session.email}</p>
      <Button block onClick={() => void signOut()}>
        Sign out
      </Button>
    </div>
  );
}
