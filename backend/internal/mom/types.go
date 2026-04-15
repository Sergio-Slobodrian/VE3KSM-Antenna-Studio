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
}

// Wire represents a single straight wire element in the antenna geometry.
// The wire runs from endpoint (X1,Y1,Z1) to (X2,Y2,Z2) in Cartesian
// coordinates (meters). Radius is the wire cross-section radius (meters),
// used for the thin-wire kernel approximation. Segments controls how many
// equal-length pieces the wire is subdivided into for the MoM discretization.
type Wire struct {
	X1       float64 `json:"x1"`       // start X coordinate (m)
	Y1       float64 `json:"y1"`       // start Y coordinate (m)
	Z1       float64 `json:"z1"`       // start Z coordinate (m)
	X2       float64 `json:"x2"`       // end X coordinate (m)
	Y2       float64 `json:"y2"`       // end Y coordinate (m)
	Z2       float64 `json:"z2"`       // end Z coordinate (m)
	Radius   float64 `json:"radius"`   // wire radius (m)
	Segments int     `json:"segments"` // number of MoM segments for this wire
}

// GroundConfig describes the ground plane configuration.
// Type selects the ground model: "free_space" (no ground), "perfect" (PEC
// image theory), or "real" (lossy ground, not yet implemented).
type GroundConfig struct {
	Type         string  `json:"type"`         // "free_space", "perfect", "real"
	Conductivity float64 `json:"conductivity"` // ground conductivity in S/m (only for "real")
	Permittivity float64 `json:"permittivity"` // relative permittivity (only for "real")
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
	Currents  []CurrentEntry   `json:"currents"`  // current on each segment
	Impedance ComplexImpedance `json:"impedance"` // feed-point impedance (ohms)
	SWR       float64          `json:"swr"`       // voltage standing wave ratio (50-ohm ref)
	GainDBi   float64          `json:"gain_dbi"`  // peak directivity in dBi
	Pattern   []PatternPoint   `json:"pattern"`   // far-field radiation pattern samples
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
	Frequencies []float64          `json:"frequencies"` // frequency points (MHz)
	SWR         []float64          `json:"swr"`         // SWR at each frequency
	Impedance   []ComplexImpedance `json:"impedance"`   // impedance at each frequency
}
