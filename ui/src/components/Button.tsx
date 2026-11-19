// SPDX-License-Identifier: MIT
import type { AnchorHTMLAttributes, ButtonHTMLAttributes } from "react";
import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "destructive";

interface CommonProps {
  variant?: ButtonVariant;
  /** Stretch to the container width. */
  block?: boolean;
}

function classes({ variant = "secondary", block }: CommonProps, extra?: string): string {
  return [styles.button, variant !== "secondary" && styles[variant], block && styles.block, extra]
    .filter(Boolean)
    .join(" ");
}

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & CommonProps;

/** A real <button>; type defaults to "button" so it never submits a form by accident. */
export function Button({ variant, block, className, type = "button", ...rest }: ButtonProps) {
  return <button type={type} className={classes({ variant, block }, className)} {...rest} />;
}

export type LinkButtonProps = AnchorHTMLAttributes<HTMLAnchorElement> & CommonProps;

/** A plain link (full page navigation, e.g. /auth/oidc/start) that looks like a button. */
export function LinkButton({ variant, block, className, ...rest }: LinkButtonProps) {
  return <a className={classes({ variant, block }, className)} {...rest} />;
}
