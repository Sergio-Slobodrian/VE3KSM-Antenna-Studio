package mom

import (
	"fmt"
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/mat"
)

// TLEnd represents one end of a transmission line.  When WireIndex is
// >= 0 the end attaches to a (wire, segment) on the antenna model just
// like a lumped load.  Negative WireIndex selects a special termination:
//
//	TLEndShorted = -1   (short to ground)
//	TLEndOpen    = -2   (open termination)
//
// Stubs use one regular end and one termination end, collapsing the
// 2-port to an effective lumped load.
type TLEnd struct {
	WireIndex    int `json:"wire_index"`
	SegmentIndex int `json:"segment_index"`
}

const (
	TLEndShorted = -1
	TLEndOpen    = -2
)

// TransmissionLine is a NEC-style "TL card": a lossy two-port
// transmission line connecting two segments (or one segment plus a
// short / open termination).  The element is stamped into the Z-matrix
// at the two basis functions nearest the requested segments using the
// TL's Z-parameters:
//
//	Z11 = Z22 = Z0 · coth(γL)
//	Z12 = Z21 = Z0 / sinh(γL)
//
// where γ = α + jβ, β = ω · VF / c, α from LossDbPerM.
type TransmissionLine struct {
	A              TLEnd   `json:"a"`
	B              TLEnd   `json:"b"`
	Z0             float64 `json:"z0"`              // characteristic impedance (Ω)
	Length         float64 `json:"length"`          // physical length (m)
	VelocityFactor float64 `json:"velocity_factor"` // 0..1; default 1.0 if zero
	LossDbPerM     float64 `json:"loss_db_per_m"`   // attenuation (dB/m); 0 = lossless
}

// TLPropagation returns the complex propagation constant γ = α + jβ
// (rad/m) for the given line at frequency ω (rad/s).  α is derived from
// LossDbPerM (1 Np = 8.6859 dB).
func TLPropagation(tl TransmissionLine, omega float64) complex128 {
	vf := tl.VelocityFactor
	if vf <= 0 {
		vf = 1.0
	}
	beta := omega / (vf * C0)
	alphaNp := tl.LossDbPerM / 8.6858896380650365 // dB/m → Np/m
	return complex(alphaNp, beta)
}

// TLImpedanceMatrix returns the 2×2 Z-parameter matrix of the line at
// frequency ω.  The off-diagonal entries are zero for a degenerate
// (zero-length) line; in that case the line collapses to a direct
// interconnect and the caller should treat it as a wire.
//
// For an open-circuited stub (B is TLEndOpen) returns the input
// impedance as Z11 with Z12 = 0; similarly for a short.  This keeps
// the stub case as a one-port stamp on a single basis.
func TLImpedanceMatrix(tl TransmissionLine, omega float64) (z11, z12 complex128, err error) {
	if tl.Z0 <= 0 {
		return 0, 0, fmt.Errorf("Z0 must be positive, got %g", tl.Z0)
	}
	if tl.Length <= 0 {
		return 0, 0, fmt.Errorf("length must be positive, got %g", tl.Length)
	}
	gammaL := TLPropagation(tl, omega) * complex(tl.Length, 0)

	// Stub cases: B is open or shorted.  At exact resonant lengths
	// (λ/4 for shorted, λ/2 for open) the lossless stub impedance is
	// mathematically infinite and cmplx.Tanh overflows to NaN.  Clamp to
	// a large finite value so the linear solve still produces a sensible
	// result (a huge diagonal entry effectively isolates that basis,
	// which is the right physical behaviour).
	switch tl.B.WireIndex {
	case TLEndShorted:
		t := cmplx.Tanh(gammaL)
		z := complex(tl.Z0, 0) * t
		return clampLargeC(z), 0, nil
	case TLEndOpen:
		t := cmplx.Tanh(gammaL)
		if cmplx.Abs(t) < 1e-15 {
			// Half-wave open stub looks like a short.
			return 0, 0, nil
		}
		z := complex(tl.Z0, 0) / t
		return clampLargeC(z), 0, nil
	}

	sh := cmplx.Sinh(gammaL)
	if cmplx.Abs(sh) < 1e-30 {
		return 0, 0, fmt.Errorf("degenerate line: sinh(γL) ≈ 0 at ω=%g", omega)
	}
	z0c := complex(tl.Z0, 0)
	z11 = clampLargeC(z0c / cmplx.Tanh(gammaL)) // Z0·coth(γL)
	z12 = clampLargeC(z0c / sh)
	return z11, z12, nil
}

// applyTransmissionLines stamps every TL onto the Z-matrix.  Two-port
// TLs add to four entries (m,m), (n,n), (m,n), (n,m); stubs add to a
// single diagonal entry on the connected basis.  The resistive parts of
// Z11 and Z22 contribute to the radiation-efficiency loss accounting
// in the same way lumped loads and skin-effect losses do.
func applyTransmissionLines(Z *mat.CDense, tls []TransmissionLine, omega float64,
	wires []Wire, wireSegCounts, wireBasisOffsets []int,
	lossPerBasis []float64) error {

	for ti, tl := range tls {
		basisA, err := resolveTLBasis(tl.A, wires, wireSegCounts, wireBasisOffsets, "A")
		if err != nil {
			return fmt.Errorf("transmission_line %d: %w", ti, err)
		}
		z11, z12, err := TLImpedanceMatrix(tl, omega)
		if err != nil {
			return fmt.Errorf("transmission_line %d: %w", ti, err)
		}
		if !finiteC(z11) || !finiteC(z12) {
			return fmt.Errorf("transmission_line %d: non-finite Z (%v, %v)", ti, z11, z12)
		}

		stub := tl.B.WireIndex < 0
		if stub {
			Z.Set(basisA, basisA, Z.At(basisA, basisA)+z11)
			if lossPerBasis != nil && basisA < len(lossPerBasis) {
				lossPerBasis[basisA] += real(z11)
			}
			continue
		}

		basisB, err := resolveTLBasis(tl.B, wires, wireSegCounts, wireBasisOffsets, "B")
		if err != nil {
			return fmt.Errorf("transmission_line %d: %w", ti, err)
		}
		Z.Set(basisA, basisA, Z.At(basisA, basisA)+z11)
		Z.Set(basisB, basisB, Z.At(basisB, basisB)+z11) // Z22 == Z11 for symmetric TL
		Z.Set(basisA, basisB, Z.At(basisA, basisB)+z12)
		Z.Set(basisB, basisA, Z.At(basisB, basisA)+z12)
		if lossPerBasis != nil {
			if basisA < len(lossPerBasis) {
				lossPerBasis[basisA] += real(z11)
			}
			if basisB < len(lossPerBasis) {
				lossPerBasis[basisB] += real(z11)
			}
		}
	}
	return nil
}

// resolveTLBasis maps a TLEnd to the global basis index, using the
// same nearest-interior-node rule the source and lumped-load placement
// use.  side is "A" or "B" only for error messages.
func resolveTLBasis(end TLEnd, wires []Wire, wireSegCounts, wireBasisOffsets []int, side string) (int, error) {
	if end.WireIndex < 0 || end.WireIndex >= len(wires) {
		return 0, fmt.Errorf("end %s: wire_index %d out of range [0, %d)", side, end.WireIndex, len(wires))
	}
	nSeg := wireSegCounts[end.WireIndex]
	if nSeg < 2 {
		return 0, fmt.Errorf("end %s: wire %d has %d segments; need ≥ 2", side, end.WireIndex, nSeg)
	}
	if end.SegmentIndex < 0 || end.SegmentIndex >= nSeg {
		return 0, fmt.Errorf("end %s: segment_index %d out of range [0, %d)", side, end.SegmentIndex, nSeg)
	}
	nodeIdx := end.SegmentIndex + 1
	if nodeIdx < 1 {
		nodeIdx = 1
	}
	if nodeIdx > nSeg-1 {
		nodeIdx = nSeg - 1
	}
	return wireBasisOffsets[end.WireIndex] + (nodeIdx - 1), nil
}

func finiteC(z complex128) bool {
	return !math.IsNaN(real(z)) && !math.IsNaN(imag(z)) &&
		!math.IsInf(real(z), 0) && !math.IsInf(imag(z), 0)
}

// clampLargeC substitutes a large finite value for NaN/±Inf in either
// the real or imaginary part.  This keeps the matrix solver well-
// conditioned at exact stub-resonance lengths without changing the
// observable physics (a 1e12 Ω diagonal entry is functionally
// indistinguishable from infinity for any practical antenna).
func clampLargeC(z complex128) complex128 {
	const big = 1e12
	re := real(z)
	im := imag(z)
	if math.IsNaN(re) || math.IsInf(re, 0) {
		re = big
	}
	if math.IsNaN(im) || math.IsInf(im, 0) {
		im = big
	}
	return complex(re, im)
}
