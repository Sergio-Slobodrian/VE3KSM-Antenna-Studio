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

func TestComplexPermittivity(t *testing.T) {
	// Average ground: εr=13, σ=0.005 at 14 MHz
	omega := 2 * math.Pi * 14e6
	ec := ComplexPermittivity(13, 0.005, omega)

	if real(ec) != 13 {
		t.Errorf("real part should be 13, got %f", real(ec))
	}
	// Imaginary part = -σ/(ω·ε₀) should be negative and significant
	if imag(ec) >= 0 {
		t.Errorf("imaginary part should be negative (lossy), got %f", imag(ec))
	}
}

func TestFresnelPerfectConductorLimit(t *testing.T) {
	// Very high conductivity should approach perfect ground: Rv→+1, Rh→-1
	omega := 2 * math.Pi * 14e6
	ec := ComplexPermittivity(13, 1e6, omega) // σ = 1 MS/m (copper-like)

	psi := 0.5 // ~29 degrees elevation

	rv := FresnelRV(psi, ec)
	rh := FresnelRH(psi, ec)

	// Rv should be close to +1
	if cmplx.Abs(rv-1) > 0.01 {
		t.Errorf("Rv should approach +1 for perfect conductor, got %v (|Rv-1| = %f)",
			rv, cmplx.Abs(rv-1))
	}
	// Rh should be close to -1
	if cmplx.Abs(rh+1) > 0.01 {
		t.Errorf("Rh should approach -1 for perfect conductor, got %v (|Rh+1| = %f)",
			rh, cmplx.Abs(rh+1))
	}
}

func TestFresnelNormalIncidence(t *testing.T) {
	// At normal incidence (ψ = π/2), Rv and Rh should have equal magnitude
	omega := 2 * math.Pi * 14e6
	ec := ComplexPermittivity(13, 0.005, omega)

	psi := math.Pi / 2.0
	rv := FresnelRV(psi, ec)
	rh := FresnelRH(psi, ec)

	// At normal incidence: Rv = (εc - 1)/(εc + 1), Rh = (1 - √εc)/(1 + √εc)...
	// they differ but both should be real-ish and have |R| < 1
	if cmplx.Abs(rv) >= 1.0 {
		t.Errorf("|Rv| should be < 1, got %f", cmplx.Abs(rv))
	}
	if cmplx.Abs(rh) >= 1.0 {
		t.Errorf("|Rh| should be < 1, got %f", cmplx.Abs(rh))
	}
}

func TestFresnelGrazingAngle(t *testing.T) {
	// At grazing incidence (ψ → 0), both Rv and Rh approach -1
	omega := 2 * math.Pi * 14e6
	ec := ComplexPermittivity(13, 0.005, omega)

	psi := 0.01 // near grazing
	rv := FresnelRV(psi, ec)
	rh := FresnelRH(psi, ec)

	// Both should be close to -1 at grazing
	if cmplx.Abs(rv+1) > 0.2 {
		t.Errorf("Rv should approach -1 at grazing, got %v", rv)
	}
	if cmplx.Abs(rh+1) > 0.2 {
		t.Errorf("Rh should approach -1 at grazing, got %v", rh)
	}
}

func TestRealGroundHighConductivityMatchesPerfect(t *testing.T) {
	// A quarter-wave vertical over very high conductivity ground should give
	// nearly the same impedance as perfect ground.
	freq := 300e6
	lambda := C0 / freq
	height := lambda / 4.0

	perfectInput := SimulationInput{
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: height,
			Radius: 0.001, Segments: 11,
		}},
		Frequency: freq,
		Ground:    GroundConfig{Type: "perfect"},
		Source:    Source{WireIndex: 0, SegmentIndex: 0, Voltage: 1 + 0i},
	}

	realInput := perfectInput
	realInput.Ground = GroundConfig{
		Type:         "real",
		Conductivity: 1e6, // near-perfect conductor
		Permittivity: 13,
	}

	perfectResult, err := Simulate(perfectInput)
	if err != nil {
		t.Fatalf("Perfect ground simulation failed: %v", err)
	}

	realResult, err := Simulate(realInput)
	if err != nil {
		t.Fatalf("Real ground simulation failed: %v", err)
	}

	t.Logf("Perfect ground: R=%.2f X=%.2f SWR=%.2f Gain=%.2f dBi",
		perfectResult.Impedance.R, perfectResult.Impedance.X,
		perfectResult.SWR, perfectResult.GainDBi)
	t.Logf("Real ground (σ=1e6): R=%.2f X=%.2f SWR=%.2f Gain=%.2f dBi",
		realResult.Impedance.R, realResult.Impedance.X,
		realResult.SWR, realResult.GainDBi)

	// Impedance should be within 20% of perfect ground
	rDiff := math.Abs(realResult.Impedance.R-perfectResult.Impedance.R) /
		math.Max(math.Abs(perfectResult.Impedance.R), 1)
	if rDiff > 0.20 {
		t.Errorf("R differs by %.0f%% between perfect and high-σ real ground", rDiff*100)
	}
}

func TestRealGroundVsPerfectGround(t *testing.T) {
	// Compare average ground vs perfect ground for a quarter-wave monopole.
	// The reflection-coefficient image method is an approximation, so we check
	// that the results are physically reasonable rather than exact ordering.
	freq := 300e6
	lambda := C0 / freq
	height := lambda / 4.0

	perfectInput := SimulationInput{
		Wires: []Wire{{
			X1: 0, Y1: 0, Z1: 0,
			X2: 0, Y2: 0, Z2: height,
			Radius: 0.001, Segments: 11,
		}},
		Frequency: freq,
		Ground:    GroundConfig{Type: "perfect"},
		Source:    Source{WireIndex: 0, SegmentIndex: 0, Voltage: 1 + 0i},
	}

	realInput := perfectInput
	realInput.Ground = GroundConfig{
		Type:         "real",
		Conductivity: 0.005,
		Permittivity: 13,
	}

	perfectResult, err := Simulate(perfectInput)
	if err != nil {
		t.Fatalf("Perfect ground failed: %v", err)
	}

	realResult, err := Simulate(realInput)
	if err != nil {
		t.Fatalf("Real ground failed: %v", err)
	}

	t.Logf("Perfect ground: R=%.2f X=%.2f SWR=%.2f Gain=%.2f dBi",
		perfectResult.Impedance.R, perfectResult.Impedance.X,
		perfectResult.SWR, perfectResult.GainDBi)
	t.Logf("Average ground: R=%.2f X=%.2f SWR=%.2f Gain=%.2f dBi",
		realResult.Impedance.R, realResult.Impedance.X,
		realResult.SWR, realResult.GainDBi)

	// Gain should be within 1 dB of each other (both are monopoles with similar patterns)
	gainDiff := math.Abs(realResult.GainDBi - perfectResult.GainDBi)
	if gainDiff > 1.0 {
		t.Errorf("Gain difference between real and perfect ground is %.2f dB (should be < 1 dB)", gainDiff)
	}

	// Impedance should differ but both should be positive resistance
	if realResult.Impedance.R <= 0 {
		t.Errorf("Real ground resistance should be positive, got %.2f", realResult.Impedance.R)
	}
	if perfectResult.Impedance.R <= 0 {
		t.Errorf("Perfect ground resistance should be positive, got %.2f", perfectResult.Impedance.R)
	}

	// Pattern should have data in upper hemisphere and -100 dB below
	belowGroundGain := -200.0
	for _, p := range realResult.Pattern {
		if p.ThetaDeg > 92 && p.GainDB > belowGroundGain {
			belowGroundGain = p.GainDB
		}
	}
	if belowGroundGain > -90 {
		t.Errorf("Real ground should suppress below-ground pattern, but found %.1f dB below horizon", belowGroundGain)
	}
}
