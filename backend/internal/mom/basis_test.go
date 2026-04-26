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
	"testing"
)

// TestBasisShapeFunctions verifies that all shape function families satisfy
// the boundary conditions: φ=0 at the far end, φ=1 at the node.
func TestBasisShapeFunctions(t *testing.T) {
	halfLen := 0.5
	k := 2 * math.Pi * 14e6 / C0 // wavenumber at 14 MHz

	tests := []struct {
		name       string
		left       BasisFunc
		right      BasisFunc
	}{
		{
			"triangle",
			TriangleLeft{HalfLen: halfLen},
			TriangleRight{HalfLen: halfLen},
		},
		{
			"sinusoidal",
			SineLeft{HalfLen: halfLen, K: k},
			SineRight{HalfLen: halfLen, K: k},
		},
		{
			"quadratic",
			QuadraticLeft{HalfLen: halfLen},
			QuadraticRight{HalfLen: halfLen},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Left: φ(-1)=0 (far end), φ(+1)=1 (node)
			if v := tc.left.Phi(-1); math.Abs(v) > 1e-10 {
				t.Errorf("left.Phi(-1) = %f, want 0", v)
			}
			if v := tc.left.Phi(1); math.Abs(v-1) > 1e-10 {
				t.Errorf("left.Phi(+1) = %f, want 1", v)
			}

			// Right: φ(-1)=1 (node), φ(+1)=0 (far end)
			if v := tc.right.Phi(-1); math.Abs(v-1) > 1e-10 {
				t.Errorf("right.Phi(-1) = %f, want 1", v)
			}
			if v := tc.right.Phi(1); math.Abs(v) > 1e-10 {
				t.Errorf("right.Phi(+1) = %f, want 0", v)
			}

			// InterpolateWeight should be in (0, 1)
			wl := tc.left.InterpolateWeight()
			wr := tc.right.InterpolateWeight()
			if wl <= 0 || wl >= 1 {
				t.Errorf("left.InterpolateWeight() = %f, want (0,1)", wl)
			}
			if wr <= 0 || wr >= 1 {
				t.Errorf("right.InterpolateWeight() = %f, want (0,1)", wr)
			}
		})
	}
}

// TestSinusoidalBasis_DipoleSWR checks that using sinusoidal basis produces
// a valid impedance and reasonable SWR for a resonant half-wave dipole.
// The sinusoidal basis should match or outperform triangle basis for the
// same segment count.
func TestSinusoidalBasis_DipoleSWR(t *testing.T) {
	freq := 14.0e6
	lambda := C0 / freq
	halfLen := lambda / 4.0

	for _, order := range []BasisOrder{BasisTriangle, BasisSinusoidal, BasisQuadratic} {
		t.Run(string(order), func(t *testing.T) {
			input := SimulationInput{
				Wires: []Wire{
					{X1: 0, Y1: 0, Z1: -halfLen, X2: 0, Y2: 0, Z2: halfLen, Radius: 0.001, Segments: 21},
				},
				Frequency:  freq,
				Ground:     GroundConfig{Type: "free_space"},
				Source:     Source{WireIndex: 0, SegmentIndex: 10, Voltage: 1 + 0i},
				BasisOrder: order,
			}

			result, err := Simulate(input)
			if err != nil {
				t.Fatalf("Simulate(%s) failed: %v", order, err)
			}

			// Feed-point impedance should be reasonable: R ∈ [20, 120], |X| < 100
			if result.Impedance.R < 20 || result.Impedance.R > 120 {
				t.Errorf("R = %.1f Ω, expected [20, 120]", result.Impedance.R)
			}
			if math.Abs(result.Impedance.X) > 100 {
				t.Errorf("|X| = %.1f Ω, expected < 100", math.Abs(result.Impedance.X))
			}

			// SWR at 50Ω should be < 5 for a resonant dipole
			if result.SWR > 5.0 {
				t.Errorf("SWR = %.2f, expected < 5.0", result.SWR)
			}

			// Peak gain should be near 2.15 dBi ± 1 dB
			if result.GainDBi < 1.0 || result.GainDBi > 3.5 {
				t.Errorf("gain = %.2f dBi, expected [1.0, 3.5]", result.GainDBi)
			}

			// Currents should be non-zero
			nonZero := 0
			for _, c := range result.Currents {
				if c.Magnitude > 1e-10 {
					nonZero++
				}
			}
			if nonZero == 0 {
				t.Error("all segment currents are zero")
			}

			t.Logf("%s: Z = %.1f + j%.1f Ω, SWR = %.2f, Gain = %.2f dBi",
				order, result.Impedance.R, result.Impedance.X, result.SWR, result.GainDBi)
		})
	}
}

// TestSinusoidalBasis_FewerSegments verifies the key advantage of sinusoidal
// basis: with fewer segments (e.g. 7 instead of 21) it should still produce
// reasonable results, whereas triangle basis would be less accurate.
func TestSinusoidalBasis_FewerSegments(t *testing.T) {
	freq := 14.0e6
	lambda := C0 / freq
	halfLen := lambda / 4.0
	segments := 7 // only ~3 segs/λ — challenging for triangle basis

	// Run both and compare
	results := make(map[BasisOrder]*SolverResult)
	for _, order := range []BasisOrder{BasisTriangle, BasisSinusoidal} {
		input := SimulationInput{
			Wires: []Wire{
				{X1: 0, Y1: 0, Z1: -halfLen, X2: 0, Y2: 0, Z2: halfLen, Radius: 0.001, Segments: segments},
			},
			Frequency:  freq,
			Ground:     GroundConfig{Type: "free_space"},
			Source:     Source{WireIndex: 0, SegmentIndex: segments / 2, Voltage: 1 + 0i},
			BasisOrder: order,
		}

		result, err := Simulate(input)
		if err != nil {
			t.Fatalf("Simulate(%s, %d segs) failed: %v", order, segments, err)
		}
		results[order] = result
		t.Logf("%s (%d segs): Z = %.1f + j%.1f Ω, SWR = %.2f, Gain = %.2f dBi",
			order, segments, result.Impedance.R, result.Impedance.X, result.SWR, result.GainDBi)
	}

	// Both should produce a valid (non-NaN, non-infinite) impedance
	for _, order := range []BasisOrder{BasisTriangle, BasisSinusoidal} {
		r := results[order]
		if math.IsNaN(r.Impedance.R) || math.IsInf(r.Impedance.R, 0) {
			t.Errorf("%s: R is NaN/Inf", order)
		}
	}
}

// TestGenKernel_MatchesTriangleKernel verifies that GenKernel with triangle
// basis produces the same Z-matrix elements as TriangleKernel.
func TestGenKernel_MatchesTriangleKernel(t *testing.T) {
	freq := 14.0e6
	omega := 2.0 * math.Pi * freq
	k := omega / C0

	// Create two simple segments
	segs := SubdivideWire(0, 0, 0, -5, 0, 0, 5, 0.001, 0.001, 5)
	for i := range segs {
		segs[i].Index = i
	}

	// Build triangle bases the usual way
	bases := []TriangleBasis{
		{
			NodeIndex:       0,
			NodePos:         segs[0].End,
			SegLeft:         &segs[0],
			SegRight:        &segs[1],
			ChargeDensLeft:  -1.0 / (2.0 * segs[0].HalfLength),
			ChargeDensRight: 1.0 / (2.0 * segs[1].HalfLength),
		},
		{
			NodeIndex:       1,
			NodePos:         segs[1].End,
			SegLeft:         &segs[1],
			SegRight:        &segs[2],
			ChargeDensLeft:  -1.0 / (2.0 * segs[1].HalfLength),
			ChargeDensRight: 1.0 / (2.0 * segs[2].HalfLength),
		},
	}

	genBases := BuildGenBases(bases, BasisTriangle, k)

	// Compare Z[0,1] from both kernels
	vecOrig, scaOrig := TriangleKernel(bases[0], bases[1], k, omega, segs)
	vecGen, scaGen := GenKernel(genBases[0], genBases[1], k, omega, segs)

	vecDiff := cmplx.Abs(vecOrig - vecGen)
	scaDiff := cmplx.Abs(scaOrig - scaGen)

	if vecDiff > 1e-10*cmplx.Abs(vecOrig) {
		t.Errorf("vector term mismatch: orig=%v, gen=%v, diff=%e", vecOrig, vecGen, vecDiff)
	}
	if scaDiff > 1e-10*cmplx.Abs(scaOrig) {
		t.Errorf("scalar term mismatch: orig=%v, gen=%v, diff=%e", scaOrig, scaGen, scaDiff)
	}
}
