// SPDX-License-Identifier: MIT
import { api } from "../../api/client";
import type { components } from "../../api/schema.gen";

type S = components["schemas"];

export type AuditList = S["AuditList"];
export type AuditEntry = S["AuditEntry"];
export type AdminUserList = S["AdminUserList"];

export interface AuditQuery {
  actor?: number;
  action?: string;
  limit: number;
  offset: number;
}

export const listAudit = (q: AuditQuery) => {
  const p = new URLSearchParams();
  if (q.actor !== undefined) p.set("actor", String(q.actor));
  if (q.action) p.set("action", q.action);
  p.set("limit", String(q.limit));
  p.set("offset", String(q.offset));
  return api.get<AuditList>(`/api/v1/admin/audit?${p.toString()}`);
};
export const listActors = () => api.get<AdminUserList>("/api/v1/admin/users");
