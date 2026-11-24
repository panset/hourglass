// SPDX-License-Identifier: MIT
import { api } from "../../api/client";
import type { components } from "../../api/schema.gen";

type S = components["schemas"];

export type ServiceAccount = S["ServiceAccount"];
export type ServiceAccountList = S["ServiceAccountList"];
export type ServiceToken = S["ServiceToken"];
export type IssuedServiceToken = S["IssuedServiceToken"];
export type ServiceScope = S["ServiceScope"];

/** The five scopes, with what each lets the service do (SVC-3). */
export const SCOPES: { scope: ServiceScope; hint: string }[] = [
  { scope: "state:read", hint: "read the state of every setup and hold" },
  { scope: "pressure:write", hint: "report lab pressure" },
  { scope: "deployments:write", hint: "report which setups are deployed" },
  { scope: "holds:release", hint: "release any hold" },
  { scope: "weights:write", hint: "report a weight per setup" },
];

const base = "/api/v1/admin/service-accounts";

export const listServiceAccounts = () => api.get<ServiceAccountList>(base);
export const createServiceAccount = (body: S["CreateServiceAccountRequest"]) => api.post<ServiceAccount>(base, body);
export const updateServiceAccount = (id: number, body: S["UpdateServiceAccountRequest"]) =>
  api.put<ServiceAccount>(`${base}/${id}`, body);
export const deleteServiceAccount = (id: number) => api.delete<void>(`${base}/${id}`);
export const issueServiceToken = (id: number, body: S["IssueServiceTokenRequest"]) =>
  api.post<IssuedServiceToken>(`${base}/${id}/tokens`, body);
export const revokeServiceToken = (id: number, tokenId: number) => api.delete<void>(`${base}/${id}/tokens/${tokenId}`);
