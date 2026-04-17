# VE3KSM Antenna Studio — User Guide

**Version:** 1.0  
**Application:** VE3KSM Antenna Studio  
**URL (local):** http://localhost:8080

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Getting Started](#2-getting-started)
3. [Application Layout](#3-application-layout)
4. [Designing an Antenna](#4-designing-an-antenna)
   - 4.1 [Using a Preset Template](#41-using-a-preset-template)
   - 4.2 [Building a Wire Geometry by Hand](#42-building-a-wire-geometry-by-hand)
   - 4.3 [Using the 3D Wire Editor](#43-using-the-3d-wire-editor)
   - 4.4 [Setting the Voltage Source](#44-setting-the-voltage-source)
   - 4.5 [Configuring the Ground Plane](#45-configuring-the-ground-plane)
   - 4.6 [Adding Lumped Loads](#46-adding-lumped-loads)
   - 4.7 [Adding Transmission-Line Elements](#47-adding-transmission-line-elements)
   - 4.8 [Choosing a Conductor Material](#48-choosing-a-conductor-material)
5. [Running a Simulation](#5-running-a-simulation)
   - 5.1 [Single-Frequency Simulation](#51-single-frequency-simulation)
   - 5.2 [Frequency Sweep](#52-frequency-sweep)
6. [Interpreting Results](#6-interpreting-results)
   - 6.1 [Status Bar](#61-status-bar)
   - 6.2 [3D Radiation Pattern](#62-3d-radiation-pattern)
   - 6.3 [2D Polar Cuts](#63-2d-polar-cuts)
   - 6.4 [SWR Chart](#64-swr-chart)
   - 6.5 [Impedance Chart](#65-impedance-chart)
   - 6.6 [Segment Current Distribution](#66-segment-current-distribution)
   - 6.7 [Smith Chart](#67-smith-chart)
   - 6.8 [Near-Field Viewer](#68-near-field-viewer)
   - 6.9 [Polarization Viewer](#69-polarization-viewer)
7. [Impedance Matching Networks](#7-impedance-matching-networks)
8. [Advanced Analysis Tools](#8-advanced-analysis-tools)
   - 8.1 [Characteristic Mode Analysis (CMA)](#81-characteristic-mode-analysis-cma)
   - 8.2 [Single-Objective Optimizer](#82-single-objective-optimizer)
   - 8.3 [Multi-Objective (Pareto) Optimizer](#83-multi-objective-pareto-optimizer)
   - 8.4 [Transient Analysis](#84-transient-analysis)
   - 8.5 [Convergence Analysis](#85-convergence-analysis)
9. [Saving, Loading, and Exporting](#9-saving-loading-and-exporting)
   - 9.1 [Save and Load Designs](#91-save-and-load-designs)
   - 9.2 [Export Sweep Data as CSV](#92-export-sweep-data-as-csv)
   - 9.3 [NEC-2 Import and Export](#93-nec-2-import-and-export)
10. [Validation and Warnings](#10-validation-and-warnings)
11. [Reference: Units and Conventions](#11-reference-units-and-conventions)
12. [Reference: Keyboard & Mouse Controls](#12-reference-keyboard--mouse-controls)
13. [Troubleshooting](#13-troubleshooting)

---

## 1. Introduction

VE3KSM Antenna Studio is a browser-based antenna design and simulation tool. It uses the **Method of Moments (MoM)** electromagnetic solver to compute how a wire antenna radiates, how efficiently it accepts power from a feed, and how its performance changes across a range of frequencies.

You do not need to install any antenna software or run a separate solver binary. Everything runs inside the single web server process on your local machine, and results appear in interactive plots directly in your browser.

**What you can do with Antenna Studio:**

- Draw a wire antenna structure in 3D (or load a preset)
- Define a feed point, ground plane, lumped loads, and transmission lines
- Run a single-frequency simulation to get impedance, SWR, gain, and the radiation pattern
- Sweep across a frequency band to plot SWR, impedance, and Smith-chart loci
- Synthesize an impedance matching network
- Run Characteristic Mode Analysis, optimization, transient, and convergence studies
- Import and export NEC-2 antenna decks for cross-tool compatibility

---

## 2. Getting Started

### Prerequisites

- **Go 1.22+** installed on your system
- **Node.js 18+** with **npm** (needed once, for installing frontend dependencies)

### First-Time Setup

```bash
# 1. Clone or unzip the project
cd antenna-studio

# 2. Install frontend JavaScript dependencies (one time only)
make deps

# 3. Start the development server
make run
```

Then open your browser at **http://localhost:8080**.

The server will recompile the TypeScript frontend on every page load in `-dev` mode, so any edits to the source are immediately visible on refresh.

### Production Build

```bash
make build          # compiles to ./bin/antenna-studio
./bin/antenna-studio
```

The production binary bundles the frontend once at startup (minified), then serves it cached on every request.

### Docker

```bash
make docker-up      # builds image and starts container on port 8080
make docker-down    # stops the container
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | TCP port the server listens on |
| `CORS_ORIGINS` | `http://localhost:8080` | Comma-separated allowed CORS origins |

---

## 3. Application Layout

The Antenna Studio interface has three main zones:

```
┌─────────────────────────────────────────────────────────────────┐
│  HEADER: Template selector │ Simulate │ Sweep │ Save │ Load     │
├──────────────────────────┬──────────────────────────────────────┤
│                          │                                      │
│   LEFT PANEL             │   RIGHT PANEL                        │
│   Antenna Design Inputs  │   Simulation Results / Visualization │
│                          │                                      │
│  • Wire Table            │  Tabs:                               │
│  • 3D Wire Editor        │   Pattern 3D | SWR | Impedance |     │
│  • Source Config         │   Currents | Smith Chart |           │
│  • Ground Config         │   Matching | Near-Field |            │
│  • Frequency Input       │   Polarization | CMA |               │
│  • Load Editor           │   Optimizer | Pareto |               │
│  • TL Editor             │   Transient | Convergence            │
│                          │                                      │
├──────────────────────────┴──────────────────────────────────────┤
│  STATUS BAR: Z = R + jX Ω │ SWR x.x │ Gain x.x dBi │ warnings │
└─────────────────────────────────────────────────────────────────┘
```

The **divider** between the left and right panels is draggable — click and drag it horizontally to resize either panel.

---

## 4. Designing an Antenna

### 4.1 Using a Preset Template

The fastest way to get started is to load a built-in template.

1. Click the **Template** dropdown in the header.
2. Choose from: **Half-Wave Dipole**, **Quarter-Wave Vertical**, **3-Element Yagi**, **Inverted-V Dipole**, or **Full-Wave Loop**.
3. Each template prompts for a **design frequency** (in MHz). Enter your target frequency and confirm.
4. All wire dimensions are automatically scaled to the chosen frequency.

You can use a template as a starting point and then modify the wires manually.

### 4.2 Building a Wire Geometry by Hand

The **Wire Table** on the left panel lists every wire in the antenna. Each wire is defined by two endpoints in 3D space and a circular cross-section radius.

**To add a wire:**
1. Click **+ Add Wire** at the bottom of the wire table.
2. A new row appears with default values (a short wire along the Z-axis).
3. Click any cell in the row and type to edit the coordinates, radius, and segment count.

**Wire table columns:**

| Column | Description |
|---|---|
| Wire # | Auto-assigned index (used by Source and Load editors) |
| X₁, Y₁, Z₁ | Start-point coordinates |
| X₂, Y₂, Z₂ | End-point coordinates |
| Radius | Wire cross-section radius |
| Segments | Number of MoM segments to subdivide this wire into |
| Actions | Delete icon to remove the wire |

**Unit selector:** A unit dropdown above the table applies to all coordinate and radius inputs. Switch between **m**, **ft**, **in**, **cm**, and **mm**. Values are converted and retained when you change units.

**To delete a wire:** Click the trash icon at the right of any row.

**Segment count guidelines:**
- As a rule of thumb, use at least **10 segments per half-wavelength** of wire.
- The solver accepts finer segmentation for better accuracy at the cost of solve time.
- The Convergence tool (Section 8.5) helps you find the minimum acceptable count.

### 4.3 Using the 3D Wire Editor

The 3D canvas in the left panel gives a visual preview of your antenna geometry and lets you edit wires interactively.

**Navigation controls:**

| Action | Control |
|---|---|
| Orbit (rotate view) | Left-click and drag |
| Zoom | Scroll wheel |
| Pan | Right-click and drag |
| Reset view | Double-click empty space |

**Editing wires:**

- **Select a wire:** Click on it in the canvas. The corresponding row in the Wire Table highlights.
- **Move an endpoint:** Click to select a wire, then drag the endpoint sphere that appears at each end.
- **Add a wire:** Click **+ Add Wire** in the Wire Table; the new wire appears in the canvas.
- **Delete a wire:** Right-click a wire in the canvas to open the context menu, then choose Delete, or use the delete icon in the Wire Table.

The feed wire (the wire containing the voltage source) is drawn in a distinct color to distinguish it visually.

### 4.4 Setting the Voltage Source

There is one excitation port per simulation.

In the **Source Config** section:

1. **Wire index** — select which wire the source is applied to (matches Wire # in the table).
2. **Segment position** — which segment along that wire carries the feed (1 = first segment from start end; use the midpoint segment for a center-fed dipole).
3. **Voltage magnitude** — amplitude in volts (default 1 V). This affects absolute current magnitudes and radiated power; normalized quantities like gain and SWR are independent of this value.

### 4.5 Configuring the Ground Plane

In the **Ground Config** section:

**Ground type:**

| Type | Description |
|---|---|
| **Free space** | No ground. Use for antennas in the air, drones, aircraft. |
| **Perfect ground** | Ideal PEC ground at Z = 0. Fast; good for comparing designs. |
| **Real ground** | Lossy soil. Requires permittivity εᵣ and conductivity σ. |

**Typical soil parameters for Real Ground:**

| Ground type | εᵣ | σ (S/m) |
|---|---|---|
| Very dry soil / desert | 3 | 0.0001 |
| Average soil | 13 | 0.005 |
| Good / moist soil | 20 | 0.030 |
| Sea water | 80 | 4.000 |

For antennas above real ground, the **Complex-Image Method** is used automatically to model near-field interactions between the antenna and the ground surface.

### 4.6 Adding Lumped Loads

Loads allow you to simulate reactive tuning elements, ferrite chokes, or resistive losses at any point along the antenna.

In the **Load Editor**:

1. Click **+ Add Load**.
2. Set the **Wire index** and **Segment** where the load is applied.
3. Choose the **Topology**: Series or Parallel.
4. Enter values for **R** (Ω), **L** (µH), and/or **C** (pF). Leave unused components at 0.

At each simulation frequency, the load impedance Z_load = R + jωL + 1/(jωC) (series) or its parallel equivalent is inserted at the specified segment junction.

**Example use cases:**
- A series 50 Ω resistor to simulate a balun insertion loss
- A parallel LC trap to create a band-stop notch
- An inductive loading coil to electrically lengthen a short antenna

### 4.7 Adding Transmission-Line Elements

Transmission lines (coaxial cables, ladder lines) can be inserted between two segments as two-port network elements.

In the **TL Editor**:

1. Click **+ Add TL**.
2. Set the **Port 1 wire/segment** and **Port 2 wire/segment**.
3. Enter **Z₀** (characteristic impedance in Ω), **electrical length** (degrees at the simulation frequency), **velocity factor** (0 < vf ≤ 1.0), and an optional **loss** value (dB/100 ft or dB/m).

Transmission lines are useful for modeling feed cables, phasing harnesses in phased arrays, and hairpin matching stubs.

### 4.8 Choosing a Conductor Material

Select the wire conductor material from the **Material** dropdown (if shown). Options include:

- **Perfect conductor** (lossless, default for most studies)
- **Copper** — highest conductivity; use for coil and dipole modeling
- **Aluminum** — common for Yagi elements
- **Brass**, **Steel**, **Stainless Steel** — for exotic or structural conductors

The solver applies frequency-dependent skin-effect resistance to each segment when a lossy material is selected, reducing radiation efficiency and affecting feed impedance slightly at high frequencies.

---

## 5. Running a Simulation

### 5.1 Single-Frequency Simulation

1. In the **Frequency Input** section, enter a single frequency in **MHz**.
2. Make sure the mode is set to **Single**.
3. Click the **Simulate** button in the header (or press the keyboard shortcut — see Section 12).

Results appear within 1–3 seconds for typical antenna sizes. After completion:

- The **Status Bar** updates with Z, SWR, and peak gain.
- The **3D Pattern**, **Currents**, and all result tabs populate.
- Any validation warnings appear in a yellow banner below the header.

### 5.2 Frequency Sweep

1. In the **Frequency Input** section, set mode to **Sweep**.
2. Enter **Start frequency**, **Stop frequency** (both in MHz), and **number of points** (2–500).
3. Choose the sweep **mode**:
   - **Linear** — equal spacing in frequency
   - **Logarithmic** — equal spacing in log(f), better for wideband antennas
   - **Interpolated (fast)** — Asymptotic Waveform Evaluation for 10–50× speedup; very accurate for smooth, resonant systems
4. Click the **Sweep** button in the header.

After completion the **SWR**, **Impedance**, and **Smith Chart** tabs populate with sweep data. The **3D Pattern** and **Currents** tabs show results at the center frequency.

**Sweep performance guidance:**

| Number of segments | Recommended mode | Points | Typical time |
|---|---|---|---|
| < 50 | Any | Up to 500 | < 5 s |
| 50 – 150 | Linear or Interpolated | Up to 200 | 5 – 30 s |
| > 150 | Interpolated | 50 – 100 | 10 – 60 s |

---

## 6. Interpreting Results

### 6.1 Status Bar

The status bar at the bottom of the window shows a quick summary after every simulation:

- **Z** — feed-point impedance as R + jX Ω
- **SWR** — standing wave ratio at the configured reference impedance (default 50 Ω)
- **Gain** — peak gain in dBi
- **Warnings** — count of active validation issues (click to expand)

### 6.2 3D Radiation Pattern

The **Pattern 3D** tab shows the antenna's far-field radiation pattern as a 3D surface mesh. The surface radius in each direction is proportional to the field intensity; the color indicates the gain in dBi using a rainbow colormap (blue = low, red = high).

**Navigation:** Same orbit/zoom/pan controls as the 3D Wire Editor.

**Color scale:** The gain colorbar on the right shows the dBi range. The peak gain value is labeled at the top.

**Reading the pattern:**
- A prolate (elongated) surface indicates directional radiation.
- A sphere-like surface indicates near-isotropic radiation.
- The pattern is plotted in the antenna's coordinate frame: Z is vertical (up), X is broadside.

The **Metrics Panel** (visible below or alongside the pattern) lists: directivity, gain, efficiency, front-to-back ratio, 3 dB beamwidth (E/H plane), and sidelobe level.

### 6.3 2D Polar Cuts

The **Polar Cut** tab plots a 2D slice of the 3D pattern.

Controls:

- **Plane** — Azimuth (horizontal, constant elevation) or Elevation (vertical, constant azimuth)
- **Cut angle** — the fixed angle of the complementary plane (e.g., elevation = 0° for the azimuth cut in the horizontal plane)

The outer circle represents the peak gain level. A gray reference circle at 0 dBi is drawn for quick absolute reference. Gain values below −30 dB relative to the peak are truncated.

### 6.4 SWR Chart

The **SWR** tab plots Standing Wave Ratio vs. frequency across the sweep range.

- Horizontal axis: frequency (MHz)
- Vertical axis: SWR (linear; auto-switches to log if range > 10:1)
- Reference lines at SWR = 1.5 (green dashed) and SWR = 2.0 (orange dashed)
- The frequency of minimum SWR is annotated on the plot

**What to look for:** A SWR < 2.0 over your operating band means the antenna can be fed directly with a 50 Ω coaxial cable and a typical transceiver without a tuner.

### 6.5 Impedance Chart

The **Impedance** tab plots R (resistance) and X (reactance) vs. frequency on dual Y-axes.

- Blue series (left axis): R in Ω
- Red series (right axis): X in Ω
- A horizontal dashed line marks X = 0 (resonance)

**What to look for:** At resonance X crosses zero and R is the radiation resistance. For a half-wave dipole in free space, R ≈ 73 Ω at resonance.

### 6.6 Segment Current Distribution

The **Currents** tab shows the current magnitude and phase at every MoM segment at the simulated frequency.

**Bar chart view:** Each bar represents one segment. Height = |I| in milliamperes (normalized to the source segment). Color = phase in degrees.

**Table view:** Columns for segment number, wire index, current magnitude (mA), and phase (degrees). Click a column header to sort.

**What to look for:** The current should peak at or near the feed segment and fall off symmetrically toward the wire ends for a simple dipole. Lumped loads should show visible current discontinuities at the load location.

### 6.7 Smith Chart

The **Smith Chart** tab plots the normalized impedance locus across the sweep.

- The Smith chart is normalized to the reference impedance (default 50 Ω).
- Each point on the locus corresponds to one frequency in the sweep; hover to read frequency, R, X, and |Γ|.
- The center of the chart (R = 1, X = 0 normalized) represents a perfect 50 Ω match.

Use the Smith chart to design matching networks: a capacitive shunt or series inductor rotates the locus toward the chart center.

### 6.8 Near-Field Viewer

The **Near-Field** tab computes the electric (E) or magnetic (H) field strength on an observation grid you define.

**Grid settings:**

| Setting | Description |
|---|---|
| Plane | XY, XZ, or YZ |
| Center | Grid center coordinates (m) |
| Width / Height | Physical dimensions of the grid (m) |
| Resolution | Grid points per side (max 50 × 50) |
| Field | E-field or H-field |

Click **Compute Near Field** to run the calculation. The result is shown as a color-mapped heatmap. Units are V/m (E-field) or A/m (H-field).

**Typical use:** Compute the E-field in the XZ plane close to a base-fed vertical to verify a null at the feed point, or compute radiation directly above a ground-mounted antenna.

### 6.9 Polarization Viewer

The **Polarization** tab shows the E-field polarization at a selected far-field direction (θ, φ).

- **Linear polarization** — the E-field oscillates along a fixed line. Tilt angle shown.
- **Elliptical polarization** — the E-field traces an ellipse. Axial ratio and tilt shown.
- **Circular polarization (CP)** — axial ratio = 0 dB (1:1); used for satellite work.

The polarization ellipse is drawn on a unit circle. Left- and right-hand circular components are decomposed.

---

## 7. Impedance Matching Networks

After a simulation, go to the **Matching** tab to synthesize a lumped-element network that transforms the antenna feed-point impedance to 50 Ω (or another system impedance).

**Steps:**

1. The antenna's simulated impedance Z_ant = R + jX is filled in automatically from the last single-frequency result.
2. Set the **System impedance** (default 50 Ω) and **frequency** (MHz).
3. Choose a **Topology**:

| Topology | Best for |
|---|---|
| **L-network** | Simple two-component match; two solutions (LP and HP) |
| **Pi-network** | Transmitter output matching; controls Q and bandwidth |
| **T-network** | High-impedance loads; controls Q |
| **Gamma-match** | Driven-element matching in Yagi antennas |
| **Beta-match (hairpin)** | Simple Yagi or loop matching without galvanic connection |
| **Toroidal transformer** | Wideband balun/unun; isolation between feed and antenna |

4. Click **Calculate**. The tool displays:
   - Exact component values (L in µH, C in pF, R in Ω)
   - Nearest **E12 standard** component values
   - Network Q-factor and estimated 3 dB bandwidth
   - ASCII schematic of the network

5. For the **Toroidal transformer**, additional outputs include: turns ratio, recommended core (from T-37 through FT-240 ferrite / iron-powder selection chart), and primary/secondary turns count.

**Tips:**
- The L-network solver returns two solutions (low-pass and high-pass). The LP solution (capacitor across source) is typical for HF use.
- Use the Pi-network when you need to set the operating Q independently of the impedance ratio.
- For an X ≠ 0 load, the tool adds a series compensation reactance as part of the design.

---

## 8. Advanced Analysis Tools

### 8.1 Characteristic Mode Analysis (CMA)

**CMA** computes the natural resonant modes of the antenna structure without any excitation. It answers the question: *what shapes does the current naturally want to take on this conductor?*

**How to run:**
1. Go to the **CMA** tab in the results panel.
2. Set the analysis frequency (MHz).
3. Click **Run CMA**.

**Results displayed:**

- **Eigenvalue spectrum** — a bar chart of eigenvalue magnitudes. Modes near eigenvalue = 0 are at resonance.
- **Modal significance** — normalized 0–1 scale showing how effectively each mode radiates.
- **Current distributions** — select any mode number to visualize its current pattern on the antenna.
- **Radiation Q** per mode (lower = broader bandwidth).

**Practical use:** CMA is essential for designing multi-band antennas (identify modes that resonate at each band), MIMO antennas (ensure orthogonal modal currents), and electrically small antennas (find the best excitation position for a given mode).

### 8.2 Single-Objective Optimizer

The optimizer automatically varies wire dimensions or component values to minimize (or maximize) a single performance metric.

**How to run:**
1. Go to the **Optimizer** tab.
2. Choose the **objective**: Minimize SWR, Maximize Gain, or Match target impedance Z_target.
3. Select the **design variables**: checkboxes next to wire coordinates, wire lengths, or load values. Set min/max bounds for each variable.
4. Choose the **algorithm**: Nelder-Mead (faster, local) or Particle Swarm (slower, global).
5. Set the **maximum iterations** (50–500).
6. Click **Run Optimizer**.

**Results:**
- A convergence plot showing objective value vs. iteration
- Best parameter values found
- Final impedance, SWR, and gain at the optimized design

**Tips:**
- Use Nelder-Mead first for a quick local minimum. If the result is unsatisfying, switch to Particle Swarm for a global search.
- Constrain variables to physically reasonable ranges (e.g., wire length ± 20% of initial) to speed convergence.
- Optimizing for SWR = 1.0 at a single frequency may result in a very narrow bandwidth. Use the Pareto optimizer to balance SWR with bandwidth.

### 8.3 Multi-Objective (Pareto) Optimizer

The Pareto optimizer finds the set of *non-dominated* solutions — the trade-off frontier between two competing objectives.

**How to run:**
1. Go to the **Pareto** tab.
2. Choose **Objective 1** and **Objective 2** from the available metrics (SWR, Gain, Bandwidth, Size/length).
3. Define design variables and bounds (same as single-objective).
4. Set the population size and generations.
5. Click **Run Pareto Optimization**.

**Results:**
- An interactive scatter plot of the Pareto front
- Each point is a design that is optimal for its trade-off position
- Hover over any point to see its wire parameters and both objective values
- Click a point to load that design into the main wire editor

**Practical use:** Use the Pareto optimizer to explore the trade-off between gain and SWR — you might discover that accepting SWR = 1.4 instead of SWR = 1.1 buys you 1.5 dB of additional gain.

### 8.4 Transient Analysis

Transient analysis shows how the antenna responds in the time domain to a fast-switching signal.

**How to run:**
1. Go to the **Transient** tab.
2. Choose an **excitation waveform**: Step, Gaussian pulse, or Sinusoidal burst.
3. Set the pulse width / rise time and amplitude.
4. Select the observation point (wire index and segment).
5. Click **Run Transient**.

The computation applies an inverse FFT to the frequency-domain MoM results across a wide bandwidth to reconstruct the time-domain waveform.

**Results:**
- Time-domain current (or voltage) waveform at the observation segment
- Overlay of the excitation waveform for reference
- Frequency-domain spectrum of the response

**Practical use:** Identify ringing, standing-wave echoes on feed cables, and group delay across the operating band.

### 8.5 Convergence Analysis

The convergence checker runs the same simulation repeatedly with increasing segment density and reports how the results stabilize.

**How to run:**
1. Go to the **Convergence** tab.
2. Set the frequency (MHz).
3. Set the range of segment counts to test (e.g., 5, 10, 15, 20, 25 segments per wire).
4. Click **Run Convergence Study**.

**Results:**
- Impedance R and X vs. segment count (should plateau at the converged value)
- Peak gain vs. segment count
- A green marker at the recommended minimum segment count (where changes drop below 1%)

**Rule of thumb:** For most HF dipoles, 10 segments per wire is sufficient. For antennas with current singularities (loaded shorts, feeds near junctions), 20+ segments improve accuracy.

---

## 9. Saving, Loading, and Exporting

### 9.1 Save and Load Designs

**Save:** Click the **Save** button in the header. The browser downloads a `.json` file containing the complete antenna configuration: all wires, source, loads, transmission lines, ground settings, and frequency range.

**Load:** Click the **Load** button, browse to a previously saved `.json` file, and confirm. The entire workspace is replaced with the saved configuration.

The JSON format is human-readable and can be version-controlled with git.

### 9.2 Export Sweep Data as CSV

After running a frequency sweep, the **SWR** and **Impedance** tabs each have an **Export CSV** button. The exported file contains one row per frequency point with columns: `frequency_MHz`, `R_ohm`, `X_ohm`, `SWR`, `gain_dBi`.

This data can be imported into Excel, MATLAB, Python/pandas, or any plotting tool for post-processing.

### 9.3 NEC-2 Import and Export

NEC-2 is the most widely used antenna simulation file format. Antenna Studio can read and write `.nec` deck files for compatibility with tools like 4NEC2 and EZNEC.

**Export to NEC-2:**
1. Click **Export NEC-2** in the header (or File menu).
2. The browser downloads a `.nec` text file with GW (wire) cards, EX (excitation) card, GN (ground) card, LD (load) cards, TL (transmission line) cards, and FR (frequency) cards matching your current design.

**Import from NEC-2:**
1. Click **Import NEC-2** in the header.
2. Browse to your `.nec` file.
3. Antenna Studio parses the deck and loads wires, source, ground, loads, and frequency into the editor.

**Supported NEC-2 cards on import:** CM, CE (comments), GW (wire), GS (scale), GE (ground end), GN (ground), EX (excitation), LD (load), TL (transmission line), FR (frequency), EN (end).

---

## 10. Validation and Warnings

Antenna Studio validates your design before submission and reports non-blocking warnings in a yellow banner beneath the header.

**Common warnings and their meaning:**

| Warning | Cause | Fix |
|---|---|---|
| Segment too long (> λ/10) | Segment length exceeds 10% of the wavelength. MoM accuracy degrades. | Increase the segment count on that wire. |
| Wire radius too large | Radius approaches segment length (thin-wire assumption violated). | Use a shorter segment count or reduce radius. |
| Open-ended feed | Feed segment is at the very end of a wire instead of an interior segment. | Move the source to a middle segment of the driven wire. |
| Overlapping wires | Two wires share the same start or end coordinates without a shared junction. | Snap endpoints together or delete the duplicate. |
| No source defined | No voltage source has been placed. | Set a wire and segment in the Source Config section. |

Warnings do not prevent simulation — click **Simulate** anyway to proceed. Errors (shown in red) do prevent simulation until resolved.

---

## 11. Reference: Units and Conventions

| Quantity | Unit | Notes |
|---|---|---|
| Coordinates | m (default) | Switchable to ft, in, cm, mm in Wire Table |
| Frequency | MHz | All frequency inputs |
| Impedance | Ω | R + jX displayed |
| SWR | dimensionless | Computed at reference Z₀ (default 50 Ω) |
| Gain | dBi | Relative to isotropic radiator |
| Current | mA | Normalized to feed segment current = 1 A |
| E-field | V/m | Near-field only |
| H-field | A/m | Near-field only |
| Inductance | µH | Load editor |
| Capacitance | pF | Load editor |
| Resistance | Ω | Load editor |

**Coordinate system:**
- X — broadside direction (antenna boresight for a horizontal dipole along Y)
- Y — along the wire axis for a dipole
- Z — vertical (up); ground plane is the Z = 0 plane

**Phase convention:** e^(+jωt) time-harmonic convention (positive imaginary part = inductive).

---

## 12. Reference: Keyboard & Mouse Controls

### 3D Canvas (Wire Editor and Pattern Viewer)

| Action | Control |
|---|---|
| Orbit / rotate | Left-click drag |
| Zoom in/out | Scroll wheel |
| Pan | Right-click drag |
| Select wire | Left-click on wire |
| Context menu | Right-click on wire |
| Reset camera | Double-click empty space |

### General UI

| Action | Control |
|---|---|
| Simulate | Header **Simulate** button |
| Sweep | Header **Sweep** button |
| Save design | Header **Save** button |
| Load design | Header **Load** button |
| Resize panels | Drag the vertical divider |
| Switch result tab | Click tab labels in right panel |

---

## 13. Troubleshooting

**The page loads but the canvas is blank.**  
Make sure WebGL is enabled in your browser. Open Developer Tools → Console; look for WebGL errors. On integrated Intel graphics you may need to disable hardware acceleration in browser settings.

**"make deps" fails.**  
Ensure Node.js 18+ is installed (`node --version`). If npm produces a permissions error, try `npm install --prefix frontend` from the project root instead.

**Simulation returns NaN or infinite SWR.**  
This usually means the impedance matrix is singular. Common causes: duplicate wires occupying the same space, a wire of zero length, or a segment count of 0. Check the wire table for degenerate entries.

**The sweep is extremely slow.**  
Switch to **Interpolated** sweep mode (AWE) for long sweeps with many points. Also reduce the number of segments per wire if accuracy is not critical.

**Matching network shows negative component values.**  
Negative component values indicate the load impedance cannot be matched with the chosen topology at this frequency without adding a pre-compensation element. Try a different topology (T or Pi instead of L), or simulate at a frequency where X is closer to zero.

**NEC-2 import fails to load my file.**  
Antenna Studio supports the free-format NEC-2 deck subset. Extended NEC-4 cards (PQ, KH, etc.) are silently ignored. If the file uses GH (helix) or SP (surface patch) cards, those geometries will not import — wire (GW) geometries only.

**The Docker container exits immediately.**  
Run `make deps` on the host before building the Docker image. The image build needs `frontend/node_modules/` present (or let the Dockerfile run npm install — check the Dockerfile for the build stage).

---

*VE3KSM Antenna Studio is open-source software. For bugs, feature requests, and contributions, see the project repository.*
