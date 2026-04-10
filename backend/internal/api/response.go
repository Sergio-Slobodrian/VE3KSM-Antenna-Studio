package api

import "antenna-studio/backend/internal/mom"

// SimulateResponse is the JSON response for a single-frequency simulation.
type SimulateResponse struct {
	Impedance ImpedanceDTO    `json:"impedance"`
	SWR       float64         `json:"swr"`
	GainDBi   float64         `json:"gain_dbi"`
	Pattern   []PatternDTO    `json:"pattern"`
	Currents  []CurrentDTO    `json:"currents"`
}

// ImpedanceDTO holds resistance and reactance.
type ImpedanceDTO struct {
	R float64 `json:"r"`
	X float64 `json:"x"`
}

// PatternDTO is a single far-field pattern sample point.
type PatternDTO struct {
	Theta  float64 `json:"theta"`
	Phi    float64 `json:"phi"`
	GainDB float64 `json:"gain_db"`
}

// CurrentDTO holds per-segment current data.
type CurrentDTO struct {
	Segment   int     `json:"segment"`
	Magnitude float64 `json:"magnitude"`
	Phase     float64 `json:"phase"`
}

// SweepResponse is the JSON response for a frequency sweep.
type SweepResponse struct {
	Frequencies []float64      `json:"frequencies"`
	SWR         []float64      `json:"swr"`
	Impedance   []ImpedanceDTO `json:"impedance"`
}

// ErrorResponse wraps an error message for JSON output.
type ErrorResponse struct {
	Error string `json:"error"`
}

// SolverResultToResponse converts a mom.SolverResult to a SimulateResponse.
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
		Impedance: ImpedanceDTO{R: r.Impedance.R, X: r.Impedance.X},
		SWR:       r.SWR,
		GainDBi:   r.GainDBi,
		Pattern:   pattern,
		Currents:  currents,
	}
}

// SweepResultToResponse converts a mom.SweepResult to a SweepResponse.
func SweepResultToResponse(r *mom.SweepResult) SweepResponse {
	impedance := make([]ImpedanceDTO, len(r.Impedance))
	for i, z := range r.Impedance {
		impedance[i] = ImpedanceDTO{R: z.R, X: z.X}
	}

	return SweepResponse{
		Frequencies: r.Frequencies,
		SWR:         r.SWR,
		Impedance:   impedance,
	}
}
