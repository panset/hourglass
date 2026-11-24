// SPDX-License-Identifier: MIT
// Text-field state for the settings form and its conversion to the API body. Parsing only: the server validates.
import type { RuleInput, RulesResponse, Settings } from "./api";

export interface BandRow {
  from: string;
  multiplier: string;
}

export interface SettingsForm {
  defaultWeight: string;
  labelKeys: string;
  baseMultiplier: string;
  bands: BandRow[];
  allowance: string;
  maxHolds: string;
  teardown: boolean;
  poll: string;
  enforcement: boolean;
  /** The "Reported weights" section (D1). Sent only when one of these four was changed. */
  rwEnabled: boolean;
  rwMin: string;
  rwMax: string;
  rwMaxAge: string;
}

/** Defaults of the setting (D1) for a server that omits the key. */
const RW_DEFAULT = { enabled: false, min: 0, max: null, max_age: "10m" } as const;

/** The four reported-weights fields of a form, for comparing against the saved ones. */
export const reportedFields = (f: SettingsForm) => ({
  enabled: f.rwEnabled,
  min: f.rwMin.trim(),
  max: f.rwMax.trim(),
  max_age: f.rwMaxAge.trim(),
});

export function toForm(s: Settings): SettingsForm {
  const rw = s.reported_weights ?? RW_DEFAULT;
  return {
    rwEnabled: rw.enabled,
    rwMin: String(rw.min),
    rwMax: rw.max === null ? "" : String(rw.max),
    rwMaxAge: rw.max_age,
    defaultWeight: String(s.default_weight),
    labelKeys: s.display_label_keys.join(", "),
    baseMultiplier: String(s.bands.base),
    bands: s.bands.steps.map((b) => ({ from: String(b.from), multiplier: String(b.multiplier) })),
    allowance: String(s.default_allowance),
    maxHolds: String(s.max_holds_per_user),
    teardown: s.release_requires_teardown,
    poll: s.poll_interval,
    enforcement: s.enforcement,
  };
}

export type FieldErrors = Record<string, string>;

function num(text: string): number {
  return text.trim() === "" ? NaN : Number(text);
}

/** Parses the text fields. Returns field errors for anything that is not a number. */
export function toSettings(
  f: SettingsForm,
  includeReported = false,
): { ok: true; value: Settings } | { ok: false; errors: FieldErrors } {
  const errors: FieldErrors = {};
  const need = (key: string, text: string): number => {
    const n = num(text);
    if (!Number.isFinite(n)) errors[key] = "Enter a number.";
    return n;
  };
  const default_weight = need("default_weight", f.defaultWeight);
  const base = need("bands.base", f.baseMultiplier);
  const default_allowance = need("default_allowance", f.allowance);
  const max_holds_per_user = need("max_holds_per_user", f.maxHolds);
  const steps = f.bands.map((b, i) => ({
    from: need(`bands.steps[${i}].from`, b.from),
    multiplier: need(`bands.steps[${i}].multiplier`, b.multiplier),
  }));
  // Blank max means "no maximum" (null); the server decides whether that is allowed with the switch on.
  const rwMin = includeReported ? need("reported_weights.min", f.rwMin) : 0;
  const rwMax = !includeReported || f.rwMax.trim() === "" ? null : need("reported_weights.max", f.rwMax);
  if (Object.keys(errors).length > 0) return { ok: false, errors };
  return {
    ok: true,
    value: {
      ...(includeReported
        ? { reported_weights: { enabled: f.rwEnabled, min: rwMin, max: rwMax, max_age: f.rwMaxAge.trim() } }
        : {}),
      default_weight,
      display_label_keys: f.labelKeys
        .split(",")
        .map((k) => k.trim())
        .filter((k) => k !== ""),
      bands: { base, steps },
      default_allowance,
      max_holds_per_user,
      release_requires_teardown: f.teardown,
      poll_interval: f.poll.trim(),
      enforcement: f.enforcement,
    },
  };
}

export interface RuleRow {
  /** Stable React key: `r<id>` for saved rules, `n<counter>` for new ones. */
  key: string;
  id?: number;
  selector: string;
  weight: string;
  /** Match count as last saved; absent for new rules. */
  matchCount?: number;
}

export function toRuleRows(r: RulesResponse): RuleRow[] {
  return r.rules.map((x) => ({
    key: `r${x.id}`,
    id: x.id,
    selector: x.selector,
    weight: String(x.weight),
    matchCount: x.match_count,
  }));
}

export function toRuleInputs(rows: RuleRow[]): { ok: true; value: RuleInput[] } | { ok: false; errors: FieldErrors } {
  const errors: FieldErrors = {};
  const value = rows.map((r, i) => {
    const w = num(r.weight);
    if (!Number.isFinite(w)) errors[`rules[${i}].weight`] = "Enter a number.";
    return { selector: r.selector.trim(), weight: w };
  });
  return Object.keys(errors).length > 0 ? { ok: false, errors } : { ok: true, value };
}

export function sameJson(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}
