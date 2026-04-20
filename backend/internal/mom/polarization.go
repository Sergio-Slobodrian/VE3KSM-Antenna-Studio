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

// PolarizationType classifies the polarisation state of a radiated field.
type PolarizationType string

const (
	PolLinear   PolarizationType = "linear"
	PolCircular PolarizationType = "circular"
	PolElliptic PolarizationType = "elliptical"
)

// PolarizationSense is the rotation direction of the E-field tip.
type PolarizationSense string

const (
	SenseRHCP PolarizationSense = "RHCP"
	SenseLHCP PolarizationSense = "LHCP"
	SenseNone PolarizationSense = ""
)

// PolarizationPoint holds the polarisation parameters at a single far-field
// observation direction (θ, φ).
type PolarizationPoint struct {
	ThetaDeg  float64          `json:"theta"`
	PhiDeg    float64          `json:"phi"`
	AxialRatio float64         `json:"axial_ratio_db"` // axial ratio in dB (0 = CP, ∞ = LP)
	TiltDeg   float64          `json:"tilt_deg"`       // tilt angle of the polarisation ellipse (−90..+90)
	PolType   PolarizationType `json:"pol_type"`       // "linear", "circular", "elliptical"
	Sense     PolarizationSense `json:"sense"`          // "RHCP", "LHCP", "" (for linear)
}

// PolarizationMetrics holds aggregate polarisation data for the full pattern,
// plus principal-plane cuts of axial ratio.
type PolarizationMetrics struct {
	// Polarisation at the peak-gain direction
	PeakAxialRatioDB float64           `json:"peak_axial_ratio_db"`
	PeakTiltDeg      float64           `json:"peak_tilt_deg"`
	PeakPolType      PolarizationType  `json:"peak_pol_type"`
	PeakSense        PolarizationSense `json:"peak_sense"`

	// Principal-plane axial-ratio cuts (same grid as PolarCuts)
	AzimuthDeg      []float64 `json:"azimuth_deg"`
	AzimuthARdB     []float64 `json:"azimuth_ar_db"`
	AzimuthTiltDeg  []float64 `json:"azimuth_tilt_deg"`
	ElevationDeg    []float64 `json:"elevation_deg"`
	ElevationARdB   []float64 `json:"elevation_ar_db"`
	ElevationTiltDeg []float64 `json:"elevation_tilt_deg"`

	// Full pattern polarisation data (same grid as the far-field pattern)
	Points []PolarizationPoint `json:"points"`
}

// ComputePolarization computes polarisation parameters from complex Eθ and Eφ
// field components.  The inputs are parallel arrays of eTheta and ePhi for each
// far-field direction in the pattern grid, plus the pattern itself (for the
// theta/phi angles and peak-gain lookup).
//
// Polarisation ellipse parameters are derived from the Stokes parameters:
//
//	S0 = |Eθ|² + |Eφ|²
//	S1 = |Eθ|² − |Eφ|²
//	S2 = 2·Re(Eθ·conj(Eφ))
//	S3 = −2·Im(Eθ·conj(Eφ))  (IEEE convention: +S3 → RHCP)
//
// From these:
//
//	Tilt angle τ = ½·atan2(S2, S1)
//	Axial ratio AR = cot(½·asin(S3/S0))  (|AR| ≥ 1, sign = sense)
//	AR_dB = 20·log10(|AR|)               (0 dB = CP, large = LP)
func ComputePolarization(pattern []PatternPoint, eTheta, ePhi []complex128) PolarizationMetrics {
	n := len(pattern)
	if n == 0 || len(eTheta) != n || len(ePhi) != n {
		return PolarizationMetrics{}
	}

	points := make([]PolarizationPoint, n)

	for i := 0; i < n; i++ {
		pp := polarizationFromField(eTheta[i], ePhi[i])
		pp.ThetaDeg = pattern[i].ThetaDeg
		pp.PhiDeg = pattern[i].PhiDeg
		points[i] = pp
	}

	// Find peak-gain index
	peakIdx := 0
	for i, p := range pattern {
		if p.GainDB > pattern[peakIdx].GainDB {
			peakIdx = i
		}
	}

	peakTheta := pattern[peakIdx].ThetaDeg
	peakPhi := pattern[peakIdx].PhiDeg

	// Extract principal-plane cuts
	azDeg, azAR, azTilt := extractPolCut(points, peakTheta, true)
	elTheta, elAR, elTilt := extractPolCut(points, peakPhi, false)

	// Convert theta to elevation for the elevation cut
	elDeg := make([]float64, len(elTheta))
	for i, th := range elTheta {
		elDeg[i] = 90.0 - th
	}

	return PolarizationMetrics{
		PeakAxialRatioDB: points[peakIdx].AxialRatio,
		PeakTiltDeg:      points[peakIdx].TiltDeg,
		PeakPolType:      points[peakIdx].PolType,
		PeakSense:        points[peakIdx].Sense,

		AzimuthDeg:       azDeg,
		AzimuthARdB:      azAR,
		AzimuthTiltDeg:   azTilt,
		ElevationDeg:     elDeg,
		ElevationARdB:    elAR,
		ElevationTiltDeg: elTilt,

		Points: points,
	}
}

// polarizationFromField computes the polarisation ellipse for a single
// direction from its complex Eθ and Eφ components.
func polarizationFromField(et, ep complex128) PolarizationPoint {
	absEt := cmplx.Abs(et)
	absEp := cmplx.Abs(ep)

	// Stokes parameters (IEEE antenna convention)
	s0 := absEt*absEt + absEp*absEp
	s1 := absEt*absEt - absEp*absEp
	cross := et * cmplx.Conj(ep)
	s2 := 2.0 * real(cross)
	s3 := -2.0 * imag(cross) // +S3 → RHCP (IEEE convention)

	var pp PolarizationPoint

	if s0 < 1e-30 {
		// Negligible field — default to linear
		pp.AxialRatio = 100.0 // dB
		pp.PolType = PolLinear
		return pp
	}

	// Tilt angle of the polarisation ellipse major axis
	pp.TiltDeg = 0.5 * math.Atan2(s2, s1) * 180.0 / math.Pi

	// Ellipticity angle ε = ½·asin(S3/S0)
	sinRatio := s3 / s0
	if sinRatio > 1.0 {
		sinRatio = 1.0
	} else if sinRatio < -1.0 {
		sinRatio = -1.0
	}
	epsilon := 0.5 * math.Asin(sinRatio) // range ±π/4

	// Axial ratio = |cot(ε)|; sign of ε gives sense
	if math.Abs(epsilon) < 1e-10 {
		// Pure linear
		pp.AxialRatio = 100.0 // cap at 100 dB
		pp.PolType = PolLinear
		pp.Sense = SenseNone
	} else {
		tanEps := math.Tan(epsilon)
		if math.Abs(tanEps) < 1e-15 {
			pp.AxialRatio = 100.0
		} else {
			arLinear := math.Abs(1.0 / tanEps)
			pp.AxialRatio = 20.0 * math.Log10(arLinear)
			if pp.AxialRatio > 100.0 {
				pp.AxialRatio = 100.0
			}
		}

		// Classify
		if pp.AxialRatio < 3.0 {
			pp.PolType = PolCircular
		} else if pp.AxialRatio > 40.0 {
			pp.PolType = PolLinear
		} else {
			pp.PolType = PolElliptic
		}

		// Sense from sign of S3 (or equivalently sign of ε)
		if s3 > 0 {
			pp.Sense = SenseRHCP
		} else {
			pp.Sense = SenseLHCP
		}
		if pp.PolType == PolLinear {
			pp.Sense = SenseNone
		}
	}

	return pp
}

// extractPolCut extracts a principal-plane polarisation cut from the full
// set of polarisation points.  If azimuth==true, it selects all points at
// the given theta and returns (phi[], AR[], tilt[]).  Otherwise, it selects
// all points at the given phi and returns (theta[], AR[], tilt[]).
func extractPolCut(pts []PolarizationPoint, fixedAngle float64, azimuth bool) ([]float64, []float64, []float64) {
	var angles, ar, tilt []float64
	for _, p := range pts {
		var match, axis float64
		if azimuth {
			match = p.ThetaDeg
			axis = p.PhiDeg
		} else {
			match = p.PhiDeg
			axis = p.ThetaDeg
		}
		if math.Abs(match-fixedAngle) < 1.0 {
			angles = append(angles, axis)
			ar = append(ar, p.AxialRatio)
			tilt = append(tilt, p.TiltDeg)
		}
	}
	return angles, ar, tilt
}
