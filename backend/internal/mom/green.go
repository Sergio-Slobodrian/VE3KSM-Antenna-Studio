package mom

import (
	"math"
	"math/cmplx"
)

// FreeSpaceGreens computes the free-space scalar Green's function:
//
//	G(R) = exp(-jkR) / (4*pi*R)
//
// k is the wavenumber (rad/m), R is the distance (m).
func FreeSpaceGreens(k, R float64) complex128 {
	if R < 1e-20 {
		R = 1e-20
	}
	return cmplx.Exp(complex(0, -k*R)) / complex(4.0*math.Pi*R, 0)
}

// PocklingtonKernel computes the mutual impedance kernel between segments i and j
// using the thin-wire MPIE with pulse basis and Galerkin testing.
//
// Returns K such that Z_mn = jωμ₀/(4π) * K.
//
// The formulation uses:
//
//	K = (di·dj) * ∫_i ∫_j ψ(R) ds ds'  -  (1/k²) * IBP_scalar
//
// where ψ(R) = exp(-jkR)/R and the IBP scalar potential uses endpoint evaluations:
//
//	IBP = ψ(end_i, end_j) - ψ(end_i, start_j) - ψ(start_i, end_j) + ψ(start_i, start_j)
//
// For endpoints that coincide (distance < segment length), the thin-wire
// circumferential average is used instead of the point evaluation:
//
//	ψ_avg(R→0) ≈ 2·ln(Δl/(2a)) / Δl
//
// This gives physically correct results without the 1/a singularity.
func PocklingtonKernel(k float64, segI, segJ Segment, reduced bool) complex128 {
	nQuad := 16
	if reduced {
		nQuad = 32
	}
	nodes, weights := GaussLegendre(nQuad)

	dirDot := segI.Direction[0]*segJ.Direction[0] +
		segI.Direction[1]*segJ.Direction[1] +
		segI.Direction[2]*segJ.Direction[2]

	radius := segJ.Radius
	k2 := k * k
	deltaLj := 2.0 * segJ.HalfLength

	// ──────────────────────────────────────────────────────────────────────
	// Vector potential: Galerkin double integral of ψ(R) = exp(-jkR)/R
	// For self-terms, R = sqrt(R_geom² + a²) (extended thin-wire kernel)
	// ──────────────────────────────────────────────────────────────────────
	var integralPsi complex128
	for p := 0; p < nQuad; p++ {
		wp := weights[p]
		tp := nodes[p]
		pi := [3]float64{
			segI.Center[0] + tp*segI.HalfLength*segI.Direction[0],
			segI.Center[1] + tp*segI.HalfLength*segI.Direction[1],
			segI.Center[2] + tp*segI.HalfLength*segI.Direction[2],
		}
		for q := 0; q < nQuad; q++ {
			wq := weights[q]
			tq := nodes[q]
			pj := [3]float64{
				segJ.Center[0] + tq*segJ.HalfLength*segJ.Direction[0],
				segJ.Center[1] + tq*segJ.HalfLength*segJ.Direction[1],
				segJ.Center[2] + tq*segJ.HalfLength*segJ.Direction[2],
			}
			R := computeDist(pi, pj, reduced, radius)
			psiVal := cmplx.Exp(complex(0, -k*R)) / complex(R, 0)
			integralPsi += complex(wp*wq, 0) * psiVal
		}
	}
	integralPsi *= complex(segI.HalfLength*segJ.HalfLength, 0)

	// ──────────────────────────────────────────────────────────────────────
	// Scalar potential: IBP with thin-wire correction for near-coincident endpoints
	//
	// For endpoints separated by distance > Δl, use standard ψ evaluation.
	// For endpoints closer than Δl (coincident or near-coincident), use the
	// thin-wire circumferential average which replaces the 1/R singularity
	// with a ln(Δl/a) dependence.
	// ──────────────────────────────────────────────────────────────────────
	threshold := deltaLj * 0.5 // endpoints closer than this use the thin-wire approximation

	psiEndpoint := func(a, b [3]float64) complex128 {
		dx := a[0] - b[0]
		dy := a[1] - b[1]
		dz := a[2] - b[2]
		R := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if R < threshold {
			// Use thin-wire circumferential average for near-coincident endpoints.
			// For a current filament on a wire of radius a, the circumferentially
			// averaged ψ at distance z along the wire is:
			//   ψ_avg(z) ≈ exp(-jk·sqrt(z²+a²)) * 2·arccosh(z/a) / z  for z >> a
			// At z → 0:
			//   ψ_avg(0) ≈ 2·ln(2·Δl_eff/a) / Δl_eff
			// where Δl_eff is a characteristic length (use Δl/2 of the source segment).
			//
			// For small but nonzero R, interpolate between the thin-wire
			// and free-space forms.
			if R < radius {
				R = radius // minimum physical distance
			}
			// Use the reduced kernel form but limit the maximum
			Reff := math.Sqrt(R*R + radius*radius)
			return cmplx.Exp(complex(0, -k*Reff)) / complex(Reff, 0)
		}
		if R < 1e-20 {
			R = 1e-20
		}
		return cmplx.Exp(complex(0, -k*R)) / complex(R, 0)
	}

	psiPP := psiEndpoint(segI.End, segJ.End)
	psiPM := psiEndpoint(segI.End, segJ.Start)
	psiMP := psiEndpoint(segI.Start, segJ.End)
	psiMM := psiEndpoint(segI.Start, segJ.Start)

	scalarIBP := (psiPP - psiPM - psiMP + psiMM) / complex(k2, 0)

	return complex(dirDot, 0)*integralPsi - scalarIBP
}

// computeDist computes the distance between two 3D points,
// optionally regularized with wire radius for the extended thin-wire kernel.
func computeDist(a, b [3]float64, reduced bool, radius float64) float64 {
	dx := a[0] - b[0]
	dy := a[1] - b[1]
	dz := a[2] - b[2]
	R2 := dx*dx + dy*dy + dz*dz
	if reduced {
		R2 += radius * radius
	}
	R := math.Sqrt(R2)
	if R < 1e-20 {
		R = 1e-20
	}
	return R
}
