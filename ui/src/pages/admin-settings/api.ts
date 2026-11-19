// SPDX-License-Identifier: MIT
import { api } from "../../api/client";
import type { components } from "../../api/schema.gen";

type S = components["schemas"];

export type Settings = S["Settings"];
export type SettingsResponse = S["SettingsResponse"];
export type RulesResponse = S["RulesResponse"];
export type RuleInput = S["RuleInput"];
export type Collector = S["Collector"];

export const getSettings = () => api.get<SettingsResponse>("/api/v1/admin/settings");
export const putSettings = (body: Settings) => api.put<SettingsResponse>("/api/v1/admin/settings", body);
export const getRules = () => api.get<RulesResponse>("/api/v1/admin/rules");
export const putRules = (rules: RuleInput[]) =>
  api.put<RulesResponse>("/api/v1/admin/rules", { rules } satisfies S["RulesInput"]);
export const getCollector = () => api.get<Collector>("/api/v1/admin/collector");
