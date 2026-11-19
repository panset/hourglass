// SPDX-License-Identifier: MIT
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

// Dev server: `npm run dev` proxies /api, /auth and /healthz to a local hourglassd
// (VITE_PROXY_TARGET, default http://localhost:8080), so the browser sees one origin (UI-4).
// Set VITE_API_BASE instead to call another origin directly (needs cors.allowed_origins on the server).
export default defineConfig(({ mode }) => {
  const target = loadEnv(mode, ".", "VITE_").VITE_PROXY_TARGET ?? "http://localhost:8080";
  return {
    plugins: [react()],
    build: { sourcemap: false, target: "es2022" },
    server: {
      proxy: {
        "/api": { target },
        "/auth": { target },
        "/healthz": { target },
      },
    },
  };
});
