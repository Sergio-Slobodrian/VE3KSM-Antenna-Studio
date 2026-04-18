# VE3KSM Antenna Studio — Claude Context

Continuity file for Claude sessions. Updated April 2026.

## Project Overview

**VE3KSM Antenna Studio** is a web-based wire-antenna design and simulation tool built by Sergio (VE3KSM, alpha_zorro@hotmail.com). It implements a full Method of Moments (MoM) electromagnetic solver in Go with a React/TypeScript frontend — no external EM engine dependency.

- **Backend:** Go, Gin HTTP framework, `gonum` for linear algebra / FFT. Single binary serves both API and frontend (esbuild-bundled in-process, no Node/Vite at runtime).
- **Frontend:** React + TypeScript, Zustand state management, SVG-based charting (no chart library), Three.js for 3D editor/pattern viewer. Path aliases via `@/`.
- **Build:** `make test` runs `cd backend && go test ./...`. No Go compiler in the Claude sandbox — compilation can only be verified by the user.

## Architecture

```
backend/
  cmd/server/main.go          — entry point, route registration
  internal/api/
    handlers.go                — Gin handlers for all POST /api/* endpoints
    request.go                 — Request/response DTOs, validation
  internal/mom/
    solver.go                  — Simulate(), main MoM entry point
    types.go                   — SimulationInput, SolverResult, Wire, Source, Load, etc.
    zmatrix.go                 — Z-matrix assembly
    farfield.go                — Far-field pattern computation
    metrics.go                 — F/B, beamwidth, sidelobe, efficiency
    green.go                   — Green's function kernel
    gen_kernel.go              — Generalised kernel for higher-order bases
    basis.go                   — Triangle/sinusoidal/quadratic basis functions
    gmres.go                   — GMRES iterative solver
    ground_complex_image.go    — Bannister complex-image ground model
    ground_real.go             — Lossy real ground (Fresnel)
    ground_image.go            — PEC image theory
    load.go                    — Lumped R/L/C loads
    material.go                — Conductor material library + skin-effect
    transmission_line.go       — NEC-style 2-port TL elements
    coating.go                 — Dielectric coating IS-card model (NEW)
    sweep_interpolated.go      — AWE/interpolated frequency sweeps
    nearfield.go               — Near-field E/H computation
    cma.go                     — Characteristic Mode Analysis (eigendecomposition)
    optimizer.go               — PSO single-objective optimizer
    pareto.go                  — NSGA-II multi-objective optimizer
    transient.go               — Time-domain transient via IFFT
    convergence.go             — Convergence reporter (1x vs 2x segmentation)
    polarization.go            — Stokes-parameter polarization analysis
    validate.go                — Segmentation accuracy validator
    reflection.go              — Smith chart / reflection coefficient
    spline.go                  — Cubic spline for interpolation
    segment.go                 — Wire segmentation
    quadrature.go              — Numerical integration
    coating_test.go            — Tests for coated-wire dielectric loading (NEW)
  internal/nec2/               — NEC-2 .nec import/export
  internal/match/              — Matching network synthesis (L/T/pi/gamma/beta)
  internal/geometry/           — Coordinate geometry helpers
  internal/assets/             — esbuild frontend bundler
  internal/config/             — Server config

frontend/
  src/
    main.tsx                   — App entry
    api/client.ts              — All backend API calls (fetch wrappers)
    types/index.ts             — Shared TypeScript types + constants
    store/antennaStore.ts      — Zustand global state
    components/
      layout/MainLayout.tsx    — Split-panel layout, tab bar, tab routing
      layout/Header.tsx        — Top header bar
      layout/StatusBar.tsx     — Bottom status bar
      editor/WireEditor.tsx    — 3D Three.js wire geometry editor
      input/                   — WireTable, WireRow, SourceConfig, LoadEditor, TLEditor,
                                 GroundConfig, FrequencyInput, TemplateSelector
      results/                 — All result viewer tabs (see tab list below)
```

## API Endpoints

| Method | Path | Handler | Purpose |
|--------|------|---------|---------|
| POST | /api/simulate | HandleSimulate | Single-frequency MoM simulation |
| POST | /api/sweep | HandleSweep | Multi-frequency sweep |
| POST | /api/nearfield | HandleNearField | Near-field E/H on observation plane |
| POST | /api/cma | HandleCMA | Characteristic Mode Analysis |
| POST | /api/optimize | HandleOptimize | PSO single-objective optimization |
| POST | /api/pareto-optimize | HandleParetoOptimize | NSGA-II multi-objective |
| POST | /api/transient | HandleTransient | Time-domain transient (IFFT) |
| POST | /api/convergence | HandleConvergence | Convergence check (1x vs 2x segs) |
| POST | /api/match | HandleMatch | Matching network synthesis |
| POST | /api/nec2/import | HandleNEC2Import | Parse NEC-2 .nec deck |
| POST | /api/nec2/export | HandleNEC2Export | Generate NEC-2 .nec deck |
| GET | /api/templates | HandleGetTemplates | List antenna templates |
| POST | /api/templates/:name | HandleGenerateTemplate | Generate template geometry |

## Frontend Tabs

The MainLayout tab bar has these tabs (Tab type union in MainLayout.tsx):

`3d` | `pattern` | `cuts` | `smith` | `metrics` | `swr` | `impedance` | `currents` | `matching` | `nearfield` | `polarization` | `cma` | `optimizer` | `pareto` | `transient` | `convergence`

Each maps to a viewer component in `components/results/`.

## Zustand Store (antennaStore.ts)

Global state holds: `wires`, `source`, `loads`, `transmissionLines`, `ground`, `frequency`, `referenceImpedance`, `simulationResult`, `sweepResult`, `displayUnit`, `selectedWireId`, `isSimulating`, `error`, `patternCamera`, plus **persisted result caches** for expensive computations:

- `cmaResult` / `setCmaResult`
- `optimResult` / `setOptimResult` + `optimVariables` / `optimGoals` (config persists too)
- `paretoResult` / `setParetoResult` + `paretoVariables` / `paretoObjectives`
- `transientResult` / `setTransientResult`
- `convergenceResult` / `setConvergenceResult`

These survive tab switches so users don't lose expensive computation results. The `loadTemplate` action clears all cached results.

## Coordinate Convention

All spatial coordinates stored internally in **meters**, physics convention (**Z-up**). Display-unit conversion is handled at the UI layer via `METERS_TO_UNIT` factors (meters, feet, inches, cm, mm).

## Wire Type (types/index.ts)

```typescript
interface Wire {
  id: string;
  x1, y1, z1, x2, y2, z2: number;  // endpoints (meters)
  radius: number;                    // meters
  segments: number;
  material: Material;                // '' = perfect conductor
  coatingThickness: number;          // outer shell thickness (m); 0 = bare
  coatingEpsR: number;               // relative permittivity εr
  coatingLossTan: number;            // loss tangent tanδ
}
```

The `lengthFields` set in WireRow.tsx includes `coatingThickness` for unit conversion.

## Shipped Features (All Roadmap Items Complete)

All 17 numbered roadmap items are shipped. Additionally shipped:

- Touchstone/CSV sweep export
- Convergence reporter (1x vs 2x segmentation)
- **Coated-wire dielectric loading** (IS-card model, εr shell + tanδ)
- **Coating preset dropdown** in WireTable (Bare wire default, PVC, PE, PTFE, FEP, XLPE, Nylon, Rubber, Enamel, Ice, Water film)
- **Global weather/environment loading** — dielectric film stacked on top of per-wire coatings, multi-layer IS-card formula
- **Elevation polar cut full-circle rendering** — back lobe (phi+180°) now visible on left half of elevation plot

## Coated-Wire Dielectric Loading (IS-card model)

**Physics:** Distributed series impedance per unit length added to Z-matrix diagonal:
```
Z'_coat = (jωμ₀ / 2π) · (1 − 1/ε_r*) · ln(b/a)
```
where a = conductor radius, b = a + coatingThickness, ε_r* = εr(1 − j·tanδ).

**Implementation:**
- `mom/coating.go`: `applyCoatingLoading()` — adds coating impedance onto each interior basis (50/50 split over adjacent segment lengths), matching `applyMaterialLoss` convention
- Called in both `Simulate()` and `SimulateNearField()` after material loss
- `mom/types.go`: `Wire` struct has `CoatingThickness`, `CoatingEpsR`, `CoatingLossTan`
- `api/request.go`: `WireDTO` has matching fields
- `api/handlers.go`: `simulateRequestToInput()` forwards coating fields

**Frontend:**
- `types/index.ts`: `CoatingPreset` interface + `COATING_PRESETS` array (11 entries)
- `components/input/WireTable.tsx`: "Coating Preset", "Coat-t", "εr", "tanδ" column headers
- `components/input/WireRow.tsx`: preset dropdown (fills all three fields), coat-t input (unit-converted), εr/tanδ inputs (greyed out when thickness=0)
- `api/client.ts`: `buildWires()` serializes coating fields as snake_case, omits when bare
- `store/antennaStore.ts`: default wire + `addWire` + `loadTemplate` all include coating defaults

**Tests** (`mom/coating_test.go`):
1. `TestCoating_BareWireUnchanged` — zero-thickness coating produces identical impedance
2. `TestCoating_ResonanceShift` — 2mm PVC on 20m dipole lowers resonance ≥0.4% and raises reactance ≥5Ω at bare resonant frequency
3. `TestCoating_LossyCoatingAddsResistance` — tanδ=0.05 raises feed-point resistance

**Coating Presets** (standard thicknesses, can be edited after applying):

Dropdown defaults to "Bare wire". No "— Preset —" placeholder — bare wire is the first and default entry.

| Preset | εr | tanδ | Default thickness |
|---|---|---|---|
| Bare wire *(default)* | 1.0 | 0 | 0 |
| PVC | 3.8 | 0.05 | 1.5 mm |
| PE | 2.3 | 0.0002 | 2 mm |
| PTFE (Teflon) | 2.1 | 0.0002 | 1 mm |
| FEP | 2.1 | 0.0003 | 1 mm |
| XLPE | 2.3 | 0.0003 | 2 mm |
| Nylon (PA) | 3.5 | 0.04 | 1 mm |
| Rubber (EPDM) | 3.0 | 0.02 | 2 mm |
| Enamel/varnish | 3.5 | 0.04 | 0.08 mm |
| Ice (weather) | 3.17 | 0.002 | 1 mm |
| Water film (wet) | 80 | 0.2 | 0.1 mm |

## Global Weather / Environment Loading

**Physics:** Multi-layer generalised IS-card formula stacked inner→outer:
```
Z'_total = (jωμ₀/2π) · Σ_i (1/ε_{i−1}* − 1/ε_i*) · ln(b_i / b_{i−1}),  ε₀*=1
```
Layer 1 (optional): per-wire coating. Layer 2 (optional): global weather film.

**Backend:**
- `mom/coating.go`: `dielectricLayer` struct + `multilayerZPerUnitLen()` + `weatherLayer()` preset lookup + `applyCoatingLoading()` (replaces single-layer version)
- `mom/types.go`: `WeatherConfig { Preset, Thickness, EpsR, LossTan }` + `Weather WeatherConfig` in `SimulationInput`
- `api/request.go`: `WeatherDTO` + `Weather WeatherDTO` in `SimulateRequest` / `SweepRequest`
- `api/handlers.go`: `simulateRequestToInput()` forwards all four weather fields
- Weather applied in all simulation modes (simulate, sweep, nearfield, CMA, optimizer, pareto, transient, convergence)

**Frontend:**
- `types/index.ts`: `WeatherConfig`, `WeatherPreset`, `WeatherPresetDef`, `WEATHER_PRESETS` (Dry/Rain/Ice/Wet snow with εr, tanδ, default thickness)
- `store/antennaStore.ts`: `weather: WeatherConfig` state + `setWeather(Partial<WeatherConfig>)` partial updater
- `components/input/WeatherPanel.tsx`: four `config-row` controls (preset dropdown, film thickness unit-converted, εr, tanδ) — all disabled when preset=dry; preset selection fills all three numeric fields; header accent-coloured when active
- `api/client.ts`: `buildWeather()` helper (returns undefined when dry/zero thickness); `weather` param threaded through all 8 API call functions
- `components/layout/MainLayout.tsx`: `<WeatherPanel />` inserted between GroundConfig and FrequencyInput
- Seven callers subscribe to `useAntennaStore((s) => s.weather)` and pass it to API: Header, CMAViewer, ConvergenceViewer, NearFieldViewer, TransientViewer, OptimizerViewer, ParetoViewer

**Preset fallback logic:** Backend `weatherLayer(preset)` returns hardcoded εr/tanδ by preset name. If the frontend sends explicit `EpsR ≥ 1`, those values override the preset defaults, allowing per-field editing without breaking the preset mechanism.

**Weather presets:**
| Preset | εr | tanδ | Default film |
|---|---|---|---|
| Dry | — | — | 0 mm |
| Rain | 80.0 | 0.05 | 0.1 mm |
| Ice | 3.17 | 0.001 | 1.0 mm |
| Wet snow | 1.6 | 0.005 | 3.0 mm |

## Elevation Polar Cut — Full-Circle Rendering

**Problem fixed:** The elevation cut previously sliced at `phi = peakPhi` only (theta 0→180°), so the back lobe (at `phi = peakPhi + 180°`) was invisible.

**Backend (`metrics.go`):** `PolarCuts` now has `ElevationBackDeg` / `ElevationBackGainDB`, populated by `sliceAt(pattern, peakPhi+180°, false)`. JSON: `elevation_back_deg`, `elevation_back_gain_db`.

**Frontend (`PolarCut.tsx`):** Elevation cut rendered as full 360° circle:
- Front side (phi=peak): `angleRad = (-deg × π) / 180`, sorted ascending (-90→+90), traces bottom→right→top
- Back side (phi=peak+180°): `angleRad = π + (deg × π) / 180`, sorted descending (+90→-90), traces top→left→bottom
- Combined as one closed SVG path; spokes at 8 × 45° positions labelled 0°–315°

**`api/client.ts`:** `RawCuts` interface + `polarCuts` mapping both include the two new back-side fields with `?? []` fallback.

## Remaining Roadmap (Polish)

1. **Regression benchmarks** — pin DL6WU Yagi + K1FO design against published NEC-2 numbers.
2. **Frequency-dependent ε/tanδ tables** for coatings (deferred from coating feature).
3. **Per-wire b/a ratio warnings** when coating is thick relative to wire radius (deferred).

## Tab Bar

The tab bar (`.tab-bar` in `index.css`) uses `flex-wrap: wrap` so tabs wrap to a second row when the window is narrow rather than overflowing off-screen.

## Known Recurring Issues

### Bash heredoc `\!` escaping
When using bash heredocs (`cat << 'EOF'`) to write Go files, bash escapes `!` to `\!`, breaking Go `!=` operators. **Always verify** Go files written via bash for `\!` and fix with the Edit tool (`replace_all: true`, `\!` → `!`).

### Mount sync between Windows and Linux sandbox
The Windows Edit/Write tools and the Linux bash mount can get out of sync. Files written via Edit may appear truncated in bash, and files patched via bash Python may cause duplicate tails on the Windows side. **Workaround:** prefer Edit tool for all file modifications; use bash only for verification reads and running commands.

### No Go compiler in sandbox
Go is not available in the Claude sandbox environment. All Go compilation must be verified by the user running `make test` on their machine.

## Wire Numbering Convention
The GUI shows wires numbered **1 to N** (1-based) in all dropdowns and labels, but the underlying data uses **0-based indexing** when sent to the backend. This was fixed across Optimizer and Pareto viewers (display `Wire {wi + 1}`, send `wi` as the value).
