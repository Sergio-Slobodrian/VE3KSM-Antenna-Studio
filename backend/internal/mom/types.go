// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package mom implements a Method of Moments (MoM) electromagnetic solver for
// thin-wire antennas. MoM converts the electric field integral equation (EFIE)
// into a linear system Z*I = V by expanding the unknown current in basis
// functions and testing with weighting functions. This package uses triangle
// (rooftop) basis functions on straight wire segments, with Gauss-Legendre
// quadrature for numerical integration of the Green's function kernels.
//
// The solver supports free-space and perfect-ground-plane environments, voltage
// source excitation, frequency sweeps, and far-field radiation pattern computation.
package mom

// SimulationInput holds the full input for a single-frequency MoM simulation run.
// It describes the antenna geometry (wires), operating frequency, ground plane
// configuration, and excitation source.
type SimulationInput struct {
	Wires     []Wire       `json:"wires"`
	Frequency float64      `json:"frequency"` // operating frequency in Hz
	Ground    GroundConfig `json:"ground"`
	Source    Source       `json:"source"`
	Loads     []Load       `json:"loads,omitempty"` // optional lumped R/L/C loads
	// skipBandgapRetry suppresses the negative-R self-diagnosis
	// recursion in Simulate.  Set internally on perturbed probes.
	skipBandgapRetry bool `json:"-"`
	TransmissionLines []TransmissionLine `json:"transmission_lines,omitempty"` // optional 2-port TLs
	// ReferenceImpedance (Ω) is used for the reflection coefficient and VSWR
	// calculations.  When zero or negative the solver substitutes
	// DefaultReferenceImpedance (50 Ω).
	ReferenceImpedance float64 `json:"reference_impedance,omitempty"`
	// BasisOrder selects the current expansion function family.
	// "" or "triangle" = piecewise-linear rooftop (default, classic).
	// "sinusoidal" = piecewise-sinusoidal King-type (3-5x fewer unknowns).
	// "quadratic" = piecewise-quadratic Hermite (smooth, higher-order).
	BasisOrder BasisOrder   `json:"basis_order,omitempty"`
	Weather    WeatherConfig `json:"weather,omitempty"`
}

// LoadTopology selects how a Load's R, L, and C components are combined.
//
//   - LoadSeriesRLC:   Z = R + jωL + 1/(jωC)
//     Components with value 0 are simply omitted from the sum, so a single
//     non-zero field models a pure resistor, inductor, or capacitor.
//   - LoadParallelRLC: Y = 1/R + 1/(jωL) + jωC, then Z = 1/Y
//     Components with value 0 are omitted from the admittance sum.
type LoadTopology string

const (
	LoadSeriesRLC   LoadTopology = "series_rlc"
	LoadParallelRLC LoadTopology = "parallel_rlc"
)

// Load is a lumped passive R / L / C circuit attached to a single segment of
// a wire.  It is the standard NEC-style "LD" element used to model traps,
// loading coils, resistive terminations, hat capacitors, folded-dipole
// stubs, and lumped baluns.
//
// The load is realised by adding its complex impedance Z_load(ω) directly to
// the diagonal entry of the Z-matrix for the basis function nearest the
// requested segment, which is the same convention the existing voltage
// source uses.  This is exact for delta-gap-style lumped elements and is
// the standard treatment in NEC-2/4 for the LD card.
//
// Field semantics by topology:
//
//	series_rlc:   any combination of R (Ω), L (H), C (F).  Zero values
//	              are skipped.  Pure-R, pure-L, pure-C all valid.
//	parallel_rlc: at least one of R, L, C must be non-zero.  Zero values
//	              are skipped (treated as open / infinite).
type Load struct {
	WireIndex    int          `json:"wire_index"`
	SegmentIndex int          `json:"segment_index"`
	Topology     LoadTopology `json:"topology"` // "series_rlc" or "parallel_rlc"
	R            float64      `json:"r"`        // resistance (Ω)
	L            float64      `json:"l"`        // inductance (H)
	C            float64      `json:"c"`        // capacitance (F)
}

// Wire represents a single straight wire element in the antenna geometry.
// The wire runs from endpoint (X1,Y1,Z1) to (X2,Y2,Z2) in Cartesian
// coordinates (meters). Radius is the wire cross-section radius (meters),
// used for the thin-wire kernel approximation. Segments controls how many
// equal-length pieces the wire is subdivided into for the MoM discretization.
type Wire struct {
	X1       float64      `json:"x1"`                 // start X coordinate (m)
	Y1       float64      `json:"y1"`                 // start Y coordinate (m)
	Z1       float64      `json:"z1"`                 // start Z coordinate (m)
	X2       float64      `json:"x2"`                 // end X coordinate (m)
	Y2       float64      `json:"y2"`                 // end Y coordinate (m)
	Z2       float64      `json:"z2"`                 // end Z coordinate (m)
	Radius   float64      `json:"radius"`             // wire radius (m)
	Segments int          `json:"segments"`           // number of MoM segments for this wire
	Material MaterialName `json:"material,omitempty"` // optional conductor material; "" = perfect conductor
	// Optional linear taper. When both > 0, each segment gets a radius
	// interpolated along the wire from (X1,Y1,Z1) to (X2,Y2,Z2). Either
	// unset (0) falls back to the uniform Radius above.
	RadiusStart float64 `json:"radius_start,omitempty"`
	RadiusEnd   float64 `json:"radius_end,omitempty"`
	// Dielectric coating (IS-card model). Zero thickness or EpsR ≤ 1 = bare wire.
	CoatingThickness float64 `json:"coating_thickness,omitempty"` // coating outer shell thickness (m)
	CoatingEpsR      float64 `json:"coating_eps_r,omitempty"`     // coating relative permittivity (εr)
	CoatingLossTan   float64 `json:"coating_loss_tan,omitempty"`  // coating loss tangent (tanδ)
}

// isTapered reports whether both RadiusStart and RadiusEnd are set.
func (w Wire) isTapered() bool {
	return w.RadiusStart > 0 && w.RadiusEnd > 0
}

// RadiusAt returns the wire radius at parametric position s ∈ [0,1] along
// the wire from (X1,Y1,Z1) to (X2,Y2,Z2). For uniform (non-tapered) wires
// it returns Radius regardless of s.
func (w Wire) RadiusAt(s float64) float64 {
	if !w.isTapered() {
		return w.Radius
	}
	return w.RadiusStart + s*(w.RadiusEnd-w.RadiusStart)
}

// RadiusAtEndpoint1 returns the wire radius at the (X1,Y1,Z1) endpoint.
func (w Wire) RadiusAtEndpoint1() float64 {
	if !w.isTapered() {
		return w.Radius
	}
	return w.RadiusStart
}

// RadiusAtEndpoint2 returns the wire radius at the (X2,Y2,Z2) endpoint.
func (w Wire) RadiusAtEndpoint2() float64 {
	if !w.isTapered() {
		return w.Radius
	}
	return w.RadiusEnd
}

// taperRadii returns the (start, end) radii to use when generating segments.
// For uniform wires both values equal Radius.
func (w Wire) taperRadii() (rStart, rEnd float64) {
	if w.isTapered() {
		return w.RadiusStart, w.RadiusEnd
	}
	return w.Radius, w.Radius
}

// WeatherConfig describes a global environmental film applied as an outer
// dielectric layer on every wire (stacked on top of any per-wire coating).
// Preset "dry" (or empty) contributes no loading; the other presets supply
// εr and tanδ for the IS-card multi-layer formula.
type WeatherConfig struct {
	Preset    string  `json:"preset"`    // "dry", "rain", "ice", "wet_snow"
	Thickness float64 `json:"thickness"` // film thickness (m); 0 = no film
	EpsR      float64 `json:"eps_r"`     // relative permittivity (overrides preset default when > 0)
	LossTan   float64 `json:"loss_tan"`  // loss tangent tanδ (overrides preset default when EpsR > 0)
}

// GroundConfig describes the ground plane configuration.
// Type selects the ground model: "free_space" (no ground), "perfect" (PEC
// image theory), or "real" (lossy ground via Fresnel reflection coefficients).
type GroundConfig struct {
	Type           string  `json:"type"`                        // "free_space", "perfect", "real"
	Conductivity   float64 `json:"conductivity"`                // ground conductivity in S/m (only for "real")
	Permittivity   float64 `json:"permittivity"`                // relative permittivity (only for "real")
	MoisturePreset string  `json:"moisture_preset,omitempty"`   // label only ("custom" or soil category); εr/σ remain authoritative
	RegionPreset   string  `json:"region_preset,omitempty"`     // label only (e.g. "itu:3", "user:<uuid>"); εr/σ remain authoritative
	// Method selects the near-field ground kernel for real ground:
	//   "" or "image" — Bannister complex-image method (fast, default)
	//   "sommerfeld"  — full Sommerfeld integration (rigorous for h < λ/10)
	Method string `json:"ground_method,omitempty"`
}

// Source describes the voltage excitation applied to the antenna.
// WireIndex and SegmentIndex identify which segment receives the source.
// Voltage is the complex source voltage (V); it defaults to 1+0j if zero.
// The Voltage field is excluded from JSON since it is always set internally.
type Source struct {
	WireIndex    int        `json:"wire_index"`
	SegmentIndex int        `json:"segment_index"`
	Voltage      complex128 `json:"-"`
}

// SolverResult holds the full output of a single-frequency simulation.
type SolverResult struct {
	Currents           []CurrentEntry   `json:"currents"`            // current on each segment
	Impedance          ComplexImpedance `json:"impedance"`           // feed-point impedance (ohms)
	SWR                float64          `json:"swr"`                 // VSWR at ReferenceImpedance
	Reflection         complex128       `json:"-"`                   // complex reflection coefficient Γ at ReferenceImpedance
	ReferenceImpedance float64          `json:"reference_impedance"` // Z₀ used for SWR / Γ (Ω)
	GainDBi            float64          `json:"gain_dbi"`            // peak directivity in dBi
	Pattern            []PatternPoint      `json:"pattern"`             // far-field radiation pattern samples
	Metrics            FarFieldMetrics     `json:"metrics"`             // F/B, beamwidth, sidelobe, efficiency
	Cuts               PolarCuts           `json:"polar_cuts"`          // azimuth & elevation 2D cuts
	Polarization       PolarizationMetrics `json:"polarization"`        // axial ratio, tilt, sense per direction
	Warnings           []Warning           `json:"warnings,omitempty"`  // non-blocking accuracy heuristics
}

// CurrentEntry holds the current phasor for one segment, decomposed into
// magnitude (amperes) and phase (degrees).
type CurrentEntry struct {
	SegmentIndex int     `json:"segment"`   // segment index in the global segment array
	Magnitude    float64 `json:"magnitude"` // current magnitude (A)
	PhaseDeg     float64 `json:"phase"`     // current phase (degrees)
}

// ComplexImpedance holds the real (resistance) and imaginary (reactance)
// parts of an impedance value, both in ohms.
type ComplexImpedance struct {
	R float64 `json:"r"` // resistance (ohms)
	X float64 `json:"x"` // reactance (ohms)
}

// PatternPoint is a single sample of the far-field radiation pattern in
// spherical coordinates. ThetaDeg is the polar angle from the +z axis,
// PhiDeg is the azimuthal angle from the +x axis, and GainDB is the
// directivity at that angle in dB relative to isotropic (dBi).
type PatternPoint struct {
	ThetaDeg float64 `json:"theta"`   // polar angle (degrees, 0=zenith, 90=horizon)
	PhiDeg   float64 `json:"phi"`     // azimuthal angle (degrees)
	GainDB   float64 `json:"gain_db"` // directivity (dBi)
}

// SweepResult holds results from a frequency sweep: impedance and SWR
// at each frequency point. Frequencies are stored in MHz for display convenience.
type SweepResult struct {
	Frequencies        []float64          `json:"frequencies"`         // frequency points (MHz)
	SWR                []float64          `json:"swr"`                 // SWR at each frequency, at ReferenceImpedance
	Impedance          []ComplexImpedance `json:"impedance"`           // impedance at each frequency
	Reflections        []complex128       `json:"-"`                   // complex Γ at each frequency
	ReferenceImpedance float64            `json:"reference_impedance"` // Z₀ used for SWR / Γ (Ω)
	Warnings           []Warning          `json:"warnings,omitempty"`  // accuracy warnings for the sweep range (validated at start + end freq)
}
