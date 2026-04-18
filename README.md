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
make build    #