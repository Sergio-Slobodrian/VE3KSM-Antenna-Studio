# VE3KSM Antenna Studio — Project Planning Notes

## Project Overview

A web-based antenna design tool using the **Wire/Method of Moments (MoM)** approach to model antenna geometry, simulate electromagnetic behavior, and visualize results as 3D radiation patterns and SWR graphs.

---

## Architecture

```
┌─────────────────────────────────────┐
│         React Frontend              │
│  - Three.js 3D wire editor          │
│  - Three.js radiation pattern       │
│  - Recharts/Plotly SWR & impedance  │
│  - Antenna template library         │
└────────────────┬────────────────────┘
                 │ REST or WebSocket
┌────────────────▼────────────────────┐
│         Go Backend                  │
│  - Gin/Echo HTTP server             │
│  - NEC2 geometry validator          │
│  - MoM solver (pure Go)             │
│    · Z-matrix builder               │
│    · Green's function integration   │
│    · LU decomposition (gonum)       │
│    · Far-field / near-field calc    │
│  - Result serializer (JSON)         │
└─────────────────────────────────────┘
```

### Technology Choices

| Layer | Technology |
|---|---|
| Frontend framework | React (Vite) |
| 3D rendering | Three.js |
| Charts | Recharts or Plotly |
| State management | Zustand |
| Backend language | Go |
| HTTP router | Gin |
| Linear algebra | gonum |
| Deployment | Self-hosted server |

---

## Go Backend — MoM Solver Pipeline

**Key Go libraries:**
- `gonum.org/v1/gonum` — matrix ops, LU decomposition, complex linear algebra
- `gin-gonic/gin` — HTTP router
- Standard `math/cmplx` — complex arithmetic throughout

**Solver pipeline:**
```
1. Parse wire geometry → segments (N segments total)
2. Build N×N complex Z-matrix
   └── Z[i][j] = mutual impedance via Pocklington kernel
       └── Numerical integration (Gauss-Legendre quadrature)
3. Build excitation vector V (voltage source at feed segment)
4. Solve Z·I = V  (gonum LU decomp → current vector I)
5. Compute far-field E(θ,φ) from currents at all (θ,φ) points
6. Derive: gain, directivity, SWR(f), impedance(f)
```

---

## Project Structure (Monorepo)

```
antenna-studio/
├── frontend/                  # React app (Vite)
│   ├── src/
│   │   ├── components/
│   │   │   ├── WireEditor/    # 3D Three.js canvas
│   │   │   ├── PatternViewer/ # 3D radiation pattern
│   │   │   ├── SWRChart/      # Frequency sweep plots
│   │   │   └── WireTable/     # Tabular wire input
│   │   ├── store/             # Zustand state
│   │   └── api/               # Fetch calls to Go backend
├── backend/                   # Go server
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── mom/               # MoM solver core
│   │   │   ├── segment.go     # Wire→segment subdivision
│   │   │   ├── zmatrix.go     # Impedance matrix builder
│   │   │   ├── green.go       # Green's function kernel
│   │   │   ├── solver.go      # LU solve, current vector
│   │   │   └── farfield.go    # Radiation pattern, gain
│   │   ├── geometry/          # Wire validation, ground plane
│   │   └── api/               # Gin handlers, JSON models
├── docker-compose.yml
└── README.md
```

---

## API Contract (Frontend ↔ Go)

### POST /api/simulate

**Request:**
```json
{
  "wires": [
    { "x1": 0, "y1": 0, "z1": 0, "x2": 0, "y2": 0, "z2": 1.0, "radius": 0.001, "segments": 11 }
  ],
  "frequency_mhz": 14.0,
  "ground": {
    "type": "perfect",
    "conductivity": 0.005,
    "permittivity": 13
  },
  "source": {
    "wire_index": 0,
    "segment_index": 5,
    "voltage": 1.0
  }
}
```

**Response:**
```json
{
  "impedance": { "r": 73.1, "x": 42.5 },
  "swr": 2.1,
  "gain_dbi": 8.3,
  "pattern": [
    { "theta": 0, "phi": 0, "gain_db": 2.1 }
  ],
  "currents": [
    { "segment": 0, "magnitude": 0.013, "phase": -12.3 }
  ]
}
```

### POST /api/sweep

**Request:** Same as `/simulate` plus:
```json
{
  "freq_start": 14.0,
  "freq_end": 14.35,
  "freq_steps": 50
}
```

**Response:**
```json
{
  "frequencies": [14.0, 14.007, "..."],
  "swr": [1.8, 1.7, "..."],
  "impedance": [{ "r": 73.1, "x": 42.5 }, "..."]
}
```

---

## Recommended Build Order

1. **Go backend skeleton** — Gin server, stub `/simulate` endpoint returning mock data
2. **React frontend shell** — Wire table + Three.js canvas, calls stub API
3. **Segment subdivision** — Turn wires into discrete segments in Go
4. **Z-matrix + solver** — The core MoM math (start with free-space, no ground)
5. **Far-field calculation** — Radiation pattern from solved currents
6. **Frontend 3D pattern** — Render the pattern sphere in Three.js
7. **Frequency sweep + SWR chart** — Loop solver over frequency range
8. **Ground plane** — Image theory for perfect ground, Sommerfeld for real ground
9. **Antenna templates** — Dipole, vertical, Yagi, loop presets
10. **Polish** — Export NEC2 deck, import NEC2, save/load designs

---

## Key Math Notes

### Pocklington Kernel Singularity
The trickiest part of the MoM implementation is the **Pocklington kernel integration**, specifically handling the singularity when `i == j` (self-impedance terms). Mitigation strategy:
- Use the **reduced kernel approximation** for thin wires
- Apply **Gaussian quadrature** with 16–32 points for stable integration
- `gonum` provides Gauss-Legendre weights out of the box

### Ground Plane Approaches
| Ground Type | Method |
|---|---|
| Free space | No modification needed |
| Perfect ground | Image theory (mirror currents) |
| Real ground | Sommerfeld integrals (more complex) |

---

## Starting Point

Begin with:
1. `backend/internal/mom/segment.go` — wire-to-segment subdivision data structures
2. `backend/cmd/server/main.go` — Gin server with stub endpoints
3. `frontend/` — Vite + React scaffold with Three.js canvas and wire table

Good luck with the build!
