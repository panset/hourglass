// SPDX-License-Identifier: MIT
import {
  formatDay,
  formatUtc,
  localZoneAbbrev,
  formatDate,
  formatDateTime,
  formatDuration,
  formatMultiplier,
  formatPercent,
  formatPerHour,
  formatPeriod,
  formatTokens,
} from "./format";

describe("formatTokens", () => {
  it("trims trailing zeros and groups thousands", () => {
    expect(formatTokens(142)).toBe("142");
    expect(formatTokens(142.2)).toBe("142.2");
    expect(formatTokens(3410)).toBe("3,410");
    expect(formatTokens(0.125)).toBe("0.125");
  });
  it("supports fixed decimals and signs with a true minus", () => {
    expect(formatTokens(142, { fixed: 1 })).toBe("142.0");
    expect(formatTokens(-0.17, { fixed: 2, signed: true })).toBe("−0.17");
    expect(formatTokens(40, { fixed: 2, signed: true })).toBe("+40.00");
    expect(formatTokens(-12)).toBe("−12");
  });
  it("never shows negative zero and handles null", () => {
    expect(formatTokens(-0.0001, { fixed: 2 })).toBe("0.00");
    expect(formatTokens(null)).toBe("—");
  });
});

describe("formatDuration", () => {
  it.each([
    [0, "<1 min"],
    [40 * 60, "40 min"],
    [3600, "1 h"],
    [80 * 60, "1 h 20 m"],
    [15 * 3600, "15 h"],
    [2 * 86400 + 4 * 3600, "2 d 4 h"],
    [86400, "1 d"],
  ])("%s s -> %s", (s, text) => expect(formatDuration(s)).toBe(text));
});

describe("dates", () => {
  it("formats in the given zone", () => {
    expect(formatDateTime("2026-11-18T17:30:00Z", { timeZone: "UTC" })).toBe("Nov 18, 17:30");
    expect(formatDate("2026-11-18T17:30:00Z", { timeZone: "UTC" })).toBe("Nov 18");
    expect(formatDateTime("2026-11-18T17:30:00Z", { timeZone: "America/Los_Angeles" })).toBe("Nov 18, 09:30");
  });
  it("shows a dash for a bad date", () => expect(formatDate("nope")).toBe("—"));
  it("formats a period", () => {
    expect(formatPeriod("2026-11")).toBe("November 2026");
    expect(formatPeriod("bad")).toBe("bad");
  });
});

describe("pressure and rate", () => {
  it("formats percent, multiplier and per hour", () => {
    expect(formatPercent(0.82)).toBe("82%");
    expect(formatMultiplier(2)).toBe("2×");
    expect(formatMultiplier(1.5)).toBe("1.5×");
    expect(formatPerHour(2)).toBe("2.0 / h");
    expect(formatPerHour(0.125)).toBe("0.125 / h");
  });
});

describe("zones and day labels", () => {
  it("formatUtc gives the UTC instant", () => {
    expect(formatUtc("2026-11-01T00:00:00Z")).toBe("2026-11-01 00:00 UTC");
  });
  it("localZoneAbbrev names the zone", () => {
    expect(localZoneAbbrev("UTC")).toBe("UTC");
  });
  it("formatDay uses the API date unchanged", () => {
    expect(formatDay("2026-11-01")).toBe("Nov 1");
  });
});
