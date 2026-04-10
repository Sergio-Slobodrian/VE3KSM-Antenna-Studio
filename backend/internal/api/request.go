package api

import (
	"fmt"
	"math"
)

// SimulateRequest is the input DTO for a single-frequency simulation.
type SimulateRequest struct {
	Wires        []WireDTO `json:"wires" binding:"required,min=1"`
	FrequencyMHz float64   `json:"frequency_mhz" binding:"required,gt=0"`
	Ground       GroundDTO `json:"ground"`
	Source       SourceDTO `json:"source" binding:"required"`
}

// WireDTO describes a single wire element.
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
type GroundDTO struct {
	Type         string  `json:"type"`
	Conductivity float64 `json:"conductivity"`
	Permittivity float64 `json:"permittivity"`
}

// SourceDTO describes the excitation source location and voltage.
type SourceDTO struct {
	WireIndex    int     `json:"wire_index"`
	SegmentIndex int     `json:"segment_index"`
	Voltage      float64 `json:"voltage"`
}

// SweepRequest describes a frequency sweep simulation. It duplicates
// wire/source/ground fields rather than embedding SimulateRequest so
// that the binding tag "required" on FrequencyMHz doesn't reject sweeps.
type SweepRequest struct {
	Wires     []WireDTO `json:"wires" binding:"required,min=1"`
	Ground    GroundDTO `json:"ground"`
	Source    SourceDTO `json:"source" binding:"required"`
	FreqStart float64   `json:"freq_start" binding:"required,gt=0"`
	FreqEnd   float64   `json:"freq_end" binding:"required,gtfield=FreqStart"`
	FreqSteps int       `json:"freq_steps" binding:"required,min=2,max=500"`
}

// ToSimulateRequest converts a SweepRequest into a SimulateRequest using
// the sweep start frequency so the shared Validate() logic can be reused.
func (s *SweepRequest) ToSimulateRequest() SimulateRequest {
	return SimulateRequest{
		Wires:        s.Wires,
		FrequencyMHz: s.FreqStart,
		Ground:       s.Ground,
		Source:        s.Source,
	}
}

// validGroundTypes lists the accepted ground type values.
var validGroundTypes = map[string]bool{
	"free_space": true,
	"perfect":    true,
	"real":       true,
}

// Validate performs semantic validation on the SimulateRequest beyond
// what Gin's binding tags handle.
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
		// Thin-wire approximation check: radius should be much smaller than segment length
		segLen := length / float64(w.Segments)
		if w.Radius > segLen/2 {
			return fmt.Errorf("wire %d: radius (%e m) too large relative to segment length (%e m); thin-wire approximation requires radius << segment length",
				i, w.Radius, segLen)
		}
	}

	// Validate ground type, default to free_space
	if r.Ground.Type == "" {
		r.Ground.Type = "free_space"
	}
	if !validGroundTypes[r.Ground.Type] {
		return fmt.Errorf("invalid ground type %q; must be one of: free_space, perfect, real", r.Ground.Type)
	}

	if r.Ground.Type == "real" {
		if r.Ground.Conductivity <= 0 {
			return fmt.Errorf("real ground requires positive conductivity")
		}
		if r.Ground.Permittivity <= 0 {
			return fmt.Errorf("real ground requires positive permittivity")
		}
	}

	// Validate source references
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
