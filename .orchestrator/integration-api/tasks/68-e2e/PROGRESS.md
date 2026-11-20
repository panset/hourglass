2026-11-20 wrote config, compose override, reporter, integration spec, run.sh mode, A7 update, README, D63, integration-stack.sh; next: run integration twice, bundled, split, ci
2026-11-20 integration runs 1-2 found test bugs only (default bands charge 0 when idle -> set base multiplier 1 like A0; OIDC user has empty username -> compare email; admin API needs content-type; outage field is duration_minutes). Run 3 (integration3.log) passed 10/10 in ~13 min.
2026-11-20 next: run 4 (second pass in a row), then e2e/run.sh bundled, split, ./ci.sh, integration-stack.sh check, RESULT.md.
2026-11-20 done: integration x2 pass, bundled, split, ci pass; RESULT.md written
