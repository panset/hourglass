// SPDX-License-Identifier: MIT
import type { HTMLAttributes } from "react";
import styles from "./Badge.module.css";

export type BadgeTone = "blue" | "green" | "grey" | "amber";

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  /** blue: Deployed; green: Free/Enabled; grey: Acquired/Disabled; amber: pressure. */
  tone?: BadgeTone;
}

/** Pill-shaped status badge. Colour is never the only signal: always pass text. */
export function Badge({ tone = "grey", className, ...rest }: BadgeProps) {
  return <span className={[styles.badge, styles[tone], className].filter(Boolean).join(" ")} {...rest} />;
}

/** Monospace `key=value` label chip. */
export function Chip({ className, ...rest }: HTMLAttributes<HTMLElement>) {
  return <code className={[styles.chip, className].filter(Boolean).join(" ")} {...rest} />;
}
