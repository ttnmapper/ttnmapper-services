# TTN Mapper services

This repository contains all TTN Mapper / CoverageMap Go backend services.
Make backend changes here on `master`. The original standalone GitHub repositories
are archived for historical reference; their local checkouts have been removed.
Existing database workers remain in the root module. Imported
standalone services retain their own modules and vendored dependencies under
`services/`, avoiding dependency changes during the deployment migration.

The repository is [ttnmapper/ttnmapper-services](https://github.com/ttnmapper/ttnmapper-services).
The root Go module is named `ttnmapper-services`; standalone service modules
retain their existing names. Docker service names and executable names are unchanged.

## Build and check

Run `scripts/check-go.sh build` to build every module. Set `GO_BIN` to select a Go
executable. `compile-tests` compiles test packages without running test cases;
`test` runs the isolated suite across all six modules with a 60-second timeout
per package. No production credentials, live databases, RabbitMQ or APRS servers
are required. HTTP fetch tests use in-memory transports; database query tests use
GORM DryRun with automatic ping disabled. Tile-server tests use temporary files
and local fake executables and still require the module's native build dependencies.

Legacy manual migration, reprocessing, status-fetch and APRS-send checks were
removed. Their replacement unit tests cover selected logic, not database execution
or end-to-end delivery. The website authentication test was an empty placeholder
and was removed without claiming authentication coverage. Add future integration
tests behind an explicit opt-in and use disposable infrastructure.

Root `go build ./...` and `go test ./...` do not traverse nested Go modules. Use
the script to cover the entire repository. Each module uses `-mod=vendor` and can
also be developed independently from its directory.

## Services

| Service | Source directory | Dockerfile |
| --- | --- | --- |
| postgres-insert-raw | `cmd/insert-raw` / repository root | `Dockerfile.insert-raw` |
| aggregate | `cmd/aggregate` / repository root | `Dockerfile.aggregate` |
| gateway-update | `cmd/gateway-update` / repository root | `Dockerfile.gateway-update` |
| gateway-bbox | `cmd/gateway-boundingbox` / repository root | `Dockerfile.gateway-bbox` |
| website-api | `cmd/website-api` / repository root | `Dockerfile.website-api` |
| websockets-live | `services/websockets-live` | `Dockerfile.websockets-live` |
| gateway-channels | `services/gateway-channels` | `Dockerfile.gateway-channels` |
| ttn-to-aprs-bridge | `services/ttn-to-aprs-bridge` | `Dockerfile.ttn-to-aprs-bridge` |
| ingress-api | `services/ingress-api` | `Dockerfile.ingress-api` |
| tile-map-server | `services/tile-map-server` | `Dockerfile.tilemapserver` |

The consolidation was merged into `master`. The tile server includes
the working changes from its original `simulated-coverage` branch, including
Go/JavaScript tests and simulation documentation that were untracked at import.
File hashes and the original source status are recorded in
[`docs/tile-server-import.json`](docs/tile-server-import.json).

Ingress uses its committed source; its former local test edit was discarded
during cleanup. The imported ChirpStack test was updated to use the current
`NetworkId` field so test packages compile.

All Compose Git build contexts use the plain repository URL and therefore build
from the default-branch head. Root-level Dockerfiles support every service.

Example: `docker build -f Dockerfile.websockets-live -t websockets-live .`.
The original module-local Dockerfiles also support local subdirectory builds.

Imported service images use an empty `conf.default.json` to retain Go defaults
and environment overrides. Source templates remain available as examples; real
`conf.json` files are excluded from imported build contexts. Existing root worker
configuration behavior remains unchanged.

Production orchestration lives in the separate `ttnmapper-deployment` repository.
Keep independent service processes, names, queues, API routes, and database schema
behavior when maintaining these services.

The services VM was deployed successfully from source commit `189c74d` on
6 November 2026. See [validation and rollout evidence](docs/merge-validation.md).
The separate production tile-server host has not been migrated.

## Import provenance

Standalone imports copy committed source snapshots, except the tile server, which
includes the explicitly requested working changes; the original repositories retain
their full Git histories. No runtime caches, compiled binaries, IDE state, or
local credentials are imported. Original copyright notices are retained.

| Original GitHub repository | Commit | Destination |
| --- | --- | --- |
| `ttnmapper/websockets-live` | `5c1d35effd361f4d18113c91a0ff09e2fac8e640` | `services/websockets-live` |
| `ttnmapper/gateway-channels` | `220e100b2e9c32fb6bc0e879209417fc9175e3cd` | `services/gateway-channels` |
| `ttnmapper/ttn-to-aprs-bridge` | `90aceb405eca83ab651f3b4cc1a56510c354d816` | `services/ttn-to-aprs-bridge` |
| `ttnmapper/ingress-api` | `4e93ff6` | `services/ingress-api` |
| `ttnmapper/tile-map-server` | `4efbeb9` plus working changes recorded in the manifest | `services/tile-map-server` |

The tile-server module also contains `cmd/optimize-location` and
`cmd/optimize-radio`.

Existing maintenance commands remain under `cmd/`: `clean-gateway-locations`,
`export-packets`, `fix-helium-hotspots`, `fix-v2-networkid`,
`migrate-mysql-gateways`, and `migrate-mysql-packets`. Moving these tools does not
require executing them.
