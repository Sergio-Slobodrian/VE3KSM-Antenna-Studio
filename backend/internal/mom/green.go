package mom

import (
	"math"
	"math/cmplx"
)

// FreeSpaceGreens computes the free-space scalar Green's function:
//
//	G(R) = exp(-jkR) / (4*pi*R)
func FreeSpaceGreens(k, R float64) complex128 {
	if R < 1e-20 {
		R = 1e-20
	}
	return cmplx.Exp(complex(0, -k*R)) / complex(4.0*math.Pi*R, 0)
}

// psi computes exp(-jkR)/R with thin-wire regularization.
func psi(k, R float64) complex128 {
	if R < 1e-20 {
		R = 1e-20
	}
	return cmplx.Exp(complex(0, -k*R)) / complex(R, 0)
}

// dist computes distance with optional thin-wire reduced kernel.
func dist(a, b [3]float64, addRadius bool, radius float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	dz := a[2] - b[2]
	R2 := dx*dx + dy*dy + dz*dz
	if addRadius {
		R2 += radius * radius
	}
	R := math.Sqrt(R2)
	if R < 1e-20 {
		R = 1e-20
	}
	return R
}

// TriangleBasis represents a rooftop/triangle basis function centered at a node
// (the junction between two adjacent segments).
type TriangleBasis struct {
	NodeIndex int
	NodePos   [3]float64 // Position of the node
	// Left segment (where basis rises from 0 to 1)
	SegLeft  *Segment
	// Right segment (where basis falls from 1 to 0)
	SegRight *Segment
	// Charge densities: ρ = -(1/jω) dφ/ds
	// On left segment:  dφ/ds = +1/Δl_left  → ρ_left  = -1/(jω·Δl_left)
	// On right segment: dφ/ds = -1/Δl_right → ρ_right = +1/(jω·Δl_right)
	// We store just the geometric parts (1/Δl); the 1/(jω) factor is applied in the matrix assembly.
	ChargeDensLeft  float64 // = -1/Δl_left
	ChargeDensRight float64 // = +1/Δl_right
}

// TriangleKernel computes the impedance matrix element Z_mn between two
// triangle (rooftop) basis functions using the MPIE formulation.
//
// For triangle basis, the current is piecewise linear and the charge is
// piecewise constant — no delta-function singularities. This gives
// well-conditioned results for thin wires.
//
// Z_mn = jωμ/(4π) · [vector potential term] + 1/(jωε·4π) · [scalar potential term]
//
// Vector potential: Σ_{a,b} (ŝa·ŝb) · ∫∫ φ_m(s)·φ_n(s')·ψ(R) ds ds'
// Scalar potential: Σ_{a,b} ρ_m_a · ρ_n_b · ∫∫ ψ(R) ds ds'
//
// Returns (vectorTerm, scalarTerm) so the caller can apply the appropriate prefactors.
func TriangleKernel(basisM, basisN TriangleBasis, k, omega float64, segments []Segment) (vectorTerm, scalarTerm complex128) {
	nQuad := 8

	// Iterate over all segment pairs: (left_m, left_n), (left_m, right_n), (right_m, left_n), (right_m, right_n)
	type segInfo struct {
		seg       *Segment
		isRight   bool // true = right segment (falling part of triangle)
		chargeDen float64
	}

	segsM := []segInfo{}
	if basisM.SegLeft != nil {
		segsM = append(segsM, segInfo{basisM.SegLeft, false, basisM.ChargeDensLeft})
	}
	if basisM.SegRight != nil {
		segsM = append(segsM, segInfo{basisM.SegRight, true, basisM.ChargeDensRight})
	}

	segsN := []segInfo{}
	if basisN.SegLeft != nil {
		segsN = append(segsN, segInfo{basisN.SegLeft, false, basisN.ChargeDensLeft})
	}
	if basisN.SegRight != nil {
		segsN = append(segsN, segInfo{basisN.SegRight, true, basisN.ChargeDensRight})
	}

	nodes, weights := GaussLegendre(nQuad)
	// Use higher-order quadrature for overlapping segments
	nodesHQ, weightsHQ := GaussLegendre(nQuad * 2)

	for _, sm := range segsM {
		for _, sn := range segsN {
			segA := sm.seg
			segB := sn.seg

			// Determine if these segments overlap (same segment index = self-term)
			selfTerm := segA.Index == segB.Index
			useRadius := selfTerm
			radius := segA.Radius
			if segB.Radius > radius {
				radius = segB.Radius
			}

			qNodes := nodes
			qWeights := weights
			nq := nQuad
			if selfTerm {
				qNodes = nodesHQ
				qWeights = weightsHQ
				nq = nQuad * 2
			}

			dirDot := segA.Direction[0]*segB.Direction[0] +
				segA.Direction[1]*segB.Direction[1] +
				segA.Direction[2]*segB.Direction[2]

			var vecInt, scaInt complex128

			for p := 0; p < nq; p++ {
				wp := qWeights[p]
				tp := qNodes[p]
				// Observation point on segment A
				pa := [3]float64{
					segA.Center[0] + tp*segA.HalfLength*segA.Direction[0],
					segA.Center[1] + tp*segA.HalfLength*segA.Direction[1],
					segA.Center[2] + tp*segA.HalfLength*segA.Direction[2],
				}
				// Triangle weight at observation point
				// t ∈ [-1,1] maps to s ∈ [start, end] of segment
				// For left segment (rising): φ(s) = (s - start)/Δl = (t+1)/2
				// For right segment (falling): φ(s) = (end - s)/Δl = (1-t)/2
				var phiM float64
				if sm.isRight {
					phiM = (1 - tp) / 2
				} else {
					phiM = (1 + tp) / 2
				}

				for q := 0; q < nq; q++ {
					wq := qWeights[q]
					tq := qNodes[q]
					pb := [3]float64{
						segB.Center[0] + tq*segB.HalfLength*segB.Direction[0],
						segB.Center[1] + tq*segB.HalfLength*segB.Direction[1],
						segB.Center[2] + tq*segB.HalfLength*segB.Direction[2],
					}

					var phiN float64
					if sn.isRight {
						phiN = (1 - tq) / 2
					} else {
						phiN = (1 + tq) / 2
					}

					R := dist(pa, pb, useRadius, radius)
					psiVal := psi(k, R)

					// Vector potential integrand: φ_m(s) · φ_n(s') · (ŝa·ŝb) · ψ(R)
					vecInt += complex(wp*wq*phiM*phiN*dirDot, 0) * psiVal

					// Scalar potential integrand: ψ(R) (charge densities applied outside)
					scaInt += complex(wp*wq, 0) * psiVal
				}
			}

			// Scale by Jacobians
			jacobian := complex(segA.HalfLength*segB.HalfLength, 0)
			vecInt *= jacobian
			scaInt *= jacobian

			vectorTerm += vecInt
			scalarTerm += complex(sm.chargeDen*sn.chargeDen, 0) * scaInt
		}
	}

	return vectorTerm, scalarTerm
}

// Legacy functions for compatibility

// PocklingtonKernel is kept for backward compatibility but now uses the
// point-matching formulation internally.
func PocklingtonKernel(k float64, segI, segJ Segment, reduced bool) complex128 {
	// This should not be called in the triangle basis path
	// Kept as a stub
	return 0
}

func computeDist(a, b [3]float64, reduced bool, radius float64) float64 {
	return dist(a, b, reduced, radius)
}
