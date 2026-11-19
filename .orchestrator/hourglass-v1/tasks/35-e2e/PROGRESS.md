2026-11-19 created e2e/browser (Playwright 1.49.1), e2e/run.sh, .bazelignore/.gitignore entries; next: run e2e/run.sh bundled
2026-11-19 Chromium install via npx hangs at extraction on node 26.7; installed chromium, headless shell, ffmpeg by manual download+unzip into ~/Library/Caches/ms-playwright. run.sh bundled started
2026-11-19 bundled run 2: A0-A7,A9 pass, A8 failed: SSO re-login after clock jump rejected (Keycloak ID token on real clock expired vs jumped Hourglass clock). Fix: stretch realm token lifetime via Keycloak admin API in A0. rerunning
2026-11-19 bundled run 3 passed (713 s). running split
