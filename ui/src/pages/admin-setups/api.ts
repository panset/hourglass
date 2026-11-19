// SPDX-License-Identifier: MIT
import { api } from "../../api/client";
import type { components } from "../../api/schema.gen";

type S = components["schemas"];

export type Setup = S["Setup"];
export type SetupList = S["SetupList"];
export type SetupInput = S["SetupInput"];
export type Collector = S["Collector"];
export type Orphan = S["Orphan"];

export const listSetups = () => api.get<SetupList>("/api/v1/setups");
export const getCollector = () => api.get<Collector>("/api/v1/admin/collector");
export const createSetup = (body: SetupInput) => api.post<Setup>("/api/v1/admin/setups", body);
export const updateSetup = (id: number, body: SetupInput) => api.put<Setup>(`/api/v1/admin/setups/${id}`, body);
export const deleteSetup = (id: number) => api.delete<void>(`/api/v1/admin/setups/${id}`);
export const forceRelease = (holdId: number, reason: string) =>
  api.post<unknown>(`/api/v1/holds/${holdId}/release`, { force: true, reason } satisfies S["ReleaseRequest"]);
