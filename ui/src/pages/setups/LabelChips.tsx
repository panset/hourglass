// SPDX-License-Identifier: MIT
// "Available labels" under the Label selector box (UI-9, design B2): every key of `label_index` with its values as chips.
import { useState } from "react";
import type { SetupList } from "../usage/holdApi";
import styles from "./setups.module.css";

export type LabelIndex = SetupList["label_index"];

/** Design B2: a key shows its first 12 values; with more than 8 keys the block starts collapsed. */
export const VALUES_SHOWN = 12;
export const KEYS_BEFORE_COLLAPSE = 8;

/** The placeholder of the selector box: the first term the API gave, or undefined when there is none. */
export function placeholderFrom(index: LabelIndex): string | undefined {
  for (const k of index) for (const v of k.values) if (v.selector) return `e.g. ${v.selector}`;
  return undefined;
}

export function LabelChips({ index, applied, onToggle }: { index: LabelIndex; applied: string[]; onToggle: (key: string, term: string) => void }) {
  const [open, setOpen] = useState(false);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const collapsed = index.length > KEYS_BEFORE_COLLAPSE && !open;
  const hasUnusable = index.some((k) => k.values.some((v) => v.selector === null));

  return (
    <section className={styles.labels} aria-labelledby="available-labels-h">
      <div className={styles.labelsHead}>
        <h2 id="available-labels-h" className={styles.labelsTitle}>Available labels</h2>
        {index.length > KEYS_BEFORE_COLLAPSE ? (
          <button type="button" className={styles.linkButton} aria-expanded={open} aria-controls="available-labels-body" onClick={() => setOpen(!open)}>
            {open ? "Hide labels" : "Show labels"}
          </button>
        ) : null}
      </div>
      <div id="available-labels-body" hidden={collapsed}>
        {index.length === 0 ? <p className={styles.muted}>No setup has labels.</p> : null}
        {index.map((k) => {
          const all = expanded[k.key];
          const shown = all ? k.values : k.values.slice(0, VALUES_SHOWN);
          const more = k.values.length - shown.length;
          return (
            <div key={k.key} role="group" aria-label={`Label ${k.key}`} className={styles.labelKey}>
              <span className={styles.labelKeyName}>{k.key}</span>
              <span className={styles.chipList}>
                {shown.map((v) => {
                  const text = v.value === "" ? "(empty)" : v.value;
                  if (v.selector === null) {
                    // Focusable and described, not `disabled`, so keyboard and screen reader users can reach the explanation.
                    return (
                      <button
                        key={v.value}
                        type="button"
                        className={`${styles.chip} ${styles.chipUnusable}`}
                        aria-disabled="true"
                        aria-label={`${k.key}=${v.value} cannot be used in a selector`}
                        title="This value contains spaces or one of , ( ) = ! so it cannot be used in a selector."
                        onClick={(e) => e.preventDefault()}
                      >
                        {text}
                      </button>
                    );
                  }
                  const term = v.selector;
                  const selected = applied.includes(term);
                  return (
                    <button
                      key={v.value}
                      type="button"
                      className={`${styles.chip} ${selected ? styles.chipSelected : ""}`}
                      aria-pressed={selected}
                      aria-label={`Filter by ${term}`}
                      onClick={() => onToggle(k.key, term)}
                    >
                      {text}
                    </button>
                  );
                })}
                {more > 0 ? (
                  <button type="button" className={styles.linkButton} onClick={() => setExpanded({ ...expanded, [k.key]: true })}>
                    +{more} more
                  </button>
                ) : null}
                {all && k.values.length > VALUES_SHOWN ? (
                  <button type="button" className={styles.linkButton} onClick={() => setExpanded({ ...expanded, [k.key]: false })}>
                    Show fewer
                  </button>
                ) : null}
              </span>
            </div>
          );
        })}
        {hasUnusable ? <p className={styles.muted}>Struck-through values contain spaces or one of , ( ) = ! and cannot be used in a selector.</p> : null}
      </div>
    </section>
  );
}
