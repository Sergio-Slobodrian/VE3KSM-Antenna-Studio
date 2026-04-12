package mom

import (
	"math"
	"math/cmplx"
)

// ComputeFarField computes the far-field radiation pattern and peak directivity (gain in dBi).
//
// For each direction (theta, phi), the electric field is computed by summing contributions
// from all current-carrying segments:
//
//	E(theta,phi) ~ sum_n I_n * deltaL_n * (dir_n projected onto theta/phi) * exp(jk * r_hat · center_n)
//
// Directivity is: D = 4*pi * |E_max|^2 / integral(|E|^2 sin(theta) dtheta dphi)
//
// Returns pattern points (theta 0..180 step 2 deg, phi 0..360 step 2 deg) and peak gain in dBi.
func ComputeFarField(segments []Segment, currents []complex128, k float64) ([]PatternPoint, float64) {
	const (
		thetaStep = 2.0 // degrees
		phiStep   = 2.0 // degrees
		deg2rad   = math.Pi / 180.0
	)

	nTheta := int(180.0/thetaStep) + 1
	nPhi := int(360.0/phiStep) + 1
	total := nTheta * nPhi

	pattern := make([]PatternPoint, 0, total)
	eSquared := make([]float64, 0, total)
	thetaRads := make([]float64, 0, total)
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

			// Observation direction unit vector
			rHat := [3]float64{
				sinTheta * cosPhi,
				sinTheta * sinPhi,
				cosTheta,
			}

			// Polarization unit vectors
			thetaHat := [3]float64{
				cosTheta * cosPhi,
				cosTheta * sinPhi,
				-sinTheta,
			}
			phiHat := [3]float64{
				-sinPhi,
				cosPhi,
				0,
			}

			var eTheta, ePhi complex128

			for n, seg := range segments {
				if n >= len(currents) {
					break
				}
				In := currents[n]
				deltaL := 2.0 * seg.HalfLength

				// Phase: exp(jk * r_hat · center_n)
				dotRC := rHat[0]*seg.Center[0] + rHat[1]*seg.Center[1] + rHat[2]*seg.Center[2]
				phase := cmplx.Exp(complex(0, k*dotRC))

				moment := In * complex(deltaL, 0) * phase

				// Project segment current direction onto far-field polarization vectors
				dTheta := seg.Direction[0]*thetaHat[0] + seg.Direction[1]*thetaHat[1] + seg.Direction[2]*thetaHat[2]
				dPhi := seg.Direction[0]*phiHat[0] + seg.Direction[1]*phiHat[1] + seg.Direction[2]*phiHat[2]

				eTheta += moment * complex(dTheta, 0)
				ePhi += moment * complex(dPhi, 0)
			}

			esq := real(eTheta)*real(eTheta) + imag(eTheta)*imag(eTheta) +
				real(ePhi)*real(ePhi) + imag(ePhi)*imag(ePhi)

			eSquared = append(eSquared, esq)
			thetaRads = append(thetaRads, thetaRad)

			if esq > maxEsq {
				maxEsq = esq
			}

			pattern = append(pattern, PatternPoint{
				ThetaDeg: theta,
				PhiDeg:   phi,
				GainDB:   0, // filled in below
			})
		}
	}

	// Integrate |E|^2 over sphere for total radiated power
	dTheta := thetaStep * deg2rad
	dPhi := phiStep * deg2rad
	totalPower := 0.0
	for idx, esq := range eSquared {
		sinTheta := math.Sin(thetaRads[idx])
		totalPower += esq * sinTheta * dTheta * dPhi
	}

	// Directivity and gain
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

	return pattern, gainDBi
}

// ComputeFarFieldWithGround computes the far-field for an antenna over a perfect ground plane.
// It sums contributions from both real and image segments, and restricts the pattern
// to the upper hemisphere (theta 0..90°). Below ground (theta > 90°) is set to -100 dB.
// The total radiated power is integrated over the upper hemisphere only, and then
// doubled (by symmetry) for the directivity calculation.
func ComputeFarFieldWithGround(realSegs, imageSegs []Segment, currents []complex128, k float64) ([]PatternPoint, float64) {
	const (
		thetaStep = 2.0
		phiStep   = 2.0
		deg2rad   = math.Pi / 180.0
	)

	nTheta := int(180.0/thetaStep) + 1
	nPhi := int(360.0/phiStep) + 1
	total := nTheta * nPhi

	pattern := make([]PatternPoint, 0, total)
	eSquared := make([]float64, 0, total)
	thetaRads := make([]float64, 0, total)
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

			// Only compute field in upper hemisphere (theta <= 90°)
			if theta <= 90.0 {
				rHat := [3]float64{sinTheta * cosPhi, sinTheta * sinPhi, cosTheta}
				thetaHat := [3]float64{cosTheta * cosPhi, cosTheta * sinPhi, -sinTheta}
				phiHat := [3]float64{-sinPhi, cosPhi, 0}

				var eTheta, ePhi complex128

				// Sum contributions from real segments
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

				// Sum contributions from image segments (same currents)
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
			pattern = append(pattern, PatternPoint{ThetaDeg: theta, PhiDeg: phi, GainDB: 0})
		}
	}

	// Integrate |E|² over upper hemisphere only, then double for ground symmetry
	dTheta := thetaStep * deg2rad
	dPhi := phiStep * deg2rad
	upperPower := 0.0
	for idx, esq := range eSquared {
		sinTheta := math.Sin(thetaRads[idx])
		upperPower += esq * sinTheta * dTheta * dPhi
	}
	// Total radiated power = 2 × upper hemisphere (perfect ground mirror symmetry)
	totalPower := 2.0 * upperPower

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

	return pattern, gainDBi
}
