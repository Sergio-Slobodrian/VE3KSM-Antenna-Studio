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

// dielectricLayer describes one concentric shell of dielectric material.
type dielectricLayer struct {
	EpsR        float64 // relative permittivity (≥ 1)
	LossTan     float64 // loss tangent tanδ (≥ 0)
	OuterRadius float64 // outer surface radius (m)
}

// multilayerZPerUnitLen computes the per-unit-length distributed series
// impedance for a stack of concentric dielectric shells on a conductor of
// radius a, using the generalised NEC-4 IS-card formula:
//
//	Z'_total = (jωμ₀/2π) · Σ_i (1/ε_{i−1}* − 1/ε_i*) · ln(b_i / b_{i−1})
//
// Layers are ordered inner-to-outer; ε_0* = 1 (the formula's starting
// condition).  Layers with OuterRadius ≤ previous radius are skipped.
func multilayerZPerUnitLen(a float64, layers []dielectricLayer, omega float64) complex128 {
	if len(layers) == 0 {
		return 0
	}
	jOmegaMu0Over2pi := complex(0, omega*Mu0/(2*math.Pi))
	var sum complex128
	prevEps := complex128(1) // ε_0* = 1
	prevR := a
	for _, l := range layers {
		if l.OuterRadius <= prevR || l.EpsR < 1 {
			continue
		}
		lnRatio := math.Log(l.OuterRadius / prevR)
		epsI := complex(l.EpsR, -l.EpsR*l.LossTan)
		sum += (1/prevEps - 1/epsI) * complex(lnRatio, 0)
		prevEps = epsI
		prevR = l.OuterRadius
	}
	return jOmegaMu0Over2pi * sum
}

// weatherLayer returns the dielectric parameters for a weather preset, or
// (0,0) for "dry" / unknown presets.
func weatherLayer(preset string) (epsR, lossTan float64) {
	switch preset {
	case "rain":
		return 80.0, 0.05
	case "ice":
		return 3.17, 0.001
	case "wet_snow":
		return 1.6, 0.005
	}
	return 0, 0
}

// applyCoatingLoading adds the distributed series impedance from per-wire
// dielectric coatings and/or a global weather film to the Z-matrix diagonal,
// using the generalised multi-layer IS-card model.
//
// Layer stack (inner to outer) per wire:
//  1. Wire coating (CoatingThickness > 0, CoatingEpsR > 1)
//  2. Weather film  (weather.Thickness > 0, preset != "dry")
//
// For bare wires in dry conditions both layers are absent and the function
// skips that wire entirely.  For a bare wire in rain the single water layer
// reduces to the original single-layer formula.  For a PVC-coated wire in
// ice the two-layer sum is used.
//
// The basis split and lossPerBasis accounting match applyMaterialLoss.
func applyCoatingLoading(zmat zMatSetter, wires []Wire, segments []Segment,
	wireSegOffsets, wireSegCounts, wireBasisOffsets []int,
	omega float64, weather WeatherConfig, lossPerBasis []float64) {

	// Use explicitly provided εr/tanδ if supplied; otherwise fall back to preset defaults.
	weatherEpsR, weatherLossTan := weatherLayer(weather.Preset)
	if weather.EpsR >= 1 {
		weatherEpsR = weather.EpsR
		weatherLossTan = weather.LossTan
	}
	hasWeather := weather.Thickness > 0 && weatherEpsR >= 1

	for wi, w := range wires {
		if w.Radius <= 0 && !w.isTapered() {
			continue
		}

		// Whether any layer at all is present is a per-wire property
		// (coating thickness / εr and weather film don't vary per segment).
		hasCoating := w.CoatingThickness > 0 && w.CoatingEpsR > 1
		if !hasCoating && !hasWeather {
			continue
		}

		// zPerUnitLen depends on the inner conductor radius a, so for tapered
		// wires we evaluate it inside the basis loop at each segment's own
		// radius. buildLayers returns the (wire-wide) layer stack for a given
		// inner radius a.
		buildLayers := func(a float64) []dielectricLayer {
			var layers []dielectricLayer
			curR := a
			if hasCoating {
				curR = a + w.CoatingThickness
				layers = append(layers, dielectricLayer{
					EpsR:        w.CoatingEpsR,
					LossTan:     w.CoatingLossTan,
					OuterRadius: curR,
				})
			}
			if hasWeather {
				layers = append(layers, dielectricLayer{
					EpsR:        weatherEpsR,
					LossTan:     weatherLossTan,
					OuterRadius: curR + weather.Thickness,
				})
			}
			return layers
		}

		segOff := wireSegOffsets[wi]
		nSeg := wireSegCounts[wi]
		basisOff := wireBasisOffsets[wi]

		for k := 0; k < nSeg-1; k++ {
			seg1 := segments[segOff+k]
			seg2 := segments[segOff+k+1]
			len1 := 2 * seg1.HalfLength
			len2 := 2 * seg2.HalfLength

			// Evaluate the per-unit-length impedance at each segment's own
			// conductor radius. When the wire is uniform both calls use the
			// same radius and the result collapses to the pre-taper formula.
			zPul1 := multilayerZPerUnitLen(seg1.Radius, buildLayers(seg1.Radius), omega)
			zPul2 := multilayerZPerUnitLen(seg2.Radius, buildLayers(seg2.Radius), omega)
			zBasis := 0.5*zPul1*complex(len1, 0) + 0.5*zPul2*complex(len2, 0)
			if cmplx.Abs(zBasis) == 0 {
				continue
			}
			bi := basisOff + k
			zmat.Add(bi, bi, zBasis)
			if lossPerBasis != nil && bi < len(lossPerBasis) {
				lossPerBasis[bi] += real(zBasis)
			}
		}
	}
}
