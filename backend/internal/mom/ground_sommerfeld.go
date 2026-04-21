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

// ground_sommerfeld.go assembles the Z-matrix ground contributions using the
// full Sommerfeld half-space Green's function (sommerfeld.go) instead of the
// Bannister complex-image approximation.
//
// For each pair of triangle basis functions the scattered MPIE kernel is:
//
//	G_A^s(r,r') = j/(4π) · [I₀(ρ,z⁺) + I₂(ρ,z⁺)·cos(2φ₀)]   vector potential
//	G_Φ^s(r,r') = j/(4π) ·  IΦ(ρ,z⁺)                           scalar potential
//
// where ρ = |(r−r')_xy|, z⁺ = z_obs + z_src, and φ₀ is the azimuthal angle
// between the wire direction ŝ and the horizontal separation vector.
//
// The double integral over each basis-function pair follows the same
// Gauss-Legendre quadrature pattern as TriangleKernel (green.go): 8-point
// for off-diagonal entries, 16-point for near-self entries.
package mom

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

// addSommerfeldGroundBasis adds the rigorous Sommerfeld half-space scattered
// Green's function contributions to the impedance matrix Z for real lossy ground.
//
// This replaces addComplexImageGroundBasis when Ground.Method == "sommerfeld".
// The function signature is intentionally parallel to addComplexImageGroundBasis
// so the two can be swapped in solver.go with a one-line change.
func addSommerfeldGroundBasis(Z *mat.CDense, bases []TriangleBasis, segs []Segment, k, omega, sigma, epsilonR float64) {
	// MPIE prefactors (identical to buildTriangleZMatrix)
	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	// j/(4π) multiplier absorbed into the Sommerfeld integrals; the integrals
	// themselves return the dimensionless spectral sum — apply it here.
	// Per Michalski & Mosig (1997) the scattered kernel replaces the image ψ(R)
	// term, so the overall sign and prefactor structure is the same as the
	// free-space/PEC kernel but the Green's function comes from SommerfeldIntegrals.
	somPre := complex(0, 1.0/(4.0*math.Pi)) // j/(4π) for the scattered half-space kernel

	const nQuad = 8
	nodes, weights := GaussLegendre(nQuad)
	nodesHQ, weightsHQ := GaussLegendre(nQuad * 2)

	type segInfo struct {
		seg       *Segment
		isRight   bool
		chargeDen float64
	}

	n := len(bases)
	for i := 0; i < n; i++ {
		segsI := []segInfo{}
		if bases[i].SegLeft != nil {
			segsI = append(segsI, segInfo{bases[i].SegLeft, false, bases[i].ChargeDensLeft})
		}
		if bases[i].SegRight != nil {
			segsI = append(segsI, segInfo{bases[i].SegRight, true, bases[i].ChargeDensRight})
		}

		for j := 0; j < n; j++ {
			segsJ := []segInfo{}
			if bases[j].SegLeft != nil {
				segsJ = append(segsJ, segInfo{bases[j].SegLeft, false, bases[j].ChargeDensLeft})
			}
			if bases[j].SegRight != nil {
				segsJ = append(segsJ, segInfo{bases[j].SegRight, true, bases[j].ChargeDensRight})
			}

			var vecTerm, scaTerm complex128

			for _, si := range segsI {
				for _, sj := range segsJ {
					segA := si.seg
					segB := sj.seg

					selfTerm := segA.Index == segB.Index

					qNodes, qWeights, nq := nodes, weights, nQuad
					if selfTerm {
						qNodes, qWeights, nq = nodesHQ, weightsHQ, nQuad*2
					}

					// ŝa · ŝb — direction cosine between observation and source segments.
					dirDot := segA.Direction[0]*segB.Direction[0] +
						segA.Direction[1]*segB.Direction[1] +
						segA.Direction[2]*segB.Direction[2]

					var vecInt, scaInt complex128

					for p := 0; p < nq; p++ {
						wp := qWeights[p]
						tp := qNodes[p]
						pa := [3]float64{
							segA.Center[0] + tp*segA.HalfLength*segA.Direction[0],
							segA.Center[1] + tp*segA.HalfLength*segA.Direction[1],
							segA.Center[2] + tp*segA.HalfLength*segA.Direction[2],
						}
						var phiI float64
						if si.isRight {
							phiI = (1 - tp) / 2
						} else {
							phiI = (1 + tp) / 2
						}

						for q := 0; q < nq; q++ {
							wq := qWeights[q]
							tq := qNodes[q]
							pb := [3]float64{
								segB.Center[0] + tq*segB.HalfLength*segB.Direction[0],
								segB.Center[1] + tq*segB.HalfLength*segB.Direction[1],
								segB.Center[2] + tq*segB.HalfLength*segB.Direction[2],
							}
							var phiJ float64
							if sj.isRight {
								phiJ = (1 - tq) / 2
							} else {
								phiJ = (1 + tq) / 2
							}

							// Geometry for Sommerfeld integrals.
							dx := pa[0] - pb[0]
							dy := pa[1] - pb[1]
							rho := math.Sqrt(dx*dx + dy*dy)
							// z_sum = z_obs + z_src (both must be ≥ 0 over ground)
							zsum := pa[2] + pb[2]
							if zsum < 1e-9 {
								zsum = 1e-9
							}

							// Near-zero ρ: use quasi-static normal-incidence limit to
							// avoid slow convergence of J₂ term.
							var i0, i2, iPhi complex128
							if rho < 1e-9/k {
								// At ρ→0 the I₂ integral vanishes (J₂(0)=0) and
								// I₀ → R_TM(k₀)·exp(−γ₀·z_sum)/z_sum (leading term).
								// Fall back to the complex-image scalar for stability.
								i0, i2, iPhi = sommerfeldSmallRho(zsum, k, sigma, epsilonR)
							} else {
								i0, i2, iPhi = SommerfeldIntegrals(rho, zsum, k, sigma, epsilonR)

								// φ₀: angle between wire direction ŝ and horizontal (dx,dy).
								// cos(2φ₀) = cos²φ₀ − sin²φ₀;  cosφ₀ = (ŝ·ρ̂) projected.
								rhoInv := 1.0 / rho
								cosPhi := (segA.Direction[0]*dx + segA.Direction[1]*dy) * rhoInv
								cos2Phi := 2*cosPhi*cosPhi - 1
								i2 *= complex(cos2Phi, 0)
							}

							// Scattered Green's function values (j/4π factor via somPre).
							gVecScat := somPre * (i0 + i2)
							gScaScat := somPre * iPhi

							ww := complex(wp*wq*phiI*phiJ, 0)
							vecInt += ww * complex(dirDot, 0) * gVecScat
							scaInt += ww * gScaScat
						}
					}

					jacobian := complex(segA.HalfLength*segB.HalfLength, 0)
					vecInt *= jacobian
					scaInt *= jacobian

					vecTerm += vecInt
					scaTerm += complex(si.chargeDen*sj.chargeDen, 0) * scaInt
				}
			}

			val := vecPrefactor*vecTerm + scaPrefactor*scaTerm
			old := Z.At(i, j)
			Z.Set(i, j, old+val)
		}
	}
}

// sommerfeldSmallRho returns (i0, i2, iPhi) for ρ ≈ 0 using the normal-
// incidence (λ=0) approximation as a stable limit.  At ρ=0, J₀=1, J₂=0,
// so only the J₀-weighted integrals survive; a crude Gauss-Laguerre-style
// estimate over the decaying exp(−γ₀·z) suffices for the self-image term.
func sommerfeldSmallRho(zsum, k, sigma, epsilonR float64) (i0, i2, iPhi complex128) {
	omega := k * C0
	epsilonC := ComplexPermittivity(epsilonR, sigma, omega)

	// Evaluate integrand at a single representative λ (normal incidence).
	const nPts = 32
	s0, _, sPhi := integrateSubInterval(0, 15*k, 0, k, 0, zsum, epsilonC, nPts)
	return s0, 0, sPhi
}
