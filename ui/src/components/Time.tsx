// SPDX-License-Identifier: MIT
import { formatDateTime, formatUtc, localZoneAbbrev } from "../lib/format";

/** The browser's zone abbreviation ("PST"), for a column header: times below are shown in it. */
export function zoneLabel(): string {
  return localZoneAbbrev();
}

/** A timestamp from the API in the browser's local zone; the tooltip gives the UTC value. */
export function Time({ iso }: { iso: string }) {
  return (
    <time dateTime={iso} title={formatUtc(iso)}>
      {formatDateTime(iso)}
    </time>
  );
}
