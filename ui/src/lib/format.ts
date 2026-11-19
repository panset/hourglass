// SPDX-License-Identifier: MIT
// Display formatting only. Nothing here computes a balance, cost or projection (UI-1).

const MINUS = "−";

export interface FormatTokensOptions {
  /** Fixed number of decimals (e.g. 1 gives "142.0"). Default: trim trailing zeros up to 3. */
  fixed?: number;
  /** Prefix "+" for positive amounts (ledger style). Negatives always use a true minus sign. */
  signed?: boolean;
}

/** Token amounts as the API sends them (whole tokens, up to three decimals). */
export function formatTokens(value: number | null | undefined, opts: FormatTokensOptions = {}): string {
  if (value === null || value === undefined || Number.isNaN(value)) return "—";
  const digits = opts.fixed;
  const nf = new Intl.NumberFormat("en-US", {
    minimumFractionDigits: digits ?? 0,
    maximumFractionDigits: digits ?? 3,
  });
  const abs = nf.format(Math.abs(value));
  const isNegative = value < 0 && Number(abs.replace(/,/g, "")) !== 0;
  if (isNegative) return MINUS + abs;
  return opts.signed && value > 0 ? "+" + abs : abs;
}

/** "2 d 4 h", "1 h 20 m", "40 min", "<1 min". Input is whole seconds. */
export function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.floor(totalSeconds));
  const days = Math.floor(s / 86400);
  const hours = Math.floor((s % 86400) / 3600);
  const minutes = Math.floor((s % 3600) / 60);
  if (days > 0) return hours > 0 ? `${days} d ${hours} h` : `${days} d`;
  if (hours > 0) return minutes > 0 ? `${hours} h ${minutes} m` : `${hours} h`;
  if (minutes > 0) return `${minutes} min`;
  return "<1 min";
}

export interface FormatDateOptions {
  /** IANA zone name. Default: the browser's zone. Tests pass "UTC". */
  timeZone?: string;
}

function parts(iso: string, opts: Intl.DateTimeFormatOptions, tz?: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat("en-US", { ...opts, timeZone: tz }).format(d);
}

/** "Nov 18, 17:30" */
export function formatDateTime(iso: string, o: FormatDateOptions = {}): string {
  return parts(iso, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", hourCycle: "h23" }, o.timeZone);
}

/** "Nov 18" */
export function formatDate(iso: string, o: FormatDateOptions = {}): string {
  return parts(iso, { month: "short", day: "numeric" }, o.timeZone);
}

/** "2026-11" -> "November 2026" */
export function formatPeriod(period: string): string {
  const m = /^(\d{4})-(0[1-9]|1[0-2])$/.exec(period);
  if (!m) return period;
  return parts(`${m[1]}-${m[2]}-15T12:00:00Z`, { month: "long", year: "numeric" }, "UTC");
}

/** 0.82 -> "82%" */
export function formatPercent(fraction: number): string {
  return `${Math.round(fraction * 100)}%`;
}

/** 2 -> "2×" */
export function formatMultiplier(multiplier: number): string {
  return `${new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 }).format(multiplier)}×`;
}

/** 2 -> "2.0 / h" */
export function formatPerHour(tokensPerHour: number): string {
  const nf = new Intl.NumberFormat("en-US", { minimumFractionDigits: 1, maximumFractionDigits: 3 });
  return `${nf.format(tokensPerHour)} / h`;
}
