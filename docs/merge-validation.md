# Backend consolidation and validation

Updated 8 November 2026. The backend has one Go module, one vendor tree, thin
`cmd/` entry points, service implementations under `internal/`, and shared contracts
and infrastructure under `pkg/`. Original import revisions are listed in the root
README; `tile-server-import.json` retains the historical tile import manifest.

## Checks

The following local checks cover the consolidated repository:

- `scripts/check-go.sh build`: all services and maintenance commands.
- `scripts/check-go.sh test`: isolated unit/regression tests, with a per-package timeout.
- `go test -mod=vendor -race -timeout 120s ./...`: the same tests with the race detector.
- `node --test internal/tilemapserver/debug/coverage.test.cjs`: browser coverage regression.
- `scripts/check-postgres-integration.sh -v`: real database helpers against a newly
  created local PostgreSQL cluster, using a private Unix socket and automatic cleanup.
- `scripts/build-images.sh`: all ten targets of the root Dockerfile, retaining
  `golang:latest` and the unpinned Signal-Server source.

Set `GO_BIN` when Go is installed outside PATH. PostgreSQL server tools are required
for the opt-in database suite; `PG_BIN` selects their directory. The image check
requires a Docker daemon. The GitHub Actions workflow runs these checks without
publishing images or deploying services.

## Regression coverage

- Shared messages preserve JSON names, nanosecond timestamps and legacy optional
  fields. The public WebSocket projection preserves its original output fields.
- Ingress handlers use synthetic packets, injected validation and in-memory queues;
  DNS-backed production email checks are not invoked by handler unit tests.
  Combined ingress tests exercise ChirpStack v3 JSON and v4 JSON/protobuf routes
  with the default strict protobuf registry. v3 compatibility tests cover field
  aliases, quoted numbers, enum encodings, base64 IDs and fine timestamps.
- Raw insertion tests cover filtering, conversion, database/acknowledgement failures
  and forwarding only stored gateways. Conversion tests cover lookup reuse, caches,
  timestamps, numeric bounds and optional fields.
- Grid/radar tests cover signal boundaries, geometry, repeated receptions and
  transactional per-antenna replacement. PostgreSQL fault injection verifies that
  read, delete, insert, commit and later-batch failures preserve previous aggregates.
  Retries and subsequent live updates use correct counts and committed row IDs.
- Gateway frequency tests verify cache isolation and actual PostgreSQL upsert/counting.
- WebSocket tests cover filters, delivery, registration/removal, slow clients and shutdown.
- Shared RabbitMQ setup tests use fake channels to verify durability, auto-delete,
  acknowledgement, prefetch and persistence flags. No live broker is contacted.
- Configuration tests verify hardcoded defaults, environment overrides, invalid
  values and that configuration files are ignored. Startup configuration is env-only.
  Configuration values are no longer dumped at startup, and parser errors omit
  supplied values. In-memory APRS transport tests cover login, frame delivery,
  disconnects and cancellation without logging credentials.

Service-specific validation policies and query/cache adapters remain local. Shared
models/connections use GORM v2. Go's minimum version remains 1.24, the tile server's
pre-existing minimum; local checks used Go 1.27.3 and PostgreSQL 16.15.

## Limits and separate reliability work

The database suite migrates current Go models into a fresh database. It does not
prove compatibility with an existing production schema. RabbitMQ delivery,
acknowledgement/redelivery and real WebSocket/APRS delivery still need dedicated
integration coverage. Unit tests contact no production systems.

Raw deliveries have no delivery-wide transaction: earlier packet rows remain after
a later insert failure. Grid and radar replacement each use their own transaction
per antenna; there is no combined transaction across aggregate types or coordination
with concurrent live writers in other processes. These remain separate reliability
work, rather than prerequisites for the repository consolidation.

## Historical rollout evidence

On 6 November 2026 the operator built and started all ten services from source
commit `189c74d`. Supplied logs showed live ingress, PostgreSQL insertion,
aggregation, gateway processing and WebSocket consumption. All eight listed
RabbitMQ queues had one consumer and no backlog; Prometheus reported ready.
APRS authenticated, without independent confirmation of frame delivery.

On 8 November 2026 the operator rebuilt and deployed the module/layout/infrastructure
cleanup on both hosts. Supplied services-vm output showed all ten application
containers running after four minutes, all eight queues with one consumer and no
backlog, and Prometheus ready. Production TMS output showed both application
images built and containers replaced, with the API and tile server starting.
The TMS configuration directory now links into the deployment Git checkout.
These logs predate the credential logging and protobuf fixes described above;
those fixes need another deployment. Public endpoint checks, individual scrape-target
health and sustained runtime stability were not independently established.
Historical rollback image tags were removed during operator-requested cleanup;
the old rollback manifest is not a ready-to-use recovery mechanism.
