# CoverageMap.net (a.k.a. TTN Mapper)

People drive around with LoRaWAN sensors, reporting RSSI, SNR, location and other radio parameters.
They opt-in to upload these "packets" to TTN Mapper, where they are received by the [ingress-api](../ingress-api/).
These packets are written to PostgreSQL by a microservice ([source](../../cmd/insert-raw/)).
These packets are aggregated to zoom level 19 tiles (slippy map / OpenStreetMap tiles) by a microservice ([source](../../cmd/aggregate/)), and written to a database table.

## This project - the tile map server

This is a tile map server (TMS) implementation in Go.

The measured heat map tiles are drawn from the zoom level 19 aggregations in the database.
The TMS exposes endpoints to render these heat map tile PNGs per gateway or per network.

### Tile endpoints

Measured coverage:

- `GET /circles/network/{network_id}/{z}/{x}/{y}`
- `GET /circles/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}`
- `GET /blocks/network/{network_id}/{z}/{x}/{y}`
- `GET /blocks/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}`

Simulated coverage (see below):

- `GET /simcov/network/{network_id}/{z}/{x}/{y}`
- `GET /simcov/gateway/{network_id}/{gateway_id}/{z}/{x}/{y}`

Native simulated coverage, used by the debug map:

- `GET /simcov/rasters/network/{network_id}?west=...&south=...&east=...&north=...`
  returns a viewport manifest. Add `gateway_id=...` to select a single gateway,
  including one that has not been heard recently.
- `GET /simcov/rasters/gateway/{network_id}/{gateway_id}` returns a native-grid
  GeoTIFF, generating it on demand if missing or stale. Supports byte ranges,
  ETags, and conditional requests. The manifest supplies URLs with input versions.

Debug / comparison endpoints:

- `GET /debug/` — interactive Leaflet map overlaying the measured and simulated coverage layers (accepts `network_id`, `gateway_id`, `lat`, `lon`, `zoom` query parameters)
- `GET /debug/gateway/{network_id}/{gateway_id}/comparison` — measured vs simulated signal-vs-distance data (JSON)
- `GET /debug/gateway/{network_id}/{gateway_id}/analysis` — mean bias / RMSE between measured and simulated (JSON)
- `GET /debug/gateway/comparison.html?network_id=...&gateway_id=...` — a chart of the above

## Simulations

We are adding simulated coverage layers, per gateway and per network.

Simulations are produced by shelling out to an external [Signal-Server](https://github.com/Cloud-RF/Signal-Server) binary
(configured via `SignalServerBin`), using the Longley-Rice (ITM) propagation model.
Terrain is taken from Copernicus DEM tiles, which are downloaded on demand into `SignalServerSrtmDir`.
Simulation results are cached to disk per gateway (a `.gob` sample cache plus the raw `.tif`) and re-used to
render individual map tiles.

### Radio parameters

We use recommended default radio simulation parameters (climate, clutter, conductivity, dielectric, reliability,
etc.) for every gateway. These defaults, together with the terrain, are enough to produce a useful coverage estimate.

There is also an optional `optimize-radio` tool that tunes an extra constant-loss term per gateway, but in practice it
does not improve the match much, so we normally keep the default radio parameters.

### Location optimization

A registered gateway location can be off — due to GPS drift, or people setting it wrong for privacy.
The `optimize-location` tool searches for a better location by running many simulations and keeping the location whose
simulated coverage best matches the measured coverage. Per-gateway results (optimized location, height, clutter,
progress counters) are stored in `gateway_params.json`.

Location optimization is an **offline / manual** step; it is not run on the tile-request path, because it is far too
slow to run while serving a tile. When it (or `optimize-radio`) later updates `gateway_params.json`, the affected
gateway's simulation cache is detected as stale and the coverage is re-simulated on the next tile request.

Tuning the simulations further from the measured packets / aggregated z19 tiles is still a work in progress.

## Configuration

Configuration is read from `conf.json` (see `conf.json.template`). Notable keys:

- `SignalServerBin` — path to the Signal-Server executable (required for simulations). The
  Docker image builds [Signal-Server](https://github.com/loorisr/Signal-Server) from source
  and installs it at `/usr/local/bin/signalserver` (the default).
- `SignalServerSrtmDir` — directory for Copernicus DEM tiles
- `GatewayParamsFile` — per-gateway simulation parameters (e.g. `gateway_params.json`)
- `SimCovRadiusKm` — how far, in km, each gateway's coverage is simulated
- `CacheEnabled`, `CacheDirSimCov` — enable and locate the on-disk tile / simulation caches

### Simulation CPU budget

`SignalServerMaxProcesses` (`SIGNAL_SERVER_MAX_PROCESSES`, default `1`) limits
simultaneous Signal-Server processes within each Go process, including terrain-elevation queries and
offline optimization. `SignalServerThreads` (`SIGNAL_SERVER_THREADS`, default
`0`) controls the binary's `-number_threads` option. Zero divides the CPU budget
equally across the configured processes. Explicit settings are clamped so that
processes × threads does not exceed `GOMAXPROCS` or the logical CPU count. Set `GOMAXPROCS` explicitly to
the CPU allocation when running in a CPU-limited container. Configuration is
applied on the first binary invocation; restart after changing it.

For example, with `GOMAXPROCS=8`, the defaults run one process with eight threads.
Setting `SIGNAL_SERVER_MAX_PROCESSES=2` runs up to two processes with four threads
each. More processes also require more memory; measure both single-gateway
latency and network throughput before raising this limit.

`SignalServerTimeoutSeconds` (`SIGNAL_SERVER_TIMEOUT_SECONDS`, default `300`)
limits each binary invocation after it acquires a process slot. Queue time is
additional. Coverage is still generated synchronously on demand.

Unchanged parameter files and simulation metadata are reused in memory. Successful
terrain-elevation lookups are reused for ten minutes and invalidated when the DEM
tile changes. Existing `.gob` and `.tif` simulation caches remain compatible.

### Progressive coverage in the debug map

Select either simulated coverage layer on `/debug/`. The browser requests gateway
rasters independently (two at a time, cached artifacts first), decodes them in a
worker using the pinned GeoTIFF.js browser bundle, and projects their native
latitude/longitude grid with WebGL2 or a Canvas fallback. It takes the maximum
numeric signal strength before applying the existing coverage palette. Arrival order
does not affect the result; the status control distinguishes partial, complete,
and failed coverage. Panning reuses downloaded rasters and rebuilds the maximum
from the current contributors. During a pan the existing canvas moves with the
base map; composition resumes when movement ends. CPU-rendered pixels and their
map position are published together, and existing coverage remains visible while
the next gateway manifest loads. Requests for superseded views are cancelled.

This delivery path avoids server-side PNG rendering and zoom-19 sample expansion.
Legacy PNG and comparison endpoints remain available; they share the raster
simulation and construct samples lazily when needed. Native raster freshness is
stored alongside the TIFF in `.tif.json` metadata files. There is no offline
precalculation or network-wide coverage preaggregation.

This initial implementation downloads complete gateway GeoTIFFs. Each layer has a
64 MiB raster memory budget, a 64 MiB limit per download, and rejects rasters exceeding
the browser's texture limit or 16 million pixels. The manifest limits views to
`SimCovMaxRasterGateways` (`SIMCOV_MAX_RASTER_GATEWAYS`, default `128`); zoom in when
a view exceeds the limit. Browser-side window reads and resolution levels can be
added later to support much wider views. These debug layers use WebGL2 when
available, and automatically fall back to Canvas CPU rendering otherwise. The
fallback keeps progressive loading and strongest-signal composition, renders at
most one million viewport pixels, and yields between row batches to keep the map
responsive. Its rendering can be slower than WebGL2 for many overlapping gateways.

Run client regression tests with `node --test pkg/tms/debug/coverage.test.cjs`.
