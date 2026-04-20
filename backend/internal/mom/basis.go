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

package mom

import "math"

// BasisOrder selects the type of current expansion function used by the MoM.
//
//   - BasisTriangle ("triangle") — piecewise-linear rooftop.  The classical
//     choice with N-1 unknowns per N segments.  Simple, robust, well-understood.
//     Needs ~10 segments/λ for 1% accuracy.
//
//   - BasisSinusoidal ("sinusoidal") — piecewise-sinusoidal (King-type).  The
//     current on each sub-domain follows sin(k·(Δl-|s|))/sin(k·Δl), which
//     matches the physical thin-wire current much more closely.  Achieves
//     comparable accuracy with ~3–5 segments/λ, cutting unknowns 3–5×.
//
//   - BasisQuadratic ("quadratic") — piecewise-quadratic (second-order).
//     Each sub-domain carries a·t² + b·t + c shape, adding one degree of
//     freedom per segment.  Useful for broadband or electrically-large structures
//     where the current shape is complex.
type BasisOrder string

const (
	BasisTriangle   BasisOrder = "triangle"
	BasisSinusoidal BasisOrder = "sinusoidal"
	BasisQuadratic  BasisOrder = "quadratic"
)

// BasisFunc is an abstraction over shape functions that can be evaluated
// at a parametric coordinate t ∈ [-1, +1] on a segment.
//
// The interface provides:
//   - Phi(t) — current shape function value at parametric point t
//   - ChargeDens() — charge density coefficient (geometric factor only;
//     the 1/(jω) is applied by the caller)
//   - InterpolateWeight() — weight at segment centre (t=0) for mapping
//     basis coefficients to segment-level currents
type BasisFunc interface {
	Phi(t float64) float64
	ChargeDens() float64
	InterpolateWeight() float64
}

// ──────────────────────────────────────────────────────────────────────
// Triangle (piecewise-linear) shape functions
// ──────────────────────────────────────────────────────────────────────

// TriangleLeft is the rising half: φ(t) = (1+t)/2.
// At t=-1 (segment start) φ=0; at t=+1 (node) φ=1.
type TriangleLeft struct {
	HalfLen float64
}

func (f TriangleLeft) Phi(t float64) float64         { return (1 + t) / 2 }
func (f TriangleLeft) ChargeDens() float64            { return -1.0 / (2.0 * f.HalfLen) }
func (f TriangleLeft) InterpolateWeight() float64     { return 0.5 }

// TriangleRight is the falling half: φ(t) = (1-t)/2.
// At t=-1 (node) φ=1; at t=+1 (segment end) φ=0.
type TriangleRight struct {
	HalfLen float64
}

func (f TriangleRight) Phi(t float64) float64         { return (1 - t) / 2 }
func (f TriangleRight) ChargeDens() float64            { return 1.0 / (2.0 * f.HalfLen) }
func (f TriangleRight) InterpolateWeight() float64     { return 0.5 }

// ──────────────────────────────────────────────────────────────────────
// Sinusoidal (King-type piecewise-sin) shape functions
// ──────────────────────────────────────────────────────────────────────
//
// The piecewise-sinusoidal basis (Richmond 1965, King 1969) is:
//
//   Left segment (rising):
//     φ(s) = sin(k·s) / sin(k·Δl)        s ∈ [0, Δl]
//   Right segment (falling):
//     φ(s) = sin(k·(Δl - s)) / sin(k·Δl) s ∈ [0, Δl]
//
// Mapped to parametric t ∈ [-1, +1]:
//   s = halfLen · (1 + t)         (left segment)
//   s = halfLen · (1 - t)         (right segment — reversed)
//
// The charge density from ∇·J = -jωρ for sinusoidal current:
//   dI/ds = k·cos(k·s)/sin(k·Δl)   →  ρ = -k/(jω·sin(k·Δl)) · cos(k·s)
//
// For the MPIE scalar potential integral we use the average charge density
// over the segment, which is:
//   ρ_avg = -(1/Δl) · [I(Δl) - I(0)] / (jω)
//
// For the left segment: I(0)=0, I(Δl)=1  →  ρ_avg = -1/(jω·Δl) · 1
// For the right segment: I(0)=1, I(Δl)=0  →  ρ_avg = +1/(jω·Δl) · 1
//
// So the piecewise-constant charge density is identical in sign and magnitude
// to the triangle basis. The improvement is entirely in the vector potential
// integral where φ(t) captures the sinusoidal current shape.

// SineLeft is the rising sinusoidal half: φ(t) = sin(k·halfLen·(1+t)) / sin(k·2·halfLen).
type SineLeft struct {
	HalfLen float64
	K       float64 // wavenumber
}

func (f SineLeft) Phi(t float64) float64 {
	kDelta := f.K * 2.0 * f.HalfLen
	sinDenom := math.Sin(kDelta)
	if math.Abs(sinDenom) < 1e-15 {
		// Degenerate (segment = half-wavelength): fall back to triangle
		return (1 + t) / 2
	}
	return math.Sin(f.K*f.HalfLen*(1+t)) / sinDenom
}

func (f SineLeft) ChargeDens() float64 {
	return -1.0 / (2.0 * f.HalfLen) // same as triangle
}

func (f SineLeft) InterpolateWeight() float64 {
	kDelta := f.K * 2.0 * f.HalfLen
	sinDenom := math.Sin(kDelta)
	if math.Abs(sinDenom) < 1e-15 {
		return 0.5
	}
	// Value at segment centre (t=0, i.e. s = halfLen)
	return math.Sin(f.K*f.HalfLen) / sinDenom
}

// SineRight is the falling sinusoidal half: φ(t) = sin(k·halfLen·(1-t)) / sin(k·2·halfLen).
type SineRight struct {
	HalfLen float64
	K       float64
}

func (f SineRight) Phi(t float64) float64 {
	kDelta := f.K * 2.0 * f.HalfLen
	sinDenom := math.Sin(kDelta)
	if math.Abs(sinDenom) < 1e-15 {
		return (1 - t) / 2
	}
	return math.Sin(f.K*f.HalfLen*(1-t)) / sinDenom
}

func (f SineRight) ChargeDens() float64 {
	return 1.0 / (2.0 * f.HalfLen)
}

func (f SineRight) InterpolateWeight() float64 {
	kDelta := f.K * 2.0 * f.HalfLen
	sinDenom := math.Sin(kDelta)
	if math.Abs(sinDenom) < 1e-15 {
		return 0.5
	}
	return math.Sin(f.K*f.HalfLen) / sinDenom
}

// ──────────────────────────────────────────────────────────────────────
// Quadratic (piecewise-parabolic) shape functions
// ──────────────────────────────────────────────────────────────────────
//
// The quadratic basis uses φ(t) = 1 - t² on the segment containing
// the node, giving a smoother current distribution than linear.
// Combined with the two half-bases on adjacent segments, this produces
// a 2nd-order expansion.
//
// For node-centred quadratic (single extra DOF per node):
//   Left segment (rising):  φ(t) = (1+t)²/4 · (3-t)/3  ≈ quadratic rise
//   Right segment (falling): φ(t) = (1-t)²/4 · (3+t)/3  ≈ quadratic fall
//
// Simplified to Hermite-like form:
//   Left:  φ(t) = (1+t)² · (2-t) / 4
//          φ(-1) = 0, φ(1) = 1, φ'(1) = 0 (smooth peak)
//   Right: φ(t) = (1-t)² · (2+t) / 4
//          φ(-1) = 1, φ(1) = 0, φ'(-1) = 0

type QuadraticLeft struct {
	HalfLen float64
}

func (f QuadraticLeft) Phi(t float64) float64 {
	u := 1 + t // u ∈ [0, 2]
	return u * u * (2 - t) / 4.0
}

func (f QuadraticLeft) ChargeDens() float64 {
	return -1.0 / (2.0 * f.HalfLen)
}

func (f QuadraticLeft) InterpolateWeight() float64 {
	return f.Phi(0) // 0.5 for Hermite
}

type QuadraticRight struct {
	HalfLen float64
}

func (f QuadraticRight) Phi(t float64) float64 {
	u := 1 - t
	return u * u * (2 + t) / 4.0
}

func (f QuadraticRight) ChargeDens() float64 {
	return 1.0 / (2.0 * f.HalfLen)
}

func (f QuadraticRight) InterpolateWeight() float64 {
	return f.Phi(0)
}

// ──────────────────────────────────────────────────────────────────────
// GeneralisedBasis — the unified basis function used by the higher-order
// kernel (GenKernel).  This is a thin wrapper around TriangleBasis that
// attaches BasisFunc instances for each half.
// ──────────────────────────────────────────────────────────────────────

// GenBasis extends TriangleBasis with abstract shape functions.
type GenBasis struct {
	TriangleBasis              // embed: NodeIndex, NodePos, SegLeft, SegRight, charge dens
	ShapeLeft     BasisFunc    // shape function for left segment (nil if no left seg)
	ShapeRight    BasisFunc    // shape function for right segment (nil if no right seg)
	Order         BasisOrder   // which family this belongs to
}

// BuildGenBases constructs generalised basis functions for the given order.
// It re-uses the TriangleBasis layout (same segment topology, same basis
// count, same node positions) and merely swaps the shape function objects.
func BuildGenBases(triBases []TriangleBasis, order BasisOrder, k float64) []GenBasis {
	out := make([]GenBasis, len(triBases))
	for i, tb := range triBases {
		gb := GenBasis{
			TriangleBasis: tb,
			Order:         order,
		}
		switch order {
		case BasisSinusoidal:
			if tb.SegLeft != nil {
				gb.ShapeLeft = SineLeft{HalfLen: tb.SegLeft.HalfLength, K: k}
			}
			if tb.SegRight != nil {
				gb.ShapeRight = SineRight{HalfLen: tb.SegRight.HalfLength, K: k}
			}
		case BasisQuadratic:
			if tb.SegLeft != nil {
				gb.ShapeLeft = QuadraticLeft{HalfLen: tb.SegLeft.HalfLength}
			}
			if tb.SegRight != nil {
				gb.ShapeRight = QuadraticRight{HalfLen: tb.SegRight.HalfLength}
			}
		default: // triangle
			if tb.SegLeft != nil {
				gb.ShapeLeft = TriangleLeft{HalfLen: tb.SegLeft.HalfLength}
			}
			if tb.SegRight != nil {
				gb.ShapeRight = TriangleRight{HalfLen: tb.SegRight.HalfLength}
			}
		}
		out[i] = gb
	}
	return out
}

// InterpolateGenSegmentCurrents maps basis coefficients to segment currents
// using the shape-function-specific interpolation weights (which differ from
// the constant 0.5 used by triangle bases for sinusoidal/quadratic bases).
func InterpolateGenSegmentCurrents(basisCurrents []complex128, genBases []GenBasis, segments []Segment) []complex128 {
	segI := make([]complex128, len(segments))
	for _, gb := range genBases {
		idx := gb.NodeIndex
		if idx >= len(basisCurrents) {
			continue
		}
		Ib := basisCurrents[idx]
		if gb.ShapeLeft != nil && gb.SegLeft != nil {
			segI[gb.SegLeft.Index] += Ib * complex(gb.ShapeLeft.InterpolateWeight(), 0)
		}
		if gb.ShapeRight != nil && gb.SegRight != nil {
			segI[gb.SegRight.Index] += Ib * complex(gb.ShapeRight.InterpolateWeight(), 0)
		}
	}
	return segI
}
