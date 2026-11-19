// SPDX-License-Identifier: MIT
import type { ReactNode } from "react";
import styles from "./Table.module.css";

export interface Column<T> {
  key: string;
  /** Header text. Leave "" for an action column; it is then announced as `actionLabel`. */
  header: string;
  render: (row: T) => ReactNode;
  align?: "left" | "right";
}

export interface TableProps<T> {
  /** Accessible name of the table (every table has one). */
  label: string;
  columns: Column<T>[];
  rows: readonly T[];
  rowKey: (row: T) => string | number;
  /** Shown instead of rows when there are none. */
  emptyMessage?: ReactNode;
  /** Screen-reader name for blank-header columns. */
  actionLabel?: string;
}

/** Data table inside a horizontal scroller; shows an empty state when `rows` is empty. */
export function Table<T>({
  label,
  columns,
  rows,
  rowKey,
  emptyMessage = "Nothing to show.",
  actionLabel = "Actions",
}: TableProps<T>) {
  return (
    <div className={styles.scroller}>
      <table className={styles.table} aria-label={label}>
        <thead>
          <tr>
            {columns.map((c) => (
              <th key={c.key} scope="col" className={c.align === "right" ? styles.right : undefined}>
                {c.header === "" ? <span className="sr-only">{actionLabel}</span> : c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td className={styles.empty} colSpan={columns.length}>
                {emptyMessage}
              </td>
            </tr>
          ) : (
            rows.map((row) => (
              <tr key={rowKey(row)}>
                {columns.map((c) => (
                  <td key={c.key} className={c.align === "right" ? styles.right : undefined}>
                    {c.render(row)}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
