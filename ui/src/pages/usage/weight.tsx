// SPDX-License-Identifier: MIT
// Display helpers for a weight and where it comes from (UI-12, UI-13, UI-14). Nothing here computes a weight:
// every value is an API field and is only formatted (UI-1).
import { Badge } from "../../components";
import { formatTokens } from "../../lib/format";

/** The sources a tag is shown for on a price cell. `policy` is shown on holds only. */
const TAGGED = new Set(["reported", "override", "policy"]);

/** A small tag with the source word when the source is `reported`, `override` or `policy`; nothing for `rule`, `default` or null. */
export function WeightTag({ source }: { source: string | null | undefined }) {
  if (!source || !TAGGED.has(source)) return null;
  return (
    <>
      {" "}
      <Badge tone={source === "reported" ? "blue" : "grey"} title={`Weight source: ${source}`}>
        {source}
      </Badge>
    </>
  );
}

/** `memory-gb 512 · storage-gb 4000 · tier gold`, or null when there are none. Not a column and not a filter (WGT-16). */
export function attributesLine(attrs: Record<string, string> | null | undefined): string | null {
  const parts = Object.entries(attrs ?? {}).map(([k, v]) => `${k} ${v}`);
  return parts.length > 0 ? parts.join(" · ") : null;
}

const range = (v: number) => formatTokens(v, { fixed: 2 });

/**
 * A ledger debit's weight after the multiplier: one value when min equals max, `1.15 to 1.40` otherwise, with the
 * source words when they are not only `rule` or `default`, and nothing (null) when the weight is unknown.
 */
export function ledgerWeight(
  min: number | null | undefined,
  max: number | null | undefined,
  sources: readonly string[] | null | undefined,
): string | null {
  if (min === null || min === undefined || max === null || max === undefined) return null;
  const text = min === max ? formatTokens(min) : `${range(min)} to ${range(max)}`;
  const shown = (sources ?? []).filter((s) => s !== "rule" && s !== "default");
  return shown.length > 0 ? `weight ${text} (${shown.join(", ")})` : `weight ${text}`;
}

/** Why a reported weight is not applied, in words (UI-14). The API sends the reason; the page only words it. */
export function notAppliedText(reason: string | null | undefined): string | null {
  switch (reason) {
    case "expired":
      return "Not applied: the report is older than the maximum age.";
    case "out_of_bounds":
      return "Not applied: the weight is outside the minimum and maximum.";
    case "override":
      return "Not applied: the weight override takes its place.";
    default:
      return null;
  }
}
