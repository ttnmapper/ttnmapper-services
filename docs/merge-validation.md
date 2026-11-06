# Source consolidation validation

The migration branch was merged into `master`. Source commit `189c74d` was deployed to the
services VM by the operator on 6 November 2026.

## Passing checks

- All six modules build with Go 1.27.3 using their original vendored dependencies.
- All six modules' test packages compile with `scripts/check-go.sh compile-tests`.
- All ten service images build locally and built successfully on the services VM.
- Tile-server isolated simulation/runner/raster/cache/configuration tests pass.
- Tile-server browser regression test passes with
  `node --test pkg/tms/debug/coverage.test.cjs`.
- Compose validation passes with only the missing Cloudflare env-file path replaced
  by `/dev/null` in a temporary copy. No production credentials are needed for this check.

## Existing test limitations

The full suite is not green. The imported ingress handler tests depend on DNS
lookups for email validation and fail in the restricted test environment. Its
committed `TestTtsV3Pb` fixture also panics in `CopyV3Fields`; the test and runtime
implementation are unchanged from the source revision. The original ingress
checkout's separate local test changes were not imported and were discarded during cleanup.

Database/external-API tests have not been run against production. APRS tests can
transmit packets or wait indefinitely and were compiled without executing them.

## Small baseline repairs

- Root NOC test: replace an obsolete `ttnmapper-gateway-update/utils` import with
  the existing root utility package, and use `t.Fatal` for a dynamic error.
- Root raw-insert logging: use `Printf` for the existing formatted error message.
- Imported ChirpStack test: replace removed `NetworkType` with `NetworkId`.

No dependency upgrades, database schema changes, or maintenance jobs are part of
this consolidation. Uncommitted destination export/aggregate edits were discarded
at the operator's request during cleanup. Tile-server working source is captured separately in
`tile-server-import.json`.

## Services-VM rollout evidence

The operator supplied the server build, startup, logs, and queue output after the
rollout at approximately 15:34 UTC on 6 November 2026:

- All ten Go service images built from source commit `189c74d`.
- All ten service containers started; infrastructure containers stayed running.
- Logs showed live ingress, PostgreSQL inserts, aggregation, gateway processing,
  and WebSocket consumption. The website API started its HTTP listener.
- APRS authenticated successfully. This does not by itself verify delivery of a frame.
- All eight listed RabbitMQ queues had one consumer, zero ready messages, and zero
  unacknowledged messages at the time of the check.
- Prometheus reported ready. Individual scrape-target health and public API/tile
  responses were not included in the supplied output.
- Rollback image references were saved on the server in
  `/root/ttnmapper-go-rollback-20261106-151907/images.yml`.

The tile-server missing-config message was followed by successful loading of
its defaults/environment settings and HTTP startup. Invalid-location messages
were accompanied by continued processing of valid packets.

This is recorded operator-supplied evidence, not an independent remote inspection
or a long-term stability guarantee. The separate production tile-server host is
unchanged. Retain server rollback images until the operator chooses to retire them.
