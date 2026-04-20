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

// ground_real.go implements the lossy (real) ground plane model using
// Fresnel reflection coefficients applied to the image-theory framework.
//
// Instead of full Sommerfeld integration (which is prohibitively expensive),
// this uses the reflection-coefficient image method: the geometric image
// segments are the same as for perfect ground (created by ApplyPerfectGround),
// but each image contribution is scaled by an angle-dependent complex Fresnel
// reflection coefficient. This is the standard approach used by MININEC, EZNEC,
// and similar antenna modeling tools.
//
// The reflection coefficient depends on:
//   - The elevation/grazing angle (ψ) of the ray from image to observation
//   - The complex permittivity of the ground: εc = εr - jσ/(ωε₀)
//   - The polarization: vertical (TM) or horizontal (TE)
//
// For the Z-matrix, a single effective reflection coefficient per basis pair
// is computed from the average geometry. For the far-field, the exact per-angle
// Fresnel coefficients are applied to each observation direction.
package mom

import (
	"math"
	"math/cmplx"

	"gonum.org/v1/gonum/mat"
)

// ComplexPermittivity computes the complex relative permittivity of the ground:
//
//	εc = εr - j·σ/(ω·ε₀)
//
// where εr is relative permittivity, σ is conductivity (S/m), ω is angular
// frequency (rad/s), and ε₀ is the permittivity of free space.
// The imaginary part represents conduction losses in the ground material.
func ComplexPermittivity(epsilonR, sigma, omega float64) complex128 {
	return complex(epsilonR, -sigma/(omega*Eps0))
}

// FresnelRV computes the Fresnel reflection coefficient for vertical
// polarization (TM / parallel / p-polarized) at a planar air-ground interface:
//
//	Rv = (εc·sin(ψ) - √(εc - cos²(ψ))) / (εc·sin(ψ) + √(εc - cos²(ψ)))
//
// ψ is the grazing angle in radians (0 = horizon, π/2 = zenith).
// For perfect ground (|εc| → ∞), Rv → +1.
func FresnelRV(psi float64, epsilonC complex128) complex128 {
	sinPsi := complex(math.Sin(psi), 0)
	cosPsi := math.Cos(psi)
	cosPsi2 := complex(cosPsi*cosPsi, 0)

	sqrtTerm := cmplx.Sqrt(epsilonC - cosPsi2)

	num := epsilonC*sinPsi - sqrtTerm
	den := epsilonC*sinPsi + sqrtTerm

	if cmplx.Abs(den) < 1e-30 {
		return -1 + 0i
	}
	return num / den
}

// FresnelRH computes the Fresnel reflection coefficient for horizontal
// polarization (TE / perpendicular / s-polarized):
//
//	Rh = (sin(ψ) - √(εc - cos²(ψ))) / (sin(ψ) + √(εc - cos²(ψ)))
//
// ψ is the grazing angle in radians. For perfect ground, Rh → -1.
func FresnelRH(psi float64, epsilonC complex128) complex128 {
	sinPsi := complex(math.Sin(psi), 0)
	cosPsi := math.Cos(psi)
	cosPsi2 := complex(cosPsi*cosPsi, 0)

	sqrtTerm := cmplx.Sqrt(epsilonC - cosPsi2)

	num := sinPsi - sqrtTerm
	den := sinPsi + sqrtTerm

	if cmplx.Abs(den) < 1e-30 {
		return -1 + 0i
	}
	return num / den
}

// addRealGroundTriangleBasis adds lossy ground contributions to the Z-matrix
// using the half-space Green's function with Fresnel reflection coefficients.
//
// The PEC image kernel (TriangleKernelPerfectGround) computes the image
// contribution with correct parameterisation on real basis functions.
// Each Z-matrix entry is then scaled by an effective Fresnel coefficient
// that blends Rv (vertical polarisation) and Rh (horizontal polarisation)
// based on the current direction and the quasi-static grazing angle.
func addRealGroundTriangleBasis(Z *mat.CDense, bases []TriangleBasis, realSegs, imageSegs []Segment, k, omega, sigma, epsilonR float64) {
	epsilonC := ComplexPermittivity(epsilonR, sigma, omega)

	vecPrefactor := complex(0, omega*Mu0/(4.0*math.Pi))
	k2 := k * k
	scaPrefactor := -complex(0, omega*Mu0/(4.0*math.Pi*k2))

	n := len(bases)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			vecTerm, scaTerm := TriangleKernelPerfectGround(bases[i], bases[j], k)

			// Quasi-static grazing angle from basis node geometry.
			obsZ := bases[i].NodePos[2]
			srcZ := bases[j].NodePos[2]
			vertDist := math.Abs(obsZ + srcZ)

			dx := bases[i].NodePos[0] - bases[j].NodePos[0]
			dy := bases[i].NodePos[1] - bases[j].NodePos[1]
			horizDist := math.Sqrt(dx*dx + dy*dy)

			psi := math.Atan2(vertDist, horizDist)
			if psi < 0.01 {
				psi = 0.01
			}

			rv := FresnelRV(psi, epsilonC)
			rh := FresnelRH(psi, epsilonC)

			vertFracI := basisVerticalFraction(bases[i])
			vertFracJ := basisVerticalFraction(bases[j])
			vertFrac := (vertFracI + vertFracJ) / 2.0
			rEff := complex(vertFrac, 0)*rv + complex(1-vertFrac, 0)*rh

			val := rEff * (vecPrefactor*vecTerm + scaPrefactor*scaTerm)
			old := Z.At(i, j)
			Z.Set(i, j, old+val)
		}
	}
}

// basisVerticalFraction returns 0..1 indicating how "vertical" the current
// is on this basis function. 1.0 = purely z-directed, 0.0 = purely horizontal.
func basisVerticalFraction(b TriangleBasis) float64 {
	var zSum, count float64
	if b.SegLeft != nil {
		zSum += math.Abs(b.SegLeft.Direction[2])
		count++
	}
	if b.SegRight != nil {
		zSum += math.Abs(b.SegRight.Direction[2])
		count++
	}
	if count == 0 {
		return 0.5
	}
	return zSum / count
}

// ComputeFarFieldRealGround computes the far-field radiation pattern for an
// antenna over a lossy ground plane, using Fresnel reflection coefficients.
//
// For each observation direction (theta, phi):
//  1. Real segments contribute directly (same as free-space).
//  2. Image segments contribute with their fields scaled by the Fresnel
//     reflection coefficient at the elevation angle. Vertical current components
//     use Rv, horizontal components use Rh.
//  3. The pattern is restricted to the upper hemisphere (theta <= 90°).
//
// Total power is integrated over the upper hemisphere WITHOUT doubling
// (unlike perfect ground), because the lossy ground absorbs some power.
func ComputeFarFieldRealGround(realSegs, imageSegs []Segment, currents []complex128, k, omega, sigma, epsilonR float64) ([]PatternPoint, float64, []complex128, []complex128) {
	const (
		thetaStep = 2.0
		phiStep   = 2.0
		deg2rad   = math.Pi / 180.0
	)

	epsilonC := ComplexPermittivity(epsilonR, sigma, omega)

	nTheta := int(180.0/thetaStep) + 1
	nPhi := int(360.0/phiStep) + 1
	total := nTheta * nPhi

	pattern := make([]PatternPoint, 0, total)
	eSquared := make([]float64, 0, total)
	thetaRads := make([]float64, 0, total)
	allETheta := make([]complex128, 0, total)
	allEPhi := make([]complex128, 0, total)
	maxEsq := 0.0

	for it := 0; it < nTheta; it++ {
		theta := float64(it) * thetaStep
		thetaRad := theta * deg2rad
		sinTheta := math.Sin(thetaRad)
		cosTheta := math.Cos(thetaRad)

		for ip := 0; ip < nPhi; ip++ {
			phi := float64(ip) * phiStep
			phiRad := phi * deg2rad
			sinPhi := math.Sin(phiRad)
			cosPhi := math.Cos(phiRad)

			var esq float64
			var eTheta, ePhi complex128

			if theta <= 90.0 {
				rHat := [3]float64{sinTheta * cosPhi, sinTheta * sinPhi, cosTheta}
				thetaHat := [3]float64{cosTheta * cosPhi, cosTheta * sinPhi, -sinTheta}
				phiHat := [3]float64{-sinPhi, cosPhi, 0}

				// Direct contributions from real segments
				for n, seg := range realSegs {
					if n >= len(currents) {
						break
					}
					In := currents[n]
					deltaL := 2.0 * seg.HalfLength
					dotRC := rHat[0]*seg.Center[0] + rHat[1]*seg.Center[1] + rHat[2]*seg.Center[2]
					phase := cmplx.Exp(complex(0, k*dotRC))
					moment := In * complex(deltaL, 0) * phase

					dTheta := seg.Direction[0]*thetaHat[0] + seg.Direction[1]*thetaHat[1] + seg.Direction[2]*thetaHat[2]
					dPhi := seg.Direction[0]*phiHat[0] + seg.Direction[1]*phiHat[1] + seg.Direction[2]*phiHat[2]

					eTheta += moment * complex(dTheta, 0)
					ePhi += moment * complex(dPhi, 0)
				}

				// Ground-reflected contributions from image segments,
				// scaled by Fresnel reflection coefficients at the elevation angle.
				// Elevation angle ψ = π/2 - θ (grazing angle from horizon).
				psi := (math.Pi/2.0 - thetaRad)
				if psi < 0.001 {
					psi = 0.001
				}
				rv := FresnelRV(psi, epsilonC)
				rh := FresnelRH(psi, epsilonC)

				for n, seg := range imageSegs {
					if n >= len(currents) {
						break
					}
					In := currents[n]
					deltaL := 2.0 * seg.HalfLength
					dotRC := rHat[0]*seg.Center[0] + rHat[1]*seg.Center[1] + rHat[2]*seg.Center[2]
					phase := cmplx.Exp(complex(0, k*dotRC))
					moment := In * complex(deltaL, 0) * phase

					// Decompose the image segment direction into vertical and horizontal.
					// The vertical (z) component uses Rv, horizontal (in theta-phi plane) uses Rh.
					dirZ := seg.Direction[2] // vertical component
					dirH := math.Sqrt(seg.Direction[0]*seg.Direction[0] + seg.Direction[1]*seg.Direction[1])

					// Project onto far-field polarization vectors
					dTheta := seg.Direction[0]*thetaHat[0] + seg.Direction[1]*thetaHat[1] + seg.Direction[2]*thetaHat[2]
					dPhi := seg.Direction[0]*phiHat[0] + seg.Direction[1]*phiHat[1] + seg.Direction[2]*phiHat[2]

					// Blend Rv/Rh based on current orientation
					var rEff complex128
					if math.Abs(dirZ)+dirH < 1e-20 {
						rEff = rv
					} else {
						vertFrac := math.Abs(dirZ) / (math.Abs(dirZ) + dirH)
						rEff = complex(vertFrac, 0)*rv + complex(1-vertFrac, 0)*rh
					}

					eTheta += rEff * moment * complex(dTheta, 0)
					ePhi += rEff * moment * complex(dPhi, 0)
				}

				esq = real(eTheta)*real(eTheta) + imag(eTheta)*imag(eTheta) +
					real(ePhi)*real(ePhi) + imag(ePhi)*imag(ePhi)

				if esq > maxEsq {
					maxEsq = esq
				}
			}

			eSquared = append(eSquared, esq)
			thetaRads = append(thetaRads, thetaRad)
			allETheta = append(allETheta, eTheta)
			allEPhi = append(allEPhi, ePhi)
			pattern = append(pattern, PatternPoint{ThetaDeg: theta, PhiDeg: phi, GainDB: 0})
		}
	}

	// Integrate over upper hemisphere only — no doubling for lossy ground
	// (ground absorbs power, so the lower hemisphere is not a perfect mirror)
	dTheta := thetaStep * deg2rad
	dPhi := phiStep * deg2rad
	totalPower := 0.0
	for idx, esq := range eSquared {
		sinTheta := math.Sin(thetaRads[idx])
		totalPower += esq * sinTheta * dTheta * dPhi
	}

	var gainDBi float64
	if totalPower > 1e-30 {
		directivityMax := 4.0 * math.Pi * maxEsq / totalPower
		if directivityMax > 1e-30 {
			gainDBi = 10.0 * math.Log10(directivityMax)
		}

		for idx := range pattern {
			d := 4.0 * math.Pi * eSquared[idx] / totalPower
			if d > 1e-30 {
				pattern[idx].GainDB = 10.0 * math.Log10(d)
			} else {
				pattern[idx].GainDB = -100.0
			}
		}
	}

	return pattern, gainDBi, allETheta, allEPhi
}
