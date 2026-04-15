// Package api defines the HTTP API layer for the Antenna Studio backend.
// It contains request/response DTOs, Gin handlers, and middleware.
package api

import (
	"fmt"
	"math"
)

// SimulateRequest is the JSON body the frontend sends to POST /api/simulate.
// It describes a complete single-frequency MoM simulation: antenna geometry
// (wires), operating frequency, ground environment, and excitation source.
// Gin binding tags enforce structural constraints; Validate() handles semantic ones.
type SimulateRequest struct {
	Wires        []WireDTO `json:"wires" binding:"required,min=1"`
	FrequencyMHz float64   `json:"frequency_mhz" binding:"required,gt=0"`
	Ground       GroundDTO `json:"ground"`
	Source       SourceDTO `json:"source" binding:"required"`
}

// WireDTO describes a single straight wire element in 3D space.
// The wire runs from (X1,Y1,Z1) to (X2,Y2,Z2) and is discretized into
// Segments equal-length pieces for the MoM solver. Coordinates are in meters.
// Radius is the wire conductor radius in meters; Segments is capped at 200
// to keep the impedance matrix size manageable (N^2 memory).
type WireDTO struct {
	X1       float64 `json:"x1"`
	Y1       float64 `json:"y1"`
	Z1       float64 `json:"z1"`
	X2       float64 `json:"x2"`
	Y2       float64 `json:"y2"`
	Z2       float64 `json:"z2"`
	Radius   float64 `json:"radius" binding:"required,gt=0"`
	Segments int     `json:"segments" binding:"required,min=1,max=200"`
}

// GroundDTO describes the ground plane configuration.
// Type must be one of "free_space" (default), "perfect", or "real".
// For "real" ground, Conductivity (S/m) and Permittivity (relative)
// must both be positive; they are ignored for other ground types.
type GroundDTO struct {
	Type         string  `json:"type"`
	Conductivity float64 `json:"conductivity"`
	Permittivity float64 `json:"permittivity"`
}

// SourceDTO identifies the excitation point on the antenna structure.
// WireIndex selects which wire carries the source (0-based into the Wires slice).
// SegmentIndex selects which segment on that wire is the feed point (0-based).
// Voltage is the applied voltage magnitude in volts; 0 defaults to 1V in the solver.
type SourceDTO struct {
	WireIndex    int     `json:"wire_index"`
	SegmentIndex int     `json:"segment_index"`
	Voltage      float64 `json:"voltage"`
}

// SweepRequest is the JSON body for POST /api/sweep, which runs the MoM solver
// at multiple frequencies to produce SWR and impedance curves.
// It duplicates wire/source/ground fields rather than embedding SimulateRequest
// because Gin's binding tag "required" on FrequencyMHz would reject sweep
// requests (which use FreqStart/FreqEnd instead).
// FreqSteps is capped at 500 to bound total computation time.
type SweepRequest struct {
	Wires     []WireDTO `json:"wires" binding:"required,min=1"`
	Ground    GroundDTO `json:"ground"`
	Source    SourceDTO `json:"source" binding:"required"`
	FreqStart float64   `json:"freq_start" binding:"required,gt=0"`
	FreqEnd   float64   `json:"freq_end" binding:"required,gtfield=FreqStart"`
	FreqSteps int       `json:"freq_steps" binding:"required,min=2,max=500"`
}

// ToSimulateRequest converts a SweepRequest into a SimulateRequest using
// the sweep start frequency. This lets us reuse SimulateRequest.Validate()
// for checking wire geometry, ground config, and source references.
func (s *SweepRequest) ToSimulateRequest() SimulateRequest {
	return SimulateRequest{
		Wires:        s.Wires,
		FrequencyMHz: s.FreqStart,
		Ground:       s.Ground,
		Source:        s.Source,
	}
}

// validGroundTypes is the set of accepted ground type strings.
// Empty string is not listed here; Validate() normalizes it to "free_space".
var validGroundTypes = map[string]bool{
	"free_space": true,
	"perfect":    true,
	"real":       true,
}

// Validate performs semantic validation on the SimulateRequest that goes beyond
// what Gin's struct binding tags can express. It checks:
//   - At least one wire with non-zero length and positive radius
//   - Thin-wire approximation: wire radius must be less than half the segment length,
//     because the MoM kernel assumes current flows along a thin filament
//   - Ground type is valid; "real" ground has positive conductivity and permittivity
//   - Source wire_index and segment_index are within bounds of the wire array
//
// This method may mutate r.Ground.Type (normalizing "" to "free_space").
func (r *SimulateRequest) Validate() error {
	if len(r.Wires) == 0 {
		return fmt.Errorf("at least one wire is required")
	}

	if r.FrequencyMHz <= 0 {
		return fmt.Errorf("frequency must be positive, got %f", r.FrequencyMHz)
	}

	for i, w := range r.Wires {
		dx := w.X2 - w.X1
		dy := w.Y2 - w.Y1
		dz := w.Z2 - w.Z1
		length := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if length < 1e-10 {
			return fmt.Errorf("wire %d has zero length (start == end)", i)
		}
		if w.Radius <= 0 {
			return fmt.Errorf("wire %d radius must be positive, got %f", i, w.Radius)
		}
		if w.Segments < 1 {
			return fmt.Errorf("wire %d must have at least 1 segment, got %d", i, w.Segments)
		}
		// Thin-wire approximation: the MoM solver assumes current flows along
		// a filament; if the radius approaches the segment length, the kernel
		// integrals become inaccurate and results are physically meaningless.
		segLen := length / float64(w.Segments)
		if w.Radius > segLen/2 {
			return fmt.Errorf("wire %d: radius (%e m) too large relative to segment length (%e m); thin-wire approximation requires radius << segment length",
				i, w.Radius, segLen)
		}
	}

	// Normalize empty ground type to the default free-space environment
	if r.Ground.Type == "" {
		r.Ground.Type = "free_space"
	}
	if !validGroundTypes[r.Ground.Type] {
		return fmt.Errorf("invalid ground type %q; must be one of: free_space, perfect, real", r.Ground.Type)
	}

	// Real ground needs material properties for the Fresnel reflection coefficients
	if r.Ground.Type == "real" {
		if r.Ground.Conductivity <= 0 {
			return fmt.Errorf("real ground requires positive conductivity")
		}
		if r.Ground.Permittivity <= 0 {
			return fmt.Errorf("real ground requires positive permittivity")
		}
	}

	// Ensure the source references a valid wire and segment within that wire
	if r.Source.WireIndex < 0 || r.Source.WireIndex >= len(r.Wires) {
		return fmt.Errorf("source wire_index %d out of range [0, %d)", r.Source.WireIndex, len(r.Wires))
	}
	srcWire := r.Wires[r.Source.WireIndex]
	if r.Source.SegmentIndex < 0 || r.Source.SegmentIndex >= srcWire.Segments {
		return fmt.Errorf("source segment_index %d out of range [0, %d) for wire %d",
			r.Source.SegmentIndex, srcWire.Segments, r.Source.WireIndex)
	}

	return nil
}
