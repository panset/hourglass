// SPDX-License-Identifier: MIT
import { ToastProvider } from "./components";
import { Gate } from "./shell/Gate";
import { SessionProvider } from "./shell/session";

/** Providers plus the session gate. The router is supplied by the caller (BrowserRouter, or MemoryRouter in tests). */
export function App() {
  return (
    <SessionProvider>
      <ToastProvider>
        <Gate />
      </ToastProvider>
    </SessionProvider>
  );
}
