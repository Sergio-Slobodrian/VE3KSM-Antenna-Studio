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

import (
	"math"
	"math/cmplx"
)

// ComputeFarField computes the far-field radiation pattern and peak directivity
// for an antenna in free space (no ground plane).
//
// The far-field electric field in direction (theta, phi) is computed using the
// standard array-factor summation over all current-carrying segments:
//
//	E(θ,φ) ∝ Σ_n I_n · Δl_n · [ŝ_n projected onto θ̂/φ̂] · exp(jk · r̂ · r_n)
//
// where:
//   - I_n is the complex current on segment n (A)
//   - Δl_n is the segment length (m)
//   - ŝ_n is the segment direction unit vector
//   - r_n is the segment center position (m)
//   - r̂ is the observation direction unit vector
//   - k is the free-space wavenumber (rad/m)
//
// The phase term exp(jk·r̂·r_n) accounts for the path length difference between
// each segment and the far-field reference point (origin). The projection onto
// θ̂ and φ̂ decomposes the radiated field into its two polarization components.
//
// Directivity is computed as:
//
//	D(θ,φ) = 4π · |E(θ,φ)|² / ∫∫ |E|² sin(θ) dθ dφ
//
// The sphere is sampled on a 2-degree grid in both theta and phi. The integral
// is evaluated via rectangular quadrature over the grid.
//
// Returns the full-sphere pattern and the peak directivity in dBi.
func ComputeFarField(segments []Segment, currents []complex128, k float64) ([]PatternPoint, float64, []complex128, []complex128) {
	const (
		thetaStep = 2.0              // angular resolution in theta (degrees)
		phiStep   = 2.0              // angular resolution in phi (degrees)
		deg2rad   = math.Pi / 180.0
	)

	nTheta := int(180.0/thetaStep) + 1 // 0, 2, 4, ..., 180
	nPhi := int(360.0/phiStep) + 1     // 0, 2, 4, ..., 360
	total := nTheta * nPhi

	pattern := make([]PatternPoint, 0, total)
	eSquared := make([]float64, 0, total)   // |E|² at each sample point
	thetaRads := make([]float64, 0, total)  // theta in radians (for integration weights)
	allETheta := make([]complex128, 0, total) // complex Eθ for polarisation analysis
	allEPhi := make([]complex128, 0, total)   // complex Eφ for polarisation analysis
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

			// Spherical coordinate unit vectors in Cartesian components
			rHat := [3]float64{          // radial direction (observation)
				sinTheta * cosPhi,
				sinTheta * sinPhi,
				cosTheta,
			}
			thetaHat := [3]float64{      // theta polarization direction
				cosTheta * cosPhi,
				cosTheta * sinPhi,
				-sinTheta,
			}
			phiHat := [3]float64{        // phi polarization direction
				-sinPhi,
				cosPhi,
				0,
			}

			// Sum contributions from all segments to get E_theta and E_phi
			var eTheta, ePhi complex128

			for n, seg := range segments {
				if n >= len(currents) {
					break
				}
				In := currents[n]
				deltaL := 2.0 * seg.HalfLength // full segment length (m)

				// Far-field phase factor: accounts for the path length difference
				// between this segment and the phase reference at the origin
				dotRC := rHat[0]*seg.Center[0] + rHat[1]*seg.Center[1] + rHat[2]*seg.Center[2]
				phase := cmplx.Exp(complex(0, k*dotRC))

				// Current moment: I_n * Δl_n * exp(jk·r̂·r_n)
				moment := In * complex(deltaL, 0) * phase

				// Project the segment's current direction onto the far-field
				// polarization basis vectors to get theta and phi field components
				dTheta := seg.Direction[0]*thetaHat[0] + seg.Direction[1]*thetaHat[1] + seg.Direction[2]*thetaHat[2]
				dPhi := seg.Direction[0]*phiHat[0] + seg.Direction[1]*phiHat[1] + seg.Direction[2]*phiHat[2]

				eTheta += moment * complex(dTheta, 0)
				ePhi += moment * complex(dPhi, 0)
			}

			// Total |E|² = |E_theta|² + |E_phi|² (both polarizations)
			esq := real(eTheta)*real(eTheta) + imag(eTheta)*imag(eTheta) +
				real(ePhi)*real(ePhi) + imag(ePhi)*imag(ePhi)

			eSquared = append(eSquared, esq)
			thetaRads = append(thetaRads, thetaRad)
			allETheta = append(allETheta, eTheta)
			allEPhi = append(allEPhi, ePhi)

			if esq > maxEsq {
				maxEsq = esq
			}

			pattern = append(pattern, PatternPoint{
				ThetaDeg: theta,
				PhiDeg:   phi,
				GainDB:   0, // placeholder, filled in after integration
			})
		}
	}

	// Numerically integrate |E|² over the full sphere to get total radiated power.
	// Uses rectangular quadrature: P_rad ∝ Σ |E|² sin(θ) Δθ Δφ
	dTheta := thetaStep * deg2rad
	dPhi := phiStep * deg2rad
	totalPower := 0.0
	for idx, esq := range eSquared {
		sinTheta := math.Sin(thetaRads[idx])
		totalPower += esq * sinTheta * dTheta * dPhi
	}

	// Convert |E|² to directivity D = 4π·|E|²/P_rad, then to dBi = 10·log10(D)
	var gainDBi float64
	if totalPower > 1e-30 {
		directivityMax := 4.0 * math.Pi * maxEsq / totalPower
		if directivityMax > 1e-30 {
			gainDBi = 10.0 * math.Log10(directivityMax)
		}

		// Fill in per-direction gain values
		for idx := range pattern {
			d := 4.0 * math.Pi * eSquared[idx] / totalPower
			if d > 1e-30 {
				pattern[idx].GainDB = 10.0 * math.Log10(d)
			} else {
				pattern[idx].GainDB = -100.0 // floor for negligible radiation
			}
		}
	}

	return pattern, gainDBi, allETheta, allEPhi
}

// ComputeFarFieldWithGround computes the far-field radiation pattern for an
// antenna over a perfect electric conductor (PEC) ground plane at z=0.
//
// The computation differs from free-space (ComputeFarField) in two ways:
//  1. Both real and image segment contributions are summed for each observation
//     direction. The image segments carry the same currents but with directions
//     modified per PEC image theory (see ApplyPerfectGround).
//  2. The pattern is restricted to the upper hemisphere (theta 0..90 degrees).
//     Below the ground plane (theta > 90), gain is set to -100 dB.
//
// For directivity calculation, the total radiated power is computed by
// integrating |E|^2 over the upper hemisphere and then doubling it. This
// doubling accounts for the mirror symmetry: the ground plane reflects all
// downward radiation upward, so the total power equals twice the upper
// hemisphere integral. The directivity formula D = 4pi*|E_max|^2 / P_total
// then correctly yields the ground-plane gain enhancement.
func ComputeFarFieldWithGround(realSegs, imageSegs []Segment, currents []complex128, k float64) ([]PatternPoint, float64, []complex128, []complex128) {
	const (
		thetaStep = 2.0              // angular resolution (degrees)
		phiStep   = 2.0              // angular resolution (degrees)
		deg2rad   = math.Pi / 180.0
	)

	nTheta := int(180.0/thetaStep) + 1
	nPhi := int(360.0/phiStep) + 1
	total := nTheta * nPhi

	pattern := make([]PatternPoint, 0, total)
	eSquared := make([]float64, 0, total)  // |E|^2 at each sample point
	thetaRads := make([]float64, 0, total) // theta in radians for integration
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

			// Only compute field in upper hemisphere; below ground is zero
			if theta <= 90.0 {
				// Spherical coordinate unit vectors
				rHat := [3]float64{sinTheta * cosPhi, sinTheta * sinPhi, cosTheta}
				thetaHat := [3]float64{cosTheta * cosPhi, cosTheta * sinPhi, -sinTheta}
				phiHat := [3]float64{-sinPhi, cosPhi, 0}

				// Sum contributions from real (physical) segments
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

				// Sum contributions from image segments (below ground, same currents,
				// direction already modified by ApplyPerfectGround)
				for n, seg := range imageSegs {
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

	// Integrate |E|^2 over the upper hemisphere for radiated power.
	// Since only theta <= 90 has nonzero |E|^2, the sum naturally covers
	// only the upper hemisphere even though we loop over all entries.
	dTheta := thetaStep * deg2rad
	dPhi := phiStep * deg2rad
	upperPower := 0.0
	for idx, esq := range eSquared {
		sinTheta := math.Sin(thetaRads[idx])
		upperPower += esq * sinTheta * dTheta * dPhi
	}
	// For an antenna over a ground plane, ALL radiation exists in the upper
	// hemisphere (the image creates reflected waves that appear above ground,
	// not below). The total radiated power IS the upper hemisphere integral.
	// No factor-of-2 doubling — the image contributions are already included
	// in the field summation above.
	totalPower := upperPower

	// Compute directivity and per-direction gain (same formula as free-space)
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
				pattern[idx].GainDB = -100.0 // floor for below-ground or negligible radiation
			}
		}
	}

	return pattern, gainDBi, allETheta, allEPhi
}
