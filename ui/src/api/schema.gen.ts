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
         * @description Newest first, by descending `id`. `period` defaults to the current period; a malformed value is 400 `validation`. `available_periods` feeds the Month select. Without `rollup`, `limit` and `cursor` the response is one entry per stored row of the period, unpaged, with `next_cursor` null (LED-11). `rollup=day` merges the debit rows of one hold and local day (configured timezone), raw or rolled up in storage, into one entry (LED-12). `limit` pages the view and `next_cursor` leads to the next older page; the cursor applies to the view's entries after merging, so a walk returns no entry twice (LED-13). `balance_after_tokens` is the running sum of the view's entries of the period up to and including the entry, in id order, on every page (LED-14). Bad `rollup`, `limit` or `cursor` values are 400 `validation`.
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
         * @description Visible to every user with a role. Also returns `display_label_keys`,
         *     kept for compatibility; the Setups page does not read it. Admins
         *     additionally get disabled setups and the `admin` sub-object; for other
         *     users `admin` is null and disabled setups are omitted. `selector` uses
         *     the RULE-2 syntax; a bad one is 400 `invalid_selector` with the parse
         *     error. The state filter and exact-match label tokens are applied
         *     client-side on this list.
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
         * @description HOLD-2, HOLD-6. The owner or an admin may call it. With an empty body
         *     or `{"force": false}` it is a normal release. While the provider still
         *     reports the setup and `release_requires_teardown` is on, the hold is
         *     not released: the answer is 202 with the still-active hold, whose
         *     `release_requested_at` is set (once; a repeat request returns the same
         *     202 and fires nothing) and `hold.releasing` fires. The hold stays
         *     active and metered, and the collector releases it at the first
         *     successful poll in which the setup is no longer reported. A pending
         *     release cannot be cancelled. In every other situation the release is
         *     immediate (200). `{"force": true, "reason": "..."}` is the admin
         *     force-release: it skips the teardown check, completes a pending release
         *     at once, needs a non-empty reason (400 `validation`, `field` =
         *     `reason`) and is audited. `force` by a non-admin, or a normal release
         *     of another user's hold, is 403 `forbidden`. An already released hold is
         *     409 `conflict`.
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
    "/api/v1/admin/preapprovals": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List pre-approved SSO users from the database and the file
         * @description PRE-5, PRE-6. Every pre-approval, by email: the database list managed
         *     here (`source` = `database`) and the read-only file named by
         *     `preapproved_users_file` (`source` = `config`). An email in both
         *     sources appears twice, the database entry first. A file entry has
         *     `id`, `created_at`, `created_by`, `used_at` and `used_by` null. A
         *     database entry has `used_at` and `used_by` null until a pending SSO
         *     user with that email claim first signs in; an entry is never consumed.
         *     A user who signs in with a matching email claim gets the union of the
         *     roles of both sources.
         *
         */
        get: operations["listPreapprovals"];
        put?: never;
        /**
         * Pre-approve SSO users by email
         * @description PRE-5. Adds 1 to 1,000 emails to the database list with one role set.
         *     An email already in the list gets the new roles and keeps its id,
         *     creator and use record. Emails are stored trimmed and in lower case;
         *     an email given more than once in a request counts once. An email is
         *     valid when it is at most 254 characters after trimming, has exactly
         *     one `@` with text on both sides, and contains no whitespace, comma or
         *     `*`. Roles must not be empty and follow the rules of `setUserRoles`.
         *     All or nothing: any invalid email or role is 400 `validation` naming
         *     the first bad value (`field` = `emails` or `roles`), and nothing is
         *     stored. A pending user is approved at their next sign-in. Audited as
         *     `preapproval.add` with the emails and roles.
         *
         */
        post: operations["addPreapprovals"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/preapprovals/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /**
         * Remove a pre-approval from the database list
         * @description PRE-5. Removes one database entry. It changes no user, including one
         *     it already approved. File entries have no id and cannot be removed
         *     here. Audited as `preapproval.delete`.
         *
         */
        delete: operations["deletePreapproval"];
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
         * @description Full replacement of `SetupInput`. Setting `enabled: false` on a held setup is 409 `has_active_hold` (SET-5). Changing the name of a held setup is also 409 `has_active_hold` (the name ties the setup to what the provider reports, D40); release the hold first. Other edits to a held setup are allowed. A duplicate name is 409 `conflict`.
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
    "/api/v1/admin/service-accounts": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * List service accounts with their tokens
         * @description SVC-6. Every account by name, with its scopes, enabled flag and each
         *     token's id, creation, expiry and last-used times. A token itself is
         *     never listed (SVC-4).
         *
         */
        get: operations["listServiceAccounts"];
        put?: never;
        /**
         * Create a service account
         * @description SVC-6. The account starts enabled and without a token. `name` is 1 to
         *     64 letters, digits, `.`, `_` or `-`, starting with a letter or digit,
         *     and cannot change later; a duplicate is 409 `conflict`. `scopes` is any
         *     subset of the four scopes; an unknown scope is 400 `validation`
         *     (`field` = `scopes`). Audited as `service_account.create`.
         *
         */
        post: operations["createServiceAccount"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/service-accounts/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Change a service account's description, scopes and enabled flag
         * @description SVC-5, SVC-6. Full replacement of the three fields, all required.
         *     Disabling the account, or removing a scope, takes effect on the
         *     account's next request. Audited as `service_account.update`.
         *
         */
        put: operations["updateServiceAccount"];
        post?: never;
        /**
         * Delete a service account and its tokens
         * @description SVC-5, SVC-6. Its tokens are refused from the next request on. Holds
         *     and audit entries keep the account's name. Audited as
         *     `service_account.delete`.
         *
         */
        delete: operations["deleteServiceAccount"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/service-accounts/{id}/tokens": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Issue a token for a service account
         * @description SVC-4, SVC-6. The response is the only place the token ever appears:
         *     Hourglass stores only a SHA-256 hash of its secret and cannot show it
         *     again. The token has the form `hgsa_<token id>_<secret>` and is sent
         *     as `Authorization: Bearer <token>`. The body is optional;
         *     `expires_at`, when given, must be in the future (400 `validation`,
         *     `field` = `expires_at`). An account may hold several live tokens, for
         *     rotation. Audited as `service_account.token_issue` (token id and
         *     expiry only).
         *
         */
        post: operations["issueServiceToken"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/admin/service-accounts/{id}/tokens/{tokenId}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /**
         * Revoke one token
         * @description SVC-5, SVC-6. The token is refused from the next request on. A token
         *     id that does not belong to the account is 404 `not_found`. Audited as
         *     `service_account.token_revoke`.
         *
         */
        delete: operations["revokeServiceToken"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/integration/pressure": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Report the raw lab pressure
         * @description Scope `pressure:write`. `value` is required, finite, from 0 to 1
         *     inclusive (INT-1); `observed_at` defaults to the time of receipt.
         *     `expires_at` is `observed_at` plus the configured `max_age`. A report
         *     older than the stored one is 409 `stale_report` (INT-3); an equal
         *     `observed_at` replaces it. An `observed_at` more than 2 minutes in the
         *     future is 400 `validation`. Without the `reported` pressure source
         *     configured it is 409 `not_configured` (INT-4). A report takes effect
         *     at the next poll.
         *
         */
        put: operations["reportPressure"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/integration/deployments": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Report the complete set of deployed setups
         * @description Scope `deployments:write`. The list is the complete set deployed now;
         *     an absent setup is not deployed, an empty list means nothing is. It
         *     replaces the stored set atomically (INT-2). A name that matches no
         *     setup is accepted, ignored and listed in `unknown`. A duplicate name
         *     is 400 `validation`. Limits: 10,000 entries, 32 labels per entry, 1 MB
         *     body. Ordering, clock skew and `not_configured` (the `reported`
         *     provider) are as for the pressure report.
         *
         */
        put: operations["reportDeployments"];
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/integration/state": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Everything a service needs to act and to reconcile
         * @description Scope `state:read` (INT-8). Every setup, disabled ones included, with
         *     its active hold (`null` for a free setup) and the holder's id,
         *     username, email and name. `reported_deployed` reflects the stored
         *     report, not the last poll. `reported` and the `observed_at` fields are
         *     `null` before the first report, and always `null` (and nothing is
         *     `reported_deployed`) when the pressure source or the provider is not
         *     the `reported` one. `weights` is the `reported_weights` switch and
         *     limits and the stored weights report's times (null when the setting is
         *     off or before the first report); each setup's `weight` is the weight a
         *     poll now would charge, its source, the stored reported weight and why
         *     it is not applied (`reported` and `not_applied` are null for every
         *     setup with the setting off). Never includes balances, ledger entries,
         *     other users, rate rules, other settings or audit data.
         *
         */
        get: operations["getIntegrationState"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/integration/holds/{id}/release": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Release a hold from the service's automation
         * @description Scope `holds:release` (HOLD-8). `reason` is required, 1 to 500
         *     characters after trimming (400 `validation`, `field` = `reason`). The
         *     teardown check is skipped. The hold, including one with a pending
         *     release, is released at once, `released_by` stays null, the account's
         *     name is recorded on it, an audit entry
         *     `hold.service_release` is written and `hold.released` fires with
         *     `service_account` in `data`. 404 `not_found` for an unknown id, 409
         *     `conflict` if already released.
         *
         */
        post: operations["serviceReleaseHold"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/integration/weights": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        /**
         * Report the complete set of reported weights
         * @description Scope `weights:write`; no other scope allows this route and this scope
         *     allows no other. The list is the complete set of reported weights from
         *     one writer: an absent setup has no reported weight and is priced by its
         *     override, rate rule or default; an empty list means no setup has one.
         *     It replaces the stored set atomically and takes effect at the next poll
         *     and the next read; it never triggers a poll, a debit or an event.
         *
         *     Checks run in this order: scope, then whether the `reported_weights`
         *     setting is on (off is 409 `not_configured` whatever the body, and
         *     nothing is stored), then the body, then ordering. A missing or null
         *     `weights` is 400 `validation`. Each entry is checked in turn; the first
         *     invalid one refuses the whole report with 400 `validation`, `field`
         *     `weights`, and a message that gives its position (from 0), its setup
         *     name when that is a non-empty string, and the reason, for example
         *     `weights[3] (setup "lab-03"): weight 25 is above the maximum 20`.
         *     `weight` must be a JSON number (not a string), finite, and within the
         *     setting's `min` and `max`, both inclusive; negative zero counts as 0.
         *     `setup` must be a non-empty string, not repeated. `attributes` is
         *     optional: at most 16, keys under the label key rule, string values of
         *     at most 63 characters; they are stored and shown, never interpreted.
         *     An entry holds only these three fields. A body that is not valid JSON
         *     (bare `NaN` or `Infinity` included) is the generic 400 and names no
         *     entry. Nothing is stored on any refusal, and the previous report stays
         *     in force until it expires. A name that matches no setup is validated,
         *     stored, ignored and listed in `unknown`. Limits: 10,000 entries and a
         *     1 MB body. Ordering and clock skew are those of the other reports,
         *     compared with the stored weights report only: an older `observed_at`
         *     is 409 `stale_report`, an equal one replaces, more than 2 minutes ahead
         *     is 400 `validation`. `expires_at` is `observed_at` plus the setting's
         *     `max_age`.
         *
         */
        put: operations["reportWeights"];
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
        ErrorCode: "unauthenticated" | "no_role" | "forbidden" | "csrf" | "password_change_required" | "not_found" | "validation" | "invalid_selector" | "insufficient_tokens" | "hold_limit" | "setup_held" | "setup_disabled" | "hook_veto" | "has_active_hold" | "last_admin" | "self_demote" | "local_admin_protected" | "conflict" | "stale_report" | "not_configured" | "rate_limited" | "internal";
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
         *     these fields and nothing else (AUTH-5, AUTH-9): `roles` is empty.
         *     `display_name` is the name, else the username, else the email, by the
         *     same rule the rest of the API uses to show a user; it is derived only
         *     from fields this response already carries.
         *      */
        Session: {
            /** Format: int64 */
            user_id: number;
            username: string;
            email: string;
            name: string;
            /** @description Name, else username, else email; empty only when all three are empty. */
            display_name: string;
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
            /**
             * @description The weight the setup resolves to now, or the hold's latest usage sample's weight when that sample's source is `policy`.
             * @example 1
             */
            weight: number;
            weight_source: components["schemas"]["HoldWeightSource"];
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
            /** @description Active hold: the weight its setup resolves to now, or the latest usage sample's weight when that sample's source is `policy`. Released hold: the weight of its last usage sample (what it was last charged at), or, without a sample, what the setup resolves to now. */
            weight: number;
            /**
             * @description Where `weight` comes from. Null only for a released hold whose last usage sample was written before the upgrade that added sources.
             * @enum {string|null}
             */
            weight_source: "override" | "reported" | "rule" | "default" | "policy" | null;
            /** @description The setup's reported attributes when its reported weight is usable; `{}` otherwise, and always `{}` for a released hold with a usage sample. */
            weight_attributes: components["schemas"]["WeightAttributes"];
            /** Format: date-time */
            acquired_at: string;
            /** Format: date-time */
            metering_started_at: string | null;
            /** Format: date-time */
            deployed_at: string | null;
            /**
             * Format: date-time
             * @description Set when the owner asked to release while the setup was still deployed; the hold stays active until the setup is no longer reported. Always sent, null when unset.
             */
            release_requested_at: string | null;
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
        /** @description A ledger entry: a stored ledger row, or, in `GET /api/v1/me/ledger` with `rollup=day`, the debit rows of one hold and local day merged. */
        LedgerEntry: {
            /**
             * Format: int64
             * @description The stored row's id; for a merged `rollup=day` entry, the id of its newest row (the row storage compaction keeps).
             */
            id: number;
            /**
             * Format: date-time
             * @description For a merged `rollup=day` entry, that of its newest row.
             */
            created_at: string;
            /** @example 2026-11 */
            period: string;
            type: components["schemas"]["LedgerEntryType"];
            /** @description Signed; debits are negative. */
            amount_tokens: components["schemas"]["Tokens"];
            /** @description On `GET /api/v1/me/ledger`, the running sum of the view's entries of the period, in id order, up to and including this entry. */
            balance_after_tokens: components["schemas"]["Tokens"];
            /** @description Multiplier of the poll, for debits (shown as "Debit · 2×"). Null when `entry_count` is above 1. */
            multiplier: number | null;
            setup_name: string | null;
            /** Format: int64 */
            hold_id: number | null;
            /** @description Granting or resetting admin. Null for a merged `rollup=day` entry. */
            actor_name: string | null;
            /** @description Null for a merged `rollup=day` entry. */
            reason: string | null;
            /** @description Always present on `GET /api/v1/me/ledger` entries, and absent on the admin grant and reset responses. The number of polls the entry stands for: 1 for a raw debit or a non-debit row, the stored roll-up count for a row compacted in storage, the sum over the merged rows in the `rollup=day` view. */
            entry_count?: number;
            /** @description Always present on `GET /api/v1/me/ledger` entries, and absent on the admin grant and reset responses. For a debit, the lowest weight among the charged polls the entry stands for (the usage samples of its hold with a debit above zero: the one poll of a raw row, the local day of a rolled-up or merged entry). Null unless a sample is found for every poll (`entry_count`), and for entries that are not debits. */
            weight_min?: number | null;
            /** @description As `weight_min`, the highest weight. Equal to `weight_min` for a single poll. */
            weight_max?: number | null;
            /** @description As `weight_min`, the distinct weight sources of those polls, `policy` first and then `override`, `reported`, `rule`, `default`. Null when `weight_min` is null or any of those samples has no source. */
            weight_sources?: components["schemas"]["HoldWeightSource"][] | null;
        };
        LedgerResponse: {
            period: string;
            /** @description Periods with data, newest first (feeds the Month select). */
            available_periods: string[];
            entries: components["schemas"]["LedgerEntry"][];
            /** @description Pass as `cursor` (with the same `period`, `rollup` and `limit`) for the next older page. Null when no older entry exists, and always null without `limit`. */
            next_cursor: string | null;
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
            /**
             * Format: date-time
             * @description Set while the holder's release waits for the setup to be torn down (HOLD-6). Always sent, null when unset.
             */
            release_requested_at: string | null;
        };
        /** @description Admin-only fields. */
        SetupAdminFields: {
            enabled: boolean;
            /**
             * Format: date-time
             * @description Last time the provider reported an allocation with the setup's exact name; null if never.
             */
            last_seen_at: string | null;
            /** @description Selector of the first rate rule that matches the setup's labels, whatever the weight's source; null when no rule matches. */
            matched_rule_selector: string | null;
            /** @description The stored weights report's weight for this setup, usable or not; null when there is none or reported weights are off. */
            reported_weight: number | null;
            /**
             * @description Why `reported_weight` is not applied; null when it is applied, when there is none, or when reported weights are off.
             * @enum {string|null}
             */
            reported_weight_not_applied: "expired" | "out_of_bounds" | "override" | null;
            /** @description The setup's weight override; null when none. */
            weight_override: number | null;
        };
        Setup: {
            /** Format: int64 */
            id: number;
            /** @example lab-01 */
            name: string;
            labels: components["schemas"]["Labels"];
            /**
             * @description The weight Hourglass applies now; `weight_source` says where it comes from.
             * @example 1
             */
            weight: number;
            weight_source: components["schemas"]["WeightSource"];
            /** @description The reported attributes when the setup's reported weight is usable (also under an override); `{}` otherwise. */
            weight_attributes: components["schemas"]["WeightAttributes"];
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
            /** @description The `reported_weights` setting is on, so prices may change, even for a setup with no reported weight yet. */
            reported_weights_enabled: boolean;
            setups: components["schemas"]["Setup"][];
            /** @description Every label key and value over every setup the caller may see
             *     (enabled setups for a developer, all setups for an admin),
             *     ignoring `selector` like `summary`. Keys sorted, values sorted and
             *     distinct. `[]` when no setup has a label (SET-8).
             *      */
            label_index: components["schemas"]["LabelIndexKey"][];
            /**
             * @description Kept for compatibility; no page reads it since v1.3.0.
             * @example [
             *       "tier"
             *     ]
             */
            display_label_keys: string[];
            /** @description Counts over all enabled setups, ignoring `selector` ("9 of 11 setups deployed · 2 free"). */
            summary: {
                enabled: number;
                deployed: number;
                free: number;
            };
            /** @description Always null; the metering policy interface has no describe method. The per-hold reason is `metering.reason`. */
            metering_description: string | null;
            /** @description What every `cost_per_hour_tokens` in this response was computed
             *     with, read once per request (UI-18). Added in v1.3.0; a client
             *     should treat it as optional when it may talk to an older server.
             *      */
            pricing: {
                /** @description The multiplier of `cost_per_hour_tokens.now`. */
                multiplier: number;
                /** @description The highest band's multiplier, of `cost_per_hour_tokens.at_peak`. */
                peak_multiplier: number;
                /** @description Smoothed pressure of the sample `multiplier` comes from; null before the first successful poll. */
                pressure: number | null;
            };
        };
        LabelIndexKey: {
            /** @example site */
            key: string;
            values: components["schemas"]["LabelIndexValue"][];
        };
        LabelIndexValue: {
            /** @example sjc */
            value: string;
            /**
             * @description The selector term that selects exactly the setups with this value
             *     (`key=value`, `key=` for an empty value). Null when the value
             *     cannot be written in a selector: it contains whitespace or one of
             *     `, ( ) = !`.
             *
             * @example site=sjc
             */
            selector: string | null;
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
            /** @description Optional. Pins the setup's weight above a reported weight and the rate rules; not bound by the reported weights' limits. On create, absent or null means none. On update, absent leaves it unchanged, null clears it, a number sets it. */
            weight_override?: number | null;
        };
        /**
         * @description Where a setup's weight comes from.
         * @enum {string}
         */
        WeightSource: "override" | "reported" | "rule" | "default";
        /**
         * @description Where a charged or shown weight comes from; `policy` when the metering policy set the weight.
         * @enum {string}
         */
        HoldWeightSource: "override" | "reported" | "rule" | "default" | "policy";
        WeightAttributes: {
            [key: string]: string;
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
        /**
         * @description `database`: added through the admin API or page. `config`: read from `preapproved_users_file`.
         * @enum {string}
         */
        PreapprovalSource: "database" | "config";
        PreapprovalUser: {
            /** Format: int64 */
            id: number;
            /** @description The display name, or "unknown user". */
            name: string;
        } | null;
        Preapproval: {
            /**
             * Format: int64
             * @description Null for a file entry.
             */
            id: number | null;
            /** @description Trimmed and in lower case. */
            email: string;
            roles: components["schemas"]["Role"][];
            source: components["schemas"]["PreapprovalSource"];
            /**
             * Format: date-time
             * @description Null for a file entry.
             */
            created_at: string | null;
            /** @description The admin who added the entry. Null for a file entry. */
            created_by: components["schemas"]["PreapprovalUser"] | null;
            /**
             * Format: date-time
             * @description The first sign-in it approved; null until used, always null for a file entry.
             */
            used_at: string | null;
            /** @description The user that first sign-in approved; null until used, always null for a file entry. */
            used_by: components["schemas"]["PreapprovalUser"] | null;
        };
        PreapprovalList: {
            /** @description By email; for an email in both sources, the database entry first. */
            preapprovals: components["schemas"]["Preapproval"][];
        };
        AddPreapprovalsRequest: {
            emails: string[];
            roles: components["schemas"]["Role"][];
        };
        AddPreapprovalsResult: {
            /** @description Emails that were not in the database list. */
            added: number;
            /** @description Emails already in the list whose roles were replaced. */
            updated: number;
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
            /**
             * @description Kept for compatibility; no page reads it since v1.3.0.
             * @example [
             *       "tier"
             *     ]
             */
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
            reported_weights?: components["schemas"]["ReportedWeightsSetting"];
        };
        /** @description Whether a service account may report a weight per setup, and the limits such a weight must lie within. Always returned by `GET`. Optional on `PUT`: a body without it leaves the stored value unchanged; when present, all four fields are required. Changes apply from the next poll and the next read. */
        ReportedWeightsSetting: {
            /**
             * @description Can be true only when `max` is set.
             * @default false
             */
            enabled: boolean;
            /**
             * @description Lowest weight a report may carry, inclusive; at most `max` when `max` is set.
             * @default 0
             */
            min: number;
            /**
             * @description Highest weight a report may carry, inclusive; null when no maximum is set.
             * @default null
             */
            max: number | null;
            /**
             * @description Duration string from `1m` to `24h`: how long a weights report stays usable after its `observed_at`. Should be at least twice the poll interval.
             * @default 10m
             * @example 10m
             */
            max_age: string;
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
        ReportedReportTimes: {
            /** Format: date-time */
            observed_at: string;
            /** Format: date-time */
            received_at: string;
            /** @description Name of the service account that sent the report. */
            service_account: string;
        };
        Collector: {
            extensions: {
                provider: string | null;
                pressure_source: string;
                metering_policy: string;
                hooks: string[];
            };
            /** @description The stored pressure report. Null before the first one and when the pressure source is not `reported`. */
            last_pressure_report: components["schemas"]["ReportedReportTimes"] | null;
            /** @description The stored deployment report. Null before the first one and when the provider is not `reported`. */
            last_deployments_report: components["schemas"]["ReportedReportTimes"] | null;
            /** @description The stored weights report. Null before the first one and while reported weights are off. */
            last_weights_report: {
                /** Format: date-time */
                observed_at: string;
                /** Format: date-time */
                received_at: string;
                /** @description Name of the service account that sent the report. */
                service_account: string;
                /** @description How many weights the stored report holds. */
                entries: number;
                /** @description How many of them the minimum and maximum now in force exclude. */
                out_of_bounds: number;
            } | null;
            /** @description The most recent weights report refused with 400, kept in memory (lost at a restart). Null when none was refused since the start and while reported weights are off. */
            last_weights_refusal: {
                /** Format: date-time */
                at: string;
                service_account: string;
                message: string;
            } | null;
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
        /**
         * @description What a service account may call (one integration route each).
         * @enum {string}
         */
        ServiceScope: "state:read" | "pressure:write" | "deployments:write" | "holds:release" | "weights:write";
        /** @description A token as listed. The token itself is shown only once, when issued. */
        ServiceToken: {
            /** Format: int64 */
            id: number;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expires_at: string | null;
            /**
             * Format: date-time
             * @description Recorded at most once a minute.
             */
            last_used_at: string | null;
        };
        IssuedServiceToken: {
            /** Format: int64 */
            id: number;
            /** Format: date-time */
            created_at: string;
            /** Format: date-time */
            expires_at: string | null;
            /** Format: date-time */
            last_used_at: string | null;
            /** @description `hgsa_<token id>_<secret>`. Shown in this response only; store it now. */
            token: string;
        };
        ServiceAccount: {
            /** Format: int64 */
            id: number;
            name: string;
            description: string;
            scopes: components["schemas"]["ServiceScope"][];
            enabled: boolean;
            /** Format: date-time */
            created_at: string;
            /** @description Oldest first. */
            tokens: components["schemas"]["ServiceToken"][];
        };
        ServiceAccountList: {
            /** @description By name. */
            service_accounts: components["schemas"]["ServiceAccount"][];
        };
        CreateServiceAccountRequest: {
            name: string;
            description?: string;
            scopes: components["schemas"]["ServiceScope"][];
        };
        UpdateServiceAccountRequest: {
            description: string;
            scopes: components["schemas"]["ServiceScope"][];
            enabled: boolean;
        };
        IssueServiceTokenRequest: {
            /**
             * Format: date-time
             * @description Optional; must be in the future.
             */
            expires_at?: string;
        };
        PressureReport: {
            /** @description Raw pressure; Hourglass smooths it. */
            value: number;
            /**
             * Format: date-time
             * @description Defaults to the time of receipt.
             */
            observed_at?: string;
        };
        PressureReportResult: {
            value: number;
            /** Format: date-time */
            observed_at: string;
            /** Format: date-time */
            received_at: string;
            /** Format: date-time */
            expires_at: string;
        };
        DeploymentReport: {
            /** @description The setup's name in Hourglass. */
            setup: string;
            /** @description Free-form; shown to admins like a provider's allocation labels. */
            labels?: components["schemas"]["Labels"];
        };
        DeploymentsReport: {
            /**
             * Format: date-time
             * @description Defaults to the time of receipt.
             */
            observed_at?: string;
            /** @description The complete set deployed now; empty means nothing is deployed. */
            deployments: components["schemas"]["DeploymentReport"][];
        };
        DeploymentsReportResult: {
            /** Format: date-time */
            observed_at: string;
            /** Format: date-time */
            received_at: string;
            /** Format: date-time */
            expires_at: string;
            accepted: number;
            /** @description Reported names that match no setup. */
            unknown: string[];
        };
        StateHolder: {
            /** Format: int64 */
            id: number;
            username: string;
            email: string;
            name: string;
        };
        StateHold: {
            /** Format: int64 */
            id: number;
            user: components["schemas"]["StateHolder"];
            /** Format: date-time */
            acquired_at: string;
            /** Format: date-time */
            metering_started_at: string | null;
            /** Format: date-time */
            deployed_at: string | null;
            /** Format: date-time */
            release_requested_at: string | null;
        };
        StateSetup: {
            name: string;
            labels: components["schemas"]["Labels"];
            details: {
                [key: string]: unknown;
            };
            enabled: boolean;
            /** @description From the stored report, not the last poll. */
            reported_deployed: boolean;
            /** @description The active hold; null for a free setup. */
            hold: components["schemas"]["StateHold"] | null;
            weight: components["schemas"]["StateSetupWeight"];
        };
        /** @description The weight a poll now would charge this setup, and the stored reported weight. The rate rule's selector is not returned. */
        StateSetupWeight: {
            /** @description The weight applied now. */
            applied: number;
            /**
             * @description Where `applied` comes from.
             * @enum {string}
             */
            source: "override" | "reported" | "rule" | "default";
            /** @description The stored report's weight for this setup, usable or not; null when there is none or reported weights are off. */
            reported: number | null;
            /**
             * @description Why `reported` is not applied, the first of `expired`, `out_of_bounds` and `override` that holds; null when it is applied, when there is none, or when reported weights are off.
             * @enum {string|null}
             */
            not_applied: "expired" | "out_of_bounds" | "override" | null;
            /** @description The entry's attributes when it is usable; `{}` otherwise. */
            attributes: {
                [key: string]: string;
            };
        };
        /** @description The `reported_weights` switch and limits, and the stored weights report's times (null when reported weights are off or before the first report). */
        StateWeights: {
            enabled: boolean;
            min: number;
            /** @description Null when no maximum is set. */
            max: number | null;
            /** Format: date-time */
            observed_at: string | null;
            /** Format: date-time */
            expires_at: string | null;
        };
        WeightReport: {
            /** @description The setup's name in Hourglass. */
            setup: string;
            /** @description Finite, and within the setting's min and max, both inclusive. */
            weight: number;
            /** @description For display only; keys follow the label key rule. */
            attributes?: {
                [key: string]: string;
            };
        };
        WeightsReport: {
            /**
             * Format: date-time
             * @description Defaults to the time of receipt.
             */
            observed_at?: string;
            /** @description The complete set of reported weights; empty means no setup has one. */
            weights: components["schemas"]["WeightReport"][];
        };
        WeightsReportResult: {
            /** Format: date-time */
            observed_at: string;
            /** Format: date-time */
            received_at: string;
            /**
             * Format: date-time
             * @description `observed_at` plus the setting's `max_age`.
             */
            expires_at: string;
            /** @description Entries whose name matches a setup. */
            accepted: number;
            /** @description Reported names that match no setup. */
            unknown: string[];
        };
        IntegrationState: {
            /** Format: date-time */
            server_time: string;
            poll: {
                interval_seconds: number;
                /** Format: date-time */
                last_ok_at: string | null;
            };
            pressure: {
                /** @description The configured pressure source. */
                source: string | null;
                reported: {
                    value: number;
                    /** Format: date-time */
                    observed_at: string;
                    /** Format: date-time */
                    expires_at: string;
                } | null;
                smoothed: number | null;
                multiplier: number;
            };
            deployments: {
                /** @description The configured provider. */
                provider: string | null;
                /** Format: date-time */
                observed_at: string | null;
                /** Format: date-time */
                expires_at: string | null;
            };
            weights: components["schemas"]["StateWeights"];
            setups: components["schemas"]["StateSetup"][];
        };
        ServiceReleaseRequest: {
            reason: string;
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
        /** @description 401 `unauthenticated`. Missing, wrong, expired or revoked token, or a disabled or deleted account; the reason is not given (SVC-5). A session cookie is not read (SVC-2). */
        ServiceUnauthenticated: {
            headers: {
                [name: string]: unknown;
            };
            content: {
                "application/json": components["schemas"]["Error"];
            };
        };
        /** @description 403 `forbidden`. The token lacks the route's scope; nothing else is said (SVC-3). */
        ServiceForbidden: {
            headers: {
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
                /** @description `day` merges debit rows into one entry per hold and local day. Absent: one entry per stored row. */
                rollup?: "day";
                /** @description Page size. Absent, every entry of the view is returned and `next_cursor` is null. */
                limit?: number;
                /** @description The `next_cursor` of the previous page, for the same `period` and `rollup`. Opaque to clients. Requires `limit`; without it, or malformed, is 400 `validation`. */
                cursor?: string;
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
            /** @description The setup is still deployed. The hold is still active with `release_requested_at` set; it is released when the setup is no longer reported. */
            202: {
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
    listPreapprovals: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Pre-approvals. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PreapprovalList"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    addPreapprovals: {
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
                "application/json": components["schemas"]["AddPreapprovalsRequest"];
            };
        };
        responses: {
            /** @description How many emails were added and how many existing entries were updated. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["AddPreapprovalsResult"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    deletePreapproval: {
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
            /** @description Removed. */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
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
    listServiceAccounts: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Service accounts. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ServiceAccountList"];
                };
            };
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            500: components["responses"]["InternalError"];
        };
    };
    createServiceAccount: {
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
                "application/json": components["schemas"]["CreateServiceAccountRequest"];
            };
        };
        responses: {
            /** @description The new account. */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ServiceAccount"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            409: components["responses"]["Conflict"];
            500: components["responses"]["InternalError"];
        };
    };
    updateServiceAccount: {
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
                "application/json": components["schemas"]["UpdateServiceAccountRequest"];
            };
        };
        responses: {
            /** @description The updated account. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ServiceAccount"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            500: components["responses"]["InternalError"];
        };
    };
    deleteServiceAccount: {
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
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            500: components["responses"]["InternalError"];
        };
    };
    issueServiceToken: {
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
                "application/json": components["schemas"]["IssueServiceTokenRequest"];
            };
        };
        responses: {
            /** @description The new token, shown once. */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["IssuedServiceToken"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            500: components["responses"]["InternalError"];
        };
    };
    revokeServiceToken: {
        parameters: {
            query?: never;
            header: {
                /** @description The `csrf_token` from `GET /api/v1/session`. A missing or wrong token is 403 `csrf`. */
                "X-CSRF-Token": components["parameters"]["CsrfToken"];
            };
            path: {
                id: components["parameters"]["Id"];
                tokenId: number;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Revoked. */
            204: {
                headers: {
                    [name: string]: unknown;
                };
                content?: never;
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["Unauthenticated"];
            403: components["responses"]["Forbidden"];
            404: components["responses"]["NotFound"];
            500: components["responses"]["InternalError"];
        };
    };
    reportPressure: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["PressureReport"];
            };
        };
        responses: {
            /** @description The stored report. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["PressureReportResult"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["ServiceUnauthenticated"];
            403: components["responses"]["ServiceForbidden"];
            409: components["responses"]["Conflict"];
            429: components["responses"]["RateLimited"];
            500: components["responses"]["InternalError"];
        };
    };
    reportDeployments: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["DeploymentsReport"];
            };
        };
        responses: {
            /** @description The stored report. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["DeploymentsReportResult"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["ServiceUnauthenticated"];
            403: components["responses"]["ServiceForbidden"];
            409: components["responses"]["Conflict"];
            429: components["responses"]["RateLimited"];
            500: components["responses"]["InternalError"];
        };
    };
    getIntegrationState: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description The state. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["IntegrationState"];
                };
            };
            401: components["responses"]["ServiceUnauthenticated"];
            403: components["responses"]["ServiceForbidden"];
            429: components["responses"]["RateLimited"];
            500: components["responses"]["InternalError"];
        };
    };
    serviceReleaseHold: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                id: components["parameters"]["Id"];
            };
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ServiceReleaseRequest"];
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
            401: components["responses"]["ServiceUnauthenticated"];
            403: components["responses"]["ServiceForbidden"];
            404: components["responses"]["NotFound"];
            409: components["responses"]["Conflict"];
            429: components["responses"]["RateLimited"];
            500: components["responses"]["InternalError"];
        };
    };
    reportWeights: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["WeightsReport"];
            };
        };
        responses: {
            /** @description The stored report. */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["WeightsReportResult"];
                };
            };
            400: components["responses"]["BadRequest"];
            401: components["responses"]["ServiceUnauthenticated"];
            403: components["responses"]["ServiceForbidden"];
            409: components["responses"]["Conflict"];
            429: components["responses"]["RateLimited"];
            500: components["responses"]["InternalError"];
        };
    };
}
