2026-11-23 started; running bundled
2026-11-23 bundled run started; host processes hd(8091) and hourglassd(8080) by owner also listen on IPv4 localhost; docker binds wildcard; steps A0-A2p reached the container stack
2026-11-23 bundled exit 0 first try (712 s); starting split
2026-11-23 split failed only A7 (existing step, race: holds count read before holds section rendered; screenshot shows lab-03 Releasing, 2 of 2). Fix in acceptance.spec.ts untilHome callback in A7: wait for the 'Current holds' heading first. Rerun split.
2026-11-23 split exit 0 after fix; running integration
2026-11-23 integration exit 0 (830 s); final runs: both then integration
2026-11-23 final both failed in split at A8c (openUsage race: SPA redirects to login after the URL check, so no SSO sign-in was attempted; flaky, passed in 2 earlier runs). Fixed openUsage to wait for heading or SSO link. Rerun both and integration after the chained integration run ends.
2026-11-23 final rerun of both then integration started on the final tree (A7 and openUsage fixes)
2026-11-23 all done, RESULT.md written
