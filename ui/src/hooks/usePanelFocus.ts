// SPDX-License-Identifier: MIT
import { useEffect, useRef, useState } from "react";

export const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * Focus management for an inline side panel (<aside>): focus moves to its first field when it
 * opens, Tab stays inside while it is open, Escape calls `onClose`, and when the panel goes away
 * focus returns to the control that opened it. Dialogs opened from inside the panel are portalled
 * to <body>, so their keys never reach the panel's own listener.
 */
export function usePanelFocus<T extends HTMLElement>(onClose: () => void) {
  const ref = useRef<T>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  // Captured during the first render, before React moves focus, so swapping the panel from one row to
  // another still remembers the button that was actually pressed.
  const [opener] = useState(() => document.activeElement as HTMLElement | null);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    const fields = node.querySelectorAll<HTMLElement>("input:not([disabled]), select:not([disabled]), textarea:not([disabled])");
    (fields[0] ?? node.querySelector<HTMLElement>(FOCUSABLE) ?? node).focus();

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key !== "Tab") return;
      const items = Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE));
      if (items.length === 0) {
        e.preventDefault();
        return;
      }
      const first = items[0]!;
      const last = items[items.length - 1]!;
      const active = document.activeElement;
      if (e.shiftKey && (active === first || active === node)) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    node.addEventListener("keydown", onKey);
    return () => {
      node.removeEventListener("keydown", onKey);
      if (opener?.isConnected) opener.focus();
    };
  }, [opener]);

  return ref;
}
