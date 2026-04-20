# VE3KSM Antenna Studio

A web-based antenna design and simulation tool using the Method of Moments
(MoM) electromagnetic solver.  Design wire antennas visually, run simulations,
and visualize 3D radiation patterns, SWR curves, and impedance plots.

## Architecture

**One Go binary does everything.**

- The Go backend (`backend/`) serves the JSON API at `/api/*` and compiles
  the TypeScript/TSX frontend in-process using
  [esbuild's Go library](https://pkg.go.dev/github.com/evanw/esbuild/pkg/api).
  The compiled JavaScript and CSS are served at `/assets/app.js` and
  `/assets/app.css`.
- **TypeScript never runs in the browser.**  It is transpiled inside the Go
  backend before any byte is sent to the client — the browser only ever
  receives plain JavaScript.
- There is **no Node.js, Vite, or nginx at runtime.**  Node is used only
  once at build time (`npm install`) to populate `frontend/node_modules/`
  so the Go bundler can resolve React, Three.js, Recharts, and friends.

```
                ┌──────────────────────────────────────┐
 Browser  ──►   │  Go process (cmd/server)             │
                │   /api/*      → MoM solver (Gin)     │
                │   /assets/*   → esbuild output       │
                │   /           → index.html (SPA)     │
                └──────────────────────────────────────┘
                        ▲
                        │ reads frontend/src + node_modules at startup,
                        │ bundles via github.com/evanw/esbuild/pkg/api
                        │
                ┌──────────────────┐
                │ frontend/ tree   │  (TS/TSX sources only)
                └──────────────────┘
```

## Prerequisites

- **Go** 1.22+ — [install](https://go.dev/dl/)
- **Node.js** 18+ with **npm** — required *only* to run `npm install` once
  so the Go bundler can find React et al.  Not used at runtime.

See [Installation Guide](doc/INSTALL.md) for WSL setup and full build instructions (includes automation scripts for Windows users).

## Quick Start

```bash
make deps         # npm install inside frontend/ (one-time)
make run          # go run ./cmd/server -dev
```

Then open <http://localhost:8080>.

In `-dev` mode the backend re-bundles the TypeScript tree on every asset
request, so edits to any file under `frontend/src/` are live on the next
page load.  Drop the flag for production (minified, cached, build-once).

## CLI flags

```bash
go run ./cmd/server \
  -dev                        # rebuild bundle on every request
  -frontend-dir ../frontend   # override auto-detection
```

Environment: `PORT` (default `8080`), `CORS_ORIGINS` (comma-separated list).

## Production Build

```bash
make build        # compiles to ./bin/antenna-studio
./bin/antenna-studio
```

The production binary bundles the frontend once at startup (minified and
cached), then serves it on every subsequent request without re-compilation.

## Docker

```bash
make docker-up    # builds image and starts container on port 8080
make docker-down  # stops the container
```

## Testing

```bash
make test         # cd backend && go test ./...
```

86+ unit tests across all backend packages.

## Key Capabilities

- **MoM solver** — MPIE formulation, triangle/sinusoidal/quadratic basis
  functions, LU and GMRES solvers, automatic dispatch
- **Ground models** — free space, PEC, real lossy ground, complex-image method;
  soil moisture presets (Very dry → Sea water) and interactive world-map
  ground-region picker (ITU-R P.832 conductivity zones) with user-drawn polygons
- **Wire geometry** — arbitrary 3D structures, interactive 3D editor,
  per-wire conductor material with skin-effect loss
- **Dielectric coatings** — per-wire insulation (PVC, PE, PTFE, enamel, …)
  modelled via the IS-card distributed-impedance formula; coating preset
  dropdown with 10 standard materials
- **Weather / environment loading** — global dielectric film (rain, ice, wet
  snow) stacked on top of per-wire coatings using a multi-layer IS-card
  formula; applied to all simulation modes
- **Frequency analysis** — single frequency, linear sweep, log sweep,
  interpolated fast sweep (AWE, 10–50× speedup)
- **Lumped loads** — R/L/C series or parallel on any segment
- **Transmission-line elements** — 2-port NEC-style TL cards
- **Far-field & metrics** — gain, directivity, F/B ratio, beamwidth, sidelobe
  level, efficiency; elevation polar cut renders as a full 360° circle
  (front lobe + back lobe visible simultaneously)
- **Impedance matching** — L, Pi, T, Gamma, Beta-match, toroidal transformer
  with E12 values and ASCII schematics
- **Advanced tools** — CMA, single-objective optimizer (Nelder-Mead / PSO),
  NSGA-II Pareto optimizer, transient analysis, convergence checker
- **Near-field** — E/H field on user-defined observation grid
- **Polarization** — Stokes-parameter ellipse decomposition
- **NEC-2** import and export for cross-tool compatibility; coated-wire designs
  use an effective-radius approximation on export, with a warning banner when
  lossy coatings cannot be represented exactly
- **Save / Load** — JSON design files; CSV sweep export

## Antenna Templates

Half-wave dipole · Quarter-wave vertical · 3-element Yagi · Inverted-V ·
Full-wave loop · Spiral

## Result Tabs

3D Pattern · Polar Cuts · SWR · Impedance · Currents · Smith Chart ·
Matching · Near-Field · Polarization · CMA · Optimizer · Pareto ·
Transient · Convergence

## Full Documentation

- [Feature List](README2.md) — complete capability reference
- [User Guide](doc/UserGuide.md) — step-by-step usage guide
- [Architecture](ARCHITECTURE.md) — internal design notes
