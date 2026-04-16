# VE3KSM Antenna Studio Roadmap

A research-informed enhancement plan, ordered by ROI (user value per unit
of implementation effort).  Items are derived from a survey of recent
(2020–2026) wire-MoM literature and a feature-gap analysis against
EZNEC, MMANA-GAL, 4nec2, xnec2c, and AN-SOF.

Each item lists effort (Low / Medium / High), the primary files to
touch, and the user-visible payoff.  Status is updated as work lands.

---

## Week 1 — Quick wins (low effort, high payoff)

### 1. Lumped loads on segments — *Status: planned*

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

### 2. Smith chart data + arbitrary-Z₀ VSWR — *Status: planned*

Hardcoded 50 Ω today.  Make the reference impedance a per-request
parameter and return the complex reflection coefficient `S11` so the
frontend can plot a Smith chart and report VSWR at user Z₀.

- **Effort:** Low (half day backend, 1 day frontend)
- **Backend:** add `reference_impedance` to simulate/sweep request
  DTOs; compute Γ = (Z − Z₀)/(Z + Z₀), return as `{re, im}`.
- **Frontend:** Smith chart canvas component (SVG with constant-R and
  constant-X arcs); add to results pane.

### 3. Far-field metrics + 2D polar cuts — *Status: planned*

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

### 4. Conductor materials + skin-effect loss — *Status: planned*

Wires currently carry only a radius (loss-free).  Add a material
library (Cu, Al, brass, steel, stainless) and apply the surface
impedance correction `Z_s = (1 + j) / (σ δ)` with `δ = 1/√(πfμσ)` to
the Z-matrix diagonal.  Turns the solver into something that matches
measured Q on loaded / electrically-small antennas.

- **Effort:** Low–Medium (1–2 days)
- **Backend:** new `material` field on `Wire`; lookup table; assembly
  changes in `mom/zmatrix.go`.
- **Frontend:** material dropdown in wire editor; default = Cu.

### 5. Segmentation validator — *Status: planned*

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

### 9. GMRES + simple preconditioning

Dense LU via gonum tops out around a few hundred segments.  Iterative
GMRES with a diagonal or near-field block preconditioner unlocks
arrays, large Yagis, collinear stacks, and big quads.

- **Effort:** Medium

### 10. Complex-image ground model

Bannister/Lindell two-fictitious-image approximation — far more
accurate than pure Fresnel for close-to-ground wires, and only
marginally more code than the current `mom/ground_real.go`.  Also
expose explicit radial-ground geometry (N radials, length, depth).

- **Effort:** Medium

---

## Month 2+ — Strategic

### 11. NEC-2 import / export — *Status: shipped*

Parser handles CM/CE/GW/GS/GE/GN/EX/LD/TL/FR/EN cards in free-format
layout.  Writer emits a complete NEC-2 deck from a SimulationInput,
including per-wire skin-effect via LD type 5.  Frontend has
.nec ⬇ / .nec ⬆ buttons next to Save/Load JSON.

- **Effort:** Medium (shipped)

### 12. Higher-order basis functions

Quadratic or hierarchical Legendre basis cuts unknown count 3–5× for
the same accuracy.  Pairs naturally with the iterative solver.

- **Effort:** High

### 13. Optimization loop (PSO / differential evolution)

Wrap `Simulate()` in an objective function ("max gain + min SWR over
14.0–14.35 MHz"); expose tunable parameters on templates (Yagi
element lengths/spacings, matching dimensions).  Game-changer for
the hobbyist audience.

- **Effort:** Medium–High

### 14. Characteristic Mode Analysis (CMA)

Generalized eigendecomposition of the existing Z-matrix to show which
modes resonate and how well each is excited.  Research frontier for
electrically-small antenna design.

- **Effort:** Medium

### 15. Near-field (E/H) at arbitrary points

Reuses the Green's function path that far-field already uses.  Useful
for EMC / RF-exposure checks and for coupling studies.

- **Effort:** Medium

---

## Polish (interleave throughout)

- **Regression benchmarks** — pin the existing dipole test plus a
  DL6WU Yagi and a K1FO reference design against published NEC-2
  numbers.
- **Coated-wire dielectric loading** — ε_r and shell thickness; a
  3–5 % resonant-frequency shift on insulated HF wires.
- **Environmental knobs** — rain / ice as a thin dielectric shell
  or tan δ bump.
- **Touchstone (.s1p) and CSV export** of sweep data — *shipped*.
  CSV: freq / R / X / |Z| / SWR / Γ / RL.  Touchstone v1.1 .s1p
  RI-format, Hz freq, configurable Z₀.
- **Convergence reporter** — re-run at 2× segmentation and report
  the relative change in driving-point impedance.

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
