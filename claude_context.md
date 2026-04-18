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
      input/                   — WireTable, SourceConfig, LoadEditor, TLEditor,
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

## Shipped Roadmap Items (1–17 + polish)

All 17 numbered roadmap items are shipped. See `ROADMAP.md` for details. Additionally shipped from the polish list: Touchstone/CSV sweep export, and convergence reporter.

## Remaining Roadmap (Polish)

From `ROADMAP.md`, still to do:

1. **Regression benchmarks** — pin DL6WU Yagi + K1FO design against published NEC-2 numbers.
2. **Environmental knobs** — rain/ice as dielectric shell or tan δ bump.

## Recent Session Work (This Chat)

### 1. Persisted expensive results across tab switches
Lifted CMA, Optimizer, Pareto, and Transient results (and Optimizer/Pareto config: variables, goals, objectives) from local `useState` into the global Zustand store so switching tabs doesn't lose expensive computation results.

**Files changed:** `antennaStore.ts`, `CMAViewer.tsx`, `OptimizerViewer.tsx`, `ParetoViewer.tsx`, `TransientViewer.tsx`

### 2. Convergence reporter
New feature: `POST /api/convergence` runs MoM at user segments (1x) and doubled segments (2x), returns impedance/SWR/gain at both levels plus relative deltas and a plain-English verdict.

**Files created:** `mom/convergence.go`, `ConvergenceViewer.tsx`
**Files changed:** `handlers.go`, `main.go` (route), `request.go` (no new DTO needed — reuses SimulateRequest), `types/index.ts` (ConvergenceResult), `client.ts` (checkConvergence), `MainLayout.tsx` (new tab), `antennaStore.ts` (convergenceResult slot)

### 3. Coated-wire dielectric loading
NEC-2 IS-card model: cylindrical dielectric shell on any wire adds distributed series impedance per unit length to the Z-matrix diagonal. Complex permittivity `ε = ε₀·εr·(1 − j·tanδ)` gives:

```
Z'_coat = ln(b/a) / (2π · ω · ε₀ · εr · (tanδ + j))   [Ω/m]
```

Applied as half-segment averaging over each triangle basis (same convention as `applyMaterialLoss`). Resistive component (tanδ > 0) feeds `lossPerBasis` for efficiency accounting.

**UI:** Wire table gains four new columns: Coating (preset dropdown: None/PTFE/PE/XLPE/Silicone/PVC/Kapton/Nylon/Custom), εr (editable number), Coat t (thickness, unit-converted), tan δ (loss tangent).

**Files created:** none
**Files changed:** `mom/types.go` (Wire + CoatingLossTangent), `mom/segment.go` (Segment), `mom/load.go` (applyCoating()), `mom/solver.go` (2× propagation + call), `mom/cma.go` (propagation + call), `api/request.go` (WireDTO + validation), `api/handlers.go` (mapping), `types/index.ts` (Wire + COATING_PRESETS), `store/antennaStore.ts` (defaults), `api/client.ts` (buildWires), `components/input/WireRow.tsx`, `components/input/WireTable.tsx`

### 4. Zoomable transient charts with CSV export

Upgraded TransientViewer charts: clicking any chart opens a large modal overlay (900x480) with proper axis ticks (niceTicks algorithm), dim grid lines, chart title, and an "Export CSV" button. Escape or click-outside to dismiss.

**Files changed:** `TransientViewer.tsx` — replaced simple `LineChart` with `DetailChart` + `ClickableChart` + `ZoomModal` components. Added `niceTicks()`, `formatTickLabel()`, `exportCsv()` helpers.

### 5. Tab bar wrapping on narrow windows

Added `flex-wrap: wrap` to `.tab-bar` so all 16 tabs remain accessible when the right panel is too narrow — they flow onto a second row instead of scrolling off-screen.

**Files changed:** `frontend/src/index.css`

## Known Recurring Issues

### Bash heredoc `\!` escaping
When using bash heredocs (`cat << 'EOF'`) to write Go files, bash escapes `!` to `\!`, breaking Go `!=` operators. **Always verify** Go files written via bash for `\!` and fix with the Edit tool (`replace_all: true`, `\!` → `!`).

### Mount sync between Windows and Linux sandbox
The Windows Edit/Write tools and the Linux bash mount can get out of sync. Files written via Edit may appear truncated in bash, and files patched via bash Python may cause duplicate tails on the Windows side. **Workaround:** prefer Edit tool for all file modifications; use bash only for verification reads and running commands.

### No Go compiler in sandbox
Go is not available in the Claude sandbox environment. All Go compilation must be verified by the user running `make test` on their machine.

## Wire Numbering Convention
The GUI shows wires numbered **1 to N** (1-based) in all dropdowns and labels, but the underlying data uses **0-based indexing** when sent to the backend. This was fixed across Optimizer and Pareto viewers (display `Wire {wi + 1}`, send `wi` as the value).
