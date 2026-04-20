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

// NearFieldPoint holds the E and H field magnitudes at one observation point.
type NearFieldPoint struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	EMag   float64 `json:"e_mag"`
	HMag   float64 `json:"h_mag"`
	EMagDB float64 `json:"e_mag_db"`
	HMagDB float64 `json:"h_mag_db"`
}

// NearFieldRequest describes the observation grid: a 2D plane slice through
// 3D space.  The frontend picks a plane (xy, xz, or yz) at a fixed value
// of the third coordinate, plus bounds and resolution.
type NearFieldRequest struct {
	Plane      string  `json:"plane"`       // "xy", "xz", or "yz"
	FixedCoord float64 `json:"fixed_coord"` // constant value of the 3rd axis (m)
	Min1       float64 `json:"min1"`        // first in-plane axis min (m)
	Max1       float64 `json:"max1"`        // first in-plane axis max (m)
	Min2       float64 `json:"min2"`        // second in-plane axis min (m)
	Max2       float64 `json:"max2"`        // second in-plane axis max (m)
	Steps1     int     `json:"steps1"`      // grid points along axis 1
	Steps2     int     `json:"steps2"`      // grid points along axis 2
}

// NearFieldResult holds the full computed grid.
type NearFieldResult struct {
	Points     []NearFieldPoint `json:"points"`
	Plane      string           `json:"plane"`
	Axis1Label string           `json:"axis1_label"`
	Axis2Label string           `json:"axis2_label"`
	Axis1Vals  []float64        `json:"axis1_vals"`
	Axis2Vals  []float64        `json:"axis2_vals"`
	Steps1     int              `json:"steps1"`
	Steps2     int              `json:"steps2"`
	EMaxDB     float64          `json:"e_max_db"`
	EMinDB     float64          `json:"e_min_db"`
	HMaxDB     float64          `json:"h_max_db"`
	HMinDB     float64          `json:"h_min_db"`
}

// ComputeNearField evaluates E and H on a 2D observation grid using the
// Hertzian-dipole superposition.  Each MoM segment of length Δl carrying
// current I_n is treated as a short dipole with effective electric dipole
// moment p = I·Δl·ŝ/(jω).
//
// The exact near-field of a Hertzian dipole in SI (Jackson §9.1):
//
//	E = 1/(4πε₀) { k²(r̂×p)×r̂ · e^{-jkR}/R
//	    + [3r̂(r̂·p) − p]·(1/R³ − jk/R²)·e^{-jkR} }
//
//	H = ck²/(4π) · (r̂×p) · e^{-jkR}/R · (1 − 1/(jkR))
//
// where c = ω/k, r̂ points from source to observation, R = |r−r'|.
func ComputeNearField(segments []Segment, currents []complex128, k, freq float64, req NearFieldRequest) *NearFieldResult {
	omega := 2.0 * math.Pi * freq
	eps0 := 8.854187817e-12
	fourPiEps0 := 4.0 * math.Pi * eps0
	c0 := omega / k // speed of light
	k2 := k * k
	fourPi := 4.0 * math.Pi
	jk := complex(0, k)
	jOmega := complex(0, omega)

	// Clamp grid size
	clamp := func(v, lo, hi int) int {
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}
	req.Steps1 = clamp(req.Steps1, 2, 200)
	req.Steps2 = clamp(req.Steps2, 2, 200)

	// Resolve plane labels
	var axis1Label, axis2Label string
	switch req.Plane {
	case "xy":
		axis1Label, axis2Label = "x", "y"
	case "xz":
		axis1Label, axis2Label = "x", "z"
	case "yz":
		axis1Label, axis2Label = "y", "z"
	default:
		req.Plane = "xz"
		axis1Label, axis2Label = "x", "z"
	}

	axis1Vals := linspace(req.Min1, req.Max1, req.Steps1)
	axis2Vals := linspace(req.Min2, req.Max2, req.Steps2)

	// Pre-compute per-segment data
	type segData struct {
		cx, cy, cz float64    // center
		pEffX, pEffY, pEffZ complex128 // effective dipole moment = I·Δl·ŝ/(jω)
	}
	sd := make([]segData, 0, len(segments))
	for n, seg := range segments {
		if n >= len(currents) {
			break
		}
		moment := currents[n] * complex(2.0*seg.HalfLength, 0)
		sd = append(sd, segData{
			cx: seg.Center[0], cy: seg.Center[1], cz: seg.Center[2],
			pEffX: moment * complex(seg.Direction[0], 0) / jOmega,
			pEffY: moment * complex(seg.Direction[1], 0) / jOmega,
			pEffZ: moment * complex(seg.Direction[2], 0) / jOmega,
		})
	}

	total := req.Steps1 * req.Steps2
	points := make([]NearFieldPoint, total)
	eMaxDB, hMaxDB := -999.0, -999.0
	eMinDB, hMinDB := 999.0, 999.0

	for i2, v2 := range axis2Vals {
		for i1, v1 := range axis1Vals {
			idx := i2*req.Steps1 + i1

			var obsX, obsY, obsZ float64
			switch req.Plane {
			case "xy":
				obsX, obsY, obsZ = v1, v2, req.FixedCoord
			case "xz":
				obsX, obsY, obsZ = v1, req.FixedCoord, v2
			case "yz":
				obsX, obsY, obsZ = req.FixedCoord, v1, v2
			}

			var Ex, Ey, Ez complex128
			var Hx, Hy, Hz complex128

			for _, s := range sd {
				dx := obsX - s.cx
				dy := obsY - s.cy
				dz := obsZ - s.cz
				R := math.Sqrt(dx*dx + dy*dy + dz*dz)
				if R < 1e-10 {
					R = 1e-10
				}
				invR := 1.0 / R
				rx, ry, rz := dx*invR, dy*invR, dz*invR

				expMjkR := cmplx.Exp(-jk * complex(R, 0))

				// r̂ · p
				rDotP := complex(rx, 0)*s.pEffX + complex(ry, 0)*s.pEffY + complex(rz, 0)*s.pEffZ

				// r̂ × p
				crX := complex(ry, 0)*s.pEffZ - complex(rz, 0)*s.pEffY
				crY := complex(rz, 0)*s.pEffX - complex(rx, 0)*s.pEffZ
				crZ := complex(rx, 0)*s.pEffY - complex(ry, 0)*s.pEffX

				// (r̂ × p) × r̂
				ccX := crY*complex(rz, 0) - crZ*complex(ry, 0)
				ccY := crZ*complex(rx, 0) - crX*complex(rz, 0)
				ccZ := crX*complex(ry, 0) - crY*complex(rx, 0)

				invR2 := invR * invR
				invR3 := invR2 * invR

				// E-field: term1 = k²(r̂×p)×r̂ · e^{-jkR}/R / (4πε₀)
				t1 := complex(k2, 0) * expMjkR * complex(invR, 0) / complex(fourPiEps0, 0)

				// E-field: term2 = [3r̂(r̂·p) - p] · (1/R³ - jk/R²) · e^{-jkR} / (4πε₀)
				t2 := (complex(invR3, 0) - jk*complex(invR2, 0)) * expMjkR / complex(fourPiEps0, 0)

				threeRdotP := 3.0 * rDotP
				bx := complex(rx, 0)*threeRdotP - s.pEffX
				by := complex(ry, 0)*threeRdotP - s.pEffY
				bz := complex(rz, 0)*threeRdotP - s.pEffZ

				Ex += t1*ccX + t2*bx
				Ey += t1*ccY + t2*by
				Ez += t1*ccZ + t2*bz

				// H-field: ck²/(4π) · (r̂×p) · e^{-jkR}/R · (1 - 1/(jkR))
				hf := complex(c0*k2, 0) / complex(fourPi, 0) *
					expMjkR * complex(invR, 0) *
					(complex(1, 0) - complex(1, 0)/(jk*complex(R, 0)))

				Hx += hf * crX
				Hy += hf * crY
				Hz += hf * crZ
			}

			eMag := math.Sqrt(
				real(Ex)*real(Ex) + imag(Ex)*imag(Ex) +
					real(Ey)*real(Ey) + imag(Ey)*imag(Ey) +
					real(Ez)*real(Ez) + imag(Ez)*imag(Ez))
			hMag := math.Sqrt(
				real(Hx)*real(Hx) + imag(Hx)*imag(Hx) +
					real(Hy)*real(Hy) + imag(Hy)*imag(Hy) +
					real(Hz)*real(Hz) + imag(Hz)*imag(Hz))

			eDB := -100.0
			if eMag > 1e-30 {
				eDB = 20.0 * math.Log10(eMag)
			}
			hDB := -100.0
			if hMag > 1e-30 {
				hDB = 20.0 * math.Log10(hMag)
			}

			points[idx] = NearFieldPoint{
				X: obsX, Y: obsY, Z: obsZ,
				EMag: eMag, HMag: hMag,
				EMagDB: eDB, HMagDB: hDB,
			}

			if eDB > eMaxDB {
				eMaxDB = eDB
			}
			if eDB < eMinDB {
				eMinDB = eDB
			}
			if hDB > hMaxDB {
				hMaxDB = hDB
			}
			if hDB < hMinDB {
				hMinDB = hDB
			}
		}
	}

	return &NearFieldResult{
		Points: points, Plane: req.Plane,
		Axis1Label: axis1Label, Axis2Label: axis2Label,
		Axis1Vals: axis1Vals, Axis2Vals: axis2Vals,
		Steps1: req.Steps1, Steps2: req.Steps2,
		EMaxDB: eMaxDB, EMinDB: eMinDB,
		HMaxDB: hMaxDB, HMinDB: hMinDB,
	}
}

// ComputeNearFieldWithGround adds PEC ground-plane image contributions.
// Image segments (produced by ApplyPerfectGround) already carry flipped
// directions per PEC image theory; we simply concatenate them with the
// real segments and re-use the same currents.
func ComputeNearFieldWithGround(segments []Segment, currents []complex128, k, freq float64, req NearFieldRequest) *NearFieldResult {
	imageSegs := ApplyPerfectGround(segments)
	combined := make([]Segment, 0, 2*len(segments))
	combined = append(combined, segments...)
	combined = append(combined, imageSegs...)
	cur := make([]complex128, 0, 2*len(currents))
	cur = append(cur, currents...)
	cur = append(cur, currents...)
	return ComputeNearField(combined, cur, k, freq, req)
}

// linspace returns n evenly-spaced values from lo to hi inclusive.
func linspace(lo, hi float64, n int) []float64 {
	if n < 2 {
		return []float64{(lo + hi) / 2}
	}
	v := make([]float64, n)
	step := (hi - lo) / float64(n-1)
	for i := range v {
		v[i] = lo + float64(i)*step
	}
	return v
}
