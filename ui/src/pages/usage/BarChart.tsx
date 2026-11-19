// SPDX-License-Identifier: MIT
// Hand-drawn SVG bars for tokens per day (no chart library). Heights are scaled to the API's numbers; nothing is derived.
import { formatDay, formatTokens } from "../../lib/format";
import type { UsageResponse } from "./holdApi";

const W = 600;
const H = 150;
const GAP = 4;

export function BarChart({ days, label, highlightLast }: { days: UsageResponse["days"]; label: string; highlightLast: boolean }) {
  const max = Math.max(...days.map((d) => d.debited_tokens), 0);
  const n = days.length;
  const barW = n > 0 ? (W - GAP * (n - 1)) / n : 0;
  return (
    <svg viewBox={`0 0 ${W} ${H + 2}`} width="100%" role="img" aria-label={label} preserveAspectRatio="none">
      {days.map((d, i) => {
        const h = max > 0 && d.debited_tokens > 0 ? Math.max(2, (d.debited_tokens / max) * H) : 2;
        const today = highlightLast && i === n - 1;
        return (
          <rect
            key={d.date}
            data-testid="usage-bar"
            data-today={today ? "true" : undefined}
            x={i * (barW + GAP)}
            y={H - h}
            width={barW}
            height={h}
            fill={today ? "#1D4ED8" : "#9CA3AF"}
          >
            <title>{`${formatDay(d.date)}: ${formatTokens(d.debited_tokens)} tokens`}</title>
          </rect>
        );
      })}
      <line x1={0} x2={W} y1={H + 1} y2={H + 1} stroke="#BDBDBD" strokeWidth={1} />
    </svg>
  );
}
