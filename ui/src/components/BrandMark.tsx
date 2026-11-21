// SPDX-License-Identifier: MIT
import styles from "./BrandMark.module.css";

/**
 * The Hourglass mark (the "rack stack": seven bars on a 32-unit grid), drawn in the
 * surrounding text colour and sized to the surrounding font. Decorative: it always sits
 * next to the product name, so it is hidden from assistive technology.
 * The same shape is ui/public/favicon.svg.
 */
export function BrandMark() {
  return (
    <svg
      className={styles.mark}
      data-testid="brand-mark"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 32 32"
      fill="currentColor"
      aria-hidden="true"
      focusable="false"
    >
      <rect x="4" y="3" width="24" height="2" rx="0.4" />
      <rect x="12" y="7" width="8" height="2" rx="0.4" />
      <rect x="14" y="11" width="4" height="2" rx="0.4" />
      <rect x="15" y="15" width="2" height="2" rx="0.4" />
      <rect x="11" y="19" width="10" height="2" rx="0.4" />
      <rect x="7" y="23" width="18" height="2" rx="0.4" />
      <rect x="4" y="27" width="24" height="2" rx="0.4" />
    </svg>
  );
}
