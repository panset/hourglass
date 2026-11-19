// SPDX-License-Identifier: MIT
// Segmented band bar with a marker at the current pressure. Bands and pressure come from GET /api/v1/pressure.
import type { Pressure } from "../../api/types";
import styles from "./usage.module.css";

const COLORS = ["#FFFFFF", "#E5E7EB", "#9CA3AF", "#4B5563"];
const pct = (f: number) => Math.round(f * 100);

interface Segment {
  from: number;
  to: number;
  multiplier: number;
}

function segments(bands: Pressure["bands"]): Segment[] {
  const steps = bands?.steps ?? [];
  if (steps.length === 0) return [{ from: 0, to: 1, multiplier: bands?.base ?? 0 }];
  const out: Segment[] = [{ from: 0, to: steps[0]!.from, multiplier: bands?.base ?? 0 }];
  steps.forEach((s, i) => out.push({ from: s.from, to: steps[i + 1]?.from ?? 1, multiplier: s.multiplier }));
  return out;
}

export function PressureBar({ pressure }: { pressure: Pressure }) {
  const segs = segments(pressure.bands);
  const name = (m: number) => (m === 0 ? "free" : `${m}x`);
  const text = segs
    .map((s, i) =>
      segs.length === 1
        ? `${name(s.multiplier)} throughout`
        : i === 0
          ? `${name(s.multiplier)} below ${pct(s.to)}`
          : i === segs.length - 1
            ? `${name(s.multiplier)} above ${pct(s.from)}`
            : `${name(s.multiplier)} to ${pct(s.to)}`,
    )
    .join(", ");
  return (
    <div
      className={styles.bar}
      role="img"
      aria-label={`Pressure bands: ${text}; currently ${pct(pressure.smoothed)} percent`}
    >
      {segs.map((s, i) => (
        <div
          key={i}
          style={{ width: `${(s.to - s.from) * 100}%`, background: COLORS[Math.min(i, COLORS.length - 1)] }}
        />
      ))}
      <div className={styles.marker} style={{ left: `${pressure.smoothed * 100}%` }} />
    </div>
  );
}
