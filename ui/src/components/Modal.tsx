// SPDX-License-Identifier: MIT
import { useEffect, useId, useRef, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Alert } from "./Alert";
import { Button } from "./Button";
import styles from "./Modal.module.css";

import { FOCUSABLE } from "../hooks/usePanelFocus";

export interface ModalProps {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  /** Buttons row. */
  footer?: ReactNode;
  /**
   * Where focus goes on close when the element that opened the dialog is no longer in the page
   * (its row was filtered out or removed). Give it `tabIndex={-1}` if it is not focusable.
   * Without it focus returns to the opener only, as before.
   */
  fallbackFocus?: RefObject<HTMLElement>;
}

/**
 * Accessible modal dialog: role="dialog" + aria-modal, labelled by the title,
 * focus moves in on open, Tab stays inside, Escape closes, focus returns to the
 * element that opened it, or to `fallbackFocus` when that element is gone.
 */
export function Modal({ open, title, onClose, children, footer, fallbackFocus }: ModalProps) {
  const titleId = useId();
  const ref = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const fallbackRef = useRef(fallbackFocus);
  fallbackRef.current = fallbackFocus;

  useEffect(() => {
    if (!open) return;
    const opener = document.activeElement as HTMLElement | null;
    const node = ref.current;
    // A field in the body first, then the element marked data-autofocus (the safe button of a
    // confirmation, so a stray Enter does not confirm), then the first focusable element.
    const first =
      bodyRef.current?.querySelector<HTMLElement>(FOCUSABLE) ??
      node?.querySelector<HTMLElement>("[data-autofocus]") ??
      node?.querySelector<HTMLElement>(FOCUSABLE);
    (first ?? node)?.focus();

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onCloseRef.current();
        return;
      }
      if (e.key !== "Tab" || !node) return;
      const items = Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE));
      if (items.length === 0) {
        e.preventDefault();
        return;
      }
      const firstEl = items[0]!;
      const lastEl = items[items.length - 1]!;
      const active = document.activeElement as HTMLElement | null;
      // Shift+Tab from the first item, the dialog itself, or an element outside the Tab order
      // (a heading with tabIndex -1 that took focus on open) wraps to the last item.
      if (e.shiftKey && (active === firstEl || active === node || !items.includes(active as HTMLElement))) {
        e.preventDefault();
        lastEl.focus();
      } else if (!e.shiftKey && active === lastEl) {
        e.preventDefault();
        firstEl.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      if (opener?.isConnected) opener.focus?.();
      else fallbackRef.current?.current?.focus();
    };
  }, [open]);

  if (!open) return null;
  return createPortal(
    <div className={styles.backdrop}>
      <div ref={ref} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1} className={styles.dialog}>
        <h2 id={titleId}>{title}</h2>
        <div ref={bodyRef}>{children}</div>
        {footer ? <div className={styles.footer}>{footer}</div> : null}
      </div>
    </div>,
    document.body,
  );
}

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  children: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  /** Red confirm button for destructive actions. */
  destructive?: boolean;
  /** Disables both buttons while the request runs. */
  busy?: boolean;
  /** Error from the last attempt, shown inside the dialog. */
  error?: string;
  onConfirm: () => void;
  onCancel: () => void;
}

/** Modal with Cancel and Confirm. The caller owns the request and decides when to close. */
export function ConfirmDialog({
  open,
  title,
  children,
  confirmLabel,
  cancelLabel = "Cancel",
  destructive,
  busy,
  error,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  return (
    <Modal
      open={open}
      title={title}
      onClose={onCancel}
      footer={
        <>
          <Button variant={destructive ? "destructive" : "primary"} disabled={busy} onClick={onConfirm}>
            {confirmLabel}
          </Button>
          <Button disabled={busy} onClick={onCancel} data-autofocus="">
            {cancelLabel}
          </Button>
        </>
      }
    >
      {children}
      {error ? <Alert tone="error">{error}</Alert> : null}
    </Modal>
  );
}
