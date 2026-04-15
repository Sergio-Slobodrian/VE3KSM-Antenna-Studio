package mom

import (
	"math"
	"math/cmplx"
)

// FreeSpaceGreens computes the free-space scalar Green's function:
//
//	G(r, r') = exp(-jkR) / (4πR)
//
// where R = |r - r'| is the distance between source and observation points,
// k = 2πf/c is the wavenumber (rad/m), and j is the imaginary unit.
// This is the fundamental solution to the Helmholtz equation in 3D and
// represents the field radiated by a point source in free space.
// A minimum distance clamp of 1e-20 prevents division by zero.
func FreeSpaceGreens(k, R float64) complex128 {
	if R < 1e-20 {
		R = 1e-20
	}
	return cmplx.Exp(complex(0, -k*R)) / complex(4.0*math.Pi*R, 0)
}

// psi computes the "reduced" Green's function kernel ψ(R) = exp(-jkR)/R,
// which is the Green's function without the 1/(4π) normalization factor.
// The 1/(4π) is instead folded into the impedance matrix prefactors for clarity.
// A minimum distance clamp of 1e-20 prevents singularity at R=0.
func psi(k, R float64) complex128 {
	if R < 1e-20 {
		R = 1e-20
	}
	return cmplx.Exp(complex(0, -k*R)) / complex(R, 0)
}

// dist computes the Euclidean distance between two 3D points a and b.
// When addRadius is true, the wire radius is added in quadrature to the
// geometric distance: R = sqrt(|a-b|^2 + a^2). This implements the
// "reduced kernel" approximation for thin-wire self-terms, which avoids
// the 1/R singularity when source and observation points coincide on the
// same wire by displacing the source to the wire surface.
func dist(a, b [3]float64, addRadius bool, radius float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	dz := a[2] - b[2]
	R2 := dx*dx + dy*dy + dz*dz
	if addRadius {
		// Thin-wire approximation: source current is on the axis, but the
		// field is evaluated on the wire surface at distance = radius
		R2 += radius * radius
	}
	R := math.Sqrt(R2)
	if R < 1e-20 {
		R = 1e-20
	}
	return R
}

// TriangleBasis represents a rooftop (triangle) basis function centered at a node,
// which is the junction between two adjacent segments on the same wire.
//
// The triangle basis function is piecewise linear: it rises from 0 at the far end
// of the left segment to 1 at the node, then falls back to 0 at the far end of the
// right segment. This "rooftop" shape ensures current continuity at segment junctions
// and enforces zero current at wire endpoints (boundary condition for open-ended wires).
//
// Using triangle basis functions (instead of pulse basis) makes the charge density
// piecewise constant rather than containing Dirac delta singularities, yielding a
// much better-conditioned impedance matrix.
type TriangleBasis struct {
	NodeIndex int        // index of this basis function in the global basis array
	NodePos   [3]float64 // 3D position of the node (junction between left and right segments)

	SegLeft  *Segment // left segment: basis function rises from 0 (start) to 1 (node)
	SegRight *Segment // right segment: basis function falls from 1 (node) to 0 (end)

	// Charge density coefficients (geometric part only).
	// From the continuity equation, charge density ρ = -(1/jω) · dI/ds.
	// Since I(s) is piecewise linear with slope ±1/Δl, the charge is piecewise constant:
	//   Left segment:  dI/ds = +1/Δl_left  → ρ = -1/(jω·Δl_left)
	//   Right segment: dI/ds = -1/Δl_right → ρ = +1/(jω·Δl_right)
	// We store only the geometric factors here; the 1/(jω) is applied during matrix assembly.
	ChargeDensLeft  float64 // = -1/Δl_left  (negative: current increasing → charge depleting)
	ChargeDensRight float64 // = +1/Δl_right (positive: current decreasing → charge accumulating)
}

// TriangleKernel computes the impedance matrix element Z_mn between two
// triangle (rooftop) basis functions using the Mixed Potential Integral
// Equation (MPIE) formulation.
//
// The MPIE separates the impedance into vector potential (A) and scalar
// potential (Φ) contributions:
//
//	Z_mn = jωμ/(4π) · A_term + 1/(jωε·4π) · Φ_term
//
// where:
//
//	A_term = Σ_{a∈m} Σ_{b∈n} (ŝa·ŝb) ∫∫ φ_m(s)·φ_n(s')·ψ(R) ds ds'
//	Φ_term = Σ_{a∈m} Σ_{b∈n} ρ_a · ρ_b ∫∫ ψ(R) ds ds'
//
// The sums are over the segment pairs (up to 2x2 = 4 pairs for two full
// triangle bases). φ_m, φ_n are the piecewise-linear current shape functions,
// ρ_a, ρ_b are the piecewise-constant charge densities, ŝ are segment unit
// directions, and ψ(R) = exp(-jkR)/R is the reduced Green's function.
//
// The double integrals are evaluated via Gauss-Legendre quadrature, with
// higher order (16-point) used for self-terms to handle the near-singularity.
//
// Returns (vectorTerm, scalarTerm) separately so the caller can apply the
// frequency-dependent prefactors jωμ/(4π) and 1/(jωε·4π).
func TriangleKernel(basisM, basisN TriangleBasis, k, omega float64, segments []Segment) (vectorTerm, scalarTerm complex128) {
	nQuad := 8 // base quadrature order for off-diagonal terms

	// segInfo bundles a segment pointer with its role (left/right half of the
	// triangle) and its piecewise-constant charge density coefficient.
	type segInfo struct {
		seg       *Segment
		isRight   bool    // true = right segment (falling part of triangle)
		chargeDen float64 // geometric charge density factor (±1/Δl)
	}

	// Collect the segments that make up each basis function (1 or 2 segments each).
	// A basis at a wire tip may have only one segment (half-triangle).
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

	// Precompute quadrature rules: standard order for well-separated segments,
	// double order for self-terms where the 1/R kernel is nearly singular.
	nodes, weights := GaussLegendre(nQuad)
	nodesHQ, weightsHQ := GaussLegendre(nQuad * 2)

	// Loop over all segment-pair combinations between the two basis functions
	for _, sm := range segsM {
		for _, sn := range segsN {
			segA := sm.seg // observation segment (testing function)
			segB := sn.seg // source segment (basis function)

			// Self-term detection: when source and observation are the same segment,
			// use the reduced kernel (add wire radius) and higher quadrature order
			// to handle the near-singularity of ψ(R) as R→0.
			selfTerm := segA.Index == segB.Index
			useRadius := selfTerm
			radius := segA.Radius
			if segB.Radius > radius {
				radius = segB.Radius
			}

			// Select quadrature order based on whether this is a self-term
			qNodes := nodes
			qWeights := weights
			nq := nQuad
			if selfTerm {
				qNodes = nodesHQ
				qWeights = weightsHQ
				nq = nQuad * 2
			}

			// Dot product of segment directions: ŝa · ŝb
			// This projects the source current onto the observation segment direction,
			// accounting for the vector nature of the magnetic vector potential.
			dirDot := segA.Direction[0]*segB.Direction[0] +
				segA.Direction[1]*segB.Direction[1] +
				segA.Direction[2]*segB.Direction[2]

			var vecInt, scaInt complex128

			// Double Gauss-Legendre quadrature over both segments.
			// The parametric variable t ∈ [-1, 1] maps to physical position along a
			// segment via: r(t) = center + t*halfLength*direction
			for p := 0; p < nq; p++ {
				wp := qWeights[p]
				tp := qNodes[p]
				// Observation (test) point on segment A
				pa := [3]float64{
					segA.Center[0] + tp*segA.HalfLength*segA.Direction[0],
					segA.Center[1] + tp*segA.HalfLength*segA.Direction[1],
					segA.Center[2] + tp*segA.HalfLength*segA.Direction[2],
				}

				// Evaluate the triangle basis weight φ_m at the observation point.
				// The parametric variable t ∈ [-1,1] maps to the normalized position
				// along the segment: t=-1 is the segment start, t=+1 is the segment end.
				//   Left segment (rising):  φ(t) = (t+1)/2   (0 at start, 1 at node)
				//   Right segment (falling): φ(t) = (1-t)/2   (1 at node, 0 at end)
				var phiM float64
				if sm.isRight {
					phiM = (1 - tp) / 2
				} else {
					phiM = (1 + tp) / 2
				}

				for q := 0; q < nq; q++ {
					wq := qWeights[q]
					tq := qNodes[q]
					// Source point on segment B
					pb := [3]float64{
						segB.Center[0] + tq*segB.HalfLength*segB.Direction[0],
						segB.Center[1] + tq*segB.HalfLength*segB.Direction[1],
						segB.Center[2] + tq*segB.HalfLength*segB.Direction[2],
					}

					// Triangle basis weight φ_n at the source point
					var phiN float64
					if sn.isRight {
						phiN = (1 - tq) / 2
					} else {
						phiN = (1 + tq) / 2
					}

					// Distance between observation and source points (with optional
					// thin-wire radius offset for self-terms)
					R := dist(pa, pb, useRadius, radius)
					psiVal := psi(k, R)

					// Vector potential integrand: φ_m · φ_n · (ŝa·ŝb) · ψ(R)
					vecInt += complex(wp*wq*phiM*phiN*dirDot, 0) * psiVal

					// Scalar potential integrand: ψ(R) weighted by quadrature only;
					// the charge density coefficients are applied after the integral.
					scaInt += complex(wp*wq, 0) * psiVal
				}
			}

			// Apply the Jacobians from the change of variables: ds = halfLength · dt
			// for both the observation and source integrals.
			jacobian := complex(segA.HalfLength*segB.HalfLength, 0)
			vecInt *= jacobian
			scaInt *= jacobian

			vectorTerm += vecInt
			// Multiply the scalar integral by both charge density coefficients
			scalarTerm += complex(sm.chargeDen*sn.chargeDen, 0) * scaInt
		}
	}

	return vectorTerm, scalarTerm
}

// --- Legacy functions retained for backward compatibility ---

// PocklingtonKernel was the original point-matching kernel used before the
// switch to triangle basis functions. It is retained as a stub for interface
// compatibility (used in BuildZMatrix) but returns zero. The triangle basis
// path in solver.go uses TriangleKernel instead.
func PocklingtonKernel(k float64, segI, segJ Segment, reduced bool) complex128 {
	return 0
}

// computeDist is a legacy wrapper around dist, kept for backward compatibility.
func computeDist(a, b [3]float64, reduced bool, radius float64) float64 {
	return dist(a, b, reduced, radius)
}
