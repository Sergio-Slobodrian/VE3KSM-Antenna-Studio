// Package geometry provides antenna geometry types, validation, and preset
// templates. It is used by the api package to validate and generate wire
// structures before they are passed to the MoM solver.
//
// The DTO types here mirror those in the api package. They are duplicated
// (rather than imported) to avoid a circular dependency: api imports geometry
// for templates, so geometry cannot import api.
package geometry

import (
	"fmt"
	"math"
)

// WireDTO describes a single straight wire element in 3D space.
// Coordinates (X1,Y1,Z1) and (X2,Y2,Z2) are the wire endpoints in meters.
// Radius is the conductor radius in meters. Segments is the number of
// equal-length pieces the wire is divided into for the MoM discretization.
// This type is duplicated from api.WireDTO to avoid circular imports.
type WireDTO struct {
	X1       float64 `json:"x1"`
	Y1       float64 `json:"y1"`
	Z1       float64 `json:"z1"`
	X2       float64 `json:"x2"`
	Y2       float64 `json:"y2"`
	Z2       float64 `json:"z2"`
	Radius   float64 `json:"radius"`
	Segments int     `json:"segments"`
}

// SourceDTO describes the excitation source placement on the antenna.
// WireIndex selects the wire (0-based), SegmentIndex selects the segment
// on that wire, and Voltage is the applied voltage in volts.
type SourceDTO struct {
	WireIndex    int     `json:"wire_index"`
	SegmentIndex int     `json:"segment_index"`
	Voltage      float64 `json:"voltage"`
}

// GroundDTO describes the ground plane environment for the simulation.
// Type is one of "free_space", "perfect", or "real". For "real" ground,
// Conductivity (S/m) and Permittivity (relative dielectric constant) are required.
type GroundDTO struct {
	Type         string  `json:"type"`
	Conductivity float64 `json:"conductivity"`
	Permittivity float64 `json:"permittivity"`
}

// WireLength computes the Euclidean distance (in meters) between two 3D
// endpoints, which gives the physical length of a straight wire element.
func WireLength(x1, y1, z1, x2, y2, z2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// ValidateWire checks that a wire definition is physically valid for MoM simulation.
// It enforces three constraints:
//   - Non-zero length: degenerate wires (start == end) cannot carry current.
//   - Positive radius: needed for the thin-wire kernel's self-impedance term.
//   - Thin-wire ratio: the conductor radius must be less than half the segment
//     length (ratio < 0.5). The MoM thin-wire approximation assumes current
//     flows along a filament; when the radius is comparable to the segment
//     length, the assumption breaks and results become inaccurate.
func ValidateWire(w WireDTO) error {
	length := WireLength(w.X1, w.Y1, w.Z1, w.X2, w.Y2, w.Z2)
	if length < 1e-10 {
		return fmt.Errorf("wire has zero length")
	}

	if w.Radius <= 0 {
		return fmt.Errorf("wire radius must be positive, got %e", w.Radius)
	}

	if w.Segments < 1 {
		return fmt.Errorf("wire must have at least 1 segment, got %d", w.Segments)
	}

	// Thin-wire ratio check: the MoM kernel integrations assume a thin
	// filament. When radius/segment_length exceeds 0.5, the approximation
	// is no longer valid and impedance results become unreliable.
	segLen := length / float64(w.Segments)
	ratio := w.Radius / segLen
	if ratio > 0.5 {
		return fmt.Errorf("thin-wire ratio violated: radius/segment_length = %.3f (must be < 0.5)", ratio)
	}

	return nil
}
