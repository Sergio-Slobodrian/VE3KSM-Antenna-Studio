# Antenna Studio

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
make build    # → ./bin/antenna-studio
```

Deploy the binary alongside the `frontend/` directory (with `node_modules/`
populated) and launch with `-frontend-dir /path/to/frontend`.

## Docker

```bash
docker-compose up --build
```

- App: <http://localhost:8080>

The Dockerfile uses a throwaway Node stage to run `npm install`, a Go stage
to build the binary, and a minimal alpine runtime containing the binary
plus the frontend source tree and `node_modules/`.  Node itself is not in
the runtime image.

## Project Structure

```
backend/
  cmd/server/        Entry point — API + frontend bundler + static serving
  internal/
    api/             HTTP handlers, request/response DTOs (Gin)
    assets/          esbuild-backed TS/TSX/CSS bundler + asset handlers
    config/          Env-var configuration loader
    geometry/        Wire validation, antenna templates
    mom/             MoM solver (Z-matrix, Green's function, far-field)

frontend/            TS/TSX source tree consumed by backend/internal/assets
  src/
    components/      UI components (editor, inputs, results, layout)
    store/           Zustand state management
    api/             Backend API client
```

## API Endpoints

| Method | Path                   | Description                                 |
|--------|------------------------|---------------------------------------------|
| POST   | `/api/simulate`        | Run single-frequency simulation             |
| POST   | `/api/sweep`           | Run frequency sweep (SWR/impedance vs freq) |
| GET    | `/api/templates`       | List available antenna presets              |
| POST   | `/api/templates/:name` | Generate geometry from a template           |

## Available Templates

- Half-Wave Dipole
- Quarter-Wave Vertical
- 3-Element Yagi
- Inverted-V Dipole
- Full-Wave Loop
