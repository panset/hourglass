// SPDX-License-Identifier: MIT
import { Component, type ErrorInfo, type ReactNode } from "react";
import { Alert } from "./Alert";
import { Button } from "./Button";

export interface ErrorBoundaryProps {
  children: ReactNode;
  /** Defaults to reloading the page, which also fetches fresh chunk names after a redeploy. */
  onReload?: () => void;
}

interface State {
  failed: boolean;
}

/**
 * Catches render errors and failed lazy-chunk loads below it, so the user sees a message with a
 * Reload button instead of a blank page. Remount it (change its `key`) to clear the failure,
 * for example on navigation.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, State> {
  state: State = { failed: false };

  static getDerivedStateFromError(): State {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("UI error", error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return (
      <div className="page">
        <h1>Something went wrong</h1>
        <Alert tone="error">
          This page could not be loaded or displayed. Reload to try again; if the server was updated, reloading fetches
          the new version.
        </Alert>
        <div>
          <Button variant="primary" onClick={this.props.onReload ?? (() => window.location.reload())}>
            Reload
          </Button>
        </div>
      </div>
    );
  }
}
