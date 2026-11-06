# TTN Mapper Go backend

This repository contains all TTN Mapper / CoverageMap Go backend services.
Make backend changes here on `master`. The original standalone GitHub repositories
are retired and are being archived for historical reference. Existing database workers remain in the root module. Imported
standalone services retain their own modules and vendored dependencies under
`services/`, avoiding dependency changes during the deployment migration.

## Build and check

Run `scripts/check-go.sh build` to build every module. Set `GO_BIN` to select a Go
executable. `compile-tests` compiles test packages without running test cases;
`test` runs all tests and requires the external systems used by integration tests.
Some existing tests contact PostgreSQL or external APIs, and the APRS socket test
waits indefinitely. Do not run the full test mode against production systems.

Root `go build ./...` and `go test ./...` do not traverse nested Go modules. Use
the script to cover the entire repository. Each module uses `-mod=vendor` and can
also be developed independently from its directory.

## Services

| Service | Source / Docker build context | Dockerfile |
| --- | --- | --- |
| postgres-insert-raw | `cmd/insert-raw` / repository root | `Dockerfile.insert-raw` |
| aggregate | `cmd/aggregate` / repository root | `Dockerfile.aggregate` |
| gateway-update | `cmd/gateway-update` / repository root | `Dockerfile.gateway-update` |
| gateway-bbox | `cmd/gateway-boundingbox` / repository root | `Dockerfile.gateway-bbox` |
| website-api | `cmd/website-api` / repository root | `Dockerfile.website-api` |
| websockets-live | `services/websockets-live` | `Dockerfile` in service directory |
| gateway-channels | `services/gateway-channels` | `Dockerfile` in service directory |
| ttn-to-aprs-bridge | `services/ttn-to-aprs-bridge` | `Dockerfile` in service directory |
| ingress-api | `services/ingress-api` | `Dockerfile` in service directory |
| tile-map-server | `services/tile-map-server` | `Dockerfile` in service directory |

The consolidation was merged into `master`. The tile server includes
the working changes from its original `simulated-coverage` branch, including
Go/JavaScript tests and simulation documentation that were untracked at import.
File hashes and the original source status are recorded in
[`docs/tile-server-import.json`](docs/tile-server-import.json).

Ingress uses its committed source; its former local test edit was discarded
during cleanup. The imported ChirpStack test was updated to use the current
`NetworkId` field so test packages compile.

Example: `docker build -t websockets-live services/websockets-live`.

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
