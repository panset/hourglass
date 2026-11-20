// SPDX-License-Identifier: MIT
export interface paths {
    "/auth/oidc/start": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Begin the OIDC authorization code flow with PKCE
         * @description Redirects (302) to the issuer's authorization endpoint. The UI links
         *     here from "Sign in with SSO"; it never sees a token (AUTH-1).
         *
         */
        get: operations["oidcStart"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/callback": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Finish the OIDC flow
         * @description Exchanges the code, creates or refreshes the user (AUTH-2), sets the
         *     session cookie and redirects (302) back to the UI at `/`, whatever the
         *     user's roles; the UI decides what to show from `GET /api/v1/session`
         *     (UI-7). On failure it redirects to `/?auth_error=<code>` using an error
         *     code from `ErrorCode`; every failure of the flow is such a redirect,
         *     never a JSON error.
         *
         */
        get: operations["oidcCallback"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/local": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Sign in as the local admin
         * @description Used only by the local `admin` account (AUTH-6). Rate limited to 5
         *     failures per 15 minutes (AUTH-7); 429 `rate_limited` carries
         *     `Retry-After`. Wrong credentials return 401 `unauthenticated`. The
         *     response session may have `must_change_password: true`.
         *
         */
        post: operations["localSignIn"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/local/password": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Change the local admin password
         * @description Minimum 12 characters (AUTH-7). Allowed while `must_change_password`
         *     is true. A wrong `current_password` or a short new one returns 400
         *     `validation` with `field`; wrong passwords count toward the same
         *     5-failures-per-15-minutes limit as sign-in (429 `rate_limited`). Returns the refreshed session.
         *
         */
        post: operations["changeLocalPassword"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/auth/logout": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** End the session */
        post: operations["logout"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/healthz": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Liveness
         * @description Reports the database and the age of the last successful poll. 503 with the same body when the database is unreachable.
         */
        get: operations["getHealth"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/auth/methods": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Sign-in methods the sign-in page should show
         * @description Nothing else is returned.
         */
        get: operations["getAuthMethods"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/session": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Identity, roles, status and CSRF token
         * @description The only data route open to a user with no role (AUTH-5). For such a
         *     user the response holds exactly the `Session` fields and nothing else.
         *     The not-authorized page shows `email` from here.
         *
         */
        get: operations["getSession"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/me": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Balance, burn rate, projection, hold count and limit
         * @description Feeds the top bar balance and the My usage stat cards. The local admin has `balance_tokens: null` and `hold_limit: 0`.
         */
        get: operations["getMe"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/me/holds": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Current and past holds
         * @description Newest first. Without `state` both are returned.
         */
        get: operations["listMyHolds"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/me/ledger": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Own ledger entries for a period
         * @description Newest first. `period` defaults to the current period; a malformed value is 400 `validation`. `available_periods` feeds the Month select.
         */
        get: operations["listMyLedger"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/me/usage": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Tokens debited per day
         * @description One entry per calendar day of the period up to today (the whole month for a past period), including zero days.
         */
        get: operations["getMyUsage"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/pressure": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Current lab pressure, multiplier, bands and 7-day history
         * @description Feeds the top bar pill and the My usage pressure card.
         */
        get: operations["getPressure"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/setups": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Setups with labels, weight, state, holder and holder's balance
         * @description Visible to every user with a role. Also returns `display_label_keys`
         *     (SET-6). Admins additionally get disabled setups and the `admin`
         *     sub-object; for other users `admin` is null and disabled setups are
         *     omitted. `selector` uses the RULE-2 syntax; a bad one is 400
         *     `invalid_selector` with the parse error. The state and label filters
         *     of the Setups page are applied client-side on this list.
         *
         */
        get: operations["listSetups"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/setups/{id}/acquire": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Acquire a free setup
         * @description HOLD-1. Errors (all 409 unless noted): `insufficient_tokens`,
         *     `hold_limit`, `setup_held`, `setup_disabled`, `hook_veto` (message is
         *     the hook's). The local admin cannot acquire: 403 `forbidden`. Unknown
         *     id is 404 `not_found`.
         *
         */
        post: operations["acquireSetup"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/holds/{id}/release": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Release a hold, or force-release as admin
         * @description HOLD-2. The owner or an admin may call it. With an empty body or
         *     `{"force": false}` it is a normal release, refused with 409
         *     `still_deployed` while the provider still reports the setup and
         *     `release_requires_teardown` is on ("This setup is still deployed. Tear
         *     it down, then release."). `{"force": true, "reason": "..."}` is the
         *     admin force-release: it skips the teardown check, needs a non-empty
         *     reason (400 `validation`, `field` = `reason`) and is audited. `force`
         *     by a non-admin, or a normal release of another user's hold, is 403
         *     `forbidden`. An already released hold is 409 `conflict`.
         *
         */
        post: operations["releaseHold"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/users": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List users, with pending count and page totals
         * @description ADM-1, ADM-5. `status=pending` returns only pending requests, oldest
         *     first. Without a filter every user is returned, overdrawn users first
         *     and then by name. `pending_count` (for the navigation "Users · N
         *     pending") and `summary` (the four stat cards) are always present and
         *     ignore the filter. The users-page search box filters this list
         *     client-side.
         *
         */
        get: operations["listAdminUsers"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/users/{id}/roles": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Replace a user's role set
         * @description A non-empty set approves a pending (or rejected) user and credits the
         *     allowance once per period. An empty set returns the user to
         *     unassigned. Errors (409): `has_active_hold` (emptying the roles of a
         *     user who holds a setup), `self_demote` (an admin removing their own
         *     `admin`), `last_admin`, `local_admin_protected` (the local admin).
         *
         */
        put: operations["setUserRoles"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/users/{id}/reject": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Reject a pending access request
         * @description The user leaves the pending list and keeps seeing the not-authorized page. Only a `pending` user can be rejected (otherwise 409 `conflict`); the local admin is 409 `local_admin_protected`.
         */
        post: operations["rejectUser"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/users/{id}/grants": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Grant tokens to a user
         * @description LED-4. Negative amounts are allowed for corrections; zero is 400
         *     `validation`. A reason is required. The target must have a role (else
         *     409 `conflict`) and a balance (the local admin is 409
         *     `local_admin_protected`).
         *
         */
        post: operations["grantTokens"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/users/{id}/reset": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Reset a user's balance to the default allowance
         * @description LED-5. A `reset` entry brings the balance to exactly the default allowance and clears carried debt. A reason is required. Same target rules as grants.
         */
        post: operations["resetBalance"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/setups": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Add a setup
         * @description SET-1 to SET-3. A duplicate name is 409 `conflict`. Invalid labels or `details` (not JSON, over 256 KB) are 400 `validation` with `field`.
         */
        post: operations["createSetup"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/setups/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Replace a setup's editable fields
         * @description Full replacement of `SetupInput`. Setting `enabled: false` on a held setup is 409 `has_active_hold` (SET-5). A duplicate name is 409 `conflict`.
         */
        put: operations["updateSetup"];
        post?: never;
        /**
         * Delete a setup
         * @description A setup with an active hold is 409 `has_active_hold` (SET-5).
         */
        delete: operations["deleteSetup"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/rules": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Ordered rate rules with match counts
         * @description RULE-3. `default_weight` is read-only here; it is edited through settings. `unmatched_setups` lists enabled setups that fall through to the default weight.
         */
        get: operations["getRules"];
        /**
         * Replace the ordered rule list
         * @description The array order is the position. An invalid selector rejects the whole
         *     request with 400 `invalid_selector`, `field` = `rules[N].selector` and
         *     the parse error as `message` (RULE-3). Applies from the next poll,
         *     never retroactively, and is audited (RULE-4). Returns the new list with
         *     match counts.
         *
         */
        put: operations["replaceRules"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/settings": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Metering settings and read-only runtime names
         * @description ADM-2. `runtime` holds names only, never secrets.
         */
        get: operations["getSettings"];
        /**
         * Replace the metering settings
         * @description The body is the complete `Settings` object; unknown fields are
         *     rejected. The first invalid value is 400 `validation` with `field`
         *     (negative weight, bad label key, bad bands, poll interval under 1 minute, and so
         *     on). Applies from the next poll, never retroactively; changed keys are
         *     audited. Returns the same body as GET.
         *
         */
        put: operations["updateSettings"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/collector": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Collector health, outages, orphans and event delivery
         * @description ADM-4, HOLD-4, EXT-6, EXT-7. Orphans, outage history, recent failed event deliveries and the event queue length are carried here.
         */
        get: operations["getCollector"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/audit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Audit log, newest first
         * @description ADM-3. Filter by actor user id and by exact action; page with `limit` (default 50, max 200) and `offset`.
         */
        get: operations["listAudit"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        /**
         * @description Machine-readable error code. HTTP status per code is in `x-http-status`.
         *
         * @enum {string}
         */
        ErrorCode: "unauthenticated" | "no_role" | "forbidden" | "csrf" | "password_change_required" | "not_found" | "validation" | "invalid_selector" | "insufficient_tokens" | "hold_limit" | "setup_held" | "setup_disabled" | "still_deployed" | "hook_veto" | "has_active_hold" | "last_admin" | "self_demote" | "local_admin_protected" | "conflict" | "rate_limited" | "internal";
        Error: {
            error: {
                code: components["schemas"]["ErrorCode"];
                /** @description Human-readable; safe to show. For `hook_veto` it is the hook's message. */
                message: string;
                /** @description For `validation` and `invalid_selector`, the request field at fault, e.g. `reason` or `rules[1].selector`. */
                field?: string;
            };
        };
        /**
         * @description Tokens with up to three decimals.
         * @example 142.2
         */
        Tokens: number;
        /**
         * @description Label key to value (SET-3).
         * @example {
         *       "tier": "a",
         *       "site": "x"
         *     }
         */
        Labels: {
            [key: string]: string;
        };
        /** @enum {string} */
        Role: "developer" | "admin";
        /** @enum {string} */
        UserStatus: "pending" | "active" | "rejected";
        /** @enum {string} */
        AuthSource: "oidc" | "local";
        Health: {
            /** @enum {string} */
            status: "ok" | "degraded";
            /** @enum {string} */
            database: "ok" | "error";
            /** @description Seconds since the last successful poll; null before the first one. */
            last_poll_age_seconds: number | null;
        };
        AuthMethods: {
            /** @description Show "Sign in with SSO". */
            oidc: boolean;
            /** @description Show the local admin form. */
            local: boolean;
        };
        LocalSignInRequest: {
            /** @example admin */
            username: string;
            /** Format: password */
            password: string;
        };
        ChangePasswordRequest: {
            /** Format: password */
            current_password: string;
            /** Format: password */
            new_password: string;
        };
        /** @description Identity of the signed-in user. For a user with no role this is exactly
         *     these fields and nothing else (AUTH-5): `roles` is empty.
         *      */
        Session: {
            /** Format: int64 */
            user_id: number;
            username: string;
            email: string;
            name: string;
            auth_source: components["schemas"]["AuthSource"];
            status: components["schemas"]["UserStatus"];
            roles: components["schemas"]["Role"][];
            csrf_token: string;
            must_change_password: boolean;
            is_local_admin: boolean;
        };
        /** @description One metering hold's current burn. */
        Burning: {
            /** Format: int64 */
            hold_id: number;
            /** Format: int64 */
            setup_id: number;
            /** @example lab-01 */
            setup_name: string;
            /** @example 1 */
            weight: number;
            /** @example 2 */
            multiplier: number;
            tokens_per_hour: components["schemas"]["Tokens"];
        };
        Me: {
            /** @example 2026-11 */
            period: string;
            /**
             * Format: date-time
             * @description Start of the next period; unused tokens expire here.
             */
            period_ends_at: string;
            /** @description Null for the local admin (no balance). */
            balance_tokens: (number & components["schemas"]["Tokens"]) | null;
            allowance_tokens: components["schemas"]["Tokens"];
            /** @description Sum of `grant` entries this period. */
            granted_tokens: components["schemas"]["Tokens"];
            /** @description Debt carried in at the period start (negative or 0). */
            carryover_tokens: components["schemas"]["Tokens"];
            /** @description Month-to-date debits as a positive number. */
            debited_tokens: components["schemas"]["Tokens"];
            /** @description Current burn per hour, summed over metering holds. */
            burn_per_hour_tokens: components["schemas"]["Tokens"];
            burning: components["schemas"]["Burning"][];
            /**
             * @description `date`: `projected_runout_at` is set. `not_this_month`: the projection passes month end. `not_burning`: nothing is metering.
             * @enum {string}
             */
            projection: "date" | "not_this_month" | "not_burning";
            /**
             * Format: date-time
             * @description Balance divided by the average hourly burn over the last 24 hours; null unless `projection` is `date`.
             */
            projected_runout_at: string | null;
            hold_count: number;
            /** @description `max_holds_per_user`; 0 for the local admin, who cannot hold. */
            hold_limit: number;
            /** @description False means shadow mode (nobody is blocked at zero). */
            enforcement: boolean;
            /** @description Balance is zero or below. */
            overdrawn: boolean;
            /** @description Whether this user may acquire a setup now, apart from the setup itself (taken, disabled, hook veto). Computed by the same rule as acquire (UI-1); the UI shows it and decides nothing. */
            can_acquire: boolean;
            /**
             * @description Why `can_acquire` is false; null when it is true. `local_admin` is the operator account, which cannot hold setups.
             * @enum {string|null}
             */
            cannot_acquire_reason: "insufficient_tokens" | "hold_limit" | "local_admin" | null;
        };
        /** @description Metering status of a hold. */
        MeteringState: {
            active: boolean;
            /** Format: date-time */
            started_at: string | null;
            /**
             * Format: date-time
             * @description When the policy expects to start metering (for example 2 h after acquire); null when unknown or already started.
             */
            expected_start_at: string | null;
            /** @description The metering policy's reason text, e.g. `2 h since acquire`. */
            reason: string | null;
        };
        CostPerHour: {
            /** @description Tokens per hour at the current multiplier. */
            now: components["schemas"]["Tokens"];
            /** @description Tokens per hour at the highest band's multiplier. */
            at_peak: components["schemas"]["Tokens"];
        };
        Hold: {
            /** Format: int64 */
            id: number;
            /** Format: int64 */
            setup_id: number;
            setup_name: string;
            setup_labels: components["schemas"]["Labels"];
            /** Format: int64 */
            user_id: number;
            active: boolean;
            weight: number;
            /** Format: date-time */
            acquired_at: string;
            /** Format: date-time */
            metering_started_at: string | null;
            /** Format: date-time */
            deployed_at: string | null;
            /** Format: date-time */
            released_at: string | null;
            /** @description The provider currently reports the setup. */
            deployed: boolean;
            metering: components["schemas"]["MeteringState"];
            /** @description Null once released. */
            cost_per_hour_tokens: components["schemas"]["CostPerHour"] | null;
            /** @description Total debited for this hold so far (positive). */
            charged_tokens: components["schemas"]["Tokens"];
            released_by: {
                /** Format: int64 */
                id: number;
                name: string;
            } | null;
            release_reason: string | null;
            /** @description Released by an admin other than the holder. */
            forced: boolean;
        };
        /** @enum {string} */
        LedgerEntryType: "allowance" | "grant" | "debit" | "expiry" | "carryover" | "reset";
        LedgerEntry: {
            /** Format: int64 */
            id: number;
            /** Format: date-time */
            created_at: string;
            /** @example 2026-11 */
            period: string;
            type: components["schemas"]["LedgerEntryType"];
            /** @description Signed; debits are negative. */
            amount_tokens: components["schemas"]["Tokens"];
            balance_after_tokens: components["schemas"]["Tokens"];
            /** @description Multiplier of the poll, for debits (shown as "Debit · 2×"). */
            multiplier: number | null;
            setup_name: string | null;
            /** Format: int64 */
            hold_id: number | null;
            /** @description Granting or resetting admin. */
            actor_name: string | null;
            reason: string | null;
        };
        LedgerResponse: {
            period: string;
            /** @description Periods with data, newest first (feeds the Month select). */
            available_periods: string[];
            entries: components["schemas"]["LedgerEntry"][];
        };
        UsageDay: {
            /** Format: date */
            date: string;
            /** @description Positive amount debited that day. */
            debited_tokens: components["schemas"]["Tokens"];
        };
        UsageResponse: {
            period: string;
            available_periods: string[];
            total_debited_tokens: components["schemas"]["Tokens"];
            days: components["schemas"]["UsageDay"][];
        };
        BandStep: {
            /**
             * @description Pressure fraction at which this band starts
             * @example 0.5
             */
            from: number;
            /** @example 1 */
            multiplier: number;
        };
        /** @description Below the first step the multiplier is `base`; from each step's `from` upward it is that step's multiplier. Default: base 0, steps 0.5 to 1, 0.75 to 2, 0.9 to 3. */
        Bands: {
            /** @example 0 */
            base: number;
            steps: components["schemas"]["BandStep"][];
        };
        PressurePoint: {
            /** Format: date-time */
            at: string;
            raw: number;
            smoothed: number;
            multiplier: number;
        };
        Pressure: {
            /** @description Latest source value. */
            raw: number;
            /** @description 15-minute average used for the multiplier. */
            smoothed: number;
            multiplier: number;
            bands: components["schemas"]["Bands"];
            /**
             * @description Pressure source name
             * @example inventory
             */
            source: string;
            /** Format: date-time */
            updated_at: string | null;
            /** @description Enabled setups. */
            setups_enabled: number;
            /** @description Enabled setups the provider reports ("9 of 11 setups deployed"). */
            setups_deployed: number;
            /** @description Up to 7 days of samples, oldest first. */
            history: components["schemas"]["PressurePoint"][];
        };
        Holder: {
            /** Format: int64 */
            user_id: number;
            /**
             * Format: int64
             * @description Use with POST /api/v1/holds/{id}/release.
             */
            hold_id: number;
            name: string;
            balance_tokens: components["schemas"]["Tokens"];
            overdrawn: boolean;
            is_you: boolean;
        };
        /** @description Admin-only fields. */
        SetupAdminFields: {
            enabled: boolean;
            /**
             * Format: date-time
             * @description Last time the provider reported an allocation with the setup's exact name; null if never.
             */
            last_seen_at: string | null;
            /** @description Selector of the rate rule that set the weight; null for the default weight. */
            matched_rule_selector: string | null;
        };
        Setup: {
            /** Format: int64 */
            id: number;
            /** @example lab-01 */
            name: string;
            labels: components["schemas"]["Labels"];
            /**
             * @description Resolved weight from the rate rules
             * @example 1
             */
            weight: number;
            /** @enum {string} */
            state: "free" | "held" | "disabled";
            holder: components["schemas"]["Holder"] | null;
            /** Format: date-time */
            held_since: string | null;
            /** @description The provider currently reports the setup ("Deployed" vs "Acquired · deploying"). */
            deployed: boolean;
            /** @description Null unless held. */
            metering: components["schemas"]["MeteringState"] | null;
            cost_per_hour_tokens: components["schemas"]["CostPerHour"];
            /**
             * @description Opaque JSON returned unmodified (SET-2).
             * @example {
             *       "note": "opaque"
             *     }
             */
            details: unknown;
            /** @description Null for users who are not admins. */
            admin: components["schemas"]["SetupAdminFields"] | null;
        };
        SetupList: {
            setups: components["schemas"]["Setup"][];
            /** @example [
             *       "tier"
             *     ] */
            display_label_keys: string[];
            /** @description Counts over all enabled setups, ignoring `selector` ("9 of 11 setups deployed · 2 free"). */
            summary: {
                enabled: number;
                deployed: number;
                free: number;
            };
            /** @description Always null; the metering policy interface has no describe method. The per-hold reason is `metering.reason`. */
            metering_description: string | null;
        };
        SetupInput: {
            /** @example lab-01 */
            name: string;
            labels: components["schemas"]["Labels"];
            /**
             * @description Any well-formed JSON, at most 256 KB.
             * @example {
             *       "note": "opaque"
             *     }
             */
            details: unknown;
            enabled: boolean;
        };
        ReleaseRequest: {
            /**
             * @description Admin force-release.
             * @default false
             */
            force: boolean;
            /** @description Required when `force` is true. */
            reason?: string;
        };
        ActiveHoldRef: {
            /** Format: int64 */
            hold_id: number;
            /** Format: int64 */
            setup_id: number;
            setup_name: string;
        };
        AdminUser: {
            /** Format: int64 */
            id: number;
            username: string;
            email: string;
            name: string;
            auth_source: components["schemas"]["AuthSource"];
            status: components["schemas"]["UserStatus"];
            roles: components["schemas"]["Role"][];
            /** @description Role cannot be changed; no balance; cannot hold. */
            is_local_admin: boolean;
            /** @description Null for the local admin, for users with no role (they have no allowance) and for users who never had a role. */
            balance_tokens: (number & components["schemas"]["Tokens"]) | null;
            overdrawn: boolean;
            /** @description Null for users with no role; 0 for the local admin. */
            month_to_date_burn_tokens: (number & components["schemas"]["Tokens"]) | null;
            burn_per_hour_tokens: components["schemas"]["Tokens"];
            active_holds: components["schemas"]["ActiveHoldRef"][];
            /** Format: date-time */
            first_sign_in_at: string;
            /** Format: date-time */
            last_login_at: string | null;
        };
        AdminUserList: {
            users: components["schemas"]["AdminUser"][];
            /** @description Pending requests, for the navigation */
            pending_count: number;
            summary: {
                user_count: number;
                /** @description Admins including the local admin. */
                admin_count: number;
                overdrawn_count: number;
                debited_this_month_tokens: components["schemas"]["Tokens"];
                granted_this_month_tokens: components["schemas"]["Tokens"];
                grant_count_this_month: number;
            };
            period: string;
            /**
             * Format: date-time
             * @description When unused tokens expire and debt carries over.
             */
            period_ends_at: string;
            /** @description What "Reset to N" sets the balance to. */
            default_allowance_tokens: components["schemas"]["Tokens"];
        };
        SetRolesRequest: {
            roles: components["schemas"]["Role"][];
        };
        GrantRequest: {
            /**
             * @description Non-zero; negative corrects mistakes.
             * @example 100
             */
            amount_tokens: components["schemas"]["Tokens"];
            /** @example soak test week */
            reason: string;
        };
        ResetRequest: {
            reason: string;
        };
        BalanceChangeResult: {
            entry: components["schemas"]["LedgerEntry"];
            user: components["schemas"]["AdminUser"];
        };
        RuleInput: {
            /** @example tier=a */
            selector: string;
            /** @example 2 */
            weight: number;
        };
        RulesInput: {
            rules: components["schemas"]["RuleInput"][];
        };
        Rule: {
            /** Format: int64 */
            id: number;
            position: number;
            selector: string;
            weight: number;
            /** @description Setups that this rule currently sets the weight of (first match). */
            match_count: number;
        };
        SetupRef: {
            /** Format: int64 */
            id: number;
            name: string;
            labels: components["schemas"]["Labels"];
        };
        RulesResponse: {
            rules: components["schemas"]["Rule"][];
            /** @description Read-only here; edit through settings. */
            default_weight: number;
            /** @description Enabled setups that fall through to the default weight. */
            unmatched_setups: components["schemas"]["SetupRef"][];
        };
        /** @description Admin-editable metering settings (ADM-2); unknown fields are rejected. */
        Settings: {
            /** @default 1 */
            default_weight: number;
            /** @example [
             *       "tier"
             *     ] */
            display_label_keys: string[];
            bands: components["schemas"]["Bands"];
            /**
             * @description Whole tokens, decimals allowed.
             * @example 300
             */
            default_allowance: components["schemas"]["Tokens"];
            /** @default 2 */
            max_holds_per_user: number;
            /** @default true */
            release_requires_teardown: boolean;
            /**
             * @description Duration string, at least `1m`.
             * @example 5m
             */
            poll_interval: string;
            /**
             * @description False is shadow mode.
             * @default false
             */
            enforcement: boolean;
        };
        /** @description Read-only names from the config file; never secrets. */
        Runtime: {
            /** @description Null when none is configured. */
            provider: string | null;
            /** @example inventory */
            pressure_source: string;
            metering_policy: {
                /** @example default */
                name: string;
                /** @description Always null; the extension interfaces have no describe method. */
                description: string | null;
            };
            hooks: string[];
            oidc_issuer: string | null;
            /** @example UTC */
            timezone: string;
        };
        SettingsResponse: {
            settings: components["schemas"]["Settings"];
            runtime: components["schemas"]["Runtime"];
        };
        CollectorRun: {
            /** Format: int64 */
            id: number;
            /** Format: date-time */
            started_at: string;
            /** Format: date-time */
            finished_at: string | null;
            ok: boolean;
            error: string | null;
        };
        /** @description A gap longer than twice the poll interval that was not charged (MET-4), or a failed-provider gap. */
        Outage: {
            /** Format: date-time */
            started_at: string;
            /** Format: date-time */
            ended_at: string | null;
            duration_minutes: number;
        };
        Orphan: {
            /**
             * @description The name the provider reported, which is the setup's name (D40).
             * @example lab-09
             */
            resource_name: string;
            /** Format: date-time */
            first_seen_at: string;
            /** Format: date-time */
            last_seen_at: string;
            /** @description The setup the resource name matches (it exists but nobody holds it). */
            setup: {
                /** Format: int64 */
                id: number;
                name: string;
            } | null;
            /** @description Provider-specific labels. */
            labels: {
                [key: string]: string;
            };
        };
        FailedDelivery: {
            /** Format: int64 */
            id: number;
            /** @example hold.acquired */
            event_type: string;
            hook: string;
            error: string;
            /** Format: date-time */
            started_at: string;
            /** Format: date-time */
            finished_at: string | null;
        };
        Collector: {
            extensions: {
                provider: string | null;
                pressure_source: string;
                metering_policy: string;
                hooks: string[];
            };
            /** Format: date-time */
            last_successful_poll_at: string | null;
            last_error: {
                message: string;
                /** Format: date-time */
                at: string;
            } | null;
            /** @description Enabled setups the latest poll matched to an allocation. */
            matched_setups: number;
            orphan_count: number;
            outages_this_month: {
                count: number;
                minutes_not_charged: number;
            };
            /** @description Latest runs, newest first (up to 20). */
            recent_runs: components["schemas"]["CollectorRun"][];
            /** @description Outage history, newest first. */
            outages: components["schemas"]["Outage"][];
            orphans: components["schemas"]["Orphan"][];
            /** @description Recent failed event deliveries, newest first (up to 20). */
            failed_deliveries: components["schemas"]["FailedDelivery"][];
            /** @description Events waiting in the in-memory delivery queue. */
            queue_length: number;
        };
        AuditEntry: {
            /** Format: int64 */
            id: number;
            actor: {
                /** Format: int64 */
                id: number;
                name: string;
            };
            /** @example ledger.grant */
            action: string;
            /**
             * @description What was changed, e.g. `user:12` or `setup:4`
             * @example user:12
             */
            target: string;
            before: {
                [key: string]: unknown;
            } | null;
            after: {
                [key: string]: unknown;
            } | null;
            /** Format: date-time */
            created_at: string;
        };
        AuditList: {
            entries: components["schemas"]["AuditEntry"][];
            /** @description Entries matching the filters. */
            total: number;
            limit: number;
            offset: number;
        };
    };
    responses: {
        /** @description 400 `validation` (optional `field`) or `invalid_selector`. */
        BadRequest: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 401 `unauthenticated`. */
        Unauthenticated: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 403 `no_role`, `forbidden`, `csrf` or `password_change_required`. */
        Forbidden: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 404 `not_found`. */
        NotFound: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 409 with one of the conflict codes named in the operation description. */
        Conflict: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 429 `rate_limited`. */
        RateLimited: {
            headers: {
                /** @description Seconds until another attempt is allowed. */
                "Retry-After"?: number;
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 500 `internal`. */
        InternalError: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
    };
    parameters: {
        /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
        CsrfToken: string;
        Id: number;
        /** @description Period as `YYYY-MM`; defaults to the current period. */
        PeriodQuery: string;
    };
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    oidcStart: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Redirect to the identity provider. */
            302: {
                headers: {
                    /** @description Authorization endpoint URL. */
                    Location?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            404: components["responses"]["NotFound"];
            500: components["responses"]["InternalError"];
        };
    };
    oidcCallback: {
        parameters: {
            query?: {
                /** @description Authorization code. */
                code?: string;
                /** @description Anti-forgery state. */
                state?: string;
                /** @description Error reported by the issuer. */
                error?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Redirect back to the UI. */
            302: {
                headers: {
                    Location?: string;
                    /** @description Session cookie (HttpOnly, Secure, SameSite=Lax), set only when sign-in succeeded. */
                    "Set-Cookie"?: string;
                    [name: string]: unknown;
                };
                content?: never;
            };
            404: components["responses"]["NotFound"];
        };
    };
    localSignIn: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["LocalSignInRequest"];
            };
        };
        responses: {
            /** @description Signed in; session cookie set. */
            200: {
                headers: {
                    "Set-Cookie"?: string;
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Session"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            429: components["responses"]["RateLimited"];
            500: components["responses"]["InternalError"];
        };
    };
    changeLocalPassword: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ChangePasswordRequest"];
            };
        };
        responses: {
            /** @description Password changed. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Session"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            429: components["responses"]["RateLimited"];
            500: components["responses"]["InternalError"];
        };
    };
    logout: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Session ended; cookie cleared. */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    getHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Healthy. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Health"];
                };
            };
            /** @description Not healthy. */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Health"];
                };
            };
        };
    };
    getAuthMethods: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Methods. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuthMethods"];
                };
            };
            500: components["responses"]["InternalError"];
        };
    };
    getSession: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description The session. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Session"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            500: components["responses"]["InternalError"];
        };
    };
    getMe: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Own summary. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Me"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    listMyHolds: {
        parameters: {
            query?: {
                /** @description Restrict to active or released holds. */
                state?: "active" | "released";
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Holds. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": {
                        holds: components["schemas"]["Hold"][];
                    };
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    listMyLedger: {
        parameters: {
            query?: {
                /** @description Period as `YYYY-MM`; defaults to the current period. */
                period?: components["parameters"]["PeriodQuery"];
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Ledger. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["LedgerResponse"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    getMyUsage: {
        parameters: {
            query?: {
                /** @description Period as `YYYY-MM`; defaults to the current period. */
                period?: components["parameters"]["PeriodQuery"];
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Usage. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["UsageResponse"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    getPressure: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Pressure. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Pressure"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    listSetups: {
        parameters: {
            query?: {
                /** @description Label selector, e.g. `tier=a,site in (x,y)`. */
                selector?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Setups. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SetupList"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    acquireSetup: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description The new hold. */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Hold"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    releaseHold: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody?: {
            content: {
                "application/json": components["schemas"]["ReleaseRequest"];
            };
        };
        responses: {
            /** @description The released hold. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Hold"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    listAdminUsers: {
        parameters: {
            query?: {
                status?: components["schemas"]["UserStatus"];
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Users. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AdminUserList"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    setUserRoles: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SetRolesRequest"];
            };
        };
        responses: {
            /** @description The updated user. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AdminUser"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    rejectUser: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description The updated user. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AdminUser"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    grantTokens: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["GrantRequest"];
            };
        };
        responses: {
            /** @description The ledger entry and the updated user. */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BalanceChangeResult"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    resetBalance: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ResetRequest"];
            };
        };
        responses: {
            /** @description The ledger entry and the updated user. */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["BalanceChangeResult"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    createSetup: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SetupInput"];
            };
        };
        responses: {
            /** @description The new setup (admin view). */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Setup"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    updateSetup: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["SetupInput"];
            };
        };
        responses: {
            /** @description The updated setup (admin view). */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Setup"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    deleteSetup: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Deleted. */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    getRules: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Rules. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RulesResponse"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    replaceRules: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["RulesInput"];
            };
        };
        responses: {
            /** @description Saved rules. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RulesResponse"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    getSettings: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Settings. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SettingsResponse"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    updateSettings: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["Settings"];
            };
        };
        responses: {
            /** @description Saved settings. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["SettingsResponse"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    getCollector: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Collector state. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["Collector"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    listAudit: {
        parameters: {
            query?: {
                /** @description Actor user id. */
                actor?: number;
                /** @description Exact action name, e.g. `ledger.grant`. */
                action?: string;
                limit?: number;
                offset?: number;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Audit entries. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AuditList"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
}
