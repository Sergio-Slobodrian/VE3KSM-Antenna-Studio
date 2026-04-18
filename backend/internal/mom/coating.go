package mom

import (
	"math"
	"math/cmplx"
)

// applyCoatingLoading adds the distributed impedance of a dielectric coating
// to the Z-matrix diagonal using the NEC-4 IS-card model (Popović 1991).
//
// For a wire of conductor radius a coated to outer radius b = a + thickness
// with relative permittivity εr and loss tangent tanδ, the coating presents
// a distributed series impedance per unit length:
//
//	Z'_coat = (jωμ₀/2π) · (1 − 1/ε_r*) · ln(b/a)
//
// where ε_r* = εr(1 − j·tanδ) is the complex permittivity.
//
// For εr > 1 the imaginary part of Z'_coat is positive (inductive), slowing
// the guided wave velocity and shifting resonance downward by 3–5% for typical
// PVC coatings.  Dielectric loss (tanδ > 0) adds a real resistive term.
//
// The impedance is distributed over each triangle basis support (two adjacent
// segments) with a 50/50 split, matching the convention in applyMaterialLoss.
// The real part (dielectric loss) is credited to lossPerBasis for the
// radiation-efficiency calculation.
//
// Skip condition: CoatingThickness ≤ 0 or CoatingEpsR ≤ 1 ⇒ wire is bare.
func applyCoatingLoading(zmat zMatSetter, wires []Wire, segments []Segment,
	wireSegOffsets, wireSegCounts, wireBasisOffsets []int,
	omega float64, lossPerBasis []float64) {

	for wi, w := range wires {
		if w.CoatingThickness <= 0 || w.CoatingEpsR <= 1.0 {
			continue
		}
		a := w.Radius
		b := a + w.CoatingThickness
		if a <= 0 {
			continue
		}
		lnba := math.Log(b / a)

		// Complex permittivity: ε_r* = εr(1 − j·tanδ)
		epsrStar := complex(w.CoatingEpsR, -w.CoatingEpsR*w.CoatingLossTan)

		// Per-unit-length impedance: Z'_coat = (jωμ₀/2π) · (1 − 1/ε_r*) · ln(b/a)
		jOmegaMu0Over2pi := complex(0, omega*Mu0/(2*math.Pi))
		zPerUnitLen := jOmegaMu0Over2pi * (1 - 1/epsrStar) * complex(lnba, 0)

		if cmplx.Abs(zPerUnitLen) == 0 {
			continue
		}

		segOff := wireSegOffsets[wi]
		nSeg := wireSegCounts[wi]
		basisOff := wireBasisOffsets[wi]

		// nSeg-1 interior basis nodes; basis k spans segments k and k+1.
		// Charge each basis with half of each adjacent segment's coating impedance.
		for k := 0; k < nSeg-1; k++ {
			seg1 := segments[segOff+k]
			seg2 := segments[segOff+k+1]
			len1 := 2 * seg1.HalfLength
			len2 := 2 * seg2.HalfLength
			zBasis := 0.5*zPerUnitLen*complex(len1, 0) + 0.5*zPerUnitLen*complex(len2, 0)
			bi := basisOff + k
			zmat.Add(bi, bi, zBasis)
			if lossPerBasis != nil && bi < len(lossPerBasis) {
				lossPerBasis[bi] += real(zBasis)
			}
		}
	}
}
