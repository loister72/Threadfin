# Application Health Backlog

This backlog tracks production-readiness work for the Threadfin fork. The focus is stream stability, guide correctness, operational safety, and keeping public automation maintainable.

## P0 - Stream Stability

- Add deterministic Plex/HDHR live-stream smoke tests that tune a channel, read HLS segments for a fixed window, then tear down the session.
- Instrument stream lifecycle transitions: tune request, provider connect, first bytes, first segment, client disconnect, fallback stream selected, provider EOF, and cleanup.
- Replace raw process kills with a single helper that checks for nil process handles, kills only when running, waits once, and records the reason.
- Audit buffer timeout behavior. The current third-party buffer timeout is hardcoded around loop counts and should be tied to `buffer.timeout`.
- Add regression coverage for provider EOF after first segment, zero-byte reads, missing temp files, and backup-stream failover.
- Split stream delivery into explicit Direct, Remux, and Transcode profiles. `ffmpeg-remux` should become the Plex IPTV default after smoke tests prove the structured profile in `docs/stream-engine-architecture.md`.
- Replace opaque third-party command strings with structured argument builders while preserving an advanced override path.
- Add a supervised process helper for FFmpeg/VLC with context cancellation, process-group cleanup, stderr ring buffer, and redacted diagnostics.
- Evaluate replacing the third-party `.ts` chunk handoff with a direct response stream or tested ring buffer to reduce post-start buffering.

## P1 - Guide And Logo Correctness

- Consolidate channel-logo source of truth. The mapping UI, XEPG channel state, M3U output, and XMLTV output can each hold a logo value.
- Add tests for XMLTV remapping in the UI model: changing XMLTV file/id should update active state, mapping id, and logo consistently.
- Add automated report checks for missing guide data and missing logos before refreshing Plex.
- Avoid duplicate DOM ids in mapping controls and add frontend tests for backup-channel pickers.

## P1 - Build And CI Health

- Keep Go checks on `-mod=mod` unless vendoring is intentionally restored and regenerated.
- Add CI for Go test/vet, Python guide-enricher tests, and shell syntax checks.
- Make TypeScript reproducible. Add a pinned `package.json`/lockfile or move generated JS out of review paths.
- Add a secret scan to prevent tokenized IPTV/Plex URLs from landing in source or history.

## P2 - Runtime Safety

- Replace remaining stray `fmt.Println` calls in runtime paths with `showDebug`, `showInfo`, or structured errors.
- Remove unreachable code and self-assignments whenever vet/static analysis finds them.
- Centralize domain/HTTPS/image-cache URL construction so M3U, XMLTV, upload-logo, and image-cache code cannot diverge.
- Redact tokens, provider URLs, and credentials from logs and support outputs.

## P2 - Operational UX

- Document supported on-prem automation patterns for cron/systemd/CI.
- Add a support bundle command that collects version, settings summary, stream health, guide report, and recent logs with secrets redacted.
- Add clear health report fields for Plex integration: tuner count, active sessions, guide channel count, matched guide count, and recent tune failures.
