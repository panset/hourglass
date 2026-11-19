// SPDX-License-Identifier: MIT
import { useEffect, useId, useRef, type ReactNode } from "react";
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
}

/**
 * Accessible modal dialog: role="dialog" + aria-modal, labelled by the title,
 * focus moves in on open, Tab stays inside, Escape closes, focus returns to the
 * element that opened it.
 */
export function Modal({ open, title, onClose, children, footer }: ModalProps) {
  const titleId = useId();
  const ref = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

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
      const active = document.activeElement;
      if (e.shiftKey && (active === firstEl || active === node)) {
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
      opener?.focus?.();
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
