# Antenna Studio — Architecture & Design Document

## 1. Executive Summary

Antenna Studio is a web-based antenna design and simulation tool built on the **Method of Moments (MoM)** electromagnetic solver. Users define wire antenna geometries through a visual 3D editor and tabular input, run simulations against a Go-based MoM solver, and visualize results as 3D radiation patterns, SWR curves, and impedance plots.

The system is a monorepo with two primary components:
- **Frontend**: React (Vite) SPA with Three.js for 3D visualization and Zustand for state management
- **Backend**: Go HTTP server (Gin) hosting a pure-Go MoM solver backed by gonum for linear algebra

---

## 2. System Architecture

### 2.1 High-Level Diagram

```
┌──────────────────────────────────────────────────────────┐
│                     Browser (SPA)                        │
│                                                          │
│  ┌──────────────┐  ┌───────────────┐  ┌──────────────┐  │
│  │ WireEditor   │  │ PatternViewer │  │ SWRChart     │  │
│  │ (Three.js)   │  │ (Three.js)    │  │ (Recharts)   │  │
│  └──────┬───────┘  └───────┬───────┘  └──────┬───────┘  │
│         │                  │                  │          │
│  ┌──────▼──────────────────▼──────────────────▼───────┐  │
│  │              Zustand Store                         │  │
│  │  - wires[], source, ground, frequency              │  │
│  │  - simulationResult, sweepResult                   │  │
│  │  - uiState (selected wire, view mode)              │  │
│  └──────────────────────┬─────────────────────────────┘  │
│                         │                                │
│  ┌──────────────────────▼─────────────────────────────┐  │
│  │              API Client (fetch)                     │  │
│  │  POST /api/simulate  |  POST /api/sweep            │  │
│  └──────────────────────┬─────────────────────────────┘  │
└─────────────────────────┼────────────────────────────────┘
                          │ HTTP/JSON
┌─────────────────────────▼────────────────────────────────┐
│                    Go Backend (Gin)                       │
│                                                          │
│  ┌─────────────┐    ┌────────────────────────────────┐   │
│  │ API Layer   │───▶│ MoM Solver Pipeline            │   │
│  │ (handlers,  │    │                                │   │
│  │  validation)│    │  Geometry ──▶ Z-Matrix ──▶ LU  │   │
│  └─────────────┘    │  ──▶ Currents ──▶ Far-Field    │   │
│                     └────────────────────────────────┘   │
│                              │                           │
│                     ┌────────▼───────┐                   │
│                     │ gonum (matrix  │                   │
│                     │ ops, LU decomp)│                   │
│                     └────────────────┘                   │
└──────────────────────────────────────────────────────────┘
```

### 2.2 Communication Protocol

All frontend-backend communication is **synchronous HTTP REST** (JSON request/response). WebSocket is reserved as a future option for long-running simulations with progress reporting.

| Aspect | Decision |
|---|---|
| Protocol | HTTP/1.1 (upgrade to HTTP/2 via reverse proxy) |
| Serialization | JSON |
| CORS | Backend allows frontend origin in development (`localhost:5173`) |
| Timeout | 30s default; frequency sweeps may take longer, so the sweep endpoint uses 120s |

---

## 3. Backend Architecture (Go)

### 3.1 Package Layout

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point: wires up Gin, config, starts server
├── internal/
│   ├── api/
│   │   ├── handlers.go          # HTTP handler functions (Simulate, Sweep)
│   │   ├── middleware.go         # CORS, request logging, recovery
│   │   ├── request.go           # Request DTOs + validation
│   │   └── response.go          # Response DTOs + serialization helpers
│   ├── geometry/
│   │   ├── wire.go              # Wire struct, validation (non-zero length, positive radius)
│   │   ├── ground.go            # Ground plane config (free-space, perfect, real)
│   │   └── templates.go         # Preset antenna geometries (dipole, Yagi, etc.)
│   ├── mom/
│   │   ├── segment.go           # Wire → segment subdivision (piecewise-linear basis)
│   │   ├── zmatrix.go           # N×N complex impedance matrix assembly
│   │   ├── green.go             # Free-space Green's function & Pocklington kernel
│   │   ├── quadrature.go        # Gauss-Legendre quadrature (wraps gonum)
│   │   ├── solver.go            # LU decomposition solve (Z·I = V)
│   │   ├── farfield.go          # Far-field E(θ,φ), gain, directivity
│   │   ├── ground_image.go      # Image theory for perfect ground plane
│   │   └── ground_sommerfeld.go # Sommerfeld integral for real ground (Phase 2)
│   └── config/
│       └── config.go            # Server config (port, CORS origins, solver defaults)
├── go.mod
└── go.sum
```

### 3.2 Solver Pipeline — Detailed Flow

```
                         Input: SimulateRequest
                                │
                    ┌───────────▼────────────┐
                    │  1. Validate Geometry   │
                    │  - Wire lengths > 0     │
                    │  - Radius > 0           │
                    │  - Segments ≥ 1 (odd)   │
                    │  - Source on valid seg   │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼────────────┐
                    │  2. Subdivide Wires     │
                    │  into Segments          │
                    │                         │
                    │  Each wire with N segs  │
                    │  → N segment structs    │
                    │  with center, endpoints,│
                    │  half-length, direction  │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼────────────┐
                    │  3. Build Z-Matrix      │
                    │  (N_total × N_total)    │
                    │                         │
                    │  For each (i,j) pair:   │
                    │  - Compute mutual Z via │
                    │    Pocklington kernel    │
                    │  - Use Gauss-Legendre   │
                    │    quadrature (16-32 pt) │
                    │  - Self-terms (i==j):   │
                    │    reduced kernel approx│
                    │                         │
                    │  If ground == perfect:  │
                    │  - Add image segment    │
                    │    contributions        │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼────────────┐
                    │  4. Build V Vector      │
                    │                         │
                    │  V = [0, 0, ..., Vs,    │
                    │       ..., 0]           │
                    │  Vs = source voltage at │
                    │  the designated feed    │
                    │  segment                │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼────────────┐
                    │  5. LU Solve: Z·I = V  │
                    │                         │
                    │  gonum/mat CDense LU    │
                    │  → complex current      │
                    │    vector I             │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼────────────┐
                    │  6. Compute Results     │
                    │                         │
                    │  a) Feed impedance:     │
                    │     Z_in = V_s / I_s    │
                    │                         │
                    │  b) SWR:                │
                    │     Γ = (Z-50)/(Z+50)   │
                    │     SWR=(1+|Γ|)/(1-|Γ|) │
                    │                         │
                    │  c) Far-field E(θ,φ):   │
                    │     Sum segment currents │
                    │     × phase factor      │
                    │     over (θ,φ) grid     │
                    │                         │
                    │  d) Gain (dBi):         │
                    │     4π|E|²_max /        │
                    │     ∫∫|E|² sinθ dθ dφ   │
                    └───────────┬─────────────┘
                                │
                         Output: SimulateResponse
```

### 3.3 Core Data Structures

```go
// geometry/wire.go
type Wire struct {
    X1, Y1, Z1 float64 // Start point (meters)
    X2, Y2, Z2 float64 // End point (meters)
    Radius      float64 // Wire radius (meters)
    Segments    int     // Number of segments (should be odd for center feed)
}

type GroundConfig struct {
    Type         string  // "free_space" | "perfect" | "real"
    Conductivity float64 // S/m (only for "real")
    Permittivity float64 // Relative (only for "real")
}

type Source struct {
    WireIndex    int     // Index into the wires array
    SegmentIndex int     // Segment on that wire (0-based)
    Voltage      complex128 // Typically 1+0i
}
```

```go
// mom/segment.go
type Segment struct {
    Index      int        // Global segment index
    WireIndex  int        // Which wire this came from
    Center     [3]float64 // Midpoint (x, y, z)
    Start      [3]float64 // Segment start endpoint
    End        [3]float64 // Segment end endpoint
    HalfLength float64    // Half the segment length (Δ/2)
    Direction  [3]float64 // Unit vector along segment
    Radius     float64    // Wire radius (inherited from wire)
}
```

```go
// mom/solver.go
type SolverResult struct {
    Currents    []CurrentEntry  // Per-segment current magnitude & phase
    Impedance   ComplexImpedance
    SWR         float64
    GainDBi     float64
    Pattern     []PatternPoint  // Far-field pattern samples
}

type CurrentEntry struct {
    SegmentIndex int
    Magnitude    float64
    PhaseDeg     float64
}

type ComplexImpedance struct {
    R float64 // Resistance (Ω)
    X float64 // Reactance (Ω)
}

type PatternPoint struct {
    ThetaDeg float64 // Elevation angle (0=zenith, 90=horizon)
    PhiDeg   float64 // Azimuth angle
    GainDB   float64 // Gain in dB relative to isotropic
}
```

### 3.4 Z-Matrix Assembly — Algorithm Detail

The impedance matrix is the computational core. For segments `i` and `j`:

```
Z[i][j] = (jωμ/4π) ∫∫ [ ŝᵢ·ŝⱼ G(R) - (1/k²)(∂²G/∂z²) ] ds' ds
```

Where:
- `G(R) = e^{-jkR} / R` is the free-space Green's function
- `R = |r - r'|` distance between observation and source points
- `k = 2πf/c` is the wavenumber
- `ŝᵢ, ŝⱼ` are unit direction vectors of segments i, j

**Implementation approach**:
1. Use the **thin-wire Pocklington kernel** (exact kernel is computationally expensive)
2. For the outer integral (observation segment `i`): use N-point Gauss-Legendre quadrature
3. For the inner integral (source segment `j`): use M-point Gauss-Legendre quadrature
4. **Self-terms** (`i == j`): replace `R` with `sqrt(R² + a²)` where `a` is wire radius (reduced kernel)
5. Quadrature order: 16 points for off-diagonal, 32 points for self-terms

**Parallelization**: The Z-matrix is symmetric (`Z[i][j] = Z[j][i]`), so only compute the upper triangle. Each row/element is independent — use `goroutines` with a worker pool (bounded to `runtime.NumCPU()`) to parallelize row computation.

### 3.5 Far-Field Computation

For each angular sample point `(θ, φ)`:

```
E(θ,φ) = Σᵢ Iᵢ · Δlᵢ · ŝᵢ × (ŝᵢ × r̂) · e^{jk(r̂·rᵢ)} · (-jωμ/4πr)
```

Simplification: compute on a unit sphere (`r = 1`, drop the `1/r` for pattern shape).

**Angular grid**: Default to 2° resolution → 91 θ values × 181 φ values = 16,471 points. Return as a flat array of `PatternPoint` structs for the frontend to render.

### 3.6 Frequency Sweep

The `/api/sweep` endpoint repeats the full solver pipeline for each frequency step:

```
for each freq in linspace(freq_start, freq_end, freq_steps):
    k = 2π·freq / c
    rebuild Z-matrix (frequency-dependent via k)
    solve Z·I = V
    compute impedance, SWR at this freq
```

**Optimization**: The geometry and segmentation are frequency-independent — only rebuild them once. The Z-matrix must be rebuilt at each frequency because the Green's function depends on `k`.

**Parallelization**: Each frequency point is independent. Use a goroutine worker pool to solve multiple frequencies concurrently. For 50 frequency steps on an 8-core machine, expect ~6x speedup.

### 3.7 Ground Plane Implementation

#### Phase 1: Perfect Ground (Image Theory)

For a perfect ground at `z = 0`:
- For every real segment at position `(x, y, z)`, create an image segment at `(x, y, -z)`
- Image currents are inverted for horizontal components, preserved for vertical
- Add image segment contributions to the Z-matrix (doubles the integration work, but no additional unknowns)

#### Phase 2: Real Ground (Sommerfeld Integrals)

Deferred to a later phase. Requires numerical evaluation of Sommerfeld integrals which involve oscillatory infinite integrals — significantly more complex. Consider using lookup tables or asymptotic approximations.

---

## 4. Frontend Architecture (React)

### 4.1 Component Tree

```
<App>
├── <Header>
│   ├── <ProjectName>
│   ├── <TemplateSelector>          # Dropdown: Dipole, Yagi, Vertical, Loop, Custom
│   └── <SimulateButton>            # Triggers POST /api/simulate
│
├── <MainLayout>                     # Resizable split panels
│   ├── <LeftPanel>
│   │   ├── <WireTable>             # Tabular wire geometry input
│   │   │   ├── <WireRow>           # One row per wire (x1,y1,z1,x2,y2,z2,radius,segs)
│   │   │   └── <AddWireButton>
│   │   ├── <SourceConfig>          # Feed point: wire index, segment, voltage
│   │   ├── <GroundConfig>          # Ground type selector + params
│   │   └── <FrequencyInput>        # Single freq or sweep range
│   │
│   └── <RightPanel>                 # Tabbed visualization area
│       ├── <Tab: 3D Editor>
│       │   └── <WireEditor>        # Three.js interactive 3D canvas
│       ├── <Tab: Radiation Pattern>
│       │   └── <PatternViewer>     # Three.js 3D pattern sphere
│       ├── <Tab: SWR Chart>
│       │   └── <SWRChart>          # Recharts line chart
│       ├── <Tab: Impedance>
│       │   └── <ImpedanceChart>    # R and X vs frequency
│       └── <Tab: Currents>
│           └── <CurrentDisplay>    # Segment current magnitudes
│
└── <StatusBar>                      # Simulation status, error messages
```

### 4.2 Zustand Store Design

```typescript
// store/antennaStore.ts

interface Wire {
  id: string;           // UUID for React keys
  x1: number; y1: number; z1: number;
  x2: number; y2: number; z2: number;
  radius: number;       // meters
  segments: number;     // integer, preferably odd
}

interface Source {
  wireIndex: number;
  segmentIndex: number;
  voltage: number;
}

interface GroundConfig {
  type: 'free_space' | 'perfect' | 'real';
  conductivity: number;
  permittivity: number;
}

interface FrequencyConfig {
  mode: 'single' | 'sweep';
  frequencyMhz: number;     // For single mode
  freqStart: number;        // For sweep mode
  freqEnd: number;
  freqSteps: number;
}

interface PatternPoint {
  theta: number;
  phi: number;
  gainDb: number;
}

interface SimulationResult {
  impedance: { r: number; x: number };
  swr: number;
  gainDbi: number;
  pattern: PatternPoint[];
  currents: { segment: number; magnitude: number; phase: number }[];
}

interface SweepResult {
  frequencies: number[];
  swr: number[];
  impedance: { r: number; x: number }[];
}

interface AntennaStore {
  // --- Geometry State ---
  wires: Wire[];
  source: Source;
  ground: GroundConfig;
  frequency: FrequencyConfig;

  // --- Results State ---
  simulationResult: SimulationResult | null;
  sweepResult: SweepResult | null;

  // --- UI State ---
  selectedWireId: string | null;
  isSimulating: boolean;
  error: string | null;

  // --- Actions ---
  addWire: (wire: Omit<Wire, 'id'>) => void;
  updateWire: (id: string, updates: Partial<Wire>) => void;
  removeWire: (id: string) => void;
  setSource: (source: Source) => void;
  setGround: (ground: GroundConfig) => void;
  setFrequency: (freq: FrequencyConfig) => void;
  selectWire: (id: string | null) => void;
  loadTemplate: (templateName: string) => void;
  runSimulation: () => Promise<void>;
  runSweep: () => Promise<void>;
}
```

### 4.3 Component Specifications

#### 4.3.1 WireEditor (Three.js 3D Canvas)

**Purpose**: Interactive 3D visualization and editing of wire antenna geometry.

**Rendering**:
- Each wire rendered as a `THREE.CylinderGeometry` (or `TubeGeometry` for curved paths) between its two endpoints
- Wire endpoints shown as small spheres (drag handles)
- Ground plane shown as a semi-transparent grid at `z = 0` when ground is not `free_space`
- Axis helper (X=red, Y=green, Z=blue) in corner
- Feed point indicated by a colored marker (e.g., red arrow) on the source segment

**Interaction**:
- Orbit controls (rotate, zoom, pan) via `OrbitControls`
- Click wire to select it (highlights in store, syncs with WireTable)
- Drag endpoints to move them (updates store, snaps to grid optionally)
- Right-click context menu: delete wire, set as feed point

**Camera**: Default isometric view. Buttons to snap to front/side/top views.

**Implementation**: Use `@react-three/fiber` and `@react-three/drei` for React-friendly Three.js integration.

#### 4.3.2 PatternViewer (3D Radiation Pattern)

**Purpose**: Visualize the 3D radiation pattern as a colored surface.

**Rendering**:
- Convert `PatternPoint[]` (θ, φ, gain_dB) to a 3D surface mesh
- For each (θ, φ): `r = gain_linear`, then spherical → Cartesian
- Color map: gain_dB mapped to a colorscale (jet/viridis) applied as vertex colors
- Wireframe overlay option for clarity
- Antenna geometry shown as thin lines at the center for reference

**Controls**:
- Orbit controls (rotate, zoom)
- Toggle between 3D surface and 2D polar cuts (E-plane, H-plane)
- Gain scale selector (dBi, dBd, linear)
- Max gain label displayed

#### 4.3.3 SWRChart (Recharts Line Chart)

**Purpose**: Plot SWR vs. frequency from sweep results.

**Features**:
- X-axis: frequency (MHz)
- Y-axis: SWR (linear scale, range 1.0 to max, clamp at 10)
- Horizontal reference line at SWR = 2.0 (dashed, labeled "2:1")
- Tooltip showing exact SWR and frequency on hover
- Responsive sizing

#### 4.3.4 ImpedanceChart

**Purpose**: Plot R (resistance) and X (reactance) vs. frequency.

**Features**:
- Dual Y-axis or overlaid traces: R in solid line, X in dashed line
- X-axis: frequency (MHz)
- Reference line at X = 0 (resonance indicator)
- Tooltip with R + jX formatted display

### 4.4 API Client Layer

```typescript
// api/client.ts

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

interface SimulateRequest {
  wires: WireDTO[];
  frequency_mhz: number;
  ground: GroundDTO;
  source: SourceDTO;
}

interface SweepRequest extends SimulateRequest {
  freq_start: number;
  freq_end: number;
  freq_steps: number;
}

export async function simulate(req: SimulateRequest): Promise<SimulationResult> {
  const res = await fetch(`${API_BASE}/api/simulate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Simulation failed');
  }
  return res.json();
}

export async function sweep(req: SweepRequest): Promise<SweepResult> {
  const res = await fetch(`${API_BASE}/api/sweep`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
    signal: AbortSignal.timeout(120_000), // 2 minute timeout for sweeps
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.error || 'Sweep failed');
  }
  return res.json();
}
```

---

## 5. API Contract

### 5.1 POST /api/simulate

Run a single-frequency MoM simulation.

**Request Body**:
```json
{
  "wires": [
    {
      "x1": 0, "y1": 0, "z1": 0,
      "x2": 0, "y2": 0, "z2": 1.0,
      "radius": 0.001,
      "segments": 11
    }
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

**Validation Rules**:
| Field | Rule |
|---|---|
| `wires` | Non-empty array, max 100 wires |
| `wires[].x1..z2` | Finite floats; start != end (non-zero length) |
| `wires[].radius` | > 0, < segment_length/4 (thin-wire approximation) |
| `wires[].segments` | Integer ≥ 1, ≤ 200 |
| `frequency_mhz` | > 0, ≤ 30000 (30 GHz practical limit) |
| `ground.type` | One of: `free_space`, `perfect`, `real` |
| `source.wire_index` | Valid index into wires array |
| `source.segment_index` | Valid index for that wire's segment count |

**Response Body** (200 OK):
```json
{
  "impedance": { "r": 73.1, "x": 42.5 },
  "swr": 2.1,
  "gain_dbi": 8.3,
  "pattern": [
    { "theta": 0, "phi": 0, "gain_db": 2.1 },
    { "theta": 2, "phi": 0, "gain_db": 2.3 }
  ],
  "currents": [
    { "segment": 0, "magnitude": 0.013, "phase": -12.3 },
    { "segment": 1, "magnitude": 0.019, "phase": -8.7 }
  ]
}
```

**Error Response** (400/500):
```json
{
  "error": "wire 0: radius exceeds thin-wire limit for given segment length"
}
```

### 5.2 POST /api/sweep

Run the solver across a frequency range.

**Request Body**: Same as `/simulate` plus:
```json
{
  "freq_start": 14.0,
  "freq_end": 14.35,
  "freq_steps": 50
}
```

**Additional Validation**:
| Field | Rule |
|---|---|
| `freq_start` | > 0 |
| `freq_end` | > `freq_start` |
| `freq_steps` | Integer 2–500 |

**Response Body** (200 OK):
```json
{
  "frequencies": [14.0, 14.007, 14.014],
  "swr": [1.8, 1.7, 1.65],
  "impedance": [
    { "r": 73.1, "x": 42.5 },
    { "r": 72.8, "x": 38.2 },
    { "r": 72.5, "x": 34.1 }
  ]
}
```

### 5.3 GET /api/templates

Return available antenna preset templates.

**Response Body** (200 OK):
```json
{
  "templates": [
    {
      "name": "Half-Wave Dipole",
      "description": "Center-fed half-wave dipole for given frequency",
      "parameters": [
        { "name": "frequency_mhz", "type": "number", "default": 14.0 }
      ]
    },
    {
      "name": "3-Element Yagi",
      "description": "3-element Yagi-Uda beam antenna",
      "parameters": [
        { "name": "frequency_mhz", "type": "number", "default": 14.0 },
        { "name": "boom_height_m", "type": "number", "default": 10.0 }
      ]
    }
  ]
}
```

### 5.4 POST /api/templates/{name}

Generate wire geometry from a template with given parameters.

**Response**: Returns the full wires/source/ground config to load into the editor.

---

## 6. Antenna Templates

Pre-built antenna geometries that auto-generate wires, source, and ground config.

| Template | Wires | Source | Default Ground |
|---|---|---|---|
| Half-Wave Dipole | 1 vertical wire, length = λ/2 | Center segment | Free space |
| Quarter-Wave Vertical | 1 vertical wire, length = λ/4 | Base segment | Perfect |
| 3-Element Yagi | 3 parallel wires (reflector, driven, director) | Center of driven | Free space |
| Inverted-V Dipole | 2 wires from apex angled down | Junction segment | Perfect |
| Full-Wave Loop | 4 wires forming a square, perimeter = λ | Middle of bottom wire | Free space |

**Template generation formula** (example — half-wave dipole):
```
λ = 300 / frequency_mhz  (meters)
wire_length = λ / 2
wire: (0, 0, -wire_length/2) → (0, 0, +wire_length/2)
segments: nearest odd number to (wire_length / (λ/20))
source: center segment
```

---

## 7. Project Structure

```
antenna-studio/
├── frontend/
│   ├── public/
│   ├── src/
│   │   ├── components/
│   │   │   ├── layout/
│   │   │   │   ├── Header.tsx
│   │   │   │   ├── MainLayout.tsx
│   │   │   │   └── StatusBar.tsx
│   │   │   ├── editor/
│   │   │   │   ├── WireEditor.tsx          # Three.js 3D canvas
│   │   │   │   ├── WireEditorControls.tsx  # View angle buttons, grid toggle
│   │   │   │   └── WireEndpointHandle.tsx  # Draggable endpoint sphere
│   │   │   ├── input/
│   │   │   │   ├── WireTable.tsx           # Wire geometry table
│   │   │   │   ├── WireRow.tsx             # Single wire row
│   │   │   │   ├── SourceConfig.tsx        # Feed point config
│   │   │   │   ├── GroundConfig.tsx        # Ground type/params
│   │   │   │   ├── FrequencyInput.tsx      # Frequency config
│   │   │   │   └── TemplateSelector.tsx    # Preset antenna selector
│   │   │   ├── results/
│   │   │   │   ├── PatternViewer.tsx       # 3D radiation pattern
│   │   │   │   ├── PatternViewer2D.tsx     # 2D polar cut views
│   │   │   │   ├── SWRChart.tsx            # SWR vs frequency
│   │   │   │   ├── ImpedanceChart.tsx      # R,X vs frequency
│   │   │   │   └── CurrentDisplay.tsx      # Segment currents table/viz
│   │   │   └── common/
│   │   │       ├── NumericInput.tsx        # Validated number input
│   │   │       └── ColorScale.tsx          # Gain colormap legend
│   │   ├── store/
│   │   │   └── antennaStore.ts             # Zustand store
│   │   ├── api/
│   │   │   └── client.ts                   # Backend API calls
│   │   ├── hooks/
│   │   │   ├── useSimulation.ts            # Simulation trigger + state
│   │   │   └── useThreeSetup.ts            # Shared Three.js scene setup
│   │   ├── utils/
│   │   │   ├── conversions.ts              # Spherical↔Cartesian, dB↔linear
│   │   │   └── validation.ts               # Client-side input validation
│   │   ├── types/
│   │   │   └── index.ts                    # Shared TypeScript interfaces
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── index.html
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
│
├── backend/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── api/
│   │   │   ├── handlers.go
│   │   │   ├── middleware.go
│   │   │   ├── request.go
│   │   │   └── response.go
│   │   ├── geometry/
│   │   │   ├── wire.go
│   │   │   ├── ground.go
│   │   │   └── templates.go
│   │   ├── mom/
│   │   │   ├── segment.go
│   │   │   ├── zmatrix.go
│   │   │   ├── green.go
│   │   │   ├── quadrature.go
│   │   │   ├── solver.go
│   │   │   ├── farfield.go
│   │   │   ├── ground_image.go
│   │   │   └── ground_sommerfeld.go
│   │   └── config/
│   │       └── config.go
│   ├── go.mod
│   └── go.sum
│
├── docker-compose.yml
├── Makefile
├── ARCHITECTURE.md
└── README.md
```

---

## 8. Build Order & Milestones

### Phase 1: Skeleton (Milestone: end-to-end data flow with mock data)

| Step | Task | Deliverable |
|---|---|---|
| 1.1 | Go backend scaffold | Gin server, `/api/simulate` returns hardcoded JSON |
| 1.2 | React frontend scaffold | Vite app, WireTable, calls stub API, displays raw JSON |
| 1.3 | Three.js WireEditor | Renders wires from store as 3D cylinders |
| 1.4 | Connect store to API | WireTable edits → store → API call → result displayed |

### Phase 2: Core Solver (Milestone: correct simulation for simple dipole)

| Step | Task | Deliverable |
|---|---|---|
| 2.1 | Segment subdivision | `segment.go` — wires subdivided, unit tests |
| 2.2 | Green's function | `green.go` — free-space Green's function, unit tests |
| 2.3 | Quadrature | `quadrature.go` — Gauss-Legendre wrapper, validated against known integrals |
| 2.4 | Z-matrix assembly | `zmatrix.go` — builds N×N complex matrix, validated for 1-wire case |
| 2.5 | LU solver | `solver.go` — gonum LU decomp, returns current vector |
| 2.6 | Feed impedance + SWR | Compute from I at feed segment, validate against known dipole (~73+j42 Ω) |

### Phase 3: Visualization (Milestone: 3D pattern + SWR chart working)

| Step | Task | Deliverable |
|---|---|---|
| 3.1 | Far-field calculation | `farfield.go` — E(θ,φ) grid, gain computation |
| 3.2 | PatternViewer | Three.js 3D radiation pattern surface |
| 3.3 | Frequency sweep | `/api/sweep` endpoint, loops solver over freq range |
| 3.4 | SWR Chart | Recharts SWR vs. frequency plot |
| 3.5 | Impedance Chart | Recharts R,X vs. frequency plot |

### Phase 4: Ground & Templates (Milestone: practical antenna modeling)

| Step | Task | Deliverable |
|---|---|---|
| 4.1 | Perfect ground (image theory) | `ground_image.go`, validated vertical antenna over ground |
| 4.2 | Antenna templates | Dipole, vertical, Yagi, loop presets |
| 4.3 | Template selector UI | Dropdown + parameter form, loads into editor |

### Phase 5: Polish (Milestone: release-ready)

| Step | Task | Deliverable |
|---|---|---|
| 5.1 | NEC2 export | Generate `.nec` deck file from current geometry |
| 5.2 | NEC2 import | Parse `.nec` file, load into editor |
| 5.3 | Save/load designs | JSON export/import of full antenna config |
| 5.4 | 2D polar pattern cuts | E-plane and H-plane polar plots |
| 5.5 | Current visualization | Color-coded current magnitude on wire segments |
| 5.6 | Docker packaging | `docker-compose.yml` for frontend + backend |

---

## 9. Validation & Testing Strategy

### 9.1 Backend Testing

**Unit tests** (per-package):
- `mom/segment_test.go` — verify segment count, positions, lengths for known wires
- `mom/green_test.go` — verify Green's function against analytical values at known distances
- `mom/quadrature_test.go` — integrate known functions, verify accuracy to 10⁻¹⁰
- `mom/zmatrix_test.go` — verify self-impedance of thin dipole matches King-Middleton formula
- `mom/solver_test.go` — solve simple 1-wire system, verify current symmetry
- `mom/farfield_test.go` — verify pattern of short dipole matches `sin²(θ)` shape

**Integration tests**:
- Full pipeline test: submit a half-wave dipole at 300 MHz (λ=1m), verify:
  - Feed impedance ≈ 73 + j42.5 Ω (±10%)
  - SWR ≈ 1.96 at 50Ω reference (±10%)
  - Gain ≈ 2.15 dBi (±0.5 dB)
  - Pattern null at θ=0° (along wire axis)
  - Pattern max at θ=90° (broadside)

**Benchmark tests**:
- Z-matrix assembly time for N = 50, 100, 200 segments
- Full solve time including far-field for typical antenna sizes

### 9.2 Frontend Testing

- Component tests (Vitest + React Testing Library) for WireTable, SourceConfig, GroundConfig
- Store tests: verify state transitions for add/update/remove wire actions
- API client tests: mock fetch, verify request shape and error handling
- Visual regression: Storybook stories for chart components (optional)

### 9.3 Reference Validation

Validate solver output against known NEC2 results for:
1. **Half-wave dipole** in free space
2. **Quarter-wave vertical** over perfect ground
3. **3-element Yagi** in free space

Acceptable error margin: ±5% on impedance, ±0.5 dB on gain, ±5% on SWR.

---

## 10. Performance Considerations

| Concern | Mitigation |
|---|---|
| Z-matrix is O(N²) to build | Parallelize with goroutine worker pool; exploit symmetry (compute upper triangle only) |
| LU decomposition is O(N³) | gonum uses optimized BLAS; for N < 500 segments, this is sub-second |
| Frequency sweep repeats full solve | Parallelize across frequencies; geometry subdivision done once |
| Far-field grid can be large | Default 2° resolution (16K points); allow user to select coarser grid |
| Frontend rendering large pattern mesh | Use `BufferGeometry` with vertex colors; avoid re-creating mesh on orbit |
| JSON payload size for pattern | ~16K points × 24 bytes ≈ 400KB; acceptable for HTTP; compress with gzip |

**Practical limits**: The system targets antennas with up to ~500 total segments. Beyond that, the O(N³) LU decomposition becomes the bottleneck. This covers most wire antennas including multi-element Yagis and loop antennas.

---

## 11. Deployment

### Development

```bash
# Terminal 1: Backend
cd backend && go run ./cmd/server

# Terminal 2: Frontend
cd frontend && npm run dev
```

Vite dev server proxies `/api/*` to `localhost:8080` (configure in `vite.config.ts`).

### Production (Docker Compose)

```yaml
# docker-compose.yml
services:
  backend:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - CORS_ORIGIN=http://localhost:3000

  frontend:
    build: ./frontend
    ports:
      - "3000:80"
    depends_on:
      - backend
    environment:
      - VITE_API_BASE=http://backend:8080
```

**Frontend Dockerfile**: Multi-stage build — `node` stage for `npm run build`, then `nginx` to serve static files.

**Backend Dockerfile**: Multi-stage build — `golang` stage for `go build`, then `scratch` or `alpine` for minimal runtime image.

---

## 12. Future Enhancements (Out of Scope for V1)

- **WebSocket progress**: Report % complete during long sweeps
- **Real ground (Sommerfeld)**: Full Sommerfeld integral evaluation for lossy ground
- **Wire loading**: Lumped loads (R, L, C) on segments
- **Transmission lines**: Model feedlines and matching networks
- **Optimization**: Auto-tune wire lengths/positions to minimize SWR
- **Multi-band sweep**: Discontinuous frequency ranges
- **NEC4 compatibility**: Extended thin-wire kernel, stepped-radius junctions
- **User accounts & persistence**: Save designs to a database
