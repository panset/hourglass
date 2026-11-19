// SPDX-License-Identifier: MIT
import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from "react";
import styles from "./Field.module.css";

export interface FieldControlProps {
  id: string;
  "aria-describedby"?: string;
  "aria-invalid"?: true;
}

export interface FieldProps {
  label: string;
  hint?: ReactNode;
  /** Error text; also marks the control invalid for assistive tech. */
  error?: string;
  /** Receives the props to spread on the control: `{(p) => <Input {...p} />}`. */
  children: (props: FieldControlProps) => ReactNode;
}

/** Label, control, hint and error message wired together with ids. */
export function Field({ label, hint, error, children }: FieldProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [errorId, hintId].filter(Boolean).join(" ") || undefined;
  return (
    <div className={styles.field}>
      <label className={styles.label} htmlFor={id}>
        {label}
      </label>
      {children({ id, "aria-describedby": describedBy, "aria-invalid": error ? true : undefined })}
      {hint ? (
        <div className={styles.hint} id={hintId}>
          {hint}
        </div>
      ) : null}
      {error ? (
        <div className={styles.error} id={errorId}>
          {error}
        </div>
      ) : null}
    </div>
  );
}

export function Input({ className, ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={[styles.control, className].filter(Boolean).join(" ")} {...rest} />;
}

export function Select({ className, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={[styles.control, className].filter(Boolean).join(" ")} {...rest} />;
}
