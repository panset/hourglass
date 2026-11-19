// SPDX-License-Identifier: MIT
// Friendly aliases over the generated schema (schema.gen.ts). Never edit schema.gen.ts by hand.
import type { components } from "./schema.gen";

type S = components["schemas"];

export type Session = S["Session"];
export type Role = S["Role"];
export type Me = S["Me"];
export type Pressure = S["Pressure"];
export type AuthMethods = S["AuthMethods"];
export type AdminUserList = S["AdminUserList"];
export type ErrorCode = S["ErrorCode"];
export type LocalSignInRequest = S["LocalSignInRequest"];
export type ChangePasswordRequest = S["ChangePasswordRequest"];
