# VE3KSM Antenna Studio Roadmap

A research-informed enhancement plan, ordered by ROI (user value per unit
of implementation effort).  Items are derived from a survey of recent
(2020–2026) wire-MoM literature and a feature-gap analysis against
EZNEC, MMANA-GAL, 4nec2, xnec2c, and AN-SOF.

Each item lists effort (Low / Medium / High), the primary files to
touch, and the user-visible payoff.  Status is updated as work lands.

---

## Week 1 — Quick wins (low effort, high payoff)

### 1. Lumped loads on segments — *Status: shipped*

R / L / C in series and parallel topologies, applied to any segment of
any wire.  The single biggest functional gap versus EZNEC/4nec2;
unlocks traps, loading coils, resistive terminations, hat capacitors,
folded-dipole stubs, lumped 4:1 baluns.

- **Effort:** Low (1–2 days)
- **Backend:** new `Load` struct in `backend/internal/mom/types.go`;
  `Z_load(ω)` injected onto Z-matrix diagonal in `mom/zmatrix.go`;
  request DTO + handler plumbing.
- **Frontend:** new `LoadConfig` panel; rendering in current/
  geometry views.

### 2. Smith chart data + arbitrary-Z₀ VSWR — *Status: shipped*

Hardcoded 50 Ω today.  Make the reference impedance a per-request
parameter and return the complex reflection coefficient `S11` so the
frontend can plot a Smith chart and report VSWR at user Z₀.

- **Effort:** Low (half day backend, 1 day frontend)
- **Backend:** add `reference_impedance` to simulate/sweep request
  DTOs; compute Γ = (Z − Z₀)/(Z + Z₀), return as `{re, im}`.
- **Frontend:** Smith chart canvas component (SVG with constant-R and
  constant-X arcs); add to results pane.

### 3. Far-field metrics + 2D polar cuts — *Status: shipped*

The 3D pattern is computed but only directivity is surfaced.  Add
front-to-back ratio, 3 dB beamwidth (E and H planes), main-lobe
azimuth/elevation, sidelobe level, total radiated efficiency, and a
2D polar cut (azimuth at fixed elevation, or elevation at fixed
azimuth) — what users want for day-to-day work.

- **Effort:** Low (1 day total)
- **Backend:** post-process pass over the existing far-field grid in
  `mom/farfield.go`; return new fields in `SimulationResult`.
- **Frontend:** numeric metrics in results header; new polar plot
  component (SVG or Recharts radial bar variant).

### 4. Conductor materials + skin-effect loss — *Status: shipped*

Wires currently carry only a radius (loss-free).  Add a material
library (Cu, Al, brass, steel, stainless) and apply the surface
impedance correction `Z_s = (1 + j) / (σ δ)` with `δ = 1/√(πfμσ)` to
the Z-matrix diagonal.  Turns the solver into something that matches
measured Q on loaded / electrically-small antennas.

- **Effort:** Low–Medium (1–2 days)
- **Backend:** new `material` field on `Wire`; lookup table; assembly
  changes in `mom/zmatrix.go`.
- **Frontend:** material dropdown in wire editor; default = Cu.

### 5. Segmentation validator — *Status: shipped*

Pre-flight checks that warn (don't block) when a wire violates known
thin-wire-MoM accuracy rules:

- segment length > λ/10 (λ/20 ideal, λ/10 minimum),
- segment_length / radius < 2 (kernel validity limit),
- adjacent segment lengths differ by > 2× (taper too aggressive),
- wire endpoints share radius when junctioned.

Each warning carries a severity, a pointer to the offending wire/
segment, and a link to a remediation note.

- **Effort:** Low (half day)
- **Backend:** new `mom.Validate(geom, freq) []Warning`; called from
  `/api/simulate` and returned alongside results.
- **Frontend:** non-blocking warning banner above results.

---

## Week 2 — Solver and modeling depth

### 6. Transmission-line elements — *Status: shipped*

NEC-style 2-port TL elements stamped as a 2×2 Z-parameter block onto
the Z-matrix.  Connects two basis functions (or one basis + a short /
open termination for stubs).  Lossy via dB/m attenuation; velocity
factor for non-air dielectric.  Frontend has a dedicated TLEditor.

- **Effort:** Medium (shipped)

### 7. Move matching-network synthesis to the backend — *Status: shipped*

A frontend calculator already exists (`frontend/src/utils/matching.ts`,
`components/results/MatchingNetwork.tsx`).  Move the math to Go,
expose it via `/api/match`, and add π-match, T-match, gamma-match,
and beta-match in addition to the existing L-match.

- **Effort:** Medium

### 8. AWE / vector-fitting for frequency sweeps — *Status: shipped*

Rebuilding Z from scratch at every frequency in a 201-point sweep is
the dominant runtime cost.  Asymptotic Waveform Evaluation (or simple
rational fitting) over a small set of anchor frequencies cuts sweep
time 10–50×.

- **Effort:** Medium

---

## Week 3–4 — Numerics and ground

### 9. GMRES + simple preconditioning — *Status: shipped*

Restarted GMRES(50) with diagonal (Jacobi) preconditioning, working
directly on the complex Z-matrix (no 2N real doubling).  Auto-dispatch:
N ≤ 150 bases → LU, N > 150 → GMRES with LU fallback on non-convergence.
Unlocks arrays, large Yagis, collinear stacks, and big quads without
the O(N³) memory/time wall.

- **Effort:** Medium (shipped)

### 10. Complex-image ground model — *Status: shipped*

Bannister (1986) complex-image method replaces the simple Fresnel
reflection-coefficient image with an image at complex depth
`z_img = -(z_src + 2/γ_g)` where `γ_g = jk₀√εc`.  The complex
distance naturally captures the Sommerfeld lateral-wave and surface-
wave contributions that the Fresnel approximation misses for near-
field interactions (wires close to ground).  Far-field still uses
standard Fresnel coefficients (accurate at large distance).
Radial ground geometry deferred to a future polish item.

- **Effort:** Medium (shipped)

---

## Month 2+ — Strategic

### 11. NEC-2 import / export — *Status: shipped*

Parser handles CM/CE/GW/GS/GE/GN/EX/LD/TL/FR/EN cards in free-format
layout.  Writer emits a complete NEC-2 deck from a SimulationInput,
including per-wire skin-effect via LD type 5.  Frontend has
.nec ⬇ / .nec ⬆ buttons next to Save/Load JSON.

- **Effort:** Medium (shipped)

### 12. Higher-order basis functions — *Status: shipped*

Piecewise-sinusoidal (King-type) and piecewise-quadratic (Hermite)
basis functions alongside the default triangle (rooftop) basis.
Generalised MPIE kernel with abstract shape-function evaluation,
12-point quadrature for non-triangle bases, concurrent Z-matrix
assembly, and shape-function-specific current interpolation.
Frontend exposes a basis-function selector in the frequency panel.

- **Effort:** High (shipped)

### 13. Optimization loop (PSO / differential evolution) — *Status: shipped*

Wrap `Simulate()` in an objective function ("max gain + min SWR over
14.0–14.35 MHz"); expose tunable parameters on templates (Yagi
element lengths/spacings, matching dimensions).  Game-changer for
the hobbyist audience.

- **Effort:** Medium–High

### 13b. Pareto multi-objective optimization (NSGA-II) — *Status: shipped*

NSGA-II (Deb et al. 2002) multi-objective optimizer returns a Pareto
front of non-dominated trade-off designs instead of a single scalar
optimum.  Supports any combination of objectives (minimize SWR,
maximize gain, maximize F/B, etc.) with proper dominance ranking,
crowding-distance diversity preservation, SBX crossover, and
polynomial mutation.  Frontend has a dedicated "Pareto" tab with a
2D interactive scatter plot (selectable axes), a solution table,
and one-click "Apply Selected Design" to load any frontier point.
Optional worst-case band evaluation across a frequency range.

- **Effort:** Medium (shipped)

### 14. Characteristic Mode Analysis (CMA) — *Status: shipped*

Generalized eigendecomposition of the existing Z-matrix to show which
modes resonate and how well each is excited.  Research frontier for
electrically-small antenna design.

- **Effort:** Medium

### 15. Near-field (E/H) at arbitrary points — *Status: shipped*

Hertzian-dipole superposition evaluates exact near-field E and H on a
user-specified 2D observation plane (XZ, XY, or YZ).  Backend exposes
`POST /api/nearfield`; frontend has a heat-map viewer with jet colour
scale, wire overlay, selectable |E|/|H| display, and adjustable dynamic
range.  Supports free-space and PEC ground (image contributions).

- **Effort:** Medium (shipped)

### 16. Polarization analysis — *Status: shipped*

Full Stokes-parameter polarisation analysis derived from complex Eθ/Eφ
far-field components.  Computes axial ratio (dB), tilt angle, polarisation
type (linear / circular / elliptical), and rotation sense (RHCP / LHCP)
at every pattern direction.  Frontend "Polarization" tab shows headline
metrics at peak-gain, a polarisation ellipse visualisation, and
principal-plane AR and tilt-angle cuts with a 3 dB CP reference line.

- **Effort:** Medium (shipped)

### 17. Time-domain transient analysis — *Status: shipped*

Frequency-domain IFFT-based transient analysis at the antenna feed
point.  Runs a dense MoM sweep across a user-specified band, computes
the transfer function (reflection Γ(f), input voltage Z/(Z+Z₀), or
feed current 1/Z(f)), multiplies by the spectrum of a user-chosen
excitation pulse (Gaussian, RC step, or modulated Gaussian), and
inverse-FFTs to the time domain.  Frontend "Transient" tab shows the
time-domain waveform with excitation overlay, |H(f)| and phase plots,
plus headline metrics (peak amplitude, peak time, ringdown time to
-20 dB, and pulse FWHM).  Uses gonum/dsp/fourier for the FFT.

- **Effort:** Medium (shipped)

---

## Month 3 — Ground accuracy and geometry depth

### 18. Sommerfeld full integration — *Status: shipped*

The infrastructure already exists (`mom/sommerfeld.go`, 32-point
Gauss-Legendre with caching) but is not wired into the Z-matrix
assembly pipeline — only the complex-image method is active.  Exposing
it as a user-selectable "rigorous" ground mode gives accurate results
for antennas within λ/10 of lossy ground: critical for verticals, 160m
close-spaced dipoles, and any wire at low height over poor soil.  This
is the primary NEC-4 ground-accuracy differentiator.

- **Effort:** Low–Medium (2–3 days integration + NEC-4 benchmark
  validation)
- **Backend:** Wire `sommerfeld.go` into the `zmatrix.go`
  mutual-impedance path when ground method = `"sommerfeld"`; plumb the
  `method` selector through `SimulationInput` / request DTO.
- **Frontend:** "Ground method" radio button (Complex-image /
  Sommerfeld) in the ground configuration panel; advisory note when
  antenna height > λ/5 (complex-image already accurate there).

### 19. Tapered / stepped-radius wires — NEC-4 geometry

NEC-4 allows each segment of a wire to carry a different radius,
modelling tapered Yagi elements, flared feedpoint transitions, and
end-loaded whips with realistic diameter profiles.  The current `Wire`
struct holds a single `Radius` field; adding optional `RadiusStart` /
`RadiusEnd` (linear taper) propagates through the Z-matrix kernel.
Backward-compatible: omitting the new fields defaults to uniform radius.

- **Effort:** Medium (3–5 days)
- **Backend:** Extend `Wire` in `mom/types.go`; update per-segment
  radius interpolation in `zmatrix.go` and `gen_kernel.go`.
- **Frontend:** Wire editor gains radius-taper controls (single vs.
  start+end); 3D viewport scales the wire tube per segment to reflect
  the taper visually.

### 20. ESA Q-factor and Chu–Harrington bandwidth bound

No formal Q-factor computation exists.  Given that CMA already
provides the eigendecomposition, computing the total radiation Q from
the Harrington energy-field integral is mathematically straightforward.
Output: modal Q per CMA mode, system Q at operating frequency,
predicted 3 dB fractional bandwidth, and a comparison against the Chu
limit `Q_Chu = 1/(ka)³ + 1/(ka)`.  Directly useful for 160m, 40m
portable, and POTA backpack ESA designs.

- **Effort:** Low (1–2 days)
- **Backend:** New `mom/esa.go`; post-process the existing Z-matrix
  and CMA eigenvalues; extend `SimulationResult` with `ESAMetrics`.
- **Frontend:** "ESA / Q" metrics card added to the CMA viewer, with
  the Chu limit overlaid on the modal-significance plot.

### 21. Radial / mesh ground-plane geometry

Vertical antennas over real ground depend critically on the buried or
surface radial system.  An auto-generator for N radial wires at
configurable length, angle, and height above ground (0 = buried
approximation) lets users model 4-, 16-, 32-, and 64-radial systems
without manually entering every wire.  A mesh-grid option covers
elevated ground screens.

- **Effort:** Medium (2–3 days backend, 1 day frontend)
- **Backend:** New `geometry/radial.go` template generator emitting a
  standard `Wire` slice — no solver changes required.
- **Frontend:** "Ground radials" sub-panel in the geometry editor
  with count, length (λ fractions), elevation, and wire properties;
  radial wires rendered in a distinct colour in the 3D viewer.

### 22. Expanded antenna template library

Five templates (dipole, vertical, Yagi, inverted-V, loop) cover the
basics.  Helical, LPDA, spiral, and collinear templates need only
parametric geometry code — no solver changes.

- **Effort:** Low (1 day per template, ~4 days total)
- **Backend:** New cases in `geometry/templates.go`:
  - **Helical** — axial-mode and normal-mode; parameters: diameter,
    pitch, turns, design frequency.
  - **Log-periodic dipole array (LPDA)** — tau, sigma, design
    frequency; auto-calculates all element lengths and spacings.
  - **Archimedes spiral** — polyline approximation; inner/outer radius,
    turns, arm width.
  - **Collinear / in-phase stack** — N dipoles on a vertical boom with
    λ/2 phasing stubs modelled as TL elements.
- **Frontend:** New entries in the template picker; preview renders in
  the 3D viewer before insertion.

---

## Month 4+ — Advanced excitation and analysis

### 23. Plane-wave excitation and radar cross-section (RCS)

NEC-4 EX-card type 1 illuminates the structure with a uniform plane
wave at arbitrary θ/φ incidence and polarisation.  The scattered field
is the full MoM response minus the incident wave.  Unlocks: RCS
(monostatic and bistatic), receiving pattern / effective aperture, noise
temperature from sky-brightness maps, and interference modelling (tower
proximity).

- **Effort:** Medium (4–6 days)
- **Backend:** New excitation vector in `mom/types.go`; plane-wave
  current injection in `solver.go`; post-process scattered fields in
  `farfield.go`; new `POST /api/rcs` endpoint.
- **Frontend:** Excitation-type selector (voltage source / plane wave);
  RCS result tab showing monostatic RCS vs. angle and peak RCS in dBsm.

### 24. Multi-port / S-parameter analysis

Extending to N ports enables: two-port S₁₁/S₂₁ measurement simulation,
balun and hybrid coupler modelling, mutual-impedance matrix for phased
arrays, and feed-network analysis.  The existing TL 2-port
infrastructure is a foundation.

- **Effort:** Medium–High (5–7 days)
- **Backend:** `SimulationInput` gains a `Ports []Port` slice; solver
  loops over excitations building a full Z-parameter matrix; convert to
  S-parameters at user Z₀; extend Touchstone export to N-port .sNp.
- **Frontend:** Multi-source editor; S-parameter result tab with
  magnitude/phase vs. frequency per port pair.

### 25. Multi-layer stratified ground model

NEC-4 supports a second ground layer at user-specified depth with
independent σ and εr, capturing coastal sites (sand over seawater),
rock substrates, permafrost, and high-conductivity subsoil.  Extends
the Fresnel path in `ground_real.go` with a recursive Wait / King
two-layer impedance transfer matrix.

- **Effort:** Medium (3–5 days)
- **Backend:** Extend `Ground` struct with optional `Layer2Sigma`,
  `Layer2EpsR`, `Layer2Depth`; recursive impedance transfer in
  `ground_real.go`.
- **Frontend:** Second layer input fields in the ground panel; depth
  slider in λ fractions; preset table (sand, rock, permafrost, urban
  fill).

### 26. Buried wire / underground conductor model — NEC-4 SOMNEC

NEC-4's SOMNEC module models wires embedded in lossy ground via
Sommerfeld integrals evaluated at complex depth.  Covers buried
radials, underground feed cables, and Beverage antenna counterpoise.
Prerequisite: item 18 (Sommerfeld in production).

- **Effort:** High (7–10 days; prerequisite: item 18)
- **Backend:** Extend `Wire` with `BuriedDepth float64`; route
  buried-wire interactions through a new `ground_buried.go` kernel
  evaluating Sommerfeld integrals with source below the interface;
  adapt near-field for buried observation points.
- **Frontend:** Per-wire "buried at depth" toggle + depth input; buried
  wires rendered below the ground plane with a dash style in the 3D
  viewer.

### 27. WebSocket / SSE streaming for long-running jobs

Optimization and Pareto runs can take minutes and today block a single
HTTP request.  Server-Sent Events streaming delivers iteration-by-
iteration convergence updates, intermediate best-point patterns, and
live Pareto-front evolution.  The `ARCHITECTURE.md` already flags this
as the planned long-job mechanism.

- **Effort:** Medium (4–6 days)
- **Backend:** Channel-based progress reporter wrapping `optimizer.go`
  and `pareto.go`; new SSE endpoints `GET /api/optimize/stream` and
  `GET /api/pareto/stream`; token-based job ID for reconnect.
- **Frontend:** Replace blocking spinner with live convergence chart,
  real-time best-parameter table, and streaming 3D pattern update every
  N iterations.

### 28. Group-delay and dispersion analysis

The transient analysis computes the IFFT time-domain response but does
not extract group delay `τ_g(f) = −dφ(H(f))/dω`.  Adding a group-delay
curve, phase-linearity metric, and chirped / swept-FM excitation
makes the tool useful for UWB antenna evaluation (FCC Part 15,
IEEE 802.15.4a).

- **Effort:** Low–Medium (2–3 days)
- **Backend:** Phase-differentiation pass on the sweep transfer
  function in `transient.go`; add chirped-Gaussian waveform option;
  new `GroupDelayResult` fields.
- **Frontend:** Group-delay sub-plot in the "Transient" tab; excitation
  picker adds "chirp" with start/stop frequency controls.

### 29. Near-field to far-field transformation (NFT)

The near-field viewer shows E/H on a 2D grid but does not transform it
to a far-field pattern.  An NFT using the Love equivalence principle
(surface equivalence + spherical-mode expansion) enables: simulating
anechoic-chamber near-field scans, cross-validating the MoM far-field
against the Huygens-surface result, and future import of measured
near-field data as an excitation.

- **Effort:** Medium (4–6 days)
- **Backend:** New `mom/nft.go` implementing equivalent-current
  integration over the near-field grid using a spherical-harmonic
  expansion; expose via `POST /api/nft`.
- **Frontend:** "Transform to far-field" button in the near-field
  viewer; result overlaid on the existing 3D pattern viewer for
  side-by-side comparison.

---

## Strategic horizon

### 30. Sensitivity analysis and tolerance-aware design

The optimizer finds optima but does not quantify sensitivity to
manufacturing tolerances.  A local gradient pass after optimization —
perturbing each parameter by ±1 % and re-simulating — produces a
ranked sensitivity table and a worst-case SWR / gain spread across a
tolerance band.  Directly useful for deciding element-cutting precision.

- **Effort:** Medium (3–4 days)
- **Backend:** New `mom/sensitivity.go`; post-optimization perturbation
  sweep; return `SensitivityResult` with per-variable partial
  derivatives and worst-case metric ranges.
- **Frontend:** "Sensitivity" sub-tab on the optimizer and Pareto
  result views; bar chart of ranked sensitivities; tolerance slider for
  worst-case spread.

### 31. Concurrent geometry + matching-network co-optimization

Physical dimensions and matching networks are currently optimized
sequentially.  Exposing matching-network topology and component values
as first-class optimizer variables alongside wire geometry allows the
solver to find intrinsically well-matched designs — essential for ESA
work where the matching network is part of the antenna system.

- **Effort:** Medium–High (5–7 days)
- **Backend:** Extend optimizer variable schema in `optimizer.go` /
  `pareto.go` to include `MatchTopology` + per-component bounds; call
  `match.Synthesize()` inside the inner evaluation loop; objective
  function receives post-match SWR.
- **Frontend:** Optimizer parameter editor gains a "Matching network"
  section with topology selection and component bounds; Pareto viewer
  can plot matched-SWR vs. gain trade-offs.

### 32. Surface MoM with triangular-patch elements

The single largest capability gap versus FEKO, CST, and AN-SOF.
Rao-Wilton-Glisson (RWG) triangular basis functions alongside the
existing wire bases enable planar radiators (microstrip patch arrays,
slot antennas, reflector dishes, horn aperture equivalents) and hybrid
wire+surface structures.  Full subsystem: new bases, new MPIE kernel,
new Z-matrix assembly, new current visualisation, STL mesh import.

- **Effort:** Very High (14–21 days)
- **Backend:** New `mom/patch.go` with RWG basis definition and Green's
  function evaluation; extend `zmatrix.go` for wire–patch and
  patch–patch interactions; STL/OBJ mesh importer; extend `farfield.go`
  for surface-current radiation integral.
- **Frontend:** Surface mesh import (STL upload); per-face material
  assignment; 3D viewer shows surface current density as a colour map;
  new patch-array and reflector templates.

### 33. Machine-learning surrogate accelerator

A Gaussian-process (GP) or neural-network surrogate trained on MoM
evaluation samples guides PSO/NSGA-II and calls the full solver only to
validate promising candidates.  10–100× reduction in optimization
wall-clock time; particularly impactful for Pareto runs that today
require thousands of full MoM evaluations.

- **Effort:** High (10–14 days; requires offline training pipeline)
- **Backend:** New `mom/surrogate.go` wrapping a lightweight GP or ONNX
  neural network; online active-learning loop calling MoM only when
  surrogate uncertainty exceeds a threshold; model serialized to disk
  for reuse across sessions.
- **Frontend:** "Accelerated" toggle on optimizer / Pareto panels;
  surrogate vs. MoM call ratio in the convergence chart; model accuracy
  metrics displayed.

---

## Polish (interleave throughout)

- **Regression benchmarks** — pin the existing dipole test plus a
  DL6WU Yagi and a K1FO reference design against published NEC-2
  numbers.
- **Coated-wire dielectric loading** — ε_r and shell thickness; a
  3–5 % resonant-frequency shift on insulated HF wires.  *Shipped.*
- **Environmental knobs** — rain / ice as a thin dielectric shell
  or tan δ bump.  *Shipped* (global weather-film model stacked on
  per-wire coatings).
- **Touchstone (.s1p) and CSV export** of sweep data — *shipped*.
  CSV: freq / R / X / |Z| / SWR / Γ / RL.  Touchstone v1.1 .s1p
  RI-format, Hz freq, configurable Z₀.
- **Convergence reporter** — re-run at 2× segmentation and report
  the relative change in driving-point impedance.  *Shipped.*
  Backend `POST /api/convergence` runs the MoM solver at user
  segments (1×) then at doubled segments (2×), returning impedance,
  SWR, and gain at both levels plus relative deltas.  Frontend
  "Convergence" tab shows a colour-coded comparison table, delta bar,
  and a plain-English verdict (excellent / good / marginal / poor).

---

## Sources

- Conformal MoM advances (AN-SOF):
  <https://antennasimulator.com/index.php/knowledge-base/overcoming-7-limitations-in-antenna-design-introducing-an-sofs-conformal-method-of-moments/>
- Higher-order Legendre basis functions (IEEE TAP):
  <https://ieeexplore.ieee.org/document/1353496/>
- NEC-4.2 ground models (OSTI): <https://www.osti.gov/biblio/1117909>
- Characteristic Mode Analysis (Nature Sci. Reports):
  <https://www.nature.com/articles/s41598-024-66515-x>
- PSO/GA for antenna optimization:
  <https://www.sciencedirect.com/science/article/abs/pii/S2214785322008203>
- NEC-2 / NEC-3 Method of Moments (Burke & Poggio, 1981):
  NOSC Technical Document 116, Naval Ocean Systems Center
- Sommerfeld ground integrals (Michalski & Mosig, 1997):
  IEEE Trans. Antennas Propagat. 45(3):508–519
- Chu limit and ESA Q-factor (Chu, 1948):
  J. Appl. Phys. 19:1163–1175
- Two-layer ground model (Wait, 1954):
  Can. J. Phys. 32:571–579
- RWG triangular basis functions for surface MoM (Rao, Wilton &
  Glisson, 1982): IEEE Trans. Antennas Propagat. 30(3):409–418
- LPDA design equations (Carrel, 1961):
  IRE Trans. Antennas Propagat. AP-9(6):553–562
