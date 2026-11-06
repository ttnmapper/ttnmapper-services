# Source consolidation validation

Branch: `merge-go-services`. No production rollout performed yet.

## Passing checks

- All six modules build with Go 1.27.3 using their original vendored dependencies.
- All six modules' test packages compile with `scripts/check-go.sh compile-tests`.
- Imported WebSockets, gateway-channels, APRS, and ingress images build locally.
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
checkout's separate local test changes have not been imported.

Database/external-API tests have not been run against production. APRS tests can
transmit packets or wait indefinitely and were compiled without executing them.

## Small baseline repairs

- Root NOC test: replace an obsolete `ttnmapper-gateway-update/utils` import with
  the existing root utility package, and use `t.Fatal` for a dynamic error.
- Root raw-insert logging: use `Printf` for the existing formatted error message.
- Imported ChirpStack test: replace removed `NetworkType` with `NetworkId`.

No dependency upgrades, database schema changes, or maintenance jobs are part of
this consolidation. Existing destination export/aggregate edits remain outside
migration commits. Tile-server working source is captured separately in
`tile-server-import.json`.
