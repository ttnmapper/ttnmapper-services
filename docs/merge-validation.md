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

The original ingress smoke tests depended on DNS for email validation and included
an incomplete `TestTtsV3Pb` fixture that panicked in `CopyV3Fields`. On 7 November
2026, these were replaced with isolated handler and conversion tests using
synthetic payloads, injected email validation, and in-memory queues. Production
email validation still uses DNS. Existing ingress validation and ChirpStack tests
that already run locally are retained.

On 7 November 2026, the remaining live-system checks were replaced or removed:

- Gateway status fetches use an in-memory HTTP transport; conversion tests use
  synthetic NOC, web, Helium, Packet Broker and TTS data.
- Database tests cover parameter binding in GORM DryRun, cache separation and an
  empty-batch guard. They do not verify execution or results against PostgreSQL.
- MySQL packet migration conversion uses synthetic records rather than database IDs.
- Aggregation and radar tests assert signal buckets, distances, bearings and merges.
- APRS processing uses injected database callbacks and in-memory channels, covering
  malformed input, registration, lookup failure, duplicate suppression and frames.
  Rate-limit tests exercise first reports, boundaries and device separation.
- Raw packet conversion tests cover lookup inputs and IDs, nanosecond timestamps,
  precision/range handling, optional fields, cache identity and lookup-error retries.
- Raw insertion tests cover malformed/null-island messages, RSSI filtering,
  conversion/write/acknowledgement failures, and forwarding only stored gateways.
  Failed storage leaves the delivery unacknowledged and unpublished; earlier writes
  are not rolled back. This is not a transaction or an exactly-once delivery guarantee.
- Live grid/radar aggregation tests use fake storage and known coordinates to check
  repeated samples, signal boundaries, bearings/distances, invalid inputs and lookup/
  write failures. Live radar now uses database gateway coordinates for bearings;
  caches are updated only after successful saves. Save errors are surfaced by the
  database helpers and logged by the live aggregators.
- Aggregate command tests verify experiment filtering and moved-gateway routing to
  all antennas with the database's last-move cutoff. Database replay and replacement
  atomicity are now covered by the opt-in PostgreSQL suite described below;
  RabbitMQ acknowledgement/redelivery still requires integration coverage. The affected unit suites also pass with the race detector.
- Gateway-channels tests cover malformed and empty packets, gateway normalization,
  repeated reception counts, identity-separated caches and failures at each storage
  step. Frequency results now populate the frequency cache; association write errors
  are propagated to the service's existing fatal error handler.
- WebSocket hub tests cover combined application/device/user/experiment filters,
  JSON delivery, slow-client removal, repeated unregister, invalid packets and loop
  shutdown. The two services also pass their isolated suites with `go test -race`.
  These tests use in-memory channels and fake storage, not RabbitMQ, PostgreSQL or
  real WebSocket connections.
- Tile-server legacy database checks use cached synthetic last-heard timestamps;
  tile-range checks now assert expected bounds. Existing isolated regressions remain.
- Manual schema-migration, bulk reprocessing and database mutation checks were
  discarded. An empty website authentication test was also removed. Those workflows
  have no replacement end-to-end coverage.

The full six-module suite passes locally with Go 1.27.3 using
`GO_BIN=/snap/go/11382/bin/go GOCACHE=/tmp/ttnmapper-unit-go-cache scripts/check-go.sh test`.
The script applies a 60-second per-package timeout. These checks do not establish
production database behavior, live API availability, or APRS delivery.

## Disposable PostgreSQL integration validation (8 November 2026)

The opt-in `scripts/check-postgres-integration.sh -v` suite passed against a fresh
PostgreSQL 16.15 cluster using Go 1.27.3. PostgreSQL ran on a private temporary Unix
socket with TCP disabled; the runner removed its cluster afterward. The default
unit test script does not run these tagged tests.

Verified production database helpers against schemas migrated from current models:

- Foreign-key lookup values and reuse; packet timestamps stored at microsecond
  precision; numeric rounding and nullable/experiment fields.
- Invalid device foreign keys rejected; injected device-lookup failure followed
  by a successful retry without a poisoned cache.
- Replay excludes experiments, packets at/before the cutoff and other antennas.
  Grid/radar rebuilding applies distance filtering, produces known counts and
  geometry, preserves other antennas and produces the same result on a second run.
- Injected packet insertion failure leaves earlier rows committed.
- Grid/radar rebuilding now preserves previous aggregates on read, delete,
  insertion and commit errors. A failed later batch rolls back both earlier batches
  and deletion. Retrying in the same process produces the correct aggregate counts.
- Successful empty rebuilds remove only the target antenna's aggregates. Unit tests
  verify that cache replacement waits for commit and preserves other antennas.
  A subsequent live update reuses the committed row IDs and increments the rebuilt
  counts without resurrecting old rows. This regression also exposed and fixed the
  shared antenna cache: lookup now reads the same uint ID format as packet conversion.

The original integration suite demonstrated aggregate loss on failed replacement.
That behavior has been fixed: replay builds local state, checks scan/iteration/close
errors, and atomically replaces rows per antenna and aggregate type. Errors are
returned and logged by callers instead of terminating the process. Failed rebuilds
leave the existing cache untouched; successful commits replace only that antenna's
cached rows, including the newly assigned database IDs.

Grid and radar use separate transactions. This does not serialize concurrent live
writers in other processes or provide a single transaction across both aggregate
types. Packet delivery-wide transactions/idempotency remain follow-up work. No
production database, deployment or RabbitMQ service was contacted. The suite does
not prove recovery under concurrent live aggregation or compatibility with an
existing production schema.

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
