// SPDX-License-Identifier: MIT
// Typed fixtures for the admin page tests (shapes come from schema.gen.ts).
import type { components } from "../../api/schema.gen";

type S = components["schemas"];

export type AdminUser = S["AdminUser"];
export type AdminUserList = S["AdminUserList"];
export type Setup = S["Setup"];
export type SetupList = S["SetupList"];
export type RulesResponse = S["RulesResponse"];
export type SettingsResponse = S["SettingsResponse"];
export type Collector = S["Collector"];
export type AuditList = S["AuditList"];
export type AuditEntry = S["AuditEntry"];

export const priya: AdminUser = {
  id: 10,
  username: "priya",
  email: "priya@example.com",
  name: "Priya N.",
  auth_source: "oidc",
  status: "active",
  roles: ["developer"],
  is_local_admin: false,
  balance_tokens: -12,
  overdrawn: true,
  month_to_date_burn_tokens: 312,
  burn_per_hour_tokens: 4,
  active_holds: [{ hold_id: 5, setup_id: 2, setup_name: "setup-02" }],
  first_sign_in_at: "2026-10-01T09:00:00Z",
  last_login_at: "2026-11-18T09:00:00Z",
};

export const sam: AdminUser = {
  ...priya,
  id: 11,
  username: "sam",
  email: "sam@example.com",
  name: "Sam Lee",
  balance_tokens: 61,
  overdrawn: false,
  month_to_date_burn_tokens: 239,
  burn_per_hour_tokens: 0,
  active_holds: [],
};

export const localAdmin: AdminUser = {
  ...priya,
  id: 1,
  username: "admin",
  email: "",
  name: "admin",
  auth_source: "local",
  roles: ["admin"],
  is_local_admin: true,
  balance_tokens: null,
  overdrawn: false,
  month_to_date_burn_tokens: 0,
  burn_per_hour_tokens: 0,
  active_holds: [],
};

export const morgan: AdminUser = {
  ...priya,
  id: 20,
  username: "morgan",
  email: "morgan@example.com",
  name: "Morgan Q.",
  status: "pending",
  roles: [],
  balance_tokens: null,
  overdrawn: false,
  month_to_date_burn_tokens: 0,
  burn_per_hour_tokens: 0,
  active_holds: [],
  first_sign_in_at: "2026-11-18T09:14:00Z",
};

const summary = {
  user_count: 3,
  admin_count: 1,
  overdrawn_count: 1,
  debited_this_month_tokens: 551,
  granted_this_month_tokens: 0,
  grant_count_this_month: 0,
};

export function userList(users: AdminUser[]): AdminUserList {
  return {
    users,
    pending_count: users.filter((u) => u.status === "pending").length,
    summary,
    period: "2026-11",
    period_ends_at: "2026-12-01T00:00:00Z",
    default_allowance_tokens: 300,
  };
}

export const setup04: Setup = {
  id: 4,
  name: "setup-04",
  labels: { size: "medium", site: "sjc" },
  weight: 1,
  state: "held",
  holder: { user_id: 11, hold_id: 77, name: "Sam Lee", balance_tokens: 61, overdrawn: false, is_you: false },
  held_since: "2026-11-18T08:00:00Z",
  deployed: true,
  metering: null,
  cost_per_hour_tokens: { current: 1, at_highest_band: 3 } as unknown as Setup["cost_per_hour_tokens"],
  details: { rack: "r1" },
  admin: {
    enabled: true,
    resource_name: "setup-04_RP",
    resource_name_override: null,
    last_seen_at: "2026-11-18T17:28:00Z",
    matched_rule_selector: null,
  },
};

export const setup07: Setup = {
  ...setup04,
  id: 7,
  name: "setup-07",
  labels: { size: "large", site: "rtp" },
  weight: 2,
  state: "disabled",
  holder: null,
  held_since: null,
  deployed: false,
  details: null,
  admin: {
    enabled: false,
    resource_name: "lab7-pool",
    resource_name_override: "lab7-pool",
    last_seen_at: null,
    matched_rule_selector: "size=large",
  },
};

export function setupList(setups: Setup[]): SetupList {
  return {
    setups,
    display_label_keys: ["size"],
    summary: { enabled: 1, deployed: 1, free: 0 },
    metering_description: "deployed or 2 h after acquire",
  };
}

export const emptyCollector: Collector = {
  extensions: { provider: "vsphere", pressure_source: "vsphere", metering_policy: "default", hooks: [] },
  last_successful_poll_at: "2026-11-18T17:30:00Z",
  last_error: null,
  matched_setups: 9,
  orphan_count: 0,
  outages_this_month: { count: 0, minutes_not_charged: 0 },
  recent_runs: [],
  outages: [],
  orphans: [],
  failed_deliveries: [],
  queue_length: 0,
};
