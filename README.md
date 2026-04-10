# Antenna Studio

A web-based antenna design and simulation tool using the Method of Moments (MoM) electromagnetic solver. Design wire antennas visually, run simulations, and visualize 3D radiation patterns, SWR curves, and impedance plots.

## Prerequisites

- **Go** 1.22+ — [install](https://go.dev/dl/)
- **Node.js** 18+ and **npm** — [install](https://nodejs.org/)

## Quick Start

The simplest way to start both backend and frontend together:

```bash
cd frontend && npm install && cd ..
make run
```

This launches both processes, prefixing output with `[backend]` and `[frontend]`.

- **Ctrl+C** — restart both processes
- **Ctrl+C x2** (within 2s) — shut down

### Launcher options

```bash
cd backend && go run ./cmd/launcher \
  -port 9090 \
  -frontend-port 3000 \
  -cors "http://localhost:3000"
```

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | 8080 | Backend API port |
| `-frontend-port` | 5173 | Vite dev server port |
| `-cors` | `http://localhost:<frontend-port>` | CORS allowed origins |
| `-backend-dir` | auto-detected | Path to `backend/` |
| `-frontend-dir` | auto-detected | Path to `frontend/` |

### Running separately (two terminals)

```bash
# Terminal 1
make dev-backend

# Terminal 2
make dev-frontend
```

Or manually:

```bash
# Backend (terminal 1)
cd backend && go run ./cmd/server

# Frontend (terminal 2)
cd frontend && npm run dev
```

The backend serves on `http://localhost:8080`. The frontend dev server on `http://localhost:5173` proxies `/api` to the backend.

## Production Build

```bash
# Build Go binary to bin/antenna-server
make build-backend

# Build frontend static files to frontend/dist/
make build-frontend
```

## Docker

```bash
docker-compose up --build
```

- Frontend: `http://localhost:3000`
- Backend API: `http://localhost:8080`

## Project Structure

```
backend/               Go server + MoM solver
  cmd/server/          Entry point
  internal/
    api/               HTTP handlers, request/response DTOs
    geometry/          Wire validation, antenna templates
    mom/               MoM solver (Z-matrix, Green's function, far-field)

frontend/              React app (Vite + TypeScript)
  src/
    components/        UI components (editor, inputs, results, layout)
    store/             Zustand state management
    api/               Backend API client
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/simulate` | Run single-frequency simulation |
| POST | `/api/sweep` | Run frequency sweep (SWR/impedance vs freq) |
| GET | `/api/templates` | List available antenna presets |
| POST | `/api/templates/:name` | Generate geometry from a template |

## Available Templates

- Half-Wave Dipole
- Quarter-Wave Vertical
- 3-Element Yagi
- Inverted-V Dipole
- Full-Wave Loop
