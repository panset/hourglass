// SPDX-License-Identifier: MIT
import { api } from "../../api/client";
import type { components } from "../../api/schema.gen";

type S = components["schemas"];

export type AdminUser = S["AdminUser"];
export type AdminUserList = S["AdminUserList"];
export type Role = S["Role"];

export const listUsers = () => api.get<AdminUserList>("/api/v1/admin/users");
export const listPending = () => api.get<AdminUserList>("/api/v1/admin/users?status=pending");
export const setRoles = (id: number, roles: Role[]) =>
  api.put<AdminUser>(`/api/v1/admin/users/${id}/roles`, { roles } satisfies S["SetRolesRequest"]);
export const rejectUser = (id: number) => api.post<AdminUser>(`/api/v1/admin/users/${id}/reject`);
export const grantTokens = (id: number, body: S["GrantRequest"]) =>
  api.post<S["BalanceChangeResult"]>(`/api/v1/admin/users/${id}/grants`, body);
export const resetBalance = (id: number, reason: string) =>
  api.post<S["BalanceChangeResult"]>(`/api/v1/admin/users/${id}/reset`, { reason } satisfies S["ResetRequest"]);
