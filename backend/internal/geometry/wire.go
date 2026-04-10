package geometry

import (
	"fmt"
	"math"
)

// WireDTO describes a single wire element (duplicated from api to avoid circular imports).
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

// SourceDTO describes the excitation source location and voltage.
type SourceDTO struct {
	WireIndex    int     `json:"wire_index"`
	SegmentIndex int     `json:"segment_index"`
	Voltage      float64 `json:"voltage"`
}

// GroundDTO describes the ground plane configuration.
type GroundDTO struct {
	Type         string  `json:"type"`
	Conductivity float64 `json:"conductivity"`
	Permittivity float64 `json:"permittivity"`
}

// WireLength computes the Euclidean distance between two 3D points.
func WireLength(x1, y1, z1, x2, y2, z2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	dz := z2 - z1
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// ValidateWire checks that a wire has non-zero length, positive radius, and
// satisfies the thin-wire approximation constraint.
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

	// Thin-wire ratio check: radius should be much less than segment length
	segLen := length / float64(w.Segments)
	ratio := w.Radius / segLen
	if ratio > 0.5 {
		return fmt.Errorf("thin-wire ratio violated: radius/segment_length = %.3f (must be < 0.5)", ratio)
	}

	return nil
}
