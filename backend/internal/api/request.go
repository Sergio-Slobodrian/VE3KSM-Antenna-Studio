// Package api defines the HTTP API layer for the VE3KSM Antenna Studio backend.
// It contains request/response DTOs, Gin handlers, and middleware.
package api

import (
	"fmt"
	"math"

	"antenna-studio/backend/internal/mom"
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
	Loads        []LoadDTO `json:"loads,omitempty"`
	TransmissionLines []TransmissionLineDTO `json:"transmission_lines,omitempty"`
	// ReferenceImpedance (Ω) for VSWR and Smith-chart reflection coefficient.
	// Zero or omitted → 50 Ω default.
	ReferenceImpedance float64 `json:"reference_impedance,omitempty"`
}

// LoadDTO describes a lumped R/L/C load attached to a single segment.
// Topology is "series_rlc" (default) or "parallel_rlc".  Any subset of
// R (Ω), L (H), and C (F) may be non-zero; zero-valued components are
// omitted from the impedance/admittance combination, so a single
// non-zero field models a pure resistor, inductor, or capacitor.
type LoadDTO struct {
	WireIndex    int     `json:"wire_index"`
	SegmentIndex int     `json:"segment_index"`
	Topology     string  `json:"topology"`
	R            float64 `json:"r"`
	L            float64 `json:"l"`
	C            float64 `json:"c"`
}

// TLEndDTO references one end of a transmission-line element.  When
// WireIndex is >= 0 the end attaches to a (wire, segment) on the
// antenna model; -1 = shorted termination, -2 = open termination.
type TLEndDTO struct {
	WireIndex    int `json:"wire_index"`
	SegmentIndex int `json:"segment_index"`
}

// TransmissionLineDTO describes a NEC-style 2-port transmission-line
// element connecting two ends.  Stubs use the special A end with a
// regular wire/segment and the B end set to -1 (short) or -2 (open).
type TransmissionLineDTO struct {
	A              TLEndDTO `json:"a"`
	B              TLEndDTO `json:"b"`
	Z0             float64  `json:"z0"`
	Length         float64  `json:"length"`
	VelocityFactor float64  `json:"velocity_factor"`
	LossDbPerM     float64  `json:"loss_db_per_m"`
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
	// Material name from mom.MaterialLibrary (e.g. "copper", "aluminum").
	// Empty / omitted = perfect conductor (lossless).
	Material string `json:"material,omitempty"`
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
	Loads     []LoadDTO `json:"loads,omitempty"`
	TransmissionLines []TransmissionLineDTO `json:"transmission_lines,omitempty"`
	FreqStart float64   `json:"freq_start" binding:"required,gt=0"`
	FreqEnd   float64   `json:"freq_end" binding:"required,gtfield=FreqStart"`
	FreqSteps int       `json:"freq_steps" binding:"required,min=2,max=500"`
	// SweepMode picks "exact", "interpolated", or "" (auto).  Auto
	// switches to interpolated when freq_steps > 32.
	SweepMode    string `json:"sweep_mode,omitempty"`
	SweepAnchors int    `json:"sweep_anchors,omitempty"`
	// ReferenceImpedance (Ω) for VSWR.  Zero or omitted → 50 Ω.
	ReferenceImpedance float64 `json:"reference_impedance,omitempty"`
}

// ToSimulateRequest converts a SweepRequest into a SimulateRequest using
// the sweep start frequency. This lets us reuse SimulateRequest.Validate()
// for checking wire geometry, ground config, and source references.
func (s *SweepRequest) ToSimulateRequest() SimulateRequest {
	return SimulateRequest{
		Wires:              s.Wires,
		FrequencyMHz:       s.FreqStart,
		Ground:             s.Ground,
		Source:             s.Source,
		Loads:              s.Loads,
		TransmissionLines:  s.TransmissionLines,
		ReferenceImpedance: s.ReferenceImpedance,
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
//   - Each lumped load points at a real wire/segment with valid topology and
//     non-negative component values
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
		if w.Material != "" {
			if _, ok := mom.LookupMaterial(mom.MaterialName(w.Material)); !ok {
				return fmt.Errorf("wire %d: unknown material %q", i, w.Material)
			}
		}
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

	// Validate any lumped loads.  Topology defaults to "series_rlc" so that
	// a bare {wire_index, segment_index, r} JSON object works as a pure
	// resistor terminator without extra ceremony.
	for i, ld := range r.Loads {
		if ld.WireIndex < 0 || ld.WireIndex >= len(r.Wires) {
			return fmt.Errorf("load %d: wire_index %d out of range [0, %d)",
				i, ld.WireIndex, len(r.Wires))
		}
		w := r.Wires[ld.WireIndex]
		if w.Segments < 2 {
			return fmt.Errorf("load %d: wire %d has %d segments; need ≥2 to attach a load",
				i, ld.WireIndex, w.Segments)
		}
		if ld.SegmentIndex < 0 || ld.SegmentIndex >= w.Segments {
			return fmt.Errorf("load %d: segment_index %d out of range [0, %d) for wire %d",
				i, ld.SegmentIndex, w.Segments, ld.WireIndex)
		}
		topo := ld.Topology
		if topo == "" {
			topo = "series_rlc"
		}
		if topo != "series_rlc" && topo != "parallel_rlc" {
			return fmt.Errorf("load %d: invalid topology %q (must be series_rlc or parallel_rlc)", i, ld.Topology)
		}
		if ld.R < 0 || ld.L < 0 || ld.C < 0 {
			return fmt.Errorf("load %d: R, L, C must be non-negative", i)
		}
		if topo == "parallel_rlc" && ld.R == 0 && ld.L == 0 && ld.C == 0 {
			return fmt.Errorf("load %d: parallel_rlc requires at least one of R, L, C to be non-zero", i)
		}
	}

	if r.ReferenceImpedance < 0 {
		return fmt.Errorf("reference_impedance must be non-negative, got %f", r.ReferenceImpedance)
	}

	// Transmission lines: A must point at a real wire/segment; B may be
	// a real end or a special termination (-1 short, -2 open).
	for i, tl := range r.TransmissionLines {
		if tl.Z0 <= 0 {
			return fmt.Errorf("transmission_line %d: z0 must be positive", i)
		}
		if tl.Length <= 0 {
			return fmt.Errorf("transmission_line %d: length must be positive", i)
		}
		if tl.VelocityFactor < 0 || tl.VelocityFactor > 1 {
			return fmt.Errorf("transmission_line %d: velocity_factor must be in [0, 1]", i)
		}
		if tl.LossDbPerM < 0 {
			return fmt.Errorf("transmission_line %d: loss_db_per_m must be non-negative", i)
		}
		if tl.A.WireIndex < 0 || tl.A.WireIndex >= len(r.Wires) {
			return fmt.Errorf("transmission_line %d: A.wire_index out of range", i)
		}
		wa := r.Wires[tl.A.WireIndex]
		if wa.Segments < 2 {
			return fmt.Errorf("transmission_line %d: A wire %d needs ≥ 2 segments", i, tl.A.WireIndex)
		}
		if tl.A.SegmentIndex < 0 || tl.A.SegmentIndex >= wa.Segments {
			return fmt.Errorf("transmission_line %d: A.segment_index out of range", i)
		}
		if tl.B.WireIndex >= 0 {
			if tl.B.WireIndex >= len(r.Wires) {
				return fmt.Errorf("transmission_line %d: B.wire_index out of range", i)
			}
			wb := r.Wires[tl.B.WireIndex]
			if wb.Segments < 2 {
				return fmt.Errorf("transmission_line %d: B wire %d needs ≥ 2 segments", i, tl.B.WireIndex)
			}
			if tl.B.SegmentIndex < 0 || tl.B.SegmentIndex >= wb.Segments {
				return fmt.Errorf("transmission_line %d: B.segment_index out of range", i)
			}
		} else if tl.B.WireIndex != -1 && tl.B.WireIndex != -2 {
			return fmt.Errorf("transmission_line %d: B.wire_index must be ≥ 0 (real end), -1 (short), or -2 (open); got %d", i, tl.B.WireIndex)
		}
	}

	return nil
}
