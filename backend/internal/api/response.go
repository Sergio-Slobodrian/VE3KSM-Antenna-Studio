package api

import "antenna-studio/backend/internal/mom"

// SimulateResponse is the JSON body returned by POST /api/simulate.
// It contains the full results of a single-frequency MoM simulation:
// feed-point impedance, standing wave ratio, peak gain, the 3D radiation
// pattern, and per-segment current distribution. The frontend uses these
// to render the polar plot, current overlay, and impedance readout.
type SimulateResponse struct {
	Impedance          ImpedanceDTO        `json:"impedance"`
	SWR                float64             `json:"swr"`
	Reflection         ReflectionDTO       `json:"reflection"`          // Γ at ReferenceImpedance, complex (Smith-chart input)
	ReferenceImpedance float64             `json:"reference_impedance"` // Z₀ used for SWR / reflection (Ω)
	GainDBi            float64             `json:"gain_dbi"`
	Metrics            mom.FarFieldMetrics `json:"metrics"`             // F/B, beamwidth, sidelobe level, efficiency
	PolarCuts          mom.PolarCuts       `json:"polar_cuts"`          // azimuth + elevation 2D cuts
	Pattern            []PatternDTO        `json:"pattern"`
	Currents           []CurrentDTO        `json:"currents"`
	Warnings           []mom.Warning       `json:"warnings,omitempty"`  // non-blocking accuracy warnings
}

// ReflectionDTO is the rectangular form of a complex reflection coefficient
// suitable for JSON transport.  Re and Im are the real and imaginary parts
// of Γ; |Γ| ≤ 1 inside the Smith chart's unit circle.
type ReflectionDTO struct {
	Re float64 `json:"re"`
	Im float64 `json:"im"`
}

// ImpedanceDTO holds the complex feed-point impedance decomposed into
// resistance R (real part, ohms) and reactance X (imaginary part, ohms).
type ImpedanceDTO struct {
	R float64 `json:"r"`
	X float64 `json:"x"`
}

// PatternDTO is a single sample point in the far-field radiation pattern.
// Theta is the elevation angle in degrees (0 = zenith, 90 = horizon),
// Phi is the azimuth angle in degrees, and GainDB is the directive gain
// at that direction in dBi.
type PatternDTO struct {
	Theta  float64 `json:"theta"`
	Phi    float64 `json:"phi"`
	GainDB float64 `json:"gain_db"`
}

// CurrentDTO holds the current magnitude and phase for one wire segment.
// Segment is the global segment index across all wires. The frontend uses
// this to color-code the 3D wire model by current intensity.
type CurrentDTO struct {
	Segment   int     `json:"segment"`
	Magnitude float64 `json:"magnitude"`
	Phase     float64 `json:"phase"`
}

// SweepResponse is the JSON body returned by POST /api/sweep.
// It contains parallel arrays of frequency (MHz), SWR, and impedance values
// that the frontend plots as SWR-vs-frequency and impedance-vs-frequency charts.
type SweepResponse struct {
	Frequencies        []float64       `json:"frequencies"`
	SWR                []float64       `json:"swr"`
	Impedance          []ImpedanceDTO  `json:"impedance"`
	Reflections        []ReflectionDTO `json:"reflections"`         // Γ at each frequency (Smith-chart locus)
	ReferenceImpedance float64         `json:"reference_impedance"` // Z₀ used for SWR / Γ (Ω)
	Warnings           []mom.Warning   `json:"warnings,omitempty"`  // accuracy warnings for the sweep range
}

// ErrorResponse is the standard JSON error envelope used by all endpoints.
// The frontend checks for this structure to display user-facing error messages.
type ErrorResponse struct {
	Error string `json:"error"`
}

// SolverResultToResponse converts the MoM solver's internal result type into
// the API response DTO. It maps solver field names (ThetaDeg, PhiDeg, PhaseDeg)
// to the shorter JSON-friendly names used by the frontend.
func SolverResultToResponse(r *mom.SolverResult) SimulateResponse {
	pattern := make([]PatternDTO, len(r.Pattern))
	for i, p := range r.Pattern {
		pattern[i] = PatternDTO{
			Theta:  p.ThetaDeg,
			Phi:    p.PhiDeg,
			GainDB: p.GainDB,
		}
	}

	currents := make([]CurrentDTO, len(r.Currents))
	for i, c := range r.Currents {
		currents[i] = CurrentDTO{
			Segment:   c.SegmentIndex,
			Magnitude: c.Magnitude,
			Phase:     c.PhaseDeg,
		}
	}

	return SimulateResponse{
		Impedance:          ImpedanceDTO{R: r.Impedance.R, X: r.Impedance.X},
		SWR:                r.SWR,
		Reflection:         ReflectionDTO{Re: real(r.Reflection), Im: imag(r.Reflection)},
		ReferenceImpedance: r.ReferenceImpedance,
		GainDBi:            r.GainDBi,
		Metrics:            r.Metrics,
		PolarCuts:          r.Cuts,
		Pattern:            pattern,
		Currents:           currents,
		Warnings:           r.Warnings,
	}
}

// SweepResultToResponse converts the MoM solver's sweep result into the API
// response DTO. Frequencies are in MHz, matching the frontend's display units.
func SweepResultToResponse(r *mom.SweepResult) SweepResponse {
	impedance := make([]ImpedanceDTO, len(r.Impedance))
	for i, z := range r.Impedance {
		impedance[i] = ImpedanceDTO{R: z.R, X: z.X}
	}

	refls := make([]ReflectionDTO, len(r.Reflections))
	for i, g := range r.Reflections {
		refls[i] = ReflectionDTO{Re: real(g), Im: imag(g)}
	}

	return SweepResponse{
		Frequencies:        r.Frequencies,
		SWR:                r.SWR,
		Impedance:          impedance,
		Reflections:        refls,
		ReferenceImpedance: r.ReferenceImpedance,
		Warnings:           r.Warnings,
	}
}
