package mom

// SimulationInput holds the full input for a MoM simulation run.
type SimulationInput struct {
	Wires     []Wire       `json:"wires"`
	Frequency float64      `json:"frequency"` // Hz
	Ground    GroundConfig `json:"ground"`
	Source    Source       `json:"source"`
}

// Wire represents a single wire element in the antenna geometry.
type Wire struct {
	X1       float64 `json:"x1"`
	Y1       float64 `json:"y1"`
	Z1       float64 `json:"z1"`
	X2       float64 `json:"x2"`
	Y2       float64 `json:"y2"`
	Z2       float64 `json:"z2"`
	Radius   float64 `json:"radius"`
	Segments int     `json:"segments"`
}

// GroundConfig describes the ground plane configuration.
type GroundConfig struct {
	Type         string  `json:"type"`         // "free_space", "perfect", "real"
	Conductivity float64 `json:"conductivity"` // S/m (only for "real")
	Permittivity float64 `json:"permittivity"` // relative (only for "real")
}

// Source describes the excitation source on a wire segment.
type Source struct {
	WireIndex    int        `json:"wire_index"`
	SegmentIndex int        `json:"segment_index"`
	Voltage      complex128 `json:"-"`
}

// SolverResult holds the full output of a single-frequency simulation.
type SolverResult struct {
	Currents  []CurrentEntry   `json:"currents"`
	Impedance ComplexImpedance `json:"impedance"`
	SWR       float64          `json:"swr"`
	GainDBi   float64          `json:"gain_dbi"`
	Pattern   []PatternPoint   `json:"pattern"`
}

// CurrentEntry holds current magnitude and phase for one segment.
type CurrentEntry struct {
	SegmentIndex int     `json:"segment"`
	Magnitude    float64 `json:"magnitude"`
	PhaseDeg     float64 `json:"phase"`
}

// ComplexImpedance holds resistance and reactance in ohms.
type ComplexImpedance struct {
	R float64 `json:"r"`
	X float64 `json:"x"`
}

// PatternPoint is a single sample of the far-field radiation pattern.
type PatternPoint struct {
	ThetaDeg float64 `json:"theta"`
	PhiDeg   float64 `json:"phi"`
	GainDB   float64 `json:"gain_db"`
}

// SweepResult holds results from a frequency sweep.
type SweepResult struct {
	Frequencies []float64          `json:"frequencies"`
	SWR         []float64          `json:"swr"`
	Impedance   []ComplexImpedance `json:"impedance"`
}
