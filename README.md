# TTN Mapper services

This repository contains all TTN Mapper / CoverageMap Go backend services.
Make backend changes here on `master`. The original standalone GitHub repositories
are archived for historical reference; their local checkouts have been removed.
All services and maintenance commands now belong to one root Go module and share
one dependency graph and root vendor tree. Service processes remain independent.

The repository is [ttnmapper/ttnmapper-services](https://github.com/ttnmapper/ttnmapper-services).
The root Go module is named `ttnmapper-services`. Docker service names and
executable names are unchanged.

## Build and check

Run `scripts/check-go.sh build` to build every service and tool. Set `GO_BIN` to select a Go
executable. `compile-tests` compiles test packages without running test cases;
`test` runs the isolated suite across the root module with a 60-second timeout
per package. No production credentials, live databases, RabbitMQ or APRS servers
are required. HTTP fetch tests use in-memory transports; database query tests use
GORM DryRun with automatic ping disabled. Tile-server tests use temporary files
and local fake executables. The real native simulator is built by the Docker target.

Legacy manual migration, reprocessing, status-fetch and APRS-send checks were
removed. Their replacement unit tests cover selected logic, not database execution
or end-to-end delivery. The website authentication test was an empty placeholder
and was removed without claiming authentication coverage. Add future integration
tests behind an explicit opt-in and use disposable infrastructure.

Root `go build -mod=vendor ./...` and `go test -mod=vendor ./...` cover the
entire repository. The minimum Go version is 1.24, matching the tile server's
existing requirement. The TTN throttled fork and APRS fork remain selected in the
root `go.mod`. GORM v2/PostgreSQL dependencies now use the tile server's versions;
all PostgreSQL services now use GORM v2 through a shared connection helper.
Docker build contexts must be the repository root to include this shared module.

## PostgreSQL integration tests

Run `GO_BIN=/path/to/go scripts/check-postgres-integration.sh -v` explicitly.
PostgreSQL server tools (`initdb`, `pg_ctl`, `createdb`) must be installed and the
runner must be a non-root user. Set `PG_BIN` to their directory if needed; the
runner also discovers Debian-style installations. No Docker dependency is needed.

The script creates a fresh cluster under `/tmp`, disables TCP, uses a private Unix
socket, and removes the cluster on exit. It accepts no existing server DSN and does
not use the production `POSTGRES_*` settings. Tests have the `integration` build tag
and are excluded from the ordinary unit suite. Test selection flags such as
`-run TestRebuild` can be passed to the script.

The suite migrates the current Go models and verifies insertion, lookup reuse,
foreign keys, numeric/nullable storage, and replay of non-experiment packets strictly
after a gateway move. Rebuild failure tests require the previous aggregates to
survive read, delete, insertion, commit and later-batch failures; retries must produce
the correct counts. Successful empty rebuilds clear only the target antenna.

Grid and radar replacement each use a transaction per antenna, with cache changes
published only after commit. They are separate transactions, and concurrent live
writers in other processes are not serialized by this change. Earlier packet rows
still survive a later raw insertion failure; delivery-wide transactions/idempotency
remain separate work. The suite does not validate an existing production schema or
exercise RabbitMQ acknowledgement/redelivery.

## Layout

`cmd/<service>/main.go` contains each executable's entry point. Implementations
and service-owned adapters live under `internal/<service>/`; shared contracts and
infrastructure remain under `pkg/`. Maintenance/optimization tools are also under
`cmd/`. Service documentation lives under `docs/`; preserved tuning data lives under
`examples/`. Original license notices are retained under `LICENSES/`.

## Services

| Service / Docker target | Entry point |
| --- | --- |
| `postgres-insert-raw` | `cmd/insert-raw` |
| `aggregate` | `cmd/aggregate` |
| `gateway-update` | `cmd/gateway-update` |
| `gateway-bbox` | `cmd/gateway-boundingbox` |
| `website-api` | `cmd/website-api` |
| `ingress-api` | `cmd/ingress-api` |
| `gateway-channels` | `cmd/gateway-channels` |
| `websockets-live` | `cmd/websockets-live` |
| `ttn-to-aprs-bridge` | `cmd/ttn-to-aprs-bridge` |
| `tile-map-server` | `cmd/tile-map-server` |

The tile-server working changes from its original `simulated-coverage` branch are
retained, including embedded browser assets and regression tests. Original file
hashes and import status are recorded in
[`docs/tile-server-import.json`](docs/tile-server-import.json). Those paths describe
the import snapshot; the source has since moved and been consolidated.

Service details: [ingress API](docs/ingress-api.md), [tile server](docs/tile-map-server.md),
and [APRS bridge](docs/aprs-bridge.md).

All Compose Git build contexts use the plain repository URL and therefore build
from the default-branch head. One root `Dockerfile` builds all ten services with
named targets. It retains `golang:latest` and the unpinned Signal-Server source.

Run `scripts/build-images.sh` for all images, or pass service names, for example
`scripts/build-images.sh website-api ingress-api`. `IMAGE_PREFIX` defaults to
`ttnmapper`. The equivalent direct command is
`docker build --pull --build-arg SERVICE=ingress-api --target ingress-api -t ttnmapper/ingress-api:latest .`.
All scratch images include CA certificates. The tile-server target retains Ubuntu
and its native terrain-simulation libraries. No image contains application config.

All services and maintenance tools use hardcoded defaults overlaid by environment
variables. Configuration files are not read or included in images. Set environment
variables directly or pass Docker an explicit `--env-file`; application startup does
not automatically read `.env` files. Invalid typed values fail startup.

Common settings include `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DATABASE`, `POSTGRES_DEBUG_LOG`, `AMQP_HOST`,
`AMQP_PORT`, `AMQP_USER`, `AMQP_PASSWORD`, and `PROMETHEUS_PORT`. Each service's
`Configuration` and default values are in its `internal/<service>/main.go` (tile
server: `common.go`). Existing queue-specific keys, including legacy spellings,
remain unchanged. Tile-server `gateway_params.json` is persisted optimization data,
not an application configuration file, and remains supported via `GATEWAY_PARAMS_FILE`.

Production orchestration lives in the separate `ttnmapper-deployment` repository.
Keep independent service processes, names, queues, API routes, and database schema
behavior when maintaining these services.

The operator confirmed the original services-VM rollout and production TMS source
migration. The subsequent cleanup changes have not been deployed here. See
[validation and rollout evidence](docs/merge-validation.md).

## Continuous integration

GitHub Actions runs static command builds, unit tests with the race detector,
the browser regression, disposable PostgreSQL integration tests, and all ten
Docker image builds on pull requests and pushes to `master`. It builds images
without publishing or deploying them. Go uses the current stable release, matching
the decision to keep `golang:latest` for Docker builds.

## Import provenance

Standalone imports copy committed source snapshots, except the tile server, which
includes the explicitly requested working changes; the original repositories retain
their full Git histories. No runtime caches, compiled binaries, IDE state, or
local credentials are imported. Original copyright notices are retained.

| Original GitHub repository | Commit | Current implementation |
| --- | --- | --- |
| `ttnmapper/websockets-live` | `5c1d35effd361f4d18113c91a0ff09e2fac8e640` | `internal/websocketslive` |
| `ttnmapper/gateway-channels` | `220e100b2e9c32fb6bc0e879209417fc9175e3cd` | `internal/gatewaychannels` |
| `ttnmapper/ttn-to-aprs-bridge` | `90aceb405eca83ab651f3b4cc1a56510c354d816` | `internal/aprsbridge` |
| `ttnmapper/ingress-api` | `4e93ff6` | `internal/ingressapi` |
| `ttnmapper/tile-map-server` | `4efbeb9` plus working changes recorded in the manifest | `internal/tilemapserver` |

The root module also contains `cmd/optimize-location` and
`cmd/optimize-radio`.

Existing maintenance commands remain under `cmd/`: `clean-gateway-locations`,
`export-packets`, `fix-helium-hotspots`, `fix-v2-networkid`,
`migrate-mysql-gateways`, and `migrate-mysql-packets`. Moving these tools does not
require executing them.
