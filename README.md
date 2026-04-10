# Antenna Studio

A web-based antenna design and simulation tool using the Method of Moments (MoM) electromagnetic solver. Design wire antennas visually, run simulations, and visualize 3D radiation patterns, SWR curves, and impedance plots.

## Prerequisites

- **Go** 1.22+ — [install](https://go.dev/dl/)
- **Node.js** 18+ and **npm** — [install](https://nodejs.org/)

## Quick Start

### Backend

```bash
cd backend
go run ./cmd/server
```

The API server starts on `http://localhost:8080`.

To change the port:

```bash
PORT=9090 go run ./cmd/server
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

The dev server starts on `http://localhost:5173` and proxies `/api` requests to the backend.

### Both at once (two terminals)

```bash
# Terminal 1
make dev-backend

# Terminal 2
make dev-frontend
```

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
