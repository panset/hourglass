// SPDX-License-Identifier: MIT
// Typed wrappers for the calls the shell and public pages make. Page tasks add their own beside their page.
import { api } from "./client";
import type {
  AdminUserList,
  AuthMethods,
  ChangePasswordRequest,
  LocalSignInRequest,
  Me,
  Pressure,
  Session,
} from "./types";

export const getSession = () => api.get<Session>("/api/v1/session", { skipUnauthorizedHandler: true });
export const getAuthMethods = () => api.get<AuthMethods>("/api/v1/auth/methods");
export const getMe = () => api.get<Me>("/api/v1/me");
export const getPressure = () => api.get<Pressure>("/api/v1/pressure");
export const getPendingUsers = () => api.get<AdminUserList>("/api/v1/admin/users?status=pending");
export const localSignIn = (body: LocalSignInRequest) =>
  api.post<Session>("/auth/local", body, { skipUnauthorizedHandler: true });
export const changePassword = (body: ChangePasswordRequest) =>
  api.post<Session>("/auth/local/password", body);
export const logout = () => api.post<void>("/auth/logout", undefined, { skipUnauthorizedHandler: true });
