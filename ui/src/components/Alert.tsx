// SPDX-License-Identifier: MIT
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type HTMLAttributes, type ReactNode } from "react";
import styles from "./Alert.module.css";

export type AlertTone = "info" | "error" | "success" | "warning";

export interface AlertProps extends HTMLAttributes<HTMLDivElement> {
  tone?: AlertTone;
}

/** Inline message. Errors use role="alert" (announced at once); others role="status". */
export function Alert({ tone = "info", className, ...rest }: AlertProps) {
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      className={[styles.alert, tone !== "info" && styles[tone], className].filter(Boolean).join(" ")}
      {...rest}
    />
  );
}

interface ToastItem {
  id: number;
  message: ReactNode;
  tone: AlertTone;
}

interface ToastApi {
  push: (message: ReactNode, tone?: AlertTone) => void;
}

const ToastContext = createContext<ToastApi | null>(null);

/** How long a toast stays, in milliseconds. */
export const TOAST_MS = 6000;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const dismiss = useCallback((id: number) => setItems((xs) => xs.filter((x) => x.id !== id)), []);
  const api = useMemo<ToastApi>(
    () => ({
      push: (message, tone = "info") => setItems((xs) => [...xs, { id: nextId++, message, tone }]),
    }),
    [],
  );
  return (
    <ToastContext.Provider value={api}>
      {children}
      <div className={styles.toasts} aria-live="polite">
        {items.map((t) => (
          <ToastView key={t.id} item={t} onDismiss={dismiss} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

let nextId = 1;

function ToastView({ item, onDismiss }: { item: ToastItem; onDismiss: (id: number) => void }) {
  useEffect(() => {
    const t = setTimeout(() => onDismiss(item.id), TOAST_MS);
    return () => clearTimeout(t);
  }, [item.id, onDismiss]);
  return (
    <Alert tone={item.tone} className={styles.toast}>
      <span>{item.message}</span>
      <button type="button" aria-label="Dismiss" onClick={() => onDismiss(item.id)}>
        {"×"}
      </button>
    </Alert>
  );
}

/** `useToast().push("Saved", "success")`. Needs a ToastProvider above it (the app has one). */
export function useToast(): ToastApi {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast needs a <ToastProvider>");
  return ctx;
}
