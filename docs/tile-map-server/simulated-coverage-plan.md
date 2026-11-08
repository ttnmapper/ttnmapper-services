# Simulated coverage at regional and world scales

Draft for review. This document records the proposed architecture and implementation sequence for a future session. Implementation is deferred to a later session.

The goal is to show where coverage is expected from a detailed gateway view through a full Earth view, without downloading every gateway's native raster. Start with an inexpensive overview derived from observed gateway reach, retain the existing progressive rendering for detailed views, and evaluate line-of-sight coverage and server composition for intermediate views separately. Optimistic estimates and filling gaps are acceptable when zoomed out; finer representations should improve accuracy as the user zooms in, subject to resource limits.

## Goals and constraints

- Show expected coverage at world scale, including areas smaller than one display pixel. Coarse estimates may fill gaps and overstate coverage.
- Render all estimated coverage in the existing weakest-signal blue at every zoom level for now. Colour indicates expected coverage, not signal strength.
- Retain numeric strength and validity for simulation composition, even though the initial display uses one colour.
- Keep browser downloads, decoded raster memory, server memory, and concurrent work bounded.
- Keep panning responsive and distinguish potential coverage, partial simulation results, and complete simulation results.
- Support both WebGL2 and the Canvas fallback.
- Treat the current development machine as the lowest-performance client the website targets. Tune default browser budgets and mode-switching thresholds against this baseline, including the Canvas fallback.
- Continue generating detailed simulations on demand. Offline precalculation, persistent overview pyramids, and additional coverage caching remain deferred. Existing gateway artifacts and their freshness checks remain available.
- Preserve measured coverage, legacy simulation endpoints, and single-gateway inspection.

## Current behavior

The debug map requests a viewport manifest from `/simcov/rasters/network/{network_id}` and downloads gateway TIFFs independently. A worker decodes each TIFF into two bytes per cell: signal strength and validity. The browser combines the strengths before applying the palette. Each layer has a 64 MiB decoded raster budget and permits two concurrent downloads. A separate limit rejects manifests with more than 128 gateways by default.

The observed `26/66 loaded, 40 failed` warning is a resource limit, not necessarily a simulation failure. The manifest does not currently estimate decoded memory before downloads begin. Entire TIFFs are transferred even when a gateway contributes only a handful of display pixels.

Gateway discovery currently loads network gateways through `GetNetworkGateways`, applies effective parameter overrides, and filters by viewport intersection with the configured simulation radius. Network views use the existing five-day activity rule. A selected gateway can be inspected even when inactive.

Relevant implementation files:

- [Raster endpoints and selection](../../internal/tilemapserver/simcov_rasters.go)
- [Browser rendering and interaction](../../internal/tilemapserver/debug/coverage.js)
- [TIFF decoding](../../internal/tilemapserver/debug/coverage-worker.js)
- [Debug map](../../internal/tilemapserver/debug/map.html)
- [Configuration](../../internal/tilemapserver/common.go)
- [Legacy simulation tiles](../../internal/tilemapserver/tile_simcov.go)
- [Simulation and raster processing](../../internal/tilemapserver/signal_server.go)

## Proposed rendering modes

| Mode | Data | Meaning | Intended use |
| --- | --- | --- | --- |
| Potential overview | Effective gateway positions and mean directional reach from `radar_beam`, aggregated into a bounded grid | Optimistic expected coverage | World and country views, or detailed views beyond resource limits |
| Line-of-sight estimate (candidate) | Coarse terrain visibility results | Expected coverage approximated by terrain visibility | Intermediate views, if generation fits work budgets |
| Regional simulation | Reduced-resolution numeric results composed on the server | Predicted reception at the displayed resolution | Views with too many overlapping rasters for the browser |
| Detailed simulation | Existing progressive native gateway rasters | Predicted reception at native resolution | Local views and gateway inspection |

The regional mode is a later increment, not a prerequisite for a usable world overview. The first implementation can switch directly between potential overview and detailed simulation.

### Potential overview

Generate the overview on demand from metadata. A circle uses the effective gateway location and mean observed directional reach from the `radar_beam` table. For each `(antenna_id, bearing)`, take `MAX(distance_max)` across all signal buckets, without filtering by `level`. Then take the unweighted mean of those bearing maxima for each antenna. This preserves the original mean-radius proposal while selecting the furthest observation in each direction regardless of signal bucket. Do not use `distance2nd` or weight bearings by `samples`. Missing bearings do not contribute zero.

The circle intentionally fills directional gaps and represents optimistic expected coverage, rather than guaranteed reception or a simulation search boundary. The supplied table columns are `id`, `antenna_id`, `level`, `bearing`, `samples`, `last_updated`, `distance_max`, and `distance2nd`. Confirm distance units and the antenna-to-gateway mapping before implementation. The fallback for gateways without usable radials and handling of measurements from before a gateway moved remain review decisions; do not silently treat missing observations as no coverage.

Return a bounded grid of occupied cells rather than an unbounded list of gateway circles. Mark a cell when any footprint intersects it, including footprints whose centers are outside the viewport. Preserve small footprints as visible occupied cells. Grid resolution should follow display scale and have an explicit maximum cell count; its initial size remains a tuning decision.

A simple binary occupancy indication is the proposed first version. Optional gateway counts or density shading need a separate definition and legend, because overlap count is not signal strength.

Use the same blue currently used for the weakest signal level for all estimated coverage, including overview, line-of-sight, regional, and detailed modes. The legend should say “Expected coverage” and identify the current approximation or simulation mode in text; blue must not imply a particular received signal strength. Overlapping estimates should remain the same blue rather than accumulate into density or strength shading. Preserve measured-coverage styling. Handle wrapped longitude, repeated world copies, and the antimeridian explicitly. Leaflet's Web Mercator view covers approximately ±85 degrees latitude; a full Earth view here means that map extent, not polar coverage.

This endpoint must not run simulations, perform elevation lookups, decode TIFFs, or inspect every gateway artifact on disk. Initially reuse the current effective-parameter selection logic. Measure database and parameter-selection costs before optimizing discovery. A database bounding-box filter must account for radius intersection and optimized positions stored outside the gateway row; filtering only original gateway coordinates can omit valid contributors.

### Line-of-sight candidate

Evaluate coarse line-of-sight coverage as an intermediate representation. Benchmark whole-area visibility generation; the existing single-path `HasLineOfSight` helper must not be called independently for every output cell without proving acceptable work costs. Terrain visibility is an approximation of expected reception, and should be identified as such in the mode label. This candidate is not required for the initial overview and detailed-mode implementation.

### Regional simulation

Serve bounded raster tiles, provisionally 256 × 256 pixels, built from gateway simulation results. Read only intersecting source regions and reduce them to output resolution before composition where the TIFF layout and reader permit it.

Compose numeric strength and validity, then display covered cells in the shared blue. Downsampling should preserve the strongest valid source sample contributing to an output cell. This means “coverage exists somewhere within this cell,” which must be documented at coarse resolutions. The current packed signal increases with strength; taking a maximum over the TIFF's inverted red channel would choose the wrong result. Keep the coverage-validity rule explicit so missing cells remain transparent; the shared colour does not define a new signal cutoff.

Missing or invalid samples must remain separate from valid weak signals. Arrival order must not affect the completed result. Maximum resampling is a supported raster operation in [GDAL's reprojection documentation](https://gdal.org/en/stable/programs/gdal_raster_reproject.html), but introducing GDAL is an implementation option to evaluate, not an approved dependency.

Server rendering bounds client memory, but does not by itself bound source decoding or simulation cost. Current TIFF layout, compression, and reader capabilities must be measured before promising efficient window reads. If reading a window still decodes a full image, retain explicit per-request limits and reconsider the reader or artifact format in a separate decision.

Start by using fresh existing gateway artifacts. Do not trigger country-wide simulations when a user zooms out. Missing or stale simulations should be represented through a separately identified potential overview. If regional requests may eventually initiate simulations, require a small-area policy and explicit work budgets first.

A partial tile must never be silently presented as complete. Define a companion status response or manifest that distinguishes ready, missing, stale, and failed contributors. Cancel superseded requests and deduplicate identical in-flight composition without introducing a persistent tile cache. Use a separate rendering concurrency budget so overview traffic cannot monopolize interactive simulation work. Release simulation slots before entering a separate rendering queue.

## Automatic switching and map behavior

Make the client choose a mode before expensive downloads begin. Use a conservative zoom threshold initially, then add estimated decoded bytes, gateway count, raster dimensions, and renderer capability. Compressed TIFF size is not a decoded-memory estimate. Budget estimates must include already retained rasters and rendering buffers.

Choose default limits from measurements on the baseline machine, with headroom for transient decoding allocations and mode transitions. A high zoom level must not force an expensive mode when gateway count or estimated memory exceeds those limits. More capable clients may support greater detail, but baseline clients must retain a usable overview and responsive map interaction.

Extend the manifest with lightweight dimensions and eligibility information when available from existing metadata or small TIFF-header reads. Unknown dimensions must have a conservative fallback. An oversized view should produce an overview recommendation with counts and a reason, rather than merely returning an error that leaves the layer unusable. Exact endpoint names and response fields remain open.

Use separate entry and exit thresholds to prevent repeated mode changes near a limit. Keep the previous image visible while the replacement loads, following the current pan behavior. During a change between potential and simulated coverage, keep the legend matched to what is actually displayed. Cancel obsolete downloads and worker jobs, and evict unneeded detailed rasters when entering overview mode.

Do not fetch all gateway rasters before deciding to switch. A budget rejection should trigger an overview transition rather than dozens of failed gateway downloads. Genuine network, decoding, and simulation errors should remain visible as errors.

Proposed controls are automatic mode by default, an explicit potential-overview option, and detailed gateway inspection. A forced detailed network mode should retain resource limits. Initial thresholds, renderer-specific limits, and configuration names belong in the review decisions below.

## Implementation sequence

### Phase 1 Establish measurements

Use the current development machine as the minimum supported client performance baseline. Record its CPU, available RAM, GPU, operating system, browser version, viewport dimensions, and device pixel ratio with the benchmark results so the baseline can be reproduced. Measure WebGL2 and the Canvas fallback separately. Record network conditions and server execution separately; client hardware does not establish a production server capacity target. If client and server run on the same machine, identify that shared resource load in the results.

Record time to first visible coverage, time to complete coverage, transfer bytes, retained decoded bytes, peak browser memory, frame responsiveness, server CPU and memory, and gateway count. Separate simulation queue time, binary execution, TIFF reads and decoding, projection, composition, and PNG encoding.

Use the reported 66-gateway view as a reproducible case, alongside a single gateway, dense local overlap, country view, and full Earth view. Run with fresh artifacts already present and with artifacts missing. Establish performance targets from these measurements rather than choosing unsupported latency guarantees.

### Phase 2 Add the potential overview

Implement radial aggregation, bounded footprint aggregation, and the client overview renderer. Reuse effective locations and activity rules. Apply the shared blue to all estimated-coverage modes, with mode information in the legend, and add world-wrap behavior. Verify that world overview causes zero simulation and elevation invocations and zero TIFF decodes.

This phase is independently useful and should be reviewable before regional rendering begins.

### Phase 3 Switch before exhausting resources

Add the manifest recommendation, conservative estimates, switching thresholds, hysteresis, cancellation, and memory eviction. Ensure the 66-gateway case chooses overview before exceeding the browser budget. Preserve the existing progressive detail path and selected-gateway behavior.

### Phase 4 Prototype regional composition

Benchmark the existing TIFFs and candidate readers on a fixed regional request. Compare native browser rasters, bounded server tiles, and optionally a bounded viewport image. Measure total source-read work and duplicate work across adjacent tiles, not just PNG output time.

Implement the regional mode only after confirming acceptable source processing and choosing a completeness policy. If it cannot meet budgets without precalculated overviews or additional caching, keep the metadata overview and defer this phase rather than reintroducing unlimited work.

### Phase 5 Validate and document

Verify strongest-signal results against the existing detailed renderer on controlled fixtures. Document overview meaning, coarse-cell semantics, mode selection, resource limits, and incomplete-result behavior. Recheck the debug map in Chrome with WebGL2 and with WebGL disabled.

## Acceptance checks

- On the baseline machine, the single-gateway, 66-gateway, dense local, country, and full Earth cases meet the responsiveness and memory targets established in Phase 1, with both WebGL2 and the Canvas fallback. Views exceeding detailed-rendering limits switch to a bounded overview before exhausting resources.
- Full Earth view displays potential coverage without downloading gateway TIFFs or invoking simulations.
- Increasing gateway count does not increase overview response cells beyond the configured cap; server metadata-processing cost is measured separately.
- Circle radii equal the mean of populated bearing maxima across all signal buckets; neither sample counts nor `distance2nd` change the radius.
- All estimated-coverage modes use the existing weakest-signal blue at every zoom level, with the mode identified in text and uncovered cells transparent.
- Footprints smaller than a display pixel remain visible, with a legend explaining that the overview is approximate.
- Effective location overrides, activity filtering, radius intersection, antimeridian crossings, and repeated world copies behave consistently.
- Detailed views retain progressive loading, strongest-signal composition, and responsive panning.
- Budget pressure causes an understandable mode change instead of many resource-related gateway failures.
- Rapid pan and zoom do not publish obsolete responses or leak workers, retained rasters, or rendering jobs.
- Mode switching does not flicker near thresholds or temporarily label potential coverage as actual simulation.
- Regional results, if implemented, preserve validity and maximum strength, respect concurrency and source-memory limits, and identify incomplete coverage.
- Missing and stale artifacts are tested explicitly; no broad view silently schedules simulations for every intersecting gateway.

## Decisions for review

| Decision | Proposed starting point | What to resolve |
| --- | --- | --- |
| World overview meaning | Optimistic expected coverage from mean directional reach; gap filling accepted | Confirm distance units, antenna mapping, missing-radial fallback, and treatment of gateway moves |
| Radius calculation | Maximum `distance_max` across signal buckets per bearing, then unweighted mean across populated bearings | Validate against representative gateways |
| Estimated-coverage colour | Existing weakest-signal blue at all zoom levels | Match the existing colour value and choose mode-label wording |
| Overview representation | Bounded binary grid | Grid scale, appearance, minimum visible footprint, and whether counts are useful |
| Client performance baseline | Current development machine is the lowest-performance supported client | Record hardware and browser configuration; establish measured responsiveness targets and safe budgets for both renderers |
| Gateway eligibility | Current five-day rule and effective overrides | Whether overview and detail should use a different shared activity policy |
| Mode selection | Conservative zoom threshold, then memory and count estimates | Initial thresholds and hysteresis for CPU and WebGL2 |
| Single gateway | Retain detailed inspection where safe | Whether very wide selected-gateway views should also use a footprint |
| Regional delivery | Bounded tiles, with a viewport-image prototype for comparison | Window-read feasibility, adjacent-tile duplication, and cancellation behavior |
| Line-of-sight candidate | Coarse visibility generation prototype | Whether accuracy and generation cost justify an intermediate mode |
| Missing regional results | Fresh artifacts only, with separate potential indication | How status and partial completeness are communicated |
| Raster processing | Prefer an efficient existing Go path if practical | Whether a new reader or GDAL materially improves measured performance |
| Server budgets | Separate bounded composition and simulation queues | Concurrency, source bytes, timeout, and cancellation limits |

The recommended first implementation scope is Phases 1–3. The optimistic overview meaning, radial aggregation rule, and shared blue are agreed. Resolve the remaining data and fallback questions before implementation, then use measured results to decide whether intermediate line-of-sight or regional rendering is needed immediately.
