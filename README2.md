# VE3KSM Antenna Studio — Feature List

A web-based antenna design and electromagnetic simulation platform powered by a full Method of Moments (MoM) solver. Designed for amateur radio operators, antenna engineers, and researchers who need to prototype, simulate, and analyze wire antenna structures before physical construction.

---

## Core Simulation Engine

### Method of Moments (MoM) Solver
- Full **Mixed Potential Integral Equation (MPIE)** formulation using vector and scalar potentials
- **Triangle (rooftop) basis functions** — piecewise-linear current distribution, default for stability and accuracy
- **Sinusoidal basis functions** (King-type) — higher-order accuracy with fewer unknowns
- **Quadratic (Hermite) basis functions** — smooth current representation for large continuous conductors
- **Gauss-Legendre numerical quadrature** (8–16 integration points) for accurate impedance matrix assembly
- **LU decomposition** solver (via gonum) for systems with ≤150 basis functions (direct, robust)
- **GMRES iterative solver** (restarted, 50 iterations) with diagonal preconditioning for large systems
- Automatic solver dispatch: LU for small/medium problems, GMRES for large arrays and Yagi-style beams
- **Thin-wire kernel** with reduced-kernel approximation for accurate self-term singularity handling
- Wire segmentation validation enforcing λ/10 minimum segment length and thin-wire radius limits

---

## Ground Plane Models

- **Free space** — no ground boundary, pure isotropic medium
- **Perfect electric conductor (PEC) ground** — image theory with unity reflection coefficients
- **Real lossy ground** — Fresnel reflection coefficients for both TM and TE polarization components
- **Complex-image method** (Bannister 1986) — captures Sommerfeld lateral-wave effects for antennas near the ground surface without full Sommerfeld integration

---

## Weather / Environment Loading

Global dielectric film applied uniformly to all wires, stacked on top of any per-wire coating using the multi-layer IS-card formula:

```
Z'_total = (jωμ₀/2π) · Σᵢ (1/εᵢ₋₁* − 1/εᵢ*) · ln(bᵢ / bᵢ₋₁),  ε₀* = 1
```

- Layer 1: per-wire coating (optional)
- Layer 2: global weather film (optional)
- Applied to all simulation modes (simulate, sweep, near-field, CMA, optimizer, Pareto, transient, convergence)
- Configured in the **Weather Panel** between the Ground Config and Frequency Input controls
- Film thickness is unit-converted with the rest of the geometry

**Weather presets:**

| Preset | εᵣ | tanδ | Default film |
|---|---|---|---|
| Dry *(default, inactive)* | — | — | 0 mm |
| Rain | 80.0 | 0.05 | 0.1 mm |
| Ice | 3.17 | 0.001 | 1.0 mm |
| Wet snow | 1.6 | 0.005 | 3.0 mm |

All four fields (preset, thickness, εᵣ, tanδ) are editable; selecting a preset fills the numeric fields as defaults. The panel header is accent-coloured when weather loading is active.

---

## Frequency Analysis

- **Single-frequency simulation** — full impedance, SWR, current distribution, and far-field pattern at one frequency
- **Linear frequency sweep** — uniform step from start to stop (2–500 points)
- **Logarithmic frequency sweep** — decade-spaced points ideal for broadband antennas
- **Interpolated fast sweep** — Asymptotic Waveform Evaluation (AWE) / vector-fitting achieves 10–50× speedup over full re-simulation at each frequency point

---

## Antenna Geometry & Components

### Wire Geometry
- Arbitrary 3D wire structures defined by start/end coordinates (X, Y, Z) and wire radius
- Per-wire segment count control for accuracy vs. speed trade-off
- Unit conversion in editor: meters, feet, inches, centimeters, millimeters
- Non-degenerate wire validation (non-zero length, positive radius, thin-wire limits)
- Interactive 3D wire editor with orbit, zoom, pan, and drag-to-move endpoints

### Lumped Loads
- Resistive (R), inductive (L), and capacitive (C) loads applied to any wire segment
- **Series** and **parallel** topologies (NEC-style LD card equivalent)
- Frequency-dependent impedance computed at each sweep point

### Transmission-Line Elements
- 2-port transmission-line elements between arbitrary wire segments
- Characteristic impedance, electrical length, and velocity factor configurable
- Loss factor support for realistic coaxial or ladder-line representation

### Conductor Materials
- Copper, aluminum, brass, steel, and stainless steel
- Frequency-dependent skin-effect loss calculated per material conductivity and permeability
- Lossless (perfect conductor) mode available

### Dielectric Wire Coatings
- Per-wire insulating shell modelled as distributed series impedance (IS-card formula):
  `Z'_coat = (jωμ₀/2π) · (1 − 1/εᵣ*) · ln(b/a)` where `a` = conductor radius, `b` = outer radius, `εᵣ* = εᵣ(1 − j·tanδ)`
- Three coating parameters per wire: **coating thickness** (m), **relative permittivity εᵣ**, **loss tangent tanδ**
- Applied in all simulation modes: single-frequency, sweep, near-field, CMA, optimizer, Pareto, transient, convergence
- **Coating preset dropdown** with 10 standard insulation materials (editable after selection):

| Preset | εᵣ | tanδ | Default thickness |
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

**Interpretation notes:**
- **Velocity factor.** A coated wire supports a slower guided wave; in the
  quasi-TEM limit the phase velocity in the coated region scales as
  `vf ≈ 1/√εᵣ_eff` where `εᵣ_eff` is the geometric mean over the layer
  stack. For a 2 mm PVC jacket (εᵣ = 2.3) the expected resonance shift of
  a short dipole is on the order of a few percent; the
  [coating_test.go](backend/internal/mom/coating_test.go) suite measures it.
- **Loss tangent convention.** `tanδ = ε''/ε'` is a dimensionless ratio
  (not a percentage). Typical values: PTFE ≈ 0.0002, PE ≈ 0.0002, XLPE ≈
  0.0003, PVC ≈ 0.05, water ≈ 0.2. The solver treats `ε* = εᵣ − j·εᵣ·tanδ`;
  any `tanδ > 0` contributes real resistance to the Z-matrix diagonal and
  is tracked in the power budget.
- **Geometric limit.** The MoM kernel assumes current flows along a thin
  filament. The validator rejects any wire whose coated outer radius
  (`a + thickness`, plus weather film if enabled) exceeds half the segment
  length, and the NEC-2 exporter collapses the coating to an effective
  radius using the lossless Tsai/Richmond relation
  `ln(a_eff) = ln(a) + Σ_i (1 − 1/εᵣ_i) · ln(b_i/b_{i−1})`. NEC-2 cannot
  represent lossy coatings; the exporter warns and drops the resistive
  term in that case.

### Voltage Source
- Single excitation port on any wire segment with configurable segment position
- Voltage magnitude settable for normalized or absolute current/field output

---

## Antenna Presets / Templates

Built-in parametric templates for rapid prototyping:

- **Half-wave dipole** — center-fed, configurable frequency and conductor radius
- **Quarter-wave vertical (monopole)** — over perfect ground, with ground-plane radials
- **3-element Yagi-Uda** — driven element, reflector, and director with adjustable spacing
- **Inverted-V dipole** — apex-fed with configurable apex height and arm angle
- **Full-wave loop antenna** — horizontal or vertical plane, configurable perimeter
- **Spiral antenna** — progressive radius geometry (parametric, future enhancement)

All templates accept frequency as the primary parameter and auto-scale all dimensions accordingly.

---

## Radiation Pattern & Far-Field Analysis

- **3D far-field radiation pattern** computed on a 2° × 2° angular grid (180 × 360 points)
- **Gain in dBi** — peak directivity relative to isotropic radiator
- **Directivity** — maximum directive gain independent of efficiency
- **Front-to-back ratio (F/B)** in dB — primary lobe vs. 180° back direction
- **3 dB beamwidth** — both E-plane and H-plane independently
- **Sidelobe level** — highest sidelobe relative to main beam in dB
- **Main-lobe azimuth and elevation** — angles of peak gain direction
- **Radiation efficiency** — ratio of radiated to input power accounting for material and load losses

---

## Impedance & Matching

### Feed-Point Impedance
- Complex impedance R + jX at the driven segment at any frequency
- **SWR** computed against a user-configurable reference impedance (default 50 Ω)
- **Reflection coefficient Γ** (magnitude and phase) for Smith-chart plotting

### Impedance Matching Network Synthesis
- **L-network** — two solutions (low-pass and high-pass) with Q-factor and 3 dB bandwidth
- **Pi-network (π-match)** — shunt-series-shunt topology, common for transmitter output networks
- **T-network** — series-shunt-series topology for high-impedance loads
- **Gamma-match** — shunt stub matching for Yagi driven elements
- **Beta-match (hairpin match)** — series transmission-line section matching
- **Toroidal transformer** — turns ratio calculation with core selection from T-37 through T-106 iron-powder and FT-series ferrite cores
- All topologies display exact component values AND nearest E12 standard values
- SVG schematic rendering in the UI
- **Off-resonance targeting** — "Design at" frequency input lets you specify an operating frequency independent of the antenna's resonant frequency; the tool runs a background simulation at the target frequency, reports Z_ant and SWR there, then synthesizes the matching network for that impedance

---

## Visualization Tools

### 3D Radiation Pattern Viewer
- Interactive Three.js mesh colored by gain level (dBi colormap)
- Orbit, zoom, and pan controls
- Toggleable gain color scale legend

### 2D Polar Cut Viewer
- Azimuth cut (constant elevation) and elevation cut (constant azimuth)
- User-selectable cut plane angle
- Overlaid reference circle at 0 dBi
- **Elevation cut rendered as a full 360° circle** — front lobe (peak azimuth φ) on the right half, back lobe (φ + 180°) on the left half, combined into one closed SVG path; spokes at 45° intervals labelled 0°–315°

### Smith Chart
- Normalized impedance locus plotted across the entire frequency sweep
- Constant-R and constant-X circles rendered
- Cursor readout of R, X, and Γ at any selected frequency point

### SWR Chart
- SWR vs. frequency with auto log-scale for dynamic range > 10:1
- Horizontal reference line at SWR = 1.5 and SWR = 2.0
- Minimum SWR frequency annotated

### Impedance Chart
- Real (R) and imaginary (X) parts plotted on independent Y-axes
- Dual-axis Recharts layout with color-coded series
- Resonance crossing (X = 0) highlighted

### Segment Current Distribution
- Per-segment current magnitude and phase at the simulated frequency
- Bar chart and tabular views
- Color-coded by magnitude

### Near-Field Viewer
- Electric (E) and magnetic (H) field strength on a user-defined rectangular observation grid
- Grid dimensions, resolution, and orientation (XY, XZ, YZ plane) configurable
- Color-mapped field magnitude overlay

### Polarization Viewer
- E-field polarization analysis at any far-field direction
- Linear, elliptical, and circular component decomposition
- Tilt angle and axial ratio display

### Characteristic Mode Analysis (CMA) Viewer
- Modal current distributions for each significant characteristic mode
- Eigenvalue spectrum showing modal resonances
- Radiation efficiency per mode

### Optimizer Results Viewer (Single-Objective)
- Convergence plot of objective function vs. iteration
- Best design parameters table
- Objective metrics: SWR minimization, gain maximization, impedance matching

### Pareto Front Viewer (Multi-Objective)
- Interactive scatter plot of trade-off frontier
- Two-axis selection of competing objectives (e.g., SWR vs. gain, gain vs. size)
- Hover to inspect individual Pareto-optimal design configurations

### Transient Analysis Viewer
- Time-domain current/voltage waveforms at selected segment
- Excitation waveform types: step, Gaussian pulse, sinusoidal burst
- Frequency-domain transform overlay

### Convergence Checker Viewer
- Segmentation refinement study across multiple segment densities
- Impedance and gain convergence curves vs. segment count
- Recommended minimum segment count highlighted

---

## Advanced Analysis Capabilities

### Characteristic Mode Analysis (CMA)
- Computes characteristic modes (eigenvectors) and eigenvalues of the MoM impedance matrix
- Identifies resonant frequencies and mode shapes without excitation
- Modal significance and radiation Q-factor per mode
- Essential for designing multi-band, MIMO, and electrically small antennas

### Single-Objective Optimizer
- **Nelder-Mead simplex** algorithm for gradient-free local optimization
- **Particle Swarm Optimization (PSO)** for global search
- Optimizable variables: wire coordinates, wire lengths, segment counts, load values
- Objectives: minimize SWR, maximize gain, match target impedance, minimize size

### Multi-Objective (Pareto) Optimizer
- Evolutionary multi-objective algorithm producing a Pareto-optimal front
- Supports simultaneous trade-off of two competing objectives
- Returns full set of non-dominated solutions for informed design decisions

### Transient Analysis
- Time-domain simulation via inverse Fourier transform of frequency-domain MoM results
- Excitation waveforms: Heaviside step, Gaussian pulse, arbitrary user-defined
- Visualizes waveform propagation and transient response at any wire segment

### Convergence Analysis
- Automatic segmentation refinement study (coarse → fine)
- Reports impedance and gain variation vs. number of segments per wavelength
- Flags under-resolved geometries and recommends convergence-validated segment density

---

## NEC-2 Interoperability

- **Import NEC-2 decks** — parses free-format `.nec` files including cards: CM, CE, GW, GS, GE, GN, EX, LD, TL, FR, EN
- **Export to NEC-2** — generates valid `.nec` deck from the current antenna configuration for use in external NEC-2-compatible tools (4NEC2, EZNEC, etc.)
- Bidirectional conversion between the internal MoM data model and NEC-2 card format

---

## Data Import / Export

- **Save design** — serialize full antenna configuration (wires, loads, transmission lines, source, ground, frequency) to JSON
- **Load design** — restore a previously saved JSON configuration
- **Export sweep data as CSV** — frequency, impedance (R+jX), SWR, gain at each sweep point; compatible with Excel, MATLAB, Python
- **NEC-2 deck export** — plaintext `.nec` file for cross-tool validation

---

## User Interface

### Layout
- Resizable split-panel layout: left panel for antenna design inputs, right panel for simulation results
- Tab-based results panel: Pattern 3D, SWR, Impedance, Currents, Smith Chart, Matching, Near-Field, Polarization, CMA, Optimizer, Pareto, Transient, Convergence
- Header with template selector, Simulate and Sweep action buttons, and Save/Load controls
- Status bar showing real-time impedance, SWR, and peak gain after each simulation

### Wire Editor
- Interactive Three.js 3D canvas with orbit (click-drag), zoom (scroll), and pan (right-drag) controls
- Click to select a wire; drag endpoints to reposition
- Context menu for wire operations (add, delete, duplicate)
- Color-coded wire rendering; feed wire highlighted distinctly

### Input Forms
- **Wire table** — spreadsheet-style coordinate entry with per-row unit conversion; columns include coating preset, coating thickness, εᵣ, and tanδ for per-wire insulation
- **Source configuration** — wire index, segment position, voltage magnitude
- **Ground configuration** — type selector, permittivity (εr), conductivity (σ) for real ground
- **Weather panel** — global dielectric film (preset, thickness, εᵣ, tanδ); accent-coloured header when active
- **Frequency input** — single frequency or sweep range with start/stop/step and mode selector
- **Load editor** — add/remove/edit R/L/C loads with series or parallel topology
- **Transmission-line editor** — add/remove/edit TL elements with Z₀, electrical length, velocity factor

### Validation & Warnings
- Non-blocking validation warnings displayed in a dismissible banner
- Warnings for: segment length > λ/10, wire radius approaching segment length, overlapping wires, open-ended feed
- Client-side validation runs before submission; server-side validation enforces numerical limits

---

## Deployment & Build

- **Single Go binary** at runtime — no Node.js, Vite, or nginx required after `make deps`
- **In-process TypeScript bundling** via esbuild Go library — frontend compiled inside the backend on startup
- **Dev mode** (`-dev` flag) — TypeScript re-bundled on every asset request for live editing
- **Production mode** — minified, cached bundle built once at startup
- **Docker Compose** — single-service container exposing port 8080
- **Makefile targets**: `deps`, `run`, `build`, `test`, `clean`, `docker-up`, `docker-down`
- Environment variables: `PORT` (default 8080), `CORS_ORIGINS` (comma-separated allowed origins)

---

## Testing

- **86+ unit tests** across all backend packages
- MoM solver validated against analytical references: half-wave dipole at 300 MHz (Z ≈ 73.1 + j42.5 Ω), short dipole sin²θ pattern, λ/4 monopole gain ≈ 5 dBi
- Geometry package: 24 tests covering wire validation, ground configuration, and all template generators
- API package: 13 tests covering request validation, DTO mapping, and response fields
- Solver package: 26 tests covering Green's function, quadrature, basis functions, Fresnel coefficients, GMRES, CMA
- Matching network package: topology synthesis and component value accuracy

---

## Technology Stack

| Layer | Technology |
|---|---|
| Backend language | Go 1.24 |
| HTTP framework | Gin 1.9 |
| Linear algebra | gonum 0.17 (LU, GMRES, complex math) |
| Frontend bundler | esbuild Go library 0.21 (in-process) |
| Frontend framework | React 18.3 (TypeScript) |
| 3D graphics | Three.js 0.163 + @react-three/fiber + drei |
| Charts | Recharts 2.12 |
| State management | Zustand 4.5 |
| Build tool (dev only) | Vite + Node.js 18 |
| Container | Docker Compose |
